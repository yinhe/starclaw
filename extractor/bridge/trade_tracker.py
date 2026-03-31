"""
Trade Tracker — 记录每笔交易的完整生命周期，计算策略归因指标。

Persists to trades.json. Provides:
  - Win rate, profit factor, avg win/loss
  - Per-reason attribution (which exit reasons perform best)
  - Per-signal-strength attribution (which strength levels are profitable)
  - Holding period analysis
"""

import json
import logging
import os
from datetime import datetime
from typing import Dict, List, Optional

logger = logging.getLogger("trade_tracker")

TRADES_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "trades.json")


class Trade:
    """A completed (closed) trade."""

    def __init__(self, code: str, direction: str,
                 entry_price: float, exit_price: float,
                 volume: int, entry_time: str, exit_time: str,
                 entry_reason: str = "", exit_reason: str = "",
                 signal_strength: int = 0, score: float = 0,
                 pnl_pct: float = 0, pnl_amount: float = 0,
                 holding_days: int = 0):
        self.code = code
        self.direction = direction
        self.entry_price = entry_price
        self.exit_price = exit_price
        self.volume = volume
        self.entry_time = entry_time
        self.exit_time = exit_time
        self.entry_reason = entry_reason
        self.exit_reason = exit_reason
        self.signal_strength = signal_strength
        self.score = score
        self.pnl_pct = pnl_pct
        self.pnl_amount = pnl_amount
        self.holding_days = holding_days

    def to_dict(self) -> dict:
        return self.__dict__

    @classmethod
    def from_dict(cls, d: dict) -> "Trade":
        return cls(**{k: v for k, v in d.items() if k in cls.__init__.__code__.co_varnames})


class TradeTracker:
    """Tracks and analyzes completed trades."""

    def __init__(self):
        self.trades: List[Trade] = []
        self._load()

    def _load(self):
        if os.path.exists(TRADES_FILE):
            try:
                with open(TRADES_FILE, "r", encoding="utf-8") as f:
                    data = json.load(f)
                self.trades = [Trade.from_dict(d) for d in data]
                logger.info(f"Loaded {len(self.trades)} historical trades")
            except Exception as e:
                logger.warning(f"Failed to load trades: {e}")

    def _save(self):
        try:
            data = [t.to_dict() for t in self.trades]
            with open(TRADES_FILE, "w", encoding="utf-8") as f:
                json.dump(data, ensure_ascii=False, indent=2, fp=f)
        except Exception as e:
            logger.error(f"Failed to save trades: {e}")

    def record_trade(self, code: str, entry_price: float, exit_price: float,
                     volume: int, entry_time: str, exit_time: str = "",
                     entry_reason: str = "", exit_reason: str = "",
                     signal_strength: int = 0, score: float = 0):
        """Record a completed trade."""
        if not exit_time:
            exit_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        pnl_pct = (exit_price - entry_price) / entry_price * 100 if entry_price > 0 else 0
        pnl_amount = (exit_price - entry_price) * volume

        try:
            entry_dt = datetime.strptime(entry_time, "%Y-%m-%d %H:%M:%S")
            exit_dt = datetime.strptime(exit_time, "%Y-%m-%d %H:%M:%S")
            holding_days = (exit_dt - entry_dt).days
        except Exception:
            holding_days = 0

        trade = Trade(
            code=code, direction="long",
            entry_price=entry_price, exit_price=exit_price,
            volume=volume, entry_time=entry_time, exit_time=exit_time,
            entry_reason=entry_reason, exit_reason=exit_reason,
            signal_strength=signal_strength, score=score,
            pnl_pct=round(pnl_pct, 2), pnl_amount=round(pnl_amount, 2),
            holding_days=holding_days,
        )
        self.trades.append(trade)
        self._save()
        logger.info(f"[tracker] Recorded: {code} {pnl_pct:+.2f}% "
                     f"({entry_price:.2f}→{exit_price:.2f}) {exit_reason}")

    # ------------------------------------------------------------------
    # Analytics
    # ------------------------------------------------------------------

    def stats(self, last_n: int = 0) -> dict:
        """Compute overall strategy statistics.

        Args:
            last_n: if >0, only use last N trades
        """
        trades = self.trades[-last_n:] if last_n > 0 else self.trades
        if not trades:
            return {"total_trades": 0, "message": "no trades recorded"}

        wins = [t for t in trades if t.pnl_pct > 0]
        losses = [t for t in trades if t.pnl_pct <= 0]

        total = len(trades)
        win_count = len(wins)
        loss_count = len(losses)
        win_rate = win_count / total if total > 0 else 0

        avg_win = sum(t.pnl_pct for t in wins) / win_count if win_count > 0 else 0
        avg_loss = sum(t.pnl_pct for t in losses) / loss_count if loss_count > 0 else 0
        profit_factor = abs(avg_win * win_count / (avg_loss * loss_count)) if loss_count > 0 and avg_loss != 0 else 999

        total_pnl = sum(t.pnl_amount for t in trades)
        avg_holding = sum(t.holding_days for t in trades) / total if total > 0 else 0

        # Expectancy: avg_win * win_rate + avg_loss * (1 - win_rate)
        expectancy = avg_win * win_rate + avg_loss * (1 - win_rate)

        # Max consecutive losses
        max_consec_loss = 0
        cur_consec = 0
        for t in trades:
            if t.pnl_pct <= 0:
                cur_consec += 1
                max_consec_loss = max(max_consec_loss, cur_consec)
            else:
                cur_consec = 0

        return {
            "total_trades": total,
            "wins": win_count,
            "losses": loss_count,
            "win_rate": round(win_rate, 3),
            "avg_win_pct": round(avg_win, 2),
            "avg_loss_pct": round(avg_loss, 2),
            "profit_factor": round(profit_factor, 2),
            "expectancy_pct": round(expectancy, 2),
            "total_pnl": round(total_pnl, 2),
            "avg_holding_days": round(avg_holding, 1),
            "max_consecutive_losses": max_consec_loss,
        }

    def attribution_by_exit_reason(self) -> List[dict]:
        """Analyze performance grouped by exit reason."""
        groups: Dict[str, list] = {}
        for t in self.trades:
            # Normalize reason to category
            reason = t.exit_reason.split("(")[0] if t.exit_reason else "unknown"
            groups.setdefault(reason, []).append(t)

        result = []
        for reason, trades in sorted(groups.items()):
            wins = sum(1 for t in trades if t.pnl_pct > 0)
            avg_pnl = sum(t.pnl_pct for t in trades) / len(trades)
            result.append({
                "exit_reason": reason,
                "count": len(trades),
                "win_rate": round(wins / len(trades), 3),
                "avg_pnl_pct": round(avg_pnl, 2),
            })
        return sorted(result, key=lambda x: x["avg_pnl_pct"], reverse=True)

    def attribution_by_signal_strength(self) -> List[dict]:
        """Analyze performance grouped by signal strength (0-6)."""
        groups: Dict[int, list] = {}
        for t in self.trades:
            groups.setdefault(t.signal_strength, []).append(t)

        result = []
        for strength, trades in sorted(groups.items()):
            wins = sum(1 for t in trades if t.pnl_pct > 0)
            avg_pnl = sum(t.pnl_pct for t in trades) / len(trades)
            result.append({
                "signal_strength": strength,
                "count": len(trades),
                "win_rate": round(wins / len(trades), 3),
                "avg_pnl_pct": round(avg_pnl, 2),
            })
        return result

    def recent_trades(self, limit: int = 20) -> List[dict]:
        """Return most recent trades."""
        return [t.to_dict() for t in reversed(self.trades[-limit:])]
