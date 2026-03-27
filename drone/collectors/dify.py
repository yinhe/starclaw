"""
🐝 Dify Marketplace 采集器
开源 Agent template 平台，格式清晰。
同化难度: ⭐⭐
"""

from typing import List, Dict
from .base import BaseCollector


class DifyCollector(BaseCollector):
    name = "dify"

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.api_base = (config or {}).get("api_base", "https://cloud.dify.ai/api")

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        page = 1
        while True:
            data = self.http_get(f"{self.api_base}/explore/apps?page={page}&limit=50")
            if not data:
                break

            apps = data.get("data", data.get("apps", []))
            if not apps:
                break

            for app in apps:
                sid = f"dify:{app.get('id', '')}"
                if sid in seen:
                    continue
                seen.add(sid)

                model_config = app.get("model_config", {})
                items.append({
                    "id": app.get("id"),
                    "name": app.get("name", ""),
                    "description": app.get("description", ""),
                    "system_prompt": model_config.get("pre_prompt", model_config.get("prompt_template", "")),
                    "tools": [t.get("name", "") for t in model_config.get("tools", [])],
                    "tags": app.get("tags", []),
                    "url": f"https://cloud.dify.ai/explore/{app.get('id', '')}",
                    "category": app.get("category", ""),
                    "installs": app.get("installed_count", 0),
                })

            if len(apps) < 50:
                break
            page += 1

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[dify] collected {len(items)} templates (mode={mode})")
        return items
