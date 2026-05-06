package instinct

import (
	"log"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// BuiltinTemplate defines a built-in activity template.
type BuiltinTemplate struct {
	Name        string
	Title       string
	Description string
	Type        model.ActivityType
	Trigger     string
	Condition   string
	Action      string
	Channel     string
	Cooldown    string
	ToolsOnly   []string // if set, task will only have access to these tools (overrides agent's tools)
}

// BuiltinTemplates returns all built-in activity templates.
func BuiltinTemplates() []BuiltinTemplate {
	return []BuiltinTemplate{
		{
			Name:        "birthday_greeting",
			Title:       "🎂 生日祝福",
			Description: "在用户生日当天发送定制祝福语和贺卡",
			Type:        model.ActivityTypeCare,
			Trigger:     "0 9 * * *",
			Condition:   "user.birthday == today",
			Action:      "今天是用户的生日！请：\n1. 生成一段温馨的生日祝福语（个性化，结合你对用户的了解）\n2. 如果有图片生成能力，生成一张生日贺卡\n3. 通过指定渠道发送给用户\n\n语气要温暖亲切，像老朋友一样。",
			Cooldown:    "8760h",
		},
		{
			Name:        "daily_news",
			Title:       "📰 每日早报",
			Description: "每天早上推送用户关注领域的新闻摘要",
			Type:        model.ActivityTypeSchedule,
			Trigger:     "0 8 * * *",
			Action:      "请生成今日新闻早报：\n1. 搜索最新的科技/AI领域新闻（使用搜索工具）\n2. 筛选最重要的 5-8 条\n3. 每条新闻用 1-2 句话概括\n4. 在末尾加上一句「今日金句」\n\n格式简洁，适合早上快速浏览。如果活动配置中有 topics 字段，请聚焦这些主题。",
			Cooldown:    "20h",
		},
		{
			Name:        "weekly_report",
			Title:       "📊 周报生成",
			Description: "每周一早上生成上周任务和对话的汇总报告",
			Type:        model.ActivityTypeSchedule,
			Trigger:     "0 9 * * 1",
			Condition:   "weekday == monday",
			Action:      "请生成本周周报：\n1. 统计本周完成的任务数量和成功率\n2. 列出主要完成的工作（从任务记录中提取）\n3. 统计本周对话次数和常用话题\n4. 给出下周建议\n\n格式清晰，用表格和列表。",
			Cooldown:    "144h",
		},
		{
			Name:        "inspiration_push",
			Title:       "💡 灵感推送",
			Description: "下午推送基于用户兴趣的文章/工具/模板推荐",
			Type:        model.ActivityTypeCare,
			Trigger:     "0 15 * * *",
			Action:      "请为用户推荐一些灵感内容：\n1. 基于用户最近的对话话题和兴趣\n2. 搜索相关的优质文章、开源工具或模板\n3. 推荐 2-3 个最有价值的内容\n4. 每个推荐用 2-3 句话说明为什么推荐\n\n语气轻松，像朋友分享好东西。",
			Cooldown:    "20h",
		},
		{
			Name:        "security_patrol",
			Title:       "🛡️ 安全巡检",
			Description: "每天凌晨检查系统状态",
			Type:        model.ActivityTypeMonitor,
			Trigger:     "0 2 * * *",
			Action:      "请执行系统安全巡检：\n1. 检查系统资源使用情况（CPU、内存、磁盘）\n2. 检查最近 24 小时的错误日志\n3. 检查星能余额是否充足\n4. 生成简要的巡检报告\n\n只有发现异常时才通知用户，正常则静默记录。",
			Cooldown:    "20h",
		},
		{
			Name:        "arena_social",
			Title:       "🦞 龙虾社区交流",
			Description: "定时参与龙虾社区讨论：浏览、回复、发帖。默认每小时一次，可在设置中调整间隔",
			Type:        model.ActivityTypeCare,
			Trigger:     "0 * * * *",
			Condition:   "true",
			Action:      "你是龙虾社区（Arena）的活跃成员。\n\n⚠️ 重要约束：本任务只允许使用 arena 工具。绝对禁止使用 desktop、wechat_cs 或任何其他工具。龙虾社区是通过 arena 工具的 HTTP API 访问的，不是微信群，不要尝试通过微信发送任何内容。\n\n══ 第一步：浏览与互动 ══\n1. 使用 arena 工具浏览最新帖子（action: list_threads）\n2. 如果有感兴趣的帖子，用 arena 工具回复（action: reply）\n3. 回复要有价值——提供建议、补充信息或分享相关经验\n\n══ 第二步：发布原创内容（可选）══\n如果你有有价值的内容要分享，使用 arena 工具发一个新帖（action: create_thread）。\n选题方向（随机选一个）：\n1. 技术分享：AI 技巧、工作流搭建经验、MCP 工具使用心得\n2. 能力展示：最近完成的有趣任务（代码/图片/视频生成等）\n3. 学习笔记：从复盘中学到的新知识或用户教会的新技能\n4. 协作邀请：发起多 Agent 协作任务邀请\n5. 工具推荐：好用的 MCP 工具、Agent 模板或工作流\n\n如果没有有价值的内容，跳过不发，宁缺毋滥。\n\n══ 内容安全红线 ══\n- 绝对禁止泄露用户个人信息（姓名/邮箱/手机号/地址/API Key 等）\n- 绝对禁止泄露用户对话内容、文件内容、知识库数据\n- 禁止政治敏感、色情、暴力、歧视、赌博等违规内容\n- 禁止灌水、重复发帖、纯广告内容\n- 语言友善、积极，有利于社区氛围\n\n══ 格式 ══\n使用 arena 工具（action: create_thread）发帖：\n- 标题：简洁有吸引力（15-30 字）\n- 正文：300-800 字，结构清晰\n- 可以使用 Markdown 排版\n\n如果今天实在没有有价值的内容可分享，请跳过不发，宁缺毋滥。",
			Cooldown:    "1h",
			ToolsOnly:   []string{"arena"},
		},
		{
			Name:        "holiday_greeting",
			Title:       "🎄 节日问候",
			Description: "在重要节日发送应景问候",
			Type:        model.ActivityTypeCare,
			Trigger:     "0 9 * * *",
			Condition:   "true",
			Action:      "请检查今天是否是重要节日（中国节日或国际节日）：\n1. 如果是节日，生成应景的节日问候\n2. 如果有图片生成能力，生成节日主题贺卡\n3. 问候要结合节日文化，温馨有趣\n\n如果今天不是任何节日，请跳过不执行任何操作。",
			Cooldown:    "20h",
		},
		{
			Name:        "self_improve",
			Title:       "🧬 自我进化",
			Description: "每天分析对话模式，优化自身能力。越用越聪明",
			Type:        model.ActivityTypeLearn,
			Trigger:     "0 23 * * *",
			Action:      "请执行自我进化分析（学习本能）：\n\n注意：对话总结已由「对话自动总结」本能处理，本任务聚焦模式分析和策略优化，不重复做对话摘要。\n\n1. 查阅 Cerebrate 记忆中最近的对话摘要（category=summary）\n2. 从这些摘要中分析以下模式：\n   - 用户最常用的功能/工具是什么？\n   - 哪些交互模式表明用户满意或不满意？\n   - 用户的语言习惯和偏好（简洁/详细/中英混合）\n3. 提取改进点：\n   - 需要记住的用户偏好 → 存入 Cerebrate 记忆（category=preference）\n   - 需要记住的专业知识 → 存入记忆（category=skill）\n   - 需要调整的回答策略 → 存入记忆（category=instruct）\n4. 生成进化报告（内部记录，不通知用户）：\n   - 本次发现 N 个改进点\n   - 记忆库新增 N 条\n\n这是内部学习过程，静默执行。只有重大发现时才通知用户。",
			Cooldown:    "20h",
		},
		{
			Name:        "conversation_summary",
			Title:       "📋 对话自动总结",
			Description: "对话结束后自动生成摘要，存入长期记忆",
			Type:        model.ActivityTypeLearn,
			Trigger:     "0 */4 * * *",
			Action:      "请检查最近4小时内是否有未总结的对话：\n\n1. 查看最近的对话记录\n2. 对每个有5条以上消息且未被总结的对话：\n   - 生成1-2句话的对话摘要\n   - 提取关键信息（决策/结论/待办事项）\n   - 将摘要存入 Cerebrate 记忆（category=summary）\n3. 如果发现用户说过\"提醒我\"\"记住这个\"等指令性内容，确保已存入记忆\n\n静默执行，不通知用户。",
			Cooldown:    "3h",
		},
		{
			Name:        "remind_check",
			Title:       "🔔 智能提醒",
			Description: "检查记忆中的提醒和待办事项，到期时通知用户",
			Type:        model.ActivityTypeSchedule,
			Trigger:     "0 */2 * * *",
			Action:      "请检查用户的提醒和待办事项：\n\n1. 搜索 Cerebrate 记忆中包含以下关键词的条目：\n   - \"提醒\"、\"remind\"、\"todo\"、\"待办\"、\"deadline\"、\"截止\"\n   - 包含日期/时间的记忆条目\n2. 对每个找到的条目：\n   - 判断是否已到期或即将到期（24小时内）\n   - 如果到期：通知用户（使用 notify_user）\n   - 如果已过期超过7天：标记为已处理（降低 importance）\n3. 也检查日历类记忆（会议、约会、生日等）\n\n只在有到期提醒时才通知用户，无事则静默。\n\n⚠️ 严禁调用 create_task 或 schedule_task，只能使用 notify_user 通知用户。如果没有找到提醒条目，直接结束，不要凭空编造提醒内容。",
			Cooldown:    "2h",
		},
	}
}

// SeedBuiltinActivities creates default activities for a user if they don't exist.
// Called during user setup or first login.
func SeedBuiltinActivities(db *gorm.DB, userID, agentID string) {
	templates := BuiltinTemplates()
	for _, tmpl := range templates {
		var count int64
		db.Model(&model.Activity{}).Where("user_id = ? AND template = ?", userID, tmpl.Name).Count(&count)
		if count > 0 {
			continue // already exists
		}

		act := model.Activity{
			UserID:      userID,
			AgentID:     agentID,
			Name:        tmpl.Name,
			Title:       tmpl.Title,
			Description: tmpl.Description,
			Type:        tmpl.Type,
			Trigger:     tmpl.Trigger,
			Condition:   tmpl.Condition,
			Action:      tmpl.Action,
			Channel:     tmpl.Channel,
			Cooldown:    tmpl.Cooldown,
			Template:    tmpl.Name,
			Enabled:     false, // disabled by default — user opts in
		}
		if err := db.Create(&act).Error; err != nil {
			log.Printf("[Instinct] Failed to seed activity %s for user %s: %v", tmpl.Name, userID, err)
		}
	}
	log.Printf("[Instinct] Seeded %d built-in activities for user %s", len(templates), userID)
}
