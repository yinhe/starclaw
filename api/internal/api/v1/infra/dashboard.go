package infra

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

func (h *DashboardHandler) Stats(c *gin.Context) {
	userID := c.GetString("user_id")

	var agentCount, conversationCount, workflowCount, kbCount, mcpCount, docCount int64

	h.db.Model(&model.Agent{}).Where("user_id = ?", userID).Count(&agentCount)
	h.db.Model(&model.Conversation{}).Where("user_id = ?", userID).Count(&conversationCount)
	h.db.Model(&model.Workflow{}).Where("user_id = ?", userID).Count(&workflowCount)
	h.db.Model(&model.KnowledgeBase{}).Where("user_id = ?", userID).Count(&kbCount)
	h.db.Model(&model.MCPServer{}).Where("user_id = ?", userID).Count(&mcpCount)
	h.db.Model(&model.Document{}).Where("user_id = ?", userID).Count(&docCount)

	// Token usage (last 30 days)
	var totalTokens int64
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	h.db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at > ? AND messages.role = ?", userID, thirtyDaysAgo, "assistant").
		Select("COALESCE(SUM(messages.tokens_used), 0)").
		Scan(&totalTokens)

	// Recent conversations
	var recentConvos []model.Conversation
	h.db.Where("user_id = ?", userID).Order("updated_at DESC").Limit(5).Find(&recentConvos)

	// Message count today
	var todayMessages int64
	today := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&model.Message{}).
		Joins("JOIN conversations ON conversations.id = messages.conversation_id").
		Where("conversations.user_id = ? AND messages.created_at > ?", userID, today).
		Count(&todayMessages)

	// Per-agent token usage (top 10)
	type AgentUsage struct {
		AgentID   string `json:"agent_id"`
		AgentName string `json:"agent_name"`
		Tokens    int64  `json:"tokens"`
		MsgCount  int64  `json:"msg_count"`
	}
	var agentUsages []AgentUsage
	h.db.Raw(`
		SELECT a.id as agent_id, a.name as agent_name,
			COALESCE(SUM(m.tokens_used), 0) as tokens,
			COUNT(m.id) as msg_count
		FROM agents a
		LEFT JOIN conversations c ON c.agent_id = a.id AND c.user_id = ?
		LEFT JOIN messages m ON m.conversation_id = c.id AND m.role = 'assistant'
		WHERE a.user_id = ?
		GROUP BY a.id, a.name
		ORDER BY tokens DESC
		LIMIT 10
	`, userID, userID).Scan(&agentUsages)

	// Daily token usage (last 7 days)
	type DailyUsage struct {
		Date   string `json:"date"`
		Tokens int64  `json:"tokens"`
		Msgs   int64  `json:"msgs"`
	}
	var dailyUsage []DailyUsage
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	h.db.Raw(`
		SELECT DATE(m.created_at) as date,
			COALESCE(SUM(m.tokens_used), 0) as tokens,
			COUNT(m.id) as msgs
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE c.user_id = ? AND m.created_at > ? AND m.role = 'assistant'
		GROUP BY DATE(m.created_at)
		ORDER BY date
	`, userID, sevenDaysAgo).Scan(&dailyUsage)

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"agents":          agentCount,
			"conversations":   conversationCount,
			"workflows":       workflowCount,
			"knowledge_bases": kbCount,
			"mcp_servers":     mcpCount,
			"documents":       docCount,
			"tokens_30d":      totalTokens,
			"messages_today":  todayMessages,
		},
		"recent_conversations": recentConvos,
		"agent_usage":          agentUsages,
		"daily_usage":          dailyUsage,
	})
}
