"""
🐝 ClawHub.ai 采集器
开源 MIT 协议，AgentSkills bundle 格式，与 StarClaw 高度匹配。
同化难度: ⭐ (几乎直通)
"""

from typing import List, Dict
from .base import BaseCollector


class ClawHubCollector(BaseCollector):
    name = "clawhub"

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.api_base = (config or {}).get("api_base", "https://clawhub.ai/api")

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        # ClawHub API: GET /api/skills?nonSuspicious=true
        page = 0
        while True:
            data = self.http_get(f"{self.api_base}/skills?nonSuspicious=true&page={page}&limit=50")
            if not data or not data.get("skills"):
                break

            for skill in data["skills"]:
                sid = f"clawhub:{skill.get('_id', skill.get('id', ''))}"
                if sid in seen:
                    continue
                seen.add(sid)

                items.append({
                    "id": skill.get("_id", skill.get("id")),
                    "name": skill.get("name", ""),
                    "description": skill.get("description", ""),
                    "system_prompt": skill.get("prompt", skill.get("instructions", "")),
                    "tools": skill.get("tools", []),
                    "tags": skill.get("tags", []),
                    "url": f"https://clawhub.ai/skills/{skill.get('slug', skill.get('_id', ''))}",
                    "author": skill.get("author", {}).get("name", ""),
                    "downloads": skill.get("downloads", 0),
                    "rating": skill.get("rating", 0),
                })

            if len(data["skills"]) < 50:
                break
            page += 1

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[clawhub] collected {len(items)} skills (mode={mode})")
        return items
