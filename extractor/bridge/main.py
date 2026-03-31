"""
Extractor Python Bridge — miniQMT ↔ Go API 桥接层
FastAPI server on :8098, wraps xtquant SDK for QMT interaction.
"""

import logging
import math
import os
import sys

# Ensure current directory is in sys.path (required for embedded Python)
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from qmt_client import QMTClient
from account_manager import AccountManager

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(message)s")
logger = logging.getLogger("bridge")

# Globals
qmt: QMTClient = None
accounts: AccountManager = None


def _json_safe(value):
    if value is None or isinstance(value, (str, bool, int)):
        return value
    if isinstance(value, float):
        return value if math.isfinite(value) else None
    if isinstance(value, dict):
        return {str(k): _json_safe(v) for k, v in value.items()}
    if isinstance(value, (list, tuple, set)):
        return [_json_safe(v) for v in value]
    if hasattr(value, "item"):
        try:
            return _json_safe(value.item())
        except Exception:
            return str(value)
    return str(value)


@asynccontextmanager
async def lifespan(app: FastAPI):
    global qmt, accounts
    qmt = QMTClient()
    accounts = AccountManager(qmt)
    logger.info("🏦 Extractor Bridge starting on :8098")
    yield
    if qmt:
        qmt.disconnect()
    logger.info("Bridge shutdown")


import json as _json

class UTF8JSONResponse(JSONResponse):
    """JSON response that preserves Chinese characters (no ASCII escaping)."""
    def render(self, content) -> bytes:
        return _json.dumps(content, ensure_ascii=False, allow_nan=False, default=str).encode("utf-8")

app = FastAPI(title="Extractor Bridge", lifespan=lifespan, default_response_class=UTF8JSONResponse)


# --- Health ---

@app.get("/health")
def health():
    connected = qmt.is_connected() if qmt else False
    return {"status": "ok", "qmt_connected": connected}


# --- Order endpoints ---

class SubmitOrderReq(BaseModel):
    account: str
    code: str
    direction: str  # buy, sell
    price: float
    volume: int
    order_type: str = "limit"


