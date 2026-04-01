"""
Cicada 🪰 Intent Classifier — A-F 六级意向分类引擎
"""

import json
import logging
from typing import Optional

logger = logging.getLogger("cicada.intent")

INTENT_LEVELS = {
    "A": {"name": "强意向", "score_range": [80, 100], "color": "#FF4D4F",
           "action": "立即通知业务员，24小时内人工跟进"},
    "B": {"name": "较强意向", "score_range": [60, 79], "color": "#FF7A45",
           "action": "48小时内人工跟进，发送资料短信"},
    "C": {"name": "一般意向", "score_range": [40, 59], "color": "#FFC53D",
           "action": "加入培育队列，3天后二次外呼"},
    "D": {"name": "弱意向", "score_range": [20, 39], "color": "#73D13D",
           "action": "加入低优队列，7天后二次外呼"},
    "E": {"name": "明确拒绝", "score_range": [1, 19], "color": "#597EF7",
           "action": "标记拒绝，30天内不再外呼"},
    "F": {"name": "无效号码", "score_range": [0, 0], "color": "#8C8C8C",
           "action": "移入无效池，不再外呼"},
}

CLASSIFY_PROMPT = """分析以下电话通话记录，判断客户意向等级。

行业: {industry}
通话时长: {duration}秒

通话文字:
{transcript}

请按以下维度打分（每项0-100）：
1. need - 需求明确度：客户是否清楚表达了需求
2. depth - 问题深度：客户提问的专业度和细节程度
3. timeline - 时间意愿：客户是否有明确的时间计划
4. engagement - 互动积极性：客户的回应频率和态度
5. buying_signal - 购买信号：是否出现价格/付款/预约等购买信号

意向等级标准：
A(80-100): 主动询问价格/付款/优惠/预约，≥3个深度问题
B(60-79): 询问具体细节(位置/配置/时间)，≥2个问题
C(40-59): 有兴趣但不深入，1个问题或"发个资料看看"
D(20-39): 未拒绝但无实质问题，"嗯""好的"为主
E(1-19): "不需要""别打了""我在开会"
F(0): 空号/停机/无人接听/忙音

请严格输出以下 JSON 格式（不要输出任何其他内容）：
{{"scores": {{"need": 0, "depth": 0, "timeline": 0, "engagement": 0, "buying_signal": 0}}, "total_score": 0, "level": "A", "key_interests": ["关键词1", "关键词2"], "summary": "一句话总结客户情况", "next_action": "建议的下一步动作"}}"""


class IntentClassifier:
    """意向分类器 — 通话结束后由 LLM 分析"""

    def __init__(self, llm_client, industry: str = "通用"):
        self.llm = llm_client
        self.industry = industry

    async def classify(self, transcript: str, duration: int = 0,
                       industry: Optional[str] = None) -> dict:
        """
        分析通话文字，返回意向分类结果
        """
        if not transcript or not transcript.strip():
            return self._empty_result("F", "无通话内容")

        # 通话太短（<5秒），直接判 D 或 E
        if duration < 5 and duration > 0:
            return self._empty_result("E", "通话时间极短")

        prompt = CLASSIFY_PROMPT.format(
            industry=industry or self.industry,
            duration=duration,
            transcript=transcript,
        )

        try:
            response = await self.llm.chat([
                {"role": "system", "content": "你是一个专业的电话销售意向分析师。只输出JSON，不要输出任何其他内容。"},
                {"role": "user", "content": prompt},
            ])

            result = self._parse_response(response)
            logger.info(
                f"[intent] classified: level={result['level']} "
                f"score={result['total_score']} summary={result['summary'][:50]}"
            )
            return result

        except Exception as e:
            logger.error(f"[intent] classify error: {e}")
            return self._empty_result("D", f"分类失败: {e}")

    def _parse_response(self, response: str) -> dict:
        """解析 LLM 返回的 JSON"""
        # 尝试提取 JSON
        text = response.strip()

        # 去掉可能的 markdown 代码块
        if text.startswith("```"):
            lines = text.split("\n")
            text = "\n".join(lines[1:-1]) if len(lines) > 2 else text

        # 查找 JSON 对象
        start = text.find("{")
        end = text.rfind("}") + 1
        if start >= 0 and end > start:
            text = text[start:end]

        try:
            data = json.loads(text)
        except json.JSONDecodeError:
            logger.warning(f"[intent] failed to parse JSON: {text[:200]}")
            return self._empty_result("D", "JSON解析失败")

        # 验证和规范化
        scores = data.get("scores", {})
        total = data.get("total_score", 0)
        level = data.get("level", "D").upper()

        # 确保 total_score 合理
        if not total and scores:
            values = [v for v in scores.values() if isinstance(v, (int, float))]
            total = int(sum(values) / max(len(values), 1))

        # 确保 level 在有效范围
        if level not in INTENT_LEVELS:
            level = self._score_to_level(total)

        # 验证 level 和 score 一致性
        expected_level = self._score_to_level(total)
        if level != expected_level:
            # LLM 判断优先，但 score 差异太大时修正
            level_range = INTENT_LEVELS[level]["score_range"]
            if total < level_range[0] - 10 or total > level_range[1] + 10:
                level = expected_level

        return {
            "scores": scores,
            "total_score": total,
            "level": level,
            "key_interests": data.get("key_interests", []),
            "summary": data.get("summary", ""),
            "next_action": data.get("next_action", INTENT_LEVELS[level]["action"]),
            "level_name": INTENT_LEVELS[level]["name"],
            "level_color": INTENT_LEVELS[level]["color"],
        }

    @staticmethod
    def _score_to_level(score: int) -> str:
        if score >= 80:
            return "A"
        elif score >= 60:
            return "B"
        elif score >= 40:
            return "C"
        elif score >= 20:
            return "D"
        elif score >= 1:
            return "E"
        return "F"

    @staticmethod
    def _empty_result(level: str, summary: str) -> dict:
        info = INTENT_LEVELS.get(level, INTENT_LEVELS["F"])
        return {
            "scores": {"need": 0, "depth": 0, "timeline": 0, "engagement": 0, "buying_signal": 0},
            "total_score": info["score_range"][0],
            "level": level,
            "key_interests": [],
            "summary": summary,
            "next_action": info["action"],
            "level_name": info["name"],
            "level_color": info["color"],
        }


