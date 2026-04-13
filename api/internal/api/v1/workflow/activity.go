package workflow

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/instinct"
	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

type ActivityHandler struct {
	db     *gorm.DB
	engine *instinct.Engine
}

func NewActivityHandler(db *gorm.DB, engine *instinct.Engine) *ActivityHandler {
	return &ActivityHandler{db: db, engine: engine}
}

// List returns all activities for the current user
func (h *ActivityHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	actType := c.Query("type") // optional filter by type

	query := h.db.Where("user_id = ?", userID)
	if actType != "" {
		query = query.Where("type = ?", actType)
	}

	var activities []model.Activity
	query.Order("type ASC, created_at ASC").Find(&activities)

	c.JSON(http.StatusOK, gin.H{"activities": activities, "total": len(activities)})
}

// Get returns a single activity
func (h *ActivityHandler) Get(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var act model.Activity
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&act).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "activity not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"activity": act})
}

// Create creates a custom activity
func (h *ActivityHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		AgentID     string `json:"agent_id"`
		Name        string `json:"name" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Type        string `json:"type" binding:"required"`
		Trigger     string `json:"trigger" binding:"required"`
		Condition   string `json:"condition"`
		Action      string `json:"action" binding:"required"`
		Channel     string `json:"channel"`
		Cooldown    string `json:"cooldown"`
		Config      string `json:"config"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cooldown := req.Cooldown
	if cooldown == "" {
		cooldown = "24h"
	}

	act := model.Activity{
		UserID:      userID,
		AgentID:     req.AgentID,
		Name:        req.Name,
		Title:       req.Title,
		Description: req.Description,
		Type:        model.ActivityType(req.Type),
		Trigger:     req.Trigger,
		Condition:   req.Condition,
		Action:      req.Action,
		Channel:     req.Channel,
		Cooldown:    cooldown,
		Config:      req.Config,
		Enabled:     enabled,
	}

	if err := h.db.Create(&act).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create activity"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"activity": act})
}

// Update updates an activity
func (h *ActivityHandler) Update(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var act model.Activity
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&act).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "activity not found"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Whitelist updatable fields
	allowed := map[string]bool{
		"title": true, "description": true, "trigger": true,
		"condition": true, "action": true, "channel": true,
		"cooldown": true, "config": true, "enabled": true,
		"agent_id": true,
	}
	updates := make(map[string]interface{})
	for k, v := range req {
		if allowed[k] {
			updates[k] = v
		}
	}

	if len(updates) > 0 {
		h.db.Model(&act).Updates(updates)
	}

	h.db.Where("id = ?", id).First(&act) // reload
	c.JSON(http.StatusOK, gin.H{"activity": act})
}

// Toggle enables/disables an activity
func (h *ActivityHandler) Toggle(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var act model.Activity
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&act).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "activity not found"})
		return
	}

	newEnabled := !act.Enabled
	h.db.Model(&act).Update("enabled", newEnabled)
	c.JSON(http.StatusOK, gin.H{"id": act.ID, "enabled": newEnabled})
}

// Delete removes an activity
func (h *ActivityHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Activity{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "activity not found"})
		return
	}
	// Also delete logs
	h.db.Where("activity_id = ?", id).Delete(&model.ActivityLog{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Logs returns execution history for an activity
func (h *ActivityHandler) Logs(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	// Verify ownership
	var act model.Activity
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&act).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "activity not found"})
		return
	}

	var logs []model.ActivityLog
	h.db.Where("activity_id = ?", id).Order("created_at DESC").Limit(50).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": len(logs)})
}

// Templates returns the list of available built-in templates
func (h *ActivityHandler) Templates(c *gin.Context) {
	templates := instinct.BuiltinTemplates()
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// Seed creates all built-in activities for the current user (disabled by default)
func (h *ActivityHandler) Seed(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")

	instinct.SeedBuiltinActivities(h.db, userID, agentID)
	c.JSON(http.StatusOK, gin.H{"message": "built-in activities seeded"})
}

// FireEvent manually fires an event (for testing or external webhook integration)
func (h *ActivityHandler) FireEvent(c *gin.Context) {
	eventType := c.Param("event")
	if eventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event type required"})
		return
	}
	if h.engine != nil {
		h.engine.FireEvent(eventType)
	}
	c.JSON(http.StatusOK, gin.H{"message": "event fired", "event": eventType})
}
