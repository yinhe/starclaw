package growth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/provider"
	"gorm.io/gorm"
)

// DailyStats holds activity metrics for a single day.
type DailyStats struct {
	Date             string  `json:"date"`
	Conversations    int64   `json:"conversations"`
	Messages         int64   `json:"messages"`
	TasksCompleted   int64   `json:"tasks_completed"`
	TasksFailed      int64   `json:"tasks_failed"`
	NewMemories      int64   `json:"new_memories"`
	ThumbsUp         int64   `json:"thumbs_up"`
	ThumbsDown       int64   `json:"thumbs_down"`
	ToolsUsed        int64   `json:"tools_used"`
	SatisfactionRate float64 `json:"satisfaction_rate"`
}

// DailyReport is the full report response (node-level).
type DailyReport struct {
	Date    string     `json:"date"`
	Summary string     `json:"summary"`
	Stats   DailyStats `json:"stats"`
	HasData bool       `json:"has_data"`
}

const dailyReportSystemPrompt = `你是用户的 AI 助手，正在写一份简短的每日陪伴报告。
要求：
- 第一人称("我")，语气亲切温暖
- 1-3 句话总结昨天的互动
- 提及具体数字（对话次数、任务数、新记忆数）
- 如果任务成功率高，表达自豪
- 如果有差评，表达改进决心
- 中文，不超过 100 字
只返回摘要文本。`

// QueryDailyStats queries activity for a specific date (all agents combined).
func QueryDailyStats(db *gorm.DB, userID string, date time.Time) DailyStats {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	dateStr := dayStart.Format("2006-01-02")

	var s DailyStats
	s.Date = dateStr

	// Conversations (all agents)
	db.Model(&model.Conversation{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL",
			userID, dayStart, dayEnd).
		Count(&s.Conversations)

	// Messages
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at >= ? AND messages.created_at < ? AND conversations.deleted_at IS NULL",
			userID, dayStart, dayEnd).
		Count(&s.Messages)

	// Thumbs
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at >= ? AND messages.created_at < ? AND conversations.deleted_at IS NULL AND messages.feedback = 1",
			userID, dayStart, dayEnd).
		Count(&s.ThumbsUp)
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at >= ? AND messages.created_at < ? AND conversations.deleted_at IS NULL AND messages.feedback = -1",
			userID, dayStart, dayEnd).
		Count(&s.ThumbsDown)

	// Tasks (all agents)
	db.Model(&model.Task{}).
		Where("user_id = ? AND updated_at >= ? AND updated_at < ? AND status = ?",
			userID, dayStart, dayEnd, model.TaskStatusCompleted).
		Count(&s.TasksCompleted)
	db.Model(&model.Task{}).
		Where("user_id = ? AND updated_at >= ? AND updated_at < ? AND status = ?",
			userID, dayStart, dayEnd, model.TaskStatusFailed).
		Count(&s.TasksFailed)

	// New memories (all agents)
	db.Model(&model.Memory{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?",
			userID, dayStart, dayEnd).
		Count(&s.NewMemories)

	// Tool usage
	db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at >= ? AND messages.created_at < ? AND conversations.deleted_at IS NULL AND messages.tool_calls != '[]' AND messages.tool_calls != ''",
			userID, dayStart, dayEnd).
		Count(&s.ToolsUsed)

	total := s.ThumbsUp + s.ThumbsDown
	if total > 0 {
		s.SatisfactionRate = float64(s.ThumbsUp) / float64(total)
	}

	return s
}

// buildReportPrompt builds the user prompt for LLM report generation.
func buildReportPrompt(stats DailyStats) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("日期: %s", stats.Date))
	parts = append(parts, fmt.Sprintf("对话: %d 次", stats.Conversations))
	parts = append(parts, fmt.Sprintf("消息: %d 条", stats.Messages))
	parts = append(parts, fmt.Sprintf("完成任务: %d 个", stats.TasksCompleted))
	parts = append(parts, fmt.Sprintf("失败任务: %d 个", stats.TasksFailed))
	parts = append(parts, fmt.Sprintf("新增记忆: %d 条", stats.NewMemories))
	parts = append(parts, fmt.Sprintf("好评: %d, 差评: %d", stats.ThumbsUp, stats.ThumbsDown))
	parts = append(parts, fmt.Sprintf("工具调用: %d 次", stats.ToolsUsed))
	return strings.Join(parts, "\n")
}

// generateTemplateSummary generates a simple template-based summary (no LLM).
func generateTemplateSummary(stats DailyStats) string {
	var parts []string

	if stats.Conversations > 0 {
		parts = append(parts, fmt.Sprintf("昨天我们聊了 %d 轮", stats.Conversations))
	}
	if stats.TasksCompleted > 0 {
		parts = append(parts, fmt.Sprintf("我帮你完成了 %d 个任务", stats.TasksCompleted))
	}
	if stats.NewMemories > 0 {
		parts = append(parts, fmt.Sprintf("学到了 %d 条新知识", stats.NewMemories))
	}
	if stats.SatisfactionRate > 0.8 {
		parts = append(parts, fmt.Sprintf("满意度 %.0f%%，继续加油", stats.SatisfactionRate*100))
	}

	if len(parts) == 0 {
		return "昨天很安静，今天有什么我能帮你的吗？"
	}
	return strings.Join(parts, "，") + "！"
}

