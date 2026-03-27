package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/middleware"
	"starclaw.net/queen/api/internal/model"
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

// POST /internal/identity/migrate — migrate all data from old claw address to new one
// Called when a Claw node's Ed25519 key changes (reinstall, Hive recreate, etc.)
// The same Queen user must own the old address (verified via NodeBinding).
func (h *NodeBindingHandler) InternalMigrateIdentity(c *gin.Context) {
	var req struct {
		OldClawID   string `json:"old_claw_id" binding:"required"`
		NewClawID   string `json:"new_claw_id" binding:"required"`
		QueenUserID string `json:"queen_user_id"` // optional: verify ownership
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "old_claw_id and new_claw_id required"})
		return
	}

	if req.OldClawID == req.NewClawID {
		c.JSON(http.StatusOK, gin.H{"message": "same address, no migration needed"})
		return
	}

	// Verify old address exists in NodeBinding
	var oldBinding model.NodeBinding
	if err := database.DB.Where("node_id = ? AND status = ?", req.OldClawID, "active").First(&oldBinding).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "old claw address not found in bindings"})
		return
	}

	// If queen_user_id provided, verify ownership
	if req.QueenUserID != "" && oldBinding.QueenUserID != req.QueenUserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "old address belongs to a different user"})
		return
	}

	migrated := []string{}

	// 1. Migrate CreditAccount: update claw_id
	var oldAccount model.CreditAccount
	if err := database.DB.Where("claw_id = ?", req.OldClawID).First(&oldAccount).Error; err == nil {
		// Check if new address already has an account
		var newAccount model.CreditAccount
		if err := database.DB.Where("claw_id = ?", req.NewClawID).First(&newAccount).Error; err == nil {
			// Merge: add old balance to new, deactivate old
			database.DB.Model(&newAccount).Updates(map[string]interface{}{
				"balance":   newAccount.Balance + oldAccount.Balance,
				"frozen":    newAccount.Frozen + oldAccount.Frozen,
				"total_in":  newAccount.TotalIn + oldAccount.TotalIn,
				"total_out": newAccount.TotalOut + oldAccount.TotalOut,
			})
			database.DB.Model(&oldAccount).Update("status", "migrated")
			migrated = append(migrated, "credit_account(merged)")
		} else {
			// Simply update the claw_id
			database.DB.Model(&oldAccount).Update("claw_id", req.NewClawID)
			migrated = append(migrated, "credit_account")
		}
	}

	// 2. Migrate NodeBinding: update node_id to new address
	database.DB.Model(&model.NodeBinding{}).Where("node_id = ?", req.OldClawID).
		Updates(map[string]interface{}{"node_id": req.NewClawID, "status": "active", "last_seen": time.Now()})
	migrated = append(migrated, "node_binding")

	// 3. Migrate CreditTransaction references (for audit trail)
	database.DB.Model(&model.CreditTransaction{}).Where("from_claw = ?", req.OldClawID).Update("from_claw", req.NewClawID)
	database.DB.Model(&model.CreditTransaction{}).Where("to_claw = ?", req.OldClawID).Update("to_claw", req.NewClawID)
	migrated = append(migrated, "credit_transactions")

	// 4. Migrate CreditFreeze records
	database.DB.Model(&model.CreditFreeze{}).Where("claw_id = ?", req.OldClawID).Update("claw_id", req.NewClawID)
	migrated = append(migrated, "credit_freezes")

	log.Printf("[identity-migrate] %s → %s (user=%s, migrated=%v)", req.OldClawID, req.NewClawID, oldBinding.QueenUserID, migrated)

	c.JSON(http.StatusOK, gin.H{
		"message":       "identity migrated",
		"old_claw_id":   req.OldClawID,
		"new_claw_id":   req.NewClawID,
		"queen_user_id": oldBinding.QueenUserID,
		"migrated":      migrated,
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
