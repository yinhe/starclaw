"""
Cicada 🪰 SIP Client — 容联云 REST API 对接
"""

import hashlib
import base64
import json
import time
import uuid
import logging
from datetime import datetime
from typing import Optional

import httpx

logger = logging.getLogger("cicada.sip")


class CloopenClient:
    """容联云 REST API 客户端"""

    def __init__(self, account_sid: str, auth_token: str, app_id: str,
                 rest_url: str = "https://app.cloopen.com:8883"):
        self.account_sid = account_sid
        self.auth_token = auth_token
        self.app_id = app_id
        self.rest_url = rest_url.rstrip("/")
        self.http = httpx.AsyncClient(timeout=30.0)

    def _make_auth(self) -> tuple[str, str]:
        """生成鉴权签名和 Authorization header"""
        ts = datetime.now().strftime("%Y%m%d%H%M%S")
        sig_str = f"{self.account_sid}{self.auth_token}{ts}"
        sig = hashlib.md5(sig_str.encode()).hexdigest().upper()
        auth_str = base64.b64encode(f"{self.account_sid}:{ts}".encode()).decode()
        return sig, auth_str, ts

    def _url(self, path: str, ts: str, sig: str) -> str:
        return (
            f"{self.rest_url}/2013-12-26/Accounts/{self.account_sid}"
            f"{path}?sig={sig}"
        )

    def _headers(self, auth_str: str) -> dict:
        return {
            "Accept": "application/json",
            "Content-Type": "application/json;charset=utf-8",
            "Authorization": auth_str,
        }

    async def make_call(
        self,
        caller: str,
        callee: str,
        callback_url: str,
        max_duration: int = 300,
        record: bool = True,
        display_num: Optional[str] = None,
    ) -> dict:
        """
        发起外呼（双向回呼模式）
        caller: 被叫号码（客户）
        callee: 接听坐席号码（留空则用IVR模式）
        """
        sig, auth_str, ts = self._make_auth()
        url = self._url("/Calls/Callback", ts, sig)

        body = {
            "appId": self.app_id,
            "from": caller,
            "to": callee,
            "customerSerNum": display_num or caller,
            "statusUrl": callback_url,
            "maxCallTime": max_duration,
            "record": "true" if record else "false",
        }

        try:
            resp = await self.http.post(url, json=body, headers=self._headers(auth_str))
            data = resp.json()
            logger.info(f"[sip] make_call caller={caller} callee={callee} resp={data}")
            return data
        except Exception as e:
            logger.error(f"[sip] make_call error: {e}")
            return {"statusCode": "error", "statusMsg": str(e)}

    async def make_ivr_call(
        self,
        callee: str,
        display_num: str,
        callback_url: str,
        media_url: Optional[str] = None,
        max_duration: int = 300,
        record: bool = True,
    ) -> dict:
        """
        发起IVR外呼（单方向，AI直接对话）
        callee: 被叫号码（客户）
        display_num: 来电显示号码
        """
        sig, auth_str, ts = self._make_auth()
        url = self._url("/ivr/call", ts, sig)

        body = {
            "appId": self.app_id,
            "to": callee,
            "fromSerNum": display_num,
            "statusUrl": callback_url,
            "maxCallTime": str(max_duration),
            "record": "true" if record else "false",
        }
        if media_url:
            body["mediaUrl"] = media_url

        try:
            resp = await self.http.post(url, json=body, headers=self._headers(auth_str))
            data = resp.json()
            logger.info(f"[sip] ivr_call callee={callee} display={display_num} resp={data}")

            if data.get("statusCode") == "000000":
                call_sid = data.get("Callback", {}).get("callSid") or data.get("callSid", "")
                return {
                    "success": True,
                    "call_sid": call_sid,
                    "data": data,
                }
            else:
                return {
                    "success": False,
                    "error": data.get("statusMsg", "unknown error"),
                    "code": data.get("statusCode"),
                    "data": data,
                }
        except Exception as e:
            logger.error(f"[sip] ivr_call error: {e}")
            return {"success": False, "error": str(e)}

    async def hangup(self, call_sid: str) -> dict:
        """挂断通话"""
        sig, auth_str, ts = self._make_auth()
        url = self._url(f"/Calls/{call_sid}/Hangup", ts, sig)

        try:
            resp = await self.http.post(url, json={}, headers=self._headers(auth_str))
            data = resp.json()
            logger.info(f"[sip] hangup call_sid={call_sid} resp={data}")
            return data
        except Exception as e:
            logger.error(f"[sip] hangup error: {e}")
            return {"statusCode": "error", "statusMsg": str(e)}

    async def get_recording(self, call_sid: str, date: Optional[str] = None) -> dict:
        """获取通话录音"""
        sig, auth_str, ts = self._make_auth()
        if not date:
            date = datetime.now().strftime("%Y%m%d")
        url = self._url(f"/Calls/Recording/{call_sid}?date={date}", ts, sig)

        try:
            resp = await self.http.get(url, headers=self._headers(auth_str))
            data = resp.json()
            logger.info(f"[sip] recording call_sid={call_sid} resp={data}")
            return data
        except Exception as e:
            logger.error(f"[sip] recording error: {e}")
            return {"statusCode": "error", "statusMsg": str(e)}

    async def close(self):
        await self.http.aclose()


class MockSIPClient:
    """模拟 SIP 客户端，用于本地测试"""

    async def make_ivr_call(self, callee: str, display_num: str,
                             callback_url: str, **kwargs) -> dict:
        call_sid = f"mock-{uuid.uuid4().hex[:12]}"
        logger.info(f"[mock-sip] ivr_call callee={callee} call_sid={call_sid}")
        return {"success": True, "call_sid": call_sid, "data": {"mock": True}}

    async def hangup(self, call_sid: str) -> dict:
        logger.info(f"[mock-sip] hangup call_sid={call_sid}")
        return {"statusCode": "000000"}

    async def get_recording(self, call_sid: str, **kwargs) -> dict:
        return {"statusCode": "000000", "Recording": {"recordUrl": ""}}

    async def close(self):
        pass
