"""
Alpha Engine — 10 core trading algorithms extracted from extractor/main.py (41K lines).

Adapted for Bridge's xtquant SDK (xtdata.*) instead of QMT ContextInfo API.

Modules:
  1. compute_layer_states     — Multi-timeframe W/D/H trend signals (RED/GREEN)
  2. score_main_rise          — 4-dimension main-wave candidate scoring [0,1]
  3. detect_prebreakout       — Pre-breakout pattern recognition
  4. compute_kelly_weight     — Kelly criterion position sizing
  5. dynamic_profit_targets   — Volatility + category-based profit targets
  6. detect_lifecycle_phase   — Stock lifecycle: accumulation/markup/shakeout/distribution
  7. classify_chip_shape      — Chip distribution: low_lock/low_dense/double_peak/high_dense
  8. detect_market_env        — Enhanced market environment (bull/bear/sideways)
  9. evaluate_sector_strength — Hot sector identification
  10. compute_layer_batch     — Layered batch-buy system (W/D/H staged entries)
"""

import logging
import math
import time as time_module
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

import numpy as np

logger = logging.getLogger("alpha_engine")

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

def _get_xtdata():
    """Lazy import xtdata."""
    try:
        from xtquant import xtdata
        return xtdata
    except ImportError:
        return None


def _get_daily(code: str, count: int = 60) -> Optional[dict]:
    """Fetch daily OHLCV as numpy arrays.  Returns dict with keys: close, high, low, volume, open."""
    xt = _get_xtdata()
    if not xt:
        return None
    try:
        md = xt.get_market_data_ex([], [code], period="1d", count=count)
        if code not in md:
            return None
        df = md[code]
        return {
            "close": df["close"].values.astype(float),
            "high": df["high"].values.astype(float),
            "low": df["low"].values.astype(float),
            "volume": df["volume"].values.astype(float),
            "open": df["open"].values.astype(float),
        }
    except Exception as e:
        logger.debug(f"_get_daily({code}) error: {e}")
        return None


def _get_period_closes(code: str, period: str, count: int) -> list:
    """Return list of close prices for given period."""
    xt = _get_xtdata()
    if not xt:
        return []
    try:
        md = xt.get_market_data_ex([], [code], period=period, count=count)
        if code not in md:
            return []
        return md[code]["close"].values.astype(float).tolist()
    except Exception:
        return []


def _ma(values, n: int) -> Optional[float]:
    """Simple moving average of last n values."""
    if not values or len(values) < n:
        return None
    return sum(values[-n:]) / n


def _pct_rank(arr, value) -> float:
    """Percentile rank of value in array (0=lowest, 1=highest)."""
    try:
        a = np.asarray(arr, dtype=float)
        if a.size == 0:
            return 0.5
        return float(np.sum(a <= value) / a.size)
    except Exception:
        return 0.5


def _compute_atr(highs, lows, closes, n: int = 14) -> float:
    """Average True Range (absolute), returns 0 on error."""
    try:
        if len(closes) < 2:
            return 0
        trs = []
        for i in range(1, len(closes)):
            tr = max(highs[i] - lows[i],
                     abs(highs[i] - closes[i - 1]),
                     abs(lows[i] - closes[i - 1]))
            trs.append(tr)
        if not trs:
            return 0
        return sum(trs[-n:]) / min(n, len(trs))
    except Exception:
        return 0


def _compute_atr_norm(highs, lows, closes, n: int = 14) -> float:
    """Normalized ATR (ATR / last close), returns fraction like 0.05."""
    atr = _compute_atr(highs, lows, closes, n)
    last_c = closes[-1] if len(closes) > 0 else 1
    return atr / max(last_c, 1e-6)


def _rsi(closes, period: int = 14) -> Optional[float]:
    """Relative Strength Index."""
    if len(closes) < period + 1:
        return None
    gains, losses = [], []
    for i in range(1, len(closes)):
        d = closes[i] - closes[i - 1]
        gains.append(max(d, 0))
        losses.append(max(-d, 0))
    avg_gain = sum(gains[-period:]) / period
    avg_loss = sum(losses[-period:]) / period
    if avg_loss == 0:
        return 100.0
    rs = avg_gain / avg_loss
    return 100 - 100 / (1 + rs)


def _ema(values, period: int) -> list:
    """Exponential Moving Average."""
    if not values or len(values) < period:
        return []
    mult = 2.0 / (period + 1)
    ema_vals = [sum(values[:period]) / period]
    for v in values[period:]:
        ema_vals.append(v * mult + ema_vals[-1] * (1 - mult))
    return ema_vals


def _macd(closes, fast: int = 12, slow: int = 26, signal: int = 9) -> Optional[dict]:
    """MACD indicator. Returns dict with macd, signal, histogram values."""
    if len(closes) < slow + signal:
        return None
    ema_fast = _ema(closes, fast)
    ema_slow = _ema(closes, slow)
    # Align lengths (ema_slow is shorter)
    offset = len(ema_fast) - len(ema_slow)
    dif = [ema_fast[offset + i] - ema_slow[i] for i in range(len(ema_slow))]
    dea = _ema(dif, signal)
    if not dea:
        return None
    offset2 = len(dif) - len(dea)
    hist = [dif[offset2 + i] - dea[i] for i in range(len(dea))]
    return {
        "dif": dif[-1] if dif else 0,
        "dea": dea[-1] if dea else 0,
        "histogram": hist[-1] if hist else 0,
        "dif_prev": dif[-2] if len(dif) >= 2 else 0,
        "dea_prev": dea[-2] if len(dea) >= 2 else 0,
    }


def _relative_strength(stock_closes: list, index_closes: list) -> Optional[float]:
    """Relative Strength vs index over 20 days. >1 means outperforming."""
    if len(stock_closes) < 20 or len(index_closes) < 20:
        return None
    stock_ret = (stock_closes[-1] - stock_closes[-20]) / stock_closes[-20]
    index_ret = (index_closes[-1] - index_closes[-20]) / index_closes[-20]
    if abs(index_ret) < 1e-6:
        return 1.0
    return (1 + stock_ret) / (1 + index_ret)


# ===========================================================================
# 1. Multi-timeframe Layer States (W/D/H)
# ===========================================================================

def _trend_state(closes: list, ma_window: int = 20, confirm_bars: int = 3,
                 slope_mult: float = 1.0, require_rsi: bool = False) -> Tuple[str, dict]:
    """Determine RED (bullish) or GREEN (neutral/bearish) for a single timeframe.

    RED = price above rising MA with positive slope for confirm_bars.
    """
    if not closes or len(closes) < ma_window + confirm_bars:
        return "GREEN", {"reason": "insufficient_data"}

    ma_now = _ma(closes, ma_window)
    ma_prev = _ma(closes[:-1], ma_window)
    if ma_now is None or ma_prev is None:
        return "GREEN", {"reason": "ma_calc_fail"}

    slope = (ma_now - ma_prev) / max(abs(ma_prev), 1e-9) * slope_mult
    last_price = closes[-1]
    above_ma = last_price > ma_now
    ma_rising = ma_now > ma_prev

    meta = {"ma": round(ma_now, 4), "slope": round(slope, 6), "above": above_ma, "rising": ma_rising}

    if above_ma and ma_rising:
        # Check confirm bars: all recent closes above MA
        recent = closes[-confirm_bars:]
        confirmed = all(c > ma_now * 0.998 for c in recent)
        if confirmed:
            if require_rsi:
                r = _rsi(closes)
                if r is not None and r < 30:
                    return "GREEN", {**meta, "rsi": r, "reason": "oversold_override"}
            return "RED", meta
    return "GREEN", meta


def compute_layer_states(code: str) -> dict:
    """Multi-timeframe trend state: Weekly (W), Daily (D), Hourly (H).

    Returns: {'W': 'RED'|'GREEN', 'D': ..., 'H': ..., 'meta': {...}}
    RED = bullish trend confirmed, GREEN = neutral/bearish.
    """
    w_closes = _get_period_closes(code, "1w", 80)
    d_closes = _get_period_closes(code, "1d", 120)
    h_closes = _get_period_closes(code, "60m", 200)

    # Approximate weekly from daily if weekly data unavailable
    if not w_closes and d_closes and len(d_closes) >= 10:
        w_closes = [sum(d_closes[i:i + 5]) / 5 for i in range(0, len(d_closes) - 4, 5)]

    w_state, w_meta = _trend_state(w_closes, ma_window=10, confirm_bars=2, slope_mult=1.5)
    d_state, d_meta = _trend_state(d_closes, ma_window=20, confirm_bars=3, slope_mult=1.0, require_rsi=True)
    h_state, h_meta = _trend_state(h_closes, ma_window=20, confirm_bars=3, slope_mult=0.8)

    return {
        "W": w_state, "D": d_state, "H": h_state,
        "meta": {"W": w_meta, "D": d_meta, "H": h_meta},
    }


# ===========================================================================
# 2. Main Rise Wave Scoring (4-dimension)
# ===========================================================================

