"""
Team Leader Agent — 组长/投资组合经理。

核心职责:
  1. 晨会: 综合宏观+猎手+风控意见，制定当日预算
  2. 仲裁: 猎手与风控冲突时，加权投票+LLM裁决
  3. 保本模式: 日亏达阈值时全面收紧
  4. 签收复盘: 确认教练的日终报告

仲裁协议:
  score = agent_confidence × agent_accuracy × gene_weight
  差距>15 → 直接判定（零LLM成本）
  差距≤15 → 调用LLM仲裁（每日最多2次）
"""

import json
import logging
import os
from datetime import date, datetime
from typing import Dict, List, Optional, Tuple

import httpx

from team_core import (
    AgentConfig, AgentStatus, BaseAgent, Decision, DecisionStore,
    default_team_configs, AgentRole,
)

logger = logging.getLogger("team_leader")


class TeamLeader(BaseAgent):
    """团队组长 — 预算分配 + 冲突仲裁。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore,
                 qmt_client=None, account: str = ""):
        super().__init__(config, decision_store)
        self.qmt = qmt_client
        self.account = account

        # Daily state
        self._today: Optional[date] = None
        self._daily_budget: float = 0
        self._daily_pnl_pct: float = 0
        self._protect_mode: bool = False
        self._llm_calls_today: int = 0
        self._max_llm_calls: int = 2
        self._arbitrations_today: int = 0

        # Agent accuracy cache (from DecisionStore)
        self._agent_accuracy: Dict[str, float] = {}
        # Gene-based weights for voting
        self._agent_weights: Dict[str, float] = {
            "hunter": 1.0,
            "risk": 1.2,    # risk officer gets higher weight
            "macro": 0.8,
            "leader": 1.0,
        }

    # ──────── Daily Reset ────────

    def _check_new_day(self):
        today = date.today()
        if self._today != today:
            self._today = today
            self._protect_mode = False
            self._llm_calls_today = 0
            self._arbitrations_today = 0
            self._refresh_accuracy()

    def _refresh_accuracy(self):
        """Refresh agent accuracy and dynamically adjust voting weights."""
        for role in ["hunter", "risk", "macro", "leader"]:
            # Prefer memory-stored accuracy (updated by coach), fallback to DB query
            mem_acc = self.memory.get(role, "accuracy_30d", None)
            if mem_acc is not None:
                self._agent_accuracy[role] = mem_acc
            else:
                self._agent_accuracy[role] = self.decisions.agent_accuracy(role)

        # Adaptive weights: base weight ± accuracy adjustment
        # Accuracy > 60% → boost weight, < 40% → reduce weight
        base_weights = {"hunter": 1.0, "risk": 1.2, "macro": 0.8, "leader": 1.0}
        for role, base in base_weights.items():
            acc = self._agent_accuracy.get(role, 0.5)
            # Scale: 40% acc → -0.2, 50% → 0, 60% → +0.2, 70% → +0.4
            adj = (acc - 0.5) * 2.0
            self._agent_weights[role] = round(max(0.4, min(2.0, base + adj)), 2)
        logger.info(f"[leader] Adaptive weights: {self._agent_weights} (acc: {self._agent_accuracy})")

    # ──────── Morning Briefing: Set Daily Budget ────────

    def morning_briefing(self, market_env: str, macro_data: dict = None,
                         total_assets: float = 0, market_value: float = 0,
                         available_cash: float = 0,
                         positions: list = None, sentiment: dict = None) -> dict:
        """
        晨会: 根据宏观环境设定当日风险预算 + LLM作战计划。

        Returns: {"budget": float, "max_exposure_pct": float, "protect_threshold": float, "battle_plan": str}
        """
        self._check_new_day()
        self.set_status(AgentStatus.ANALYZING)

        # Max exposure by environment (same as portfolio_risk but gland-adjusted)
        base_exposure = {
            "bull": 0.80, "sideways": 0.50, "bear": 0.20, "extreme_bear": 0.10,
        }.get(market_env, 0.50)

        gmod = self.gland.risk_modifier()
        adjusted_exposure = min(0.90, base_exposure * gmod)

        max_deploy = total_assets * adjusted_exposure - market_value
        budget = min(max(0, max_deploy), available_cash)
        self._daily_budget = budget

        # Protect threshold (gland-adjusted)
        protect_threshold = -0.3 * gmod

        # LLM battle plan (once per day, counts toward llm budget)
        battle_plan = ""
        if self._llm_calls_today == 0:
            battle_plan = self._generate_battle_plan(
                market_env, macro_data, total_assets, market_value,
                available_cash, budget, adjusted_exposure, positions, sentiment,
            )

        result = {
            "market_env": market_env,
            "budget": round(budget, 0),
            "max_exposure_pct": round(adjusted_exposure * 100, 1),
            "protect_threshold": round(protect_threshold, 2),
            "gland_modifier": round(gmod, 2),
            "battle_plan": battle_plan,
            "gland_state": self.gland.state,
        }

        self.log_decision(
            action="morning_briefing",
            target="portfolio",
            confidence=80,
            reasoning=f"晨会: {market_env}环境, 预算¥{budget:,.0f}, 最大仓位{adjusted_exposure:.0%}",
            data=result,
        )

        self.set_status(AgentStatus.IDLE)
        return result

    # ──────── LLM Battle Plan ────────

    def _generate_battle_plan(self, market_env, macro_data, total_assets, market_value,
                              available_cash, budget, exposure, positions, sentiment) -> str:
        """调用Qwen生成今日作战计划（每日最多1次）。"""
        api_key = os.getenv("QWEN_API_KEY", "")
        if not api_key:
            return ""
        base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
        model = os.getenv("QWEN_ARBITRATE_MODEL", "qwen-plus")

        # Build context
        pos_lines = []
        for p in (positions or [])[:8]:
            code = p.get("code", "?")
            pnl = p.get("pnl_pct", 0)
            pos_lines.append(f"{code} P&L={pnl:+.1f}%")

        sent_val = sentiment.get("composite", 50) if sentiment else 50
        macro_dir = macro_data.get("direction", "neutral") if macro_data else "neutral"
        breadth = macro_data.get("breadth_score", 0.5) if macro_data else 0.5

        # Load yesterday's coach review for continuity
        yesterday_review = ""
        pnl_hist = self.memory.get("coach", "daily_pnl_history", [])
        if pnl_hist:
            last = pnl_hist[-1]
            yesterday_review = f"\n**昨日复盘**: P&L={last.get('pnl', 0):+.2f}%, {last.get('decisions', 0)}条决策"
        last_report = self.memory.get("coach", "last_llm_report", "")
        if last_report and len(last_report) > 10:
            yesterday_review += f"\n**教练建议**: {last_report[:120]}"

        # Load team performance stats
        perf_ctx = ""
        total_days = self.memory.get("coach", "total_review_days", 0)
        profit_days = self.memory.get("coach", "profitable_days", 0)
        if total_days > 0:
            perf_ctx = f"\n**历史绩效**: {total_days}天, 盈利{profit_days}天, 胜率{profit_days/total_days:.0%}"

        prompt = f"""你是量化交易团队的组长，请制定今日作战计划（3-5句话）。

