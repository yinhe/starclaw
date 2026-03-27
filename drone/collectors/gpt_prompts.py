"""
🐝 GPT Prompts 采集器 — 从 GitHub 仓库采集完整的 GPT system prompts
数据源: linexjlin/GPTs (180+ 完整 prompt 文件, 每个 1-35KB)
同化难度: ⭐⭐ (完整 prompt，仅需清洗+分类)
"""

import re
import time
from typing import List, Dict
from urllib.parse import quote
from .base import BaseCollector


class GPTPromptsCollector(BaseCollector):
    """采集 GitHub 上的 GPT prompt 仓库 — 每个文件就是一个完整的 system prompt。"""

    name = "gpt_prompts"

    # 仓库列表：每个仓库的 prompts/ 目录下有完整 prompt 文件
    REPOS = [
        {"owner": "linexjlin", "repo": "GPTs", "path": "prompts", "branch": "main"},
    ]

    def __init__(self, config: dict = None):
        super().__init__(config)

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        for repo_info in self.REPOS:
            owner = repo_info["owner"]
            repo = repo_info["repo"]
            path = repo_info["path"]
            branch = repo_info.get("branch", "main")

            print(f"[{self.name}] scanning {owner}/{repo}/{path}...")

            # Get file list via GitHub API (Trees API, no auth needed for public repos)
            tree_url = f"https://api.github.com/repos/{owner}/{repo}/git/trees/{branch}?recursive=1"
            data = self.http_get(tree_url, headers={"Accept": "application/vnd.github.v3+json"})
            if not data or "tree" not in data:
                print(f"[{self.name}] failed to get tree for {owner}/{repo}")
                continue

            # Filter: only files in prompts/ directory, .md or .txt
            prompt_files = []
            for item in data["tree"]:
                if item["type"] != "blob":
                    continue
                file_path = item["path"]
                if not file_path.startswith(f"{path}/"):
                    continue
                if not (file_path.endswith(".md") or file_path.endswith(".txt")):
                    continue
                prompt_files.append(item)

            print(f"[{self.name}] found {len(prompt_files)} prompt files in {owner}/{repo}")

            # Fetch each prompt file
            for i, pf in enumerate(prompt_files):
                file_path = pf["path"]
                # Extract name from filename: "prompts/10x Engineer.md" → "10x Engineer"
                filename = file_path.split("/")[-1]
                name = filename.rsplit(".", 1)[0]  # remove extension

                sid = f"gpt_prompts:{owner}/{repo}:{name}"
                if sid in seen:
                    continue
                seen.add(sid)

                # Fetch raw content
                raw_url = f"https://raw.githubusercontent.com/{owner}/{repo}/{branch}/{quote(file_path)}"
                import requests
                try:
                    resp = requests.get(raw_url, timeout=15)
                    if resp.status_code != 200:
                        continue
                    content = resp.text.strip()
                except Exception as e:
                    print(f"[{self.name}] failed to fetch {filename}: {e}")
                    continue

                if len(content) < 50:
                    continue  # skip empty/tiny files

                items.append({
                    "id": sid,
                    "name": name,
                    "description": self._extract_description(content, name),
                    "system_prompt": content,
                    "tags": self._guess_tags(name, content),
                    "url": f"https://github.com/{owner}/{repo}/blob/{branch}/{quote(file_path)}",
                    "_repo": f"{owner}/{repo}",
                    "_file": file_path,
                })

                # Rate limit: GitHub allows 60 req/hr unauthenticated
                if (i + 1) % 10 == 0:
                    print(f"[{self.name}] fetched {i+1}/{len(prompt_files)} prompts...")
                    time.sleep(1)  # be polite

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[{self.name}] collected {len(items)} GPT prompts with full system_prompt")
        return items

    def _extract_description(self, content: str, name: str) -> str:
        """Extract first meaningful line as description."""
        lines = content.strip().split("\n")
        for line in lines[:5]:
            line = line.strip().lstrip("#").strip()
            if line and len(line) > 10 and not line.startswith("```") and line != name:
                return line[:200]
        return name

    def _guess_tags(self, name: str, content: str) -> list:
        """Guess tags from name and content."""
        tags = []
        text = f"{name} {content[:500]}".lower()
        tag_keywords = {
            "coding": ["code", "program", "developer", "debug", "python", "javascript"],
            "writing": ["write", "blog", "article", "essay", "copywriting", "content"],
            "creative": ["design", "art", "image", "creative", "draw", "logo"],
            "data": ["data", "analysis", "excel", "chart", "statistics"],
            "education": ["learn", "tutor", "teach", "study", "math", "science"],
            "business": ["business", "marketing", "sales", "seo", "startup"],
            "health": ["health", "doctor", "medical", "fitness", "therapy"],
            "game": ["game", "rpg", "adventure", "play", "quest"],
        }
        for tag, keywords in tag_keywords.items():
            if any(kw in text for kw in keywords):
                tags.append(tag)
        return tags[:5] if tags else ["general"]
