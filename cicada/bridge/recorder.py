"""
Cicada 🪰 Recorder — 录音下载 + 存储 + 转写
"""

import logging
import os
from datetime import datetime
from typing import Optional

import httpx

logger = logging.getLogger("cicada.recorder")


class Recorder:
    """录音管理器 — 下载、存储、管理通话录音"""

    def __init__(self, recordings_dir: str):
        self.recordings_dir = recordings_dir
        os.makedirs(recordings_dir, exist_ok=True)
        self.http = httpx.AsyncClient(timeout=60.0)

    async def download(self, call_sid: str, recording_url: str) -> Optional[str]:
        """
        下载录音文件到本地
        返回本地文件路径
        """
        if not recording_url:
            return None

        # 按日期分目录
        date_dir = datetime.now().strftime("%Y%m%d")
        save_dir = os.path.join(self.recordings_dir, date_dir)
        os.makedirs(save_dir, exist_ok=True)

        # 文件名: call_sid.wav
        filename = f"{call_sid}.wav"
        filepath = os.path.join(save_dir, filename)

        try:
            resp = await self.http.get(recording_url)
            if resp.status_code == 200:
                with open(filepath, "wb") as f:
                    f.write(resp.content)
                logger.info(f"[recorder] saved: {filepath} ({len(resp.content)} bytes)")
                return filepath
            else:
                logger.error(f"[recorder] download failed: HTTP {resp.status_code}")
                return None
        except Exception as e:
            logger.error(f"[recorder] download error: {e}")
            return None

    def get_recording_path(self, call_sid: str) -> Optional[str]:
        """查找录音文件路径"""
        for root, dirs, files in os.walk(self.recordings_dir):
            for fname in files:
                if call_sid in fname:
                    return os.path.join(root, fname)
        return None

    def list_recordings(self, date: Optional[str] = None) -> list[dict]:
        """列出录音文件"""
        result = []
        target_dir = self.recordings_dir
        if date:
            target_dir = os.path.join(self.recordings_dir, date)

        if not os.path.exists(target_dir):
            return result

        for root, dirs, files in os.walk(target_dir):
            for fname in files:
                if fname.endswith((".wav", ".mp3", ".pcm")):
                    filepath = os.path.join(root, fname)
                    stat = os.stat(filepath)
                    result.append({
                        "filename": fname,
                        "path": filepath,
                        "size": stat.st_size,
                        "created_at": datetime.fromtimestamp(stat.st_ctime).isoformat(),
                        "call_sid": fname.rsplit(".", 1)[0],
                    })

        return sorted(result, key=lambda x: x["created_at"], reverse=True)

    def get_disk_usage(self) -> dict:
        """统计录音存储用量"""
        total_size = 0
        total_files = 0
        for root, dirs, files in os.walk(self.recordings_dir):
            for fname in files:
                filepath = os.path.join(root, fname)
                total_size += os.path.getsize(filepath)
                total_files += 1

        return {
            "total_files": total_files,
            "total_size_bytes": total_size,
            "total_size_mb": round(total_size / 1024 / 1024, 2),
        }

    async def close(self):
        await self.http.aclose()
