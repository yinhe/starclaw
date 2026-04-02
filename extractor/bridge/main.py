"""
Extractor Python Bridge — miniQMT ↔ Go API 桥接层
FastAPI server on :8098, wraps xtquant SDK for QMT interaction.
"""

import logging
import math
import os
import sys

# Ensure current directory is in sys.path (required for embedded Python)
_bridge_dir = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, _bridge_dir)

# Load .env from parent directory (extractor/.env)
_env_path = os.path.join(os.path.dirname(_bridge_dir), ".env")
if os.path.exists(_env_path):
    with open(_env_path, encoding="utf-8") as _f:
        for _line in _f:
            _line = _line.strip()
            if _line and not _line.startswith("#") and "=" in _line:
                _k, _, _v = _line.partition("=")
                _k, _v = _k.strip(), _v.strip()
                if _k and not os.environ.get(_k):
                    os.environ[_k] = _v

from contextlib import asynccontextmanager

import uvicorn
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from qmt_client import QMTClient
from account_manager import AccountManager
from team_manager import TeamManager

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(message)s")
logger = logging.getLogger("bridge")


# --- Strategy Log Collector (ring buffer for Dashboard) ---
from collections import deque
import threading

class StrategyLogCollector(logging.Handler):
    """Captures strategy-related logs into a ring buffer for the Dashboard."""
    STRATEGY_LOGGERS = {"executor", "strategy", "trend_main_wave", "position_manager", "bridge"}
    STRATEGY_KEYWORDS = {"SCAN", "executor", "candidate", "RISK", "ORDER", "confirm",
                         "pullback", "SKIP", "exit", "monitor", "scheduler", "scanner",
                         "budget", "settle", "signal", "order_mgr", "asset OK", "lifespan"}
    NOISE_PATTERNS = {"get_full_tick keys", "realtime prices", "research]", "[macro]", "[sentiment]", "[sector]"}

    def __init__(self, maxlen=200):
        super().__init__()
        self.logs = deque(maxlen=maxlen)
        self.phase = "idle"           # idle/scanning/scoring/confirming/ordering/monitoring
        self.last_scan = {}           # summary of last scan
        self._lock = threading.Lock()

    def emit(self, record):
        name = record.name
        msg = self.format(record)
        # Only capture strategy-relevant logs, skip noisy ones
        if any(n in msg for n in self.NOISE_PATTERNS):
            return
        if name in self.STRATEGY_LOGGERS or any(k in msg for k in self.STRATEGY_KEYWORDS):
            with self._lock:
                self.logs.append({
                    "ts": record.created,
                    "time": self.format_time(record),
                    "level": record.levelname,
                    "source": name,
                    "msg": record.getMessage(),
                })
            # Phase detection
            raw = record.getMessage()
            if "SCAN START" in raw:
                self.phase = "scanning"
            elif "base candidates" in raw:
                self.phase = "scoring"
            elif "Claw confirm" in raw:
                self.phase = "confirming"
            elif "ORDER:" in raw:
                self.phase = "ordering"
            elif "SCAN COMPLETE" in raw:
                self.phase = "monitoring"
                # Parse scan summary
                try:
                    import re
                    m = re.search(r'(\d+\.\d+)s, (\d+) orders', raw)
                    if m:
                        self.last_scan = {
                            "time": self.format_time(record),
                            "duration": float(m.group(1)),
                            "orders": int(m.group(2)),
                        }
                except Exception:
                    pass
            elif "AUTO SCAN" in raw:
                self.phase = "scanning"

    @staticmethod
    def format_time(record):
        from datetime import datetime
        return datetime.fromtimestamp(record.created).strftime("%H:%M:%S")

    def get_state(self):
        with self._lock:
            return {
                "phase": self.phase,
                "last_scan": self.last_scan,
                "logs": list(self.logs),
            }

strategy_log_collector = StrategyLogCollector()
strategy_log_collector.setLevel(logging.INFO)
strategy_log_collector.setFormatter(logging.Formatter("%(asctime)s [%(name)s] %(message)s"))
logging.getLogger().addHandler(strategy_log_collector)


# Globals
qmt: QMTClient = None
accounts: AccountManager = None
team_mgr: TeamManager = None


def _json_safe(value):
    if value is None or isinstance(value, (str, bool)):
        return value
    if isinstance(value, int):
        return int(value)
    if isinstance(value, float):
        return float(value) if math.isfinite(value) else None
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
    import asyncio
    global qmt, accounts, team_mgr
    qmt = QMTClient()
    accounts = AccountManager(qmt)
    default_acct = qmt._config_accounts[0] if qmt._config_accounts else ""
    team_mgr = TeamManager(qmt_client=qmt, account=default_acct)
    logger.info(f"[lifespan] Team Manager initialized with 5 agents, account={default_acct}")
    logger.info("🏦 Extractor Bridge starting on :8098")

    # Pre-warm instrument cache (PreClose, UpStopPrice, DownStopPrice) at startup
    # so sentiment data is accurate from the first request
    try:
        from alpha_engine import _ensure_instrument_cache, _diagnose_lastclose
        _ensure_instrument_cache()
        logger.info("[lifespan] Instrument cache pre-warmed at startup")
        # Diagnostic: compare QMT lastClose with eastmoney to find root cause
        _diagnose_lastclose()
    except Exception as e:
        logger.warning(f"[lifespan] Instrument cache warmup failed: {e}")

    # Start all background tasks (on_event("startup") is ignored when lifespan is used)
    asyncio.create_task(_run_position_monitor())
    asyncio.create_task(_run_order_manager())
    asyncio.create_task(_run_auto_settlement())
    asyncio.create_task(_run_auto_scanner())
    asyncio.create_task(_run_equity_snapshot())
    asyncio.create_task(_run_morning_briefing())
    logger.info("[lifespan] All 7 background tasks launched: monitor, order_mgr, settlement, scanner, equity, briefing")

    yield
    if qmt:
        qmt.disconnect()
    logger.info("Bridge shutdown")


import json as _json

