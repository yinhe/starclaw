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
        self._last_asset: dict = {}  # cache last successful asset query per account
        self._config_accounts: list = self._load_config_accounts()

        if HAS_XTQUANT:
            self._init_xtquant()
        else:
            self._connected = True  # mock mode always "connected"
            logger.info("QMT client initialized in MOCK mode")

    @staticmethod
    def _load_config_accounts() -> list:
        """Load account IDs from config.yaml."""
        import os, yaml
        cfg_path = os.path.join(os.path.dirname(__file__), "config.yaml")
        try:
            with open(cfg_path, "r", encoding="utf-8") as f:
                cfg = yaml.safe_load(f)
            return [a["id"] for a in cfg.get("accounts", []) if a.get("id")]
        except Exception as e:
            logger.warning(f"Failed to load accounts from config: {e}")
            return []

    def _init_xtquant(self):
        """Initialize real xtquant connection."""
        import os
        import glob
        import time

        session_id = int(os.getenv("QMT_SESSION_ID", "654321"))

        # Build candidate paths: config.yaml path first, then env var, then auto-detect
        candidates = []

        # 1) config.yaml qmt.path (highest priority)
        cfg_path = os.path.join(os.path.dirname(__file__), "config.yaml")
        try:
            import yaml
            with open(cfg_path, "r", encoding="utf-8") as f:
                cfg = yaml.safe_load(f)
            qmt_cfg_path = cfg.get("qmt", {}).get("path", "")
            if qmt_cfg_path and os.path.isdir(qmt_cfg_path):
                candidates.append(qmt_cfg_path)
            cfg_session = cfg.get("qmt", {}).get("session_id")
            if cfg_session:
                session_id = int(cfg_session)
        except Exception as e:
            logger.warning(f"Failed to read QMT config: {e}")

        # 2) Environment variable
        env_path = os.getenv("QMT_PATH", "")
        if env_path and os.path.isdir(env_path) and env_path not in candidates:
            candidates.append(env_path)

        # 3) Auto-detect only if no config/env path found
        if not candidates:
            for drive in ["C:\\", "D:\\", "E:\\", "F:\\"]:
                for qmt_root in glob.glob(os.path.join(drive, "*QMT*")):
                    udm = os.path.join(qmt_root, "userdata_mini")
                    if os.path.isdir(udm) and udm not in candidates:
                        candidates.append(udm)

        logger.info(f"QMT candidate paths: {candidates}")

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
                    # Subscribe all configured trading accounts
                    for acct_id in self._config_accounts:
                        try:
                            acc = StockAccount(acct_id)
                            self._trader.subscribe(acc)
                            self._accounts[acct_id] = acc
                            logger.info(f"Subscribed account: {acct_id}")
                        except Exception as e:
                            logger.warning(f"Account subscribe failed for {acct_id}: {e}")
                    # Subscribe whole market quotes so get_full_tick returns
                    # real-time data for ALL stocks (not just held positions)
                    try:
                        xtdata.subscribe_whole_quote(["SH", "SZ"])
                        logger.info("Subscribed whole market quotes (SH+SZ)")
                    except Exception as e:
                        logger.warning(f"subscribe_whole_quote failed: {e}")
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

    def get_open_orders(self, account: str) -> list:
        """Get all open (unfilled/partially filled) orders."""
        if not HAS_XTQUANT:
            return []

        stock_account = StockAccount(account)
        try:
            orders = self._trader.query_stock_orders(stock_account)
            open_orders = []
            for o in orders:
                # xtquant order status: 0=submitted, 1=partial, 2=filled, 3=cancelled, etc.
                status = getattr(o, 'order_status', -1)
                if status in (0, 1):  # submitted or partially filled
                    open_orders.append({
                        "order_id": str(o.order_id),
                        "code": o.stock_code,
                        "direction": "buy" if o.order_type == 23 else "sell",  # 23=buy, 24=sell
                        "price": o.price,
                        "volume": o.order_volume,
                        "filled_volume": getattr(o, 'traded_volume', 0),
                        "status": status,
                        "order_time": getattr(o, 'order_time', ''),
                    })
            return open_orders
        except Exception as e:
            logger.warning(f"get_open_orders error: {e}")
            return []

    def get_all_orders(self, account: str) -> list:
        """Get ALL orders (including filled/cancelled) for history import."""
        if not HAS_XTQUANT:
            return []
        stock_account = StockAccount(account)
        try:
            orders = self._trader.query_stock_orders(stock_account)
            result = []
            for o in orders:
                status = getattr(o, 'order_status', -1)
                result.append({
                    "order_id": str(getattr(o, 'order_id', '')),
                    "code": getattr(o, 'stock_code', ''),
                    "direction": "buy" if getattr(o, 'order_type', 0) == 23 else "sell",
                    "price": getattr(o, 'price', 0),
                    "order_volume": getattr(o, 'order_volume', 0),
                    "traded_volume": getattr(o, 'traded_volume', 0),
                    "traded_price": getattr(o, 'traded_price', 0),
                    "status": status,
                    "status_msg": getattr(o, 'status_msg', ''),
                    "order_time": getattr(o, 'order_time', ''),
                })
            return result
        except Exception as e:
            logger.warning(f"get_all_orders error: {e}")
            return []

    def get_dealt_trades(self, account: str) -> list:
        """Get all dealt/filled trades (成交记录) from QMT."""
        if not HAS_XTQUANT:
            return []
        stock_account = StockAccount(account)
        try:
            # query_stock_trades returns filled trade records
            if not hasattr(self._trader, 'query_stock_trades'):
                logger.warning("[qmt] query_stock_trades not available")
                return []
            trades = self._trader.query_stock_trades(stock_account)
            result = []
            for t in trades:
                result.append({
                    "code": getattr(t, 'stock_code', ''),
                    "direction": "buy" if getattr(t, 'order_type', 0) == 23 else "sell",
                    "price": getattr(t, 'traded_price', 0),
                    "volume": getattr(t, 'traded_volume', 0),
                    "amount": getattr(t, 'traded_amount', 0),
                    "order_id": str(getattr(t, 'order_id', '')),
                    "traded_id": str(getattr(t, 'traded_id', '')),
                    "traded_time": getattr(t, 'traded_time', ''),
                })
            return result
        except Exception as e:
            logger.warning(f"get_dealt_trades error: {e}")
            return []

    def export_history_deals(self, account: str, start_time: str = "", end_time: str = "") -> list:
        """Export historical deal records using query_data/export_data (supports date range).

        Unlike query_stock_trades which only returns today's deals,
        this uses the generic export_data interface to get historical records.

        Args:
            account: Account ID string
            start_time: Start date string like '20260101' (optional)
            end_time: End date string like '20260402' (optional)

        Returns:
            list of dict with deal info, or empty list on failure
        """
        if not HAS_XTQUANT:
            return []
        import tempfile, os
        stock_account = StockAccount(account)
        tmp_path = os.path.join(tempfile.gettempdir(), f'qmt_deals_{account}.csv')
        try:
            # Try query_data first (returns DataFrame, auto-deletes csv)
            if hasattr(self._trader, 'query_data'):
                data = self._trader.query_data(stock_account, tmp_path, 'deal', start_time, end_time)
                if isinstance(data, dict) and 'error' in data:
                    logger.warning(f"[qmt] query_data error: {data}")
                    return []
                # data should be a DataFrame
                import pandas as pd
                if isinstance(data, pd.DataFrame) and not data.empty:
                    result = []
                    for _, row in data.iterrows():
                        code = str(row.get('stock_code', row.get('instrument_id', '')))
                        order_type = int(row.get('order_type', 0))
                        direction = "buy" if order_type == 23 else "sell"
                        result.append({
                            "code": code,
                            "direction": direction,
                            "price": float(row.get('traded_price', 0)),
                            "volume": int(row.get('traded_volume', 0)),
                            "amount": float(row.get('traded_amount', 0)),
                            "order_id": str(row.get('order_id', '')),
                            "traded_id": str(row.get('traded_id', '')),
                            "traded_time": str(row.get('traded_time', '')),
                            "order_type": order_type,
                            "offset_flag": int(row.get('offset_flag', 0)),
                        })
                    logger.info(f"[qmt] export_history_deals: got {len(result)} records via query_data")
                    return result
                else:
                    logger.info(f"[qmt] query_data returned empty or non-DataFrame: {type(data)}")
                    return []

            # Fallback: export_data (writes to csv, we parse it)
            elif hasattr(self._trader, 'export_data'):
                resp = self._trader.export_data(stock_account, tmp_path, 'deal', start_time, end_time)
                if isinstance(resp, dict) and 'error' in resp:
                    logger.warning(f"[qmt] export_data error: {resp}")
                    return []
                if os.path.exists(tmp_path):
                    import pandas as pd
                    df = pd.read_csv(tmp_path, encoding='gbk')
                    os.remove(tmp_path)
                    result = []
                    for _, row in df.iterrows():
                        code = str(row.get('stock_code', row.get('instrument_id', '')))
                        order_type = int(row.get('order_type', 0))
                        direction = "buy" if order_type == 23 else "sell"
                        result.append({
                            "code": code,
                            "direction": direction,
                            "price": float(row.get('traded_price', 0)),
                            "volume": int(row.get('traded_volume', 0)),
                            "amount": float(row.get('traded_amount', 0)),
                            "traded_time": str(row.get('traded_time', '')),
                            "order_type": order_type,
                        })
                    logger.info(f"[qmt] export_history_deals: got {len(result)} records via export_data")
                    return result
                return []
            else:
                logger.warning("[qmt] Neither query_data nor export_data available")
                return []
        except Exception as e:
            logger.error(f"[qmt] export_history_deals error: {e}", exc_info=True)
            # Clean up temp file
            try:
                if os.path.exists(tmp_path):
                    os.remove(tmp_path)
            except Exception:
                pass
            return []

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
            total = asset.total_asset or 0
            mv = asset.market_value or 0
            cash = asset.cash or 0
            frozen = asset.frozen_cash or 0
            pos_ratio = (mv / total * 100) if total > 0 else 0
            logger.info(f"[account] asset OK: acct={account} total={total} mv={mv} cash={cash}")
            result = {
                "account": account,
                "total_assets": total,
                "available": cash,
                "frozen": frozen,
                "market_value": mv,
                "today_profit": getattr(asset, "today_profit", None) or getattr(asset, "m_dDayProfit", None) or 0,
                "float_pnl": getattr(asset, "position_profit", None) or getattr(asset, "m_dPositionProfit", None) or 0,
                "withdrawable": getattr(asset, "withdrawable", None) or getattr(asset, "m_dFetchBalance", None) or max(cash - frozen, 0),
                "position_ratio": round(pos_ratio, 1),
            }
            self._last_asset[account] = result  # cache successful result
            return result

        # QMT SDK rate-limits query_stock_asset; return cached result if available
        cached = self._last_asset.get(account)
        if cached:
            logger.info(f"[account] using cache for acct={account}")
            return cached

        # No cache available — true fallback
        logger.warning(f"[account] query_stock_asset returned None, no cache, acct={account}, keys={list(self._last_asset.keys())}")
        return {
            "account": account,
            "total_assets": 0, "available": 0, "frozen": 0, "market_value": 0,
            "today_profit": 0, "float_pnl": 0, "withdrawable": 0, "position_ratio": 0,
            "_fallback": True,
        }

    def get_positions(self, account: str) -> list:
        """Get current positions with stock names and correct P&L."""
        if not HAS_XTQUANT:
            return [
                {"code": "600519.SH", "name": "贵州茅台", "volume": 100, "avail_volume": 100,
                 "cost_price": 1800.0, "market_price": 1850.0, "pnl_float": 5000.0},
            ]

        stock_account = StockAccount(account)
        positions = self._trader.query_stock_positions(stock_account)

        # Batch-fetch realtime prices for all held stocks via xtdata
        codes = [p.stock_code for p in positions]
        realtime_prices = {}
        last_closes = {}
        if codes:
            # Try get_full_tick first (realtime snapshot)
            try:
                ticks = xtdata.get_full_tick(codes)
                if ticks:
                    sample_code = codes[0]
                    sample = ticks.get(sample_code)
                    if sample:
                        logger.info(f"[positions] get_full_tick keys for {sample_code}: {list(sample.keys()) if isinstance(sample, dict) else type(sample)}")
                    for code, tick in ticks.items():
                        if tick and isinstance(tick, dict):
                            price = float(tick.get("lastPrice", 0) or tick.get("last_price", 0) or tick.get("price", 0) or tick.get("close", 0))
                            if price > 0:
                                realtime_prices[code] = price
                            lc = float(tick.get("lastClose", 0) or tick.get("last_close", 0) or 0)
                            if lc > 0:
                                last_closes[code] = lc
            except Exception as e:
                logger.warning(f"[positions] get_full_tick failed: {e}")
            # Fallback: 1m bars for any missing
            missing = [c for c in codes if c not in realtime_prices]
            if missing:
                try:
                    md = xtdata.get_market_data_ex([], missing, period="1m", count=1)
                    for code, df in md.items():
                        if df is not None and len(df) > 0:
                            price = float(df["close"].values[-1])
                            if price > 0:
                                realtime_prices[code] = price
                except Exception as e:
                    logger.warning(f"[positions] 1m bar fallback failed: {e}")
            logger.info(f"[positions] realtime prices: {len(realtime_prices)}/{len(codes)} — {realtime_prices}")

        result = []
        for p in positions:
            vol = p.volume
            if vol <= 0:
                continue  # skip cleared positions
            cost = p.open_price
            # Use realtime price if available, otherwise fall back to QMT position data
            if p.stock_code in realtime_prices:
                mkt_price = realtime_prices[p.stock_code]
            else:
                mkt_price = p.market_value / vol if vol > 0 else 0
            pnl = round((mkt_price - cost) * vol, 2) if vol > 0 else 0

            # Query stock name via xtdata (handle GBK encoding on Windows)
            name = ""
            try:
                detail = xtdata.get_instrument_detail(p.stock_code)
                if detail:
                    raw = detail.get("InstrumentName", "")
                    if isinstance(raw, bytes):
                        name = raw.decode("gbk", errors="replace")
                    elif isinstance(raw, str):
                        try:
                            name = raw.encode("latin1").decode("gbk", errors="replace")
                        except (UnicodeDecodeError, UnicodeEncodeError):
                            name = raw
            except Exception:
                pass

            result.append({
                "code": p.stock_code,
                "name": name,
                "volume": vol,
                "avail_volume": p.can_use_volume,
                "cost_price": cost,
                "market_price": round(mkt_price, 4),
                "last_close": last_closes.get(p.stock_code, 0),
                "pnl_float": pnl,
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
