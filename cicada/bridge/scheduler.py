"""
Cicada 🪰 Scheduler — 外呼调度引擎
"""

import asyncio
import logging
from datetime import datetime, time as dtime
from typing import Optional

logger = logging.getLogger("cicada.scheduler")


class CallScheduler:
    """
    外呼调度器 — 管理批量外呼的节奏和顺序
    职责：
    1. 判断当前是否在外呼时段
    2. 从待呼队列取号码批次
    3. 控制拨号间隔和并发
    4. 管理重试逻辑
    """

    def __init__(self, config: dict, crm, call_engine):
        self.crm = crm
        self.engine = call_engine

        self.daily_limit = config.get("daily_limit", 800)
        self.max_concurrent = config.get("max_concurrent", 10)
        self.call_interval = config.get("call_interval_seconds", 3)
        self.max_retries = config.get("max_retries", 3)
        self.retry_intervals = config.get("retry_intervals", [3600, 7200, 86400])

        self.schedule_start = self._parse_time(config.get("schedule_start", "09:00"))
        self.schedule_end = self._parse_time(config.get("schedule_end", "18:00"))
        self.lunch_start = self._parse_time(config.get("lunch_break_start", "12:00"))
        self.lunch_end = self._parse_time(config.get("lunch_break_end", "14:00"))
        self.schedule_days = config.get("schedule_days", [1, 2, 3, 4, 5, 6])

        self._running = False
        self._today_called = 0
        self._current_campaign_id: Optional[int] = None
        self._task: Optional[asyncio.Task] = None

    @staticmethod
    def _parse_time(s: str) -> dtime:
        parts = s.split(":")
        return dtime(int(parts[0]), int(parts[1]))

    def is_calling_time(self) -> bool:
        """检查当前是否在外呼时段"""
        now = datetime.now()
        weekday = now.isoweekday()  # 1=Mon, 7=Sun
        if weekday not in self.schedule_days:
            return False

        current_time = now.time()

        # 午休时段
        if self.lunch_start <= current_time < self.lunch_end:
            return False

        # 工作时段
        if self.schedule_start <= current_time <= self.schedule_end:
            return True

        return False

    def can_dial(self) -> bool:
        """检查是否可以继续拨号"""
        if not self._running:
            return False
        if not self.is_calling_time():
            return False
        if self._today_called >= self.daily_limit:
            return False
        if self.engine.active_count >= self.max_concurrent:
            return False
        return True

    async def start_campaign(self, campaign_id: int, script_prompt: str,
                              display_num: str, callback_url: str):
        """启动外呼任务"""
        if self._running:
            logger.warning("[scheduler] already running")
            return

        self._running = True
        self._current_campaign_id = campaign_id
        self._today_called = 0

        self.crm.update_campaign_status(campaign_id, "running")
        logger.info(f"[scheduler] campaign {campaign_id} started")

        self._task = asyncio.create_task(
            self._run_loop(campaign_id, script_prompt, display_num, callback_url)
        )

    async def pause_campaign(self):
        """暂停外呼任务"""
        self._running = False
        if self._current_campaign_id:
            self.crm.update_campaign_status(self._current_campaign_id, "paused")
        logger.info("[scheduler] paused")

    async def resume_campaign(self, script_prompt: str, display_num: str,
                               callback_url: str):
        """恢复外呼任务"""
        if not self._current_campaign_id:
            return
        self._running = True
        self.crm.update_campaign_status(self._current_campaign_id, "running")
        self._task = asyncio.create_task(
            self._run_loop(self._current_campaign_id, script_prompt, display_num, callback_url)
        )
        logger.info("[scheduler] resumed")

    async def stop_campaign(self):
        """停止外呼任务"""
        self._running = False
        if self._current_campaign_id:
            self.crm.update_campaign_status(self._current_campaign_id, "completed")
        if self._task and not self._task.done():
            self._task.cancel()
        self._current_campaign_id = None
        logger.info("[scheduler] stopped")

    async def _run_loop(self, campaign_id: int, script_prompt: str,
                         display_num: str, callback_url: str):
        """外呼主循环"""
        logger.info(f"[scheduler] run loop started for campaign {campaign_id}")

        while self._running:
            try:
                if not self.can_dial():
                    # 不在外呼时段或已达上限，等待
                    await asyncio.sleep(30)
                    continue

                # 取一批待拨号码
                batch_size = min(
                    self.max_concurrent - self.engine.active_count,
                    self.daily_limit - self._today_called,
                    10,
                )
                if batch_size <= 0:
                    await asyncio.sleep(10)
                    continue

                customers = self.crm.get_pending_calls(campaign_id, batch_size)
                if not customers:
                    # 所有号码已拨完
                    logger.info(f"[scheduler] campaign {campaign_id} all numbers called")
                    self._running = False
                    self.crm.update_campaign_status(campaign_id, "completed")
                    break

                # 逐个拨号，间隔 call_interval 秒
                for customer in customers:
                    if not self.can_dial():
                        break

                    result = await self.engine.dial(
                        customer_id=customer.id,
                        campaign_id=campaign_id,
                        phone=customer.phone,
                        display_num=display_num,
                        script_prompt=script_prompt,
                        callback_url=callback_url,
                    )

                    if result.get("success"):
                        self._today_called += 1

                    # 拨号间隔
                    await asyncio.sleep(self.call_interval)

            except asyncio.CancelledError:
                logger.info("[scheduler] loop cancelled")
                break
            except Exception as e:
                logger.error(f"[scheduler] loop error: {e}")
                await asyncio.sleep(5)

        logger.info(f"[scheduler] run loop ended. today_called={self._today_called}")

    def get_status(self) -> dict:
        return {
            "running": self._running,
            "campaign_id": self._current_campaign_id,
            "today_called": self._today_called,
            "daily_limit": self.daily_limit,
            "is_calling_time": self.is_calling_time(),
            "active_calls": self.engine.active_count,
            "max_concurrent": self.max_concurrent,
        }