class RealtimeIntentTracker:
    """
    实时意向追踪器 — 通话进行中增量评估
    每收到一段 ASR 文字就更新预判
    """

    def __init__(self):
        self.turns: list[dict] = []
        self.current_level = "D"
        self.current_score = 20
        self.question_count = 0
        self.buying_signals = 0

    # 购买信号关键词
    BUYING_KEYWORDS = [
        "多少钱", "价格", "费用", "首付", "月供", "分期", "优惠", "折扣",
        "预约", "看看", "什么时候", "怎么买", "怎么付", "报名", "签约",
        "试听", "体验", "加盟", "投资", "回本",
    ]

    # 拒绝关键词
    REJECT_KEYWORDS = [
        "不需要", "别打了", "不要再打", "没兴趣", "不考虑",
        "开会", "忙", "挂了", "投诉", "骚扰",
    ]

    def update(self, role: str, text: str) -> dict:
        """
        增量更新意向评估
        返回: {"level": "A-F", "score": 0-100, "changed": bool}
        """
        self.turns.append({"role": role, "text": text})
        old_level = self.current_level

        if role == "customer":
            text_lower = text.lower()

            # 检测拒绝信号
            for kw in self.REJECT_KEYWORDS:
                if kw in text_lower:
                    self.current_level = "E"
                    self.current_score = 10
                    return {"level": "E", "score": 10, "changed": old_level != "E"}

            # 检测购买信号
            for kw in self.BUYING_KEYWORDS:
                if kw in text_lower:
                    self.buying_signals += 1
                    break

            # 检测是否提问（问号或疑问词）
            if "？" in text or "?" in text or any(
                w in text for w in ["吗", "呢", "什么", "怎么", "多少", "哪里", "几"]
            ):
                self.question_count += 1

            # 根据问题数和购买信号更新评分
            self.current_score = min(100, 20 + self.question_count * 15 + self.buying_signals * 20)
            self.current_level = IntentClassifier._score_to_level(self.current_score)

        return {
            "level": self.current_level,
            "score": self.current_score,
            "changed": old_level != self.current_level,
            "question_count": self.question_count,
            "buying_signals": self.buying_signals,
        }

    def get_current(self) -> dict:
        info = INTENT_LEVELS.get(self.current_level, INTENT_LEVELS["D"])
        return {
            "level": self.current_level,
            "score": self.current_score,
            "level_name": info["name"],
            "level_color": info["color"],
            "question_count": self.question_count,
            "buying_signals": self.buying_signals,
        }
