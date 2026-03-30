"""
Position Manager — tracks all open positions, persists to JSON file,
provides stop-loss / take-profit / time-stop monitoring.

Lifecycle:
  1. record_buy()    — called after successful order submission
  2. check_exits()   — called every minute during trading hours
  3. record_sell()   — called after exit order submission
  4. get_held_codes() — used by scanner to avoid duplicate buys
"""

import json
import logging
import os
import time as time_module
from datetime import datetime
from typing import Dict, List, Optional

logger = logging.getLogger("position_manager")

POSITIONS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "positions.json")


class Position:
    """A single open position."""

    def __init__(self, code: str, account: str, entry_price: float, volume: int,
                 score: float = 0.0, order_id: str = "", reason: str = "",
                 entry_time: str = "", highest_price: float = 0.0):
        self.code = code
        self.account = account
        self.entry_price = entry_price
        self.volume = volume
        self.score = score
        self.order_id = order_id
        self.reason = reason
        self.entry_time = entry_time or datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        self.highest_price = highest_price or entry_price

    def to_dict(self) -> dict:
        return {
            "code": self.code,
            "account": self.account,
            "entry_price": self.entry_price,
            "volume": self.volume,
            "score": self.score,
            "order_id": self.order_id,
            "reason": self.reason,
            "entry_time": self.entry_time,
            "highest_price": self.highest_price,
        }

    @classmethod
    def from_dict(cls, d: dict) -> "Position":
        return cls(**d)


