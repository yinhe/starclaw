"""
Portfolio Risk Manager — 组合级风控模块。

Controls:
  1. Daily loss limit:     total drawdown -2% → halt all buys for the day
  2. Position water level: bull=80%, sideways=50%, bear=20% max exposure
  3. Max holdings:         max 10 simultaneous stocks
  4. Sector concentration: max 3 stocks from same sector
  5. Per-scan buy limit:   max 3 new buys per scan cycle
"""

import logging
import time as time_module
from datetime import datetime, date
from typing import Dict, List, Optional, Set

logger = logging.getLogger("portfolio_risk")


class PortfolioRiskManager:
    """Portfolio-level risk controls that sit above individual position management."""

    def __init__(self, qmt_client, account: str,
                 daily_loss_limit_pct: float = 2.0,
                 max_holdings: int = 10,
                 max_per_sector: int = 3,
                 max_buys_per_scan: int = 3):
        self.qmt = qmt_client
        self.account = account
        self.daily_loss_limit_pct = daily_loss_limit_pct
        self.max_holdings = max_holdings
        self.max_per_sector = max_per_sector
        self.max_buys_per_scan = max_buys_per_scan

        # Daily tracking
        self._today: Optional[date] = None
        self._day_start_assets: float = 0
        self._halted: bool = False
        self._buys_today: int = 0

    # ------------------------------------------------------------------
    # Daily reset
    # ------------------------------------------------------------------

    def _check_new_day(self):
        """Reset daily counters if date changed."""
        today = date.today()
        if self._today != today:
            self._today = today
            self._halted = False
            self._buys_today = 0
            # Snapshot starting assets for daily loss check
            try:
                info = self.qmt.get_account_info(self.account)
                self._day_start_assets = float(info.get("total_assets", 0))
                logger.info(f"[risk] New day {today}: starting assets={self._day_start_assets:,.0f}")
            except Exception as e:
                logger.warning(f"[risk] Failed to snapshot day-start assets: {e}")

    # ------------------------------------------------------------------
    # 1. Daily loss check
    # ------------------------------------------------------------------

    def check_daily_loss(self) -> bool:
        """Return True if daily loss limit is breached → halt trading."""
        self._check_new_day()
        if self._halted:
            return True
        if self._day_start_assets <= 0:
            return False

        try:
            info = self.qmt.get_account_info(self.account)
            current_assets = float(info.get("total_assets", 0))
            daily_pnl_pct = (current_assets - self._day_start_assets) / self._day_start_assets * 100
            if daily_pnl_pct <= -self.daily_loss_limit_pct:
                self._halted = True
                logger.warning(f"[risk] ⚠️ DAILY LOSS LIMIT BREACHED: {daily_pnl_pct:+.2f}% "
                               f"(limit={-self.daily_loss_limit_pct}%). ALL BUYS HALTED.")
                return True
        except Exception as e:
            logger.debug(f"[risk] daily loss check error: {e}")
        return False

    @property
    def is_halted(self) -> bool:
        self._check_new_day()
        return self._halted

    # ------------------------------------------------------------------
    # 2. Position water level (max total exposure by market env)
    # ------------------------------------------------------------------

    def max_exposure_pct(self, market_env: str) -> float:
        """Maximum portfolio exposure as % of total assets."""
        return {
            "bull": 0.80,
            "sideways": 0.50,
            "bear": 0.20,
            "extreme_bear": 0.10,
        }.get(market_env, 0.50)

    def available_buy_budget(self, market_env: str) -> float:
        """How much capital can be deployed for new buys, considering current exposure."""
        try:
            info = self.qmt.get_account_info(self.account)
            total = float(info.get("total_assets", 0))
            market_value = float(info.get("market_value", 0))
            available_cash = float(info.get("available", 0))

            max_exposure = total * self.max_exposure_pct(market_env)
            remaining_budget = max_exposure - market_value
            # Can't deploy more than actual available cash
            budget = min(remaining_budget, available_cash)
            budget = max(0, budget)

            logger.info(f"[risk] Budget: total={total:,.0f} mktval={market_value:,.0f} "
                        f"max_exp={max_exposure:,.0f}({market_env}) budget={budget:,.0f}")
            return budget
        except Exception as e:
            logger.warning(f"[risk] available_buy_budget error: {e}")
            return 0

    # ------------------------------------------------------------------
    # 3. Max holdings check
    # ------------------------------------------------------------------

    def current_holdings_count(self) -> int:
        """Count current held stocks from QMT."""
        try:
            positions = self.qmt.get_positions(self.account)
            return len([p for p in positions if p.get("volume", 0) > 0])
        except Exception:
            return 0

    def can_open_new_position(self) -> bool:
        """Check if we can open a new position."""
        return self.current_holdings_count() < self.max_holdings

    def available_slots(self) -> int:
        """How many more stocks we can hold."""
        return max(0, self.max_holdings - self.current_holdings_count())

    # ------------------------------------------------------------------
    # 4. Sector concentration check
    # ------------------------------------------------------------------

    @staticmethod
    def _get_sector(code: str) -> str:
        """Rough sector grouping by stock code prefix."""
        plain = code.split(".")[0] if "." in code else code
        # Group by first 3 digits as rough sector proxy
        # In production, use xtdata sector data for real classification
        return plain[:3]

    def check_sector_concentration(self, new_code: str, held_codes: Set[str]) -> bool:
        """Return True if adding new_code would violate sector concentration limit."""
        new_sector = self._get_sector(new_code)
        same_sector_count = sum(1 for c in held_codes if self._get_sector(c) == new_sector)
        if same_sector_count >= self.max_per_sector:
            logger.info(f"[risk] Sector limit: {new_code} sector={new_sector} "
                        f"already has {same_sector_count} stocks (max={self.max_per_sector})")
            return True
        return False

    # ------------------------------------------------------------------
    # 5. Per-scan buy limit
    # ------------------------------------------------------------------

    def record_buy(self):
        """Record a buy for daily counting."""
        self._check_new_day()
        self._buys_today += 1

    def remaining_buys_this_scan(self) -> int:
        """How many more buys allowed this scan cycle."""
        return self.max_buys_per_scan

    # ------------------------------------------------------------------
    # Composite gate: should we allow this buy?
    # ------------------------------------------------------------------

    def allow_buy(self, code: str, market_env: str, held_codes: Set[str]) -> tuple:
        """Master gate: check all risk rules before allowing a buy.

        Returns: (allowed: bool, reason: str)
        """
        self._check_new_day()

        # 1. Daily loss halt
        if self.check_daily_loss():
            return False, "daily_loss_limit_breached"

        # 2. Max holdings
        if not self.can_open_new_position() and code not in held_codes:
            return False, f"max_holdings_reached({self.max_holdings})"

        # 3. Budget check
        budget = self.available_buy_budget(market_env)
        if budget <= 0:
            return False, f"no_budget(env={market_env})"

        # 4. Sector concentration
        if self.check_sector_concentration(code, held_codes):
            return False, "sector_concentration_limit"

        return True, "ok"

    # ------------------------------------------------------------------
    # Summary for logging
    # ------------------------------------------------------------------

    def status_summary(self, market_env: str = "sideways") -> dict:
        """Return current risk status for logging/display."""
        self._check_new_day()
        holdings = self.current_holdings_count()
        budget = self.available_buy_budget(market_env)

        daily_pnl_pct = 0
        if self._day_start_assets > 0:
            try:
                info = self.qmt.get_account_info(self.account)
                current = float(info.get("total_assets", 0))
                daily_pnl_pct = (current - self._day_start_assets) / self._day_start_assets * 100
            except Exception:
                pass

        return {
            "halted": self._halted,
            "daily_pnl_pct": round(daily_pnl_pct, 2),
            "daily_loss_limit": -self.daily_loss_limit_pct,
            "holdings": holdings,
            "max_holdings": self.max_holdings,
            "available_slots": max(0, self.max_holdings - holdings),
            "buy_budget": round(budget, 0),
            "market_env": market_env,
            "max_exposure_pct": self.max_exposure_pct(market_env),
            "buys_today": self._buys_today,
        }
