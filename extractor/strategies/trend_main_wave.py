"""
主升浪趋势策略 (Main Wave Trend Strategy)

从生产 QMT 策略 (main.py, 41789行) 提取核心打分逻辑，
独立于 QMT ContextInfo，可由 Extractor Go API 调度。

核心算法来源: main.py score_main_rise_candidate() (line 24475)

四维评分 [0,1]:
  1) 趋势+位置 (50%): MA20/MA30多头排列 + 60日价格分位
  2) 近期涨速 (20%): 3日/10日弹性比
  3) 当日量价 (30%): 涨幅分档 + 放量程度
  4) 波动率约束: ATR14惩罚乘数
  5) 预备形态加分: 缩量整理+均线托底

与 Claw AI Agent 集成:
  量化打分 → Extractor API → Claw Agent 二次确认 → 确认后下单
"""

import logging
import math
from typing import Dict, List, Optional, Tuple

import numpy as np

from .base import BaseStrategy, Signal

logger = logging.getLogger("trend_main_wave")


# ===== 从 main.py 提取的核心工具函数 =====

def sma(data, period: int) -> float:
    """Simple Moving Average."""
    if not data or len(data) < period:
        return float(np.mean(data)) if data is not None and len(data) > 0 else 0.0
    return float(np.mean(data[-period:]))


def pct_rank(series, value) -> float:
    """百分位排名 [0,1]。"""
    try:
        arr = np.array(series, dtype=float)
        arr = arr[~np.isnan(arr)]
        if len(arr) == 0:
            return 0.5
        return float(np.sum(arr <= value) / len(arr))
    except Exception:
        return 0.5


def compute_atr_norm(highs, lows, closes, n=14) -> float:
    """ATR归一化：ATR / 最新收盘价。"""
    try:
        h = np.array(highs, dtype=float)
        l = np.array(lows, dtype=float)
        c = np.array(closes, dtype=float)
        if len(c) < n + 1:
            return 0.05
        tr_list = []
        for i in range(1, len(c)):
            tr = max(h[i] - l[i], abs(h[i] - c[i - 1]), abs(l[i] - c[i - 1]))
            tr_list.append(tr)
        if not tr_list:
            return 0.05
        atr = float(np.mean(tr_list[-n:]))
        last_c = float(c[-1])
        return atr / last_c if last_c > 0 else 0.05
    except Exception:
        return 0.05


# ===== 市场环境检测 (简化版，从 main.py 提取) =====

def detect_market_env_from_index(index_closes: List[float]) -> str:
    """基于指数收盘价序列判断市场环境。"""
    if not index_closes or len(index_closes) < 20:
        return "sideways"
    arr = np.array(index_closes, dtype=float)
    ma5 = float(np.mean(arr[-5:]))
    ma10 = float(np.mean(arr[-10:]))
    ma20 = float(np.mean(arr[-20:]))
    last = float(arr[-1])

    if last > ma5 > ma10 > ma20:
        # 5日>10日>20日>当前价 → 多头排列
        ret_20d = (last - float(arr[-20])) / float(arr[-20])
        if ret_20d > 0.08:
            return "bull"
        return "sideways"
    elif last < ma5 < ma10 < ma20:
        ret_20d = (last - float(arr[-20])) / float(arr[-20])
        if ret_20d < -0.08:
            return "extreme_bear" if ret_20d < -0.15 else "bear"
        return "sideways"
    return "sideways"


# ===== 核心：主升浪候选打分 (提取自 main.py:24475) =====