class PositionManager:
    """
    Manages all open positions with persistence and exit monitoring.

    Risk parameters:
        stop_loss_pct:      fixed stop-loss percentage (default -5%)
        trailing_stop_pct:  trailing stop from highest price (default -8%)
        time_stop_days:     sell if held N days with no profit (default 5)
        take_profit_pct:    sell if profit exceeds this (default +15%)
    """

    def __init__(self, qmt_client, account: str = "27800348",
                 stop_loss_pct: float = 5.0,
                 trailing_stop_pct: float = 8.0,
                 time_stop_days: int = 5,
                 take_profit_pct: float = 15.0):
        self.qmt = qmt_client
        self.account = account
        self.stop_loss_pct = stop_loss_pct
        self.trailing_stop_pct = trailing_stop_pct
        self.time_stop_days = time_stop_days
        self.take_profit_pct = take_profit_pct
        self.positions: Dict[str, Position] = {}
        self._load()

    def _load(self):
        """Load positions from disk."""
        if os.path.exists(POSITIONS_FILE):
            try:
                with open(POSITIONS_FILE, "r", encoding="utf-8") as f:
                    data = json.load(f)
                for d in data:
                    pos = Position.from_dict(d)
                    self.positions[pos.code] = pos
                logger.info(f"Loaded {len(self.positions)} positions from {POSITIONS_FILE}")
            except Exception as e:
                logger.error(f"Failed to load positions: {e}")

    def _save(self):
        """Persist positions to disk."""
        try:
            data = [p.to_dict() for p in self.positions.values()]
            with open(POSITIONS_FILE, "w", encoding="utf-8") as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
        except Exception as e:
            logger.error(f"Failed to save positions: {e}")

    def record_buy(self, code: str, price: float, volume: int,
                   score: float = 0.0, order_id: str = "", reason: str = ""):
        """Record a new buy. Called after order submission."""
        if code in self.positions:
            # Add to existing position (average up)
            pos = self.positions[code]
            total_cost = pos.entry_price * pos.volume + price * volume
            pos.volume += volume
            pos.entry_price = total_cost / pos.volume
            if price > pos.highest_price:
                pos.highest_price = price
            logger.info(f"[position] ADDED {code}: +{volume} @{price:.2f}, total={pos.volume}")
        else:
            pos = Position(
                code=code, account=self.account,
                entry_price=price, volume=volume,
                score=score, order_id=order_id, reason=reason,
            )
            self.positions[code] = pos
            logger.info(f"[position] NEW {code}: {volume} @{price:.2f} score={score:.2f}")
        self._save()

    def record_sell(self, code: str, sell_price: float, sell_volume: int, sell_reason: str = ""):
        """Record a sell (partial or full). Called after exit order."""
        if code not in self.positions:
            logger.warning(f"[position] sell {code} but not in positions")
            return
        pos = self.positions[code]
        pnl_pct = (sell_price - pos.entry_price) / pos.entry_price * 100
        logger.info(f"[position] SELL {code}: {sell_volume} @{sell_price:.2f} "
                     f"entry={pos.entry_price:.2f} pnl={pnl_pct:+.2f}% reason={sell_reason}")
        pos.volume -= sell_volume
        if pos.volume <= 0:
            del self.positions[code]
        self._save()

    def get_held_codes(self) -> set:
        """Return set of currently held stock codes."""
        return set(self.positions.keys())

    def get_positions_list(self) -> List[dict]:
        """Return all positions as list of dicts (for API/display)."""
        return [p.to_dict() for p in self.positions.values()]

    def check_exits(self) -> List[dict]:
        """
        Check all positions for exit conditions. Returns list of exit signals.

        Called every minute during trading hours by the monitor loop.

        Exit conditions (in priority order):
          1. Fixed stop-loss: current_price <= entry_price * (1 - stop_loss_pct/100)
          2. Trailing stop: current_price <= highest_price * (1 - trailing_stop_pct/100)
          3. Take profit: current_price >= entry_price * (1 + take_profit_pct/100)
          4. Time stop: held > N days and pnl <= 0
        """
        if not self.positions:
            return []

        codes = list(self.positions.keys())
        exits = []

        # Get current prices
        try:
            from qmt_client import HAS_XTQUANT
            if HAS_XTQUANT:
                from xtquant import xtdata
                # Batch get latest prices
                md = xtdata.get_market_data_ex([], codes, period="1d", count=1)
                prices = {}
                for code in codes:
                    if code in md:
                        df = md[code]
                        try:
                            prices[code] = float(df["close"].iloc[-1])
                        except Exception:
                            pass
            else:
                prices = {}
        except Exception as e:
            logger.error(f"[position] check_exits price fetch error: {e}")
            return []

        now = datetime.now()
        changed = False

        for code in codes:
            if code not in self.positions:
                continue
            pos = self.positions[code]
            cur_price = prices.get(code, 0)
            if cur_price <= 0:
                continue

            # Update highest price
            if cur_price > pos.highest_price:
                pos.highest_price = cur_price
                changed = True

            pnl_pct = (cur_price - pos.entry_price) / pos.entry_price * 100
            drawdown_pct = (pos.highest_price - cur_price) / pos.highest_price * 100 if pos.highest_price > 0 else 0

            # Calculate days held
            try:
                entry_dt = datetime.strptime(pos.entry_time, "%Y-%m-%d %H:%M:%S")
                days_held = (now - entry_dt).days
            except Exception:
                days_held = 0

            exit_reason = None

            # 1. Fixed stop-loss
            if pnl_pct <= -self.stop_loss_pct:
                exit_reason = f"stop_loss({pnl_pct:+.1f}%)"

            # 2. Trailing stop (only if we've been profitable)
            elif pos.highest_price > pos.entry_price * 1.02 and drawdown_pct >= self.trailing_stop_pct:
                exit_reason = f"trailing_stop(high={pos.highest_price:.2f}, drop={drawdown_pct:.1f}%)"

            # 3. Take profit
            elif pnl_pct >= self.take_profit_pct:
                exit_reason = f"take_profit({pnl_pct:+.1f}%)"

            # 4. Time stop
            elif days_held >= self.time_stop_days and pnl_pct <= 0:
                exit_reason = f"time_stop({days_held}d, pnl={pnl_pct:+.1f}%)"

            if exit_reason:
                exits.append({
                    "code": code,
                    "account": pos.account,
                    "volume": pos.volume,
                    "entry_price": pos.entry_price,
                    "current_price": cur_price,
                    "pnl_pct": pnl_pct,
                    "reason": exit_reason,
                })

        if changed:
            self._save()

        return exits

    def execute_exits(self, exits: List[dict]) -> List[dict]:
        """Execute sell orders for all exit signals."""
        results = []
        for ex in exits:
            code = ex["code"]
            volume = ex["volume"]
            price = ex["current_price"]
            reason = ex["reason"]

            logger.info(f"[position] EXIT {code}: SELL {volume} @{price:.2f} reason={reason}")

            try:
                order_id = self.qmt.submit_order(
                    account=ex["account"],
                    code=code,
                    direction="sell",
                    price=price,
                    volume=volume,
                    order_type="limit",
                )
                self.record_sell(code, price, volume, reason)
                results.append({
                    "code": code,
                    "price": price,
                    "volume": volume,
                    "order_id": order_id,
                    "reason": reason,
                    "pnl_pct": ex["pnl_pct"],
                })
            except Exception as e:
                logger.error(f"[position] EXIT {code} failed: {e}")

        return results

    def summary(self) -> dict:
        """Return position summary for display."""
        total_value = 0
        total_cost = 0
        for pos in self.positions.values():
            total_cost += pos.entry_price * pos.volume
        return {
            "count": len(self.positions),
            "total_cost": total_cost,
            "codes": list(self.positions.keys()),
        }
