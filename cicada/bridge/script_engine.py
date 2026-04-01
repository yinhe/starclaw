"""
Cicada 🪰 Script Engine — 话术模板引擎
"""

import json
import logging
import os
from typing import Optional

import yaml

logger = logging.getLogger("cicada.script")


# 内置行业话术
BUILTIN_SCRIPTS = {
    "real_estate": {
        "name": "房产新盘推广",
        "industry": "real_estate",
        "voice": "longxiaochun",
        "greeting": "您好，我是{company}的置业顾问，耽误您一分钟时间。我们{project_name}最近有一批优质房源，想跟您简单介绍一下。",
        "key_points": [
            "项目位于{location}，周边配套成熟",
            "户型从{min_area}到{max_area}平米",
            "均价{price}元/平米，首付{down_payment}万起",
        ],
        "qa_library": [
            {"q": "在什么位置", "a": "项目位于{location}，交通非常方便"},
            {"q": "多少钱", "a": "目前均价{price}元/平米，总价约{total_price}万"},
            {"q": "能分期吗", "a": "可以的，首付{down_payment_ratio}，月供约{monthly_payment}元"},
            {"q": "有学区吗", "a": "项目对口{school}，是本区重点学校"},
        ],
        "objections": [
            {"trigger": "太贵了", "response": "我们现在有特价房源，比正常优惠{discount}万"},
            {"trigger": "再考虑", "response": "没问题，我发份资料到您手机上，您方便时看看"},
            {"trigger": "不需要", "response": "好的，打扰了，祝您生活愉快，再见"},
        ],
        "closing": {
            "positive": "那我安排置业顾问跟您详细沟通，您看明天方便吗？",
            "neutral": "我把项目资料发您手机上，有问题随时联系我们",
            "negative": "好的，感谢您的时间，祝您生活愉快，再见",
        },
    },
    "education": {
        "name": "教育课程推广",
        "industry": "education",
        "voice": "longxiaoxia",
        "greeting": "您好，我是{company}的课程顾问，了解到您家孩子可能对{subject}感兴趣，想跟您聊两分钟。",
        "key_points": [
            "{subject}课程由资深老师授课",
            "小班制教学，每班不超过{class_size}人",
            "免费试听一节课，满意再报名",
        ],
        "qa_library": [
            {"q": "多大的孩子", "a": "我们{subject}课程适合{age_range}岁的孩子"},
            {"q": "在哪里上课", "a": "我们在{location}有校区，也支持线上课程"},
            {"q": "多少钱", "a": "{subject}课程{price}元/{unit}，现在报名有优惠"},
            {"q": "什么时候上课", "a": "周末和工作日晚上都有课，可以灵活选择"},
        ],
        "objections": [
            {"trigger": "太贵了", "response": "我们现在有试听优惠，先免费体验一节"},
            {"trigger": "孩子不愿意", "response": "很多孩子试听后都很喜欢，可以先来体验"},
            {"trigger": "不需要", "response": "好的，打扰了，如果以后有需要可以联系我们"},
        ],
        "closing": {
            "positive": "那我帮您预约一节免费试听课，您看周末方便吗？",
            "neutral": "我把课程介绍发您微信，您有空看看",
            "negative": "好的，祝您和孩子生活愉快，再见",
        },
    },
    "finance": {
        "name": "金融贷款推广",
        "industry": "finance",
        "voice": "longshu",
        "greeting": "您好，我是{company}的金融顾问，近期有一批低息贷款产品，想了解下您是否有资金需求。",
        "key_points": [
            "年化利率低至{rate}%",
            "额度{min_amount}到{max_amount}万",
            "最快当天放款，随借随还",
        ],
        "qa_library": [
            {"q": "利息多少", "a": "年化利率{rate}%起，具体根据您的资质评估"},
            {"q": "能贷多少", "a": "额度{min_amount}到{max_amount}万，根据收入和资产评估"},
            {"q": "需要什么条件", "a": "有稳定收入即可，无需抵押"},
            {"q": "多久放款", "a": "审批通过后最快当天放款"},
        ],
        "objections": [
            {"trigger": "不需要贷款", "response": "好的，如果以后有需要可以联系我们"},
            {"trigger": "利息太高", "response": "我们有多种产品，可以根据您的情况匹配最优方案"},
            {"trigger": "不需要", "response": "好的，打扰了，祝您生活愉快"},
        ],
        "closing": {
            "positive": "那我帮您做个初步评估，方便提供下您的基本信息吗？",
            "neutral": "我发份产品介绍给您，有需要随时联系",
            "negative": "好的，感谢您的时间，再见",
        },
    },
    "general": {
        "name": "通用话术",
        "industry": "general",
        "voice": "longxiaochun",
        "greeting": "您好，我是{company}的客服，想跟您简单介绍下我们的{product}。",
        "key_points": [
            "{product}目前有优惠活动",
            "已有{user_count}位客户选择了我们",
        ],
        "qa_library": [
            {"q": "多少钱", "a": "我们{product}现在{price}，目前有优惠"},
            {"q": "在哪里", "a": "我们在{location}，也支持线上服务"},
        ],
        "objections": [
            {"trigger": "不需要", "response": "好的，打扰了，祝您生活愉快，再见"},
            {"trigger": "再考虑", "response": "没问题，我发份资料给您参考"},
        ],
        "closing": {
            "positive": "那我帮您预约一下，您看什么时间方便？",
            "neutral": "我把详细资料发给您，有问题随时联系",
            "negative": "好的，感谢您的时间，再见",
        },
    },
}


