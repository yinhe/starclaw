"""
Cicada 🪰 Call Engine — 通话状态机 + 外呼编排
"""

import asyncio
import json
import logging
import time
import uuid
from datetime import datetime
from enum import Enum
from typing import Optional, Callable

from crm_manager import CallStatus, CRMManager, CustomerStatus

logger = logging.getLogger("cicada.engine")


class CallState(str, Enum):
    IDLE = "idle"
    DIALING = "dialing"
    RINGING = "ringing"
    CONNECTED = "connected"
    TALKING = "talking"
    HANGUP = "hangup"
    RECORDING_SAVED = "recording_saved"
    FAILED = "failed"
    NO_ANSWER = "no_answer"
    REJECTED = "rejected"
    TRANSFERRED = "transferred"
    ERROR = "error"


# 合法的状态转换
VALID_TRANSITIONS = {
    CallState.IDLE: [CallState.DIALING],
    CallState.DIALING: [CallState.RINGING, CallState.FAILED, CallState.ERROR],
    CallState.RINGING: [CallState.CONNECTED, CallState.NO_ANSWER, CallState.REJECTED, CallState.ERROR],
    CallState.CONNECTED: [CallState.TALKING, CallState.HANGUP, CallState.ERROR],
    CallState.TALKING: [CallState.HANGUP, CallState.TRANSFERRED, CallState.ERROR],
    CallState.HANGUP: [CallState.RECORDING_SAVED],
    CallState.RECORDING_SAVED: [],
    CallState.FAILED: [],
    CallState.NO_ANSWER: [],
    CallState.REJECTED: [],
    CallState.TRANSFERRED: [CallState.HANGUP],
    CallState.ERROR: [],
}


class ActiveCall:
    """单通活跃通话"""

    def __init__(self, call_sid: str, customer_id: int, campaign_id: int,
                 callee: str, caller: str):
        self.call_sid = call_sid
        self.customer_id = customer_id
        self.campaign_id = campaign_id
        self.callee = callee
        self.caller = caller
        self.state = CallState.IDLE
        self.started_at = datetime.utcnow()
        self.connected_at: Optional[datetime] = None
        self.ended_at: Optional[datetime] = None
        self.duration = 0
        self.transcript_parts: list[dict] = []
        self.intent_level = ""
        self.intent_score = 0
        self.recording_url = ""

    def transition(self, new_state: CallState) -> bool:
        """状态转换，验证合法性"""
        if new_state in VALID_TRANSITIONS.get(self.state, []):
            old = self.state
            self.state = new_state

            if new_state == CallState.CONNECTED:
                self.connected_at = datetime.utcnow()
            elif new_state in (CallState.HANGUP, CallState.FAILED,
                               CallState.NO_ANSWER, CallState.REJECTED,
                               CallState.ERROR):
                self.ended_at = datetime.utcnow()
                if self.connected_at:
                    self.duration = int((self.ended_at - self.connected_at).total_seconds())

            logger.info(f"[call:{self.call_sid[:8]}] {old} → {new_state}")
            return True
        else:
            logger.warning(
                f"[call:{self.call_sid[:8]}] invalid transition {self.state} → {new_state}"
            )
            return False

    def add_transcript(self, role: str, text: str):
        """添加对话文字"""
        self.transcript_parts.append({
            "role": role,
            "text": text,
            "timestamp": time.time(),
        })

    @property
    def full_transcript(self) -> str:
        parts = []
        for p in self.transcript_parts:
            label = "客户" if p["role"] == "customer" else "机器人"
            parts.append(f"{label}: {p['text']}")
        return "\n".join(parts)

    def to_dict(self) -> dict:
        return {
            "call_sid": self.call_sid,
            "customer_id": self.customer_id,
            "campaign_id": self.campaign_id,
            "state": self.state,
            "duration": self.duration,
            "intent_level": self.intent_level,
            "intent_score": self.intent_score,
            "transcript_count": len(self.transcript_parts),
            "started_at": self.started_at.isoformat(),
        }