@app.post("/order/submit")
def submit_order(req: SubmitOrderReq):
    try:
        order_id = qmt.submit_order(
            account=req.account,
            code=req.code,
            direction=req.direction,
            price=req.price,
            volume=req.volume,
            order_type=req.order_type,
        )
        return {"qmt_order_id": str(order_id), "status": "submitted"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


class CancelOrderReq(BaseModel):
    account: str
    order_id: str


@app.post("/order/cancel")
def cancel_order(req: CancelOrderReq):
    try:
        qmt.cancel_order(req.account, req.order_id)
        return {"status": "cancelled"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# --- Account endpoints ---

@app.get("/account/info")
def account_info(account: str):
    try:
        info = qmt.get_account_info(account)
        return info
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/account/positions")
def account_positions(account: str):
    try:
        positions = qmt.get_positions(account)
        return positions
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# --- Market data endpoints ---

@app.get("/market/quote")
def market_quote(codes: str):
    """Get real-time quotes. codes: comma-separated stock codes."""
    try:
        code_list = [c.strip() for c in codes.split(",")]
        quotes = qmt.get_quotes(code_list)
        return quotes
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/market/kline")
def market_kline(code: str, period: str = "1d", count: int = 100):
    """Get K-line data."""
    try:
        klines = qmt.get_kline(code, period, count)
        return klines
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# --- Strategy endpoints ---

class StartStrategyReq(BaseModel):
    strategy_id: str
    account: str
    params: dict = {}


@app.post("/strategy/start")
def start_strategy(req: StartStrategyReq):
    # TODO: load strategy class, instantiate, start execution loop
    logger.info(f"Starting strategy {req.strategy_id} on {req.account}")
    return {"status": "started"}


class StopStrategyReq(BaseModel):
    strategy_id: str


@app.post("/strategy/stop")
def stop_strategy(req: StopStrategyReq):
    # TODO: stop strategy execution loop
    logger.info(f"Stopping strategy {req.strategy_id}")
    return {"status": "stopped"}


# --- Scan endpoints (triggered by Go API) ---

_executor = None
_last_scan_result = None


def _get_executor():
    global _executor
    if _executor is None:
        from strategy_executor import StrategyExecutor
        _executor = StrategyExecutor(qmt, accounts)
    return _executor


class ScanReq(BaseModel):
    min_score: float = 0.0
    top_n: int = 0


@app.post("/scan")
def trigger_scan(req: ScanReq = None):
    """触发一次完整的 扫描→打分→返回候选 流程。Go API 通过此端点触发。"""
    global _last_scan_result
    try:
        executor = _get_executor()
        # Override params if provided
        if req and req.min_score > 0:
            executor.strategy.params["min_score"] = req.min_score
        if req and req.top_n > 0:
            executor.strategy.params["top_n"] = req.top_n

        result = executor.scan_once()
        safe_result = _json_safe(result)
        _last_scan_result = safe_result
        return safe_result
    except Exception as e:
        logger.error(f"Scan failed: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/scan/status")
def scan_status():
    """返回最近一次扫描的结果摘要。"""
    if _last_scan_result is None:
        return {"status": "no_scan_yet"}
    summary = {k: v for k, v in _last_scan_result.items() if k != "order_details"}
    summary["status"] = "ok"
    return _json_safe(summary)


# --- Position management endpoints ---

@app.get("/positions")
def get_positions():
    """Return all open positions from QMT real account (falls back to strategy executor if QMT unavailable)."""
    # Prefer real QMT positions from default configured account
    if qmt.is_connected() and qmt._config_accounts:
        default_account = qmt._config_accounts[0]
        try:
            return qmt.get_positions(default_account)
        except Exception as e:
            logger.warning(f"QMT positions query failed, falling back to executor: {e}")
    executor = _get_executor()
    return _json_safe(executor.positions.get_positions_list())


@app.get("/positions/summary")
def positions_summary():
    """Return position summary."""
    executor = _get_executor()
    return _json_safe(executor.positions.summary())


@app.post("/positions/check_exits")
def check_exits():
    """Check all positions for exit conditions and execute sells."""
    executor = _get_executor()
    exits = executor.positions.check_exits()
    if not exits:
        return {"exits": 0, "message": "no exit conditions triggered"}
    results = executor.positions.execute_exits(exits)
    return _json_safe({"exits": len(results), "details": results})


# --- Position monitor background task ---

_monitor_running = False

@app.on_event("startup")
async def start_position_monitor():
    """Start background position monitor that checks exits every 60 seconds."""
    import asyncio
    from datetime import time as dtime

    async def monitor_loop():
        global _monitor_running
        _monitor_running = True
        logger.info("[monitor] Position monitor started (checks every 60s during trading hours)")
        while _monitor_running:
            await asyncio.sleep(60)
            try:
                now = __import__("datetime").datetime.now().time()
                morning = dtime(9, 31) <= now <= dtime(11, 30)
                afternoon = dtime(13, 1) <= now <= dtime(14, 57)
                if not (morning or afternoon):
                    continue
                executor = _get_executor()
                exits = executor.positions.check_exits()
                if exits:
                    logger.info(f"[monitor] {len(exits)} exit signals detected")
                    results = executor.positions.execute_exits(exits)
                    for r in results:
                        logger.info(f"[monitor] SOLD {r['code']} @{r['price']:.2f} reason={r['reason']} pnl={r['pnl_pct']:+.1f}%")
                        try:
                            executor.http.post(f"{executor.go_api_url}/callback/strategy_signal", json={
                                "strategy_id": "position-monitor",
                                "signal": "sell",
                                "data": r,
                            })
                        except Exception:
                            pass
            except Exception as e:
                logger.error(f"[monitor] error: {e}")

    asyncio.create_task(monitor_loop())


# --- Order management: auto-cancel stale orders + re-submit at latest price ---

@app.on_event("startup")
async def start_order_manager():
    """Monitor open orders. Cancel unfilled orders after 5 minutes and re-submit at latest price."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    ORDER_STALE_SECONDS = 300  # 5 minutes

    async def order_manager_loop():
        logger.info("[order_mgr] Order manager started (checks every 60s)")
        while True:
            await asyncio.sleep(60)
            try:
                now = dt.now()
                now_t = now.time()
                morning = dtime(9, 31) <= now_t <= dtime(11, 30)
                afternoon = dtime(13, 1) <= now_t <= dtime(14, 57)
                if not (morning or afternoon):
                    continue

                if not qmt.is_connected() or not qmt._config_accounts:
                    continue

                account = qmt._config_accounts[0]
                open_orders = qmt.get_open_orders(account)
                if not open_orders:
                    continue

                for order in open_orders:
                    order_time_str = order.get("order_time", "")
                    order_id = order.get("order_id", "")
                    code = order.get("code", "")
                    direction = order.get("direction", "")
                    volume = order.get("volume", 0)
                    filled = order.get("filled_volume", 0)
                    remaining = volume - filled

                    if remaining <= 0:
                        continue

                    # Parse order time and check staleness
                    try:
                        if len(order_time_str) >= 8:
                            order_dt = dt.strptime(order_time_str[:14], "%Y%m%d%H%M%S") if len(order_time_str) >= 14 else now
                        else:
                            continue
                        age_seconds = (now - order_dt).total_seconds()
                    except Exception:
                        continue

                    if age_seconds < ORDER_STALE_SECONDS:
                        continue

                    # Stale order → cancel and re-submit at latest price
                    logger.info(f"[order_mgr] Stale order {order_id}: {code} {direction} "
                                f"age={age_seconds:.0f}s remaining={remaining}")

                    try:
                        qmt.cancel_order(account, order_id)
                        logger.info(f"[order_mgr] Cancelled {order_id}")
                    except Exception as e:
                        logger.warning(f"[order_mgr] Cancel failed {order_id}: {e}")
                        continue

                    # Re-submit remaining volume at latest price
                    try:
                        quotes = qmt.get_quotes([code])
                        if quotes and quotes[0].get("price", 0) > 0:
                            new_price = float(quotes[0]["price"])
                            # Round remaining to 100-share lots
                            re_volume = (remaining // 100) * 100
                            if re_volume >= 100:
                                new_id = qmt.submit_order(
                                    account=account, code=code,
                                    direction=direction, price=new_price,
                                    volume=re_volume, order_type="limit",
                                )
                                logger.info(f"[order_mgr] Re-submitted: {code} {direction} "
                                            f"@{new_price:.2f} x{re_volume} (was {order_id} → {new_id})")
                                # Log to trading log
                                try:
                                    _log_trade("resubmit", {
                                        "code": code, "direction": direction,
                                        "old_order_id": order_id, "new_order_id": new_id,
                                        "new_price": new_price, "volume": re_volume,
                                        "reason": f"stale_{age_seconds:.0f}s",
                                    })
                                except Exception:
                                    pass
                    except Exception as e:
                        logger.warning(f"[order_mgr] Re-submit {code} failed: {e}")

            except Exception as e:
                logger.error(f"[order_mgr] error: {e}")

    asyncio.create_task(order_manager_loop())


# --- Auto settlement after market close ---

@app.on_event("startup")
async def start_auto_settlement():
    """Auto-run daily settlement at 15:35 each trading day."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    async def settlement_loop():
        logger.info("[settlement] Auto-settlement scheduler started (triggers at 15:35)")
        settled_today = False
        while True:
            await asyncio.sleep(60)
            try:
                now = dt.now()
                # Reset flag at midnight
                if now.hour == 0:
                    settled_today = False

                # Trigger at 15:35
                if now.time() >= dtime(15, 35) and now.time() <= dtime(16, 0) and not settled_today:
                    settled_today = True
                    logger.info("[settlement] === AUTO SETTLEMENT ===")

                    from settlement import DailySettlement
                    from trade_tracker import TradeTracker

                    default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
                    if not default_account:
                        continue

                    s = DailySettlement(qmt, default_account)
                    tracker = TradeTracker()
                    today_str = now.strftime("%Y-%m-%d")
                    today_trades = [t.to_dict() for t in tracker.trades if t.exit_time.startswith(today_str)]

                    result = s.run(today_trades)
                    logger.info(f"[settlement] Result: PnL={result.get('realized_pnl', 0):+.2f}")

                    # Notify
                    try:
                        from notifier import notify_settlement
                        notify_settlement(
                            today_str,
                            result.get("realized_pnl", 0),
                            result.get("distribution", {}).get("star_energy_units", 0),
                        )
                    except Exception:
                        pass

            except Exception as e:
                logger.error(f"[settlement] auto error: {e}")

    asyncio.create_task(settlement_loop())


# --- Auto-scan scheduler ---

@app.on_event("startup")
async def start_auto_scanner():
    """Auto-scan every 30 minutes during trading hours."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    scan_interval = int(os.getenv("SCAN_INTERVAL", "1800"))  # 30 min default

    # Schedule: 09:35, 10:05, 10:35, 11:05, 13:05, 13:35, 14:05, 14:35
    async def scanner_loop():
        import concurrent.futures
        pool = concurrent.futures.ThreadPoolExecutor(max_workers=1)
        loop = asyncio.get_event_loop()

        logger.info(f"[scheduler] Auto-scan started (interval={scan_interval}s during trading hours)")
        last_scan_minute = -1
        while True:
            await asyncio.sleep(60)  # check every minute
            try:
                now = dt.now()
                now_t = now.time()
                morning = dtime(9, 35) <= now_t <= dtime(11, 25)
                afternoon = dtime(13, 5) <= now_t <= dtime(14, 45)
                if not (morning or afternoon):
                    continue

                # Only scan every scan_interval seconds
                minute_bucket = (now.hour * 60 + now.minute) // (scan_interval // 60)
                if minute_bucket == last_scan_minute:
                    continue
                last_scan_minute = minute_bucket

                logger.info(f"[scheduler] === AUTO SCAN {now.strftime('%H:%M')} ===")

                # Run blocking scan_once() in thread pool to avoid blocking event loop
                def do_scan():
                    executor = _get_executor()
                    return executor.scan_once()

                result = await loop.run_in_executor(pool, do_scan)
                safe = _json_safe(result)

                global _last_scan_result
                _last_scan_result = safe

                orders = safe.get("orders", 0)
                candidates = safe.get("candidates", 0)
                elapsed = safe.get("elapsed", 0)
                logger.info(f"[scheduler] Scan done: {candidates} candidates, {orders} orders, {elapsed:.0f}s")

            except Exception as e:
                logger.error(f"[scheduler] auto-scan error: {e}")

    asyncio.create_task(scanner_loop())


# --- Premarket analysis ---

@app.get("/premarket")
def premarket_analysis():
    """Run premarket analysis: market direction + position review."""
    import httpx
    from datetime import datetime as dt

    executor = _get_executor()
    go_api = executor.go_api_url

    # 1. Get index kline for market direction
    env = executor.detect_market_env()

    # 2. Get current positions
    held = executor.positions.get_positions_list()

    # 3. Ask Claw AI for market analysis
    prompt = f"""你是 Q8bot AI量化智能体的盘前分析师。

当前时间: {dt.now().strftime('%Y-%m-%d %H:%M')}
市场环境判断: {env}
当前持仓: {len(held)} 只股票

请分析:
1. 今日A股大盘方向预判（结合近期走势）
2. 建议今日仓位水位（满仓/七成/半仓/轻仓/空仓）
3. 重点关注板块
4. 持仓股票的操作建议

请用简洁中文回答，每项不超过2句话。"""

    ai_response = ""
    try:
        resp = httpx.post(
            f"{go_api}/v1/claw/confirm",
            json={"candidates": [], "market_env": env, "model": "qwen-max"},
            timeout=60,
        )
        if resp.status_code == 200:
            data = resp.json()
            ai_response = data.get("message", data.get("ai_response", ""))
    except Exception as e:
        ai_response = f"AI analysis unavailable: {e}"

    return _json_safe({
        "time": dt.now().strftime("%Y-%m-%d %H:%M"),
        "market_env": env,
        "positions_count": len(held),
        "positions": held,
        "ai_analysis": ai_response,
    })


# --- Trading Dashboard (Q8bot integration) ---

@app.get("/trading/dashboard")
def trading_dashboard():
    """Comprehensive trading dashboard for Q8bot — all data in one call.

    Returns: account info, positions with alpha analysis, risk status,
    market environment, last scan results, and today's trading log.
    """
    from datetime import datetime as dt
    import alpha_engine

    executor = _get_executor()
    env = executor._market_env or "sideways"

    # 1. Account info
    acct_info = {}
    default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
    if qmt.is_connected() and default_account:
        try:
            acct_info = qmt.get_account_info(default_account)
        except Exception:
            pass

    # 2. Real positions with alpha enrichment
    positions = []
    if qmt.is_connected() and default_account:
        try:
            raw_positions = qmt.get_positions(default_account)
            for p in raw_positions:
                pos = dict(p)
                # Add alpha analysis for each held stock
                try:
                    layers = alpha_engine.compute_layer_states(p["code"])
                    lifecycle = alpha_engine.detect_lifecycle_phase(
                        p["code"], p.get("cost_price", 0), p.get("market_price", 0), env)
                    chip = alpha_engine.classify_chip_shape(p["code"])
                    pos["layers"] = {k: v for k, v in layers.items() if k != "meta"}
                    pos["lifecycle"] = lifecycle.get("phase", "unknown")
                    pos["chip_shape"] = chip.get("shape_type", "unknown")
                    # MA signal for exit status
                    ma_sig = executor.positions._get_ma_signal(p["code"])
                    pos["ma_signal"] = ma_sig
                    # Profit target
                    targets = alpha_engine.dynamic_profit_targets(p["code"], env)
                    pos["profit_targets"] = targets
                except Exception:
                    pass
                positions.append(pos)
        except Exception as e:
            logger.warning(f"dashboard positions error: {e}")

    # 3. Risk status
    risk_status = {}
    try:
        risk_status = executor.risk.status_summary(env)
    except Exception:
        pass

    # 4. Last scan results
    scan = _last_scan_result or {}

    # 5. Market environment
    try:
        env = alpha_engine.detect_market_env()
    except Exception:
        pass

    return _json_safe({
        "time": dt.now().strftime("%Y-%m-%d %H:%M:%S"),
        "market_env": env,
        "account": acct_info,
        "positions": positions,
        "positions_count": len(positions),
        "total_pnl": sum(p.get("pnl_float", 0) for p in positions),
        "risk": risk_status,
        "last_scan": {
            "scanned": scan.get("scanned", 0),
            "candidates": scan.get("candidates", 0),
            "orders": scan.get("orders", 0),
            "elapsed": scan.get("elapsed", 0),
        },
    })


@app.get("/trading/risk")
def trading_risk():
    """Return current portfolio risk status."""
    executor = _get_executor()
    env = executor._market_env or "sideways"
    return _json_safe(executor.risk.status_summary(env))


@app.get("/trading/alpha/{code}")
def trading_alpha(code: str):
    """Run full alpha evaluation on a single stock (for Q8bot analysis)."""
    import alpha_engine
    try:
        env = alpha_engine.detect_market_env()
        result = alpha_engine.evaluate_stock(code, market_env=env)
        return _json_safe(result)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


# --- Trading log persistence ---

_trading_log: list = []  # in-memory log, newest first

def _log_trade(action: str, data: dict):
    """Append a trade event to the in-memory log."""
    from datetime import datetime as dt
    entry = {"time": dt.now().strftime("%Y-%m-%d %H:%M:%S"), "action": action, **data}
    _trading_log.insert(0, entry)
    if len(_trading_log) > 500:
        _trading_log[:] = _trading_log[:500]


@app.get("/trading/logs")
def trading_logs(limit: int = 50):
    """Return recent trading log entries."""
    return _json_safe(_trading_log[:limit])


@app.get("/trading/stats")
def trading_stats(last_n: int = 0):
    """Strategy performance statistics: win rate, profit factor, expectancy, attribution."""
    from trade_tracker import TradeTracker
    tracker = TradeTracker()
    return _json_safe({
        "overall": tracker.stats(last_n=last_n),
        "by_exit_reason": tracker.attribution_by_exit_reason(),
        "by_signal_strength": tracker.attribution_by_signal_strength(),
    })


@app.get("/trading/history")
def trading_history(limit: int = 20):
    """Recent completed trades with full details."""
    from trade_tracker import TradeTracker
    tracker = TradeTracker()
    return _json_safe(tracker.recent_trades(limit=limit))


@app.get("/trading/backtest")
def trading_backtest(days: int = 60, min_score: float = 0.60, top_n: int = 10):
    """Run strategy backtest with given parameters."""
    from backtest import run_backtest
    return _json_safe(run_backtest(days=days, min_score=min_score, top_n=top_n))


@app.get("/trading/macro")
def trading_macro():
    """Layer 1: Macro analysis — market direction, breadth, position advice."""
    import alpha_engine
    return _json_safe(alpha_engine.analyze_macro())


@app.get("/trading/sectors")
def trading_sectors():
    """Layer 2: Sector rotation — hot/cold sectors ranked by momentum."""
    import alpha_engine
    return _json_safe(alpha_engine.analyze_sectors())


@app.get("/trading/premarket_v2")
def trading_premarket_v2():
    """Comprehensive pre-market report: macro + sectors + guidance."""
    import alpha_engine
    return _json_safe(alpha_engine.premarket_report())


@app.get("/trading/settlement")
def trading_settlement_history(limit: int = 30):
    """Get settlement history."""
    from settlement import DailySettlement
    default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
    s = DailySettlement(qmt, default_account)
    return _json_safe({"history": s.get_history(limit), "cumulative": s.get_cumulative()})


@app.post("/trading/settlement/run")
def trading_settlement_run():
    """Manually trigger daily settlement."""
    from settlement import DailySettlement
    from trade_tracker import TradeTracker
    default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
    s = DailySettlement(qmt, default_account)
    tracker = TradeTracker()
    today_str = __import__("datetime").date.today().isoformat()
    today_trades = [t.to_dict() for t in tracker.trades if t.exit_time.startswith(today_str)]
    return _json_safe(s.run(today_trades))


# --- Daily report ---

@app.get("/report/daily")
def daily_report():
    """Generate daily trading report."""
    from datetime import datetime as dt

    executor = _get_executor()
    positions = executor.positions.get_positions_list()
    summary = executor.positions.summary()

    # Collect today's activity from scan results
    scan = _last_scan_result or {}

    report = {
        "date": dt.now().strftime("%Y-%m-%d"),
        "market_env": executor._market_env,
        "positions_held": len(positions),
        "positions_cost": summary.get("total_cost", 0),
        "last_scan": {
            "scanned": scan.get("scanned", 0),
            "candidates": scan.get("candidates", 0),
            "orders": scan.get("orders", 0),
        },
        "positions": positions,
    }
    return _json_safe(report)


# --- MCP Server endpoint (JSON-RPC 2.0 for Claw MCP protocol) ---

from fastapi import Request

@app.post("/")
async def mcp_jsonrpc(request: Request):
    """MCP JSON-RPC 2.0 endpoint. Claw sends POST to base URL with JSON-RPC body."""
    from mcp_server import handle_jsonrpc
    body = await request.json()
    result = handle_jsonrpc(body, {"app": app})
    return result


if __name__ == "__main__":
    port = int(os.getenv("BRIDGE_PORT", "8098"))
    uvicorn.run(app, host="0.0.0.0", port=port)
