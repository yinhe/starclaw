"""
Risk Officer Agent — 独立风控智能体。

从 portfolio_risk.py 的规则引擎升级为完整Agent:
  - 基因: paranoid, veto_rate_cap=70%
  - 本能: 规则触发，零LLM成本
  - 腺体: cortisol主导，亏损自动收紧

核心职责:
  1. 审核猎手提交的候选（approve / veto / reduce_size）
  2. 实时监控持仓（止损/止盈/锁利）
  3. 日内盈亏管控（保本模式）
  4. 否决率不超过基因上限（防止过度保守）
"""

import logging
from datetime import date, datetime
from typing import Dict, List, Optional, Tuple

from team_core import (
    AgentConfig, AgentStatus, BaseAgent, Decision, DecisionStore,
    default_team_configs, AgentRole,
)

logger = logging.getLogger("risk_officer")


class RiskOfficer(BaseAgent):
    """独立风控Agent — 本能驱动，零LLM成本。"""

    def __init__(self, config: AgentConfig, decision_store: DecisionStore,
                 qmt_client=None, account: str = ""):
        super().__init__(config, decision_store)
        self.qmt = qmt_client
        self.account = account

        # Daily tracking
        self._today: Optional[date] = None
        self._day_start_assets: float = 0
        self._daily_pnl_pct: float = 0
        self._protect_mode: bool = False  # 保本模式
        self._veto_count: int = 0
        self._review_count: int = 0

        # Trailing stop: track high-water mark per position {code: highest_price}
        self._high_water: Dict[str, float] = {}
        self._load_high_water()

    def _load_high_water(self):
        """Load persisted high-water marks from memory."""
        if hasattr(self, 'memory') and self.memory:
            hw = self.memory.get("risk", "high_water_marks", {})
            if isinstance(hw, dict):
                self._high_water = hw
                if hw:
                    logger.info(f"[risk] Loaded {len(hw)} high-water marks from memory")

    def _save_high_water(self):
        """Persist high-water marks to memory."""
        if hasattr(self, 'memory') and self.memory:
            self.memory.set("risk", "high_water_marks", self._high_water)

    # ──────── Daily Reset ────────

    def _check_new_day(self):
        today = date.today()
        if self._today != today:
            self._today = today
            self._protect_mode = False
            self._veto_count = 0
            self._review_count = 0
            self._snapshot_assets()
            self.gland.cortisol = 0
            self.gland.dopamine = 50
            self.gland.state = "calm"
            logger.info(f"[risk] New day {today}: assets={self._day_start_assets:,.0f}")

    def _snapshot_assets(self):
        if not self.qmt or not self.account:
            return
        try:
            info = self.qmt.get_account_info(self.account)
            self._day_start_assets = float(info.get("total_assets", 0))
        except Exception as e:
            logger.warning(f"[risk] snapshot error: {e}")

    # ──────── Core: Update Daily P&L ────────

    def update_pnl(self) -> float:
        """更新当日盈亏百分比，同时更新腺体状态。"""
        self._check_new_day()
        if self._day_start_assets <= 0 or not self.qmt:
            return 0
        try:
            info = self.qmt.get_account_info(self.account)
            current = float(info.get("total_assets", 0))
            self._daily_pnl_pct = (current - self._day_start_assets) / self._day_start_assets * 100
            self.gland.update(self._daily_pnl_pct)
        except Exception:
            pass
        return self._daily_pnl_pct

    # ──────── Instinct 1: Check Daily Loss → Protect Mode ────────

    def check_protect_mode(self) -> bool:
        """本能: 日亏≥0.3% → 保本模式。"""
        self._check_new_day()
        pnl = self.update_pnl()

        threshold = -0.3 * self.gland.risk_modifier()  # 腺体调节阈值

        if pnl <= threshold and not self._protect_mode:
            self._protect_mode = True
            self.set_status(AgentStatus.ALERTING)
            self.log_decision(
                action="protect_mode_on",
                target="portfolio",
                confidence=95,
                reasoning=f"日亏{pnl:+.2f}%触及阈值{threshold:.2f}%，启动保本模式",
                data={"daily_pnl_pct": round(pnl, 2), "threshold": round(threshold, 2)},
            )
            logger.warning(f"[risk] ⚠️ PROTECT MODE ON: daily P&L={pnl:+.2f}%")
            return True

        return self._protect_mode

    @property
    def is_protect_mode(self) -> bool:
        self._check_new_day()
        return self._protect_mode

    # ──────── Instinct 2: Review Candidate (approve / veto / reduce) ────────

    def review_candidate(self, candidate: dict, market_env: str = "sideways",
                         held_codes: set = None) -> Tuple[str, str, float]:
        """
        审核猎手提交的候选股票。

        Returns: (action, reason, adjusted_confidence)
            action: 'approve' / 'veto' / 'reduce_size'
        """
        self._check_new_day()
        self.set_status(AgentStatus.ANALYZING)
        self._review_count += 1

        code = candidate.get("code", "")
        score = candidate.get("score", 0)
        reason_parts = []
        risk_score = 0  # 0-100, higher = riskier

        # ── Rule 1: Protect mode → veto all new buys ──
        if self._protect_mode:
            risk_score += 90
            reason_parts.append("保本模式中")

        # ── Rule 2: Market env penalty ──
        env_penalty = {"bull": 0, "sideways": 15, "bear": 40, "extreme_bear": 70}.get(market_env, 15)
        risk_score += env_penalty
        if env_penalty > 20:
            reason_parts.append(f"市场{market_env}(+{env_penalty})")

        # ── Rule 3: Score too low ──
        if score < 0.60:
            risk_score += 30
            reason_parts.append(f"评分偏低{score:.2f}")
        elif score < 0.65:
            risk_score += 15
            reason_parts.append(f"评分临界{score:.2f}")

        # ── Rule 4: Daily P&L already negative ──
        if self._daily_pnl_pct < -0.1:
            risk_score += int(abs(self._daily_pnl_pct) * 10)
            reason_parts.append(f"日内已亏{self._daily_pnl_pct:+.2f}%")

        # ── Rule 5: Gland adjustment ──
        gland_mod = self.gland.risk_modifier()
        if gland_mod < 0.8:
            risk_score += 10
            reason_parts.append(f"腺体紧张({self.gland.state})")

        # ── Rule 6: Already too many holdings ──
        if held_codes and len(held_codes) >= 8:
            risk_score += 20
            reason_parts.append(f"持仓已{len(held_codes)}只")

        # ── Gene constraint: veto rate cap ──
        current_veto_rate = self._veto_count / max(self._review_count, 1)
        at_veto_cap = current_veto_rate >= self.gene.veto_rate_cap and self._review_count >= 3

        # ── Decision ──
        confidence = min(100, risk_score + self.gene.confidence_bias * 10)
        reason = "; ".join(reason_parts) if reason_parts else "各项指标正常"

        if risk_score >= 70 and not at_veto_cap:
            action = "veto"
            self._veto_count += 1
        elif risk_score >= 45:
            if at_veto_cap:
                # 否决率到上限 → 降为reduce_size而非veto
                action = "reduce_size"
                reason += f" [否决率{current_veto_rate:.0%}已达上限{self.gene.veto_rate_cap:.0%}]"
            else:
                action = "reduce_size"
        else:
            if at_veto_cap and risk_score >= 30:
                action = "approve"
                reason += f" [否决率上限强制放行]"
            else:
                action = "approve"

        self.set_status(AgentStatus.APPROVING if action == "approve" else AgentStatus.VETOING)
        self.log_decision(
            action=action,
            target=code,
            confidence=confidence,
            reasoning=reason,
            data={
                "risk_score": risk_score,
                "candidate_score": score,
                "market_env": market_env,
                "gland_modifier": round(gland_mod, 2),
                "veto_rate": round(current_veto_rate, 2),
                "protect_mode": self._protect_mode,
            },
        )

        self.set_status(AgentStatus.IDLE)
        return action, reason, confidence

    # ──────── Instinct 3: Position Monitor (stop-loss / take-profit) ────────

    def check_position_risk(self, position: dict) -> Optional[dict]:
        """
        检查单个持仓的风险。

        Returns: None if OK, or dict with exit signal:
            {"code": "...", "action": "stop_loss"/"take_profit"/"trailing_stop", "reason": "..."}
        """
        code = position.get("code", "")
        cost = position.get("cost_price", 0)
        price = position.get("market_price", 0)
        volume = position.get("volume", 0)
        avail = position.get("avail_volume", 0)

        if cost <= 0 or price <= 0 or volume <= 0:
            return None
        if avail <= 0:
            return None  # T+1 locked, can't sell

        pnl_pct = (price - cost) / cost * 100

        # Load buy context from memory (Phase 10)
        pos_ctx = {}
        if hasattr(self, 'memory') and self.memory:
            pos_ctx = self.memory.get("risk", f"pos_ctx_{code}", {})
            if not isinstance(pos_ctx, dict):
                pos_ctx = {}

        # Update high-water mark for trailing stop
        prev_high = self._high_water.get(code, price)
        if price > prev_high:
            self._high_water[code] = price
            self._save_high_water()
        high_water = self._high_water.get(code, price)

        # Dynamic profit targets from alpha_engine (Phase 11)
        gmod = self.gland.risk_modifier()
        try:
            import alpha_engine
            env = pos_ctx.get("market_env", "sideways")
            targets = alpha_engine.dynamic_profit_targets(code, market_env=env)
            stop_loss_thr = -targets.get("hard_stop", 5.0) * gmod
            take_profit_thr = targets.get("tp2", 15.0) / gmod
        except Exception:
            stop_loss_thr = -5.0 * gmod
            take_profit_thr = 15.0 / gmod

        # Trailing stop: if price gained >5% from cost, trigger if dropped >3% from high
        if high_water > cost * 1.05 and price < high_water * (1 - 0.03 * gmod):
            drawdown = (price - high_water) / high_water * 100
            gain_from_cost = (high_water - cost) / cost * 100
            reason = f"移动止损: 最高{high_water:.2f}回撤{drawdown:+.1f}%, 原盈{gain_from_cost:+.1f}%"
            self.set_status(AgentStatus.EXECUTING)
            self.log_decision("trailing_stop", code, 90, reason,
                              {"high_water": round(high_water, 2), "drawdown_pct": round(drawdown, 2),
                               "pnl_pct": round(pnl_pct, 2), **pos_ctx})
            return {"code": code, "action": "trailing_stop", "reason": reason, "volume": avail,
                    "strategy": pos_ctx.get("strategy", ""), "strategy_type": pos_ctx.get("strategy_type", ""),
                    "sector": pos_ctx.get("sector", "")}

        # Stop-loss
        if pnl_pct <= stop_loss_thr:
            reason = f"亏损{pnl_pct:+.1f}%触及止损线{stop_loss_thr:.1f}%"
            self.set_status(AgentStatus.EXECUTING)
            self.log_decision("stop_loss", code, 95, reason,
                              {"pnl_pct": round(pnl_pct, 2), "threshold": round(stop_loss_thr, 2), **pos_ctx})
            return {"code": code, "action": "stop_loss", "reason": reason, "volume": avail,
                    "strategy": pos_ctx.get("strategy", ""), "strategy_type": pos_ctx.get("strategy_type", ""),
                    "sector": pos_ctx.get("sector", "")}

        # Take-profit (only if sufficiently high)
        if pnl_pct >= take_profit_thr:
            reason = f"盈利{pnl_pct:+.1f}%触及止盈线{take_profit_thr:.1f}%"
            self.set_status(AgentStatus.EXECUTING)
            self.log_decision("take_profit", code, 85, reason,
                              {"pnl_pct": round(pnl_pct, 2), "threshold": round(take_profit_thr, 2), **pos_ctx})
            return {"code": code, "action": "take_profit", "reason": reason, "volume": avail,
                    "strategy": pos_ctx.get("strategy", ""), "strategy_type": pos_ctx.get("strategy_type", ""),
                    "sector": pos_ctx.get("sector", "")}

        # Protect mode: any profitable position → lock partial profit
        if self._protect_mode and pnl_pct > 0.5 and avail >= 200:
            sell_vol = int(avail * 0.5 / 100) * 100
            if sell_vol >= 100:
                reason = f"保本模式:锁利{pnl_pct:+.1f}%,卖出{sell_vol}股"
                self.log_decision("protect_lock", code, 80, reason, {"pnl_pct": round(pnl_pct, 2), **pos_ctx})
                return {"code": code, "action": "protect_lock", "reason": reason, "volume": sell_vol,
                        "strategy": pos_ctx.get("strategy", ""), "strategy_type": pos_ctx.get("strategy_type", ""),
                        "sector": pos_ctx.get("sector", "")}

        return None

    def scan_all_positions(self, positions: List[dict]) -> List[dict]:
        """扫描全部持仓，返回需要执行的退出信号列表。"""
        self.set_status(AgentStatus.SCANNING)
        signals = []
        for p in positions:
            sig = self.check_position_risk(p)
            if sig:
                signals.append(sig)
        self.set_status(AgentStatus.IDLE)
        return signals

    @property
    def is_protect_mode(self) -> bool:
        return self._protect_mode

    # ──────── Daily halt check (backward compat with portfolio_risk) ────────

    def check_daily_halt(self) -> bool:
        """日亏≥2% → 全面停止交易。"""
        self._check_new_day()
        pnl = self.update_pnl()
        return pnl <= -2.0

    # ──────── Status for frontend ────────

    def get_state(self) -> dict:
        base = super().get_state()
        base.update({
            "daily_pnl_pct": round(self._daily_pnl_pct, 2),
            "protect_mode": self._protect_mode,
            "veto_count": self._veto_count,
            "review_count": self._review_count,
            "veto_rate": round(self._veto_count / max(self._review_count, 1), 2),
            "day_start_assets": round(self._day_start_assets, 0),
        })
        return base
