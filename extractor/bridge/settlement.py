"""
Daily Settlement — 日终结算 + Queen 星能注入。

每交易日 15:30 后自动运行:
  1. 计算当日已实现盈亏
  2. 按比例分配: 60%再投资 + 20%星能池 + 10%分红 + 10%储备
  3. 星能注入 Queen (POST /internal/credits/inject)
  4. 记录结算历史
"""

import json
import logging
import os
from datetime import datetime, date
from typing import Dict, Optional

import httpx
import yaml

logger = logging.getLogger("settlement")

SETTLEMENT_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "settlement_history.json")

# Load Queen config from config.yaml (falls back to env vars)
def _load_queen_config():
    cfg_path = os.path.join(os.path.dirname(__file__), "config.yaml")
    try:
        with open(cfg_path, "r", encoding="utf-8") as f:
            cfg = yaml.safe_load(f)
        q = cfg.get("queen", {})
        return q.get("url", "") or os.getenv("QUEEN_URL", "https://api.starclaw.net"), \
               q.get("token", "") or os.getenv("QUEEN_TOKEN", "")
    except Exception:
        return os.getenv("QUEEN_URL", "https://api.starclaw.net"), os.getenv("QUEEN_TOKEN", "")

QUEEN_URL, QUEEN_TOKEN = _load_queen_config()

# Distribution ratios
REINVEST_RATIO = 0.60      # 60% stays in account
STAR_ENERGY_RATIO = 0.20   # 20% → Queen star energy
DIVIDEND_RATIO = 0.10      # 10% investor dividend
RESERVE_RATIO = 0.10       # 10% operations reserve

# Conversion: ¥1 = 100 star energy
CNY_TO_STAR_ENERGY = 100


class DailySettlement:
    """Handles end-of-day settlement and star energy distribution."""

    def __init__(self, qmt_client, account: str):
        self.qmt = qmt_client
        self.account = account
        self.history: list = []
        self._load()

    def _load(self):
        if os.path.exists(SETTLEMENT_FILE):
            try:
                with open(SETTLEMENT_FILE, "r", encoding="utf-8") as f:
                    self.history = json.load(f)
            except Exception:
                pass

    def _save(self):
        try:
            with open(SETTLEMENT_FILE, "w", encoding="utf-8") as f:
                json.dump(self.history, ensure_ascii=False, indent=2, fp=f)
        except Exception as e:
            logger.error(f"Failed to save settlement: {e}")

    def run(self, trades_today: list = None) -> dict:
        """Run daily settlement.

        Args:
            trades_today: list of completed trades with pnl_amount
        """
        today = date.today().isoformat()

        # Check if already settled today
        if any(h.get("date") == today for h in self.history):
            return {"status": "already_settled", "date": today}

        # Calculate realized P&L from today's trades
        realized_pnl = 0
        if trades_today:
            realized_pnl = sum(t.get("pnl_amount", 0) for t in trades_today)

        # Get account info for context
        acct_info = {}
        try:
            acct_info = self.qmt.get_account_info(self.account)
        except Exception:
            pass

        # Distribution
        if realized_pnl > 0:
            star_energy_cny = realized_pnl * STAR_ENERGY_RATIO
            star_energy_units = int(star_energy_cny * CNY_TO_STAR_ENERGY)
            dividend_cny = realized_pnl * DIVIDEND_RATIO
            reserve_cny = realized_pnl * RESERVE_RATIO
            reinvest_cny = realized_pnl * REINVEST_RATIO
        else:
            # Loss day: no distribution, covered by reserve
            star_energy_cny = 0
            star_energy_units = 0
            dividend_cny = 0
            reserve_cny = 0
            reinvest_cny = 0

        # Inject star energy to Queen
        queen_result = None
        if star_energy_units > 0 and QUEEN_TOKEN:
            try:
                resp = httpx.post(
                    f"{QUEEN_URL}/internal/credits/inject",
                    json={
                        "source": "extractor",
                        "amount": star_energy_units,
                        "reason": f"trading_profit_{today}",
                    },
                    headers={"Authorization": f"Bearer {QUEEN_TOKEN}"},
                    timeout=30,
                )
                queen_result = {"status": resp.status_code, "injected": star_energy_units}
                logger.info(f"[settlement] Queen inject: {star_energy_units} star energy → {resp.status_code}")
            except Exception as e:
                queen_result = {"status": "error", "error": str(e)}
                logger.warning(f"[settlement] Queen inject failed: {e}")

        record = {
            "date": today,
            "realized_pnl": round(realized_pnl, 2),
            "trades_count": len(trades_today) if trades_today else 0,
            "distribution": {
                "reinvest": round(reinvest_cny, 2),
                "star_energy_cny": round(star_energy_cny, 2),
                "star_energy_units": star_energy_units,
                "dividend": round(dividend_cny, 2),
                "reserve": round(reserve_cny, 2),
            },
            "queen_inject": queen_result,
            "account_snapshot": {
                "total_assets": acct_info.get("total_assets", 0),
                "available": acct_info.get("available", 0),
                "market_value": acct_info.get("market_value", 0),
            },
            "settled_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        }

        self.history.append(record)
        self._save()

        logger.info(f"[settlement] {today}: PnL={realized_pnl:+.2f}, "
                     f"星能={star_energy_units}, 分红={dividend_cny:.2f}")
        return record

    def get_history(self, limit: int = 30) -> list:
        """Return recent settlement history."""
        return list(reversed(self.history[-limit:]))

    def get_cumulative(self) -> dict:
        """Return cumulative settlement stats."""
        total_pnl = sum(h.get("realized_pnl", 0) for h in self.history)
        total_star = sum(h.get("distribution", {}).get("star_energy_units", 0) for h in self.history)
        total_dividend = sum(h.get("distribution", {}).get("dividend", 0) for h in self.history)
        profitable_days = sum(1 for h in self.history if h.get("realized_pnl", 0) > 0)
        loss_days = sum(1 for h in self.history if h.get("realized_pnl", 0) < 0)

        return {
            "total_days": len(self.history),
            "profitable_days": profitable_days,
            "loss_days": loss_days,
            "total_pnl": round(total_pnl, 2),
            "total_star_energy": total_star,
            "total_dividend": round(total_dividend, 2),
        }
