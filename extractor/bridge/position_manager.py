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

try:
    import alpha_engine
except ImportError:
    alpha_engine = None

logger = logging.getLogger("position_manager")

POSITIONS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "positions.json")


class Position:
    """A single open position with staged exit tracking."""

    def __init__(self, code: str, account: str, entry_price: float, volume: int,
                 score: float = 0.0, order_id: str = "", reason: str = "",
                 entry_time: str = "", highest_price: float = 0.0,
                 initial_volume: int = 0, stage_sold: int = 0):
        self.code = code
        self.account = account
        self.entry_price = entry_price
        self.volume = volume
        self.initial_volume = initial_volume or volume  # original buy volume
        self.score = score
        self.order_id = order_id
        self.reason = reason
        self.entry_time = entry_time or datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        self.highest_price = highest_price or entry_price
        self.stage_sold = stage_sold  # 0=none, 1=first tranche sold, 2=second tranche sold

    def to_dict(self) -> dict:
        return {
            "code": self.code,
            "account": self.account,
            "entry_price": self.entry_price,
            "volume": self.volume,
            "initial_volume": self.initial_volume,
            "score": self.score,
            "order_id": self.order_id,
            "reason": self.reason,
            "entry_time": self.entry_time,
            "highest_price": self.highest_price,
            "stage_sold": self.stage_sold,
        }

    @classmethod
    def from_dict(cls, d: dict) -> "Position":
        # Backward compat: old entries may lack new fields
        d.setdefault("initial_volume", d.get("volume", 0))
        d.setdefault("stage_sold", 0)
        return cls(**{k: v for k, v in d.items() if k in cls.__init__.__code__.co_varnames})


