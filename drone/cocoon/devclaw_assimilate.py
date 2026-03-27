"""
🧬 Cocoon Level 3: DevClaw 深度同化
通过 Overlord → DevClaw 团队智能体（5角色）进行深度改造和测试。
~2分钟/个，仅用于顶级/复杂 Agent（手动触发）。

流程: 设计虫分析 → 编码虫重写 → 测试虫沙盒验证 → 审查虫安全检查 → 文档虫生成文案
输出: ✅ DevClaw Certified 标记
"""

import json
import requests
import time
from typing import Optional


class DevClawAssimilator:
    """Level 3 同化器：DevClaw 团队智能体深度改造。"""

    def __init__(self, overlord_url: str, overlord_token: str, claw_url: str):
        self.overlord_url = overlord_url.rstrip("/")
        self.overlord_token = overlord_token
        self.claw_url = claw_url.rstrip("/")

    def assimilate(self, template: dict, timeout: int = 300) -> Optional[dict]:
        """
        提交给 DevClaw 团队进行深度同化。

        1. 创建 Mission: "同化外部 Agent: {name}"
        2. 等待 DevClaw 5角色完成
        3. 提取输出并更新 template
        """
        name = template.get("name", "Unknown")
        config = json.loads(template.get("config", "{}"))
        source = config.get("source", "unknown")

        # Step 1: 找到 DevClaw 实例
        instance_id = self._find_devclaw_instance()
        if not instance_id:
            print(f"[cocoon-l3] No DevClaw instance found, skipping deep assimilation for '{name}'")
            return None

        # Step 2: 创建同化 Mission
        mission_desc = f"""同化外部 Agent 到 StarClaw 原生格式。

来源: {source}
原始名称: {name}
原始描述: {template.get('description', '')}
原始工具: {template.get('tools', '[]')}

原始 System Prompt:
{template.get('system_prompt', '')[:3000]}

要求:
1. 完全重写 system_prompt，适配 StarClaw 风格
2. 映射工具到 StarClaw 内置工具
3. 在沙盒中用3个测试用例验证功能
4. 检查安全性（无 jailbreak、无敏感内容）
5. 生成市场文案（中文名称、描述、标签）

输出 JSON 格式:
{{
  "name": "中文名称",
  "description": "卖点描述",
  "system_prompt": "完整重写的 prompt",
  "tools": ["tool1"],
  "tags": ["标签"],
  "icon": "emoji",
  "test_results": [{{"input": "测试问题", "passed": true}}],
  "security_review": "安全审查结论"
}}"""

        mission_id = self._create_mission(instance_id, mission_desc)
        if not mission_id:
            return None

        # Step 3: 轮询等待完成
        result = self._wait_mission(instance_id, mission_id, timeout)
        if not result:
            return None

        # Step 4: 解析结果并更新 template
        try:
            evolved = json.loads(result)
            if evolved.get("system_prompt"):
                template["system_prompt"] = evolved["system_prompt"]
            if evolved.get("name"):
                template["name"] = evolved["name"]
            if evolved.get("description"):
                template["description"] = evolved["description"][:200]
            if evolved.get("icon"):
                template["icon"] = evolved["icon"]
            if evolved.get("tags"):
                template["tags"] = json.dumps(evolved["tags"][:10], ensure_ascii=False)
            if evolved.get("tools"):
                template["tools"] = json.dumps(evolved["tools"], ensure_ascii=False)

            old_config = json.loads(template.get("config", "{}"))
            old_config["cocoon_level"] = 3
            old_config["devclaw_certified"] = True
            old_config["test_results"] = evolved.get("test_results", [])
            old_config["security_review"] = evolved.get("security_review", "")
            template["config"] = json.dumps(old_config, ensure_ascii=False)

            template["_quality_label"] = "certified"
            template["featured"] = True
            return template
        except (json.JSONDecodeError, KeyError) as e:
            print(f"[cocoon-l3] Failed to parse DevClaw output for '{name}': {e}")
            return None

    def _find_devclaw_instance(self) -> Optional[str]:
        """查找运行中的 DevClaw 团队实例。"""
        try:
            resp = requests.get(
                f"{self.overlord_url}/brood/team-agent/instances",
                headers={"Authorization": f"Bearer {self.overlord_token}"},
                timeout=10,
            )
            if resp.status_code != 200:
                return None
            instances = resp.json().get("instances", [])
            for inst in instances:
                if "devclaw" in inst.get("template_name", "").lower() and inst.get("status") == "running":
                    return inst["id"]
        except requests.RequestException:
            pass
        return None

    def _create_mission(self, instance_id: str, description: str) -> Optional[str]:
        """创建 DevClaw Mission。"""
        try:
            resp = requests.post(
                f"{self.overlord_url}/brood/team-agent/instances/{instance_id}/missions",
                headers={
                    "Authorization": f"Bearer {self.overlord_token}",
                    "Content-Type": "application/json",
                },
                json={"title": "Agent 同化任务", "description": description},
                timeout=15,
            )
            if resp.status_code in (200, 201):
                return resp.json().get("mission_id", resp.json().get("id"))
        except requests.RequestException as e:
            print(f"[cocoon-l3] Create mission failed: {e}")
        return None

    def _wait_mission(self, instance_id: str, mission_id: str, timeout: int) -> Optional[str]:
        """轮询等待 Mission 完成并返回结果。"""
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                resp = requests.get(
                    f"{self.overlord_url}/brood/team-agent/instances/{instance_id}/missions/{mission_id}",
                    headers={"Authorization": f"Bearer {self.overlord_token}"},
                    timeout=10,
                )
                if resp.status_code == 200:
                    data = resp.json()
                    if data.get("status") == "completed":
                        return data.get("result", data.get("output", ""))
                    if data.get("status") in ("failed", "cancelled"):
                        print(f"[cocoon-l3] Mission {mission_id} {data['status']}")
                        return None
            except requests.RequestException:
                pass
            time.sleep(10)
        print(f"[cocoon-l3] Mission {mission_id} timed out after {timeout}s")
        return None