class UTF8JSONResponse(JSONResponse):
    """JSON response that preserves Chinese characters (no ASCII escaping)."""
    def render(self, content) -> bytes:
        try:
            return _json.dumps(content, ensure_ascii=False, allow_nan=False, default=str).encode("utf-8")
        except (ValueError, TypeError):
            return _json.dumps(content, ensure_ascii=False, allow_nan=True, default=str).encode("utf-8")

app = FastAPI(title="Extractor Bridge", lifespan=lifespan, default_response_class=UTF8JSONResponse)


# --- Health ---

@app.get("/health")
def health():
    connected = qmt.is_connected() if qmt else False
    return {"status": "ok", "qmt_connected": connected}


@app.get("/strategy/status")
def strategy_status(limit: int = 50):
    """Return strategy lifecycle state: phase, risk, logs, scan summary."""
    state = strategy_log_collector.get_state()
    # Add risk status
    risk_info = {}
    try:
        from portfolio_risk import PortfolioRiskManager
        default_account = qmt._config_accounts[0] if qmt and qmt._config_accounts else ""
        if default_account:
            risk_mgr = PortfolioRiskManager(qmt, default_account)
            # Use macro direction if available, fallback to sideways
            market_env = "sideways"
            try:
                import alpha_engine
                env = alpha_engine.detect_market_env()
                market_env = env.get("environment", "sideways")
            except Exception:
                pass
            risk_info = _json_safe(risk_mgr.status_summary(market_env))
    except Exception as e:
        logger.debug(f"[strategy/status] risk error: {e}")
    # Trim logs to limit
    logs = state["logs"][-limit:] if len(state["logs"]) > limit else state["logs"]
    return _json_safe({
        "phase": state["phase"],
        "last_scan": state["last_scan"],
        "risk": risk_info,
        "logs": logs,
    })


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

@app.get("/account/default")
def account_default():
    """Return the default trading account ID from config."""
    acct = qmt._config_accounts[0] if qmt._config_accounts else ""
    return {"account": acct}


@app.get("/account/info")
def account_info(account: str = ""):
    try:
        if not account:
            account = qmt._config_accounts[0] if qmt._config_accounts else ""
        if not account:
            raise HTTPException(status_code=400, detail="No account configured")
        info = qmt.get_account_info(account)
        return info
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/account/positions")
def account_positions(account: str):
    try:
        positions = qmt.get_positions(account)
        return positions
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/account/equity")
def account_equity(period: str = "1y"):
    """Return equity curve data. period: 1w, 1m, 3m, 6m, 1y, all"""
    try:
        from equity_store import get_equity_curve
        return get_equity_curve(period)
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/account/equity/snapshot")
def account_equity_snapshot():
    """Manually trigger an equity snapshot for today."""
    try:
        from equity_store import seed_from_positions
        default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
        if not default_account:
            raise HTTPException(status_code=400, detail="No account configured")
        ok = seed_from_positions(qmt, default_account)
        return {"ok": ok}
    except HTTPException:
        raise
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
    """Return all open positions from QMT real account, enriched with entry reasons from position_manager."""
    positions = []
    # Prefer real QMT positions from default configured account
    if qmt.is_connected() and qmt._config_accounts:
        default_account = qmt._config_accounts[0]
        try:
            positions = qmt.get_positions(default_account)
        except Exception as e:
            logger.warning(f"QMT positions query failed, falling back to executor: {e}")
    if not positions:
        executor = _get_executor()
        return _json_safe(executor.positions.get_positions_list())
    # Merge entry reasons + entry_time from position_manager
    try:
        executor = _get_executor()
        pm_positions = {p.code: p for p in executor.positions.positions.values()}
        for pos in positions:
            code = pos.get("code", "")
            pm = pm_positions.get(code)
            if pm:
                pos["entry_reason"] = pm.reason or ""
                pos["entry_time"] = pm.entry_time or ""
                pos["entry_score"] = pm.score or 0
            else:
                pos["entry_reason"] = ""
                pos["entry_time"] = ""
                pos["entry_score"] = 0
    except Exception:
        pass
    return _json_safe(positions)


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

async def _run_position_monitor():
    """Background position monitor that checks exits every 60 seconds."""
    import asyncio
    from datetime import time as dtime
    global _monitor_running
    _monitor_running = True
    logger.info("[monitor] Position monitor started (checks every 30s during trading hours)")
    while _monitor_running:
        await asyncio.sleep(30)
        try:
            now = __import__("datetime").datetime.now().time()
            morning = dtime(9, 31) <= now <= dtime(11, 30)
            afternoon = dtime(13, 1) <= now <= dtime(14, 57)
            if not (morning or afternoon):
                continue
            executor = _get_executor()

            # ── Team gland real-time update ──
            if team_mgr:
                try:
                    acct = qmt._config_accounts[0] if qmt._config_accounts else ""
                    if acct:
                        pnl_pct = team_mgr.risk.update_pnl()
                        team_mgr.leader.update_pnl(pnl_pct)
                        # Update macro/hunter/coach glands too
                        team_mgr.macro.gland.update(pnl_pct)
                        team_mgr.hunter.gland.update(pnl_pct)
                        # Check protect mode propagation
                        if team_mgr.risk.is_protect_mode and not team_mgr.leader.is_protect_mode:
                            team_mgr.leader.enter_protect_mode(pnl_pct)
                except Exception as e:
                    logger.debug(f"[monitor] team gland update error: {e}")

            # ── Macro rating periodic refresh (every ~5 min) ──
            if team_mgr:
                try:
                    minute = __import__("datetime").datetime.now().minute
                    if minute % 5 == 0:  # refresh every 5 minutes
                        from alpha_engine import macro_dashboard, sentiment_score
                        md = macro_dashboard()
                        sent = sentiment_score()
                        env = md.get("env", "sideways")
                        team_mgr.macro.rate_market(env, md, sent)
                except Exception as e:
                    logger.debug(f"[monitor] macro rating refresh error: {e}")

            # ── Agent collaboration signals ──
            if team_mgr:
                try:
                    team_mgr.dispatch_signals()
                except Exception as e:
                    logger.debug(f"[monitor] dispatch_signals error: {e}")

            # ── Risk Officer position scan → direct sell execution ──
            if team_mgr:
                try:
                    acct = qmt._config_accounts[0] if qmt._config_accounts else ""
                    if acct:
                        qmt_positions = qmt.get_positions(acct)
                        risk_signals = team_mgr.risk.scan_all_positions(qmt_positions)
                        for sig in risk_signals:
                            logger.info(f"[monitor] RISK OFFICER: {sig['action']} {sig['code']} — {sig['reason']}")
                            # Direct sell execution for risk officer exit signals
                            if sig.get("action") in ("stop_loss", "hard_stop", "protect_stop", "take_profit", "protect_lock", "trailing_stop"):
                                code = sig["code"]
                                vol = sig.get("volume", 0)
                                if vol > 0 and qmt.is_connected():
                                    try:
                                        from xtquant import xtdata
                                        tick = xtdata.get_full_tick([code])
                                        price = tick.get(code, {}).get("lastPrice", 0) if tick else 0
                                        if price > 0:
                                            sell_price = round(price * 0.995, 2)  # slightly below market
                                            order_id = qmt.sell(acct, code, vol, sell_price)
                                            logger.warning(f"[monitor] RISK SELL {code} vol={vol} @{sell_price} order={order_id} reason={sig['reason']}")
                                            team_mgr.risk.log_decision(
                                                action="execute_stop_loss",
                                                target=code,
                                                confidence=95,
                                                reasoning=f"风控官强制止损: {sig['reason']}",
                                                data={"price": sell_price, "volume": vol, "order_id": str(order_id)},
                                            )
                                            # Phase 10: Auto-feedback to coach for learning loop
                                            is_profit = sig["action"] in ("take_profit",)
                                            strategy = sig.get("strategy", "")
                                            sector = sig.get("sector", "")
                                            team_mgr.hunter.record_outcome(code, strategy, sector, is_profit)
                                            logger.info(f"[monitor] Feedback: {code} profit={is_profit} strategy={strategy[:30]}")
                                    except Exception as se:
                                        logger.error(f"[monitor] risk sell failed {code}: {se}")
                except Exception as e:
                    logger.debug(f"[monitor] risk officer scan error: {e}")

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


