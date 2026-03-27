"""
🐝 BaseCollector — 采集器抽象基类
所有平台采集器继承此类，实现 collect() 方法。
"""

import json
import os
import time
from abc import ABC, abstractmethod
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Optional


class BaseCollector(ABC):
    """采集器基类。每个数据源实现一个子类。"""

    name: str = "base"
    data_dir: str = "/data/harvest"

    def __init__(self, config: dict = None):
        self.config = config or {}
        self.data_dir = self.config.get("data_dir", os.getenv("DRONE_DATA_DIR", "/data/harvest"))
        Path(self.data_dir).mkdir(parents=True, exist_ok=True)

    @abstractmethod
    def collect(self, mode: str = "incremental") -> List[Dict]:
        """
        采集原始数据。
        
        Args:
            mode: "incremental"（增量，只采新的）或 "full"（全量）
        
        Returns:
            原始数据 dict 列表（未经同化的外部格式）
        """
        ...

    def save_raw(self, items: List[Dict]) -> str:
        """将原始数据保存为 JSONL 文件。"""
        ts = datetime.utcnow().strftime("%Y%m%d_%H%M%S")
        filename = f"{self.name}_{ts}.jsonl"
        filepath = os.path.join(self.data_dir, filename)

        with open(filepath, "w", encoding="utf-8") as f:
            for item in items:
                item["_collected_at"] = datetime.utcnow().isoformat()
                item["_source"] = self.name
                f.write(json.dumps(item, ensure_ascii=False) + "\n")

        print(f"[{self.name}] saved {len(items)} items to {filepath}")
        return filepath

    def load_seen_ids(self) -> set:
        """加载已采集的 source_id 集合（用于增量模式去重）。"""
        seen_file = os.path.join(self.data_dir, f".{self.name}_seen_ids")
        if os.path.exists(seen_file):
            with open(seen_file, "r") as f:
                return set(line.strip() for line in f if line.strip())
        return set()

    def save_seen_ids(self, ids: set):
        """保存已采集的 source_id 集合。"""
        seen_file = os.path.join(self.data_dir, f".{self.name}_seen_ids")
        with open(seen_file, "w") as f:
            f.write("\n".join(sorted(ids)))

    def http_get(self, url: str, headers: dict = None, timeout: int = 15) -> Optional[dict]:
        """安全的 HTTP GET，带重试。"""
        import requests
        for attempt in range(3):
            try:
                resp = requests.get(url, headers=headers or {}, timeout=timeout)
                resp.raise_for_status()
                return resp.json()
            except Exception as e:
                if attempt == 2:
                    print(f"[{self.name}] GET {url} failed after 3 attempts: {e}")
                    return None
                time.sleep(2 ** attempt)
        return None
