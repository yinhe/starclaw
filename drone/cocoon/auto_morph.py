"""
🧬 Cocoon Level 1: 自动变态 (Auto-Morph)
将外部平台原始数据转换为 StarClaw AgentTemplate 格式。
纯规则引擎，< 1秒/个，95% 的 Agent 走此路径。
"""

import json
import hashlib
import re
from pathlib import Path
from typing import Optional

import yaml

# Load tool mapping
_TOOL_MAP = {}
_map_file = Path(__file__).parent / "tool_mapping.yaml"
if _map_file.exists():
    with open(_map_file, "r", encoding="utf-8") as f:
        _TOOL_MAP = yaml.safe_load(f) or {}

# StarClaw categories
CATEGORIES = ["assistant", "coding", "writing", "data", "creative", "devops", "research"]

# Category keywords for auto-classification
CATEGORY_KEYWORDS = {
    "coding": ["code", "程序", "编程", "developer", "debug", "python", "javascript", "api", "github", "sql", "database"],
    "writing": ["write", "写作", "文案", "copywriting", "blog", "article", "essay", "content", "seo", "翻译", "translate"],
    "data": ["data", "数据", "analytics", "excel", "csv", "chart", "visualization", "statistics", "analysis"],
    "creative": ["design", "设计", "art", "creative", "image", "video", "music", "画", "剪辑", "logo", "ui", "ux"],
    "devops": ["deploy", "运维", "docker", "kubernetes", "ci/cd", "monitoring", "server", "linux", "cloud", "aws"],
    "research": ["research", "研究", "paper", "学术", "论文", "scientific", "review", "survey", "patent"],
    "assistant": ["assistant", "助手", "helper", "chat", "conversation", "general"],
}