# --- Order management: auto-cancel stale orders + re-submit at latest price ---

async def _run_order_manager():
    """Monitor open orders. Cancel unfilled orders after 5 minutes and re-submit at latest price."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    ORDER_STALE_SECONDS = 300  # 5 minutes
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


# --- Auto settlement after market close ---

async def _run_auto_settlement():
    """Auto-run daily settlement at 15:35 each trading day."""
    import asyncio
    from datetime import time as dtime, datetime as dt

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

                # ── Coach feedback loop + daily review ──
                if team_mgr:
                    try:
                        daily_pnl = result.get("realized_pnl", 0)

                        # Step 1: Feedback — mark decisions with profit/loss outcomes
                        team_mgr.coach.feedback_outcomes(today_trades, team_mgr.hunter)

                        # Step 2: LLM daily review + memory persist
                        review = team_mgr.coach.daily_review(
                            trades_today=today_trades,
                            daily_pnl=daily_pnl,
                        )
                        logger.info(f"[coach] Daily review: {review.get('total_decisions', 0)} decisions, "
                                    f"mutations={len(review.get('gene_mutations', []))}")

                        # Step 3: Apply gene mutations if coach suggests them
                        mutations = review.get("gene_mutations", [])
                        if mutations:
                            _apply_gene_mutations(mutations)

                        # Step 4: Cleanup old decisions (keep 90 days)
                        deleted = team_mgr.store.cleanup(keep_days=90)
                        if deleted:
                            logger.info(f"[coach] Cleaned up {deleted} old decisions (>90d)")
                    except Exception as e:
                        logger.warning(f"[coach] auto review error: {e}")

        except Exception as e:
            logger.error(f"[settlement] auto error: {e}")


def _apply_gene_mutations(mutations: list):
    """应用教练建议的基因微调（热更新，无需重启）。"""
    if not team_mgr or not mutations:
        return
    agents = {
        "leader": team_mgr.leader,
        "macro": team_mgr.macro,
        "hunter": team_mgr.hunter,
        "risk": team_mgr.risk,
        "coach": team_mgr.coach,
    }
    for m in mutations:
        agent_role = m.get("agent", "")
        param = m.get("param", "")
        direction = m.get("direction", "")
        reason = m.get("reason", "")
        agent = agents.get(agent_role)
        if not agent or not param:
            continue

        gene = agent.gene
        old_val = getattr(gene, param, None)
        if old_val is None:
            continue

        # Apply small delta (±5% or ±0.05)
        delta = 0.05 if direction == "increase" else -0.05
        new_val = max(0.0, min(1.0, old_val + delta))
        setattr(gene, param, new_val)

        team_mgr.store.record(
            __import__("team_core").Decision(
                agent="coach",
                action="gene_mutation",
                target=agent_role,
                confidence=80,
                reasoning=f"{param}: {old_val:.2f}→{new_val:.2f} ({reason})",
                data=__import__("json").dumps({
                    "param": param, "old": old_val, "new": new_val, "direction": direction,
                }),
            )
        )
        logger.info(f"[gene] {agent_role}.{param}: {old_val:.2f}→{new_val:.2f} ({reason})")


# --- Equity snapshot recorder ---

async def _run_equity_snapshot():
    """Record daily equity snapshot at 15:05 (after close) and on startup."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    logger.info("[equity] Equity snapshot recorder started")
    snapped_today = False

    # Seed on startup (after a delay to let QMT warm up)
    await asyncio.sleep(45)
    try:
        from equity_store import seed_from_positions
        default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
        if default_account:
            seed_from_positions(qmt, default_account)
            logger.info("[equity] Startup seed done")
    except Exception as e:
        logger.warning(f"[equity] Startup seed failed: {e}")

    while True:
        await asyncio.sleep(60)
        try:
            now = dt.now()
            if now.hour == 0:
                snapped_today = False

            # Snapshot at 15:05 (right after market close, prices are final)
            if now.time() >= dtime(15, 5) and now.time() <= dtime(15, 30) and not snapped_today:
                snapped_today = True
                default_account = qmt._config_accounts[0] if qmt._config_accounts else ""
                if default_account:
                    from equity_store import seed_from_positions
                    ok = seed_from_positions(qmt, default_account)
                    logger.info(f"[equity] Daily snapshot recorded: ok={ok}")
        except Exception as e:
            logger.error(f"[equity] snapshot error: {e}")


