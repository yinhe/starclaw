"""
Cicada 🪰 Voice Pipeline — DashScope ASR (Paraformer) + TTS (CosyVoice) 流式串联
"""

import asyncio
import json
import logging
import struct
import time
from typing import AsyncGenerator, Optional, Callable

import httpx
import websockets

logger = logging.getLogger("cicada.voice")


class ASRStream:
    """
    DashScope Paraformer 实时流式语音识别
    音频流 → WebSocket → 实时文字输出
    """

    WS_URL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"

    def __init__(self, api_key: str, model: str = "paraformer-realtime-v2",
                 sample_rate: int = 16000):
        self.api_key = api_key
        self.model = model
        self.sample_rate = sample_rate
        self.ws = None
        self._full_text = ""
        self._on_sentence: Optional[Callable] = None

    async def connect(self):
        """建立 WebSocket 连接"""
        headers = {
            "Authorization": f"bearer {self.api_key}",
            "X-DashScope-DataInspection": "enable",
        }
        try:
            self.ws = await websockets.connect(
                self.WS_URL,
                additional_headers=headers,
                ping_interval=20,
            )
            # 发送 run-task 指令
            start_msg = {
                "header": {
                    "action": "run-task",
                    "task_id": f"cicada-asr-{int(time.time()*1000)}",
                    "streaming": "duplex",
                },
                "payload": {
                    "task_group": "audio",
                    "task": "asr",
                    "function": "recognition",
                    "model": self.model,
                    "parameters": {
                        "format": "pcm",
                        "sample_rate": self.sample_rate,
                        "enable_punctuation_prediction": True,
                        "enable_inverse_text_normalization": True,
                        "enable_disfluency_detection": True,
                    },
                    "input": {},
                },
            }
            await self.ws.send(json.dumps(start_msg))
            resp = await self.ws.recv()
            data = json.loads(resp)
            if data.get("header", {}).get("code", -1) != 0:
                raise Exception(f"ASR connect failed: {data}")
            logger.info("[asr] connected")
        except Exception as e:
            logger.error(f"[asr] connect error: {e}")
            raise

    async def send_audio(self, audio_data: bytes):
        """发送音频数据"""
        if self.ws:
            await self.ws.send(audio_data)

    async def finish(self):
        """发送结束信号"""
        if self.ws:
            finish_msg = {
                "header": {
                    "action": "finish-task",
                    "task_id": f"cicada-asr-finish-{int(time.time()*1000)}",
                    "streaming": "duplex",
                },
                "payload": {"input": {}},
            }
            await self.ws.send(json.dumps(finish_msg))

    async def receive_results(self) -> AsyncGenerator[dict, None]:
        """
        接收识别结果
        yields: {"text": "...", "is_final": bool, "sentence_end": bool}
        """
        if not self.ws:
            return

        try:
            async for message in self.ws:
                if isinstance(message, bytes):
                    continue
                data = json.loads(message)
                header = data.get("header", {})
                payload = data.get("payload", {})

                if header.get("code", -1) != 0:
                    logger.error(f"[asr] error: {data}")
                    continue

                output = payload.get("output", {})
                sentence = output.get("sentence", {})

                if sentence:
                    text = sentence.get("text", "")
                    is_end = sentence.get("end_time", 0) > 0

                    if text:
                        yield {
                            "text": text,
                            "is_final": is_end,
                            "sentence_end": is_end,
                        }

                    if is_end:
                        self._full_text += text + " "

                if header.get("event") == "task-finished":
                    break
        except websockets.ConnectionClosed:
            logger.info("[asr] connection closed")
        except Exception as e:
            logger.error(f"[asr] receive error: {e}")

    @property
    def full_text(self) -> str:
        return self._full_text.strip()

    async def close(self):
        if self.ws:
            await self.ws.close()
            self.ws = None


class TTSStream:
    """
    DashScope CosyVoice 流式语音合成
    文字 → WebSocket → 实时音频流
    """

    WS_URL = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"

    def __init__(self, api_key: str, model: str = "cosyvoice-v1",
                 voice: str = "longxiaochun", sample_rate: int = 16000):
        self.api_key = api_key
        self.model = model
        self.voice = voice
        self.sample_rate = sample_rate

    async def synthesize(self, text: str) -> AsyncGenerator[bytes, None]:
        """
        将文字合成为音频流
        yields: PCM 音频 bytes
        """
        headers = {
            "Authorization": f"bearer {self.api_key}",
            "X-DashScope-DataInspection": "enable",
        }

        try:
            async with websockets.connect(
                self.WS_URL,
                additional_headers=headers,
                ping_interval=20,
            ) as ws:
                # 发送 run-task
                start_msg = {
                    "header": {
                        "action": "run-task",
                        "task_id": f"cicada-tts-{int(time.time()*1000)}",
                        "streaming": "out",
                    },
                    "payload": {
                        "task_group": "audio",
                        "task": "tts",
                        "function": "SpeechSynthesizer",
                        "model": self.model,
                        "parameters": {
                            "voice": self.voice,
                            "format": "pcm",
                            "sample_rate": self.sample_rate,
                            "rate": 1.0,
                            "volume": 50,
                        },
                        "input": {
                            "text": text,
                        },
                    },
                }
                await ws.send(json.dumps(start_msg))

                async for message in ws:
                    if isinstance(message, bytes):
                        yield message
                    else:
                        data = json.loads(message)
                        header = data.get("header", {})
                        if header.get("code", -1) != 0:
                            error_msg = header.get("message", "unknown")
                            if header.get("event") != "task-finished":
                                logger.error(f"[tts] error: {error_msg}")
                        if header.get("event") == "task-finished":
                            break

        except Exception as e:
            logger.error(f"[tts] synthesize error: {e}")