**市场**: {market_env} · 方向{macro_dir} · 宽度{breadth:.0%} · 情绪{sent_val:.0f}
**账户**: 总资产¥{total_assets:,.0f} · 市值¥{market_value:,.0f} · 可用¥{available_cash:,.0f}
**预算**: ¥{budget:,.0f} · 最大仓位{exposure:.0%}
**持仓**: {', '.join(pos_lines) if pos_lines else '无'}{yesterday_review}{perf_ctx}

要求：1.今日操作方向(进攻/防守/观望) 2.重点关注板块 3.止损纪律 4.特殊注意事项
只输出纯文本，不要markdown。"""

        try:
            self._llm_calls_today += 1
            with httpx.Client(timeout=15.0) as client:
                resp = client.post(
                    f"{base_url}/chat/completions",
                    headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
                    json={
                        "model": model,
                        "messages": [
                            {"role": "system", "content": "你是严谨的量化交易组长，言简意赅。"},
                            {"role": "user", "content": prompt},
                        ],
                        "temperature": 0.3,
                        "max_tokens": 200,
                    },
                )
                resp.raise_for_status()
                plan = resp.json()["choices"][0]["message"]["content"].strip()
                logger.info(f"[leader] Battle plan: {plan[:60]}...")
                # Save to memory
                self.memory.set("leader", "battle_plan", {
                    "date": date.today().isoformat(), "plan": plan,
                })
                return plan
        except Exception as e:
            logger.warning(f"[leader] Battle plan LLM failed: {e}")
            return ""

    # ──────── Arbitrate: Resolve Hunter vs Risk Conflict ────────

    def arbitrate(self, candidate: dict,
                  hunter_confidence: float, hunter_reason: str,
                  risk_action: str, risk_confidence: float, risk_reason: str,
                  macro_confidence: float = 50, macro_env: str = "sideways") -> Tuple[str, str, dict]:
        """
        仲裁猎手与风控的冲突。

        Returns: (final_action, reason, details)
            final_action: 'approve' / 'approve_half' / 'reject'
        """
        self._check_new_day()
        self.set_status(AgentStatus.ARBITRATING)
        self._arbitrations_today += 1

        code = candidate.get("code", "")

        # Step 1: Weighted scoring
        h_acc = self._agent_accuracy.get("hunter", 0.5)
        r_acc = self._agent_accuracy.get("risk", 0.5)
        m_acc = self._agent_accuracy.get("macro", 0.5)

        h_w = self._agent_weights.get("hunter", 1.0)
        r_w = self._agent_weights.get("risk", 1.2)
        m_w = self._agent_weights.get("macro", 0.8)

        buy_score = hunter_confidence * h_acc * h_w
        sell_score = risk_confidence * r_acc * r_w

        # Macro tilts the balance
        if macro_env in ("bear", "extreme_bear"):
            sell_score += macro_confidence * m_acc * m_w * 0.5
        elif macro_env == "bull":
            buy_score += macro_confidence * m_acc * m_w * 0.3

        diff = buy_score - sell_score

        # Phase 11: Kelly weight adjusts scoring
        kelly = candidate.get("kelly_weight", 0)
        if kelly >= 0.06:
            buy_score += 8   # strong kelly → tilt buy
        elif kelly >= 0.03:
            buy_score += 3
        elif kelly < 0.01 and kelly >= 0:
            sell_score += 5  # weak kelly → tilt sell

        diff = buy_score - sell_score

        details = {
            "buy_score": round(buy_score, 1),
            "sell_score": round(sell_score, 1),
            "diff": round(diff, 1),
            "kelly_weight": round(kelly, 4),
            "hunter": {"confidence": hunter_confidence, "accuracy": round(h_acc, 2)},
            "risk": {"confidence": risk_confidence, "accuracy": round(r_acc, 2), "action": risk_action},
            "macro_env": macro_env,
            "method": "rule",
        }

        # Step 2: Decision based on score difference
        if diff > 15:
            # Clear buy signal — kelly further determines full vs half
            if kelly >= 0.04:
                final = "approve"
                reason = f"加权投票:买{buy_score:.0f}vs卖{sell_score:.0f}(差{diff:.0f}>15),Kelly={kelly:.3f}→全仓批准"
            else:
                final = "approve_half"
                reason = f"加权投票通过但Kelly={kelly:.3f}偏低→半仓批准"
        elif diff < -15:
            # Clear reject
            final = "reject"
            reason = f"加权投票:买{buy_score:.0f}vs卖{sell_score:.0f}(差{diff:.0f}<-15),否决"
        elif risk_action == "reduce_size":
            # Risk says reduce, not full veto → compromise
            final = "approve_half"
            reason = f"加权投票接近(差{diff:.0f}),风控建议减仓→半仓买入"
        else:
            # Close call → try LLM arbitration
            if self._llm_calls_today < self._max_llm_calls:
                self._llm_calls_today += 1
                llm_result = self._call_llm_arbitrate(
                    code, candidate, hunter_confidence, hunter_reason,
                    risk_action, risk_confidence, risk_reason,
                    macro_env, self._daily_pnl_pct, diff,
                )
                if llm_result:
                    final = llm_result["action"]
                    reason = f"LLM仲裁: {llm_result['reasoning']}"
                    details["method"] = "llm_qwen"
                    details["llm_raw"] = llm_result.get("raw", "")
                else:
                    # LLM failed → fallback conservative
                    if self._daily_pnl_pct > 0:
                        final = "approve_half"
                        reason = f"LLM失败降级: 日内盈利{self._daily_pnl_pct:+.1f}%→半仓"
                    else:
                        final = "reject"
                        reason = f"LLM失败降级: 日内亏{self._daily_pnl_pct:+.1f}%→拒绝"
                    details["method"] = "llm_fallback"
            else:
                final = "reject"
                reason = f"LLM次数用尽({self._llm_calls_today}), 默认保守拒绝"
                details["method"] = "default_conservative"

        self.log_decision(
            action=f"arbitrate_{final}",
            target=code,
            confidence=abs(diff) + 50,
            reasoning=reason,
            data=details,
        )

        self.set_status(AgentStatus.IDLE)
        return final, reason, details

    # ──────── LLM Arbitration (Qwen) ────────

    def _call_llm_arbitrate(self, code: str, candidate: dict,
                            h_conf: float, h_reason: str,
                            r_action: str, r_conf: float, r_reason: str,
                            macro_env: str, daily_pnl: float, diff: float) -> Optional[dict]:
        """调用Qwen LLM做真正的仲裁决策。使用qwen-plus(更便宜)。"""
        api_key = os.getenv("QWEN_API_KEY", "")
        if not api_key:
            logger.warning("[leader] QWEN_API_KEY not set, skip LLM arbitration")
            return None

        base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
        model = os.getenv("QWEN_ARBITRATE_MODEL", "qwen-plus")

        score = candidate.get("score", 0)
        lifecycle = candidate.get("lifecycle", "unknown")
        chip = candidate.get("chip_shape", "unknown")
        kelly = candidate.get("kelly_weight", 0)

        prompt = f"""你是A股量化交易团队的组长，需要仲裁一个买入决策冲突。