def score_main_rise(code: str, min_bars: int = 40) -> float:
    """Score a stock for main-wave potential [0, 1].

    6-dimension scoring (upgraded from 4-dimension):
      1) Trend & position: MA20/30 bullish + 60d price zone  (25%)
      2) MACD confirmation: DIF>DEA, histogram positive       (15%)
      3) Momentum speed: 3-day elasticity vs 10-day           (15%)
      4) Volume-price action: breakout + volume ratio          (20%)
      5) RSI filter: penalize overbought (>80), bonus 40-65    (10%)
      6) Relative strength vs index: outperform = bonus        (15%)
      * Volatility penalty: extreme ATR multiplier
    """
    d = _get_daily(code, count=60)
    if d is None:
        return 0.0
    closes, highs, lows, vols = d["close"], d["high"], d["low"], d["volume"]
    if closes.size < min_bars:
        return 0.0

    last_c = float(closes[-1])
    prev_c = float(closes[-2]) if closes.size >= 2 else last_c
    if last_c <= 0 or prev_c <= 0:
        return 0.0

    # 1) Trend & position (25%)
    ma20 = float(np.mean(closes[-20:])) if closes.size >= 20 else last_c
    ma30 = float(np.mean(closes[-30:])) if closes.size >= 30 else ma20
    ma20_prev = float(np.mean(closes[-21:-1])) if closes.size >= 21 else ma20
    ma30_prev = float(np.mean(closes[-31:-1])) if closes.size >= 31 else ma30

    trend_ok = (last_c > ma20 > ma30) and (ma20 > ma20_prev) and (ma30 >= ma30_prev * 0.998)
    trend_score = 1.0 if trend_ok else 0.3

    pos_rank = _pct_rank(closes[-60:], last_c)
    # Smooth scoring instead of hard cutoffs
    if pos_rank < 0.25:
        pos_score = 0.1  # too low = likely still falling
    elif pos_rank > 0.92:
        pos_score = 0.1  # too high = chasing risk
    elif 0.35 <= pos_rank <= 0.85:
        pos_score = 1.0  # sweet spot
    else:
        pos_score = 0.5  # transitional

    # 2) MACD confirmation (15%)
    macd_data = _macd(closes.tolist())
    if macd_data:
        dif, dea, hist = macd_data["dif"], macd_data["dea"], macd_data["histogram"]
        if dif > dea and hist > 0:
            macd_score = 1.0  # bullish
        elif dif > dea and hist <= 0:
            macd_score = 0.6  # weakening but still above
        elif dif > macd_data["dif_prev"]:
            macd_score = 0.4  # turning up
        else:
            macd_score = 0.1  # bearish
    else:
        macd_score = 0.5

    # 3) Momentum speed (15%)
    if closes.size >= 10:
        c3 = float(closes[-4]) if closes.size >= 4 else float(closes[0])
        c10 = float(closes[-11]) if closes.size >= 11 else float(closes[0])
        r3 = (last_c - c3) / c3 if c3 > 0 else 0.0
        r10 = (last_c - c10) / c10 if c10 > 0 else 0.0
        speed_ratio = r3 / max(0.005, r10) if r10 > 0.005 else max(0, r3 * 10)
    else:
        speed_ratio = (last_c - prev_c) / prev_c * 10
    speed_score = min(1.0, max(0.0, speed_ratio / 2.0))

    # 4) Volume-price action (20%)
    today_change = (last_c - prev_c) / prev_c
    avg5_vol = float(np.mean(vols[-6:-1])) if vols.size >= 6 else 1.0
    cur_vol = float(vols[-1]) if vols.size > 0 else 0.0
    vol_ratio = cur_vol / max(avg5_vol, 1e-6)

    if today_change < 0.02:
        breakout_score = 0.0
    elif 0.02 <= today_change < 0.05:
        breakout_score = 0.5
    elif 0.05 <= today_change <= 0.095:
        breakout_score = 1.0
    elif today_change > 0.12:
        breakout_score = 0.2  # overextended, dangerous to chase
    else:
        breakout_score = 0.6

    vol_score = min(1.0, max(0.0, (vol_ratio - 1.0) / 1.5)) if vol_ratio > 1.0 else 0.0
    vp_score = breakout_score * 0.5 + vol_score * 0.5

    # 5) RSI filter (10%)
    rsi_val = _rsi(closes.tolist())
    if rsi_val is not None:
        if rsi_val >= 80:
            rsi_score = 0.0  # overbought — DO NOT BUY
        elif rsi_val >= 70:
            rsi_score = 0.3  # approaching overbought
        elif 40 <= rsi_val <= 65:
            rsi_score = 1.0  # healthy momentum zone
        elif rsi_val < 30:
            rsi_score = 0.2  # oversold, may bounce but risky
        else:
            rsi_score = 0.6
    else:
        rsi_score = 0.5

    # 6) Relative strength vs index (15%)
    index_closes = _get_period_closes("000001.SH", "1d", 30)
    rs = _relative_strength(closes.tolist(), index_closes)
    if rs is not None:
        if rs >= 1.1:
            rs_score = 1.0  # strongly outperforming
        elif rs >= 1.0:
            rs_score = 0.7  # slightly outperforming
        elif rs >= 0.95:
            rs_score = 0.4  # roughly inline
        else:
            rs_score = 0.1  # underperforming — avoid
    else:
        rs_score = 0.5

    # Volatility penalty
    atr_n = _compute_atr_norm(highs, lows, closes, n=14)
    if atr_n <= 0.06:
        vol_penalty = 1.0
    elif atr_n >= 0.15:
        vol_penalty = 0.5
    else:
        vol_penalty = 1.0 - (atr_n - 0.06) / 0.09 * 0.5

    # Weighted composite (sums to 1.0)
    raw = (trend_score * 0.15 + pos_score * 0.10 +
           macd_score * 0.15 + speed_score * 0.15 +
           vp_score * 0.20 + rsi_score * 0.10 + rs_score * 0.15) * vol_penalty
    return round(min(1.0, max(0.0, raw)), 4)


# ===========================================================================
# 3. Pre-breakout Detection
# ===========================================================================

def detect_prebreakout(codes: List[str], ma_band_max: float = 0.02,
                       vol_ratio_min: float = 1.2) -> List[Tuple[str, float]]:
    """Identify stocks with pre-breakout pattern: MA convergence + volume expansion.

    Returns sorted list of (code, score) from best to worst.
    """
    xt = _get_xtdata()
    if not xt:
        return []

    results = []
    for code in codes:
        try:
            d = _get_daily(code, count=25)
            if d is None or d["close"].size < 10:
                continue
            closes = d["close"]
            highs = d["high"]
            vols = d["volume"]
            last_c = float(closes[-1])
            open_p = float(d["open"][-1])

            # MA5, MA10, MA20
            ma5 = float(np.mean(closes[-5:])) if closes.size >= 5 else 0
            ma10 = float(np.mean(closes[-10:])) if closes.size >= 10 else 0
            ma20 = float(np.mean(closes[-20:])) if closes.size >= 20 else 0
            if not (ma5 > 0 and ma10 > 0 and ma20 > 0):
                continue

            # Band tightness (MA convergence)
            ma_max = max(ma5, ma10, ma20)
            ma_min = min(ma5, ma10, ma20)
            band = (ma_max - ma_min) / ma_min
            if band > ma_band_max:
                continue

            # Price near/above MA10
            dev_ma10 = abs(last_c - ma10) / ma10
            if dev_ma10 > 0.015 or last_c < ma10:
                continue

            # Volume expansion
            avg_vol_prev = float(np.mean(vols[-11:-1])) if vols.size >= 11 else 0
            cur_vol = float(vols[-1])
            vol_r = cur_vol / max(avg_vol_prev, 1e-6)
            if vol_r < vol_ratio_min:
                continue

            # Not already extended
            day_rise = (last_c / open_p - 1.0) if open_p > 0 else 0
            if day_rise > 0.06:
                continue

            # Score: tightness + volume + position
            tight_s = max(0, 1.0 - band / ma_band_max)
            vol_s = min(1.0, (vol_r - 1.0) / 1.5)
            score = tight_s * 0.35 + vol_s * 0.35 + (1.0 - dev_ma10 / 0.015) * 0.30
            results.append((code, round(score, 4)))
        except Exception:
            continue

    results.sort(key=lambda x: x[1], reverse=True)
    return results


# ===========================================================================
# 4. Kelly Criterion Position Sizing
# ===========================================================================

def _map_signal_strength(strength: int) -> Tuple[float, float]:
    """Map signal strength (0-6) to (win_probability, payoff_ratio)."""
    mapping = {
        6: (0.58, 1.40),  # perfect
        5: (0.55, 1.30),  # excellent
        4: (0.53, 1.25),  # premium
        3: (0.52, 1.20),  # confirmed
        2: (0.50, 1.15),  # detected
        1: (0.50, 1.10),  # weak
        0: (0.50, 1.10),
    }
    return mapping.get(min(6, max(0, strength)), (0.50, 1.10))


def compute_kelly_weight(strength: int, k_fraction: float = 0.33) -> float:
    """Fractional Kelly criterion: f = k * (p - (1-p)/b), floored at 0.

    Args:
        strength: Signal strength 0-6 (higher = stronger signal)
        k_fraction: Kelly fraction (0.33 = 1/3 Kelly, conservative)
    Returns:
        Optimal position weight as fraction of capital [0, ~0.15]
    """
    p, b = _map_signal_strength(strength)
    edge = p - (1 - p) / max(b, 1e-6)
    f_star = max(0.0, edge)
    return max(0.0, k_fraction * f_star)


