package ops

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/webhook"
	"gorm.io/gorm"
)

// WebhookRuleHandler manages event rules and webhook orchestration.
type WebhookRuleHandler struct {
	engine *webhook.Engine
	db     *gorm.DB
}

// NewWebhookRuleHandler creates the handler.
func NewWebhookRuleHandler(engine *webhook.Engine, db *gorm.DB) *WebhookRuleHandler {
	return &WebhookRuleHandler{engine: engine, db: db}
}

// ════════════════════════════════════════════════════════════════
//  Event Rules CRUD
// ════════════════════════════════════════════════════════════════

// ListRules returns all rules for the current user.
func (h *WebhookRuleHandler) ListRules(c *gin.Context) {
	userID := c.GetString("user_id")
	var rules []webhook.EventRule
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// CreateRule creates a new event rule.
func (h *WebhookRuleHandler) CreateRule(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		EventType   string `json:"event_type" binding:"required"`
		Condition   string `json:"condition"`
		Actions     string `json:"actions" binding:"required"`
		RetryCount  int    `json:"retry_count"`
		RetryDelay  int    `json:"retry_delay"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := webhook.EventRule{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		EventType:   req.EventType,
		Condition:   req.Condition,
		Actions:     req.Actions,
		Enabled:     true,
		RetryCount:  req.RetryCount,
		RetryDelay:  req.RetryDelay,
	}
	if rule.RetryCount == 0 {
		rule.RetryCount = 3
	}
	if rule.RetryDelay == 0 {
		rule.RetryDelay = 60
	}
	if rule.Condition == "" {
		rule.Condition = "{}"
	}

	if err := h.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// UpdateRule updates an existing rule.
func (h *WebhookRuleHandler) UpdateRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var rule webhook.EventRule
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		EventType   *string `json:"event_type"`
		Condition   *string `json:"condition"`
		Actions     *string `json:"actions"`
		RetryCount  *int    `json:"retry_count"`
		RetryDelay  *int    `json:"retry_delay"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.EventType != nil {
		rule.EventType = *req.EventType
	}
	if req.Condition != nil {
		rule.Condition = *req.Condition
	}
	if req.Actions != nil {
		rule.Actions = *req.Actions
	}
	if req.RetryCount != nil {
		rule.RetryCount = *req.RetryCount
	}
	if req.RetryDelay != nil {
		rule.RetryDelay = *req.RetryDelay
	}

	h.db.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// ToggleRule enables or disables a rule.
func (h *WebhookRuleHandler) ToggleRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var rule webhook.EventRule
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	rule.Enabled = !rule.Enabled
	h.db.Save(&rule)
	c.JSON(http.StatusOK, gin.H{"enabled": rule.Enabled})
}

// DeleteRule deletes a rule.
func (h *WebhookRuleHandler) DeleteRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&webhook.EventRule{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ════════════════════════════════════════════════════════════════
//  Event Logs
// ════════════════════════════════════════════════════════════════

// ListLogs returns paginated event logs.
func (h *WebhookRuleHandler) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	status := c.Query("status")
	eventType := c.Query("event_type")
	ruleID := c.Query("rule_id")

	if page < 1 {
		page = 1
	}

	q := h.db.Model(&webhook.EventLog{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if eventType != "" {
		q = q.Where("event_type = ?", eventType)
	}
	if ruleID != "" {
		q = q.Where("rule_id = ?", ruleID)
	}

	var total int64
	q.Count(&total)

	var logs []webhook.EventLog
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// RetryDeadLetter retries a dead-lettered event.
func (h *WebhookRuleHandler) RetryDeadLetter(c *gin.Context) {
	id := c.Param("id")
	result := h.db.Model(&webhook.EventLog{}).Where("id = ? AND status = ?", id, "dead_letter").
		Updates(map[string]interface{}{"status": "retrying", "attempts": 0})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "dead letter not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "retrying"})
}

// ════════════════════════════════════════════════════════════════
//  Stats + Test
// ════════════════════════════════════════════════════════════════

// Stats returns webhook engine stats.
func (h *WebhookRuleHandler) Stats(c *gin.Context) {
	stats := h.engine.Stats()
	c.JSON(http.StatusOK, stats)
}

// TestRule fires a test event to see if a rule would match.
func (h *WebhookRuleHandler) TestRule(c *gin.Context) {
	var req struct {
		EventType string                 `json:"event_type" binding:"required"`
		Data      map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := webhook.Event{
		Type:   req.EventType,
		Source: "test",
		Data:   req.Data,
	}

	// Emit via engine (will be processed asynchronously)
	h.engine.Emit(event)

	c.JSON(http.StatusOK, gin.H{"message": "test event emitted", "event_type": req.EventType})
}

// EventTypes returns all supported event types.
func (h *WebhookRuleHandler) EventTypes(c *gin.Context) {
	types := []map[string]string{
		{"type": "agent.error", "description": "Agent encountered an error during execution"},
		{"type": "agent.complete", "description": "Agent completed a task successfully"},
		{"type": "chat.message", "description": "New chat message received or sent"},
		{"type": "workflow.fail", "description": "Workflow execution failed"},
		{"type": "workflow.complete", "description": "Workflow execution completed"},
		{"type": "alert.fired", "description": "An observability alert rule fired"},
		{"type": "system.health", "description": "System health check event"},
		{"type": "marketplace.purchase", "description": "Agent was purchased on marketplace"},
		{"type": "user.login", "description": "User logged in"},
		{"type": "node.offline", "description": "A peer node went offline"},
	}
	c.JSON(http.StatusOK, types)
}
