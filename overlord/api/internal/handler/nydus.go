package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"starclaw.net/overlord/api/internal/middleware"
	"starclaw.net/overlord/api/internal/model"
	"gorm.io/gorm"
)

type NydusHandler struct {
	db *gorm.DB
}

func NewNydusHandler(db *gorm.DB) *NydusHandler {
	return &NydusHandler{db: db}
}

// ---------- POST /brood/tunnels ----------

func (h *NydusHandler) CreateTunnel(c *gin.Context) {
	var req struct {
		ClawNodeID string `json:"claw_node_id" binding:"required"`
		LocalPort  int    `json:"local_port" binding:"required"`
		RemotePort int    `json:"remote_port" binding:"required"`
		Protocol   string `json:"protocol"`
		Mode       string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify claw exists
	var node model.ClawNode
	if err := h.db.First(&node, "id = ?", req.ClawNodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "claw node not found"})
		return
	}

	protocol := req.Protocol
	if protocol == "" {
		protocol = "tcp"
	}
	mode := req.Mode
	if mode == "" {
		mode = "forward"
	}

	tunnel := model.NydusTunnel{
		ClawNodeID: req.ClawNodeID,
		ClawName:   node.Name,
		Team:       node.Team,
		LocalPort:  req.LocalPort,
		RemotePort: req.RemotePort,
		Protocol:   protocol,
		Mode:       mode,
		Status:     "pending",
	}

	if err := h.db.Create(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tunnel"})
		return
	}

	audit(h.db, c, "create_tunnel", tunnel.ID, "tunnel created: "+node.Name+
		" local:"+itoa(req.LocalPort)+" remote:"+itoa(req.RemotePort))
	c.JSON(http.StatusCreated, gin.H{"tunnel": tunnel})
}

// ---------- GET /brood/tunnels ----------

func (h *NydusHandler) ListTunnels(c *gin.Context) {
	q := middleware.TeamScope(c, h.db)
	status := c.Query("status")
	nodeID := c.Query("claw_node_id")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if nodeID != "" {
		q = q.Where("claw_node_id = ?", nodeID)
	}

	var tunnels []model.NydusTunnel
	q.Order("created_at DESC").Find(&tunnels)
	c.JSON(http.StatusOK, gin.H{"tunnels": tunnels, "total": len(tunnels)})
}

// ---------- GET /brood/tunnels/:id ----------

func (h *NydusHandler) GetTunnel(c *gin.Context) {
	id := c.Param("id")
	var tunnel model.NydusTunnel
	if err := h.db.First(&tunnel, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnel": tunnel})
}

// ---------- PUT /brood/tunnels/:id/status ----------

func (h *NydusHandler) UpdateTunnelStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status    string `json:"status" binding:"required"`
		LastError string `json:"last_error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	valid := map[string]bool{"pending": true, "active": true, "error": true, "closed": true}
	if !valid[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	updates := map[string]interface{}{"status": req.Status}
	if req.LastError != "" {
		updates["last_error"] = req.LastError
	}

	if err := h.db.Model(&model.NydusTunnel{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tunnel"})
		return
	}

	audit(h.db, c, "update_tunnel", id, "tunnel status → "+req.Status)
	c.JSON(http.StatusOK, gin.H{"message": "tunnel updated"})
}

// ---------- PUT /brood/tunnels/:id/metrics ----------
// Called by Claw to report tunnel traffic stats

func (h *NydusHandler) UpdateTunnelMetrics(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		BytesIn     int64 `json:"bytes_in"`
		BytesOut    int64 `json:"bytes_out"`
		Connections int   `json:"connections"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&model.NydusTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"bytes_in":    req.BytesIn,
		"bytes_out":   req.BytesOut,
		"connections": req.Connections,
	})
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ---------- DELETE /brood/tunnels/:id ----------

func (h *NydusHandler) DeleteTunnel(c *gin.Context) {
	id := c.Param("id")
	if err := h.db.Where("id = ?", id).Delete(&model.NydusTunnel{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tunnel"})
		return
	}
	audit(h.db, c, "delete_tunnel", id, "tunnel deleted")
	c.JSON(http.StatusOK, gin.H{"message": "tunnel deleted"})
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