class PositionManager:
    """
    Manages all open positions with persistence and exit monitoring.

    Risk parameters:
        stop_loss_pct:      fixed stop-loss percentage (default -5%)
        trailing_stop_pct:  trailing stop from highest price (default -8%)
        time_stop_days:     sell if held N days with no profit (default 5)
        take_profit_pct:    sell if profit exceeds this (default +15%)
    """

    def __init__(self, qmt_client, account: str = "",
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
        # Sync with QMT real positions on startup
        if account:
            self._sync_with_qmt()

    def _sync_with_qmt(self):
        """Sync positions.json with QMT real positions on startup.
        
        - Remove entries from positions.json that no longer exist in QMT (already sold externally)
        - Add QMT positions not tracked in positions.json (bought externally)
        """
        try:
            real_positions = self.qmt.get_positions(self.account)
            if not real_positions:
                return
            
            real_codes = {p["code"] for p in real_positions}
            real_map = {p["code"]: p for p in real_positions}
            tracked_codes = set(self.positions.keys())

            # Remove positions no longer in QMT
            removed = tracked_codes - real_codes
            for code in removed:
                logger.info(f"[sync] Removing {code} (no longer in QMT)")
                del self.positions[code]

            # Add QMT positions not yet tracked (bought externally or before tracking started)
            added = real_codes - tracked_codes
            for code in added:
                rp = real_map[code]
                pos = Position(
                    code=code,
                    account=self.account,
                    entry_price=rp.get("cost_price", 0),
                    volume=rp.get("volume", 0),
                    score=0.0,
                    order_id="external",
                    reason="synced from QMT",
                )
                self.positions[code] = pos
                logger.info(f"[sync] Added {code} from QMT (vol={pos.volume}, cost={pos.entry_price:.2f})")

            # Update volumes for existing positions (may have partial sells)
            for code in tracked_codes & real_codes:
                rp = real_map[code]
                if self.positions[code].volume != rp["volume"]:
                    old_vol = self.positions[code].volume
                    self.positions[code].volume = rp["volume"]
                    logger.info(f"[sync] Updated {code} volume: {old_vol} → {rp['volume']}")

            if removed or added:
                self._save()
                logger.info(f"[sync] Synced with QMT: {len(real_codes)} real, +{len(added)} added, -{len(removed)} removed")
            else:
                logger.info(f"[sync] QMT sync OK: {len(real_codes)} positions match")

        except Exception as e:
            logger.warning(f"[sync] QMT sync failed: {e}")

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

    def _get_atr(self, code: str, period: int = 14) -> float:
        """Calculate Average True Range for dynamic stop-loss sizing."""
        try:
            from qmt_client import HAS_XTQUANT
            if not HAS_XTQUANT:
                return 0
            from xtquant import xtdata
            md = xtdata.get_market_data_ex([], [code], period="1d", count=period + 1)
            if code not in md:
                return 0
            df = md[code]
            highs = df["high"].values
            lows = df["low"].values
            closes = df["close"].values
            if len(closes) < 2:
                return 0
            trs = []
            for i in range(1, len(closes)):
                tr = max(highs[i] - lows[i],
                         abs(highs[i] - closes[i - 1]),
                         abs(lows[i] - closes[i - 1]))
                trs.append(tr)
            return sum(trs) / len(trs) if trs else 0
        except Exception:
            return 0

    def _get_ma_signal(self, code: str) -> str:
        """Check M5/M8 moving average crossover on hourly chart.

        Checks last 3 bars to avoid missing crossovers between monitor cycles.

        Returns:
          "death_cross" — M5 crossed below M8 within last 3 bars
          "below"       — M5 is below M8 (bearish)
          "above"       — M5 is above M8 (bullish, keep holding)
          "unknown"     — insufficient data
        """
        try:
            from qmt_client import HAS_XTQUANT
            if not HAS_XTQUANT:
                return "unknown"
            from xtquant import xtdata
            md = xtdata.get_market_data_ex([], [code], period="60m", count=12)
            if code not in md:
                return "unknown"
            closes = md[code]["close"].values.tolist()
            if len(closes) < 9:
                return "unknown"

            def ma(data, n):
                return sum(data[-n:]) / n

            # Check last 3 bars for crossover (covers 3-hour window)
            for offset in range(3):
                if offset == 0:
                    seg = closes
                else:
                    seg = closes[:-offset]
                if len(seg) < 8:
                    continue
                m5_now = ma(seg, 5)
                m8_now = ma(seg, 8)
                m5_prev = ma(seg[:-1], 5)
                m8_prev = ma(seg[:-1], 8)
                if m5_prev >= m8_prev and m5_now < m8_now:
                    return "death_cross"

            # Current state
            m5_now = ma(closes, 5)
            m8_now = ma(closes, 8)
            if m5_now < m8_now:
                return "below"
            return "above"
        except Exception as e:
            logger.debug(f"[exit] MA signal error for {code}: {e}")
            return "unknown"

    def _staged_sell_volume(self, pos: "Position", pnl_pct: float) -> Optional[tuple]:
        """Staged take-profit: sell in tranches to let winners run.

        Stage 0 (not sold yet): sell 1/3 at +8%, move mental stop to breakeven
        Stage 1 (1/3 sold):     sell 1/3 at +15%
        Stage 2 (2/3 sold):     remaining 1/3 rides with MA trailing

        Returns (volume_to_sell, reason, new_stage) or None.
        """
        init_vol = pos.initial_volume
        lot = max((init_vol // 3) // 100 * 100, 100)  # round to 100-share lot

        if pos.stage_sold == 0 and pnl_pct >= 8.0:
            sell_vol = min(lot, pos.volume)
            return sell_vol, f"staged_tp1(+{pnl_pct:.1f}%, sell 1/3)", 1

        if pos.stage_sold == 1 and pnl_pct >= 15.0:
            sell_vol = min(lot, pos.volume)
            return sell_vol, f"staged_tp2(+{pnl_pct:.1f}%, sell 1/3)", 2

        return None

    def check_exits(self) -> List[dict]:
        """
        Hybrid exit engine: MA trend-following + ATR hard stop.

        Called every minute during trading hours by the monitor loop.

        Strategy (priority order):
          1. Hard stop-loss: ATR-based (absolute risk limit, non-negotiable)
          2. Breakeven stop: after stage 1 partial sell, stop at entry price
          3. MA trend exit: M5 crosses below M8 on hourly chart → trend over → sell
          4. Staged take-profit: sell 1/3 at +8%, 1/3 at +15%, rest rides with MA
          5. Time stop: held > N days with no profit → exit

        Why this is optimal:
          - MA lets winners run: a stock up +30% stays held as long as M5>M8
          - ATR hard stop limits max loss per trade regardless of trend
          - Staged sells lock in profits while keeping upside exposure
          - Breakeven stop ensures profitable trades don't become losses
        """
        if not self.positions:
            return []

        codes = list(self.positions.keys())
        exits = []

        # Batch get current prices via real-time tick data
        prices = {}
        try:
            from qmt_client import HAS_XTQUANT
            if HAS_XTQUANT:
                from xtquant import xtdata
                ticks = xtdata.get_full_tick(codes)
                for code in codes:
                    if code in ticks and ticks[code]:
                        p = ticks[code].get("lastPrice", 0)
                        if p > 0:
                            prices[code] = float(p)
                # Fallback to daily close if tick unavailable
                if len(prices) < len(codes):
                    missing = [c for c in codes if c not in prices]
                    md = xtdata.get_market_data_ex([], missing, period="1d", count=1)
                    for code in missing:
                        if code in md:
                            try:
                                prices[code] = float(md[code]["close"].iloc[-1])
                            except Exception:
                                pass
        except Exception as e:
            logger.error(f"[exit] price fetch error: {e}")
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

            # Calculate days held
            try:
                entry_dt = datetime.strptime(pos.entry_time, "%Y-%m-%d %H:%M:%S")
                days_held = (now - entry_dt).days
            except Exception:
                days_held = 0

            exit_reason = None
            sell_volume = pos.volume  # default: sell all

            # --- 1. Hard stop-loss (ATR-based, absolute floor) ---
            atr = self._get_atr(code)
            if atr > 0:
                atr_stop_pct = min((2 * atr / pos.entry_price) * 100, self.stop_loss_pct)
            else:
                atr_stop_pct = self.stop_loss_pct

            if pnl_pct <= -atr_stop_pct:
                exit_reason = f"stop_loss({pnl_pct:+.1f}%, ATR={atr_stop_pct:.1f}%)"

            # --- 2. Breakeven stop: after first partial sell, stop at entry ---
            elif pos.stage_sold >= 1 and cur_price <= pos.entry_price:
                exit_reason = f"breakeven_stop(stage={pos.stage_sold}, price≤entry)"

            # --- 3. Staged take-profit FIRST (before MA, to protect partial sells) ---
            ma_signal = None
            if not exit_reason:
                staged = self._staged_sell_volume(pos, pnl_pct)
                if staged:
                    sell_volume, exit_reason, new_stage = staged
                    pos.stage_sold = new_stage
                    changed = True

            # --- 4. MA trend exit: M5 crosses below M8 on hourly ---
            if not exit_reason:
                ma_signal = self._get_ma_signal(code)
                if ma_signal == "death_cross":
                    if pos.stage_sold >= 1 and pnl_pct > 0:
                        exit_reason = f"MA_death_cross(M5<M8, pnl={pnl_pct:+.1f}%, close_remaining)"
                    elif pos.stage_sold == 0 and pnl_pct >= 5.0:
                        # Profitable but no staged sell yet → partial (1/3)
                        init_vol = pos.initial_volume
                        sell_volume = max((init_vol // 3) // 100 * 100, 100)
                        sell_volume = min(sell_volume, pos.volume)
                        pos.stage_sold = 1
                        changed = True
                        exit_reason = f"MA_death_cross_partial(+{pnl_pct:.1f}%, sell 1/3)"
                    else:
                        exit_reason = f"MA_death_cross(M5<M8_hourly, pnl={pnl_pct:+.1f}%)"
                elif ma_signal == "below" and pnl_pct <= -1.0:
                    exit_reason = f"MA_bearish(M5<M8, losing {pnl_pct:+.1f}%)"

            # --- 5. Lifecycle + layer-based exit (alpha_engine) ---
            if not exit_reason and alpha_engine:
                try:
                    lifecycle = alpha_engine.detect_lifecycle_phase(code, pos.entry_price, cur_price)
                    phase = lifecycle.get("phase", "unknown")
                    # Distribution phase → urgent full exit
                    if phase == "distribution" and lifecycle.get("confidence", 0) >= 0.6:
                        exit_reason = f"lifecycle_distribution(conf={lifecycle['confidence']:.1f})"
                    # Shakeout + losing → cut early
                    elif phase == "shakeout" and pnl_pct <= -1.0:
                        exit_reason = f"lifecycle_shakeout(pnl={pnl_pct:+.1f}%)"
                except Exception:
                    pass

            # --- 6. Time stop (nuanced: combine days held with MA trend) ---
            if not exit_reason:
                if ma_signal is None:
                    ma_signal = self._get_ma_signal(code)
                if days_held >= 7 and pnl_pct <= -2.0:
                    exit_reason = f"time_stop({days_held}d, losing {pnl_pct:+.1f}%)"
                elif days_held >= 5 and pnl_pct <= 0 and ma_signal != "above":
                    exit_reason = f"time_stop({days_held}d, flat+bearish_MA)"

            if exit_reason:
                # A-share: sell volume must be multiple of 100
                sell_volume = max((sell_volume // 100) * 100, 100)
                sell_volume = min(sell_volume, pos.volume)
                exits.append({
                    "code": code,
                    "account": pos.account,
                    "volume": sell_volume,
                    "entry_price": pos.entry_price,
                    "current_price": cur_price,
                    "pnl_pct": pnl_pct,
                    "reason": exit_reason,
                    "is_partial": sell_volume < pos.volume,
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
