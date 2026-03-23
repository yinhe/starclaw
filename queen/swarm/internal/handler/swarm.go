package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/swarm/internal/model"
	"gorm.io/gorm"
)

type SwarmHandler struct {
	db *gorm.DB
}

func NewSwarmHandler(db *gorm.DB) *SwarmHandler {
	return &SwarmHandler{db: db}
}

// fetchCreditBalance queries Queen API for a claw's star energy balance
func fetchCreditBalance(clawID string) map[string]interface{} {
	queenAPI := os.Getenv("QUEEN_API_URL")
	if queenAPI == "" {
		queenAPI = "http://queen-api:8085"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(queenAPI + "/v1/credits/balance?claw_id=" + clawID)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil
	}

	var result map[string]interface{}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return nil
	}

	// Extract data from APIResponse envelope
	if data, ok := result["data"].(map[string]interface{}); ok {
		return data
	}
	return nil
}

// grantWelcomeBonus calls Queen API to grant 100 ⚡ to a new claw node
func grantWelcomeBonus(clawID string) {
	queenAPI := os.Getenv("QUEEN_API_URL")
	if queenAPI == "" {
		queenAPI = "http://queen-api:8080"
	}
	secret := os.Getenv("QUEEN_JWT_SECRET")
	if secret == "" {
		return // can't auth, skip
	}

	body, _ := json.Marshal(map[string]interface{}{
		"claw_id": clawID,
		"amount":  100 * 10000, // 100 Stars × 10000 units/Star
		"type":    "grant",
		"remark":  "新手礼包 100 ⚡ Welcome bonus",
	})

	req, err := http.NewRequest("POST", queenAPI+"/internal/credits/grant", bytes.NewReader(body))
	if err != nil {
		log.Printf("[swarm] grant bonus request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", secret)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[swarm] grant bonus to %s failed: %v", clawID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[swarm] granted 100 ⚡ welcome bonus to %s", clawID)
	} else {
		log.Printf("[swarm] grant bonus to %s returned status %d", clawID, resp.StatusCode)
	}
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

	// Welcome bonus disabled — users recharge via star-ai.net
	// if req.ClawID != "" {
	// 	go grantWelcomeBonus(req.ClawID)
	// }

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
	// Compute contribution (mining)
	IsContributor     bool     `json:"is_contributor"`
	ContributorModels []string `json:"contributor_models"`
	GPUInfo           string   `json:"gpu_info"`
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

	// Update contributor status
	if req.IsContributor {
		updates["is_contributor"] = true
		if len(req.ContributorModels) > 0 {
			if modelsJSON, err := json.Marshal(req.ContributorModels); err == nil {
				updates["contributor_models"] = string(modelsJSON)
			}
		}
		if req.GPUInfo != "" {
			updates["gpu_info"] = req.GPUInfo
		}
		// Increment online minutes (heartbeat interval ~60s)
		updates["online_minutes_today"] = gorm.Expr("online_minutes_today + 1")
		updates["total_online_minutes"] = gorm.Expr("total_online_minutes + 1")
	} else {
		updates["is_contributor"] = false
	}

	h.db.Model(&node).Updates(updates)

	// Fetch credit balance from Queen API and include in response
	resp := gin.H{
		"status":  "ok",
		"message": "heartbeat received",
	}
	clawID := req.ClawID
	if clawID == "" {
		clawID = node.ClawID
	}
	if clawID != "" {
		if credits := fetchCreditBalance(clawID); credits != nil {
			resp["credits"] = credits
		}
	}

	// Inject molt update directive if a rolling release targets this node
	if req.Version != "" {
		molt := h.checkMoltForNode(node.ID, req.Version, string(node.Role))
		if molt != nil {
			resp["molt"] = molt
		}
	}

	c.JSON(http.StatusOK, resp)
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
	clawIDs := c.Query("claw_ids") // comma-separated claw_ids filter

	q := h.db.Order("last_heartbeat DESC")
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if clawIDs != "" {
		ids := strings.Split(clawIDs, ",")
		q = q.Where("claw_id IN ?", ids)
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

// checkMoltForNode checks if there's a pending molt update for this node.
// Returns nil if no update is needed. Reuses the same logic as MoltHandler.Check
// but avoids an extra HTTP call — the data is injected into heartbeat response.
func (h *SwarmHandler) checkMoltForNode(nodeID, currentVersion, role string) map[string]interface{} {
	if role == "" {
		role = "claw"
	}

	// Find latest rolling release for this role
	var release model.MoltRelease
	err := h.db.Where("target_role = ? AND status = ? AND version != ?",
		role, model.ReleaseRolling, currentVersion).
		Order("created_at DESC").First(&release).Error
	if err != nil {
		return nil
	}

	// Check if this node is in the rollout batch
	var ns model.MoltNodeStatus
	if h.db.Where("release_id = ? AND node_id = ?", release.ID, nodeID).First(&ns).Error != nil {
		return nil
	}

	if ns.Status != "pending" {
		return nil
	}

	// Mark as updating
	h.db.Model(&ns).Updates(map[string]interface{}{
		"status":     "updating",
		"updated_at": time.Now(),
	})

	return map[string]interface{}{
		"update_available": true,
		"release_id":       release.ID,
		"version":          release.Version,
		"version_url":      release.VersionURL,
		"changelog":        release.Changelog,
		"mandatory":        release.Mandatory,
	}
}

// ---------- GET /swarm/mining/stats ----------

func (h *SwarmHandler) MiningStats(c *gin.Context) {
	// Total contributors online now
	var onlineCount int64
	h.db.Model(&model.Node{}).Where("is_contributor = ? AND status = ?", true, model.StatusOnline).Count(&onlineCount)

	// Total contributors ever
	var totalContributors int64
	h.db.Model(&model.Node{}).Where("is_contributor = ?", true).Count(&totalContributors)

	// Aggregate stats
	var stats struct {
		TotalOnlineMinutes int64 `gorm:"column:total_minutes"`
		TotalEarnings      int64 `gorm:"column:total_earnings"`
	}
	h.db.Model(&model.Node{}).Where("is_contributor = ?", true).
		Select("COALESCE(SUM(total_online_minutes),0) as total_minutes, COALESCE(SUM(mining_earnings),0) as total_earnings").
		Scan(&stats)

	// Top contributors (by earnings)
	var top []model.Node
	h.db.Where("is_contributor = ? AND mining_earnings > 0", true).
		Order("mining_earnings DESC").Limit(20).Find(&top)

	topList := make([]gin.H, 0, len(top))
	for _, n := range top {
		topList = append(topList, gin.H{
			"node_id":              n.ID,
			"claw_id":              n.ClawID,
			"gpu_info":             n.GPUInfo,
			"total_online_minutes": n.TotalOnlineMinutes,
			"mining_earnings":      n.MiningEarnings,
			"status":               n.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"online_contributors":  onlineCount,
		"total_contributors":   totalContributors,
		"total_online_minutes": stats.TotalOnlineMinutes,
		"total_earnings":       stats.TotalEarnings,
		"top_contributors":     topList,
	})
}

// ---------- helpers ----------

func generateToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