def score_main_rise_candidate(
    closes: np.ndarray,
    highs: np.ndarray,
    lows: np.ndarray,
    volumes: np.ndarray,
    market_env: str = "sideways",
    min_bars: int = 40,
) -> Dict:
    """
    对单只股票进行主升浪候选打分。

    从 main.py score_main_rise_candidate() 完整提取，
    去除 QMT ContextInfo 依赖，纯数据输入。

    Returns:
        dict with keys:
            score: float [0,1] — 综合得分
            trend_ok: bool — 均线多头排列
            trend_score: float
            pos_score: float — 价格位置得分
            speed_score: float — 涨速得分
            breakout_score: float — 突破得分
            vol_score: float — 量比得分
            vol_penalty: float — 波动率惩罚
            pre_consolidation_bonus: float — 预备形态加分
            today_change: float — 当日涨幅
            volume_ratio: float — 量比
            reason: str — 人类可读描述
    """
    result = {
        "score": 0.0, "trend_ok": False, "trend_score": 0.0,
        "pos_score": 0.0, "speed_score": 0.0, "breakout_score": 0.0,
        "pre_break_score": 0.0, "vol_score": 0.0, "vol_penalty": 1.0,
        "pre_consolidation_bonus": 0.0, "today_change": 0.0,
        "volume_ratio": 0.0, "reason": "",
    }

    if closes.size < min_bars:
        result["reason"] = f"数据不足({closes.size}<{min_bars})"
        return result

    last_c = float(closes[-1])
    prev_c = float(closes[-2]) if closes.size >= 2 else last_c
    if last_c <= 0 or prev_c <= 0:
        result["reason"] = "价格异常"
        return result

    # 1) 趋势与位置
    ma20 = float(np.mean(closes[-20:])) if closes.size >= 20 else last_c
    ma30 = float(np.mean(closes[-30:])) if closes.size >= 30 else ma20
    ma20_prev = float(np.mean(closes[-21:-1])) if closes.size >= 21 else ma20
    ma30_prev = float(np.mean(closes[-31:-1])) if closes.size >= 31 else ma30

    trend_ok = (last_c > ma20 > ma30) and (ma20 > ma20_prev) and (ma30 >= ma30_prev * 0.998)
    result["trend_ok"] = trend_ok

    pos_rank = pct_rank(closes[-60:] if closes.size >= 60 else closes, last_c)
    if pos_rank < 0.3 or pos_rank > 0.9:
        pos_score = 0.2
    elif 0.4 <= pos_rank <= 0.85:
        pos_score = 1.0
    else:
        pos_score = 0.6

    trend_score = 1.0 if trend_ok else 0.4
    result["trend_score"] = trend_score
    result["pos_score"] = pos_score

    # 2) 近期涨速：3日弹性 vs 10日
    if closes.size >= 10:
        c3 = float(closes[-4]) if closes.size >= 4 else float(closes[0])
        c10 = float(closes[-11]) if closes.size >= 11 else float(closes[0])
        r3 = (last_c - c3) / c3 if c3 > 0 else 0.0
        r10 = (last_c - c10) / c10 if c10 > 0 else 0.0
        speed_ratio = r3 / max(0.0001, r10) if r10 > 0 else r3
    else:
        speed_ratio = (last_c - prev_c) / prev_c

    if speed_ratio <= 0:
        speed_score = 0.0
    elif speed_ratio >= 2.0:
        speed_score = 1.0
    else:
        speed_score = speed_ratio / 2.0
    result["speed_score"] = speed_score

    # 3) 当日量价异动
    today_change = (last_c - prev_c) / prev_c
    result["today_change"] = today_change

    if volumes.size >= 6:
        avg5 = float(np.mean(volumes[-6:-1]))
    else:
        avg5 = 0.0
    cur_vol = float(volumes[-1]) if volumes.size > 0 else 0.0
    vol_ratio = (cur_vol / avg5) if avg5 > 0 and cur_vol > 0 else 0.0
    result["volume_ratio"] = vol_ratio

    # 涨幅分档
    if today_change < 0.03:
        pre_break_score = 0.0
        breakout_score = 0.0
    elif 0.03 <= today_change < 0.05:
        pre_break_score = 0.6
        breakout_score = 0.3
    elif 0.05 <= today_change <= 0.095:
        pre_break_score = 0.4
        breakout_score = 1.0
    elif today_change > 0.12:
        pre_break_score = 0.1
        breakout_score = 0.3
    else:
        pre_break_score = 0.3
        breakout_score = 0.6

    result["pre_break_score"] = pre_break_score
    result["breakout_score"] = breakout_score

    # 放量得分
    if vol_ratio <= 1.0:
        vol_score = 0.0
    elif vol_ratio >= 2.5:
        vol_score = 1.0
    else:
        vol_score = (vol_ratio - 1.0) / 1.5
    result["vol_score"] = vol_score

    # 4) 波动率惩罚
    atr_n = compute_atr_norm(highs, lows, closes, n=14)
    if atr_n <= 0.08:
        vol_penalty = 1.0
    elif atr_n >= 0.18:
        vol_penalty = 0.6
    else:
        vol_penalty = 1.0 - (atr_n - 0.08)
    result["vol_penalty"] = vol_penalty

    # 5) 预备形态识别：缩量整理 + 均线托底
    pre_consolidation_bonus = 0.0
    if trend_score > 0.4 and closes.size >= 20:
        recent_n = min(8, closes.size)
        recent_close = closes[-recent_n:]
        recent_high = highs[-recent_n:]
        recent_low = lows[-recent_n:]
        recent_vol = volumes[-recent_n:]

        rng = (recent_high - recent_low) / np.maximum(recent_low, 1e-6)
        prev_closes = np.concatenate(([closes[-recent_n - 1]], recent_close[:-1]))
        chg = (recent_close - prev_closes) / np.maximum(prev_closes, 1e-6)

        small_body_ratio = float(np.mean(np.abs(chg) <= 0.025))
        small_range_ratio = float(np.mean(rng <= 0.04))

        vol_shrink = False
        if volumes.size >= recent_n + 5:
            pre_avg = float(np.mean(volumes[-recent_n - 5:-recent_n]))
            cur_avg = float(np.mean(recent_vol))
            vol_shrink = (pre_avg > 0 and cur_avg > 0 and cur_avg <= pre_avg * 0.7)

        price_above_ma = last_c >= min(ma20, ma30) * 0.97

        if small_body_ratio >= 0.6 and small_range_ratio >= 0.6 and vol_shrink and price_above_ma:
            pre_consolidation_bonus = 0.10

    result["pre_consolidation_bonus"] = pre_consolidation_bonus

    # 汇总打分 (权重来自 main.py:24691)
    # 根据市场环境调整 pre_break vs breakout 权重
    if market_env in ("bear", "extreme_bear"):
        w_pre, w_break = 0.6, 0.4  # 熊市偏预判
    else:
        w_pre, w_break = 0.4, 0.6  # 默认偏突破

    trend_pos_score = 0.35 * trend_score + 0.15 * pos_score
    speed_total = 0.20 * speed_score
    today_factor = 0.15 * (w_pre * pre_break_score + w_break * breakout_score) + 0.15 * vol_score

    raw_score = (trend_pos_score + speed_total + today_factor + pre_consolidation_bonus) * vol_penalty

    # 熊市加严：提高阈值而非降低标准
    if market_env in ("bear", "extreme_bear"):
        bear_penalty = 0.85 if market_env == "bear" else 0.70
        raw_score *= bear_penalty

    raw_score = max(0.0, min(1.0, raw_score))
    result["score"] = raw_score

    # 构建可读描述
    parts = []
    if trend_ok:
        parts.append("多头排列")
    if vol_ratio >= 1.5:
        parts.append(f"放量VR={vol_ratio:.1f}")
    if today_change >= 0.05:
        parts.append(f"突破涨{today_change:.1%}")
    elif today_change >= 0.03:
        parts.append(f"预判涨{today_change:.1%}")
    if pre_consolidation_bonus > 0:
        parts.append("缩量整理")
    if not parts:
        parts.append(f"得分{raw_score:.2f}")
    result["reason"] = " + ".join(parts)

    return result


