package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type ComplianceHandler struct {
	DB *gorm.DB
}

func NewComplianceHandler(db *gorm.DB) *ComplianceHandler {
	return &ComplianceHandler{DB: db}
}

// --- Compliance Logs ---

// ListComplianceLogs returns compliance events with filters
func (h *ComplianceHandler) ListComplianceLogs(c *gin.Context) {
	q := h.DB.Model(&model.ComplianceLog{})

	if t := c.Query("event_type"); t != "" {
		q = q.Where("event_type = ?", t)
	}
	if s := c.Query("severity"); s != "" {
		q = q.Where("severity = ?", s)
	}
	if team := c.Query("team_id"); team != "" {
		q = q.Where("team_id = ?", team)
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}
	resolved := c.Query("resolved")
	if resolved == "true" {
		q = q.Where("resolved = ?", true)
	} else if resolved == "false" {
		q = q.Where("resolved = ?", false)
	}

	var total int64
	q.Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size > 200 {
		size = 200
	}

	var logs []model.ComplianceLog
	q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&logs)

	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total, "page": page, "size": size})
}

// CreateComplianceLog records a new compliance event
func (h *ComplianceHandler) CreateComplianceLog(c *gin.Context) {
	var req model.ComplianceLog
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Actor == "" {
		req.Actor = middleware.GetAdminActor(c)
	}
	if req.IPAddress == "" {
		req.IPAddress = c.ClientIP()
	}

	if err := h.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建合规日志失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"log": req})
}

// ResolveComplianceLog marks a compliance event as resolved
func (h *ComplianceHandler) ResolveComplianceLog(c *gin.Context) {
	id := c.Param("id")
	var log model.ComplianceLog
	if err := h.DB.First(&log, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "合规日志不存在"})
		return
	}
	h.DB.Model(&log).Update("resolved", true)
	c.JSON(http.StatusOK, gin.H{"message": "已标记为已处理"})
}

// ComplianceStats returns aggregated compliance statistics
func (h *ComplianceHandler) ComplianceStats(c *gin.Context) {
	var totalLogs int64
	var unresolvedLogs int64
	var criticalLogs int64

	h.DB.Model(&model.ComplianceLog{}).Count(&totalLogs)
	h.DB.Model(&model.ComplianceLog{}).Where("resolved = ?", false).Count(&unresolvedLogs)
	h.DB.Model(&model.ComplianceLog{}).Where("severity = ? AND resolved = ?", "critical", false).Count(&criticalLogs)

	// Event type breakdown
	type TypeCount struct {
		EventType string `json:"event_type"`
		Count     int64  `json:"count"`
	}
	var typeBreakdown []TypeCount
	h.DB.Model(&model.ComplianceLog{}).
		Select("event_type, COUNT(*) as count").
		Group("event_type").
		Order("count DESC").
		Find(&typeBreakdown)

	// Severity breakdown
	type SevCount struct {
		Severity string `json:"severity"`
		Count    int64  `json:"count"`
	}
	var sevBreakdown []SevCount
	h.DB.Model(&model.ComplianceLog{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Find(&sevBreakdown)

	// Last 7 days daily counts
	type DayCount struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	var dailyCounts []DayCount
	h.DB.Model(&model.ComplianceLog{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).
		Group("DATE(created_at)").
		Order("date ASC").
		Find(&dailyCounts)

	c.JSON(http.StatusOK, gin.H{
		"total":          totalLogs,
		"unresolved":     unresolvedLogs,
		"critical":       criticalLogs,
		"by_type":        typeBreakdown,
		"by_severity":    sevBreakdown,
		"daily_7d":       dailyCounts,
	})
}

// ExportComplianceLogs exports filtered logs as JSON (for audit export)
func (h *ComplianceHandler) ExportComplianceLogs(c *gin.Context) {
	actor := middleware.GetAdminActor(c)

	q := h.DB.Model(&model.ComplianceLog{})
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			q = q.Where("created_at < ?", t.AddDate(0, 0, 1))
		}
	}

	var logs []model.ComplianceLog
	q.Order("created_at DESC").Limit(10000).Find(&logs)

	// Record this export as a compliance event itself
	h.DB.Create(&model.ComplianceLog{
		Actor:     actor,
		EventType: "audit_export",
		Severity:  "info",
		Resource:  "compliance_logs",
		Detail:    "Exported compliance logs",
		IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"logs": logs, "exported_by": actor, "exported_at": time.Now()})
}

// --- Sensitive Word Rules ---

// ListSensitiveWords returns all sensitive word rules
func (h *ComplianceHandler) ListSensitiveWords(c *gin.Context) {
	var rules []model.SensitiveWordRule
	q := h.DB.Model(&model.SensitiveWordRule{})

	if cat := c.Query("category"); cat != "" {
		q = q.Where("category = ?", cat)
	}
	if enabled := c.Query("enabled"); enabled != "" {
		q = q.Where("enabled = ?", enabled == "true")
	}

	q.Order("category ASC, word ASC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"rules": rules, "total": len(rules)})
}

// CreateSensitiveWord adds a new sensitive word rule
func (h *ComplianceHandler) CreateSensitiveWord(c *gin.Context) {
	var req model.SensitiveWordRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Word == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "敏感词不能为空"})
		return
	}
	if req.Action == "" {
		req.Action = "log"
	}

	if err := h.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建敏感词规则失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": req})
}

