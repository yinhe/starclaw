"""
Team Manager — 团队智能体调度中心。

负责:
  1. 实例化5个Agent（组长/宏观/猎手/风控/教练）
  2. 协调Agent间的交互流程
  3. 提供统一API接口给前端
  4. 接入现有 strategy_executor 交易流程
"""

import json
import logging
from datetime import date, datetime
from typing import Dict, List, Optional, Tuple

from team_core import (
    AgentConfig, AgentStatus, BaseAgent, Decision, DecisionStore,
    Gene, Gland, GeneType, GlandState, AgentMemory,
    default_team_configs, AgentRole,
)
from risk_officer import RiskOfficer
from team_leader import TeamLeader

logger = logging.getLogger("team_manager")


class MacroAgent(BaseAgent):
    """宏观分析师 — 本能驱动的市场评级。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore):
        super().__init__(config, decision_store)
        self._current_env = "sideways"
        self._current_rating = 50  # 0-100
        self._risk_multiplier = 1.0

    def rate_market(self, env: str, macro: dict = None, sentiment: dict = None) -> dict:
        """产出市场评级 + 动态调整全队风险偏好。"""
        self.set_status(AgentStatus.ANALYZING)
        self._current_env = env

        # Rule-based rating from macro data
        rating = 50
        alerts = []

        if macro:
            breadth = macro.get("breadth_score", 0.5)
            if breadth < 0.3:
                rating -= 20
                alerts.append(f"宽度{breadth:.0%}偏低")
            elif breadth > 0.7:
                rating += 15

            direction = macro.get("direction", "neutral")
            if direction == "bearish":
                rating -= 15
                alerts.append("宏观方向偏空")
            elif direction == "bullish":
                rating += 15

        if sentiment:
            sent_val = sentiment.get("composite", 50)
            if sent_val < 30:
                rating -= 20
                alerts.append(f"情绪{sent_val}极度恐惧")
            elif sent_val > 70:
                rating += 10

        # Env override
        env_adj = {"bull": 15, "sideways": 0, "bear": -20, "extreme_bear": -35}.get(env, 0)
        rating += env_adj

        rating = max(0, min(100, rating))
        self._current_rating = rating

        # Instinct: alert thresholds
        if rating < 30:
            self.set_status(AgentStatus.ALERTING)
            alerts.append("一级风险警报")

        # Dynamic risk adjustment: macro rating → team risk_multiplier
        # rating 0-30: conservative (0.6x), 30-70: normal (1.0x), 70-100: aggressive (1.3x)
        if rating < 30:
            self._risk_multiplier = 0.6
        elif rating > 70:
            self._risk_multiplier = 1.3
        else:
            self._risk_multiplier = 0.8 + (rating - 30) / 100

        confidence = abs(rating - 50) + 50
        action = "bullish" if rating > 60 else "bearish" if rating < 40 else "neutral"

        self.log_decision(
            action=f"market_{action}",
            target="market",
            confidence=confidence,
            reasoning=f"评级{rating}: {', '.join(alerts) if alerts else '各项正常'} risk_mult={self._risk_multiplier:.2f}",
            data={"rating": rating, "env": env, "alerts": alerts, "risk_multiplier": self._risk_multiplier},
        )

        # Persist to memory
        self.memory.set("macro", "last_rating", {"rating": rating, "env": env, "action": action})
        self.memory.append_list("macro", "rating_history", {
            "date": datetime.now().strftime("%Y-%m-%d %H:%M"),
            "rating": rating, "env": env,
        }, max_len=200)

        self.set_status(AgentStatus.IDLE)
        return {
            "rating": rating,
            "env": env,
            "action": action,
            "alerts": alerts,
            "confidence": confidence,
            "risk_multiplier": self._risk_multiplier,
        }

    @property
    def risk_multiplier(self) -> float:
        return self._risk_multiplier

    def get_state(self) -> dict:
        base = super().get_state()
        base.update({
            "current_env": self._current_env,
            "current_rating": self._current_rating,
            "risk_multiplier": round(self._risk_multiplier, 2),
        })
        return base


class HunterAgent(BaseAgent):
    """选股猎手 — 记忆驱动的智能选股。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore):
        super().__init__(config, decision_store)
        self._proposals_today = 0
        self._today: Optional[date] = None
        self._max_proposals = 5
        self._base_score_threshold = 0.60
        self._score_threshold = 0.60  # dynamic, adjusted by memory

    def _check_new_day(self):
        today = date.today()
        if self._today != today:
            self._today = today
            self._proposals_today = 0
            self._load_memory_preferences()

    def _load_memory_preferences(self):
        """从记忆加载历史偏好，动态调整选股参数。"""
        # Adjust score threshold based on recent win rate
        win_rate = self.memory.get("hunter", "accuracy_30d", 0.5)
        if win_rate > 0.6:
            self._score_threshold = max(0.55, self._base_score_threshold - 0.03)
        elif win_rate < 0.4:
            self._score_threshold = min(0.70, self._base_score_threshold + 0.05)
        else:
            self._score_threshold = self._base_score_threshold

        # Load preferred strategies/sectors from memory
        self._hot_strategies = self.memory.get("hunter", "hot_strategies", [])
        self._cold_sectors = self.memory.get("hunter", "cold_sectors", [])

        # Load blacklist: stocks that triggered stop_loss within 30 days
        self._blacklist = set()
        bl_raw = self.memory.get("hunter", "stop_loss_blacklist", [])
        today = date.today()
        kept = []
        for entry in (bl_raw if isinstance(bl_raw, list) else []):
            exp = entry.get("expires", "")
            if exp >= today.isoformat():
                self._blacklist.add(entry.get("code", ""))
                kept.append(entry)
        if len(kept) != len(bl_raw):
            self.memory.set("hunter", "stop_loss_blacklist", kept)
        if self._blacklist:
            logger.info(f"[hunter] Blacklist loaded: {self._blacklist}")

    def propose_candidates(self, candidates: List[dict]) -> List[dict]:
        """提交扫描结果作为候选（记忆增强）。"""
        self._check_new_day()
        self.set_status(AgentStatus.SCANNING)

        # Apply instinct: max 5 per day
        remaining = self._max_proposals - self._proposals_today
        if remaining <= 0:
            self.log_decision("quota_exhausted", "pool", 90,
                              f"今日已提交{self._proposals_today}只,达到上限{self._max_proposals}")
            self.set_status(AgentStatus.IDLE)
            return []

        # Score adjustment: boost candidates matching hot strategies, penalize cold sectors
        scored = []
        for c in candidates:
            code = c.get("code", "")
            # Blacklist filter: skip stocks that triggered stop_loss recently
            if hasattr(self, '_blacklist') and code in self._blacklist:
                continue

            score = c.get("score", 0)
            reason_parts = [c.get("reason", f"评分{score:.2f}")]

            # Memory boost: strategies that worked before get +5% bonus
            strategy = c.get("strategy", "")
            if strategy and strategy in self._hot_strategies:
                score += 0.05
                reason_parts.append(f"热策略+5%({strategy})")

            # Memory penalty: sectors that lost money get -5%
            sector = c.get("sector", "")
            if sector and sector in self._cold_sectors:
                score -= 0.05
                reason_parts.append(f"冷板块-5%({sector})")

            if score < self._score_threshold:
                continue

            # Classify strategy type from reason text (Phase 11)
            stype = self._classify_strategy(c.get("reason", ""))
            c["strategy_type"] = stype

            scored.append((score, c, " · ".join(reason_parts)))

        # Sort by adjusted score (best first)
        scored.sort(key=lambda x: -x[0])

        proposals = []
        for score, c, reason in scored:
            if self._proposals_today >= self._max_proposals:
                break

            self._proposals_today += 1
            self.log_decision(
                action="propose_buy",
                target=c.get("code", ""),
                confidence=min(100, score * 100),
                reasoning=reason,
                data={"score": score, "rank": self._proposals_today,
                      "threshold": self._score_threshold},
            )
            proposals.append(c)

        # Track what we proposed today in memory
        if proposals:
            self.memory.append_list("hunter", "daily_proposals", {
                "date": date.today().isoformat(),
                "count": len(proposals),
                "codes": [p.get("code", "") for p in proposals],
                "threshold": self._score_threshold,
            }, max_len=90)

        self.set_status(AgentStatus.IDLE)
        return proposals[:remaining]

    # Strategy type classification keywords
    _TYPE_KEYWORDS = {
        "trend": ["多头", "趋势", "均线", "MA", "trend", "排列"],
        "breakout": ["突破", "新高", "break", "形态", "预突破"],
        "volume": ["放量", "量能", "VR", "量比", "volume", "缩量"],
    }

    def _classify_strategy(self, reason: str) -> str:
        """Classify a strategy reason into type: trend/breakout/volume/other."""
        r = reason.lower() if reason else ""
        for stype, kws in self._TYPE_KEYWORDS.items():
            for kw in kws:
                if kw.lower() in r:
                    return stype
        return "other"

    def record_outcome(self, code: str, strategy: str, sector: str, is_profit: bool):
        """教练反馈：记录选股结果，更新策略/板块/类型偏好。"""
        # Track per-strategy accuracy
        key = f"strategy_{strategy}" if strategy else "strategy_unknown"
        stats = self.memory.get("hunter", key, {"wins": 0, "total": 0})
        stats["total"] += 1
        if is_profit:
            stats["wins"] += 1
        self.memory.set("hunter", key, stats)

        # Track per-sector accuracy
        if sector:
            skey = f"sector_{sector}"
            sstats = self.memory.get("hunter", skey, {"wins": 0, "total": 0})
            sstats["total"] += 1
            if is_profit:
                sstats["wins"] += 1
            self.memory.set("hunter", skey, sstats)

        # Track per-type accuracy (trend/breakout/volume/other)
        stype = self._classify_strategy(strategy)
        tkey = f"type_{stype}"
        tstats = self.memory.get("hunter", tkey, {"wins": 0, "total": 0})
        tstats["total"] += 1
        if is_profit:
            tstats["wins"] += 1
        self.memory.set("hunter", tkey, tstats)

        # Blacklist: if loss, add code to 30-day blacklist
        if not is_profit and code:
            from datetime import timedelta
            expires = (date.today() + timedelta(days=30)).isoformat()
            self.memory.append_list("hunter", "stop_loss_blacklist", {
                "code": code, "date": date.today().isoformat(), "expires": expires,
            }, max_len=50)
            logger.info(f"[hunter] Blacklisted {code} until {expires}")

    def get_state(self) -> dict:
        base = super().get_state()
        base.update({
            "proposals_today": self._proposals_today,
            "max_proposals": self._max_proposals,
            "score_threshold": round(self._score_threshold, 3),
            "hot_strategies": self._hot_strategies if hasattr(self, '_hot_strategies') else [],
            "cold_sectors": self._cold_sectors if hasattr(self, '_cold_sectors') else [],
        })
        return base