# ===== 批量扫描 + 排序 =====

def scan_and_rank(
    stock_data: Dict[str, Dict],
    market_env: str = "sideways",
    min_score: float = 0.60,
    top_n: int = 20,
) -> List[Dict]:
    """
    批量扫描股票池，打分排序，返回 Top N 候选。

    Args:
        stock_data: { "600519.SH": {"close": [...], "high": [...], "low": [...], "volume": [...]} }
        market_env: 市场环境
        min_score: 最低分数阈值
        top_n: 返回前N只

    Returns:
        List of { "code": str, "score": float, "detail": dict, "reason": str }
    """
    results = []
    for code, data in stock_data.items():
        try:
            closes = np.array(data.get("close", []), dtype=float)
            highs = np.array(data.get("high", []), dtype=float)
            lows = np.array(data.get("low", []), dtype=float)
            volumes = np.array(data.get("volume", []), dtype=float)

            detail = score_main_rise_candidate(
                closes, highs, lows, volumes,
                market_env=market_env,
            )

            if detail["score"] >= min_score:
                results.append({
                    "code": code,
                    "score": detail["score"],
                    "detail": detail,
                    "reason": detail["reason"],
                })
        except Exception as e:
            logger.warning(f"[scan] {code} error: {e}")
            continue

    results.sort(key=lambda x: x["score"], reverse=True)
    return results[:top_n]


# ===== Claw AI Agent 集成：构建确认请求 =====

