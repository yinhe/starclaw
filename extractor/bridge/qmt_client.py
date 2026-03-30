"""
QMT Client — wraps xtquant SDK for miniQMT interaction.
Falls back to mock mode if xtquant is not available (dev environment).
"""

import logging
import threading
from typing import List, Optional

logger = logging.getLogger("qmt_client")

try:
    from xtquant import xtdata
    from xtquant.xttrader import XtQuantTrader, XtQuantTraderCallback
    from xtquant.xttype import StockAccount
    HAS_XTQUANT = True
except ImportError:
    HAS_XTQUANT = False
    logger.warning("xtquant not found, running in MOCK mode")


class _QMTCallback(XtQuantTraderCallback if HAS_XTQUANT else object):
    """Callback handler required by xtquant before connect()."""
    def on_disconnected(self):
        logger.warning("[qmt_callback] disconnected from QMT")
    def on_account_status(self, status):
        logger.info(f"[qmt_callback] account status: {status}")
    def on_stock_order(self, order):
        logger.info(f"[qmt_callback] order update: {order}")
    def on_stock_trade(self, trade):
        logger.info(f"[qmt_callback] trade filled: {trade}")
    def on_order_error(self, order_error):
        logger.warning(f"[qmt_callback] order error: {order_error}")
    def on_cancel_error(self, cancel_error):
        logger.warning(f"[qmt_callback] cancel error: {cancel_error}")


