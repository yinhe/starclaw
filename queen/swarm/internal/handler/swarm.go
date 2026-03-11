package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/swarm/internal/model"
	"gorm.io/gorm"
)

type SwarmHandler struct {
	db *gorm.DB
}

func NewSwarmHandler(db *gorm.DB) *SwarmHandler {
	return &SwarmHandler{db: db}
}

// ---------- POST /swarm/register ----------

type RegisterRequest struct {
	Name         string         `json:"name" binding:"required"`
	Role         model.NodeRole `json:"role" binding:"required"`
	Version      string         `json:"version"`
	Address      string         `json:"address" binding:"required"`
	Region       string         `json:"region"`
	ClawID       string         `json:"claw_id"`
	OverlordID   string         `json:"overlord_id"`
	Capabilities string         `json:"capabilities"`
}

func (h *SwarmHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate role
	if req.Role != model.RoleClaw && req.Role != model.RoleOverlord {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be 'claw' or 'overlord'"})
		return
	}

	// Check if this claw_id is already registered (upsert)
	if req.ClawID != "" {
		var existing model.Node
		if h.db.Where("claw_id = ?", req.ClawID).First(&existing).Error == nil {
			// Update existing registration
			updates := map[string]interface{}{
				"name":           req.Name,
				"status":         model.StatusOnline,
				"version":        req.Version,
				"address":        req.Address,
				"region":         req.Region,
				"last_heartbeat": time.Now(),
			}
			if req.Capabilities != "" {
				updates["capabilities"] = req.Capabilities
			}
			h.db.Model(&existing).Updates(updates)

			c.JSON(http.StatusOK, gin.H{
				"node_id": existing.ID,
				"token":   existing.Token,
				"message": "node re-registered (updated)",
			})
			return
		}
	}

	// Generate registration token for new node
	token := generateToken(32)

	node := model.Node{
		Name:          req.Name,
		Role:          req.Role,
		Status:        model.StatusOnline,
		Version:       req.Version,
		Address:       req.Address,
		Region:        req.Region,
		ClawID:        req.ClawID,
		OverlordID:    req.OverlordID,
		Token:         token,
		Capabilities:  req.Capabilities,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}

	if err := h.db.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register node"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"node_id": node.ID,
		"token":   token,
		"message": "node registered successfully",
	})
}

// ---------- POST /swarm/heartbeat ----------

