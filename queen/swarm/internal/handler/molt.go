package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/swarm/internal/model"
	"gorm.io/gorm"
)

type MoltHandler struct {
	db *gorm.DB
}

func NewMoltHandler(db *gorm.DB) *MoltHandler {
	return &MoltHandler{db: db}
}

// POST /swarm/molt/releases — create a new release
func (h *MoltHandler) CreateRelease(c *gin.Context) {
	var req struct {
		Version    string `json:"version" binding:"required"`
		VersionURL string `json:"version_url"`
		Changelog  string `json:"changelog"`
		Strategy   string `json:"strategy"`  // all, grayscale, manual
		Percent    int    `json:"percent"`    // for grayscale
		TargetRole string `json:"target_role"` // claw or overlord
		Mandatory  bool   `json:"mandatory"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if version already exists
	var existing model.MoltRelease
	if h.db.Where("version = ?", req.Version).First(&existing).Error == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "release already exists for this version", "release": existing})
		return
	}

	strategy := model.ReleaseStrategy(req.Strategy)
	if strategy == "" {
		strategy = model.StrategyAll
	}
	percent := req.Percent
	if percent <= 0 || percent > 100 {
		percent = 100
	}
	targetRole := model.NodeRole(req.TargetRole)
	if targetRole == "" {
		targetRole = model.RoleClaw
	}

	// Count target nodes
	var targetCount int64
	h.db.Model(&model.Node{}).Where("role = ? AND status = ?", targetRole, model.StatusOnline).Count(&targetCount)

	release := model.MoltRelease{
		Version:    req.Version,
		VersionURL: req.VersionURL,
		Changelog:  req.Changelog,
		Strategy:   strategy,
		Percent:    percent,
		Status:     model.ReleasePending,
		TargetRole: targetRole,
		Mandatory:  req.Mandatory,
		NodesTotal: int(targetCount),
	}

	if err := h.db.Create(&release).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create release"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"release": release})
}

// GET /swarm/molt/releases — list all releases
func (h *MoltHandler) ListReleases(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}

	var total int64
	h.db.Model(&model.MoltRelease{}).Count(&total)

	var releases []model.MoltRelease
	h.db.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&releases)

	c.JSON(http.StatusOK, gin.H{"releases": releases, "total": total, "page": page})
}

// GET /swarm/molt/releases/:id — get release detail with node statuses
func (h *MoltHandler) GetRelease(c *gin.Context) {
	id := c.Param("id")
	var release model.MoltRelease
	if err := h.db.First(&release, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	var nodeStatuses []model.MoltNodeStatus
	h.db.Where("release_id = ?", id).Order("updated_at DESC").Find(&nodeStatuses)

	c.JSON(http.StatusOK, gin.H{"release": release, "node_statuses": nodeStatuses})
}

// POST /swarm/molt/releases/:id/start — start rolling out
func (h *MoltHandler) StartRelease(c *gin.Context) {
	id := c.Param("id")
	var release model.MoltRelease
	if err := h.db.First(&release, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}
	if release.Status != model.ReleasePending && release.Status != model.ReleasePaused {
		c.JSON(http.StatusConflict, gin.H{"error": "release is not in pending/paused state"})
		return
	}

	now := time.Now()
	h.db.Model(&release).Updates(map[string]interface{}{
		"status":     model.ReleaseRolling,
		"started_at": &now,
	})

	// Create MoltNodeStatus entries for target nodes
	var nodes []model.Node
	q := h.db.Where("role = ? AND status = ?", release.TargetRole, model.StatusOnline)
	if release.Strategy == model.StrategyGrayscale && release.Percent < 100 {
		limit := int(float64(release.NodesTotal) * float64(release.Percent) / 100)
		if limit < 1 {
			limit = 1
		}
		q = q.Limit(limit)
	}
	q.Find(&nodes)

	for _, n := range nodes {
		if n.Version == release.Version {
			continue // already on target version
		}
		status := model.MoltNodeStatus{
			ReleaseID: release.ID,
			NodeID:    n.ID,
			NodeName:  n.Name,
			OldVer:    n.Version,
			Status:    "pending",
		}
		h.db.Create(&status)
	}

	h.db.Model(&release).Update("nodes_total", len(nodes))

	c.JSON(http.StatusOK, gin.H{"message": "release started", "target_nodes": len(nodes)})
}

// POST /swarm/molt/releases/:id/pause — pause rollout
func (h *MoltHandler) PauseRelease(c *gin.Context) {
	id := c.Param("id")
	result := h.db.Model(&model.MoltRelease{}).Where("id = ? AND status = ?", id, model.ReleaseRolling).
		Update("status", model.ReleasePaused)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "release not in rolling state"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "release paused"})
}

// POST /swarm/molt/report — node reports update result (called by Claw after self-update)
func (h *MoltHandler) Report(c *gin.Context) {
	var req struct {
		NodeID    string `json:"node_id" binding:"required"`
		ReleaseID string `json:"release_id" binding:"required"`
		Status    string `json:"status" binding:"required"` // ok, failed
		Error     string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update node status
	result := h.db.Model(&model.MoltNodeStatus{}).
		Where("release_id = ? AND node_id = ?", req.ReleaseID, req.NodeID).
		Updates(map[string]interface{}{
			"status":     req.Status,
			"error":      req.Error,
			"updated_at": time.Now(),
		})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node status not found"})
		return
	}

	// Update release counters
	var release model.MoltRelease
	if h.db.First(&release, "id = ?", req.ReleaseID).Error == nil {
		var okCount, failCount int64
		h.db.Model(&model.MoltNodeStatus{}).Where("release_id = ? AND status = ?", req.ReleaseID, "ok").Count(&okCount)
		h.db.Model(&model.MoltNodeStatus{}).Where("release_id = ? AND status = ?", req.ReleaseID, "failed").Count(&failCount)

		updates := map[string]interface{}{
			"nodes_ok":     okCount,
			"nodes_failed": failCount,
		}

		// Auto-complete if all nodes reported
		totalReported := okCount + failCount
		if int(totalReported) >= release.NodesTotal {
			now := time.Now()
			if failCount > 0 && float64(failCount)/float64(release.NodesTotal) > 0.3 {
				updates["status"] = model.ReleaseFailed
			} else {
				updates["status"] = model.ReleaseComplete
			}
			updates["completed_at"] = &now
		}

		// Circuit breaker: if >30% failed while rolling, auto-pause
		if release.Status == model.ReleaseRolling && release.NodesTotal > 0 {
			if float64(failCount)/float64(release.NodesTotal) > 0.3 {
				updates["status"] = model.ReleasePaused
			}
		}

		h.db.Model(&release).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"message": "report received"})
}

// GET /swarm/molt/check — node checks for pending update (called during heartbeat or config poll)
func (h *MoltHandler) Check(c *gin.Context) {
	nodeID := c.Query("node_id")
	version := c.Query("version")
	role := c.DefaultQuery("role", "claw")

	if nodeID == "" || version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id and version required"})
		return
	}

	// Find the latest active release for this role
	var release model.MoltRelease
	err := h.db.Where("target_role = ? AND status = ? AND version != ?",
		role, model.ReleaseRolling, version).
		Order("created_at DESC").First(&release).Error
	if err != nil {
		// No pending update
		c.JSON(http.StatusOK, gin.H{"update_available": false})
		return
	}

	// Check if this node is in the rollout
	var ns model.MoltNodeStatus
	if h.db.Where("release_id = ? AND node_id = ?", release.ID, nodeID).First(&ns).Error != nil {
		// Node not in this rollout batch (might be grayscale)
		c.JSON(http.StatusOK, gin.H{"update_available": false})
		return
	}

	if ns.Status != "pending" {
		// Already processed
		c.JSON(http.StatusOK, gin.H{"update_available": false})
		return
	}

	// Mark as updating
	h.db.Model(&ns).Updates(map[string]interface{}{
		"status":     "updating",
		"updated_at": time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{
		"update_available": true,
		"release_id":       release.ID,
		"version":          release.Version,
		"version_url":      release.VersionURL,
		"changelog":        release.Changelog,
		"mandatory":        release.Mandatory,
	})
}
