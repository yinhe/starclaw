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
