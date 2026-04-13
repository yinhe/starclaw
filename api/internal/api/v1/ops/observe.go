package ops

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/internal/observe"
	"gorm.io/gorm"
)

// ObserveHandler exposes observability endpoints: traces, alerts, logs.
type ObserveHandler struct {
	engine *observe.Engine
	db     *gorm.DB
}

// NewObserveHandler creates the observe handler.
func NewObserveHandler(engine *observe.Engine, db *gorm.DB) *ObserveHandler {
	return &ObserveHandler{engine: engine, db: db}
}

// ════════════════════════════════════════════════════════════════
//  Traces
// ════════════════════════════════════════════════════════════════

// GetTrace returns all spans for a given trace ID.
func (h *ObserveHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("trace_id")
	spans, err := h.engine.GetTrace(traceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"trace_id": traceID, "spans": spans, "count": len(spans)})
}

// QuerySpans searches spans with filters.
func (h *ObserveHandler) QuerySpans(c *gin.Context) {
	userID := c.GetString("user_id")
	agentID := c.Query("agent_id")
	kind := observe.SpanKind(c.Query("kind"))
	status := c.Query("status")
	minDurationMs, _ := strconv.ParseInt(c.Query("min_duration_ms"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	sinceStr := c.Query("since")

	var since time.Time
	if sinceStr != "" {
		since, _ = time.Parse(time.RFC3339, sinceStr)
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	spans, err := h.engine.QuerySpans(userID, agentID, kind, minDurationMs, status, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"spans": spans, "count": len(spans)})
}

// ════════════════════════════════════════════════════════════════
//  Alert Rules
// ════════════════════════════════════════════════════════════════

// ListAlertRules returns all alert rules for the current user.
func (h *ObserveHandler) ListAlertRules(c *gin.Context) {
	userID := c.GetString("user_id")
	var rules []observe.AlertRule
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&rules)
	c.JSON(http.StatusOK, rules)
}

// CreateAlertRule creates a new alert rule.
func (h *ObserveHandler) CreateAlertRule(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description string  `json:"description"`
		Metric      string  `json:"metric" binding:"required"`
		Operator    string  `json:"operator" binding:"required"`
		Threshold   float64 `json:"threshold"`
		WindowSec   int     `json:"window_sec"`
		Severity    string  `json:"severity"`
		Actions     string  `json:"actions"`
		CooldownSec int     `json:"cooldown_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := observe.AlertRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Metric:      req.Metric,
		Operator:    req.Operator,
		Threshold:   req.Threshold,
		WindowSec:   req.WindowSec,
		Severity:    observe.AlertSeverity(req.Severity),
		Enabled:     true,
		Actions:     req.Actions,
		UserID:      userID,
		CooldownSec: req.CooldownSec,
	}
	if rule.WindowSec == 0 {
		rule.WindowSec = 300
	}
	if rule.Severity == "" {
		rule.Severity = observe.AlertWarning
	}
	if rule.CooldownSec == 0 {
		rule.CooldownSec = 3600
	}
	if rule.Actions == "" {
		rule.Actions = "[]"
	}

	if err := h.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create alert rule"})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

// UpdateAlertRule updates an existing alert rule.
func (h *ObserveHandler) UpdateAlertRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var rule observe.AlertRule
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var req struct {
		Name        *string  `json:"name"`
		Description *string  `json:"description"`
		Metric      *string  `json:"metric"`
		Operator    *string  `json:"operator"`
		Threshold   *float64 `json:"threshold"`
		WindowSec   *int     `json:"window_sec"`
		Severity    *string  `json:"severity"`
		Actions     *string  `json:"actions"`
		CooldownSec *int     `json:"cooldown_sec"`
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
	if req.Metric != nil {
		rule.Metric = *req.Metric
	}
	if req.Operator != nil {
		rule.Operator = *req.Operator
	}
	if req.Threshold != nil {
		rule.Threshold = *req.Threshold
	}
	if req.WindowSec != nil {
		rule.WindowSec = *req.WindowSec
	}
	if req.Severity != nil {
		rule.Severity = observe.AlertSeverity(*req.Severity)
	}
	if req.Actions != nil {
		rule.Actions = *req.Actions
	}
	if req.CooldownSec != nil {
		rule.CooldownSec = *req.CooldownSec
	}

	h.db.Save(&rule)
	c.JSON(http.StatusOK, rule)
}

// ToggleAlertRule enables or disables an alert rule.
func (h *ObserveHandler) ToggleAlertRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")

	var rule observe.AlertRule
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&rule).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	rule.Enabled = !rule.Enabled
	h.db.Save(&rule)
	c.JSON(http.StatusOK, gin.H{"enabled": rule.Enabled})
}

// DeleteAlertRule deletes an alert rule.
func (h *ObserveHandler) DeleteAlertRule(c *gin.Context) {
	userID := c.GetString("user_id")
	id := c.Param("id")
	result := h.db.Where("id = ? AND user_id = ?", id, userID).Delete(&observe.AlertRule{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ════════════════════════════════════════════════════════════════
//  Alert History
// ════════════════════════════════════════════════════════════════

// ListAlertHistory returns recent alert firings.
func (h *ObserveHandler) ListAlertHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	severity := c.Query("severity")
	resolved := c.Query("resolved")

	if page < 1 {
		page = 1
	}

	q := h.db.Model(&observe.AlertHistory{})
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if resolved == "true" {
		q = q.Where("resolved = ?", true)
	} else if resolved == "false" {
		q = q.Where("resolved = ?", false)
	}

	var total int64
	q.Count(&total)

	var history []observe.AlertHistory
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&history)

	c.JSON(http.StatusOK, gin.H{
		"items":     history,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ResolveAlert marks an alert as resolved.
func (h *ObserveHandler) ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	now := time.Now()
	result := h.db.Model(&observe.AlertHistory{}).Where("id = ? AND resolved = ?", id, false).
		Updates(map[string]interface{}{"resolved": true, "resolved_at": now})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found or already resolved"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"resolved": true})
}

// ════════════════════════════════════════════════════════════════
//  Logs
// ════════════════════════════════════════════════════════════════

// QueryLogs searches structured log entries.
func (h *ObserveHandler) QueryLogs(c *gin.Context) {
	userID := c.GetString("user_id")
	traceID := c.Query("trace_id")
	spanID := c.Query("span_id")
	agentID := c.Query("agent_id")
	level := observe.LogLevel(c.Query("level"))
	source := c.Query("source")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	sinceStr := c.Query("since")

	var since time.Time
	if sinceStr != "" {
		since, _ = time.Parse(time.RFC3339, sinceStr)
	} else {
		since = time.Now().Add(-24 * time.Hour)
	}

	entries, err := h.engine.QueryLogs(traceID, spanID, agentID, userID, level, source, since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "count": len(entries)})
}

// ════════════════════════════════════════════════════════════════
//  Overview Stats
// ════════════════════════════════════════════════════════════════

// ObserveStats returns observability overview.
func (h *ObserveHandler) ObserveStats(c *gin.Context) {
	stats := h.engine.Stats()
	c.JSON(http.StatusOK, stats)
}