# ===========================================================================
# 5. Dynamic Profit Targets
# ===========================================================================

def _categorize_stock(code: str) -> str:
    """Classify stock as large_cap, mid_cap, or small_cap."""
    plain = code.split(".")[0] if "." in code else code
    if plain.startswith("600"):
        return "large_cap"
    elif plain.startswith(("000", "002", "300")):
        return "small_cap"
    return "mid_cap"


def dynamic_profit_targets(code: str, market_env: str = "sideways") -> Dict[str, float]:
    """Calculate adaptive profit-taking targets based on stock category + market env + volatility.

    Returns: {
        'tp1': first partial take-profit %,
        'tp2': second partial take-profit %,
        'trailing_activate': % gain to activate trailing stop,
        'trailing_pullback': % pullback from high to trigger sell,
        'hard_stop': hard stop-loss %
    }
    """
    cat = _categorize_stock(code)

    # Base parameters by market environment
    base = {
        "bull":    {"tp1": 10, "tp2": 20, "trail_x": 0.15, "trail_y": 0.04, "stop": 7},
        "sideways": {"tp1": 8, "tp2": 15, "trail_x": 0.12, "trail_y": 0.035, "stop": 5},
        "bear":    {"tp1": 5, "tp2": 10, "trail_x": 0.08, "trail_y": 0.025, "stop": 3},
    }.get(market_env, {"tp1": 8, "tp2": 15, "trail_x": 0.12, "trail_y": 0.035, "stop": 5})

    # Category multiplier
    cat_mult = {"large_cap": 0.9, "mid_cap": 1.0, "small_cap": 1.2}.get(cat, 1.0)

    # ATR-based volatility adjustment
    d = _get_daily(code, count=20)
    vol_mult = 1.0
    if d is not None:
        atr_n = _compute_atr_norm(d["high"], d["low"], d["close"])
        if atr_n > 0.06:
            vol_mult = 1.0 + (atr_n - 0.06) * 5  # wider targets for volatile stocks

    return {
        "tp1": round(base["tp1"] * cat_mult * vol_mult, 1),
        "tp2": round(base["tp2"] * cat_mult * vol_mult, 1),
        "trailing_activate": round(base["trail_x"] * cat_mult, 3),
        "trailing_pullback": round(base["trail_y"] * cat_mult, 3),
        "hard_stop": round(base["stop"] * cat_mult, 1),
    }


# ===========================================================================
# 6. Stock Lifecycle Phase Detection
# ===========================================================================

def detect_lifecycle_phase(code: str, entry_price: float, current_price: float,
                           market_env: str = "sideways") -> Dict[str, Any]:
    """Detect stock lifecycle: accumulation / markup / shakeout / distribution.

    Uses: MA position, price rank, volume pattern, trend structure.
    """
    d = _get_daily(code, count=60)
    if d is None or d["close"].size < 20:
        return {"phase": "unknown", "confidence": 0.3, "reason": "insufficient_data"}

    closes = d["close"]
    vols = d["volume"]
    price = float(current_price) if current_price else float(closes[-1])

    ma5 = float(np.mean(closes[-5:])) if closes.size >= 5 else price
    ma20 = float(np.mean(closes[-20:])) if closes.size >= 20 else price
    above_ma5 = price >= ma5
    above_ma20 = price >= ma20

    # Position rank (0=low, 1=high)
    pos_rank = _pct_rank(closes[-60:] if closes.size >= 60 else closes, price)

    # Volume trend: recent vs prior
    vol_recent = float(np.mean(vols[-5:])) if vols.size >= 5 else 0
    vol_prior = float(np.mean(vols[-20:-5])) if vols.size >= 20 else vol_recent
    vol_ratio = vol_recent / max(vol_prior, 1e-6)

    # ATR normalized
    atr_n = _compute_atr_norm(d["high"], d["low"], closes)

    phase = "markup"
    confidence = 0.5
    reasons = []

    # Accumulation: low position, low volatility, volume building
    if pos_rank <= 0.4 and atr_n <= 0.05 and not above_ma20:
        phase = "accumulation"
        confidence = 0.65
        reasons.append(f"low_pos={pos_rank:.2f}, low_atr={atr_n:.3f}")

    # Markup: high win, strong trend
    elif above_ma20 and above_ma5 and pos_rank >= 0.5 and vol_ratio >= 1.0:
        phase = "markup"
        confidence = 0.7
        reasons.append(f"above_MA5/20, pos={pos_rank:.2f}")

    # Shakeout: was in uptrend, now pulling back to MA20
    elif not above_ma20 and pos_rank >= 0.3 and atr_n >= 0.04:
        phase = "shakeout"
        confidence = 0.6
        reasons.append(f"pullback_to_MA20, atr={atr_n:.3f}")

    # Distribution: high position, high volume, price weakening
    elif pos_rank >= 0.75 and vol_ratio >= 1.5 and not above_ma5:
        phase = "distribution"
        confidence = 0.65
        reasons.append(f"high_pos={pos_rank:.2f}, vol_spike={vol_ratio:.1f}")

    return {"phase": phase, "confidence": confidence, "reason": " | ".join(reasons) or "default"}


# ===========================================================================
# 7. Chip Shape Classification
# ===========================================================================

def classify_chip_shape(code: str) -> Dict[str, Any]:
    """Classify chip distribution pattern.

    Types: low_lock, low_dense, double_peak, high_dense
    """
    d = _get_daily(code, count=150)
    if d is None or d["close"].size < 60:
        return {"shape_type": "low_dense", "confidence": 0.4, "features": {"reason": "insufficient_data"}}

    closes = d["close"]
    highs = d["high"]
    lows = d["low"]
    vols = d["volume"]
    last_c = float(closes[-1])

    pct_pos = _pct_rank(closes[-120:] if closes.size >= 120 else closes, last_c)
    atr_n = _compute_atr_norm(highs, lows, closes, n=14)
    vol_ratio = (float(np.mean(vols[-10:])) /
                 max(1e-6, float(np.mean(vols[-30:-10])))) if vols.size >= 40 else 1.0

    # Double peak detection
    def find_peaks(arr, w=3):
        peaks = []
        for i in range(w, len(arr) - w):
            if arr[i] == max(arr[i - w:i + w + 1]):
                peaks.append(i)
        return peaks

    h60 = highs[-60:]
    peaks = find_peaks(h60, w=3)
    double_peak_conf = 0.0
    if len(peaks) >= 2:
        hv1, hv2 = h60[peaks[-2]], h60[peaks[-1]]
        rel = abs(hv1 - hv2) / max(1e-6, (hv1 + hv2) / 2)
        double_peak_conf = max(0.0, 1.0 - rel * 10)

    features = {"pct_pos": round(pct_pos, 3), "atr_n": round(atr_n, 4), "vol_ratio": round(vol_ratio, 2)}

    # Low lock: bottom, tight, volume rising
    if pct_pos <= 0.35 and atr_n <= 0.035 and vol_ratio >= 1.05:
        conf = min(0.95, 0.6 + 0.4 * min(1.0, vol_ratio - 1.0))
        return {"shape_type": "low_lock", "confidence": round(conf, 2), "features": features}

    # Low dense: bottom area, moderate volatility
    if pct_pos <= 0.45 and atr_n <= 0.05:
        conf = min(0.9, 0.5 + 0.3 * max(0.0, vol_ratio - 0.8))
        return {"shape_type": "low_dense", "confidence": round(conf, 2), "features": features}

    # Double peak: mid-high position with two similar highs
    if pct_pos >= 0.5 and double_peak_conf >= 0.4:
        conf = min(0.9, 0.4 + double_peak_conf * 0.6)
        features["double_peak_conf"] = round(double_peak_conf, 2)
        return {"shape_type": "double_peak", "confidence": round(conf, 2), "features": features}

    # High dense: high position, low volatility (consolidation at top)
    if pct_pos >= 0.65 and atr_n <= 0.045:
        conf = min(0.95, 0.5 + 0.4 * min(1.0, (pct_pos - 0.65) / 0.35))
        return {"shape_type": "high_dense", "confidence": round(conf, 2), "features": features}

    # Default
    shape = "low_dense" if pct_pos < 0.5 else "double_peak"
    return {"shape_type": shape, "confidence": 0.4, "features": features}


# ===========================================================================
# 8. Enhanced Market Environment Detection
# ===========================================================================

_market_env_cache = {"val": None, "ts": 0}


def detect_market_env() -> str:
    """Detect market environment using Shanghai Composite Index.

    Returns: 'bull', 'bear', 'sideways'
    Cached for 60 seconds.
    """
    global _market_env_cache
    now = time_module.time()
    if _market_env_cache["val"] and (now - _market_env_cache["ts"]) <= 60:
        return _market_env_cache["val"]

    index_code = "000001.SH"
    closes = _get_period_closes(index_code, "1d", 30)
    if not closes or len(closes) < 10:
        return "sideways"

    # MA5, MA8, MA34 (unified system)
    ma5 = sum(closes[-5:]) / 5
    ma8 = sum(closes[-8:]) / 8 if len(closes) >= 8 else ma5
    ma34 = sum(closes[-34:]) / 34 if len(closes) >= 34 else (sum(closes[-20:]) / 20 if len(closes) >= 20 else ma8)
    last = closes[-1]

    # Trend slope (20-day)
    if len(closes) >= 20:
        slope_20 = (closes[-1] - closes[-20]) / closes[-20]
    else:
        slope_20 = (closes[-1] - closes[0]) / closes[0]

    # Bull: price > MA5 > MA8 > MA34 and rising
    if last > ma5 > ma8 > ma34 and slope_20 > 0.02:
        env = "bull"
    # Bear: price < MA5 < MA8 < MA34 and falling
    elif last < ma5 < ma8 < ma34 and slope_20 < -0.03:
        env = "bear"
    else:
        env = "sideways"

    _market_env_cache = {"val": env, "ts": now}
    logger.info(f"[alpha] Market env: {env} (slope_20={slope_20:+.2%}, last={last:.2f})")
    return env


