"""
🐝 Coze (扣子) 采集器
字节跳动 Bot + Plugin 市场，中国区直连。
同化难度: ⭐⭐⭐ (Bot+Plugin 体系差异大，L2 推荐)
"""

from typing import List, Dict
from .base import BaseCollector


class CozeCollector(BaseCollector):
    name = "coze"

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.api_base = (config or {}).get("api_base", "https://www.coze.com/api")

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        # Coze store API — discover popular bots
        categories = ["recommended", "productivity", "writing", "programming", "education", "entertainment"]
        for cat in categories:
            data = self.http_get(f"{self.api_base}/store/bots?category={cat}&limit=50")
            if not data:
                continue

            bots = data.get("bots", data.get("data", []))
            for bot in bots:
                sid = f"coze:{bot.get('bot_id', bot.get('id', ''))}"
                if sid in seen:
                    continue
                seen.add(sid)

                items.append({
                    "id": bot.get("bot_id", bot.get("id")),
                    "name": bot.get("name", bot.get("bot_name", "")),
                    "description": bot.get("description", bot.get("introduction", "")),
                    "system_prompt": bot.get("prompt", bot.get("system_prompt", bot.get("persona", ""))),
                    "tools": bot.get("plugins", bot.get("tools", [])),
                    "tags": [cat] + bot.get("tags", []),
                    "url": f"https://www.coze.com/store/bot/{bot.get('bot_id', '')}",
                    "author": bot.get("creator", {}).get("name", ""),
                    "rating": bot.get("rating", 0),
                    "installs": bot.get("use_count", 0),
                })

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[coze] collected {len(items)} bots (mode={mode})")
        return items