**候选股票**: {code}
- 评分: {score:.2f}
- 生命周期: {lifecycle}
- 筹码形态: {chip}
- Kelly仓位: {kelly:.3f}

**猎手意见**: 买入 (信心{h_conf:.0f}%)
  理由: {h_reason}

**风控官意见**: {r_action} (信心{r_conf:.0f}%)
  理由: {r_reason}

**当前状态**:
- 市场环境: {macro_env}
- 当日盈亏: {daily_pnl:+.2f}%
- 加权投票差: {diff:+.1f} (接近，无法自动裁决)

请做出最终决策，只回复JSON格式:
{{"action": "approve"或"approve_half"或"reject", "reasoning": "一句话理由(20字内)"}}"""

        try:
            with httpx.Client(timeout=15.0) as client:
                resp = client.post(
                    f"{base_url}/chat/completions",
                    headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
                    json={
                        "model": model,
                        "messages": [
                            {"role": "system", "content": "你是严谨的量化交易组长，回复纯JSON，不要markdown。"},
                            {"role": "user", "content": prompt},
                        ],
                        "temperature": 0.1,
                        "max_tokens": 100,
                    },
                )
                resp.raise_for_status()
                data = resp.json()
                raw = data["choices"][0]["message"]["content"].strip()
                # Parse JSON from response
                raw_clean = raw.strip("`").strip()
                if raw_clean.startswith("json"):
                    raw_clean = raw_clean[4:].strip()
                result = json.loads(raw_clean)
                action = result.get("action", "reject")
                if action not in ("approve", "approve_half", "reject"):
                    action = "reject"
                logger.info(f"[leader] LLM arbitrate {code}: {action} — {result.get('reasoning', '')}")
                return {"action": action, "reasoning": result.get("reasoning", ""), "raw": raw}
        except Exception as e:
            logger.warning(f"[leader] LLM arbitrate failed: {e}")
            return None

    # ──────── Protect Mode Control ────────

    def enter_protect_mode(self, daily_pnl_pct: float):
        """由风控官触发的保本模式。"""
        if not self._protect_mode:
            self._protect_mode = True
            self._daily_pnl_pct = daily_pnl_pct
            self._daily_budget = max(0, self._daily_budget * 0.3)
            self.set_status(AgentStatus.ALERTING)
            self.log_decision(
                action="protect_mode_enter",
                target="portfolio",
                confidence=95,
                reasoning=f"组长确认保本模式: 日亏{daily_pnl_pct:+.2f}%, 预算缩至¥{self._daily_budget:,.0f}",
                data={"pnl_pct": daily_pnl_pct, "budget_after": self._daily_budget},
            )
            logger.warning(f"[leader] PROTECT MODE: pnl={daily_pnl_pct:+.2f}%, budget→¥{self._daily_budget:,.0f}")

    def exit_protect_mode(self):
        if self._protect_mode:
            self._protect_mode = False
            self.log_decision("protect_mode_exit", "portfolio", 70, "保本模式解除")

    @property
    def is_protect_mode(self) -> bool:
        return self._protect_mode

    # ──────── Update P&L (called from main loop) ────────

    def update_pnl(self, daily_pnl_pct: float):
        self._daily_pnl_pct = daily_pnl_pct
        self.gland.update(daily_pnl_pct)

    # ──────── Status for Frontend ────────

    def get_state(self) -> dict:
        base = super().get_state()
        base.update({
            "daily_pnl_pct": round(self._daily_pnl_pct, 2),
            "protect_mode": self._protect_mode,
            "daily_budget": round(self._daily_budget, 0),
            "llm_calls_today": self._llm_calls_today,
            "max_llm_calls": self._max_llm_calls,
            "arbitrations_today": self._arbitrations_today,
            "agent_accuracy": {k: round(v, 2) for k, v in self._agent_accuracy.items()},
            "agent_weights": self._agent_weights,
        })
        return base
