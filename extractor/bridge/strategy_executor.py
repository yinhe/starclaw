"""
Strategy Executor — 完整的 扫描→打分→Claw确认→下单 调度循环。

运行在 Server D (139.224.10.5) 上，由 Go API 触发或定时自动执行。

流程:
  1. 从 QMT 获取全A股日线数据 (xtquant)
  2. 调用 strategies/trend_main_wave.py 批量打分
  3. 发送候选到 Claw AI 二次确认 (via Go API /v1/claw/confirm)
  4. 确认后通过 xtquant 下单
  5. 回调 Go API 记录成交
"""

import logging
import os
import sys
import time as time_module
from datetime import datetime, time
from typing import Dict, List, Optional

import httpx
import numpy as np

# Add parent dir to path for strategies import
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from strategies.trend_main_wave import (
    TrendMainWaveStrategy,
    detect_market_env_from_index,
    scan_and_rank,
    build_claw_confirmation_prompt,
    parse_claw_confirmation,
)
from position_manager import PositionManager

logger = logging.getLogger("executor")

# Go API base URL
GO_API_URL = os.getenv("EXTRACTOR_API_URL", "http://localhost:8097")
# Claw confirm endpoint (on Go API, which proxies to Claw)
CLAW_CONFIRM_URL = f"{GO_API_URL}/v1/claw/confirm"
# Callback URLs
CALLBACK_URL = f"{GO_API_URL}/callback"