class CoachAgent(BaseAgent):
    """复盘教练 — 日终分析 + LLM复盘 + 基因调优建议 + 记忆持久化。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore):
        super().__init__(config, decision_store)

    def daily_review(self, trades_today: List[dict] = None,
                     daily_pnl: float = 0) -> dict:
        """日终复盘（规则分析 + LLM总结 + 记忆写入）。"""
        self.set_status(AgentStatus.REVIEWING)

        today_decisions = self.decisions.today()

        # Analyze today's decisions by agent
        agent_summary = {}
        for d in today_decisions:
            ag = d["agent"]
            if ag not in agent_summary:
                agent_summary[ag] = {"total": 0, "actions": {}}
            agent_summary[ag]["total"] += 1
            act = d["action"]
            agent_summary[ag]["actions"][act] = agent_summary[ag]["actions"].get(act, 0) + 1

        # Count veto-then-profit (risk officer vetoed but stock went up)
        veto_miss = 0
        for d in today_decisions:
            if d["agent"] == "risk" and d["action"] == "veto" and d.get("outcome") == "missed_profit":
                veto_miss += 1

        # Gene mutation suggestions
        mutations = []
        risk_stats = self.decisions.agent_stats("risk")
        veto_rate = risk_stats.get("veto_rate", 0)
        if veto_rate > 0.8:
            mutations.append({
                "agent": "risk",
                "param": "veto_rate_cap",
                "direction": "decrease",
                "reason": f"否决率{veto_rate:.0%}过高,建议放宽到65%",
            })
        if veto_miss >= 2:
            mutations.append({
                "agent": "risk",
                "param": "risk_tolerance",
                "direction": "increase",
                "reason": f"今日{veto_miss}次否决后标的上涨,风控偏保守",
            })

        # LLM daily review report
        llm_report = self._llm_daily_review(today_decisions, trades_today, daily_pnl, agent_summary)

        review = {
            "date": date.today().isoformat(),
            "daily_pnl": round(daily_pnl, 2),
            "total_decisions": len(today_decisions),
            "agent_summary": agent_summary,
            "veto_missed_profit": veto_miss,
            "gene_mutations": mutations,
            "trades_today": len(trades_today or []),
            "llm_report": llm_report,
        }

        self.log_decision(
            action="daily_review",
            target="portfolio",
            confidence=85,
            reasoning=f"日终复盘: P&L={daily_pnl:+.2f}%, {len(today_decisions)}条决策",
            data=review,
        )

        # Persist to memory
        self._save_to_memory(review, daily_pnl)

        self.set_status(AgentStatus.IDLE)
        return review

    def _llm_daily_review(self, decisions: list, trades: list,
                          daily_pnl: float, agent_summary: dict) -> str:
        """调用Qwen生成日终复盘报告。"""
        import os, httpx
        api_key = os.getenv("QWEN_API_KEY", "")
        if not api_key:
            return ""
        base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
        model = os.getenv("QWEN_ARBITRATE_MODEL", "qwen-plus")

        # Build compact summary
        decision_lines = []
        for d in decisions[-20:]:
            decision_lines.append(f"{d['ts'][-8:]} {d['agent']}:{d['action']} {d['target']} conf={d['confidence']}")
        trade_lines = []
        for t in (trades or [])[:10]:
            code = t.get("code", "?")
            pnl = t.get("pnl_pct", t.get("pnl", 0))
            trade_lines.append(f"{code} P&L={pnl:+.1f}%")

        # Phase 11: Strategy type performance from hunter memory
        type_perf_lines = []
        all_mem = self.memory.get_all("hunter")
        for key, entry in all_mem.items():
            val = entry.get("value", {})
            if not isinstance(val, dict) or "wins" not in val:
                continue
            if key.startswith("type_") and val.get("total", 0) >= 2:
                tname = key.replace("type_", "")
                wr = val["wins"] / val["total"]
                type_perf_lines.append(f"{tname}: {val['wins']}/{val['total']}({wr:.0%})")

        type_perf_str = ", ".join(type_perf_lines) if type_perf_lines else "暂无数据"

        prompt = f"""你是AI量化交易团队的复盘教练，请分析今天的表现并给出简洁建议。

