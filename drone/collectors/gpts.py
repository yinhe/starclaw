"""
🐝 GPTs Store 采集器 (Scrapling 反爬)
OpenAI GPTs Store，需要 Scrapling 绕过 Cloudflare。
同化难度: ⭐⭐⭐⭐ (prompt 风格 OpenAI 专属，L2 必须)
"""

from typing import List, Dict
from .base import BaseCollector


class GPTsCollector(BaseCollector):
    name = "gpts_store"

    def __init__(self, config: dict = None):
        super().__init__(config)
        self.max_pages = (config or {}).get("max_pages", 50)

    def collect(self, mode: str = "incremental") -> List[Dict]:
        try:
            from scrapling import StealthFetcher
        except ImportError:
            print("[gpts_store] scrapling not installed, run: pip install scrapling")
            return []

        seen = self.load_seen_ids() if mode == "incremental" else set()
        items = []
        fetcher = StealthFetcher(auto_match=True)

        # GPTs Store discovery pages
        urls = [
            "https://chatgpt.com/gpts",
            "https://chatgpt.com/gpts/trending",
        ]

        for url in urls:
            try:
                page = fetcher.get(url, timeout=30)
                if not page:
                    continue

                # Extract GPT cards (selector may change — Scrapling auto_match handles this)
                cards = page.find_all("a", href=lambda h: h and "/g/" in str(h))
                for card in cards[:self.max_pages]:
                    href = card.get("href", "")
                    gpt_id = href.split("/g/")[-1].split("?")[0].split("/")[0] if "/g/" in href else ""
                    if not gpt_id:
                        continue

                    sid = f"gpts:{gpt_id}"
                    if sid in seen:
                        continue
                    seen.add(sid)

                    name = ""
                    desc = ""
                    # Try extracting from card structure
                    h3 = card.find("h3")
                    if h3:
                        name = h3.text.strip()
                    p = card.find("p")
                    if p:
                        desc = p.text.strip()

                    if name:
                        items.append({
                            "id": gpt_id,
                            "name": name,
                            "description": desc,
                            "url": f"https://chatgpt.com/g/{gpt_id}",
                            "system_prompt": desc,  # Detail page scrape needed for full prompt
                            "tags": ["gpt"],
                        })

            except Exception as e:
                print(f"[gpts_store] scrape failed for {url}: {e}")
                continue

        self.save_seen_ids(seen)
        if items:
            self.save_raw(items)
        print(f"[gpts_store] collected {len(items)} GPTs (mode={mode})")
        return items
