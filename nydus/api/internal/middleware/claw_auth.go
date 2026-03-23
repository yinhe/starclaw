package middleware

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/nydus/internal/database"
	"github.com/yinhe/starclaw/nydus/internal/model"
)

// ClawAuth validates Claw node identity via one of:
//  1. Ed25519 signature: X-Claw-ID + X-Claw-Timestamp + X-Claw-Signature
//  2. Bearer token: Authorization: Bearer {node-token}
//  3. Fallback to SecretAuth (X-Nydus-Secret) for backward compat
//
// On success, sets "node_id" and "node" in gin context.
func ClawAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Method 1: Ed25519 signature
		nodeID := c.GetHeader("X-Claw-ID")
		timestamp := c.GetHeader("X-Claw-Timestamp")
		signature := c.GetHeader("X-Claw-Signature")

		if nodeID != "" && timestamp != "" && signature != "" {
			if verifyEd25519(c, nodeID, timestamp, signature) {
				c.Next()
				return
			}
			return // verifyEd25519 already aborted
		}

		// Method 2: Bearer token
		auth := c.GetHeader("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token := auth[7:]
			var node model.NydusNode
			if err := database.DB.Where("token = ?", token).First(&node).Error; err == nil {
				c.Set("node_id", node.NodeID)
				c.Set("node", &node)
				now := time.Now()
				database.DB.Model(&node).Update("last_seen_at", &now)
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: provide X-Claw-ID/Signature or Bearer token"})
	}
}

// verifyEd25519 checks the Ed25519 signature from a registered Claw node.
// Message = "nodeID|timestamp" signed with the node's Ed25519 private key.
func verifyEd25519(c *gin.Context, nodeID, timestamp, signatureB64 string) bool {
	// Check timestamp freshness (±5 min)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid timestamp"})
		return false
	}
	diff := time.Now().Unix() - ts
	if math.Abs(float64(diff)) > 300 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "timestamp expired (±5min)"})
		return false
	}

	// Look up node
	var node model.NydusNode
	if err := database.DB.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("node %s not registered", nodeID)})
		return false
	}

	// Decode public key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(node.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "invalid stored public key"})
		return false
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	// Decode signature
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid signature encoding"})
		return false
	}

	// Verify: message = "nodeID|timestamp"
	message := []byte(nodeID + "|" + timestamp)
	if !ed25519.Verify(pubKey, message, sig) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed"})
		return false
	}

	c.Set("node_id", node.NodeID)
	c.Set("node", &node)
	now := time.Now()
	database.DB.Model(&node).Update("last_seen_at", &now)
	return true
}