def build_claw_confirmation_prompt(candidates: List[Dict], market_env: str) -> str:
    """
    构建发送给 Claw AI Agent 的确认请求 prompt。

    Claw 的 LLM 将基于量化打分结果 + 互联网新闻/公告 进行二次判断，
    过滤掉潜在利空、财报风险、政策风险的标的。

    Returns:
        str: 发送给 Claw /v1/chat/completions 的 system + user prompt
    """
    env_zh = {
        "bull": "牛市(进攻)", "sideways": "震荡市(中性)",
        "bear": "熊市(防守)", "extreme_bear": "极端熊市(极度防守)",
    }.get(market_env, "震荡市")

    stock_lines = []
    for i, c in enumerate(candidates, 1):
        d = c.get("detail", {})
        line = (
            f"{i}. {c['code']} | 综合分={c['score']:.2f} | "
            f"趋势={'✅多头' if d.get('trend_ok') else '⚠️非多头'} | "
            f"涨幅={d.get('today_change', 0):.1%} | "
            f"量比={d.get('volume_ratio', 0):.1f} | "
            f"理由: {c.get('reason', '')}"
        )
        stock_lines.append(line)

    prompt = f"""你是 StarClaw 量化交易系统的 AI 风控分析师。

当前市场环境: {env_zh}

量化策略(主升浪趋势)筛选出以下候选股票，请你进行二次确认分析：

{chr(10).join(stock_lines)}

请对每只股票进行以下分析：
1. **基本面快检**: 是否有近期财报预警、重大利空公告、ST风险
2. **消息面扫描**: 近3日是否有政策利空、行业利空、高管减持等负面消息
3. **技术面验证**: 量化给出的多头排列+放量突破是否与你了解的走势一致
4. **板块共振**: 所属行业/概念板块当前是否处于活跃状态

请用 JSON 格式回复，每只股票一个对象：
```json
[
  {{
    "code": "600519.SH",
    "action": "confirm" 或 "reject" 或 "reduce",
    "confidence": 0.0-1.0,
    "risk_flags": ["利空描述1", "利空描述2"],
    "suggestion": "具体建议"
  }}
]
```

action 说明:
- confirm: 确认买入，量化信号可靠
- reject: 拒绝买入，发现重大风险
- reduce: 建议减半仓位，存在一定不确定性
"""
    return prompt


def parse_claw_confirmation(response_text: str, candidates: List[Dict]) -> List[Dict]:
    """
    解析 Claw AI Agent 的确认回复，合并到候选列表。

    Returns:
        List[Dict]: 添加了 claw_action, claw_confidence, risk_flags 字段的候选列表
    """
    import json
    import re

    confirmed = []

    # 尝试从回复中提取 JSON
    try:
        # 提取 ```json ... ``` 块
        json_match = re.search(r'```(?:json)?\s*(\[.*?\])\s*```', response_text, re.DOTALL)
        if json_match:
            claw_results = json.loads(json_match.group(1))
        else:
            # 直接尝试解析
            claw_results = json.loads(response_text)
    except Exception:
        logger.warning("[claw] 无法解析AI回复JSON，全部标记为confirm(降级)")
        for c in candidates:
            c["claw_action"] = "confirm"
            c["claw_confidence"] = 0.5
            c["risk_flags"] = ["AI回复解析失败，降级放行"]
            confirmed.append(c)
        return confirmed

    # 构建 code → AI结果映射
    claw_map = {}
    for r in claw_results:
        if isinstance(r, dict) and "code" in r:
            claw_map[r["code"]] = r

    for c in candidates:
        code = c["code"]
        ai = claw_map.get(code, {})
        c["claw_action"] = ai.get("action", "confirm")
        c["claw_confidence"] = float(ai.get("confidence", 0.5))
        c["risk_flags"] = ai.get("risk_flags", [])
        c["claw_suggestion"] = ai.get("suggestion", "")

        if c["claw_action"] == "confirm":
            confirmed.append(c)
        elif c["claw_action"] == "reduce":
            c["reduce_position"] = True
            confirmed.append(c)
        else:
            logger.info(f"[claw] REJECT {code}: {c['risk_flags']}")

    return confirmed


# ===== 策略类 (BaseStrategy 兼容) =====