class QMTClient:
    """Unified QMT client. Uses xtquant when available, mock otherwise."""

    def __init__(self):
        self._connected = False
        self._trader: Optional[object] = None
        self._accounts: dict = {}  # account_id → StockAccount

        if HAS_XTQUANT:
            self._init_xtquant()
        else:
            self._connected = True  # mock mode always "connected"
            logger.info("QMT client initialized in MOCK mode")

    def _init_xtquant(self):
        """Initialize real xtquant connection."""
        import os
        import glob
        import time

        session_id = int(os.getenv("QMT_SESSION_ID", "654321"))

        # Build candidate paths: env var first, then auto-detect
        candidates = []
        env_path = os.getenv("QMT_PATH", "")
        if env_path and os.path.isdir(env_path):
            candidates.append(env_path)

        # Auto-detect: find all *QMT*\userdata_mini directories on all drives
        for drive in ["C:\\", "D:\\", "E:\\", "F:\\"]:
            for qmt_root in glob.glob(os.path.join(drive, "*QMT*")):
                udm = os.path.join(qmt_root, "userdata_mini")
                if os.path.isdir(udm) and udm not in candidates:
                    candidates.append(udm)

        logger.info(f"QMT candidate paths: {candidates}")

        # Try each path, prefer the one with more files (active miniQMT)
        candidates.sort(key=lambda p: len(os.listdir(p)), reverse=True)

        for qmt_path in candidates:
            try:
                logger.info(f"Trying QMT path: {qmt_path}")
                self._trader = XtQuantTrader(qmt_path, session_id)
                self._trader.register_callback(_QMTCallback())
                self._trader.start()
                time.sleep(1)
                connect_result = self._trader.connect()

                if connect_result == 0:
                    self._connected = True
                    logger.info(f"QMT connected successfully via {qmt_path}")
                    # Subscribe trading account so orders can be placed
                    try:
                        acc = StockAccount("27800348")
                        self._trader.subscribe(acc)
                        logger.info(f"Subscribed account: 27800348")
                    except Exception as e:
                        logger.warning(f"Account subscribe failed: {e}")
                    return
                else:
                    logger.warning(f"QMT connect returned {connect_result} for {qmt_path}")
                    self._trader.stop()
            except Exception as e:
                logger.warning(f"QMT connect error for {qmt_path}: {e}")

        logger.error("QMT: all candidate paths failed to connect")


    def is_connected(self) -> bool:
        return self._connected

    def disconnect(self):
        if self._trader:
            self._trader.stop()
        self._connected = False

    def submit_order(self, account: str, code: str, direction: str,
                     price: float, volume: int, order_type: str = "limit") -> str:
        """Submit an order. Returns QMT order ID."""
        if not HAS_XTQUANT:
            mock_id = f"MOCK-{account}-{code}-{direction}-{volume}"
            logger.info(f"[MOCK] submit order: {mock_id}")
            return mock_id

        from xtquant.xtconstant import STOCK_BUY, STOCK_SELL, FIX_PRICE, LATEST_PRICE
        stock_account = StockAccount(account)
        side = STOCK_BUY if direction == "buy" else STOCK_SELL
        price_type = LATEST_PRICE if order_type == "market" else FIX_PRICE

        order_id = self._trader.order_stock(
            account=stock_account,
            stock_code=code,
            order_type=side,
            order_volume=volume,
            price_type=price_type,
            price=price,
        )
        logger.info(f"Order submitted: {order_id} {account} {direction} {code} {price} x {volume}")
        return str(order_id)

    def cancel_order(self, account: str, order_id: str):
        """Cancel a pending order."""
        if not HAS_XTQUANT:
            logger.info(f"[MOCK] cancel order: {order_id}")
            return

        stock_account = StockAccount(account)
        self._trader.cancel_order_stock(stock_account, int(order_id))
        logger.info(f"Order cancelled: {order_id}")

    def get_account_info(self, account: str) -> dict:
        """Get account balance info."""
        if not HAS_XTQUANT:
            return {
                "account": account,
                "total_assets": 1000000.0,
                "available": 800000.0,
                "frozen": 200000.0,
                "market_value": 200000.0,
            }

        stock_account = StockAccount(account)
        asset = self._trader.query_stock_asset(stock_account)
        if asset:
            return {
                "account": account,
                "total_assets": asset.total_asset,
                "available": asset.cash,
                "frozen": asset.frozen_cash,
                "market_value": asset.market_value,
            }
        return {"account": account, "total_assets": 0, "available": 0, "frozen": 0, "market_value": 0}

    def get_positions(self, account: str) -> list:
        """Get current positions."""
        if not HAS_XTQUANT:
            return [
                {"code": "600519.SH", "name": "贵州茅台", "volume": 100, "avail_volume": 100,
                 "cost_price": 1800.0, "market_price": 1850.0, "pnl_float": 5000.0},
            ]

        stock_account = StockAccount(account)
        positions = self._trader.query_stock_positions(stock_account)
        result = []
        for p in positions:
            result.append({
                "code": p.stock_code,
                "name": p.stock_name if hasattr(p, 'stock_name') else "",
                "volume": p.volume,
                "avail_volume": p.can_use_volume,
                "cost_price": p.open_price,
                "market_price": p.market_value / p.volume if p.volume > 0 else 0,
                "pnl_float": p.on_road_volume,  # placeholder
            })
        return result

    def _download_with_timeout(self, codes, period, timeout=10):
        """Download data with timeout to prevent blocking."""
        done = threading.Event()
        def _dl():
            try:
                xtdata.download_history_data2(codes, period=period, start_time="", end_time="")
            except Exception:
                pass
            finally:
                done.set()
        t = threading.Thread(target=_dl, daemon=True)
        t.start()
        done.wait(timeout=timeout)
        return done.is_set()

    def get_quotes(self, codes: List[str]) -> list:
        """Get real-time quotes for stock codes."""
        if not HAS_XTQUANT:
            return [{"code": c, "name": "模拟", "price": 100.0 + i,
                      "open": 99.0, "high": 101.0, "low": 98.0, "pre_close": 99.5,
                      "volume": 10000, "amount": 1000000.0, "timestamp": "2026-03-29 15:00:00"}
                     for i, c in enumerate(codes)]

        self._download_with_timeout(codes, "tick", timeout=10)
        result = []
        for code in codes:
            try:
                tick = xtdata.get_full_tick([code])
                if code in tick and tick[code]:
                    t = tick[code]
                    result.append({
                        "code": code,
                        "name": "",
                        "price": t.get("lastPrice", 0),
                        "open": t.get("open", 0),
                        "high": t.get("high", 0),
                        "low": t.get("low", 0),
                        "pre_close": t.get("lastClose", 0),
                        "volume": t.get("volume", 0),
                        "amount": t.get("amount", 0),
                        "timestamp": "",
                    })
                else:
                    # Fallback: try get_market_data_ex for latest daily bar
                    try:
                        data = xtdata.get_market_data_ex([], [code], period="1d", count=1)
                        if code in data and len(data[code]) > 0:
                            row = data[code].iloc[-1]
                            result.append({
                                "code": code,
                                "name": "",
                                "price": float(row.get("close", 0)),
                                "open": float(row.get("open", 0)),
                                "high": float(row.get("high", 0)),
                                "low": float(row.get("low", 0)),
                                "pre_close": float(row.get("preClose", 0)),
                                "volume": int(row.get("volume", 0)),
                                "amount": float(row.get("amount", 0)),
                                "timestamp": "",
                                "source": "daily_fallback",
                            })
                        else:
                            result.append({"code": code, "price": 0, "note": "market_closed_no_cache"})
                    except Exception:
                        result.append({"code": code, "price": 0, "note": "market_closed_no_cache"})
            except Exception as e:
                err = repr(e).encode('ascii', 'replace').decode('ascii')
                result.append({"code": code, "error": err})
        return result

    def get_kline(self, code: str, period: str = "1d", count: int = 100) -> list:
        """Get K-line (candlestick) data."""
        if not HAS_XTQUANT:
            import random
            base = 100.0
            result = []
            for i in range(count):
                o = base + random.uniform(-2, 2)
                c = o + random.uniform(-3, 3)
                h = max(o, c) + random.uniform(0, 2)
                l = min(o, c) - random.uniform(0, 2)
                result.append({"date": f"2026-01-{i+1:02d}", "open": round(o, 2), "high": round(h, 2),
                               "low": round(l, 2), "close": round(c, 2), "volume": random.randint(5000, 50000),
                               "amount": random.uniform(500000, 5000000)})
                base = c
            return result

        self._download_with_timeout([code], period, timeout=15)
        data = xtdata.get_market_data_ex([], [code], period=period, count=count)
        if code not in data:
            return []
        df = data[code]
        result = []
        for idx, row in df.iterrows():
            result.append({
                "date": str(idx),
                "open": row.get("open", 0),
                "high": row.get("high", 0),
                "low": row.get("low", 0),
                "close": row.get("close", 0),
                "volume": int(row.get("volume", 0)),
                "amount": row.get("amount", 0),
            })
        return result