class CallEngine:
    """
    外呼引擎 — 管理所有活跃通话
    职责：
    1. 发起外呼 (dial)
    2. 管理通话状态机 (state transitions)
    3. 编排语音管道 (voice pipeline)
    4. 通话结束后触发分类+录音
    """

    def __init__(self, sip_client, voice_pipeline_factory, crm: CRMManager,
                 intent_classifier, compliance_checker, recorder):
        self.sip = sip_client
        self.voice_factory = voice_pipeline_factory
        self.crm = crm
        self.classifier = intent_classifier
        self.compliance = compliance_checker
        self.recorder = recorder

        self.active_calls: dict[str, ActiveCall] = {}
        self.max_concurrent = 10
        self._on_call_complete: Optional[Callable] = None

    def set_on_complete(self, callback: Callable):
        self._on_call_complete = callback

    @property
    def active_count(self) -> int:
        return len(self.active_calls)

    async def dial(self, customer_id: int, campaign_id: int, phone: str,
                   display_num: str, script_prompt: str,
                   callback_url: str) -> dict:
        """
        发起一通外呼
        返回: {"success": bool, "call_sid": str, "error": str}
        """
        # 检查并发限制
        if self.active_count >= self.max_concurrent:
            return {"success": False, "error": "max concurrent reached"}

        # 合规检查
        if self.compliance and self.compliance.should_block(phone):
            return {"success": False, "error": "blocked by compliance"}

        # 发起 SIP 外呼
        result = await self.sip.make_ivr_call(
            callee=phone,
            display_num=display_num,
            callback_url=callback_url,
        )

        if not result.get("success"):
            # 记录失败
            self.crm.create_call_record(
                customer_id=customer_id,
                campaign_id=campaign_id,
                call_sid=f"fail-{uuid.uuid4().hex[:12]}",
                callee_number=phone,
                caller_number=display_num,
                status=CallStatus.FAILED,
            )
            self.crm.update_campaign_stats(campaign_id, called=1)
            return result

        call_sid = result["call_sid"]

        # 创建活跃通话
        call = ActiveCall(
            call_sid=call_sid,
            customer_id=customer_id,
            campaign_id=campaign_id,
            callee=phone,
            caller=display_num,
        )
        call.transition(CallState.DIALING)
        self.active_calls[call_sid] = call

        # 创建通话记录
        self.crm.create_call_record(
            customer_id=customer_id,
            campaign_id=campaign_id,
            call_sid=call_sid,
            callee_number=phone,
            caller_number=display_num,
            status=CallStatus.DIALING,
        )
        self.crm.increment_call_count(customer_id)
        self.crm.update_campaign_stats(campaign_id, called=1)

        logger.info(f"[engine] dial started: {call_sid[:8]} → {phone}")
        return {"success": True, "call_sid": call_sid}

    async def on_call_status(self, call_sid: str, status: str, **kwargs):
        """
        处理云通信平台的状态回调
        由 main.py 的 /callback/call-status 路由调用
        """
        call = self.active_calls.get(call_sid)
        if not call:
            logger.warning(f"[engine] unknown call_sid: {call_sid}")
            return

        status_map = {
            "Ringing": CallState.RINGING,
            "Answer": CallState.CONNECTED,
            "Hangup": CallState.HANGUP,
            "NoAnswer": CallState.NO_ANSWER,
            "Reject": CallState.REJECTED,
            "Busy": CallState.FAILED,
            "Failed": CallState.FAILED,
            "Error": CallState.ERROR,
        }

        new_state = status_map.get(status)
        if not new_state:
            logger.warning(f"[engine] unknown status: {status}")
            return

        call.transition(new_state)
        self.crm.update_call_record(call_sid, status=new_state.value)

        # 接通 → 更新统计
        if new_state == CallState.CONNECTED:
            self.crm.update_campaign_stats(call.campaign_id, connected=1)
            call.transition(CallState.TALKING)
            self.crm.update_call_record(call_sid, status=CallState.TALKING.value)

        # 通话结束 → 触发后处理
        elif new_state in (CallState.HANGUP, CallState.NO_ANSWER,
                           CallState.REJECTED, CallState.FAILED, CallState.ERROR):
            await self._on_call_ended(call)

    async def on_recording_ready(self, call_sid: str, recording_url: str):
        """录音就绪回调"""
        call = self.active_calls.get(call_sid)
        if call:
            call.recording_url = recording_url
            call.transition(CallState.RECORDING_SAVED)

        # 下载录音
        if self.recorder and recording_url:
            local_path = await self.recorder.download(call_sid, recording_url)
            self.crm.update_call_record(
                call_sid,
                recording_url=recording_url,
                recording_path=local_path or "",
            )

    async def hangup(self, call_sid: str) -> dict:
        """主动挂断"""
        call = self.active_calls.get(call_sid)
        if not call:
            return {"success": False, "error": "call not found"}

        result = await self.sip.hangup(call_sid)
        call.transition(CallState.HANGUP)
        self.crm.update_call_record(call_sid, status=CallState.HANGUP.value)
        await self._on_call_ended(call)
        return {"success": True}

    async def _on_call_ended(self, call: ActiveCall):
        """通话结束后处理"""
        # 更新通话记录
        self.crm.update_call_record(
            call.call_sid,
            duration=call.duration,
            transcript=call.full_transcript,
            ended_at=call.ended_at or datetime.utcnow(),
        )

        # 如果有通话内容，进行意向分类
        if call.transcript_parts and call.duration > 5 and self.classifier:
            try:
                result = await self.classifier.classify(
                    transcript=call.full_transcript,
                    duration=call.duration,
                )
                call.intent_level = result.get("level", "D")
                call.intent_score = result.get("total_score", 0)

                # 更新通话记录
                self.crm.update_call_record(
                    call.call_sid,
                    intent_level=call.intent_level,
                    intent_score=call.intent_score,
                    summary=result.get("summary", ""),
                    ai_analysis=json.dumps(result, ensure_ascii=False),
                )

                # 更新客户意向
                self.crm.update_customer_intent(
                    call.customer_id,
                    level=call.intent_level,
                    score=call.intent_score,
                    key_interests=result.get("key_interests", []),
                    summary=result.get("summary", ""),
                )

                # 更新任务统计
                if call.intent_level == "A":
                    self.crm.update_campaign_stats(call.campaign_id, intent_a=1)
                elif call.intent_level == "B":
                    self.crm.update_campaign_stats(call.campaign_id, intent_b=1)

                logger.info(
                    f"[engine] classified: {call.call_sid[:8]} → "
                    f"{call.intent_level} ({call.intent_score})"
                )
            except Exception as e:
                logger.error(f"[engine] classify error: {e}")

        # 无接通或极短通话 → F 类
        elif call.state in (CallState.NO_ANSWER, CallState.FAILED):
            self.crm.update_customer_intent(call.customer_id, level="F", score=0)

        # 清理活跃通话
        self.active_calls.pop(call.call_sid, None)

        # 触发回调
        if self._on_call_complete:
            try:
                await self._on_call_complete(call)
            except Exception as e:
                logger.error(f"[engine] on_complete callback error: {e}")

        logger.info(
            f"[engine] call ended: {call.call_sid[:8]} "
            f"duration={call.duration}s intent={call.intent_level}"
        )

    def get_active_calls(self) -> list[dict]:
        return [c.to_dict() for c in self.active_calls.values()]

    def get_stats(self) -> dict:
        return {
            "active_calls": self.active_count,
            "max_concurrent": self.max_concurrent,
        }