# --- Morning briefing auto-trigger ---

async def _run_morning_briefing():
    """Auto-trigger morning briefing at 9:25 each trading day (before market open)."""
    import asyncio
    from datetime import time as dtime, datetime as dt

    logger.info("[briefing] Morning briefing scheduler started (triggers at 9:25)")
    briefed_today = False
    while True:
        await asyncio.sleep(30)
        try:
            now = dt.now()
            if now.hour == 0:
                briefed_today = False

            # Trigger at 9:25 (5 min before market open)
            if now.time() >= dtime(9, 25) and now.time() <= dtime(9, 35) and not briefed_today:
                briefed_today = True
                if not team_mgr or not qmt or not qmt.is_connected():
                    continue

                logger.info("[briefing] === AUTO MORNING BRIEFING ===")
                acct = qmt._config_accounts[0] if qmt._config_accounts else ""
                if not acct:
                    continue

                # Gather context
                try:
                    from alpha_engine import macro_dashboard, sentiment_score
                    macro_data = macro_dashboard()
                    sentiment = sentiment_score()
                    market_env = macro_data.get("env", "sideways")

                    asset_info = qmt.get_asset(acct)
                    total_assets = asset_info.get("total_asset", 0) if asset_info else 0
                    market_value = asset_info.get("market_value", 0) if asset_info else 0
                    available_cash = asset_info.get("available_cash", 0) if asset_info else 0

                    positions = qmt.get_positions(acct)
                    pos_list = []
                    for p in (positions or []):
                        pnl_pct = 0
                        if p.get("cost_price", 0) > 0:
                            pnl_pct = (p.get("last_price", 0) / p["cost_price"] - 1) * 100
                        pos_list.append({"code": p.get("code", ""), "pnl_pct": round(pnl_pct, 2)})

                    # Run morning briefing (includes LLM battle plan)
                    result = team_mgr.leader.morning_briefing(
                        market_env=market_env,
                        macro_data=macro_data,
                        total_assets=total_assets,
                        market_value=market_value,
                        available_cash=available_cash,
                        positions=pos_list,
                        sentiment=sentiment,
                    )
                    plan = result.get("battle_plan", "")
                    logger.info(f"[briefing] Budget=¥{result.get('budget', 0):,.0f} "
                                f"Exposure={result.get('max_exposure_pct', 0):.0f}% "
                                f"Plan={'yes' if plan else 'no'}")

                    # Also run macro rating at open
                    team_mgr.macro.rate_market(market_env, macro_data, sentiment)

                except Exception as e:
                    logger.warning(f"[briefing] error: {e}")

        except Exception as e:
            logger.error(f"[briefing] scheduler error: {e}")


# --- Auto-scan scheduler ---

async def _run_auto_scanner():
    """Auto-scan every 30 minutes during trading hours."""
    import asyncio
    import concurrent.futures
    from datetime import time as dtime, datetime as dt

    scan_interval = int(os.getenv("SCAN_INTERVAL", "1800"))  # 30 min default
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


@app.get("/trading/qmt_trades")
def trading_qmt_trades():
    """Query all dealt trades from QMT (today's 成交记录)."""
    if not qmt.is_connected() or not qmt._config_accounts:
        raise HTTPException(status_code=503, detail="QMT not connected")
    account = qmt._config_accounts[0]
    trades = qmt.get_dealt_trades(account)
    orders = qmt.get_all_orders(account)
    return _json_safe({"trades": trades, "orders": orders})


def _ts_to_str(ts) -> str:
    """Convert QMT timestamp (int Unix or string) to 'YYYY-MM-DD HH:MM:SS'."""
    from datetime import datetime as dt
    try:
        v = int(ts)
        if v > 1_000_000_000:  # Unix timestamp
            return dt.fromtimestamp(v).strftime("%Y-%m-%d %H:%M:%S")
        elif v > 20200000:  # date int like 20260402
            return dt.strptime(str(v), "%Y%m%d").strftime("%Y-%m-%d")
    except (ValueError, TypeError, OSError):
        pass
    s = str(ts)
    if len(s) >= 10 and '-' in s:
        return s  # already formatted
    return ""


def _resolve_name(code: str) -> str:
    """Resolve stock name from xtdata."""
    try:
        from xtquant import xtdata
        detail = xtdata.get_instrument_detail(code)
        if detail:
            return detail.get("InstrumentName", "")
    except Exception:
        pass
    return ""