type HeartbeatRequest struct {
	NodeID       string  `json:"node_id" binding:"required"`
	Token        string  `json:"token" binding:"required"`
	Version      string  `json:"version"`
	ClawID       string  `json:"claw_id"`
	Address      string  `json:"address"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemPercent   float64 `json:"mem_percent"`
	TasksRunning int     `json:"tasks_running"`
	TasksQueued  int     `json:"tasks_queued"`
	TokensUsed   int64   `json:"tokens_used_30d"`
	ErrorRate    float64 `json:"error_rate"`
}

func (h *SwarmHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var node model.Node
	if err := h.db.Where("id = ? AND token = ?", req.NodeID, req.Token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid node_id or token"})
		return
	}

	updates := map[string]interface{}{
		"status":          model.StatusOnline,
		"last_heartbeat":  time.Now(),
		"cpu_percent":     req.CPUPercent,
		"mem_percent":     req.MemPercent,
		"tasks_running":   req.TasksRunning,
		"tasks_queued":    req.TasksQueued,
		"tokens_used_30d": req.TokensUsed,
		"error_rate":      req.ErrorRate,
	}
	if req.Version != "" {
		updates["version"] = req.Version
	}
	if req.ClawID != "" {
		updates["claw_id"] = req.ClawID
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}

	h.db.Model(&node).Updates(updates)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "heartbeat received",
	})
}

// ---------- GET /swarm/config ----------

func (h *SwarmHandler) GetConfig(c *gin.Context) {
	nodeID := c.Query("node_id")
	token := c.Query("token")

	if nodeID == "" || token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id and token required"})
		return
	}

	var node model.Node
	if err := h.db.Where("id = ? AND token = ?", nodeID, token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid node_id or token"})
		return
	}

	// Determine latest version from MoltRelease (or fallback)
	latestVersion := "dev"
	var latestRelease model.MoltRelease
	if h.db.Where("status IN ?", []string{string(model.ReleaseRolling), string(model.ReleaseComplete)}).
		Order("created_at DESC").First(&latestRelease).Error == nil {
		latestVersion = latestRelease.Version
	}

	// Return swarm config
	cfg := model.SwarmConfig{
		Models: []model.ModelInfo{},
		Policies: map[string]any{
			"max_concurrent_tasks": 10,
			"heartbeat_interval":   "30s",
			"auto_update":          true,
		},
		Version:    latestVersion,
		VersionURL: latestRelease.VersionURL,
		UpdatedAt:  time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

// ---------- GET /swarm/nodes ----------

func (h *SwarmHandler) ListNodes(c *gin.Context) {
	role := c.Query("role")
	status := c.Query("status")

	q := h.db.Order("last_heartbeat DESC")
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var nodes []model.Node
	if err := q.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list nodes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "total": len(nodes)})
}

// ---------- GET /swarm/nodes/:id ----------

func (h *SwarmHandler) GetNode(c *gin.Context) {
	id := c.Param("id")
	var node model.Node
	if err := h.db.First(&node, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"node": node})
}

// ---------- DELETE /swarm/nodes/:id ----------

func (h *SwarmHandler) RemoveNode(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.Node{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove node"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "node removed"})
}

// ---------- POST /swarm/update/notify ----------

type UpdateNotifyRequest struct {
	Version    string `json:"version" binding:"required"`
	VersionURL string `json:"version_url"`
	Strategy   string `json:"strategy"` // "all", "grayscale", "manual"
	Percent    int    `json:"percent"`  // for grayscale (0-100)
}

func (h *SwarmHandler) NotifyUpdate(c *gin.Context) {
	var req UpdateNotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Count online nodes
	var total int64
	h.db.Model(&model.Node{}).Where("status = ?", model.StatusOnline).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"message":      "update notification queued",
		"version":      req.Version,
		"strategy":     req.Strategy,
		"target_nodes": total,
	})
}

// ---------- GET /swarm/stats ----------

func (h *SwarmHandler) Stats(c *gin.Context) {
	var totalNodes, onlineNodes, clawNodes, overlordNodes int64

	h.db.Model(&model.Node{}).Count(&totalNodes)
	h.db.Model(&model.Node{}).Where("status = ?", model.StatusOnline).Count(&onlineNodes)
	h.db.Model(&model.Node{}).Where("role = ?", model.RoleClaw).Count(&clawNodes)
	h.db.Model(&model.Node{}).Where("role = ?", model.RoleOverlord).Count(&overlordNodes)

	// Aggregate metrics from online nodes
	type MetricsResult struct {
		AvgCPU      float64 `gorm:"column:avg_cpu"`
		AvgMem      float64 `gorm:"column:avg_mem"`
		TotalTasks  int     `gorm:"column:total_tasks"`
		TotalTokens int64   `gorm:"column:total_tokens"`
	}
	var metrics MetricsResult
	h.db.Model(&model.Node{}).Where("status = ?", model.StatusOnline).
		Select("AVG(cpu_percent) as avg_cpu, AVG(mem_percent) as avg_mem, SUM(tasks_running) as total_tasks, SUM(tokens_used_30d) as total_tokens").
		Scan(&metrics)

	// Version distribution
	type VersionCount struct {
		Version string `json:"version"`
		Count   int    `json:"count"`
	}
	var versions []VersionCount
	h.db.Model(&model.Node{}).
		Select("version, COUNT(*) as count").
		Group("version").
		Order("count DESC").
		Scan(&versions)

	c.JSON(http.StatusOK, gin.H{
		"total_nodes":          totalNodes,
		"online_nodes":         onlineNodes,
		"claw_nodes":           clawNodes,
		"overlord_nodes":       overlordNodes,
		"avg_cpu":              metrics.AvgCPU,
		"avg_mem":              metrics.AvgMem,
		"total_tasks_running":  metrics.TotalTasks,
		"total_tokens_30d":     metrics.TotalTokens,
		"version_distribution": versions,
	})
}

// ---------- GET /swarm/resolve — resolve claw: address to network address ----------

func (h *SwarmHandler) Resolve(c *gin.Context) {
	clawID := c.Query("claw_id")
	if clawID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"found": false, "error": "claw_id required"})
		return
	}

	var node model.Node
	if err := h.db.Where("claw_id = ? AND status != ?", clawID, model.StatusOffline).First(&node).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"found": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"found":   true,
		"address": node.Address,
		"name":    node.Name,
		"region":  node.Region,
		"claw_id": node.ClawID,
		"version": node.Version,
		"status":  node.Status,
	})
}

// ---------- helpers ----------

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
