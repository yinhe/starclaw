"""
🐝 SkillHub.club 采集器
7000+ Claude/Codex/Gemini skills，instructions → system_prompt。
同化难度: ⭐⭐
"""

from typing import List, Dict
from .base import BaseCollector


class SkillHubCollector(BaseCollector):
    name = "skillhub"

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.api_base = (config or {}).get("api_base", "https://www.skillhub.club/api")

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        # SkillHub API: list skills with pagination
        page = 1
        while True:
            data = self.http_get(f"{self.api_base}/skills?page={page}&limit=100")
            if not data:
                # Fallback: try /api/v1/skills
                data = self.http_get(f"{self.api_base}/v1/skills?page={page}&limit=100")
            if not data:
                break

            skills = data.get("skills", data.get("data", data.get("items", [])))
            if not skills:
                break

            for skill in skills:
                sid = f"skillhub:{skill.get('id', skill.get('slug', ''))}"
                if sid in seen:
                    continue
                seen.add(sid)

                items.append({
                    "id": skill.get("id", skill.get("slug")),
                    "name": skill.get("name", skill.get("title", "")),
                    "description": skill.get("description", skill.get("summary", "")),
                    "system_prompt": skill.get("instructions", skill.get("content", skill.get("prompt", ""))),
                    "tags": skill.get("tags", skill.get("categories", [])),
                    "url": f"https://www.skillhub.club/skills/{skill.get('slug', skill.get('id', ''))}",
                    "author": skill.get("author", ""),
                    "rating": skill.get("rating", 0),
                    "installs": skill.get("installs", skill.get("uses", 0)),
                    "platform": skill.get("platform", "claude"),  # claude, codex, gemini
                })

            if len(skills) < 100:
                break
            page += 1

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[skillhub] collected {len(items)} skills (mode={mode})")
        return items