@app.post("/trading/import_history")
def trading_import_history(start_date: str = "20260101", end_date: str = "", clear: bool = False):
    """Import QMT historical deals into trades.db.

    Uses export_data/query_data API which supports date ranges (not just today).
    Falls back to query_stock_orders (today only) if export_data is unavailable.

    Args:
        start_date: Start date like '20260101' (default: year start)
        end_date: End date like '20260402' (default: today)
        clear: If true, clear existing trades first (for re-import)
    """
    if not qmt.is_connected() or not qmt._config_accounts:
        raise HTTPException(status_code=503, detail="QMT not connected")
    account = qmt._config_accounts[0]

    if not end_date:
        from datetime import datetime
        end_date = datetime.now().strftime("%Y%m%d")

    # Optionally clear existing imported trades for re-import
    if clear:
        import sqlite3
        from trade_tracker import TRADES_DB
        try:
            conn = sqlite3.connect(TRADES_DB)
            conn.execute("DELETE FROM trades WHERE entry_reason='QMT历史导入'")
            conn.commit()
            conn.close()
            logger.info("[import] Cleared existing QMT-imported trades")
        except Exception as e:
            logger.warning(f"[import] Failed to clear: {e}")

    # --- Method 1: Use export_history_deals (supports historical date range) ---
    deals = qmt.export_history_deals(account, start_time=start_date, end_time=end_date)
    method = "export_data"

    if not deals:
        # --- Fallback: query today's orders only ---
        logger.info("[import] export_history_deals returned empty, falling back to query_stock_orders")
        orders = qmt.get_all_orders(account)
        deals = []
        for o in orders:
            if o.get("traded_volume", 0) <= 0:
                continue
            deals.append({
                "code": o["code"],
                "direction": o["direction"],
                "price": o.get("traded_price", 0) or o.get("price", 0),
                "volume": o.get("traded_volume", 0),
                "traded_time": o.get("order_time", ""),
            })
        method = "query_stock_orders(today_only)"

    # Convert timestamps and separate buys/sells
    buys = {}   # code → list of buy deals (sorted by time)
    sells = {}  # code → list of sell deals (sorted by time)
    for d in deals:
        code = d.get("code", "")
        if not code:
            continue
        # Normalize timestamp
        d["traded_time_str"] = _ts_to_str(d.get("traded_time", ""))
        direction = d.get("direction", "")
        if direction == "buy":
            buys.setdefault(code, []).append(d)
        elif direction == "sell":
            sells.setdefault(code, []).append(d)

    # Sort by time
    for v in buys.values():
        v.sort(key=lambda x: x.get("traded_time_str", ""))
    for v in sells.values():
        v.sort(key=lambda x: x.get("traded_time_str", ""))

    from trade_tracker import TradeTracker
    tracker = TradeTracker()
    # Build dedup set using (code, exit_time_str)
    existing = set()
    for t in tracker.trades:
        existing.add((t.code, t.exit_time))
    imported = 0
    name_cache = {}

    for code, sell_list in sells.items():
        buy_list = buys.get(code, [])
        # Try matching: pair each sell with the most recent buy before it
        buy_idx = 0
        for s in sell_list:
            exit_time = s.get("traded_time_str", "")
            exit_price = float(s.get("price", 0))
            volume = int(s.get("volume", 0))
            if exit_price <= 0 or volume <= 0:
                continue
            # Dedup
            if (code, exit_time) in existing:
                continue

            # Find matching buy (closest buy before this sell)
            entry_price = 0.0
            entry_time = ""
            while buy_idx < len(buy_list):
                bt = buy_list[buy_idx].get("traded_time_str", "")
                if bt <= exit_time:
                    entry_price = float(buy_list[buy_idx].get("price", 0))
                    entry_time = bt
                    buy_idx += 1
                    break
                buy_idx += 1
            # If no buy matched, use first available buy
            if entry_price == 0 and buy_list:
                entry_price = float(buy_list[0].get("price", 0))
                entry_time = buy_list[0].get("traded_time_str", "")

            # Resolve stock name
            if code not in name_cache:
                name_cache[code] = _resolve_name(code)
            name = name_cache[code]

            tracker.record_trade(
                code=code,
                entry_price=entry_price,
                exit_price=exit_price,
                volume=volume,
                entry_time=entry_time,
                exit_time=exit_time,
                entry_reason="QMT历史导入",
                exit_reason="QMT历史导入",
            )
            # Patch name into DB
            if name:
                try:
                    import sqlite3
                    from trade_tracker import TRADES_DB
                    conn = sqlite3.connect(TRADES_DB)
                    conn.execute("UPDATE trades SET name=? WHERE code=? AND name=''", (name, code))
                    conn.commit()
                    conn.close()
                except Exception:
                    pass

            imported += 1
            existing.add((code, exit_time))

    return {
        "imported": imported,
        "total_deals": len(deals),
        "total_sells": sum(len(v) for v in sells.values()),
        "total_buys": sum(len(v) for v in buys.values()),
        "method": method,
        "date_range": f"{start_date}-{end_date}",
    }


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


@app.get("/trading/review")
def trading_review():
    """Comprehensive trade review: stats + attribution + time analysis + strategy suggestions."""
    from trade_tracker import TradeTracker
    tracker = TradeTracker()
    return _json_safe(tracker.review())


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


@app.get("/trading/sentiment")
def trading_sentiment():
    """Layer 3: Market sentiment — fear/greed gauge, AD ratio, limit up/down."""
    import alpha_engine
    return _json_safe(alpha_engine.analyze_sentiment())


@app.get("/trading/research")
def trading_research(code: str, cost_price: float = 0):
    """Layer 4: Individual stock deep research report."""
    import alpha_engine
    return _json_safe(alpha_engine.research_stock(code, cost_price))


@app.get("/trading/research/portfolio")
def trading_research_portfolio():
    """Layer 4: Research all current positions."""
    import alpha_engine
    default_account = qmt._config_accounts[0] if qmt and qmt._config_accounts else ""
    if not default_account:
        return {"error": "no account configured"}
    try:
        positions = qmt.get_positions(default_account)
    except Exception as e:
        return {"error": str(e)}
    reports = []
    for p in positions:
        if p.get("volume", 0) > 0:
            report = alpha_engine.research_stock(
                p["code"], p.get("cost_price", 0))
            report["name"] = p.get("name", "")
            report["volume"] = p.get("volume", 0)
            report["pnl_float"] = p.get("pnl_float", 0)
            reports.append(report)
    reports.sort(key=lambda x: x.get("composite_score", 0), reverse=True)
    return _json_safe({"positions": len(reports), "reports": reports})


@app.get("/trading/master")
def trading_master():
    """Layer 5: Qwen LLM master analysis — synthesizes all layers into actionable report."""
    import alpha_engine
    default_account = qmt._config_accounts[0] if qmt and qmt._config_accounts else ""
    positions = []
    if default_account:
        try:
            positions = [p for p in qmt.get_positions(default_account) if p.get("volume", 0) > 0]
        except Exception:
            pass
    return _json_safe(alpha_engine.master_analysis(positions))


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


# --- Briefing System (盘前分析 + 日终复盘) ---

_BRIEFING_DB = os.path.join(os.path.dirname(os.path.abspath(__file__)), "trades.db")