class StrategyExecutor:
    """
    完整的策略执行器：扫描→打分→Claw确认→下单。

    支持两种运行模式:
    1. 单次扫描 (scan_once): 由 Go API POST /v1/scan 触发
    2. 定时循环 (run_loop): 每 N 秒自动执行
    """

    def __init__(self, qmt_client, account_manager, go_api_url: str = None):
        self.qmt = qmt_client
        self.accounts = account_manager
        self.go_api_url = go_api_url or GO_API_URL
        self.http = httpx.Client(timeout=90.0)  # 90s for Claw LLM

        # Position manager: tracks holdings, enforces stop-loss/take-profit
        trend_accounts = account_manager.get_accounts_by_group("trend")
        acct_id = trend_accounts[0]["id"] if trend_accounts else "27800348"
        self.positions = PositionManager(qmt_client, account=acct_id)

        self.strategy = TrendMainWaveStrategy(
            strategy_id="trend-main-wave-v1",
            account="trend_group",  # will dispatch to individual accounts
            params={
                "min_score": float(os.getenv("MIN_SCORE", "0.60")),
                "top_n": int(os.getenv("TOP_N", "10")),
                "stop_loss_pct": 5.0,
                "trailing_stop_pct": 8.0,
                "use_claw_confirm": os.getenv("USE_CLAW_CONFIRM", "true").lower() == "true",
            },
        )
        self.strategy.start()
        self._market_env = "sideways"
        self._last_scan_ts = 0

    def detect_market_env(self) -> str:
        """用上证指数判断当前市场环境。"""
        try:
            klines = self.qmt.get_kline("000001.SH", period="1d", count=30)
            if klines:
                closes = [float(k["close"]) for k in klines if k.get("close")]
                if len(closes) >= 20:
                    env = detect_market_env_from_index(closes)
                    self._market_env = env
                    self.strategy.set_market_env(env)
                    logger.info(f"[executor] market env: {env}")
                    return env
        except Exception as e:
            logger.warning(f"[executor] detect_market_env error: {e}")
        return self._market_env

    def fetch_stock_pool(self) -> List[str]:
        """获取股票池（上证A股+深证A股，过滤创业板/科创板/ST）。"""
        try:
            from qmt_client import HAS_XTQUANT
            if HAS_XTQUANT:
                from xtquant import xtdata
                # 获取全A股代码
                sh = xtdata.get_stock_list_in_sector("上证A股") or []
                sz = xtdata.get_stock_list_in_sector("深证A股") or []
                all_codes = list(set(sh + sz))
            else:
                # Mock mode: use a sample pool
                all_codes = [
                    "600519.SH", "000858.SZ", "601318.SH", "600036.SH",
                    "000001.SZ", "600276.SH", "002714.SZ", "601012.SH",
                    "600809.SH", "000568.SZ", "601888.SH", "002415.SZ",
                    "600585.SH", "000333.SZ", "002049.SZ", "601166.SH",
                    "600887.SH", "000651.SZ", "601398.SH", "600030.SH",
                ]
                logger.info(f"[executor] MOCK mode: using {len(all_codes)} sample stocks")
        except Exception as e:
            logger.error(f"[executor] fetch_stock_pool error: {e}")
            all_codes = []

        # Filter out GEM (300/301), STAR (688/689), and BJ (8xx)
        filtered = []
        for code in all_codes:
            plain = code.split(".")[0] if "." in code else code
            if plain.startswith(("300", "301", "688", "689", "8")):
                continue
            # TODO: filter ST stocks via xtdata
            filtered.append(code)

        logger.info(f"[executor] stock pool: {len(filtered)} (from {len(all_codes)} raw)")
        return filtered

    def fetch_market_data(self, codes: List[str], count: int = 60) -> Dict[str, Dict]:
        """
        批量获取日线OHLCV数据。

        Returns:
            { "600519.SH": {"close": [...], "high": [...], "low": [...], "volume": [...]} }
        """
        stock_data = {}
        try:
            from qmt_client import HAS_XTQUANT
            if HAS_XTQUANT:
                from xtquant import xtdata
                # Download first (xtquant requires explicit download)
                xtdata.download_history_data2(codes, period="1d", start_time="", end_time="")
                # Batch fetch
                md = xtdata.get_market_data_ex([], codes, period="1d", count=count)
                for code in codes:
                    if code not in md:
                        continue
                    df = md[code]
                    try:
                        stock_data[code] = {
                            "close": df["close"].tolist(),
                            "high": df["high"].tolist(),
                            "low": df["low"].tolist(),
                            "volume": df["volume"].tolist(),
                        }
                    except Exception:
                        continue
            else:
                # Mock data
                import random
                for code in codes:
                    base = random.uniform(10, 200)
                    closes, highs, lows, volumes = [], [], [], []
                    for _ in range(count):
                        chg = random.uniform(-0.05, 0.06)
                        c = base * (1 + chg)
                        h = c * (1 + random.uniform(0, 0.03))
                        l = c * (1 - random.uniform(0, 0.03))
                        v = random.randint(500000, 50000000)
                        closes.append(round(c, 2))
                        highs.append(round(h, 2))
                        lows.append(round(l, 2))
                        volumes.append(v)
                        base = c
                    stock_data[code] = {
                        "close": closes, "high": highs,
                        "low": lows, "volume": volumes,
                    }
        except Exception as e:
            logger.error(f"[executor] fetch_market_data error: {e}")

        logger.info(f"[executor] market data: {len(stock_data)} stocks with {count} bars each")
        return stock_data

    def request_claw_confirmation(self, candidates: List[Dict]) -> List[Dict]:
        """通过 Go API 发送候选到 Claw AI 二次确认。"""
        if not self.strategy.params.get("use_claw_confirm"):
            logger.info("[executor] Claw confirm disabled, passing all candidates")
            for c in candidates:
                c["claw_action"] = "confirm"
                c["claw_confidence"] = 1.0
            return candidates

        try:
            # Convert to Go API format
            api_candidates = []
            for c in candidates:
                d = c.get("detail", {})
                api_candidates.append({
                    "code": c["code"],
                    "score": c["score"],
                    "trend_ok": d.get("trend_ok", False),
                    "today_change": d.get("today_change", 0),
                    "volume_ratio": d.get("volume_ratio", 0),
                    "reason": c.get("reason", ""),
                })

            resp = self.http.post(
                f"{self.go_api_url}/v1/claw/confirm",
                json={
                    "candidates": api_candidates,
                    "market_env": self._market_env,
                },
            )

            if resp.status_code == 200:
                data = resp.json()
                confirmed = data.get("confirmed", [])
                rejected = data.get("rejected", 0)
                logger.info(f"[executor] Claw confirm: {len(confirmed)} confirmed, {rejected} rejected")

                # Map back to candidates
                confirmed_codes = {c["code"] for c in confirmed if isinstance(c, dict)}
                result = []
                for c in candidates:
                    if c["code"] in confirmed_codes:
                        # Find matching confirmed entry
                        for cc in confirmed:
                            if isinstance(cc, dict) and cc.get("code") == c["code"]:
                                c["claw_action"] = cc.get("claw_action", "confirm")
                                c["claw_confidence"] = cc.get("claw_confidence", 0.5)
                                c["risk_flags"] = cc.get("risk_flags", [])
                                c["reduce_position"] = cc.get("reduce_position", False)
                                break
                        result.append(c)
                return result
            else:
                logger.warning(f"[executor] Claw API error {resp.status_code}, degraded pass-through")
                for c in candidates:
                    c["claw_action"] = "confirm"
                    c["claw_confidence"] = 0.5
                    c["risk_flags"] = [f"API error {resp.status_code}"]
                return candidates

        except Exception as e:
            logger.warning(f"[executor] Claw confirm failed: {e}, degraded pass-through")
            for c in candidates:
                c["claw_action"] = "confirm"
                c["claw_confidence"] = 0.5
                c["risk_flags"] = [f"Error: {e}"]
            return candidates

    def execute_orders(self, confirmed: List[Dict], account: str = None) -> List[Dict]:
        """对确认的候选执行下单。"""
        if not account:
            # Default: use first trend group account
            trend_accounts = self.accounts.get_accounts_by_group("trend")
            if trend_accounts:
                account = trend_accounts[0]["id"]
            else:
                account = "27800348"  # fallback: primary real account

        orders = []
        for c in confirmed:
            code = c["code"]
            score = c.get("score", 0)
            reduce = c.get("reduce_position", False)

            # Get current price
            try:
                quotes = self.qmt.get_quotes([code])
                if quotes:
                    price = float(quotes[0].get("price", 0))
                else:
                    price = 0
            except Exception:
                price = 0

            if price <= 0:
                logger.warning(f"[executor] skip {code}: no price")
                continue

            # Dynamic position sizing: max 10% of available capital per stock
            # Round down to nearest 100 (A-share lot size)
            try:
                acct_info = self.qmt.get_account_info(account)
                available = float(acct_info.get("available", 0))
                max_per_stock_pct = 0.10 if not reduce else 0.05
                max_amount = available * max_per_stock_pct
                volume = int(max_amount / price / 100) * 100  # round to lot
                volume = max(volume, 100)  # minimum 1 lot
                volume = min(volume, 10000)  # cap at 10000 shares
            except Exception:
                volume = 200 if not reduce else 100

            logger.info(
                f"[executor] ORDER: {code} BUY @{price:.2f} x{volume} "
                f"score={score:.2f} claw={c.get('claw_action', 'N/A')} "
                f"conf={c.get('claw_confidence', 0):.2f}"
            )

            try:
                order_id = self.qmt.submit_order(
                    account=account,
                    code=code,
                    direction="buy",
                    price=price,
                    volume=volume,
                    order_type="limit",
                )
                orders.append({
                    "code": code,
                    "price": price,
                    "volume": volume,
                    "order_id": order_id,
                    "score": score,
                    "claw_action": c.get("claw_action"),
                })

                # Callback to Go API
                try:
                    self.http.post(f"{CALLBACK_URL}/strategy_signal", json={
                        "strategy_id": self.strategy.strategy_id,
                        "signal": "buy",
                        "data": {
                            "code": code,
                            "price": price,
                            "volume": volume,
                            "score": score,
                            "order_id": order_id,
                        },
                    })
                except Exception:
                    pass

                # Track entry for stop-loss
                self.strategy.record_entry(code, price, volume)

                # Record in position manager (persistent)
                self.positions.record_buy(
                    code=code, price=price, volume=volume,
                    score=score, order_id=str(order_id),
                    reason=c.get("reason", ""),
                )

            except Exception as e:
                logger.error(f"[executor] order {code} failed: {e}")

        return orders

    def scan_once(self) -> Dict:
        """
        单次完整扫描：市场环境→股票池→打分→Claw确认→下单。
        Returns summary dict.
        """
        t0 = time_module.time()
        logger.info("=" * 60)
        logger.info("[executor] === SCAN START ===")

        # 1. Market environment
        env = self.detect_market_env()

        # 2. Stock pool (exclude already-held stocks)
        codes = self.fetch_stock_pool()
        if not codes:
            return {"error": "empty stock pool", "market_env": env}
        held = self.positions.get_held_codes()
        if held:
            before = len(codes)
            codes = [c for c in codes if c not in held]
            logger.info(f"[executor] filtered {before - len(codes)} held stocks, {len(codes)} remaining")

        # 3. Fetch market data
        stock_data = self.fetch_market_data(codes, count=60)
        if not stock_data:
            return {"error": "no market data", "market_env": env}

        # 4. Score and rank
        candidates = self.strategy.scan_candidates(stock_data)
        logger.info(f"[executor] candidates: {len(candidates)} above min_score")
        if not candidates:
            return {
                "market_env": env,
                "scanned": len(stock_data),
                "candidates": 0,
                "confirmed": 0,
                "orders": 0,
                "elapsed": time_module.time() - t0,
            }

        # Log top candidates
        for i, c in enumerate(candidates[:5]):
            logger.info(f"  #{i+1} {c['code']} score={c['score']:.2f} {c['reason']}")

        # 5. Claw AI confirmation
        confirmed = self.request_claw_confirmation(candidates)
        logger.info(f"[executor] after Claw confirm: {len(confirmed)} stocks")

        # 6. Execute orders
        orders = self.execute_orders(confirmed)

        elapsed = time_module.time() - t0
        logger.info(f"[executor] === SCAN COMPLETE === {elapsed:.1f}s, {len(orders)} orders")
        logger.info("=" * 60)

        self._last_scan_ts = time_module.time()

        return {
            "market_env": env,
            "scanned": len(stock_data),
            "candidates": len(candidates),
            "confirmed": len(confirmed),
            "orders": len(orders),
            "order_details": orders,
            "elapsed": elapsed,
        }

    def is_trading_hours(self) -> bool:
        """检查是否在A股交易时间内。"""
        now = datetime.now().time()
        morning = time(9, 30) <= now <= time(11, 30)
        afternoon = time(13, 0) <= now <= time(14, 57)
        return morning or afternoon

    def run_loop(self, interval_seconds: int = 60):
        """
        定时循环执行策略。

        在交易时间内每 interval_seconds 秒执行一次扫描。
        非交易时间休眠。
        """
        logger.info(f"[executor] Starting scan loop (interval={interval_seconds}s)")

        while True:
            try:
                if self.is_trading_hours():
                    result = self.scan_once()
                    logger.info(f"[executor] loop result: {result.get('orders', 0)} orders, "
                                f"{result.get('candidates', 0)} candidates")
                else:
                    now = datetime.now()
                    # 日终结算检查 (15:30-16:00)
                    if time(15, 30) <= now.time() <= time(16, 0):
                        self._daily_settlement()

                    # 非交易时间每5分钟检查一次
                    time_module.sleep(300)
                    continue

                time_module.sleep(interval_seconds)

            except KeyboardInterrupt:
                logger.info("[executor] Loop stopped by user")
                break
            except Exception as e:
                logger.error(f"[executor] loop error: {e}")
                time_module.sleep(30)

    def _daily_settlement(self):
        """触发日终结算。"""
        try:
            today = datetime.now().strftime("%Y-%m-%d")
            resp = self.http.post(
                f"{self.go_api_url}/v1/settlement/run",
                params={"date": today},
            )
            if resp.status_code == 200:
                logger.info(f"[executor] daily settlement triggered: {resp.json()}")
        except Exception as e:
            logger.warning(f"[executor] settlement trigger failed: {e}")


# === CLI 入口 ===

if __name__ == "__main__":
    import argparse

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
    )

    parser = argparse.ArgumentParser(description="Extractor Strategy Executor")
    parser.add_argument("--mode", choices=["once", "loop"], default="once",
                        help="once=单次扫描, loop=定时循环")
    parser.add_argument("--interval", type=int, default=60,
                        help="循环间隔秒数 (loop模式)")
    parser.add_argument("--go-api", default=None,
                        help="Go API URL (default: http://localhost:8097)")
    args = parser.parse_args()

    from qmt_client import QMTClient
    from account_manager import AccountManager

    qmt = QMTClient()
    accounts = AccountManager(qmt)
    executor = StrategyExecutor(qmt, accounts, go_api_url=args.go_api)

    if args.mode == "once":
        result = executor.scan_once()
        print(f"\n=== Result ===")
        for k, v in result.items():
            if k != "order_details":
                print(f"  {k}: {v}")
    else:
        executor.run_loop(interval_seconds=args.interval)