class ScriptEngine:
    """话术引擎 — 管理话术模板 + 生成 LLM System Prompt"""

    def __init__(self, scripts_dir: Optional[str] = None):
        self.scripts_dir = scripts_dir
        self.custom_scripts: dict[str, dict] = {}
        if scripts_dir:
            self._load_custom_scripts()

    def _load_custom_scripts(self):
        """从 YAML 文件加载自定义话术"""
        if not self.scripts_dir or not os.path.exists(self.scripts_dir):
            return
        for fname in os.listdir(self.scripts_dir):
            if fname.endswith((".yaml", ".yml")):
                path = os.path.join(self.scripts_dir, fname)
                try:
                    with open(path, "r", encoding="utf-8") as f:
                        data = yaml.safe_load(f)
                    if data and "name" in data:
                        key = fname.rsplit(".", 1)[0]
                        self.custom_scripts[key] = data
                        logger.info(f"[script] loaded: {key} ({data['name']})")
                except Exception as e:
                    logger.error(f"[script] load error {fname}: {e}")

    def get_script(self, industry: str) -> dict:
        """获取话术模板（自定义优先 > 内置）"""
        if industry in self.custom_scripts:
            return self.custom_scripts[industry]
        return BUILTIN_SCRIPTS.get(industry, BUILTIN_SCRIPTS["general"])

    def list_scripts(self) -> list[dict]:
        """列出所有可用话术"""
        result = []
        for key, script in BUILTIN_SCRIPTS.items():
            result.append({
                "key": key,
                "name": script["name"],
                "industry": script["industry"],
                "is_builtin": True,
            })
        for key, script in self.custom_scripts.items():
            result.append({
                "key": key,
                "name": script.get("name", key),
                "industry": script.get("industry", "custom"),
                "is_builtin": False,
            })
        return result

    def build_system_prompt(self, industry: str, variables: dict = None) -> str:
        """
        构建 LLM System Prompt
        将话术模板注入到 prompt 中，指导 AI 如何对话
        """
        script = self.get_script(industry)
        variables = variables or {}

        # 替换变量
        greeting = self._render(script.get("greeting", ""), variables)
        key_points = [self._render(p, variables) for p in script.get("key_points", [])]
        qa_pairs = script.get("qa_library", [])
        objections = script.get("objections", [])
        closing = script.get("closing", {})

        # 构建 Q&A 参考
        qa_section = ""
        if qa_pairs:
            qa_lines = []
            for qa in qa_pairs:
                q = self._render(qa.get("q", ""), variables)
                a = self._render(qa.get("a", ""), variables)
                qa_lines.append(f"  问: {q}\n  答: {a}")
            qa_section = "\n".join(qa_lines)

        # 构建异议处理
        objection_section = ""
        if objections:
            obj_lines = []
            for obj in objections:
                trigger = obj.get("trigger", "")
                response = self._render(obj.get("response", ""), variables)
                obj_lines.append(f'  客户说\u201c{trigger}\u201d → 回复: {response}')
            objection_section = "\n".join(obj_lines)

        prompt = f"""你是一个专业的电话外呼机器人。你的目标是通过自然、友好的对话识别客户意向。

核心规则：
1. 每句话不超过30字，语速适中，像真人一样自然
2. 不主动透露自己是机器人，但被问到时如实回答
3. 客户说"不需要"/"别打了"时，礼貌结束通话并说再见
4. 根据客户提问深度判断意向等级
5. 始终引导客户说出需求关键词
6. 禁止承诺任何不确定的信息
7. 禁止使用"最好""第一""绝对"等绝对化用语

开场白：
{greeting}

关键卖点：
{chr(10).join(f"- {p}" for p in key_points)}

常见问答参考：
{qa_section}

异议处理：
{objection_section}

结束话术：
- 客户有兴趣: {self._render(closing.get("positive", ""), variables)}
- 客户态度中性: {self._render(closing.get("neutral", ""), variables)}
- 客户拒绝: {self._render(closing.get("negative", ""), variables)}

注意：以上是参考话术，请根据实际对话灵活应答，保持自然流畅。"""

        return prompt

    @staticmethod
    def _render(template: str, variables: dict) -> str:
        """简单变量替换 {key} → value"""
        result = template
        for key, value in variables.items():
            result = result.replace(f"{{{key}}}", str(value))
        return result