**今日盈亏**: {daily_pnl:+.2f}%
**团队决策** ({len(decisions)}条):
{chr(10).join(decision_lines) if decision_lines else "无决策"}

**成交记录** ({len(trades or [])}笔):
{chr(10).join(trade_lines) if trade_lines else "无成交"}

**Agent统计**: {json.dumps(agent_summary, ensure_ascii=False)}

**策略类型胜率**: {type_perf_str}

请用3-5句话总结: 1.今天做对了什么 2.犯了什么错 3.策略类型偏好建议(哪类策略应加强/削弱) 4.明天改进建议。
只输出纯文本，不要markdown格式。"""

        try:
            with httpx.Client(timeout=20.0) as client:
                resp = client.post(
                    f"{base_url}/chat/completions",
                    headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
                    json={
                        "model": model,
                        "messages": [
                            {"role": "system", "content": "你是严谨客观的量化交易复盘教练，言简意赅。"},
                            {"role": "user", "content": prompt},
                        ],
                        "temperature": 0.3,
                        "max_tokens": 300,
                    },
                )
                resp.raise_for_status()
                content = resp.json()["choices"][0]["message"]["content"].strip()
                logger.info(f"[coach] LLM review: {content[:80]}...")
                return content
        except Exception as e:
            logger.warning(f"[coach] LLM review failed: {e}")
            return ""

    def feedback_outcomes(self, closed_trades: List[dict], hunter: 'HunterAgent' = None):
        """反馈闭环：根据已平仓交易标注决策outcome，更新猎手记忆。"""
        if not closed_trades:
            return
        self.set_status(AgentStatus.REVIEWING)

        today_decisions = self.decisions.today()
        # Build code→decision map for today's propose_buy actions
        propose_map = {}
        for d in today_decisions:
            if d["action"] == "propose_buy" and d["target"]:
                propose_map[d["target"]] = d

        marked = 0
        for trade in closed_trades:
            code = trade.get("code", "")
            pnl = trade.get("pnl_pct", trade.get("pnl", 0))
            is_profit = pnl > 0
            outcome = "profit" if is_profit else "loss"

            # Update decision outcome in DB
            if code in propose_map:
                did = propose_map[code].get("id")
                if did:
                    self.decisions.update_outcome(did, outcome, pnl)
                    marked += 1

            # Feed back to hunter memory
            if hunter:
                strategy = trade.get("strategy", trade.get("reason", ""))
                sector = trade.get("sector", "")
                hunter.record_outcome(code, strategy, sector, is_profit)

        # After all outcomes, derive hot_strategies and cold_sectors
        if hunter and marked > 0:
            self._derive_hunter_preferences(hunter)

        if marked:
            logger.info(f"[coach] Marked {marked} decisions with outcomes from {len(closed_trades)} trades")
        self.set_status(AgentStatus.IDLE)

    def _derive_hunter_preferences(self, hunter: 'HunterAgent'):
        """从猎手记忆中提取热门策略和冷门板块 + 策略类型统计。"""
        all_mem = self.memory.get_all("hunter")
        hot = []
        cold = []
        type_stats = {}  # Phase 11: strategy type stats for frontend
        for key, entry in all_mem.items():
            val = entry.get("value", {})
            if not isinstance(val, dict) or "wins" not in val:
                continue
            total = val.get("total", 0)
            if total < 2:
                continue
            win_rate = val["wins"] / total
            name = key.replace("strategy_", "").replace("sector_", "").replace("type_", "")
            if key.startswith("strategy_") and win_rate > 0.6:
                hot.append(name)
            elif key.startswith("sector_") and win_rate < 0.35:
                cold.append(name)
            elif key.startswith("type_"):
                type_stats[name] = {"win_rate": round(win_rate, 3), "total": total, "wins": val["wins"]}
        self.memory.set("hunter", "hot_strategies", hot)
        self.memory.set("hunter", "cold_sectors", cold)
        self.memory.set("hunter", "strategy_type_stats", type_stats)
        if hot or cold or type_stats:
            logger.info(f"[coach] Hunter preferences updated: hot={hot} cold={cold} types={list(type_stats.keys())}")

    def _save_to_memory(self, review: dict, daily_pnl: float):
        """将复盘结果写入持久记忆。"""
        today = date.today().isoformat()

        # Save LLM report for next-day morning briefing reference
        if review.get("llm_report"):
            self.memory.set("coach", "last_llm_report", review["llm_report"])

        # Track daily P&L history
        self.memory.append_list("coach", "daily_pnl_history", {
            "date": today,
            "pnl": round(daily_pnl, 4),
            "decisions": review["total_decisions"],
            "mutations": len(review["gene_mutations"]),
        }, max_len=90)

        # Track gene mutation history
        for m in review.get("gene_mutations", []):
            self.memory.append_list("coach", "gene_mutation_history", {
                "date": today, **m,
            }, max_len=50)

        # Update per-agent accuracy in memory
        for role in ("leader", "macro", "hunter", "risk"):
            acc = self.decisions.agent_accuracy(role)
            self.memory.set(role, "accuracy_30d", round(acc, 4))

        # Track cumulative stats
        self.memory.incr("coach", "total_review_days")
        if daily_pnl > 0:
            self.memory.incr("coach", "profitable_days")
        elif daily_pnl < 0:
            self.memory.incr("coach", "loss_days")


# ═══════════════ Team Manager ═══════════════

class TeamManager:
    """团队智能体调度中心。"""

    def __init__(self, qmt_client=None, account: str = ""):
        self.store = DecisionStore()
        self.memory = AgentMemory()
        configs = default_team_configs()

        # Instantiate all agents with shared memory
        self.leader = TeamLeader(configs[AgentRole.LEADER], self.store, qmt_client, account)
        self.leader.memory = self.memory
        self.macro = MacroAgent(configs[AgentRole.MACRO], self.store)
        self.macro.memory = self.memory
        self.hunter = HunterAgent(configs[AgentRole.HUNTER], self.store)
        self.hunter.memory = self.memory
        self.risk = RiskOfficer(configs[AgentRole.RISK], self.store, qmt_client, account)
        self.risk.memory = self.memory
        self.coach = CoachAgent(configs[AgentRole.COACH], self.store)
        self.coach.memory = self.memory

        self._agents: Dict[str, BaseAgent] = {
            "leader": self.leader,
            "macro": self.macro,
            "hunter": self.hunter,
            "risk": self.risk,
            "coach": self.coach,
        }

        logger.info(f"[team] Initialized 5 agents: {list(self._agents.keys())}")

    # ──────── Agent Collaboration Signals ────────

    def dispatch_signals(self):
        """Agent间协作通信：宏观预警→猎手限流，风控紧急→组长保护。"""
        # Macro → Hunter: if rating < 30, reduce hunter max proposals
        macro_rating = self.macro._current_rating
        if macro_rating < 30:
            self.hunter._max_proposals = max(1, self.hunter._max_proposals - 2)
            self.hunter.memory.set("hunter", "macro_warning", {
                "rating": macro_rating, "msg": "宏观一级警报:限流选股",
            })
            logger.info(f"[team] Macro→Hunter: rating={macro_rating} limit proposals to {self.hunter._max_proposals}")
        elif macro_rating > 70:
            self.hunter._max_proposals = min(8, 5 + 1)  # slightly boost in bull
        else:
            self.hunter._max_proposals = 5  # reset to default

        # Risk → Leader: if protect mode triggered, notify leader immediately
        if self.risk.is_protect_mode and not self.leader.is_protect_mode:
            self.leader.enter_protect_mode(self.risk._daily_pnl_pct)
            logger.info(f"[team] Risk→Leader: protect mode propagated, pnl={self.risk._daily_pnl_pct:.2f}%")

        # Macro → Risk: pass risk_multiplier to risk officer gland adjustment
        if hasattr(self.macro, '_risk_multiplier'):
            rm = self.macro._risk_multiplier
            if rm < 0.8:
                self.risk.gland.cortisol = min(100, self.risk.gland.cortisol + 10)

    # ──────── Full Scan Pipeline (replaces direct strategy_executor flow) ────────

    def process_candidates(self, raw_candidates: List[dict],
                           market_env: str = "sideways",
                           macro_data: dict = None,
                           sentiment: dict = None,
                           held_codes: set = None) -> List[dict]:
        """
        完整的团队决策流水线:
          1. 宏观分析师评级
          2. 猎手筛选候选
          3. 风控官逐个审核
          4. 组长仲裁冲突
          5. 返回最终批准列表

        Returns: List of approved candidates with team metadata
        """
        # Step 1: Macro rating
        macro_result = self.macro.rate_market(market_env, macro_data, sentiment)
        macro_conf = macro_result.get("confidence", 50)

        # Step 1.5: Dispatch inter-agent signals
        self.dispatch_signals()

        # Step 2: Hunter proposes
        proposals = self.hunter.propose_candidates(raw_candidates)
        if not proposals:
            return []

        # Step 3: Risk reviews each
        approved = []
        for cand in proposals:
            r_action, r_reason, r_conf = self.risk.review_candidate(
                cand, market_env, held_codes,
            )

            h_conf = min(100, cand.get("score", 0) * 100)

            if r_action == "approve":
                # Both agree → pass through
                cand["team_action"] = "approve"
                cand["team_reason"] = f"风控通过: {r_reason}"
                approved.append(cand)
                # Record position context for risk officer exit analysis
                self._record_position_context(cand)

            elif r_action == "veto":
                # Conflict! → Leader arbitrates
                final, arb_reason, details = self.leader.arbitrate(
                    cand,
                    hunter_confidence=h_conf,
                    hunter_reason=cand.get("reason", ""),
                    risk_action=r_action,
                    risk_confidence=r_conf,
                    risk_reason=r_reason,
                    macro_confidence=macro_conf,
                    macro_env=market_env,
                )
                if final == "approve":
                    cand["team_action"] = "approve_overrule"
                    cand["team_reason"] = arb_reason
                    approved.append(cand)
                    self._record_position_context(cand)
                elif final == "approve_half":
                    cand["team_action"] = "approve_half"
                    cand["team_reason"] = arb_reason
                    cand["reduce_position"] = True
                    approved.append(cand)
                    self._record_position_context(cand)
                else:
                    cand["team_action"] = "reject"
                    cand["team_reason"] = arb_reason
                    # Not added to approved

            elif r_action == "reduce_size":
                # Risk says reduce → auto-approve with smaller size
                cand["team_action"] = "approve_half"
                cand["team_reason"] = f"风控减仓: {r_reason}"
                cand["reduce_position"] = True
                approved.append(cand)
                self._record_position_context(cand)

        return approved

    def _record_position_context(self, cand: dict):
        """记录买入上下文到风控记忆, 供退出分析使用。"""
        code = cand.get("code", "")
        if not code:
            return
        ctx = {
            "code": code,
            "strategy": cand.get("strategy", cand.get("reason", "")),
            "strategy_type": cand.get("strategy_type", "other"),
            "sector": cand.get("sector", ""),
            "score": round(cand.get("score", 0), 3),
            "signal_strength": cand.get("signal_strength", 0),
            "kelly_weight": round(cand.get("kelly_weight", 0), 4),
            "team_action": cand.get("team_action", ""),
            "buy_date": date.today().isoformat(),
        }
        self.memory.set("risk", f"pos_ctx_{code}", ctx)
        logger.info(f"[team] Position context saved: {code} strategy={ctx['strategy'][:30]}")

    # ──────── Position Monitoring (called from main loop) ────────

    def check_positions(self, positions: List[dict]) -> List[dict]:
        """风控官扫描全部持仓，返回退出信号。"""
        return self.risk.scan_all_positions(positions)

    # ──────── Update P&L state for all agents ────────

    def update_pnl(self, daily_pnl_pct: float):
        """更新全队盈亏状态（由main loop调用）。"""
        self.risk.update_pnl()
        self.leader.update_pnl(daily_pnl_pct)

        # Check protect mode
        if self.risk.is_protect_mode and not self.leader.is_protect_mode:
            self.leader.enter_protect_mode(daily_pnl_pct)

    # ──────── API: Team Status ────────

    def get_team_state(self) -> dict:
        """返回全队状态（用于前端TeamPanel）。"""
        decisions = self.store.recent(limit=20)
        # Parse JSON data field for frontend consumption
        for d in decisions:
            if isinstance(d.get("data"), str) and d["data"]:
                try:
                    d["data"] = json.loads(d["data"])
                except (json.JSONDecodeError, TypeError):
                    pass
        return {
            "agents": {role: agent.get_state() for role, agent in self._agents.items()},
            "recent_decisions": decisions,
            "protect_mode": self.leader.is_protect_mode,
        }

    def get_agent_state(self, role: str) -> Optional[dict]:
        agent = self._agents.get(role)
        if agent:
            return agent.get_state()
        return None

    def get_decisions(self, limit: int = 50, agent: str = None) -> List[dict]:
        return self.store.recent(limit=limit, agent=agent)

    def get_today_decisions(self) -> List[dict]:
        return self.store.today()
