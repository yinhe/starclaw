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


app = FastAPI(title="Extractor Bridge", lifespan=lifespan)


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
    """Return all open positions."""
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


if __name__ == "__main__":
    port = int(os.getenv("BRIDGE_PORT", "8098"))
    uvicorn.run(app, host="0.0.0.0", port=port)