# ===========================================================================
# 9. Sector Strength Evaluation
# ===========================================================================

def evaluate_sector_strength(sector_codes: List[str]) -> Dict[str, float]:
    """Evaluate sector strength by average momentum of constituent stocks.

    Args:
        sector_codes: list of stock codes in a sector
    Returns:
        {'strength': 0-100, 'avg_change': float, 'rising_pct': float}
    """
    if not sector_codes:
        return {"strength": 50.0, "avg_change": 0.0, "rising_pct": 0.5}

    xt = _get_xtdata()
    if not xt:
        return {"strength": 50.0, "avg_change": 0.0, "rising_pct": 0.5}

    changes = []
    sample = sector_codes[:50]  # cap for performance
    for code in sample:
        try:
            md = xt.get_market_data_ex([], [code], period="1d", count=2)
            if code in md:
                c = md[code]["close"].values
                if len(c) >= 2 and c[-2] > 0:
                    changes.append((c[-1] - c[-2]) / c[-2])
        except Exception:
            continue

    if not changes:
        return {"strength": 50.0, "avg_change": 0.0, "rising_pct": 0.5}

    avg_chg = sum(changes) / len(changes)
    rising_pct = sum(1 for c in changes if c > 0) / len(changes)

    # Normalize to 0-100 scale
    strength = 50 + avg_chg * 1000  # 1% avg change = 60 strength
    strength = max(0, min(100, strength))

    return {
        "strength": round(strength, 1),
        "avg_change": round(avg_chg, 4),
        "rising_pct": round(rising_pct, 3),
    }


# ===========================================================================
# 10. Layered Batch-Buy System
# ===========================================================================

# Per-stock entry progress tracking (in-memory)
_layer_progress: Dict[str, dict] = {}

# Batch ratios: fraction of max single-stock allocation per layer
LAYER_BATCHES = {
    "W": [0.25, 0.15, 0.10],          # 50% total across 3 batches
    "D": [0.15, 0.15],                  # 30% total across 2 batches
    "H": [0.05, 0.05, 0.05, 0.05],    # 20% total across 4 batches
}
LAYER_MIN_INTERVAL = {"W": 3600, "D": 1800, "H": 300}  # seconds between batches


