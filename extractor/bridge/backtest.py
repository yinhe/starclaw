"""
Backtest Framework — 用历史数据验证主升浪策略参数有效性。

Usage:
  python backtest.py --days 60 --min-score 0.60 --top-n 10

Features:
  - Downloads historical daily data via xtdata
  - Simulates scan_once() → score → select → hold → exit logic
  - Reports: win rate, profit factor, max drawdown, Sharpe ratio
  - Supports parameter sweep for optimization
"""

import logging
import os
import sys
import time as time_module
from datetime import datetime, timedelta
from typing import Dict, List, Optional

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

logger = logging.getLogger("backtest")


class BacktestEngine:
    """Simulated trading engine using historical data."""

    def __init__(self, initial_capital: float = 500000,
                 min_score: float = 0.60, top_n: int = 10,
                 stop_loss_pct: float = 5.0, tp1_pct: float = 8.0, tp2_pct: float = 15.0,
                 max_holdings: int = 10, max_per_stock_pct: float = 0.10,
                 time_stop_days: int = 5):
        self.initial_capital = initial_capital
        self.capital = initial_capital
        self.min_score = min_score
        self.top_n = top_n
        self.stop_loss_pct = stop_loss_pct
        self.tp1_pct = tp1_pct
        self.tp2_pct = tp2_pct
        self.max_holdings = max_holdings
        self.max_per_stock_pct = max_per_stock_pct
        self.time_stop_days = time_stop_days

        # State
        self.positions: Dict[str, dict] = {}  # code → {entry_price, volume, entry_date, highest}
        self.trades: List[dict] = []  # completed trades
        self.equity_curve: List[dict] = []  # daily equity snapshots
        self.daily_pnl: List[float] = []

    def _score_stock(self, closes, highs, lows, volumes) -> float:
        """Simplified version of score_main_rise for backtesting speed."""
        if len(closes) < 40:
            return 0.0

        last_c = closes[-1]
        prev_c = closes[-2]
        if last_c <= 0 or prev_c <= 0:
            return 0.0

        # Trend
        ma20 = np.mean(closes[-20:])
        ma30 = np.mean(closes[-30:]) if len(closes) >= 30 else ma20
        trend_ok = last_c > ma20 > ma30
        trend_score = 1.0 if trend_ok else 0.3

        # MACD
        def ema(data, n):
            mult = 2.0 / (n + 1)
            vals = [np.mean(data[:n])]
            for v in data[n:]:
                vals.append(v * mult + vals[-1] * (1 - mult))
            return vals

        if len(closes) >= 35:
            ema12 = ema(closes, 12)
            ema26 = ema(closes, 26)
            off = len(ema12) - len(ema26)
            dif = [ema12[off + i] - ema26[i] for i in range(len(ema26))]
            macd_score = 1.0 if dif[-1] > 0 else 0.2
        else:
            macd_score = 0.5

        # Volume-price
        today_change = (last_c - prev_c) / prev_c
        avg_vol = np.mean(volumes[-6:-1]) if len(volumes) >= 6 else 1
        vol_ratio = volumes[-1] / max(avg_vol, 1e-6)
        breakout = 1.0 if 0.03 <= today_change <= 0.095 and vol_ratio >= 1.3 else 0.3
        vol_s = min(1.0, max(0, (vol_ratio - 1) / 1.5))

        # RSI
        if len(closes) >= 15:
            gains = [max(closes[i] - closes[i-1], 0) for i in range(1, len(closes))]
            losses_arr = [max(closes[i-1] - closes[i], 0) for i in range(1, len(closes))]
            avg_g = np.mean(gains[-14:])
            avg_l = np.mean(losses_arr[-14:])
            rsi = 100 - 100 / (1 + avg_g / max(avg_l, 1e-9))
            rsi_score = 0.0 if rsi >= 80 else (1.0 if 40 <= rsi <= 65 else 0.5)
        else:
            rsi_score = 0.5

        raw = (trend_score * 0.25 + macd_score * 0.15 + breakout * 0.20 +
               vol_s * 0.15 + rsi_score * 0.10 + 0.15 * (0.7 if trend_ok else 0.3))
        return round(min(1.0, max(0.0, raw)), 4)

    def _check_exit(self, code: str, pos: dict, cur_price: float, cur_date: str) -> Optional[str]:
        """Check exit conditions for a position."""
        entry = pos["entry_price"]
        highest = pos["highest"]
        pnl_pct = (cur_price - entry) / entry * 100

        # Update highest
        if cur_price > highest:
            pos["highest"] = cur_price

        # Days held
        try:
            entry_dt = datetime.strptime(pos["entry_date"], "%Y-%m-%d")
            cur_dt = datetime.strptime(cur_date, "%Y-%m-%d")
            days_held = (cur_dt - entry_dt).days
        except Exception:
            days_held = 0

        # 1. Hard stop-loss
        if pnl_pct <= -self.stop_loss_pct:
            return f"stop_loss({pnl_pct:+.1f}%)"

        # 2. Staged take-profit at tp1
        if pnl_pct >= self.tp1_pct and not pos.get("tp1_done"):
            pos["tp1_done"] = True
            return f"staged_tp1(+{pnl_pct:.1f}%)"

        # 3. Staged take-profit at tp2
        if pnl_pct >= self.tp2_pct and pos.get("tp1_done") and not pos.get("tp2_done"):
            pos["tp2_done"] = True
            return f"staged_tp2(+{pnl_pct:.1f}%)"

        # 4. Time stop
        if days_held >= self.time_stop_days and pnl_pct <= 0:
            return f"time_stop({days_held}d, {pnl_pct:+.1f}%)"

        return None

    def run(self, stock_pool: List[str], start_date: str, end_date: str) -> dict:
        """Run backtest over date range.

        Args:
            stock_pool: list of stock codes
            start_date: "YYYYMMDD"
            end_date: "YYYYMMDD"
        """
        try:
            from xtquant import xtdata
        except ImportError:
            return {"error": "xtquant not available"}

        logger.info(f"[backtest] Loading data for {len(stock_pool)} stocks from {start_date} to {end_date}...")

        # Pre-download all data
        xtdata.download_history_data2(stock_pool + ["000001.SH"], period="1d",
                                      start_time=start_date, end_time=end_date)

        # Get all daily data
        all_data = xtdata.get_market_data_ex([], stock_pool, period="1d",
                                              start_time=start_date, end_time=end_date)
        index_data = xtdata.get_market_data_ex([], ["000001.SH"], period="1d",
                                                start_time=start_date, end_time=end_date)

        if "000001.SH" not in index_data:
            return {"error": "no index data"}

        # Get trading dates from index
        idx_df = index_data["000001.SH"]
        dates = idx_df.index.tolist()  # timestamps
        date_strs = [str(d)[:10] if isinstance(d, str) else datetime.fromtimestamp(d/1000).strftime("%Y-%m-%d")
                     for d in dates]

        logger.info(f"[backtest] {len(date_strs)} trading days, {len(all_data)} stocks with data")

        # Simulate day by day
        lookback = 60  # need 60 bars for scoring
        self.capital = self.initial_capital
        self.positions = {}
        self.trades = []
        self.equity_curve = []

        for day_idx in range(lookback, len(date_strs)):
            cur_date = date_strs[day_idx]

            # Calculate portfolio value
            portfolio_value = self.capital
            for code, pos in self.positions.items():
                if code in all_data:
                    try:
                        price = float(all_data[code]["close"].iloc[day_idx])
                        portfolio_value += price * pos["volume"]
                    except Exception:
                        pass

            self.equity_curve.append({"date": cur_date, "equity": round(portfolio_value, 2)})

            # Check exits first
            codes_to_remove = []
            for code, pos in list(self.positions.items()):
                if code not in all_data:
                    continue
                try:
                    cur_price = float(all_data[code]["close"].iloc[day_idx])
                except Exception:
                    continue

                reason = self._check_exit(code, pos, cur_price, cur_date)
                if reason:
                    # Sell
                    pnl_pct = (cur_price - pos["entry_price"]) / pos["entry_price"] * 100
                    sell_volume = pos["volume"]
                    if "staged_tp1" in reason:
                        sell_volume = max(pos["volume"] // 3, 100)
                    elif "staged_tp2" in reason:
                        sell_volume = max(pos["volume"] // 2, 100)

                    sell_amount = cur_price * sell_volume
                    self.capital += sell_amount
                    pos["volume"] -= sell_volume

                    self.trades.append({
                        "code": code, "entry_price": pos["entry_price"],
                        "exit_price": cur_price, "volume": sell_volume,
                        "entry_date": pos["entry_date"], "exit_date": cur_date,
                        "pnl_pct": round(pnl_pct, 2), "exit_reason": reason,
                    })

                    if pos["volume"] <= 0:
                        codes_to_remove.append(code)

            for code in codes_to_remove:
                del self.positions[code]

            # Scan for new entries (simplified: every day)
            if len(self.positions) >= self.max_holdings:
                continue

            # Score all stocks
            candidates = []
            for code in stock_pool:
                if code in self.positions or code not in all_data:
                    continue
                try:
                    df = all_data[code]
                    closes = df["close"].iloc[:day_idx+1].values.astype(float)
                    highs = df["high"].iloc[:day_idx+1].values.astype(float)
                    lows = df["low"].iloc[:day_idx+1].values.astype(float)
                    vols = df["volume"].iloc[:day_idx+1].values.astype(float)

                    if len(closes) < 40:
                        continue
                    score = self._score_stock(closes[-60:], highs[-60:], lows[-60:], vols[-60:])
                    if score >= self.min_score:
                        candidates.append((code, score, float(closes[-1])))
                except Exception:
                    continue

            # Select top N
            candidates.sort(key=lambda x: x[1], reverse=True)
            slots = self.max_holdings - len(self.positions)
            for code, score, price in candidates[:min(self.top_n, slots, 3)]:
                if price <= 0:
                    continue
                max_amount = self.capital * self.max_per_stock_pct
                volume = int(max_amount / price / 100) * 100
                if volume < 100:
                    continue
                cost = price * volume
                if cost > self.capital:
                    continue
                self.capital -= cost
                self.positions[code] = {
                    "entry_price": price, "volume": volume,
                    "entry_date": cur_date, "highest": price,
                }

        # Final equity
        final_equity = self.capital
        for code, pos in self.positions.items():
            if code in all_data:
                try:
                    final_equity += float(all_data[code]["close"].iloc[-1]) * pos["volume"]
                except Exception:
                    pass

        return self._compute_results(final_equity)

    def _compute_results(self, final_equity: float) -> dict:
        """Compute backtest results."""
        total_return = (final_equity - self.initial_capital) / self.initial_capital * 100
        trades = self.trades

        if not trades:
            return {
                "total_return_pct": round(total_return, 2),
                "total_trades": 0,
                "final_equity": round(final_equity, 2),
            }

        wins = [t for t in trades if t["pnl_pct"] > 0]
        losses = [t for t in trades if t["pnl_pct"] <= 0]
        win_rate = len(wins) / len(trades) if trades else 0
        avg_win = np.mean([t["pnl_pct"] for t in wins]) if wins else 0
        avg_loss = np.mean([t["pnl_pct"] for t in losses]) if losses else 0
        profit_factor = abs(avg_win * len(wins) / (avg_loss * len(losses))) if losses and avg_loss != 0 else 999

        # Max drawdown from equity curve
        max_dd = 0
        peak = 0
        for point in self.equity_curve:
            eq = point["equity"]
            if eq > peak:
                peak = eq
            dd = (peak - eq) / peak * 100 if peak > 0 else 0
            if dd > max_dd:
                max_dd = dd

        # Sharpe ratio (annualized, assuming 252 trading days)
        if len(self.equity_curve) >= 2:
            returns = []
            for i in range(1, len(self.equity_curve)):
                prev = self.equity_curve[i-1]["equity"]
                cur = self.equity_curve[i]["equity"]
                returns.append((cur - prev) / prev if prev > 0 else 0)
            if returns:
                sharpe = np.mean(returns) / max(np.std(returns), 1e-9) * np.sqrt(252)
            else:
                sharpe = 0
        else:
            sharpe = 0

        return {
            "initial_capital": self.initial_capital,
            "final_equity": round(final_equity, 2),
            "total_return_pct": round(total_return, 2),
            "total_trades": len(trades),
            "wins": len(wins),
            "losses": len(losses),
            "win_rate": round(win_rate, 3),
            "avg_win_pct": round(float(avg_win), 2),
            "avg_loss_pct": round(float(avg_loss), 2),
            "profit_factor": round(float(profit_factor), 2),
            "max_drawdown_pct": round(max_dd, 2),
            "sharpe_ratio": round(float(sharpe), 2),
            "trading_days": len(self.equity_curve),
            "open_positions": len(self.positions),
        }


# --- API endpoint ---

def run_backtest(days: int = 60, min_score: float = 0.60, top_n: int = 10,
                 initial_capital: float = 500000) -> dict:
    """Run backtest with given parameters. Called from Bridge API."""
    end_date = datetime.now().strftime("%Y%m%d")
    start_date = (datetime.now() - timedelta(days=days + 80)).strftime("%Y%m%d")  # +80 for lookback

    # Get stock pool
    try:
        from xtquant import xtdata
        sh = xtdata.get_stock_list_in_sector("上证A股") or []
        sz = xtdata.get_stock_list_in_sector("深证A股") or []
        pool = [c for c in (sh + sz) if not c.split(".")[0].startswith(("300", "301", "688", "689", "8"))]
        # Sample for speed (full pool too slow for backtest)
        import random
        pool = random.sample(pool, min(500, len(pool)))
    except Exception as e:
        return {"error": f"stock pool failed: {e}"}

    engine = BacktestEngine(
        initial_capital=initial_capital,
        min_score=min_score,
        top_n=top_n,
    )
    return engine.run(pool, start_date, end_date)


# --- CLI ---

if __name__ == "__main__":
    import argparse

    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(message)s")

    parser = argparse.ArgumentParser(description="Backtest main-wave strategy")
    parser.add_argument("--days", type=int, default=60, help="Trading days to backtest")
    parser.add_argument("--min-score", type=float, default=0.60)
    parser.add_argument("--top-n", type=int, default=10)
    parser.add_argument("--capital", type=float, default=500000)
    args = parser.parse_args()

    result = run_backtest(days=args.days, min_score=args.min_score,
                          top_n=args.top_n, initial_capital=args.capital)
    print("\n=== Backtest Results ===")
    for k, v in result.items():
        print(f"  {k}: {v}")