def _init_briefing_table():
    import sqlite3
    try:
        conn = sqlite3.connect(_BRIEFING_DB)
        conn.execute("""CREATE TABLE IF NOT EXISTS briefings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            type TEXT NOT NULL,
            date TEXT NOT NULL,
            content TEXT NOT NULL,
            input_data TEXT DEFAULT '{}',
            model TEXT DEFAULT '',
            tokens_used INTEGER DEFAULT 0,
            created_at TEXT DEFAULT (datetime('now','localtime'))
        )""")
        conn.execute("CREATE INDEX IF NOT EXISTS idx_briefings_type_date ON briefings(type, date)")
        conn.commit()
        conn.close()
    except Exception as e:
        logger.warning(f"Failed to init briefings table: {e}")

_init_briefing_table()


def _save_briefing(btype: str, date: str, content: str, input_data: str = "{}", model: str = "", tokens: int = 0):
    import sqlite3
    try:
        conn = sqlite3.connect(_BRIEFING_DB)
        conn.execute("DELETE FROM briefings WHERE type=? AND date=?", (btype, date))
        conn.execute(
            "INSERT INTO briefings (type, date, content, input_data, model, tokens_used) VALUES (?,?,?,?,?,?)",
            (btype, date, content, input_data, model, tokens)
        )
        conn.commit()
        conn.close()
    except Exception as e:
        logger.warning(f"Failed to save briefing: {e}")


def _load_briefing(btype: str, date: str = "") -> dict | None:
    import sqlite3
    try:
        conn = sqlite3.connect(_BRIEFING_DB)
        conn.row_factory = sqlite3.Row
        if date:
            row = conn.execute("SELECT * FROM briefings WHERE type=? AND date=? ORDER BY id DESC LIMIT 1",
                               (btype, date)).fetchone()
        else:
            row = conn.execute("SELECT * FROM briefings WHERE type=? ORDER BY id DESC LIMIT 1",
                               (btype,)).fetchone()
        conn.close()
        if row:
            return dict(row)
    except Exception:
        pass
    return None


@app.get("/trading/briefing/latest")
def briefing_latest():
    """Get latest pre-market analysis and end-of-day review."""
    from datetime import date
    today = date.today().isoformat()
    premarket = _load_briefing("premarket", today) or _load_briefing("premarket")
    eod = _load_briefing("eod", today) or _load_briefing("eod")
    return _json_safe({"premarket": premarket, "eod": eod})


@app.post("/trading/briefing/premarket")
def briefing_premarket():
    """Generate LLM-powered pre-market analysis and save to DB."""
    import httpx, json as _json
    from datetime import datetime as dt
    import alpha_engine

    today = dt.now().strftime("%Y-%m-%d")
    api_key = os.getenv("QWEN_API_KEY", "")
    model = os.getenv("QWEN_MODEL", "qwen-max")
    base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")

    if not api_key:
        return _json_safe({"error": "QWEN_API_KEY not configured"})

    # Gather market data
    macro = alpha_engine.analyze_macro()
    sectors = alpha_engine.analyze_sectors()
    sentiment = alpha_engine.analyze_sentiment()
    env = alpha_engine.detect_market_env()

    # Get positions + account
    positions = []
    default_account = qmt._config_accounts[0] if qmt and qmt._config_accounts else ""
    acct_info = {}
    if default_account and qmt.is_connected():
        try:
            positions = [p for p in qmt.get_positions(default_account) if p.get("volume", 0) > 0]
            acct_info = qmt.get_account_info(default_account)
        except Exception:
            pass

    pos_info = []
    for p in positions[:8]:
        vol = p.get("volume", 0)
        price = p.get("market_price", 0)
        pos_info.append({"code": p.get("code"), "name": p.get("name", ""),
                         "volume": vol, "cost": p.get("cost_price", 0),
                         "price": price,
                         "market_value": round(price * vol, 0),
                         "pnl_pct": round((price - p.get("cost_price", 1)) / max(p.get("cost_price", 1), 0.01) * 100, 2)})

    # Yesterday's settlement
    settlement_info = ""
    try:
        from settlement import DailySettlement
        s = DailySettlement(qmt, default_account)
        hist = s.get_history(1)
        if hist:
            y = hist[0]
            settlement_info = f"昨日盈亏: ¥{y.get('net_pnl', 0):.0f}, 交易{y.get('trade_count', 0)}笔"
    except Exception:
        pass

    total_assets = acct_info.get("total_assets", 0)
    mkt_value = acct_info.get("market_value", 0)
    avail_cash = acct_info.get("available", 0) or acct_info.get("available_cash", 0)
    data_block = _json.dumps({
        "date": today,
        "market_env": env,
        "account": {"total_assets": round(total_assets, 0),
                    "market_value": round(mkt_value, 0),
                    "available_cash": round(avail_cash, 0),
                    "position_ratio": round(mkt_value / max(total_assets, 1) * 100, 1)},
        "macro": {"direction": macro["direction"], "confidence": macro["confidence"],
                  "position_advice": macro["position_advice"], "reasons": macro.get("reasons", [])},
        "sentiment": {"gauge": sentiment["gauge"], "score": sentiment["score"]},
        "hot_sectors": sectors.get("recommended_sectors", [])[:5],
        "avoid_sectors": sectors.get("avoid_sectors", [])[:5],
        "positions": pos_info,
        "yesterday": settlement_info,
    }, ensure_ascii=False, indent=2)

    prompt = f"""你是Q8bot量化AI盘前分析师。现在是开盘前，请基于以下数据生成今日盘前分析报告。

## 实时数据
{data_block}

## 报告格式要求（严格按此结构输出）

### 一、市场研判
今日大盘方向预判，结合宏观信号和情绪面，给出看多/看空/中性判断及理由。

### 二、仓位策略
建议总仓位水位（满仓/七成/半仓/轻仓/空仓），以及资金分配思路。

### 三、板块机会
今日重点关注和回避的板块，以及潜在的轮动方向。

### 四、持仓计划
对当前每只持仓给出今日操作计划（持有/加仓/减仓/止损），附简短理由。

### 五、风险提示
今日需要特别警惕的风险点。

请用简洁中文回答，每部分3-5句话。"""

    try:
        resp = httpx.post(
            f"{base_url}/chat/completions",
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
            json={"model": model, "messages": [
                {"role": "system", "content": "你是专业的A股量化盘前分析师，擅长技术分析和风险管理。语言简洁、可操作、有数据支撑。"},
                {"role": "user", "content": prompt},
            ], "temperature": 0.3, "max_tokens": 2000},
            timeout=60.0,
        )
        resp.raise_for_status()
        data = resp.json()
        content = data["choices"][0]["message"]["content"]
        usage = data.get("usage", {})
        tokens = usage.get("total_tokens", 0)

        _save_briefing("premarket", today, content, data_block, model, tokens)
        logger.info(f"[briefing] premarket saved: {today} tokens={tokens}")

        return _json_safe({"type": "premarket", "date": today, "content": content,
                           "model": model, "tokens_used": tokens})
    except Exception as e:
        logger.error(f"[briefing] premarket generation failed: {e}")
        return _json_safe({"error": str(e)})