class TrendMainWaveStrategy(BaseStrategy):
    """
    主升浪趋势策略 — 提取自生产QMT策略 + Claw AI 二次确认。

    执行流程:
        1. Python 批量获取行情数据
        2. score_main_rise_candidate() 四维打分
        3. scan_and_rank() 筛选 Top N
        4. build_claw_confirmation_prompt() → 发给 Claw AI
        5. parse_claw_confirmation() → 最终确认买入列表
        6. 通过 bridge 下单到 QMT

    Parameters:
        min_score: float — 最低主升浪得分 (default: 0.60)
        top_n: int — 每轮最多选几只 (default: 10)
        stop_loss_pct: float — 止损% (default: 5.0)
        trailing_stop_pct: float — 跟踪止损% (default: 8.0)
        use_claw_confirm: bool — 是否启用Claw AI二次确认 (default: True)
    """

    DEFAULT_PARAMS = {
        "min_score": 0.60,
        "top_n": 10,
        "stop_loss_pct": 5.0,
        "trailing_stop_pct": 8.0,
        "use_claw_confirm": True,
        "claw_api_url": "http://localhost:8080",
    }

    def __init__(self, strategy_id: str, account: str, params: Dict = None):
        merged = {**self.DEFAULT_PARAMS, **(params or {})}
        super().__init__(strategy_id, account, merged)
        self._bars_history: Dict[str, List[dict]] = {}
        self._highest_since_entry: Dict[str, float] = {}
        self._entry_prices: Dict[str, float] = {}
        self._market_env: str = "sideways"

    def on_init(self):
        logger.info(f"[TrendMainWave] init: min_score={self.params['min_score']}, "
                     f"top_n={self.params['top_n']}, claw_confirm={self.params['use_claw_confirm']}")

    def set_market_env(self, env: str):
        self._market_env = env

    def on_bar(self, code: str, bar: dict) -> Optional[Signal]:
        """单票逐bar回调 — 用于持仓管理(止盈止损)。"""
        if code not in self._bars_history:
            self._bars_history[code] = []
        self._bars_history[code].append(bar)
        if len(self._bars_history[code]) > 120:
            self._bars_history[code] = self._bars_history[code][-120:]

        held = self.positions.get(code, 0) > 0
        if held:
            return self._check_exit(code, bar["close"])
        return None

    def scan_candidates(self, stock_data: Dict[str, Dict]) -> List[Dict]:
        """批量扫描，返回主升浪候选列表 (不含Claw确认)。"""
        return scan_and_rank(
            stock_data,
            market_env=self._market_env,
            min_score=self.params["min_score"],
            top_n=self.params["top_n"],
        )

    def build_claw_prompt(self, candidates: List[Dict]) -> str:
        """构建 Claw AI 确认请求。"""
        return build_claw_confirmation_prompt(candidates, self._market_env)

    def apply_claw_confirmation(self, candidates: List[Dict], claw_response: str) -> List[Dict]:
        """应用 Claw AI 确认结果。"""
        return parse_claw_confirmation(claw_response, candidates)

    def _check_exit(self, code: str, price: float) -> Optional[Signal]:
        entry_price = self._entry_prices.get(code, price)
        highest = self._highest_since_entry.get(code, price)

        if price > highest:
            self._highest_since_entry[code] = price
            highest = price

        loss_pct = (price - entry_price) / entry_price * 100
        if loss_pct <= -self.params["stop_loss_pct"]:
            reason = f"固定止损({loss_pct:.1f}%)"
            self._entry_prices.pop(code, None)
            self._highest_since_entry.pop(code, None)
            return Signal(code=code, direction=Signal.SELL, price=price,
                          volume=self.positions.get(code, 0), reason=reason, confidence=0.9)

        drawdown_pct = (highest - price) / highest * 100
        if drawdown_pct >= self.params["trailing_stop_pct"]:
            reason = f"跟踪止损(高点回落{drawdown_pct:.1f}%)"
            self._entry_prices.pop(code, None)
            self._highest_since_entry.pop(code, None)
            return Signal(code=code, direction=Signal.SELL, price=price,
                          volume=self.positions.get(code, 0), reason=reason, confidence=0.85)

        return None

    def record_entry(self, code: str, price: float, volume: int):
        """记录入场信息，用于止盈止损跟踪。"""
        self._entry_prices[code] = price
        self._highest_since_entry[code] = price
        self.positions[code] = self.positions.get(code, 0) + volume

    def on_stop(self):
        logger.info(f"[TrendMainWave] stopped. positions={self.positions}")
