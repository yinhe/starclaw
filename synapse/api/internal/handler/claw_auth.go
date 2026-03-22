package handler

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/middleware"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

// ClawAuthHandler handles "Sign-In with Claw" authentication for star-ai.net.
//
// Flow (same as Queen community — MetaMask-style bidirectional auth):
//  1. Frontend calls POST /auth/claw/challenge → gets a random nonce
//  2. Frontend sends the nonce to the user's Claw node: POST <claw_url>/v1/identity/auth-request
//  3. User approves on Claw UI → Claw signs the challenge with Ed25519
//  4. Frontend polls Claw for approval, gets {node_id, public_key, signature}
//  5. Frontend sends all of that to POST /auth/claw/verify
//  6. star-ai verifies the signature, creates/links user account, returns JWT
type ClawAuthHandler struct {
	db        *gorm.DB
	jwtSecret string
	expHours  int

	nonces  map[string]time.Time
	nonceMu sync.Mutex
}

func NewClawAuthHandler(db *gorm.DB, jwtSecret string, expHours int) *ClawAuthHandler {
	h := &ClawAuthHandler{
		db:        db,
		jwtSecret: jwtSecret,
		expHours:  expHours,
		nonces:    make(map[string]time.Time),
	}
	// Background cleanup of expired nonces
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			h.nonceMu.Lock()
			now := time.Now()
			for k, exp := range h.nonces {
				if now.After(exp) {
					delete(h.nonces, k)
				}
			}
			h.nonceMu.Unlock()
		}
	}()
	return h
}

// POST /auth/claw/challenge — returns a random challenge nonce
func (h *ClawAuthHandler) Challenge(c *gin.Context) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成挑战失败"})
		return
	}

	challenge := fmt.Sprintf("starai-auth:%s:%d", hex.EncodeToString(nonce), time.Now().Unix())

	h.nonceMu.Lock()
	h.nonces[challenge] = time.Now().Add(5 * time.Minute)
	h.nonceMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"challenge":  challenge,
		"expires_in": 300,
	})
}

// POST /auth/claw/verify — verifies Ed25519 signature and issues JWT
func (h *ClawAuthHandler) Verify(c *gin.Context) {
	var req struct {
		Challenge string `json:"challenge" binding:"required"`
		NodeID    string `json:"node_id" binding:"required"`
		PublicKey string `json:"public_key" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Username  string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	// 1. Verify nonce exists and hasn't expired (one-time use)
	h.nonceMu.Lock()
	expiry, exists := h.nonces[req.Challenge]
	if exists {
		delete(h.nonces, req.Challenge)
	}
	h.nonceMu.Unlock()

	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的挑战码（可能已过期或已使用）"})
		return
	}
	if time.Now().After(expiry) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "挑战码已过期，请重新获取"})
		return
	}

	// 2. Decode and validate public key
	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公钥格式"})
		return
	}

	// 3. Verify node_id = "claw:" + SHA256(pubkey)[:40]
	hash := sha256.Sum256(pubKeyBytes)
	expectedNodeID := "claw:" + hex.EncodeToString(hash[:])[:40]
	if req.NodeID != expectedNodeID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "节点 ID 与公钥不匹配"})
		return
	}

	// 4. Verify Ed25519 signature
	sigBytes, err := hex.DecodeString(req.Signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的签名格式"})
		return
	}
	if !ed25519.Verify(pubKeyBytes, []byte(req.Challenge), sigBytes) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "签名验证失败"})
		return
	}

	// 5. Find or create user account linked to this Claw address
	user := h.findOrCreateClawUser(req.NodeID, req.Username)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建账号失败"})
		return
	}

	// 6. Issue JWT
	token, err := middleware.GenerateJWT(h.jwtSecret, user.ID, req.NodeID, h.expHours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	log.Printf("[claw-auth] Claw login success: %s → user %s (%s)", req.NodeID, user.ID, user.Name)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":      user.ID,
			"email":   user.Email,
			"phone":   user.Phone,
			"name":    user.Name,
			"claw_id": user.ClawID,
			"balance": user.Balance,
		},
	})
}

// findOrCreateClawUser finds a user by claw_id, or creates a new one.
// If username is provided from Claw, it updates the display name.
func (h *ClawAuthHandler) findOrCreateClawUser(clawID, username string) *model.User {
	// Try to find existing user by claw_id
	var user model.User
	if err := h.db.Where("claw_id = ?", clawID).First(&user).Error; err == nil {
		// Update name if Claw passed a username and current name is just the node ID
		if username != "" {
			newName := h.formatDisplayName(username, clawID)
			if user.Name != newName {
				h.db.Model(&user).Update("name", newName)
				user.Name = newName
			}
		}
		return &user
	}

	// Build display name: "username (claw:abc123...)" or just short node ID
	displayName := h.formatDisplayName(username, clawID)

	// Derive unique placeholder email/phone from claw_id to avoid MySQL unique index
	// conflicts (MySQL treats empty string as a value, not NULL)
	idHash := sha256.Sum256([]byte(clawID))
	placeholder := hex.EncodeToString(idHash[:6]) // 12 hex chars

	user = model.User{
		Name:      displayName,
		Email:     placeholder + "@claw.local",
		Phone:     "c" + placeholder,
		ClawID:    clawID,
		FreeQuota: 0,
		Status:    "active",
	}
	if err := h.db.Create(&user).Error; err != nil {
		log.Printf("[claw-auth] Failed to create user for %s: %v", clawID, err)
		return nil
	}

	log.Printf("[claw-auth] Created new star-ai user %s for claw %s", user.ID, clawID)
	return &user
}

// formatDisplayName returns "username (claw:abc1...)" or just the short node ID.
func (h *ClawAuthHandler) formatDisplayName(username, clawID string) string {
	shortID := clawID
	if len(shortID) > 14 {
		shortID = shortID[:14] + "…"
	}
	if username != "" && username != "admin" {
		return username + " (" + shortID + ")"
	}
	if username == "admin" {
		return "admin (" + shortID + ")"
	}
	return shortID
}
