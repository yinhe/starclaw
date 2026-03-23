package handler

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw/nydus/internal/database"
	"github.com/yinhe/starclaw/nydus/internal/model"
)

// RegisterNode handles node self-registration.
// The node provides its Ed25519 public key and signs a challenge.
//
// POST /api/nodes/register
//
//	{
//	  "node_id":    "claw:6aff1154...",
//	  "name":       "my-claw-node",
//	  "public_key": "<base64 Ed25519 public key>",
//	  "ssh_pub_key": "ssh-ed25519 AAAA...",  // optional
//	  "team_id":    "...",                    // optional
//	  "timestamp":  "1711180800",
//	  "signature":  "<base64 signature of 'node_id|timestamp'>"
//	}
func RegisterNode(c *gin.Context) {
	var req struct {
		NodeID    string `json:"node_id" binding:"required"`
		Name      string `json:"name"`
		PublicKey string `json:"public_key" binding:"required"`
		SSHPubKey string `json:"ssh_pub_key"`
		TeamID    string `json:"team_id"`
		Timestamp string `json:"timestamp" binding:"required"`
		Signature string `json:"signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields: " + err.Error()})
		return
	}

	// Verify Ed25519 signature to prove key ownership
	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Ed25519 public key (32 bytes base64)"})
		return
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature encoding"})
		return
	}

	message := []byte(req.NodeID + "|" + req.Timestamp)
	if !ed25519.Verify(pubKey, message, sig) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed — cannot prove key ownership"})
		return
	}

	// Check if node already registered
	var existing model.NydusNode
	if err := database.DB.Where("node_id = ?", req.NodeID).First(&existing).Error; err == nil {
		// Re-register: update public key and generate new token
		token := generateToken()
		database.DB.Model(&existing).Updates(map[string]interface{}{
			"public_key":  req.PublicKey,
			"ssh_pub_key": req.SSHPubKey,
			"name":        req.Name,
			"team_id":     req.TeamID,
			"token":       token,
		})
		log.Printf("[nydus] node re-registered: %s (%s)", req.NodeID, req.Name)
		c.JSON(http.StatusOK, gin.H{
			"node":    existing,
			"token":   token,
			"message": "node re-registered, new token issued",
		})
		return
	}

	// New registration
	token := generateToken()
	node := model.NydusNode{
		ID:        uuid.New().String(),
		NodeID:    req.NodeID,
		Name:      req.Name,
		PublicKey:  req.PublicKey,
		SSHPubKey: req.SSHPubKey,
		Role:      "member",
		TeamID:    req.TeamID,
		Token:     token,
	}
	if err := database.DB.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register node: " + err.Error()})
		return
	}

	log.Printf("[nydus] node registered: %s (%s) team=%s", req.NodeID, req.Name, req.TeamID)
	c.JSON(http.StatusCreated, gin.H{
		"node":    node,
		"token":   token,
		"message": "node registered successfully",
	})
}

// ListNodes returns all registered nodes (admin only, requires X-Nydus-Secret).
// GET /api/nodes
func ListNodes(c *gin.Context) {
	var nodes []model.NydusNode
	query := database.DB.Order("created_at DESC")

	if teamID := c.Query("team_id"); teamID != "" {
		query = query.Where("team_id = ?", teamID)
	}
	query.Find(&nodes)

	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "total": len(nodes)})
}

// GetNode returns a single node by node_id.
// GET /api/nodes/:node_id
func GetNode(c *gin.Context) {
	nodeID := c.Param("node_id")
	var node model.NydusNode
	if err := database.DB.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"node": node})
}

// UpdateNodeRole changes a node's role (owner/admin/member/readonly).
// PUT /api/nodes/:node_id/role
func UpdateNodeRole(c *gin.Context) {
	nodeID := c.Param("node_id")
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role required"})
		return
	}
	validRoles := map[string]bool{"owner": true, "admin": true, "member": true, "readonly": true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, use owner/admin/member/readonly"})
		return
	}

	result := database.DB.Model(&model.NydusNode{}).Where("node_id = ?", nodeID).Update("role", req.Role)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("node %s role updated to %s", nodeID, req.Role)})
}

// DeleteNode removes a node registration.
// DELETE /api/nodes/:node_id
func DeleteNode(c *gin.Context) {
	nodeID := c.Param("node_id")
	result := database.DB.Where("node_id = ?", nodeID).Delete(&model.NydusNode{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("node %s deleted", nodeID)})
}

// MyNode returns the authenticated node's info.
// GET /node/me
func MyNode(c *gin.Context) {
	node, exists := c.Get("node")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"node": node})
}

// --- helpers ---

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("nydus_%s_%s", time.Now().Format("20060102"), hex.EncodeToString(b))
}
