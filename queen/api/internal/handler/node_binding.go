package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

type NodeBindingHandler struct{}

// ============================================================
// User-facing API (authenticated Queen user)
// ============================================================

// POST /user/nodes — bind a Claw node to current Queen user
func (h *NodeBindingHandler) BindNode(c *gin.Context) {
	queenUserID := c.GetString("user_id")
	var req struct {
		NodeID      string `json:"node_id" binding:"required"`       // claw:xxxx
		LocalUserID string `json:"local_user_id" binding:"required"` // user ID on the Claw
		NodeName    string `json:"node_name"`
		NodeAddr    string `json:"node_addr"`
		NodeRegion  string `json:"node_region"`
		NodeVersion string `json:"node_version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest, "请提供 node_id 和 local_user_id")
		return
	}

	// Check if node already bound to another user
	var existing model.NodeBinding
	if err := database.DB.Where("node_id = ?", req.NodeID).First(&existing).Error; err == nil {
		if existing.QueenUserID != queenUserID {
			middleware.Fail(c, http.StatusConflict, middleware.CodeConflict, "该节点已被其他用户绑定")
			return
		}
		// Update existing binding
		existing.LocalUserID = req.LocalUserID
		if req.NodeName != "" {
			existing.NodeName = req.NodeName
		}
		if req.NodeAddr != "" {
			existing.NodeAddr = req.NodeAddr
		}
		if req.NodeRegion != "" {
			existing.NodeRegion = req.NodeRegion
		}
		if req.NodeVersion != "" {
			existing.NodeVersion = req.NodeVersion
		}
		existing.Status = "active"
		existing.LastSeen = time.Now()
		database.DB.Save(&existing)
		middleware.OK(c, gin.H{"binding": existing, "updated": true})
		return
	}

	binding := model.NodeBinding{
		ID:          uuid.New().String(),
		QueenUserID: queenUserID,
		NodeID:      req.NodeID,
		LocalUserID: req.LocalUserID,
		NodeName:    req.NodeName,
		NodeAddr:    req.NodeAddr,
		NodeRegion:  req.NodeRegion,
		NodeVersion: req.NodeVersion,
		Status:      "active",
		LastSeen:    time.Now(),
	}
	if err := database.DB.Create(&binding).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal, "绑定失败")
		return
	}

	log.Printf("[node-binding] User %s bound node %s (local_user=%s)", queenUserID, req.NodeID, req.LocalUserID)
	middleware.OK(c, gin.H{"binding": binding})
}

// GET /user/nodes — list all nodes bound to current Queen user
func (h *NodeBindingHandler) ListNodes(c *gin.Context) {
	queenUserID := c.GetString("user_id")
	var bindings []model.NodeBinding
	database.DB.Where("queen_user_id = ? AND status != ?", queenUserID, "revoked").
		Order("created_at DESC").Find(&bindings)
	middleware.OK(c, gin.H{"nodes": bindings, "total": len(bindings)})
}

// DELETE /user/nodes/:node_id — unbind a node
func (h *NodeBindingHandler) UnbindNode(c *gin.Context) {
	queenUserID := c.GetString("user_id")
	nodeID := c.Param("node_id")

	var binding model.NodeBinding
	if err := database.DB.Where("node_id = ? AND queen_user_id = ?", nodeID, queenUserID).First(&binding).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound, "未找到该节点绑定")
		return
	}

	binding.Status = "revoked"
	database.DB.Save(&binding)

	log.Printf("[node-binding] User %s unbound node %s", queenUserID, nodeID)
	middleware.OK(c, gin.H{"message": "节点已解绑"})
}

// ============================================================
// Internal API (called by Claw nodes via X-Node-Token)
// ============================================================

// POST /internal/user/bind — Claw node registers its binding with Queen
func (h *NodeBindingHandler) InternalBind(c *gin.Context) {
	var req struct {
		QueenUserID string `json:"queen_user_id" binding:"required"`
		NodeID      string `json:"node_id" binding:"required"`
		LocalUserID string `json:"local_user_id" binding:"required"`
		NodeName    string `json:"node_name"`
		NodeAddr    string `json:"node_addr"`
		NodeRegion  string `json:"node_region"`
		NodeVersion string `json:"node_version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	var existing model.NodeBinding
	if err := database.DB.Where("node_id = ?", req.NodeID).First(&existing).Error; err == nil {
		// Update
		existing.QueenUserID = req.QueenUserID
		existing.LocalUserID = req.LocalUserID
		if req.NodeName != "" {
			existing.NodeName = req.NodeName
		}
		if req.NodeAddr != "" {
			existing.NodeAddr = req.NodeAddr
		}
		if req.NodeRegion != "" {
			existing.NodeRegion = req.NodeRegion
		}
		if req.NodeVersion != "" {
			existing.NodeVersion = req.NodeVersion
		}
		existing.Status = "active"
		existing.LastSeen = time.Now()
		database.DB.Save(&existing)
		c.JSON(http.StatusOK, gin.H{"binding_id": existing.ID, "updated": true})
		return
	}

	binding := model.NodeBinding{
		ID:          uuid.New().String(),
		QueenUserID: req.QueenUserID,
		NodeID:      req.NodeID,
		LocalUserID: req.LocalUserID,
		NodeName:    req.NodeName,
		NodeAddr:    req.NodeAddr,
		NodeRegion:  req.NodeRegion,
		NodeVersion: req.NodeVersion,
		Status:      "active",
		LastSeen:    time.Now(),
	}
	if err := database.DB.Create(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bind failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"binding_id": binding.ID})
}

// GET /internal/user/resolve/:node_id — resolve node_id to queen_user_id
func (h *NodeBindingHandler) InternalResolve(c *gin.Context) {
	nodeID := c.Param("node_id")
	var binding model.NodeBinding
	if err := database.DB.Where("node_id = ? AND status = ?", nodeID, "active").First(&binding).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no binding for this node"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"queen_user_id": binding.QueenUserID,
		"local_user_id": binding.LocalUserID,
		"node_name":     binding.NodeName,
	})
}

// POST /internal/user/heartbeat — Claw node sends heartbeat to update last_seen
func (h *NodeBindingHandler) InternalHeartbeat(c *gin.Context) {
	var req struct {
		NodeID      string `json:"node_id" binding:"required"`
		NodeVersion string `json:"node_version"`
		NodeAddr    string `json:"node_addr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}

	updates := map[string]interface{}{
		"last_seen": time.Now(),
		"status":    "active",
	}
	if req.NodeVersion != "" {
		updates["node_version"] = req.NodeVersion
	}
	if req.NodeAddr != "" {
		updates["node_addr"] = req.NodeAddr
	}

	result := database.DB.Model(&model.NodeBinding{}).Where("node_id = ?", req.NodeID).Updates(updates)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not bound"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
