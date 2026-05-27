package swarm

// queen_embedded.go — Lightweight embedded Queen for self-hosted deployments.
// Handles /swarm/register and /swarm/heartbeat so that a Claw can join its own swarm
// without deploying the separate queen/swarm service.

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/molt"
	"gorm.io/gorm"
)

// EmbeddedNode stores registered swarm nodes in the local Claw DB.
type EmbeddedNode struct {
	ID            string    `gorm:"primaryKey;size:64" json:"id"`
	Name          string    `gorm:"size:128" json:"name"`
	Role          string    `gorm:"size:32;default:claw" json:"role"`
	Status        string    `gorm:"size:32;default:online" json:"status"`
	Version       string    `gorm:"size:64" json:"version"`
	Address       string    `gorm:"size:256" json:"address"`
	Region        string    `gorm:"size:64" json:"region"`
	ClawID        string    `gorm:"size:128;index" json:"claw_id"`
	Token         string    `gorm:"size:128" json:"token"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	RegisteredAt  time.Time `json:"registered_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (EmbeddedNode) TableName() string { return "swarm_nodes" }

// EmbeddedQueen provides minimal swarm registration for self-hosted mode.
type EmbeddedQueen struct {
	db *gorm.DB
}

func NewEmbeddedQueen(db *gorm.DB) *EmbeddedQueen {
	db.AutoMigrate(&EmbeddedNode{})
	return &EmbeddedQueen{db: db}
}

// Register handles POST /swarm/register
func (q *EmbeddedQueen) Register(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Role    string `json:"role" binding:"required"`
		Version string `json:"version"`
		Address string `json:"address" binding:"required"`
		Region  string `json:"region"`
		ClawID  string `json:"claw_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Upsert by claw_id
	if req.ClawID != "" {
		var existing EmbeddedNode
		if q.db.Where("claw_id = ?", req.ClawID).First(&existing).Error == nil {
			q.db.Model(&existing).Updates(map[string]interface{}{
				"name":           req.Name,
				"status":         "online",
				"version":        req.Version,
				"address":        req.Address,
				"region":         req.Region,
				"last_heartbeat": time.Now(),
			})
			c.JSON(http.StatusOK, gin.H{
				"node_id": existing.ID,
				"token":   existing.Token,
				"message": "node re-registered (updated)",
			})
			return
		}
	}

	token := generateNodeToken(32)
	nodeID := generateNodeToken(16)

	node := EmbeddedNode{
		ID:            nodeID,
		Name:          req.Name,
		Role:          req.Role,
		Status:        "online",
		Version:       req.Version,
		Address:       req.Address,
		Region:        req.Region,
		ClawID:        req.ClawID,
		Token:         token,
		LastHeartbeat: time.Now(),
		RegisteredAt:  time.Now(),
	}

	if err := q.db.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register node"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"node_id": nodeID,
		"token":   token,
		"message": "node registered successfully",
	})
}

// Heartbeat handles POST /swarm/heartbeat
func (q *EmbeddedQueen) Heartbeat(c *gin.Context) {
	var req struct {
		NodeID  string `json:"node_id" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Version string `json:"version"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var node EmbeddedNode
	if err := q.db.Where("id = ? AND token = ?", req.NodeID, req.Token).First(&node).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid node_id or token"})
		return
	}

	updates := map[string]interface{}{
		"status":         "online",
		"last_heartbeat": time.Now(),
	}
	if req.Version != "" {
		updates["version"] = req.Version
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	q.db.Model(&node).Updates(updates)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "heartbeat received"})
}

// Config handles GET /swarm/config
func (q *EmbeddedQueen) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"config": gin.H{
			"models":   []interface{}{},
			"policies": gin.H{"heartbeat_interval": "60s", "auto_update": true},
			"version":  molt.Version,
		},
	})
}

// Nodes handles GET /swarm/nodes
func (q *EmbeddedQueen) Nodes(c *gin.Context) {
	var nodes []EmbeddedNode
	q.db.Order("last_heartbeat DESC").Find(&nodes)
	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "total": len(nodes)})
}

func generateNodeToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