class AutoMorph:
    """Level 1 同化器：规则引擎，快速批量转换。"""

    def __init__(self, config: dict = None):
        self.config = config or {}
        self.quality_config = self.config.get("quality", {})
        self.min_prompt_len = self.quality_config.get("min_prompt_length", 10)
        self.min_desc_len = self.quality_config.get("min_description_length", 20)
        self.skip_keywords = self.quality_config.get("skip_keywords", [])

    def morph(self, raw: dict, source: str) -> Optional[dict]:
        """
        将外部原始数据转换为 StarClaw AgentTemplate 格式。
        
        Args:
            raw: 外部平台的原始数据（各采集器已做初步结构化）
            source: 来源标识（clawhub, skillhub, coze, gpts, etc.）
        
        Returns:
            StarClaw AgentTemplate dict，或 None（质量不达标时）
        """
        # Step 1: 提取核心字段（各来源字段名不同）
        name = self._extract_name(raw, source)
        description = self._extract_description(raw, source)
        system_prompt = self._extract_prompt(raw, source)
        raw_tools = self._extract_tools(raw, source)
        raw_tags = self._extract_tags(raw, source)

        # Clean HTML tags from name
        name = re.sub(r'<[^>]+>', '', name).strip()

        # If no prompt but has description, use description as prompt
        if not system_prompt and description:
            system_prompt = description

        if not name or not system_prompt:
            return None

        # Step 2: 质量门控
        if not self._quality_check(name, description, system_prompt):
            return None

        # Step 3: 工具映射
        tools = self._map_tools(raw_tools)

        # Step 4: 分类
        category = self._classify(name, description, system_prompt, raw_tags)

        # Step 5: Prompt 清洗
        system_prompt = self._clean_prompt(system_prompt, source)

        # Step 6: 质量评分
        score = self._score(name, description, system_prompt, tools, raw_tags)

        # Step 7: 生成 source_id（去重用）
        source_id = self._source_id(raw, source)

        # Step 8: 猜测 icon
        icon = self._guess_icon(category, name)

        return {
            "name": name,
            "description": description[:200] if description else name,
            "category": category,
            "tags": json.dumps(raw_tags[:10] if raw_tags else [], ensure_ascii=False),
            "system_prompt": system_prompt,
            "tools": json.dumps(tools, ensure_ascii=False),
            "config": json.dumps({
                "source": source,
                "source_id": source_id,
                "source_url": raw.get("url", raw.get("link", "")),
                "original_tools": raw_tools,
                "cocoon_level": 1,
                "quality_score": score,
            }, ensure_ascii=False),
            "icon": icon,
            "is_builtin": False,
            # metadata for importer
            "_source": source,
            "_source_id": source_id,
            "_quality_score": score,
        }

    # ── Field Extraction (各来源适配) ──

    def _extract_name(self, raw: dict, source: str) -> str:
        for key in ["name", "title", "display_name", "bot_name"]:
            if raw.get(key):
                return str(raw[key]).strip()[:200]
        return ""

    def _extract_description(self, raw: dict, source: str) -> str:
        for key in ["description", "desc", "intro", "summary", "short_description"]:
            if raw.get(key):
                return str(raw[key]).strip()
        return ""

    def _extract_prompt(self, raw: dict, source: str) -> str:
        for key in ["system_prompt", "prompt", "instructions", "system_message",
                     "instruction", "system", "persona", "configuration"]:
            if raw.get(key):
                return str(raw[key]).strip()
        # Nested: some platforms put it in config.system_prompt
        config = raw.get("config", {})
        if isinstance(config, dict):
            for key in ["system_prompt", "prompt", "instructions"]:
                if config.get(key):
                    return str(config[key]).strip()
        return ""

    def _extract_tools(self, raw: dict, source: str) -> list:
        tools = raw.get("tools", raw.get("plugins", raw.get("actions", [])))
        if isinstance(tools, str):
            try:
                tools = json.loads(tools)
            except (json.JSONDecodeError, TypeError):
                tools = [t.strip() for t in tools.split(",") if t.strip()]
        if isinstance(tools, list):
            result = []
            for t in tools:
                if isinstance(t, str):
                    result.append(t)
                elif isinstance(t, dict):
                    result.append(t.get("name", t.get("type", str(t))))
            return result
        return []

    def _extract_tags(self, raw: dict, source: str) -> list:
        tags = raw.get("tags", raw.get("categories", raw.get("labels", [])))
        if isinstance(tags, str):
            try:
                tags = json.loads(tags)
            except (json.JSONDecodeError, TypeError):
                tags = [t.strip() for t in tags.split(",") if t.strip()]
        if isinstance(tags, list):
            return [str(t).strip().lower() for t in tags if t][:10]
        return []

    # ── Tool Mapping ──

    def _map_tools(self, raw_tools: list) -> list:
        """将外部工具名映射到 StarClaw 内置工具。"""
        mapped = set()
        for tool in raw_tools:
            tool_lower = str(tool).lower().strip().replace(" ", "_").replace("-", "_")
            if tool_lower in _TOOL_MAP:
                mapped.add(_TOOL_MAP[tool_lower])
            # Partial match
            else:
                for key, value in _TOOL_MAP.items():
                    if key in tool_lower or tool_lower in key:
                        mapped.add(value)
                        break
        return sorted(list(mapped))

    # ── Classification ──

    def _classify(self, name: str, desc: str, prompt: str, tags: list) -> str:
        """基于关键词自动分类。"""
        text = f"{name} {desc} {' '.join(tags)} {prompt[:500]}".lower()
        scores = {}
        for cat, keywords in CATEGORY_KEYWORDS.items():
            scores[cat] = sum(1 for kw in keywords if kw in text)
        if max(scores.values()) == 0:
            return "assistant"
        return max(scores, key=scores.get)

    # ── Prompt Cleaning ──

    def _clean_prompt(self, prompt: str, source: str) -> str:
        """清洗 prompt：删除平台特有指令。"""
        # Remove GPT-specific preambles
        patterns = [
            r"^You are a (?:custom )?GPT[.\s]",
            r"^As a GPT[,.\s]",
            r"^You are ChatGPT[,.\s]",
            r"^This GPT[.\s]",
            r"^I am a (?:custom )?GPT[.\s]",
            r"\bGPT[-\s]?Builder\b",
            r"\bOpenAI\s+policy\b",
            r"^You have access to the following tools:.*?(?=\n\n|\Z)",
        ]
        for pattern in patterns:
            prompt = re.sub(pattern, "", prompt, flags=re.IGNORECASE | re.MULTILINE)
        return prompt.strip()

    # ── Quality Gate ──

    def _quality_check(self, name: str, desc: str, prompt: str) -> bool:
        """质量门控：过滤低质量和违规内容。"""
        if len(prompt) < self.min_prompt_len:
            return False
        combined = f"{name} {desc} {prompt}".lower()
        for kw in self.skip_keywords:
            if kw.lower() in combined:
                return False
        return True

    def _score(self, name: str, desc: str, prompt: str, tools: list, tags: list) -> int:
        """质量评分 0-100。"""
        score = 0
        # Prompt length (max 30 points)
        if len(prompt) >= 100: score += 10
        if len(prompt) >= 300: score += 10
        if len(prompt) >= 1000: score += 10
        # Has description (10 points)
        if desc and len(desc) >= 20: score += 10
        # Has tools (15 points)
        if tools: score += min(len(tools) * 5, 15)
        # Has tags (10 points)
        if tags: score += min(len(tags) * 3, 10)
        # Name quality (10 points)
        if len(name) >= 3 and not name.startswith("Untitled"): score += 10
        # Prompt structure (25 points)
        if "你是" in prompt or "You are" in prompt.lower(): score += 10
        if any(marker in prompt for marker in ["##", "步骤", "Step", "规则", "Rule"]): score += 10
        if len(prompt.split("\n")) >= 5: score += 5
        return min(score, 100)

    # ── Dedup ──

    def _source_id(self, raw: dict, source: str) -> str:
        """生成去重用的 source_id。"""
        raw_id = raw.get("id", raw.get("_id", raw.get("slug", "")))
        if raw_id:
            return f"{source}:{raw_id}"
        # Fallback: hash of name + prompt
        name = self._extract_name(raw, source)
        prompt = self._extract_prompt(raw, source)
        h = hashlib.md5(f"{name}:{prompt[:200]}".encode()).hexdigest()[:12]
        return f"{source}:hash:{h}"

    # ── Icon ──

    def _guess_icon(self, category: str, name: str) -> str:
        icons = {
            "assistant": "🤖", "coding": "💻", "writing": "✍️",
            "data": "📊", "creative": "🎨", "devops": "🔧", "research": "🔬",
        }
        return icons.get(category, "🤖")


# ── Batch processing ──

def morph_batch(raw_list: list, source: str, config: dict = None) -> list:
    """批量同化，返回转换成功的 AgentTemplate 列表。"""
    morpher = AutoMorph(config)
    results = []
    seen_ids = set()
    for raw in raw_list:
        template = morpher.morph(raw, source)
        if template is None:
            continue
        sid = template.get("_source_id", "")
        if sid in seen_ids:
            continue
        seen_ids.add(sid)
        results.append(template)
    return results
