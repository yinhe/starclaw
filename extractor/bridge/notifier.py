"""
Notifier — 交易通知推送模块。

支持:
  1. 企业微信 Webhook (群机器人)
  2. Server酱 (微信推送)
  3. 本地日志 (始终启用)

触发时机:
  - 买入成交
  - 卖出成交 (含止损/止盈原因)
  - 风控触发 (日亏损暂停/熔断)
  - 日终结算报告
  - 系统异常 (QMT断连等)
"""

import logging
import os
from datetime import datetime
from typing import Optional

import httpx
import yaml

logger = logging.getLogger("notifier")

# Load from config.yaml (falls back to env vars)
def _load_notify_config():
    cfg_path = os.path.join(os.path.dirname(__file__), "config.yaml")
    try:
        with open(cfg_path, "r", encoding="utf-8") as f:
            cfg = yaml.safe_load(f)
        n = cfg.get("notify", {})
        return {
            "enabled": n.get("enabled", True),
            "wechat_webhook": n.get("wechat_webhook", "") or os.getenv("WECHAT_WEBHOOK", ""),
            "serverchan_key": n.get("serverchan_key", "") or os.getenv("SERVERCHAN_KEY", ""),
        }
    except Exception:
        return {
            "enabled": os.getenv("NOTIFY_ENABLED", "true").lower() == "true",
            "wechat_webhook": os.getenv("WECHAT_WEBHOOK", ""),
            "serverchan_key": os.getenv("SERVERCHAN_KEY", ""),
        }

_NOTIFY_CFG = _load_notify_config()
WECHAT_WEBHOOK = _NOTIFY_CFG["wechat_webhook"]
SERVERCHAN_KEY = _NOTIFY_CFG["serverchan_key"]
NOTIFY_ENABLED = _NOTIFY_CFG["enabled"]


def _send_wechat(title: str, content: str):
    """Send via 企业微信群机器人 webhook."""
    if not WECHAT_WEBHOOK:
        return
    try:
        httpx.post(WECHAT_WEBHOOK, json={
            "msgtype": "markdown",
            "markdown": {"content": f"### {title}\n{content}"},
        }, timeout=10)
    except Exception as e:
        logger.debug(f"WeChat send error: {e}")


def _send_serverchan(title: str, content: str):
    """Send via Server酱 (https://sct.ftqq.com)."""
    if not SERVERCHAN_KEY:
        return
    try:
        httpx.post(
            f"https://sctapi.ftqq.com/{SERVERCHAN_KEY}.send",
            data={"title": title, "desp": content},
            timeout=10,
        )
    except Exception as e:
        logger.debug(f"ServerChan send error: {e}")


def _send_claw_notification(title: str, content: str, ntype: str = "info"):
    """Write notification directly to Claw's SQLite database.

    This shows up in the Claw UI notification panel (bell icon).
    Uses the same Notification model as Claw's internal system.
    """
    import sqlite3
    import uuid
    from datetime import datetime

    # Find Claw's database
    db_paths = [
        r"C:\Users\Yinhe\.spore\installed\claw\v2026.0329.0852\data\claw.db",
        os.path.join(os.path.dirname(__file__), "..", "..", "claw", "data", "claw.db"),
    ]

    db_path = None
    for p in db_paths:
        if os.path.exists(p):
            db_path = p
            break

    if not db_path:
        return

    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()

        # Get first user_id (single-user local Claw)
        cursor.execute("SELECT id FROM users LIMIT 1")
        row = cursor.fetchone()
        if not row:
            conn.close()
            return
        user_id = row[0]

        # Map type
        type_map = {"info": "info", "warning": "warning", "critical": "warning",
                     "success": "success", "buy": "success", "sell": "info"}
        db_type = type_map.get(ntype, "info")

        cursor.execute(
            "INSERT INTO notifications (id, user_id, task_id, type, title, content, is_read, created_at) "
            "VALUES (?, ?, '', ?, ?, ?, 0, ?)",
            (str(uuid.uuid4()), user_id, db_type, title, content,
             datetime.now().strftime("%Y-%m-%d %H:%M:%S"))
        )
        conn.commit()
        conn.close()
    except Exception as e:
        logger.debug(f"Claw notification error: {e}")


def notify(title: str, content: str, level: str = "info"):
    """Send notification through all configured channels.

    Args:
        title: Short alert title
        content: Detailed message (supports markdown)
        level: 'info', 'warning', 'critical'
    """
    if not NOTIFY_ENABLED:
        return

    now = datetime.now().strftime("%H:%M:%S")
    prefix = {"info": "", "warning": "⚠️ ", "critical": "🚨 "}.get(level, "")
    full_title = f"{prefix}{title}"
    full_content = f"**{now}** {content}"

    # Always log
    log_fn = {"info": logger.info, "warning": logger.warning, "critical": logger.error}.get(level, logger.info)
    log_fn(f"[notify] {full_title}: {content}")

    # Push to all channels
    _send_claw_notification(full_title, full_content, level)  # Claw UI bell icon
    _send_wechat(full_title, full_content)
    _send_serverchan(full_title, full_content)


# === Convenience functions for common trade events ===

def notify_buy(code: str, name: str, price: float, volume: int, reason: str = ""):
    """Notify on buy order."""
    notify("买入下单", f"**{code}** {name}\n价格: {price:.2f} × {volume}股\n{reason}")


def notify_sell(code: str, name: str, price: float, volume: int, pnl_pct: float, reason: str = ""):
    """Notify on sell order."""
    emoji = "🟢" if pnl_pct > 0 else "🔴"
    notify("卖出成交",
           f"{emoji} **{code}** {name}\n价格: {price:.2f} × {volume}股\n盈亏: {pnl_pct:+.2f}%\n原因: {reason}",
           level="info" if pnl_pct > 0 else "warning")


def notify_risk(event: str, detail: str):
    """Notify on risk event."""
    notify(f"风控触发: {event}", detail, level="critical")


def notify_settlement(date: str, pnl: float, star_energy: int):
    """Notify daily settlement result."""
    emoji = "📈" if pnl > 0 else "📉"
    notify(f"{emoji} 日终结算",
           f"日期: {date}\n盈亏: ¥{pnl:+,.2f}\n星能注入: {star_energy}⚡",
           level="info" if pnl >= 0 else "warning")


def notify_system(event: str, detail: str):
    """Notify system event (startup, QMT disconnect, etc)."""
    notify(f"系统: {event}", detail, level="warning")
