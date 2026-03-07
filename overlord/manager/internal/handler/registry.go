package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/manager/internal/model"
	"gorm.io/gorm"
)

type RegistryHandler struct {
	db *gorm.DB
}

func NewRegistryHandler(db *gorm.DB) *RegistryHandler {
	return &RegistryHandler{db: db}
}

// ---------- POST /brood/register — Claw registers with this Overlord ----------

type RegisterRequest struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"`
	Version string `json:"version"`
	Team    string `json:"team"`
	Tags    string `json:"tags"`
}

func (h *RegistryHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token := generateToken(32)
	node := model.ClawNode{
		Name:          req.Name,
		Address:       req.Address,
		Version:       req.Version,
		Status:        "online",
		Token:         token,
		Team:          req.Team,
		Tags:          req.Tags,
		LastHeartbeat: time.Now(),
	}

	if err := h.db.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register claw"})
		return
	}

	h.audit(c, "register", node.ID, "Claw registered: "+req.Name)

	c.JSON(http.StatusCreated, gin.H{
		"claw_id": node.ID,
		"token":   token,
		"message": "claw registered to brood",
	})
}

// ---------- POST /brood/heartbeat ----------

type HeartbeatRequest struct {
	ClawID       string  `json:"claw_id" binding:"required"`
	Token        string  `json:"token" binding:"required"`
	Version      string  `json:"version"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	TasksRunning int     `json:"tasks_running"`
	TasksQueued  int     `json:"tasks_queued"`
	TokensToday  int64   `json:"tokens_today"`
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs int     `json:"avg_latency_ms"`
}

func (h *RegistryHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var node model.ClawNode
	if err := h.db.Where("id = ? AND token = ?", req.ClawID, req.Token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claw_id or token"})
		return
	}

	updates := map[string]interface{}{
		"status":         "online",
		"last_heartbeat": time.Now(),
		"cpu_percent":    req.CPUPercent,
		"mem_percent":    req.MemPercent,
		"tasks_running":  req.TasksRunning,
		"tasks_queued":   req.TasksQueued,
		"tokens_today":   req.TokensToday,
		"error_rate":     req.ErrorRate,
		"avg_latency_ms": req.AvgLatencyMs,
	}
	if req.Version != "" {
		updates["version"] = req.Version
	}

	h.db.Model(&node).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ---------- GET /brood/claws ----------

func (h *RegistryHandler) ListClaws(c *gin.Context) {
	team := c.Query("team")
	status := c.Query("status")

	q := h.db.Order("last_heartbeat DESC")
	if team != "" {
		q = q.Where("team = ?", team)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var claws []model.ClawNode
	q.Find(&claws)
	c.JSON(http.StatusOK, gin.H{"claws": claws, "total": len(claws)})
}

// ---------- GET /brood/claws/:id ----------

func (h *RegistryHandler) GetClaw(c *gin.Context) {
	id := c.Param("id")
	var claw model.ClawNode
	if err := h.db.First(&claw, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "claw not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"claw": claw})
}

// ---------- PUT /brood/claws/:id/quota ----------

func (h *RegistryHandler) UpdateQuota(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		MaxConcurrent int   `json:"max_concurrent"`
		MaxTokensDay  int64 `json:"max_tokens_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Model(&model.ClawNode{}).Where("id = ?", id).Updates(map[string]interface{}{
		"max_concurrent": req.MaxConcurrent,
		"max_tokens_day": req.MaxTokensDay,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update quota"})
		return
	}

	h.audit(c, "update_quota", id, "quota updated")
	c.JSON(http.StatusOK, gin.H{"message": "quota updated"})
}

// ---------- DELETE /brood/claws/:id ----------

func (h *RegistryHandler) RemoveClaw(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.ClawNode{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove claw"})
		return
	}
	h.audit(c, "remove", id, "claw removed from brood")
	c.JSON(http.StatusOK, gin.H{"message": "claw removed"})
}

// ---------- GET /brood/stats ----------

func (h *RegistryHandler) Stats(c *gin.Context) {
	var total, online, feral, offline int64
	h.db.Model(&model.ClawNode{}).Count(&total)
	h.db.Model(&model.ClawNode{}).Where("status = ?", "online").Count(&online)
	h.db.Model(&model.ClawNode{}).Where("status = ?", "feral").Count(&feral)
	h.db.Model(&model.ClawNode{}).Where("status = ?", "offline").Count(&offline)

	type Agg struct {
		AvgCPU      float64 `gorm:"column:avg_cpu"`
		AvgMem      float64 `gorm:"column:avg_mem"`
		TotalTasks  int     `gorm:"column:total_tasks"`
		TotalTokens int64   `gorm:"column:total_tokens"`
	}
	var agg Agg
	h.db.Model(&model.ClawNode{}).Where("status = ?", "online").
		Select("AVG(cpu_percent) as avg_cpu, AVG(mem_percent) as avg_mem, SUM(tasks_running) as total_tasks, SUM(tokens_today) as total_tokens").
		Scan(&agg)

	// Team breakdown
	type TeamCount struct {
		Team  string `json:"team"`
		Count int    `json:"count"`
	}
	var teams []TeamCount
	h.db.Model(&model.ClawNode{}).
		Select("team, COUNT(*) as count").
		Where("team != ''").
		Group("team").Order("count DESC").Scan(&teams)

	c.JSON(http.StatusOK, gin.H{
		"total":          total,
		"online":         online,
		"feral":          feral,
		"offline":        offline,
		"avg_cpu":        agg.AvgCPU,
		"avg_mem":        agg.AvgMem,
		"total_tasks":    agg.TotalTasks,
		"total_tokens":   agg.TotalTokens,
		"teams":          teams,
	})
}

// ---------- GET /brood/audit ----------

func (h *RegistryHandler) AuditLogs(c *gin.Context) {
	var logs []model.AuditLog
	h.db.Order("created_at DESC").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": len(logs)})
}

// ---------- Scheduler: pick best Claw for a task ----------

func (h *RegistryHandler) AssignTask(c *gin.Context) {
	var req struct {
		TaskID string `json:"task_id" binding:"required"`
		Team   string `json:"team"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find least-loaded online claw (optionally filter by team)
	q := h.db.Where("status = ?", "online")
	if req.Team != "" {
		q = q.Where("team = ?", req.Team)
	}

	var best model.ClawNode
	if err := q.Order("tasks_running ASC, cpu_percent ASC").First(&best).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available claw"})
		return
	}

	// Check quota
	if best.MaxConcurrent > 0 && best.TasksRunning >= best.MaxConcurrent {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "all claws at capacity"})
		return
	}

	assignment := model.TaskAssignment{
		TaskID:     req.TaskID,
		ClawID:     best.ID,
		Status:     "assigned",
		AssignedAt: time.Now(),
	}
	h.db.Create(&assignment)

	h.audit(c, "assign_task", best.ID, "task "+req.TaskID+" assigned")

	c.JSON(http.StatusOK, gin.H{
		"claw_id":   best.ID,
		"claw_name": best.Name,
		"address":   best.Address,
		"task_id":   req.TaskID,
	})
}

// ---------- helpers ----------

func (h *RegistryHandler) audit(c *gin.Context, action, targetID, detail string) {
	actor := c.GetHeader("X-Admin-User")
	if actor == "" {
		actor = c.ClientIP()
	}
	h.db.Create(&model.AuditLog{
		Actor:    actor,
		Action:   action,
		TargetID: targetID,
		Detail:   detail,
	})
}

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