@app.post("/trading/briefing/eod")
def briefing_eod():
    """Generate LLM-powered end-of-day review and save to DB."""
    import httpx, json as _json
    from datetime import datetime as dt
    import alpha_engine

    today = dt.now().strftime("%Y-%m-%d")
    api_key = os.getenv("QWEN_API_KEY", "")
    model = os.getenv("QWEN_MODEL", "qwen-max")
    base_url = os.getenv("QWEN_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")

    if not api_key:
        return _json_safe({"error": "QWEN_API_KEY not configured"})

    # Gather market data
    macro = alpha_engine.analyze_macro()
    sentiment = alpha_engine.analyze_sentiment()
    env = alpha_engine.detect_market_env()

    # Today's positions
    positions = []
    default_account = qmt._config_accounts[0] if qmt and qmt._config_accounts else ""
    acct_info = {}
    if default_account and qmt.is_connected():
        try:
            positions = [p for p in qmt.get_positions(default_account) if p.get("volume", 0) > 0]
            acct_info = qmt.get_account_info(default_account)
        except Exception:
            pass

    pos_info = []
    for p in positions[:8]:
        vol = p.get("volume", 0)
        price = p.get("market_price", 0)
        pos_info.append({"code": p.get("code"), "name": p.get("name", ""),
                         "volume": vol, "cost": p.get("cost_price", 0),
                         "price": price,
                         "market_value": round(price * vol, 0),
                         "pnl_pct": round((price - p.get("cost_price", 1)) / max(p.get("cost_price", 1), 0.01) * 100, 2)})

    # Today's trades
    from trade_tracker import TradeTracker
    tracker = TradeTracker()
    today_trades = [t.to_dict() for t in tracker.trades if t.exit_time.startswith(today)]
    trade_summary = []
    for t in today_trades:
        trade_summary.append({"code": t["code"], "name": t.get("name", ""),
                              "entry": t["entry_price"], "exit": t["exit_price"],
                              "pnl_pct": t["pnl_pct"], "reason": t.get("exit_reason", "")})

    # Today's buys (from QMT)
    today_buys = []
    try:
        dealt = qmt.get_dealt_trades(default_account) if default_account else []
        for d in dealt:
            if d.get("direction") == "buy":
                today_buys.append({"code": d["code"], "price": d["price"], "volume": d["volume"]})
    except Exception:
        pass

    # Settlement
    settlement_info = ""
    try:
        from settlement import DailySettlement
        s = DailySettlement(qmt, default_account)
        today_settlement = s.run([])  # Just get today's data
        if today_settlement:
            settlement_info = f"今日盈亏: ¥{today_settlement.get('net_pnl', acct_info.get('today_pnl', 0)):.0f}"
    except Exception:
        settlement_info = f"今日盈亏: ¥{acct_info.get('today_pnl', 0):.0f}" if acct_info else ""

    data_block = _json.dumps({
        "date": today,
        "market_env": env,
        "macro_direction": macro["direction"],
        "sentiment": {"gauge": sentiment["gauge"], "score": sentiment["score"]},
        "account": {"total_assets": round(acct_info.get("total_assets", 0), 0),
                    "market_value": round(acct_info.get("market_value", 0), 0),
                    "available_cash": round(acct_info.get("available", 0) or acct_info.get("available_cash", 0), 0),
                    "today_pnl": acct_info.get("today_profit", 0) or acct_info.get("today_pnl", 0),
                    "position_ratio": round(acct_info.get("market_value", 0) / max(acct_info.get("total_assets", 1), 1) * 100, 1)},
        "positions": pos_info,
        "today_sells": trade_summary,
        "today_buys": today_buys,
        "settlement": settlement_info,
    }, ensure_ascii=False, indent=2)

    prompt = f"""你是Q8bot量化AI日终复盘分析师。现在收盘了，请基于以下数据生成今日复盘报告。

## 今日数据
{data_block}

## 报告格式要求（严格按此结构输出）

### 一、今日大盘回顾
大盘走势总结，与盘前预判对比（是否符合预期）。

### 二、账户表现
今日盈亏、仓位变化、是否完成交易计划。

### 三、交易复盘
逐笔分析今日买入和卖出的执行质量：入场点/出场点是否合理，有没有做对/做错的地方。

### 四、持仓评估
对当前持仓逐只点评：趋势是否健康、是否需要调整。

### 五、明日计划
基于今日表现和市场状态，给出明日操作方向和仓位建议。

### 六、经验教训
今日最大的收获或教训是什么（一句话总结）。

请用简洁中文回答，每部分2-4句话。要客观、不回避问题。"""

    try:
        resp = httpx.post(
            f"{base_url}/chat/completions",
            headers={"Authorization": f"Bearer {api_key}", "Content-Type": "application/json"},
            json={"model": model, "messages": [
                {"role": "system", "content": "你是严谨的A股量化复盘分析师。对每笔交易客观评价，不回避错误，注重从错误中提取可操作的改进建议。"},
                {"role": "user", "content": prompt},
            ], "temperature": 0.3, "max_tokens": 2000},
            timeout=60.0,
        )
        resp.raise_for_status()
        data = resp.json()
        content = data["choices"][0]["message"]["content"]
        usage = data.get("usage", {})
        tokens = usage.get("total_tokens", 0)

        _save_briefing("eod", today, content, data_block, model, tokens)
        logger.info(f"[briefing] eod saved: {today} tokens={tokens}")

        return _json_safe({"type": "eod", "date": today, "content": content,
                           "model": model, "tokens_used": tokens})
    except Exception as e:
        logger.error(f"[briefing] eod generation failed: {e}")
        return _json_safe({"error": str(e)})