def compute_layer_batch(code: str, current_price: float, total_asset: float,
                        layer_states: dict, max_single_pct: float = 0.10) -> Tuple[Optional[str], int, dict]:
    """Determine next batch to buy based on multi-timeframe layer signals.

    Returns: (layer_key, buy_shares, meta) or (None, 0, meta) if no batch due.
    Buy shares are rounded to 100-share lots.
    """
    if current_price <= 0 or total_asset <= 0:
        return None, 0, {"reason": "bad_input"}

    max_value = total_asset * max_single_pct
    now_ts = int(time_module.time())

    progress = _layer_progress.setdefault(code, {
        "done": {"W": 0, "D": 0, "H": 0},
        "last_ts": {"W": 0, "D": 0, "H": 0},
    })

    # Priority: W → D → H (more stable layers first)
    for k in ("W", "D", "H"):
        if layer_states.get(k) != "RED":
            progress["done"][k] = 0  # reset when signal drops
            continue

        batches = LAYER_BATCHES.get(k, [])
        done = progress["done"].get(k, 0)
        if done >= len(batches):
            continue

        last_ts = progress["last_ts"].get(k, 0)
        min_itv = LAYER_MIN_INTERVAL.get(k, 300)
        if last_ts and (now_ts - last_ts) < min_itv:
            continue

        batch_ratio = batches[done]
        batch_value = max_value * batch_ratio
        buy_shares = int(batch_value / current_price)
        buy_shares = (buy_shares // 100) * 100  # round to lot
        if buy_shares < 100:
            progress["done"][k] = done + 1  # skip if too small
            continue

        # Commit
        progress["done"][k] = done + 1
        progress["last_ts"][k] = now_ts
        _layer_progress[code] = progress

        return k, buy_shares, {
            "batch_ratio": batch_ratio,
            "batch_value": round(batch_value, 0),
            "batch_idx": done + 1,
            "batch_total": len(batches),
        }

    return None, 0, {"reason": "no_batch_due"}


# ===========================================================================
# Composite: Full stock evaluation
# ===========================================================================

def evaluate_stock(code: str, market_env: str = None) -> Dict[str, Any]:
    """Run full evaluation pipeline on a single stock.

    Returns combined result from all modules.
    """
    if market_env is None:
        market_env = detect_market_env()

    layers = compute_layer_states(code)
    main_score = score_main_rise(code)
    chip = classify_chip_shape(code)
    lifecycle = detect_lifecycle_phase(code, 0, 0, market_env)
    targets = dynamic_profit_targets(code, market_env)

    # Composite signal strength (0-6)
    signal = 0
    if layers["W"] == "RED":
        signal += 2
    if layers["D"] == "RED":
        signal += 2
    if layers["H"] == "RED":
        signal += 1
    if main_score >= 0.7:
        signal += 1
    signal = min(6, signal)

    kelly = compute_kelly_weight(signal)

    return {
        "code": code,
        "market_env": market_env,
        "layers": layers,
        "main_score": main_score,
        "chip_shape": chip,
        "lifecycle": lifecycle,
        "profit_targets": targets,
        "signal_strength": signal,
        "kelly_weight": round(kelly, 4),
    }


# ===========================================================================
# 11. Layer 1: Macro Analysis (盘前宏观分析)
# ===========================================================================

_macro_cache = {"data": None, "ts": 0}


def analyze_macro() -> dict:
    """Layer 1: Pre-market macro analysis.

    Analyzes: global indices (via A50/HK/US proxies from xtdata),
    market breadth, and recent trend to determine today's direction.

    Returns: {
        'direction': 'bullish'|'bearish'|'neutral',
        'confidence': float,
        'position_advice': '满仓'|'七成'|'半仓'|'轻仓'|'空仓',
        'reasons': [str],
        'indices': {...}
    }
    """
    global _macro_cache
    now = time_module.time()
    if _macro_cache["data"] and (now - _macro_cache["ts"]) < 10:  # 10s cache
        return _macro_cache["data"]

    xt = _get_xtdata()
    reasons = []
    indices = {}

    # Analyze key indices
    index_codes = {
        "000001.SH": "上证指数",
        "399001.SZ": "深证成指",
        "399006.SZ": "创业板指",
    }

    bullish_count = 0
    bearish_count = 0

    for code, name in index_codes.items():
        closes = _get_period_closes(code, "1d", 30)
        if not closes or len(closes) < 10:
            continue

        # Use realtime tick / intraday bars for live price
        last = closes[-1]
        if xt:
            # Try get_full_tick first (realtime snapshot)
            try:
                ticks = xt.get_full_tick([code])
                if code in ticks and ticks[code]:
                    tick = ticks[code]
                    rt = float(tick.get("lastPrice", 0) or tick.get("lastClose", 0))
                    if rt > 0:
                        last = rt
            except Exception:
                pass
            # Fallback: 1m or 5m bars
            if last == closes[-1]:
                for tf in ["1m", "5m"]:
                    try:
                        md1m = xt.get_market_data_ex([], [code], period=tf, count=3)
                        if code in md1m and len(md1m[code]) > 0:
                            rt = float(md1m[code]["close"].values[-1])
                            if rt > 0:
                                last = rt
                                break
                    except Exception:
                        continue
            if last != closes[-1]:
                logger.info(f"[macro] {code} realtime={last} vs daily_close={closes[-1]}")
            else:
                logger.warning(f"[macro] {code} no realtime data, using daily close {last}")

        prev_close = closes[-2] if len(closes) >= 2 else closes[-1]
        ma5 = sum(closes[-5:]) / 5
        ma8 = sum(closes[-8:]) / 8 if len(closes) >= 8 else ma5
        ma34 = sum(closes[-34:]) / 34 if len(closes) >= 34 else (sum(closes[-20:]) / 20 if len(closes) >= 20 else ma8)
        chg_1d = (last - prev_close) / prev_close * 100 if prev_close > 0 else 0
        chg_5d = (last - closes[-5]) / closes[-5] * 100 if len(closes) >= 5 else 0

        trend = "bullish" if last > ma5 > ma8 else ("bearish" if last < ma5 < ma8 else "neutral")
        if trend == "bullish":
            bullish_count += 1
        elif trend == "bearish":
            bearish_count += 1

        indices[code] = {
            "name": name, "price": round(last, 2),
            "chg_1d": round(chg_1d, 2), "chg_5d": round(chg_5d, 2),
            "trend": trend, "above_ma34": last > ma34,
        }

    # Market breadth: batch-check how many stocks are above MA34
    # Use daily bars for MA34 history + get_full_tick for realtime current price
    breadth_score = 0.5
    if xt:
        try:
            all_codes = xt.get_stock_list_in_sector("沪深A股") or []
            if all_codes:
                md = xt.get_market_data_ex([], all_codes, period="1d", count=40)
                # Build MA34 dict from daily bars
                ma34_map = {}
                for code, df in md.items():
                    if df is None or len(df) < 34:
                        continue
                    c = df["close"].values
                    ma34_val = float(c[-34:].mean())
                    if ma34_val > 0:
                        ma34_map[code] = ma34_val
                # Get realtime prices via get_full_tick
                rt_prices = {}
                BATCH = 500
                codes_list = list(ma34_map.keys())
                for i in range(0, len(codes_list), BATCH):
                    batch = codes_list[i:i+BATCH]
                    try:
                        ticks = xt.get_full_tick(batch)
                        for code, tick in ticks.items():
                            if tick and isinstance(tick, dict):
                                p = float(tick.get("lastPrice", 0))
                                if p > 0:
                                    rt_prices[code] = p
                    except Exception:
                        continue
                above_count = 0
                valid_count = 0
                for code, ma34_val in ma34_map.items():
                    cur = rt_prices.get(code)
                    if cur is None:
                        # Fallback to daily bar last close
                        df = md.get(code)
                        if df is not None and len(df) > 0:
                            cur = float(df["close"].values[-1])
                    if cur and cur > 0:
                        valid_count += 1
                        if cur > ma34_val:
                            above_count += 1
                if valid_count > 0:
                    breadth_score = above_count / valid_count
                logger.info(f"[macro] breadth: {above_count}/{valid_count} above MA34 = {breadth_score:.1%}")
        except Exception:
            pass

    # Determine direction
    if bullish_count >= 2 and breadth_score >= 0.55:
        direction = "bullish"
        reasons.append(f"主要指数多头({bullish_count}/3)")
        reasons.append(f"市场宽度{breadth_score:.0%}股票站上MA34")
    elif bearish_count >= 2 and breadth_score <= 0.40:
        direction = "bearish"
        reasons.append(f"主要指数空头({bearish_count}/3)")
        reasons.append(f"仅{breadth_score:.0%}股票站上MA34")
    else:
        direction = "neutral"
        reasons.append("指数方向不一致或宽度中性")

    # Position advice
    pos_map = {
        "bullish": ("七成", 0.7) if breadth_score < 0.65 else ("满仓", 0.8),
        "neutral": ("半仓", 0.5),
        "bearish": ("轻仓", 0.2) if breadth_score > 0.35 else ("空仓", 0.1),
    }
    advice, conf = pos_map.get(direction, ("半仓", 0.5))

    result = {
        "direction": direction,
        "confidence": round(conf, 2),
        "position_advice": advice,
        "breadth_score": round(breadth_score, 3),
        "reasons": reasons,
        "indices": indices,
    }
    _macro_cache = {"data": result, "ts": now}
    logger.info(f"[macro] Direction={direction}, advice={advice}, breadth={breadth_score:.1%}")
    return result


# ===========================================================================
# 12. Layer 2: Sector Rotation (行业轮动分析)
# ===========================================================================

_sector_cache = {"data": None, "ts": 0}

# Key A-share sector ETFs as sector proxies
SECTOR_ETFS = {
    "512010.SH": "医药", "512000.SH": "券商", "512800.SH": "银行",
    "515030.SH": "新能源车", "515790.SH": "光伏", "159915.SZ": "创业板",
    "510300.SH": "沪深300", "512660.SH": "军工", "512480.SH": "半导体",
    "515880.SH": "通信", "512690.SH": "酒", "159869.SZ": "游戏",
    "512200.SH": "房地产", "512580.SH": "环保", "515050.SH": "5G",
    "159766.SZ": "旅游", "512170.SH": "医疗", "512760.SH": "芯片",
}


def analyze_sectors() -> dict:
    """Layer 2: Sector rotation analysis.

    Ranks sectors by recent momentum, identifies hot/cold sectors.

    Returns: {
        'hot_sectors': [{name, code, chg_5d, chg_1d, momentum}],
        'cold_sectors': [...],
        'recommended_sectors': [str],  # sector names to focus on
        'avoid_sectors': [str],        # sector names to avoid
    }
    """
    global _sector_cache
    now = time_module.time()
    if _sector_cache["data"] and (now - _sector_cache["ts"]) < 30:  # 30s cache
        return _sector_cache["data"]

    xt = _get_xtdata()
    sectors = []
    etf_codes = list(SECTOR_ETFS.keys())

    if xt:
        # Batch-fetch daily data for all ETFs at once
        md_daily = {}
        try:
            md_daily = xt.get_market_data_ex([], etf_codes, period="1d", count=20)
        except Exception:
            pass

        # If no data, try downloading first then retry
        if not md_daily or all(v is None or len(v) < 2 for v in md_daily.values()):
            try:
                for code in etf_codes:
                    xt.download_history_data(code, "1d", "", "")
                md_daily = xt.get_market_data_ex([], etf_codes, period="1d", count=20)
            except Exception as e:
                logger.warning(f"[sector] download fallback failed: {e}")
        logger.info(f"[sector] ETF data: {sum(1 for v in md_daily.values() if v is not None and len(v)>=2)} of {len(etf_codes)} have data")

        # Also try 1m bar for realtime price
        md_1m = {}
        try:
            md_1m = xt.get_market_data_ex([], etf_codes, period="1m", count=1)
        except Exception:
            pass

        for code, name in SECTOR_ETFS.items():
            df = md_daily.get(code)
            if df is None or len(df) < 5:
                continue

            closes = df["close"].values.astype(float).tolist()

            # Use 1m bar for realtime last price
            last = closes[-1]
            df_1m = md_1m.get(code)
            if df_1m is not None and len(df_1m) > 0:
                rt = float(df_1m["close"].values[-1])
                if rt > 0:
                    last = rt

            prev = closes[-2] if len(closes) >= 2 else closes[-1]
            if prev <= 0:
                continue
            chg_1d = (last - prev) / prev * 100
            chg_3d = (last - closes[-4]) / closes[-4] * 100 if len(closes) >= 4 and closes[-4] > 0 else 0
            chg_5d = (last - closes[-6]) / closes[-6] * 100 if len(closes) >= 6 and closes[-6] > 0 else 0

            # Momentum = weighted: 50% 1d + 30% 3d + 20% 5d
            momentum = chg_1d * 0.5 + chg_3d * 0.3 + chg_5d * 0.2

            vols = df["volume"].values.astype(float).tolist() if "volume" in df.columns else []
            vol_ratio = 1.0
            if len(vols) >= 6:
                vol_ratio = sum(vols[-3:]) / max(sum(vols[-6:-3]), 1e-6)

            sectors.append({
                "code": code, "name": name,
                "chg_1d": round(chg_1d, 2), "chg_3d": round(chg_3d, 2),
                "chg_5d": round(chg_5d, 2), "momentum": round(momentum, 3),
                "vol_ratio": round(vol_ratio, 2),
            })

    # Sort by momentum
    sectors.sort(key=lambda x: x["momentum"], reverse=True)

    hot = [s for s in sectors if s["momentum"] > 0.5][:5]
    cold = [s for s in sectors if s["momentum"] < -0.5][-5:]
    cold.reverse()

    result = {
        "hot_sectors": hot,
        "cold_sectors": cold,
        "recommended_sectors": [s["name"] for s in hot[:3]],
        "avoid_sectors": [s["name"] for s in cold[:3]],
        "all_sectors": sectors,
    }
    _sector_cache = {"data": result, "ts": now}

    logger.info(f"[sector] {len(sectors)} sectors analyzed, hot={len(hot)} cold={len(cold)}")
    if hot:
        logger.info(f"[sector] Hot: {', '.join(s['name'] for s in hot[:3])}")
    if cold:
        logger.info(f"[sector] Cold: {', '.join(s['name'] for s in cold[:3])}")
    return result


# ===========================================================================
# 13. Sentiment Analysis (市场情绪分析 — Layer 3)
# ===========================================================================

_sentiment_cache = {"data": None, "ts": 0}
_prev_close_cache = {"data": {}, "date": ""}  # daily cache for yesterday's closes


def _diagnose_lastclose():
    """One-time diagnostic: compare QMT lastClose with eastmoney actual data
    to find why advances/declines are wrong."""
    import requests
    xt = _get_xtdata()
    if not xt:
        return
    try:
        # Fetch 20 sample stocks from eastmoney: f2=price, f3=chg%, f12=code, f13=market
        url = ("https://push2.eastmoney.com/api/qt/clist/get?"
               "pn=1&pz=20&po=1&np=1&fltt=2&invt=2&fid=f12"
               "&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23"
               "&fields=f2,f3,f12,f13,f14"
               "&ut=bd1d9ddb04089700cf9c27f6f7426281")
        r = requests.get(url, timeout=5)
        data = r.json()
        stocks = data.get("data", {}).get("diff", [])
        if not stocks:
            logger.warning("[diagnose] eastmoney returned no data")
            return
        # Compare each stock
        mismatches = []
        for s in stocks[:20]:
            code_num = str(s.get("f12", ""))
            market = s.get("f13", 0)  # 0=SZ, 1=SH
            suffix = ".SH" if market == 1 else ".SZ"
            code = code_num + suffix
            em_price = s.get("f2", 0)  # eastmoney current price
            em_chg = s.get("f3", 0)    # eastmoney change %
            name = s.get("f14", "")
            # Get QMT tick for same stock
            try:
                ticks = xt.get_full_tick([code])
                tick = ticks.get(code, {})
                qmt_last = float(tick.get("lastPrice", 0))
                qmt_close = float(tick.get("lastClose", 0))
                qmt_chg = (qmt_last - qmt_close) / qmt_close * 100 if qmt_close > 0 else 0
                # Also get instrument detail PreClose
                detail = xt.get_instrument_detail(code)
                inst_pc = float(detail.get("PreClose", 0)) if detail else 0
                mismatches.append(
                    f"{code}({name}): EM={em_chg:+.1f}% QMT={qmt_chg:+.1f}% "
                    f"| tick.lastClose={qmt_close} inst.PreClose={inst_pc} "
                    f"| EM_price={em_price} QMT_price={qmt_last}"
                )
            except Exception as e:
                mismatches.append(f"{code}: error {e}")
        for line in mismatches:
            logger.info(f"[diagnose] {line}")
    except Exception as e:
        logger.warning(f"[diagnose] failed: {e}")


def _ensure_instrument_cache() -> dict:
    """Cache instrument details (PreClose, UpStopPrice, DownStopPrice) per day.
    Uses get_instrument_detail which reads local data — fast, no download needed."""
    global _prev_close_cache
    from datetime import date
    today = date.today().isoformat()
    if _prev_close_cache["date"] == today and _prev_close_cache["data"]:
        return _prev_close_cache["data"]

    xt = _get_xtdata()
    if not xt:
        return {}

    all_codes = xt.get_stock_list_in_sector("沪深A股")
    if not all_codes:
        return {}

    instruments = {}  # code -> {pre_close, up_limit, dn_limit}
    for code in all_codes:
        try:
            detail = xt.get_instrument_detail(code)
            if not detail:
                continue
            pc = float(detail.get("PreClose", 0))
            up = float(detail.get("UpStopPrice", 0))
            dn = float(detail.get("DownStopPrice", 0))
            if pc > 0:
                instruments[code] = {"pre_close": pc, "up_limit": up, "dn_limit": dn}
        except Exception:
            continue
    logger.info(f"[sentiment] cached {len(instruments)} instrument details for {today}")
    _prev_close_cache = {"data": instruments, "date": today}
    return instruments


def analyze_sentiment() -> dict:
    """Layer 3: Market sentiment analysis.

    Analyzes: limit-up/down counts, advance/decline ratio, average turnover,
    volume vs 5-day average, to produce a fear/greed gauge.

    Returns: {
        'gauge': 'extreme_fear'|'fear'|'neutral'|'greed'|'extreme_greed',
        'score': float (0-100, 0=extreme fear, 100=extreme greed),
        'indicators': {...},
        'signal': str,
    }
    """
    global _sentiment_cache
    now = time_module.time()
    if _sentiment_cache["data"] and (now - _sentiment_cache["ts"]) < 10:  # 10s cache
        return _sentiment_cache["data"]

    xt = _get_xtdata()
    indicators = {}
    score_parts = []

    # 1. Advance/Decline ratio (涨跌比) — realtime via get_full_tick
    try:
        all_codes = xt.get_stock_list_in_sector("沪深A股") if xt else []
        advances, declines, unchanged = 0, 0, 0
        limit_up, limit_down = 0, 0
        counted = 0

        if all_codes and xt:
            # Step 1: Get instrument details (PreClose, UpStopPrice, DownStopPrice)
            # from get_instrument_detail — local data, no download, fast & accurate
            instruments = _ensure_instrument_cache()
            _diag_limit_up_samples = []
            _diag_stale = 0
            from datetime import date as _date
            today_str = _date.today().strftime("%Y%m%d")

            # Step 2: Get realtime prices via get_full_tick
            # (requires subscribe_whole_quote at startup for accurate data)
            BATCH = 500
            for i in range(0, len(all_codes), BATCH):
                batch = all_codes[i:i+BATCH]
                try:
                    ticks = xt.get_full_tick(batch)
                except Exception:
                    continue
                for code, tick in ticks.items():
                    if not tick or not isinstance(tick, dict):
                        continue
                    last_price = float(tick.get("lastPrice", 0))
                    if last_price <= 0:
                        continue
                    # Check tick freshness via timetag
                    try:
                        timetag = float(tick.get("timetag", 0))
                    except (ValueError, TypeError):
                        timetag = 0
                    if timetag > 0:
                        import time as _time
                        tick_date = _time.strftime("%Y%m%d", _time.localtime(timetag / 1000))
                        if tick_date != today_str:
                            _diag_stale += 1
                            continue  # skip stale ticks (not from today)
                    # With subscribe_whole_quote active, tick's lastClose is accurate
                    prev_close = float(tick.get("lastClose", 0))
                    if prev_close <= 0:
                        continue
                    chg = (last_price - prev_close) / prev_close * 100
                    counted += 1
                    if chg > 0.1:
                        advances += 1
                    elif chg < -0.1:
                        declines += 1
                    else:
                        unchanged += 1
                    # Use exact UpStopPrice/DownStopPrice from instrument cache
                    # (handles ST 5%, ChiNext/STAR 20%, North 30%, main 10% correctly)
                    inst = instruments.get(code)
                    if inst and inst["up_limit"] > 0:
                        if last_price >= inst["up_limit"]:
                            limit_up += 1
                            if len(_diag_limit_up_samples) < 5:
                                _diag_limit_up_samples.append(f"{code}:{last_price}/{inst['up_limit']}(pc={inst['pre_close']})")
                        elif inst["dn_limit"] > 0 and last_price <= inst["dn_limit"]:
                            limit_down += 1
            # Diagnostic logging
            if _diag_limit_up_samples:
                logger.info(f"[sentiment] limit_up samples: {_diag_limit_up_samples}")
            logger.info(f"[sentiment] counted {counted}/{len(all_codes)}, adv={advances} dec={declines} limit_up={limit_up} limit_down={limit_down} (inst={len(instruments)}, stale_skipped={_diag_stale})")

        total = advances + declines + unchanged
        ad_ratio = advances / max(declines, 1)
        indicators["advances"] = advances
        indicators["declines"] = declines
        indicators["unchanged"] = unchanged
        indicators["ad_ratio"] = round(ad_ratio, 2)
        indicators["limit_up"] = limit_up
        indicators["limit_down"] = limit_down
        indicators["sample_size"] = total

        # AD ratio scoring: >2 = greed, <0.5 = fear
        if ad_ratio >= 3:
            score_parts.append(90)
        elif ad_ratio >= 2:
            score_parts.append(75)
        elif ad_ratio >= 1.2:
            score_parts.append(60)
        elif ad_ratio >= 0.8:
            score_parts.append(50)
        elif ad_ratio >= 0.5:
            score_parts.append(30)
        else:
            score_parts.append(15)

        # Limit up/down scoring
        net_limit = limit_up - limit_down
        if net_limit >= 10:
            score_parts.append(85)
        elif net_limit >= 3:
            score_parts.append(65)
        elif net_limit >= -3:
            score_parts.append(50)
        elif net_limit >= -10:
            score_parts.append(30)
        else:
            score_parts.append(10)

    except Exception as e:
        logger.warning(f"[sentiment] AD ratio error: {e}")
        score_parts.extend([50, 50])

    # 2. Volume sentiment (量能情绪)
    try:
        idx_code = "000001.SH"
        if xt:
            md = xt.get_market_data_ex([], [idx_code], period="1d", count=10)
            if idx_code in md:
                vols = md[idx_code]["volume"].values.tolist()
                if len(vols) >= 6:
                    vol_today = vols[-1]
                    vol_avg5 = sum(vols[-6:-1]) / 5
                    vol_ratio = vol_today / max(vol_avg5, 1e-6)
                    indicators["index_vol_ratio"] = round(vol_ratio, 2)

                    if vol_ratio >= 1.8:
                        score_parts.append(80)
                    elif vol_ratio >= 1.3:
                        score_parts.append(65)
                    elif vol_ratio >= 0.8:
                        score_parts.append(50)
                    elif vol_ratio >= 0.5:
                        score_parts.append(30)
                    else:
                        score_parts.append(15)
                else:
                    score_parts.append(50)
        else:
            score_parts.append(50)
    except Exception as e:
        logger.warning(f"[sentiment] Volume error: {e}")
        score_parts.append(50)

    # 3. Recent momentum (近期动量情绪)
    try:
        idx_closes = _get_period_closes("000001.SH", "1d", 20)
        if idx_closes and len(idx_closes) >= 10:
            chg_3d = (idx_closes[-1] - idx_closes[-4]) / idx_closes[-4] * 100
            chg_10d = (idx_closes[-1] - idx_closes[-10]) / idx_closes[-10] * 100
            indicators["index_chg_3d"] = round(chg_3d, 2)
            indicators["index_chg_10d"] = round(chg_10d, 2)

            # Combined momentum score
            mom_score = 50 + chg_3d * 5 + chg_10d * 2
            mom_score = max(5, min(95, mom_score))
            score_parts.append(mom_score)
        else:
            score_parts.append(50)
    except Exception:
        score_parts.append(50)

    # Aggregate score (0-100)
    final_score = sum(score_parts) / max(len(score_parts), 1)
    final_score = round(max(0, min(100, final_score)), 1)

    # Gauge label
    if final_score >= 80:
        gauge = "extreme_greed"
        signal = "市场极度贪婪，注意高位风险"
    elif final_score >= 60:
        gauge = "greed"
        signal = "市场偏贪婪，可适度参与但控制仓位"
    elif final_score >= 40:
        gauge = "neutral"
        signal = "市场情绪中性，按策略正常操作"
    elif final_score >= 20:
        gauge = "fear"
        signal = "市场偏恐慌，优质标的可能出现低吸机会"
    else:
        gauge = "extreme_fear"
        signal = "市场极度恐慌，关注超跌反弹机会但严控风险"

    result = {
        "gauge": gauge,
        "score": final_score,
        "signal": signal,
        "indicators": indicators,
    }
    _sentiment_cache = {"data": result, "ts": now}
    logger.info(f"[sentiment] {gauge} score={final_score} AD={indicators.get('ad_ratio', '?')} limit_up={indicators.get('limit_up', 0)}")
    return result


# ===========================================================================
# 14. Individual Stock Research (个股深度研报 — Layer 4)
# ===========================================================================

def research_stock(code: str, cost_price: float = 0) -> dict:
    """Layer 4: Deep research report for a single stock.

    Combines: technical analysis, relative strength, volume profile,
    lifecycle phase, chip distribution, and key risk flags.

    Args:
        code: Stock code (e.g., '002902.SZ')
        cost_price: Entry price (0 if not held)

    Returns: Comprehensive research dict with score, recommendation, and analysis.
    """
    xt = _get_xtdata()
    report = {"code": code, "timestamp": datetime.now().strftime("%Y-%m-%d %H:%M")}

    closes = _get_period_closes(code, "1d", 120)
    if not closes or len(closes) < 20:
        report["error"] = "insufficient data"
        return report

    last = closes[-1]
    report["price"] = round(last, 2)

    # --- 1. Trend Analysis (趋势分析 — MA5/MA8/MA34/MA82 体系) ---
    ma5 = sum(closes[-5:]) / 5
    ma8 = sum(closes[-8:]) / 8 if len(closes) >= 8 else ma5
    ma34 = sum(closes[-34:]) / 34 if len(closes) >= 34 else sum(closes[-20:]) / 20
    ma82 = sum(closes[-82:]) / 82 if len(closes) >= 82 else (sum(closes[-60:]) / 60 if len(closes) >= 60 else ma34)

    trend_score = 0
    if last > ma5 > ma8 > ma34 > ma82:
        trend = "strong_bullish"
        trend_score = 95
    elif last > ma5 > ma8 > ma34:
        trend = "bullish"
        trend_score = 75
    elif last > ma34:
        trend = "neutral_up"
        trend_score = 55
    elif last > ma82:
        trend = "neutral"
        trend_score = 40
    elif ma5 < ma8 and last < ma34:
        trend = "bearish"
        trend_score = 20
    elif last < ma5 < ma8 < ma34 < ma82:
        trend = "strong_bearish"
        trend_score = 5
    else:
        trend = "neutral_down"
        trend_score = 30

    report["trend"] = {
        "status": trend,
        "score": trend_score,
        "ma5": round(ma5, 2), "ma8": round(ma8, 2),
        "ma34": round(ma34, 2), "ma82": round(ma82, 2),
        "above_ma34": last > ma34,
        "above_ma82": last > ma82,
    }

    # --- 2. Relative Strength (相对强度 vs 沪深300) ---
    idx_closes = _get_period_closes("000300.SH", "1d", 60)
    rs_score = 50
    if idx_closes and len(idx_closes) >= 20:
        stock_chg_20 = (closes[-1] / closes[-20] - 1) * 100
        idx_chg_20 = (idx_closes[-1] / idx_closes[-20] - 1) * 100
        rs_20d = stock_chg_20 - idx_chg_20

        stock_chg_5 = (closes[-1] / closes[-5] - 1) * 100 if len(closes) >= 5 else 0
        idx_chg_5 = (idx_closes[-1] / idx_closes[-5] - 1) * 100 if len(idx_closes) >= 5 else 0
        rs_5d = stock_chg_5 - idx_chg_5

        rs_score = 50 + rs_20d * 2 + rs_5d * 3
        rs_score = max(0, min(100, rs_score))

        report["relative_strength"] = {
            "rs_5d": round(rs_5d, 2),
            "rs_20d": round(rs_20d, 2),
            "score": round(rs_score, 1),
            "stronger_than_index": rs_20d > 0,
        }
    else:
        report["relative_strength"] = {"score": 50, "note": "index data unavailable"}

    # --- 3. Volume Analysis (量价分析) ---
    vol_score = 50
    try:
        if xt:
            md = xt.get_market_data_ex([], [code], period="1d", count=20)
            if code in md:
                vols = md[code]["volume"].values.tolist()
                if len(vols) >= 6:
                    vol_today = vols[-1]
                    vol_avg5 = sum(vols[-6:-1]) / 5
                    vol_avg20 = sum(vols) / len(vols)
                    vol_ratio_5 = vol_today / max(vol_avg5, 1e-6)
                    vol_ratio_20 = vol_today / max(vol_avg20, 1e-6)

                    report["volume"] = {
                        "today": int(vol_today),
                        "avg5": int(vol_avg5),
                        "ratio_5d": round(vol_ratio_5, 2),
                        "ratio_20d": round(vol_ratio_20, 2),
                        "expanding": vol_ratio_5 > 1.3,
                        "shrinking": vol_ratio_5 < 0.7,
                    }

                    if vol_ratio_5 >= 2.0:
                        vol_score = 80
                    elif vol_ratio_5 >= 1.3:
                        vol_score = 65
                    elif vol_ratio_5 >= 0.7:
                        vol_score = 50
                    else:
                        vol_score = 30
    except Exception:
        pass

    # --- 4. Volatility & Risk (波动率) ---
    if len(closes) >= 15:
        returns = [(closes[i] - closes[i-1]) / closes[i-1] for i in range(max(1, len(closes)-14), len(closes))]
        atr_pct = np.std(returns) * 100
        max_dd = 0
        peak = closes[-20]
        for p in closes[-20:]:
            if p > peak:
                peak = p
            dd = (peak - p) / peak
            if dd > max_dd:
                max_dd = dd

        report["risk"] = {
            "daily_volatility": round(atr_pct, 2),
            "max_drawdown_20d": round(max_dd * 100, 2),
            "high_volatility": atr_pct > 3.0,
        }

    # --- 5. Position Analysis (持仓分析，仅持仓时) ---
    if cost_price > 0:
        pnl_pct = (last - cost_price) / cost_price * 100
        report["position"] = {
            "cost_price": round(cost_price, 2),
            "pnl_pct": round(pnl_pct, 2),
            "profitable": pnl_pct > 0,
        }

    # --- 5b. Trading Targets (买卖点 — 与策略引擎同步) ---
    targets = {}
    try:
        # ATR-based stop-loss (mirrors position_manager: 2*ATR, capped at stop_loss_pct=5%)
        if len(closes) >= 15:
            atr_vals = [abs(closes[i] - closes[i-1]) for i in range(max(1, len(closes)-14), len(closes))]
            atr_14 = sum(atr_vals) / len(atr_vals)
            atr_stop_pct = min((2 * atr_14 / last) * 100, 5.0)
        else:
            atr_14 = 0
            atr_stop_pct = 5.0

        if cost_price > 0:
            # Held position: targets based on cost
            targets["stop_loss"] = round(cost_price * (1 - atr_stop_pct / 100), 2)
            targets["stop_loss_pct"] = round(atr_stop_pct, 1)
            targets["tp1"] = round(cost_price * 1.08, 2)   # staged sell 1: +8%
            targets["tp2"] = round(cost_price * 1.15, 2)   # staged sell 2: +15%
            targets["breakeven"] = round(cost_price, 2)
        else:
            # Not held: show entry zone based on MA support
            targets["entry_zone_low"] = round(min(ma10, ma20), 2)
            targets["entry_zone_high"] = round(ma5, 2)
            targets["stop_loss_if_buy"] = round(last * (1 - atr_stop_pct / 100), 2)
            targets["stop_loss_pct"] = round(atr_stop_pct, 1)

        # MA support/resistance levels
        targets["ma_support"] = round(ma20, 2)
        targets["ma_resistance"] = round(ma60, 2) if ma60 > last else None
        targets["atr_14"] = round(atr_14, 2) if atr_14 > 0 else None

        # Hourly M5/M8/M34 trend signal (same as position_manager exit engine)
        xt = _get_xtdata()
        if xt:
            md_h = xt.get_market_data_ex([], [code], period="60m", count=40)
            if code in md_h:
                hcloses = md_h[code]["close"].values.tolist()
                if len(hcloses) >= 9:
                    h_ma5 = sum(hcloses[-5:]) / 5
                    h_ma8 = sum(hcloses[-8:]) / 8
                    targets["hourly_ma5"] = round(h_ma5, 2)
                    targets["hourly_ma8"] = round(h_ma8, 2)
                    targets["hourly_trend"] = "bullish" if h_ma5 >= h_ma8 else "bearish"
                    if len(hcloses) >= 35:
                        h_ma34 = sum(hcloses[-34:]) / 34
                        targets["hourly_ma34"] = round(h_ma34, 2)
                        if h_ma5 < h_ma34:
                            targets["hourly_trend"] = "danger"  # M5<M34 forced exit zone
    except Exception:
        pass
    if targets:
        report["targets"] = targets

    # --- 6. Lifecycle & Chip (生命周期+筹码) ---
    try:
        env = detect_market_env()
        lifecycle = detect_lifecycle_phase(code, cost_price, last, env)
        chip = classify_chip_shape(code)
        layers = compute_layer_states(code)
        report["lifecycle"] = lifecycle.get("phase", "unknown")
        report["chip"] = chip
        report["layers"] = {k: v for k, v in layers.items() if k != "meta"}
    except Exception:
        pass

    # --- 7. Composite Score & Recommendation ---
    # Lifecycle adjustment: penalize distribution/markdown, reward accumulation/markup
    lifecycle_phase = report.get("lifecycle", "unknown")
    lifecycle_adj = 0
    if lifecycle_phase == "distribution":
        lifecycle_adj = -20  # 出货阶段：主力出逃，大幅扣分
    elif lifecycle_phase == "markdown":
        lifecycle_adj = -15  # 下跌阶段
    elif lifecycle_phase == "shakeout":
        lifecycle_adj = -5   # 洗盘阶段：短期风险
    elif lifecycle_phase == "accumulation":
        lifecycle_adj = 10   # 吸筹阶段：长期利好
    elif lifecycle_phase == "markup":
        lifecycle_adj = 5    # 拉升阶段：动能延续

    risk_score = 100 - report.get("risk", {}).get("max_drawdown_20d", 20)
    composite = (trend_score * 0.30 + rs_score * 0.20 + vol_score * 0.15 + risk_score * 0.15 + max(0, 50 + lifecycle_adj) * 0.20)
    composite = round(max(0, min(100, composite)), 1)

    if composite >= 75:
        recommendation = "strong_buy"
        advice = "综合评分优秀，可积极参与"
    elif composite >= 60:
        recommendation = "buy"
        advice = "综合评分良好，可适度买入"
    elif composite >= 45:
        recommendation = "hold"
        advice = "综合评分中性，持仓观望为主"
    elif composite >= 30:
        recommendation = "reduce"
        advice = "综合评分偏弱，建议减仓"
    else:
        recommendation = "sell"
        advice = "综合评分较差，建议卖出"

    # Lifecycle override: distribution/markdown forces downgrade regardless of score
    if lifecycle_phase == "distribution" and recommendation in ("strong_buy", "buy", "hold"):
        recommendation = "reduce"
        advice = "出货阶段，主力资金流出，建议减仓"
    elif lifecycle_phase == "markdown" and recommendation in ("strong_buy", "buy"):
        recommendation = "sell"
        advice = "下跌阶段，建议卖出止损"

    # Holding adjustment: if already held, cap at 'hold' (don't recommend adding more during late markup)
    is_held = cost_price > 0
    if is_held and recommendation in ("strong_buy", "buy"):
        if lifecycle_phase == "markup" and trend_score >= 70:
            recommendation = "hold"
            advice = "已持仓+拉升阶段，持有观望不宜追加"
        elif lifecycle_phase in ("distribution", "shakeout"):
            pass  # already handled above or keep as-is

    report["composite_score"] = composite
    report["recommendation"] = recommendation
    report["advice"] = advice

    # --- 8. Risk Flags (风险提示) ---
    flags = []
    if report.get("risk", {}).get("high_volatility"):
        flags.append("高波动率")
    if report.get("risk", {}).get("max_drawdown_20d", 0) > 15:
        flags.append(f"近20日最大回撤{report['risk']['max_drawdown_20d']:.1f}%")
    if trend in ("bearish", "strong_bearish"):
        flags.append("趋势偏空")
    if report.get("relative_strength", {}).get("rs_20d", 0) < -5:
        flags.append("弱于大盘")
    if report.get("volume", {}).get("shrinking"):
        flags.append("缩量")
    if lifecycle_phase == "distribution":
        flags.append("出货阶段")
    elif lifecycle_phase == "markdown":
        flags.append("下跌阶段")
    report["risk_flags"] = flags

    logger.info(f"[research] {code} score={composite} rec={recommendation} flags={flags}")
    return report


# ===========================================================================
# 15. Master Analysis (Qwen LLM 顶层综合分析 — Layer 5)
# ===========================================================================

_master_cache = {"data": None, "ts": 0}


def master_analysis(positions: list = None) -> dict:
    """Layer 5: LLM-powered master analysis synthesizing all layers.

    Calls Qwen to produce an actionable trading report from:
    - Layer 1: Macro direction
    - Layer 2: Sector rotation
    - Layer 3: Market sentiment
    - Layer 4: Position research (if positions provided)
    """
    import json as _json
    import os
    import httpx

    global _master_cache
    now = time_module.time()
    if _master_cache["data"] and (now - _master_cache["ts"]) < 600:
        return _master_cache["data"]

    api_key = os.getenv("QWEN_API_KEY", "")
    model = os.getenv("QWEN_MODEL", "qwen-max")
    base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")

    if not api_key:
        return {"error": "QWEN_API_KEY not configured", "hint": "Set QWEN_API_KEY in .env"}

    # Gather all layer data
    macro = analyze_macro()
    sectors = analyze_sectors()
    sentiment = analyze_sentiment()
    env = detect_market_env()

    # Position research (top 6)
    pos_summaries = []
    if positions:
        for p in positions[:6]:
            r = research_stock(p.get("code", ""), p.get("cost_price", 0))
            pos_summaries.append({
                "code": r.get("code"), "price": r.get("price"),
                "score": r.get("composite_score"), "rec": r.get("recommendation"),
                "trend": r.get("trend", {}).get("status"),
                "rs": r.get("relative_strength", {}).get("rs_20d"),
                "flags": r.get("risk_flags", []),
                "pnl_pct": r.get("position", {}).get("pnl_pct"),
            })

    data_block = _json.dumps({
        "market_env": env,
        "macro": {"direction": macro["direction"], "confidence": macro["confidence"],
                  "position_advice": macro["position_advice"], "reasons": macro.get("reasons", [])},
        "hot_sectors": sectors.get("recommended_sectors", []),
        "avoid_sectors": sectors.get("avoid_sectors", []),
        "sentiment": {"gauge": sentiment["gauge"], "score": sentiment["score"],
                      "signal": sentiment["signal"],
                      "ad_ratio": sentiment["indicators"].get("ad_ratio"),
                      "limit_up": sentiment["indicators"].get("limit_up", 0),
                      "limit_down": sentiment["indicators"].get("limit_down", 0)},
        "positions": pos_summaries,
    }, ensure_ascii=False, indent=2)

    prompt = f"""你是Q8bot量化AI首席分析师。基于以下实时市场数据，生成今日操作建议。

## 实时数据
{data_block}

## 要求
1. **市场判断** (1-2句): 今日大盘方向和信心度
2. **仓位建议**: 总仓位百分比 + 理由
3. **板块策略**: 重点关注/回避哪些板块
4. **持仓操作** (逐只): 对每个持仓给出 持有/加仓/减仓/清仓 建议 + 一句理由
5. **风险提示**: 今日需要警惕的风险

请用简洁中文回答，每部分2-3句话即可。"""

    try:
        resp = httpx.post(
            f"{base_url}/chat/completions",
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
            json={"model": model, "messages": [
                {"role": "system", "content": "你是专业的A股量化分析师，擅长技术分析和风险管理。回答简洁、可操作。"},
                {"role": "user", "content": prompt},
            ], "temperature": 0.3, "max_tokens": 1500},
            timeout=60.0,
        )
        resp.raise_for_status()
        data = resp.json()
        content = data["choices"][0]["message"]["content"]
        usage = data.get("usage", {})

        result = {
            "analysis": content,
            "model": model,
            "input_data": {
                "market_env": env,
                "macro_direction": macro["direction"],
                "sentiment_gauge": sentiment["gauge"],
                "sentiment_score": sentiment["score"],
                "positions_analyzed": len(pos_summaries),
            },
            "usage": {"prompt_tokens": usage.get("prompt_tokens", 0),
                      "completion_tokens": usage.get("completion_tokens", 0)},
        }
        _master_cache = {"data": result, "ts": now}
        logger.info(f"[master] analysis complete: model={model} env={env} tokens={usage.get('total_tokens', 0)}")
        return result

    except Exception as e:
        logger.error(f"[master] Qwen call failed: {e}")
        return {"error": str(e), "model": model}


# ===========================================================================
# 16. Premarket Report (综合盘前报告)
# ===========================================================================

def premarket_report() -> dict:
    """Generate comprehensive pre-market analysis combining Layer 1+2.

    Called once before market open, provides actionable guidance.
    """
    macro = analyze_macro()
    sectors = analyze_sectors()
    env = detect_market_env()

    return {
        "market_env": env,
        "macro": macro,
        "sectors": sectors,
        "summary": {
            "direction": macro["direction"],
            "position_advice": macro["position_advice"],
            "hot_sectors": sectors.get("recommended_sectors", []),
            "avoid_sectors": sectors.get("avoid_sectors", []),
        },
    }
