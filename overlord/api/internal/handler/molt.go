package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type MoltHandler struct {
	db *gorm.DB
}

func NewMoltHandler(db *gorm.DB) *MoltHandler {
	return &MoltHandler{db: db}
}

// ---------- POST /brood/molt/releases ----------

func (h *MoltHandler) CreateRelease(c *gin.Context) {
	var req struct {
		Version      string `json:"version" binding:"required"`
		Channel      string `json:"channel"`
		Title        string `json:"title"`
		ReleaseNotes string `json:"release_notes"`
		DownloadURL  string `json:"download_url"`
		Checksum     string `json:"checksum"`
		TargetTeam   string `json:"target_team"`
		RolloutPct   int    `json:"rollout_pct"`
		MaxFailures  int    `json:"max_failures"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel := req.Channel
	if channel == "" {
		channel = "stable"
	}
	rolloutPct := req.RolloutPct
	if rolloutPct <= 0 {
		rolloutPct = 100
	}
	maxFail := req.MaxFailures
	if maxFail <= 0 {
		maxFail = 1
	}

	release := model.MoltRelease{
		Version:      req.Version,
		Channel:      channel,
		Title:        req.Title,
		ReleaseNotes: req.ReleaseNotes,
		DownloadURL:  req.DownloadURL,
		Checksum:     req.Checksum,
		SubmittedBy:  middleware.GetAdminActor(c),
		TargetTeam:   req.TargetTeam,
		RolloutPct:   rolloutPct,
		MaxFailures:  maxFail,
		Status:       "pending",
	}

	if err := h.db.Create(&release).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create release"})
		return
	}

	audit(h.db, c, "create_release", release.ID, "molt release "+req.Version+" submitted")
	c.JSON(http.StatusCreated, gin.H{"release": release})
}

// ---------- GET /brood/molt/releases ----------

func (h *MoltHandler) ListReleases(c *gin.Context) {
	status := c.Query("status")
	channel := c.Query("channel")

	q := h.db.Order("created_at DESC")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if channel != "" {
		q = q.Where("channel = ?", channel)
	}

	var releases []model.MoltRelease
	q.Find(&releases)
	c.JSON(http.StatusOK, gin.H{"releases": releases, "total": len(releases)})
}

// ---------- GET /brood/molt/releases/:id ----------

func (h *MoltHandler) GetRelease(c *gin.Context) {
	id := c.Param("id")
	var release model.MoltRelease
	if err := h.db.First(&release, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	// Include per-node status
	var nodeStatuses []model.MoltNodeStatus
	h.db.Where("release_id = ?", id).Order("created_at DESC").Find(&nodeStatuses)

	c.JSON(http.StatusOK, gin.H{"release": release, "node_statuses": nodeStatuses})
}

// ---------- POST /brood/molt/releases/:id/approve ----------

func (h *MoltHandler) ApproveRelease(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Action string `json:"action" binding:"required"` // approve, reject
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var release model.MoltRelease
	if err := h.db.First(&release, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	if release.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "release is not pending"})
		return
	}

	now := time.Now()
	reviewer := middleware.GetAdminActor(c)

	var newStatus string
	switch req.Action {
	case "approve":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be approve or reject"})
		return
	}

	h.db.Model(&release).Updates(map[string]interface{}{
		"status":      newStatus,
		"reviewed_by": reviewer,
		"reviewed_at": &now,
		"review_note": req.Note,
	})

	audit(h.db, c, "review_release", id, "release "+release.Version+" "+newStatus)
	c.JSON(http.StatusOK, gin.H{"message": "release " + newStatus, "status": newStatus})
}

// ---------- POST /brood/molt/releases/:id/rollout ----------

func (h *MoltHandler) StartRollout(c *gin.Context) {
	id := c.Param("id")

	var release model.MoltRelease
	if err := h.db.First(&release, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "release not found"})
		return
	}

	if release.Status != "approved" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "release must be approved first"})
		return
	}

	// Find target nodes
	q := h.db.Where("status IN ?", []string{"online", "feral"})
	if release.TargetTeam != "" {
		q = q.Where("team = ?", release.TargetTeam)
	}
	// Exclude nodes already on this version
	q = q.Where("version != ?", release.Version)

	var nodes []model.ClawNode
	q.Find(&nodes)

	// Apply rollout percentage
	targetCount := len(nodes)
	if release.RolloutPct < 100 && targetCount > 0 {
		targetCount = (targetCount * release.RolloutPct) / 100
		if targetCount < 1 {
			targetCount = 1
		}
		nodes = nodes[:targetCount]
	}

	// Create per-node status records
	for _, node := range nodes {
		ns := model.MoltNodeStatus{
			ReleaseID:  release.ID,
			ClawNodeID: node.ID,
			ClawName:   node.Name,
			OldVersion: node.Version,
			Status:     "pending",
		}
		h.db.Create(&ns)
	}

	h.db.Model(&release).Updates(map[string]interface{}{
		"status":      "rolling",
		"total_nodes": len(nodes),
	})

	audit(h.db, c, "start_rollout", id, "rollout started for "+release.Version+
		" to "+fmt.Sprintf("%d", len(nodes))+" nodes")
	c.JSON(http.StatusOK, gin.H{
		"message":      "rollout started",
		"total_nodes":  len(nodes),
		"target_nodes": targetCount,
	})
}

// ---------- POST /brood/molt/node-status ----------
// Called by Claw nodes to report their update progress

func (h *MoltHandler) ReportNodeStatus(c *gin.Context) {
	var req struct {
		ReleaseID  string `json:"release_id" binding:"required"`
		ClawNodeID string `json:"claw_node_id" binding:"required"`
		Status     string `json:"status" binding:"required"` // downloading, installing, completed, failed
		Error      string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{"status": req.Status}
	if req.Status == "downloading" || req.Status == "installing" {
		updates["started_at"] = &now
	}
	if req.Status == "completed" || req.Status == "failed" {
		updates["completed_at"] = &now
	}
	if req.Error != "" {
		updates["error_detail"] = req.Error
	}

	h.db.Model(&model.MoltNodeStatus{}).
		Where("release_id = ? AND claw_node_id = ?", req.ReleaseID, req.ClawNodeID).
		Updates(updates)

	// Update aggregate counts on release
	var release model.MoltRelease
	if h.db.First(&release, "id = ?", req.ReleaseID).Error == nil {
		var updated, failed int64
		h.db.Model(&model.MoltNodeStatus{}).Where("release_id = ? AND status = ?", req.ReleaseID, "completed").Count(&updated)
		h.db.Model(&model.MoltNodeStatus{}).Where("release_id = ? AND status = ?", req.ReleaseID, "failed").Count(&failed)

		releaseUpdates := map[string]interface{}{
			"updated_nodes": updated,
			"failed_nodes":  failed,
		}

		// Auto-halt if too many failures
		if int(failed) >= release.MaxFailures && release.Status == "rolling" {
			releaseUpdates["status"] = "approved" // pause rollout back to approved
			audit(h.db, c, "auto_halt_rollout", req.ReleaseID,
				"rollout auto-halted: "+fmt.Sprintf("%d", failed)+" failures")
		}

		// Mark completed if all done
		if int(updated)+int(failed) >= release.TotalNodes && release.Status == "rolling" {
			releaseUpdates["status"] = "completed"
		}

		h.db.Model(&release).Updates(releaseUpdates)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
