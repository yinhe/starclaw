"""
🧬 Cocoon Level 2: LLM 进化 (LLM-Evolve)
调用 StarAI LLM 重写 prompt、增强描述、推荐模型。
~5秒/个，用于质量分 >= 80 的热门/高价值 Agent。
"""

import json
import requests
from typing import Optional

# LLM 进化的 System Prompt
EVOLVE_SYSTEM = """你是 StarClaw 的 Agent 同化引擎。
你的任务是将外部平台的 AI Agent 配置优化为 StarClaw 原生高质量格式。

## 规则
1. 重写 System Prompt:
   - 开头用"你是一个专业的[角色]"定义身份
   - 明确列出能力范围和限制
   - 删除所有平台特有指令（如"As a GPT", "You are a custom GPT", "OpenAI policy"）
   - 添加输出格式要求（结构化、Markdown）
   - 保持原始 Agent 的核心能力完整不丢失

2. 生成卖点描述（50字以内中文，突出核心价值）

3. 推荐最佳模型:
   - 简单对话: qwen-plus
   - 编程/推理: deepseek-chat
   - 创意/写作: qwen-max
   - 复杂任务: gpt-4o

4. 建议 icon emoji（1个）

5. 生成 3 个标签（中文）

## 输出 JSON（严格格式）
{
  "system_prompt": "重写后的完整 System Prompt",
  "description": "一句话卖点描述",
  "model": "推荐模型名",
  "icon": "emoji",
  "tags": ["标签1", "标签2", "标签3"],
  "changes": "同化改动说明（简要）"
}"""


class LLMEvolve:
    """Level 2 同化器：LLM 驱动的 prompt 重写和增强。"""

    def __init__(self, api_base: str, api_key: str = "", model: str = "qwen-plus"):
        self.api_base = api_base.rstrip("/")
        self.api_key = api_key
        self.model = model

    def evolve(self, template: dict) -> Optional[dict]:
        """
        对 L1 产出的 AgentTemplate 进行 LLM 进化。
        
        Args:
            template: L1 Auto-Morph 输出的 AgentTemplate dict
        
        Returns:
            增强后的 template，或 None（LLM 调用失败时）
        """
        name = template.get("name", "")
        prompt = template.get("system_prompt", "")
        desc = template.get("description", "")
        config = json.loads(template.get("config", "{}"))
        source = config.get("source", "unknown")

        user_msg = f"""## 输入（外部 Agent）
名称: {name}
描述: {desc}
来源平台: {source}
当前工具: {template.get('tools', '[]')}

原始 System Prompt:
{prompt[:3000]}"""

        try:
            result = self._chat(EVOLVE_SYSTEM, user_msg)
            if not result:
                return None

            evolved = json.loads(result)

            # 合并进化结果到 template
            if evolved.get("system_prompt"):
                template["system_prompt"] = evolved["system_prompt"]
            if evolved.get("description"):
                template["description"] = evolved["description"][:200]
            if evolved.get("icon"):
                template["icon"] = evolved["icon"]
            if evolved.get("tags"):
                template["tags"] = json.dumps(evolved["tags"][:10], ensure_ascii=False)
            if evolved.get("model"):
                old_config = json.loads(template.get("config", "{}"))
                old_config["recommended_model"] = evolved["model"]
                old_config["cocoon_level"] = 2
                old_config["evolve_changes"] = evolved.get("changes", "")
                template["config"] = json.dumps(old_config, ensure_ascii=False)

            # 标记为已进化
            template["_quality_label"] = "enhanced"
            return template

        except (json.JSONDecodeError, requests.RequestException, KeyError) as e:
            print(f"[cocoon-l2] evolve failed for '{name}': {e}")
            return None

    def _chat(self, system: str, user: str) -> Optional[str]:
        """调用 StarAI 兼容的 chat API。"""
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        resp = requests.post(
            f"{self.api_base}/chat/completions",
            headers=headers,
            json={
                "model": self.model,
                "messages": [
                    {"role": "system", "content": system},
                    {"role": "user", "content": user},
                ],
                "temperature": 0.3,
                "max_tokens": 4096,
            },
            timeout=30,
        )
        resp.raise_for_status()
        data = resp.json()
        content = data["choices"][0]["message"]["content"]

        # Extract JSON from markdown code block if present
        if "```json" in content:
            content = content.split("```json")[1].split("```")[0]
        elif "```" in content:
            content = content.split("```")[1].split("```")[0]

        return content.strip()


def evolve_batch(templates: list, api_base: str, api_key: str = "",
                 threshold: int = 80) -> list:
    """批量 L2 进化：仅对质量分 >= threshold 的模板执行。"""
    evolver = LLMEvolve(api_base, api_key)
    results = []
    for t in templates:
        score = t.get("_quality_score", 0)
        if score >= threshold:
            evolved = evolver.evolve(t)
            if evolved:
                results.append(evolved)
                continue
        results.append(t)  # 不符合条件的原样返回
    return results