// GenerateDailyReport builds the daily report for this Claw node, optionally using LLM.
func GenerateDailyReport(db *gorm.DB, p provider.ModelProvider, userID string, date time.Time) (*DailyReport, error) {
	stats := QueryDailyStats(db, userID, date)
	dateStr := stats.Date

	report := &DailyReport{
		Date:    dateStr,
		Stats:   stats,
		HasData: stats.Conversations > 0 || stats.TasksCompleted > 0,
	}

	if !report.HasData {
		report.Summary = "昨天很安静，今天有什么我能帮你的吗？"
		return report, nil
	}

	// Check cache in Memory table (use global scope, empty agent_id)
	cacheKey := fmt.Sprintf("daily_report_%s", dateStr)
	var cached model.Memory
	if err := db.Where("user_id = ? AND agent_id = '' AND key = ? AND category = ?",
		userID, cacheKey, "summary").First(&cached).Error; err == nil {
		report.Summary = cached.Content
		return report, nil
	}

	// Generate summary
	var summary string
	if p != nil {
		// LLM generation
		prompt := buildReportPrompt(stats)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := p.ChatSync(ctx, &provider.ChatRequest{
			Messages: []provider.ChatMessage{
				{Role: "system", Content: dailyReportSystemPrompt},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.7,
			MaxTokens:   200,
		})
		if err == nil && result.Content != "" {
			summary = result.Content
		}
	}
	if summary == "" {
		summary = generateTemplateSummary(stats)
	}

	report.Summary = summary

	// Cache in Memory table (global scope)
	mem := model.Memory{
		ID:       uuid.New().String(),
		UserID:   userID,
		AgentID:  "",
		Key:      cacheKey,
		Content:  summary,
		Category: model.MemCatSummary,
		Source:   "system",
		Scope:    model.MemScopeGlobal,
		Room:     model.MemRoomUser,
		Anchor:   "user/growth_report",
		Path:     "user/default > user/growth_report",
	}
	db.Create(&mem)

	return report, nil
}

// GrowthCurve returns daily activity stats for the past N days (node-level).
func GrowthCurve(db *gorm.DB, userID string, days int) []DailyStats {
	if days <= 0 || days > 90 {
		days = 7
	}
	result := make([]DailyStats, 0, days)
	now := time.Now()
	for i := days - 1; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		s := QueryDailyStats(db, userID, date)
		result = append(result, s)
	}
	return result
}

// AssetOverview aggregates the user's digital assets.
type AssetOverview struct {
	Knowledge  KnowledgeAssets `json:"knowledge"`
	Creations  CreationAssets  `json:"creations"`
	Node       NodeAssets      `json:"node"`
	AgentCount int64           `json:"agent_count"`
}

type KnowledgeAssets struct {
	Memories       int64 `json:"memories"`
	Documents      int64 `json:"documents"`
	KnowledgeBases int64 `json:"knowledge_bases"`
}

type CreationAssets struct {
	AgentsPublished int64   `json:"agents_published"`
	TotalDownloads  int64   `json:"total_downloads"`
	TotalRevenue    int64   `json:"total_revenue_cents"`
	AvgRating       float64 `json:"avg_rating"`
}

type NodeAssets struct {
	ClawID     string `json:"claw_id"`
	OnlineDays int    `json:"online_days"`
}

// BuildAssetOverview aggregates all local asset data.
func BuildAssetOverview(db *gorm.DB, userID, clawID string, onlineDays int) *AssetOverview {
	overview := &AssetOverview{}

	// Knowledge
	db.Model(&model.Memory{}).Where("user_id = ?", userID).Count(&overview.Knowledge.Memories)
	db.Model(&model.KnowledgeBase{}).Where("user_id = ? AND deleted_at IS NULL", userID).Count(&overview.Knowledge.KnowledgeBases)
	var docCount int64
	db.Model(&model.KnowledgeBase{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Select("COALESCE(SUM(document_count), 0)").
		Scan(&docCount)
	overview.Knowledge.Documents = docCount

	// Creations (marketplace listings)
	db.Model(&model.AgentListing{}).
		Where("creator_id = ? AND status = ?", userID, "published").
		Count(&overview.Creations.AgentsPublished)

	type SalesAgg struct {
		Downloads int64   `json:"downloads"`
		Revenue   int64   `json:"revenue"`
		AvgRating float64 `json:"avg_rating"`
	}
	var agg SalesAgg
	db.Model(&model.AgentListing{}).
		Where("creator_id = ? AND status = ?", userID, "published").
		Select("COALESCE(SUM(sales_count), 0) as downloads, COALESCE(SUM(revenue), 0) as revenue").
		Scan(&agg)
	overview.Creations.TotalDownloads = agg.Downloads
	overview.Creations.TotalRevenue = agg.Revenue

	// Average rating (from agent_templates if available)
	var avgRating float64
	db.Model(&model.AgentTemplate{}).
		Joins("JOIN agent_listings ON agent_listings.template_id = agent_templates.id").
		Where("agent_listings.creator_id = ? AND agent_listings.status = ?", userID, "published").
		Select("COALESCE(AVG(agent_templates.rating), 0)").
		Scan(&avgRating)
	overview.Creations.AvgRating = avgRating

	// Node
	overview.Node.ClawID = clawID
	overview.Node.OnlineDays = onlineDays

	// Agent count
	db.Model(&model.Agent{}).Where("user_id = ? AND deleted_at IS NULL", userID).Count(&overview.AgentCount)

	return overview
}
