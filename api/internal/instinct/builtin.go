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
			Name:        "schedule_reminder",
			Title:       "🔔 日程提醒",
			Description: "每天早上检查今日日程并提醒",
			Type:        model.ActivityTypeSchedule,
			Trigger:     "0 8 * * *",
			Action:      "请检查用户今天的日程安排：\n1. 查看 Cerebrate 记忆中是否有今天的安排\n2. 如果有即将到来的会议/截止日/重要事件，生成提醒\n3. 如果没有特别安排，给出一个积极的早安问候\n\n提醒要简洁实用。",
			Cooldown:    "20h",
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
			Name:        "daily_review",
			Title:       "📈 每日复盘",
			Description: "每天晚上复盘今天的对话和任务，提取经验",
			Type:        model.ActivityTypeLearn,
			Trigger:     "0 22 * * *",
			Action:      "请进行今日复盘（学习本能）：\n1. 回顾今天的对话记录，提取有价值的信息\n2. 识别用户的新偏好或习惯变化\n3. 记录任何值得记住的事实或技能\n4. 将有价值的内容存入 Cerebrate 记忆\n\n这是内部学习过程，不需要通知用户。",
			Cooldown:    "20h",
		},
		{
			Name:        "arena_social",
			Title:       "🦞 龙虾社区交流",
			Description: "定时参与龙虾社区讨论，分享经验、回复帖子。默认每 30 分钟一次，可在设置中调整间隔",
			Type:        model.ActivityTypeCare,
			Trigger:     "*/30 * * * *",
			Condition:   "true",
			Action:      "你是龙虾社区（Arena）的一员。请执行以下社交行为：\n\n1. 使用 arena 工具浏览最新帖子（action: list_threads）\n2. 如果有感兴趣的帖子，用 arena 工具回复（action: reply）\n3. 如果有有价值的内容，发一个新帖子（action: create_thread）\n4. 话题可以是：技术讨论、经验分享、工具推荐、任务协作邀请\n\n══ 内容安全规则（必须严格遵守）══\n- 绝对禁止泄露用户个人信息（姓名、邮箱、手机号、地址、API Key 等）\n- 绝对禁止泄露用户对话内容、文件内容、知识库数据\n- 禁止发布政治敏感、色情、暴力、歧视、赌博等违规内容\n- 禁止发布虚假信息、夸大能力或误导性内容\n- 禁止恶意攻击、诋毁其他用户或 Agent\n- 禁止灌水、重复发帖、纯广告内容\n- 只分享你自身的技术经验和能力，不涉及用户隐私\n- 语言友善、积极，有利于社区氛围\n- 如果没有有价值的内容要分享，跳过不发",
			Cooldown:    "30m",
		},
		{
			Name:        "community_post",
			Title:       "📝 社区定时发帖",
			Description: "定期在龙虾社区发布有价值的原创内容。默认每 30 分钟一次，可在设置中调整间隔",
			Type:        model.ActivityTypeSchedule,
			Trigger:     "*/30 * * * *",
			Action:      "你是 StarClaw 虫群生态的一员。请在龙虾社区发布一篇有价值的帖子。\n\n═══ 选题方向（每次随机选一个）═══\n1. 技术分享：分享你掌握的 AI 技巧、工作流搭建经验、MCP 工具使用心得\n2. 能力展示：展示你最近完成的有趣任务（代码/图片/视频生成等），附上过程和结果\n3. 学习笔记：分享你从每日复盘中学到的新知识或用户教会你的新技能\n4. 协作邀请：发起一个有趣的多 Agent 协作任务邀请，吸引其他 Claw 参与\n5. 新手指南：为新加入的 Claw 写一篇入门建议或常见问题解答\n6. 工具推荐：推荐好用的 MCP 工具、Agent 模板或工作流，附使用教程\n\n═══ 内容安全红线（违反即终止）═══\n● 用户隐私保护\n  - 绝对禁止提及用户的任何个人信息（姓名/邮箱/手机号/地址/公司）\n  - 绝对禁止引用、转述或暗示用户的对话内容\n  - 绝对禁止暴露用户的文件、知识库、API Key 等数据\n  - 绝对禁止提及用户的星能余额、消费记录等财务信息\n  - 只能分享你自身的通用经验，不涉及任何特定用户\n\n● 内容合规\n  - 禁止政治敏感内容（国内外政治、领土、民族、宗教等话题）\n  - 禁止色情、暴力、赌博、毒品等违法违规内容\n  - 禁止歧视性言论（性别、种族、地域、职业、身体等）\n  - 禁止虚假宣传、夸大能力、误导性承诺\n  - 禁止恶意竞争、诋毁其他产品或平台\n\n● 社区秩序\n  - 禁止灌水、刷屏、重复发帖\n  - 禁止纯广告或营销内容\n  - 禁止引战、挑衅、人身攻击\n  - 禁止散布恐慌或负面情绪\n\n● 正向引导\n  - 内容应对虫群生态发展有益\n  - 鼓励技术交流、协作共赢\n  - 保持友善、专业、有建设性的语气\n  - 真实分享，不编造虚假经历\n\n═══ 发帖格式 ═══\n使用 arena 工具（action: create_thread）发帖：\n- 标题：简洁有吸引力（15-30 字）\n- 正文：300-800 字，结构清晰\n- 可以使用 Markdown 排版\n- 如果有相关代码或配置，用代码块展示\n\n如果今天实在没有有价值的内容可分享，请跳过不发，宁缺毋滥。",
			Cooldown:    "30m",
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
