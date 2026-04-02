"""
Equity History Store — 资产净值曲线存储。

SQLite 存储每日总资产快照，供前端绘制日/周/月/年收益曲线。
每交易日 15:05 自动记录一次快照 (由 _run_equity_snapshot 后台任务驱动)。
"""

import logging
import os
import sqlite3
from datetime import datetime, date, timedelta
from typing import List, Dict, Optional

logger = logging.getLogger("equity_store")

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "equity.db")


def _conn() -> sqlite3.Connection:
    conn = sqlite3.connect(DB_PATH)
    conn.execute("PRAGMA journal_mode=WAL")
    conn.execute("""
        CREATE TABLE IF NOT EXISTS equity_history (
            date       TEXT PRIMARY KEY,
            total_asset REAL NOT NULL DEFAULT 0,
            market_value REAL NOT NULL DEFAULT 0,
            cash       REAL NOT NULL DEFAULT 0,
            float_pnl  REAL NOT NULL DEFAULT 0,
            realized_pnl REAL NOT NULL DEFAULT 0,
            positions  INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
        )
    """)
    conn.commit()
    return conn


def record_snapshot(
    dt_str: str,
    total_asset: float,
    market_value: float,
    cash: float,
    float_pnl: float,
    realized_pnl: float = 0,
    positions: int = 0,
):
    """Record or update a daily equity snapshot."""
    conn = _conn()
    try:
        conn.execute("""
            INSERT INTO equity_history (date, total_asset, market_value, cash, float_pnl, realized_pnl, positions)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(date) DO UPDATE SET
                total_asset=excluded.total_asset,
                market_value=excluded.market_value,
                cash=excluded.cash,
                float_pnl=excluded.float_pnl,
                realized_pnl=excluded.realized_pnl,
                positions=excluded.positions,
                created_at=datetime('now','localtime')
        """, (dt_str, total_asset, market_value, cash, float_pnl, realized_pnl, positions))
        conn.commit()
        logger.info(f"[equity] snapshot saved: {dt_str} total={total_asset:.2f} mv={market_value:.2f}")
    finally:
        conn.close()


def get_history(days: int = 365) -> List[Dict]:
    """Get equity history for the last N days."""
    conn = _conn()
    try:
        cutoff = (date.today() - timedelta(days=days)).isoformat()
        rows = conn.execute(
            "SELECT date, total_asset, market_value, cash, float_pnl, realized_pnl, positions "
            "FROM equity_history WHERE date >= ? ORDER BY date ASC",
            (cutoff,)
        ).fetchall()
        result = []
        for r in rows:
            result.append({
                "date": r[0],
                "total_asset": r[1],
                "market_value": r[2],
                "cash": r[3],
                "float_pnl": r[4],
                "realized_pnl": r[5],
                "positions": r[6],
            })
        return result
    finally:
        conn.close()


def get_equity_curve(period: str = "1y") -> Dict:
    """
    Return equity curve data for a given period.
    period: '1w', '1m', '3m', '6m', '1y', 'all'
    Returns: { points: [...], summary: { start, end, return_pct, max_drawdown, ... } }
    """
    days_map = {"1w": 7, "1m": 30, "3m": 90, "6m": 180, "1y": 365, "all": 9999}
    days = days_map.get(period, 365)
    history = get_history(days)

    if not history:
        return {"points": [], "summary": {}}

    # Compute return rates relative to first point
    base = history[0]["total_asset"]
    points = []
    peak = base
    max_dd = 0

    for h in history:
        val = h["total_asset"]
        ret_pct = ((val - base) / base * 100) if base > 0 else 0
        # Max drawdown
        if val > peak:
            peak = val
        dd = ((peak - val) / peak * 100) if peak > 0 else 0
        if dd > max_dd:
            max_dd = dd

        points.append({
            "date": h["date"],
            "total": round(val, 2),
            "mv": round(h["market_value"], 2),
            "cash": round(h["cash"], 2),
            "ret": round(ret_pct, 2),
            "pnl": round(h["float_pnl"], 2),
        })

    first_val = history[0]["total_asset"]
    last_val = history[-1]["total_asset"]
    total_return = ((last_val - first_val) / first_val * 100) if first_val > 0 else 0

    summary = {
        "start_date": history[0]["date"],
        "end_date": history[-1]["date"],
        "start_val": round(first_val, 2),
        "end_val": round(last_val, 2),
        "return_pct": round(total_return, 2),
        "max_drawdown": round(max_dd, 2),
        "days": len(history),
    }

    return {"points": points, "summary": summary}


def seed_from_positions(qmt_client, account: str):
    """
    Seed today's equity snapshot from current QMT data.
    Called on startup and periodically to ensure we have today's data.
    """
    try:
        info = qmt_client.get_account_info(account)
        total = info.get("total_assets", 0)
        mv = info.get("market_value", 0)
        cash = info.get("available", 0)
        fpnl = info.get("float_pnl", 0)

        # If total is 0 (fallback mode), compute from positions
        if total <= 0:
            positions = qmt_client.get_positions(account)
            mv = sum(p.get("market_price", 0) * p.get("volume", 0) for p in positions)
            fpnl = sum(p.get("pnl_float", 0) for p in positions)
            total = mv  # best estimate without cash info
            n_pos = len([p for p in positions if p.get("volume", 0) > 0])
        else:
            positions = qmt_client.get_positions(account)
            n_pos = len([p for p in positions if p.get("volume", 0) > 0])

        if total > 0:
            today_str = date.today().isoformat()
            record_snapshot(today_str, total, mv, cash, fpnl, 0, n_pos)
            return True
    except Exception as e:
        logger.error(f"[equity] seed failed: {e}")
    return False