# --- Team Agent API ---

@app.get("/team/status")
def team_status():
    """Return full team state for dashboard TeamPanel."""
    if not team_mgr:
        return {"error": "team not initialized"}
    return _json_safe(team_mgr.get_team_state())


@app.get("/team/decisions")
def team_decisions(limit: int = 50, agent: str = None):
    """Get recent team decisions."""
    if not team_mgr:
        return []
    return _json_safe(team_mgr.get_decisions(limit=limit, agent=agent or None))


@app.get("/team/decisions/today")
def team_decisions_today():
    """Get today's team decisions."""
    if not team_mgr:
        return []
    return _json_safe(team_mgr.get_today_decisions())


@app.get("/team/agent/{role}")
def team_agent_detail(role: str):
    """Get single agent detail."""
    if not team_mgr:
        return {"error": "team not initialized"}
    state = team_mgr.get_agent_state(role)
    if not state:
        raise HTTPException(404, f"agent '{role}' not found")
    return _json_safe(state)


@app.get("/team/memory/{role}")
def team_agent_memory(role: str):
    """Get agent's persistent memory."""
    if not team_mgr:
        return {"error": "team not initialized"}
    return _json_safe(team_mgr.memory.get_all(role))


@app.get("/team/alerts")
def team_alerts():
    """Get recent alert-level decisions (protect mode, veto, risk warnings)."""
    if not team_mgr:
        return []
    today_decisions = team_mgr.get_today_decisions()
    alert_actions = {
        "protect_mode_enter", "protect_mode_exit", "veto",
        "market_bearish", "gene_mutation", "daily_review",
        "stop_loss", "hard_stop", "trailing_stop", "take_profit",
    }
    alerts = [d for d in today_decisions if d.get("action") in alert_actions
              or d.get("confidence", 0) >= 90]
    # Parse JSON data for frontend
    for a in alerts:
        if isinstance(a.get("data"), str) and a["data"]:
            try:
                a["data"] = json.loads(a["data"])
            except Exception:
                pass
    return _json_safe(alerts[-20:])


@app.get("/team/battle-plan")
def team_battle_plan():
    """Get today's battle plan from leader's morning briefing."""
    if not team_mgr:
        return {"plan": "", "date": ""}
    # From memory
    bp = team_mgr.memory.get("leader", "battle_plan", {})
    if isinstance(bp, dict):
        return _json_safe(bp)
    return {"plan": "", "date": ""}


@app.get("/team/review")
def team_review():
    """Get latest coach daily review (LLM report + stats)."""
    if not team_mgr:
        return {}
    today_decisions = team_mgr.get_today_decisions()
    for d in reversed(today_decisions):
        if d.get("agent") == "coach" and d.get("action") == "daily_review":
            try:
                import json as _j
                data = d.get("data", "{}")
                parsed = _j.loads(data) if isinstance(data, str) else data
                return _json_safe({
                    "date": parsed.get("date", ""),
                    "daily_pnl": parsed.get("daily_pnl", 0),
                    "total_decisions": parsed.get("total_decisions", 0),
                    "llm_report": parsed.get("llm_report", ""),
                    "gene_mutations": parsed.get("gene_mutations", []),
                    "veto_missed_profit": parsed.get("veto_missed_profit", 0),
                })
            except Exception:
                pass
    # Fallback: check memory for historical data
    pnl_hist = team_mgr.memory.get("coach", "daily_pnl_history", [])
    return _json_safe({"history": pnl_hist[-7:] if pnl_hist else []})


@app.get("/team/performance")
def team_performance():
    """Team historical performance: P&L trend, win rate, agent accuracy."""
    if not team_mgr:
        return {}
    mem = team_mgr.memory

    # Daily P&L history (from coach memory)
    pnl_history = mem.get("coach", "daily_pnl_history", [])

    # Cumulative stats
    total_days = mem.get("coach", "total_review_days", 0)
    profit_days = mem.get("coach", "profitable_days", 0)
    loss_days = mem.get("coach", "loss_days", 0)
    win_rate = round(profit_days / max(total_days, 1), 4)

    # Agent accuracies (single + multi-window)
    accuracies = {}
    multi_acc = {}
    for role in ("leader", "macro", "hunter", "risk"):
        accuracies[role] = mem.get(role, "accuracy_30d", 0.5)
        multi_acc[role] = team_mgr.store.multi_accuracy(role)

    # Agent weights (from leader)
    weights = team_mgr.leader._agent_weights

    # Gene mutation history
    mutations = mem.get("coach", "gene_mutation_history", [])

    # Hunter learning stats
    hot = mem.get("hunter", "hot_strategies", [])
    cold = mem.get("hunter", "cold_sectors", [])
    proposals = mem.get("hunter", "daily_proposals", [])

    # Strategy type win rates
    type_stats = {}
    for stype in ("trend", "breakout", "volume", "other"):
        ts = mem.get("hunter", f"type_{stype}", {"wins": 0, "total": 0})
        if isinstance(ts, dict) and ts.get("total", 0) > 0:
            type_stats[stype] = {
                "wins": ts["wins"], "total": ts["total"],
                "win_rate": round(ts["wins"] / ts["total"], 3),
            }

    return _json_safe({
        "pnl_history": pnl_history[-30:],
        "total_days": total_days,
        "profitable_days": profit_days,
        "loss_days": loss_days,
        "win_rate": win_rate,
        "agent_accuracies": accuracies,
        "agent_weights": weights,
        "gene_mutations": mutations[-10:],
        "hunter_hot_strategies": hot,
        "hunter_cold_sectors": cold,
        "hunter_proposals": proposals[-7:],
        "strategy_type_stats": type_stats,
        "multi_accuracy": multi_acc,
        "blacklist": [e.get("code","") for e in mem.get("hunter", "stop_loss_blacklist", []) if isinstance(e, dict)],
        "macro_rating_history": mem.get("macro", "rating_history", [])[-30:],
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
