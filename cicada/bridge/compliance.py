"""
Cicada 🪰 Compliance — 合规检查 (黑名单 / 退订 / 频控)
"""

import logging
from collections import defaultdict
from datetime import datetime, timedelta
from typing import Optional

from crm_manager import CRMManager

logger = logging.getLogger("cicada.compliance")

# 退订触发关键词
UNSUBSCRIBE_TRIGGERS = [
    "不需要", "别打了", "不要再打", "加入黑名单",
    "投诉", "骚扰", "报警", "工信部",
]

# DTMF 退订按键
UNSUBSCRIBE_DTMF = ["9"]


class ComplianceChecker:
    """
    合规检查器
    职责：
    1. 黑名单检查
    2. 频控检查（每号码每天/每月上限）
    3. 退订检测
    4. 呼叫号码频率限制
    """

    def __init__(self, crm: CRMManager, config: dict = None):
        self.crm = crm
        config = config or {}

        self.per_number_daily = config.get("per_number_daily", 1)
        self.per_number_monthly = config.get("per_number_monthly", 3)
        self.per_caller_hourly = config.get("per_caller_hourly", 60)
        self.per_caller_daily = config.get("per_caller_daily", 200)
        self.blacklist_cooldown_days = config.get("blacklist_cooldown_days", 30)

        # 内存计数器（每日重置）
        self._caller_hourly: dict[str, list[datetime]] = defaultdict(list)
        self._caller_daily: dict[str, int] = defaultdict(int)
        self._number_daily: dict[str, int] = defaultdict(int)
        self._last_reset_date: Optional[str] = None

    def should_block(self, phone: str, caller: str = "") -> bool:
        """
        综合合规检查，返回 True 则不应拨打
        """
        self._reset_if_new_day()

        # 1. 黑名单
        if self.crm.is_blacklisted(phone):
            logger.debug(f"[compliance] blocked (blacklist): {phone[-4:]}")
            return True

        # 2. 该号码今天已拨打次数
        phone_hash = CRMManager.hash_phone(phone)
        if self._number_daily[phone_hash] >= self.per_number_daily:
            logger.debug(f"[compliance] blocked (daily limit): {phone[-4:]}")
            return True

        # 3. 外显号码频率
        if caller:
            if self._caller_daily[caller] >= self.per_caller_daily:
                logger.debug(f"[compliance] blocked (caller daily): {caller}")
                return True

            # 小时频率
            now = datetime.now()
            hour_ago = now - timedelta(hours=1)
            self._caller_hourly[caller] = [
                t for t in self._caller_hourly[caller] if t > hour_ago
            ]
            if len(self._caller_hourly[caller]) >= self.per_caller_hourly:
                logger.debug(f"[compliance] blocked (caller hourly): {caller}")
                return True

        return False

    def record_call(self, phone: str, caller: str = ""):
        """记录一次拨打（更新计数器）"""
        phone_hash = CRMManager.hash_phone(phone)
        self._number_daily[phone_hash] += 1
        if caller:
            self._caller_daily[caller] += 1
            self._caller_hourly[caller].append(datetime.now())

    def check_unsubscribe(self, text: str) -> bool:
        """检测客户语音中是否包含退订信号"""
        text_lower = text.lower().strip()
        for trigger in UNSUBSCRIBE_TRIGGERS:
            if trigger in text_lower:
                return True
        return False

    def check_dtmf_unsubscribe(self, dtmf: str) -> bool:
        """检测 DTMF 按键退订"""
        return dtmf in UNSUBSCRIBE_DTMF

    def handle_unsubscribe(self, phone: str, reason: str = "客户要求退订"):
        """处理退订请求"""
        self.crm.add_to_blacklist(phone, reason=reason)
        logger.info(f"[compliance] unsubscribed: {phone[-4:]} reason={reason}")

    def _reset_if_new_day(self):
        """每日重置计数器"""
        today = datetime.now().strftime("%Y%m%d")
        if self._last_reset_date != today:
            self._number_daily.clear()
            self._caller_daily.clear()
            self._caller_hourly.clear()
            self._last_reset_date = today
            logger.info("[compliance] daily counters reset")

    def get_stats(self) -> dict:
        return {
            "blacklist_count": 0,  # TODO: query from DB
            "today_blocks": sum(1 for v in self._number_daily.values() if v >= self.per_number_daily),
            "caller_usage": dict(self._caller_daily),
        }