class VoicePipeline:
    """
    ASR → LLM → TTS 流式串联管道
    核心：最小化端到端延迟
    """

    def __init__(self, api_key: str, llm_client, asr_config: dict = None,
                 tts_config: dict = None):
        asr_config = asr_config or {}
        tts_config = tts_config or {}

        self.asr = ASRStream(
            api_key=api_key,
            model=asr_config.get("model", "paraformer-realtime-v2"),
            sample_rate=asr_config.get("sample_rate", 16000),
        )
        self.tts = TTSStream(
            api_key=api_key,
            model=tts_config.get("model", "cosyvoice-v1"),
            voice=tts_config.get("voice", "longxiaochun"),
            sample_rate=tts_config.get("sample_rate", 16000),
        )
        self.llm = llm_client
        self._conversation_history = []
        self._transcript_parts = []

    def set_system_prompt(self, prompt: str):
        """设置 LLM 系统 prompt（话术模板注入）"""
        self._conversation_history = [{"role": "system", "content": prompt}]

    async def process_sentence(self, customer_text: str) -> AsyncGenerator[bytes, None]:
        """
        处理一句客户语音：
        1. 将文字送入 LLM
        2. LLM 流式生成回复
        3. 回复分句送入 TTS
        4. 逐帧输出音频
        """
        self._conversation_history.append({"role": "user", "content": customer_text})
        self._transcript_parts.append(f"客户: {customer_text}")

        # LLM 流式生成
        full_response = ""
        current_sentence = ""

        async for chunk in self.llm.stream_chat(self._conversation_history):
            full_response += chunk
            current_sentence += chunk

            # 检测句子边界（句号/问号/感叹号/逗号后够长）
            if self._is_sentence_boundary(current_sentence):
                sentence = current_sentence.strip()
                if sentence:
                    # TTS 合成并输出
                    async for audio_frame in self.tts.synthesize(sentence):
                        yield audio_frame
                current_sentence = ""

        # 处理剩余文本
        if current_sentence.strip():
            async for audio_frame in self.tts.synthesize(current_sentence.strip()):
                yield audio_frame

        self._conversation_history.append({"role": "assistant", "content": full_response})
        self._transcript_parts.append(f"机器人: {full_response}")

    @staticmethod
    def _is_sentence_boundary(text: str) -> bool:
        """检测是否到达句子边界"""
        if not text:
            return False
        # 中文标点分句
        for punct in ["。", "！", "？", "；"]:
            if text.endswith(punct):
                return True
        # 逗号后超过15字也分
        if "，" in text and len(text) > 15:
            return True
        # 超过30字强制分
        if len(text) > 30:
            return True
        return False

    @property
    def full_transcript(self) -> str:
        return "\n".join(self._transcript_parts)

    @property
    def conversation_history(self) -> list[dict]:
        return self._conversation_history


class LLMClient:
    """LLM 客户端 — 通过 OpenAI 兼容接口调用"""

    def __init__(self, base_url: str, api_key: str, model: str = "qwen-turbo",
                 temperature: float = 0.3, max_tokens: int = 200):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.http = httpx.AsyncClient(timeout=30.0)

    async def stream_chat(self, messages: list[dict]) -> AsyncGenerator[str, None]:
        """流式对话，逐 token 输出"""
        url = f"{self.base_url}/chat/completions"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        body = {
            "model": self.model,
            "messages": messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "stream": True,
        }

        try:
            async with self.http.stream("POST", url, json=body, headers=headers) as resp:
                async for line in resp.aiter_lines():
                    if not line.startswith("data: "):
                        continue
                    data_str = line[6:]
                    if data_str == "[DONE]":
                        break
                    try:
                        data = json.loads(data_str)
                        delta = data["choices"][0].get("delta", {})
                        content = delta.get("content", "")
                        if content:
                            yield content
                    except (json.JSONDecodeError, KeyError, IndexError):
                        continue
        except Exception as e:
            logger.error(f"[llm] stream_chat error: {e}")

    async def chat(self, messages: list[dict]) -> str:
        """非流式对话"""
        url = f"{self.base_url}/chat/completions"
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        body = {
            "model": self.model,
            "messages": messages,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
        }

        try:
            resp = await self.http.post(url, json=body, headers=headers)
            data = resp.json()
            return data["choices"][0]["message"]["content"]
        except Exception as e:
            logger.error(f"[llm] chat error: {e}")
            return ""

    async def close(self):
        await self.http.aclose()
