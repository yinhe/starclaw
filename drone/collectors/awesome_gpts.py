"""
🐝 GitHub Awesome-GPTs 采集器
从 GitHub 仓库的 README 中解析 GPT Agent 列表。
同化难度: ⭐⭐⭐ (Markdown 解析，质量参差)
"""

import re
from typing import List, Dict
from .base import BaseCollector


class AwesomeGPTsCollector(BaseCollector):
    name = "awesome_gpts"

    DEFAULT_REPOS = [
        "ai-boost/awesome-gpts",
        "taranjeet/awesome-gpts",
        "EmbraceAGI/Awesome-AI-GPTs",
    ]

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.repos = (config or {}).get("repos", self.DEFAULT_REPOS)

    def collect(self, mode: str = "incremental") -> List[Dict]:
        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []

        for repo in self.repos:
            readme = self._fetch_readme(repo)
            if not readme:
                continue
            parsed = self._parse_readme(readme, repo)
            for item in parsed:
                sid = f"github:{repo}:{item.get('name', '')[:50]}"
                if sid in seen:
                    continue
                seen.add(sid)
                items.append(item)

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[awesome_gpts] collected {len(items)} GPTs from {len(self.repos)} repos (mode={mode})")
        return items

    def _fetch_readme(self, repo: str) -> str:
        """Fetch raw README from GitHub."""
        for branch in ["main", "master"]:
            url = f"https://raw.githubusercontent.com/{repo}/{branch}/README.md"
            import requests
            try:
                resp = requests.get(url, timeout=15)
                if resp.status_code == 200:
                    return resp.text
            except Exception:
                continue
        print(f"[awesome_gpts] failed to fetch README from {repo}")
        return ""

    def _parse_readme(self, content: str, repo: str) -> List[Dict]:
        """Parse GPT entries from Markdown README."""
        items = []
        current_category = ""

        for line in content.split("\n"):
            line = line.strip()

            # Detect category headers
            if line.startswith("##"):
                current_category = line.lstrip("#").strip()
                continue

            # Parse table rows: | Name | Description | Link |
            if "|" in line and not line.startswith("|--") and not line.startswith("| -"):
                cells = [c.strip() for c in line.split("|")]
                cells = [c for c in cells if c]
                if len(cells) >= 2:
                    name = self._clean_md(cells[0])
                    desc = self._clean_md(cells[1]) if len(cells) > 1 else ""
                    link = self._extract_link(cells[-1]) if len(cells) > 2 else ""
                    if name and len(name) > 2 and name.lower() not in ("name", "名称", "gpt"):
                        items.append({
                            "name": name,
                            "description": desc,
                            "system_prompt": desc,  # Will need L2 evolve for real prompt
                            "url": link,
                            "tags": [current_category] if current_category else [],
                            "_repo": repo,
                        })
                continue

            # Parse list items: - **Name**: Description [link](url)
            list_match = re.match(r'^[-*]\s+\*\*(.+?)\*\*[:\s]*(.+)?$', line)
            if list_match:
                name = list_match.group(1).strip()
                rest = (list_match.group(2) or "").strip()
                link = ""
                link_match = re.search(r'\[.*?\]\((https?://[^\)]+)\)', rest)
                if link_match:
                    link = link_match.group(1)
                    rest = re.sub(r'\[.*?\]\(https?://[^\)]+\)', '', rest).strip()
                if name and len(name) > 2:
                    items.append({
                        "name": name,
                        "description": rest,
                        "system_prompt": rest,
                        "url": link,
                        "tags": [current_category] if current_category else [],
                        "_repo": repo,
                    })

        return items

    def _clean_md(self, text: str) -> str:
        """Remove Markdown formatting."""
        text = re.sub(r'\[([^\]]+)\]\([^\)]+\)', r'\1', text)  # [text](url) → text
        text = re.sub(r'\*\*(.+?)\*\*', r'\1', text)  # **bold** → bold
        text = re.sub(r'\*(.+?)\*', r'\1', text)  # *italic* → italic
        text = re.sub(r'`(.+?)`', r'\1', text)  # `code` → code
        return text.strip()

    def _extract_link(self, text: str) -> str:
        """Extract URL from Markdown text."""
        match = re.search(r'https?://[^\s\)]+', text)
        return match.group(0) if match else ""