// UpdateSensitiveWord updates a sensitive word rule
func (h *ComplianceHandler) UpdateSensitiveWord(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Word     string `json:"word"`
		Category string `json:"category"`
		Action   string `json:"action"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var rule model.SensitiveWordRule
	if err := h.DB.First(&rule, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	updates := map[string]interface{}{}
	if req.Word != "" {
		updates["word"] = req.Word
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Action != "" {
		updates["action"] = req.Action
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	h.DB.Model(&rule).Updates(updates)
	h.DB.First(&rule, "id = ?", id)
	c.JSON(http.StatusOK, gin.H{"rule": rule})
}

// DeleteSensitiveWord removes a sensitive word rule
func (h *ComplianceHandler) DeleteSensitiveWord(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Delete(&model.SensitiveWordRule{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// --- Data Flow Records ---

// ListDataFlows returns all data flow records
func (h *ComplianceHandler) ListDataFlows(c *gin.Context) {
	var flows []model.DataFlowRecord
	h.DB.Order("source ASC, destination ASC").Find(&flows)
	c.JSON(http.StatusOK, gin.H{"flows": flows, "total": len(flows)})
}

// CreateDataFlow adds a new data flow record
func (h *ComplianceHandler) CreateDataFlow(c *gin.Context) {
	var req model.DataFlowRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建数据流记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"flow": req})
}

// UpdateDataFlow updates a data flow record
func (h *ComplianceHandler) UpdateDataFlow(c *gin.Context) {
	id := c.Param("id")
	var req model.DataFlowRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var flow model.DataFlowRecord
	if err := h.DB.First(&flow, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据流记录不存在"})
		return
	}

	updates := map[string]interface{}{}
	if req.Source != "" {
		updates["source"] = req.Source
	}
	if req.Destination != "" {
		updates["destination"] = req.Destination
	}
	if req.DataType != "" {
		updates["data_type"] = req.DataType
	}
	if req.Encryption != "" {
		updates["encryption"] = req.Encryption
	}
	if req.Region != "" {
		updates["region"] = req.Region
	}
	updates["cross_border"] = req.CrossBorder
	if req.Description != "" {
		updates["description"] = req.Description
	}

	h.DB.Model(&flow).Updates(updates)
	h.DB.First(&flow, "id = ?", id)
	c.JSON(http.StatusOK, gin.H{"flow": flow})
}

// DeleteDataFlow removes a data flow record
func (h *ComplianceHandler) DeleteDataFlow(c *gin.Context) {
	id := c.Param("id")
	if err := h.DB.Delete(&model.DataFlowRecord{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据流记录不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
