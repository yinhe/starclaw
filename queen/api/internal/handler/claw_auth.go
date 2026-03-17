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
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

// ClawAuthHandler handles "Sign-In with Claw" authentication.
//
// Flow:
//  1. Frontend calls POST /auth/claw/challenge → gets a random nonce
//  2. Frontend sends the nonce to the user's Claw node: POST <claw_url>/v1/identity/sign-challenge
//  3. Claw signs the nonce with its Ed25519 private key, returns {node_id, public_key, signature}
//  4. Frontend sends all of that to POST /auth/claw/verify
//  5. Queen verifies the signature, creates/links the user account, returns JWT
type ClawAuthHandler struct {
	// In-memory nonce store (challenge → expiry). Production: use Redis.
	nonces  map[string]time.Time
	nonceMu sync.Mutex
}

func NewClawAuthHandler() *ClawAuthHandler {
	h := &ClawAuthHandler{
		nonces: make(map[string]time.Time),
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

// POST /auth/claw/challenge
// Returns a random challenge nonce for the Claw node to sign.
func (h *ClawAuthHandler) Challenge(c *gin.Context) {
	// Generate 32-byte random nonce
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成挑战失败"})
		return
	}

	challenge := fmt.Sprintf("starclaw-auth:%s:%d", hex.EncodeToString(nonce), time.Now().Unix())

	// Store nonce with 5-minute expiry
	h.nonceMu.Lock()
	h.nonces[challenge] = time.Now().Add(5 * time.Minute)
	h.nonceMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"challenge":  challenge,
		"expires_in": 300,
	})
}

// POST /auth/claw/verify
// Verifies the Ed25519 signature from a Claw node and issues a JWT.
func (h *ClawAuthHandler) Verify(c *gin.Context) {
	var req struct {
		Challenge string `json:"challenge" binding:"required"`
		NodeID    string `json:"node_id" binding:"required"`    // claw:xxxxx
		PublicKey string `json:"public_key" binding:"required"` // hex-encoded Ed25519 public key
		Signature string `json:"signature" binding:"required"`  // hex-encoded Ed25519 signature
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}

	// 1. Verify nonce exists and hasn't expired
	h.nonceMu.Lock()
	expiry, exists := h.nonces[req.Challenge]
	if exists {
		delete(h.nonces, req.Challenge) // one-time use
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

	// 2. Decode public key
	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公钥格式"})
		return
	}

	// 3. Verify node_id matches public key
	// node_id = "claw:" + first 40 hex chars of SHA-256(publicKey)
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

	// 5. Signature valid! Find or create user account linked to this Claw address.
	user := h.findOrCreateClawUser(req.NodeID, req.PublicKey)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建账号失败"})
		return
	}

	// 5b. Auto-link partner whitelist: if this claw_id is whitelisted as a
	// core partner or city partner but not yet linked to a user, bind now.
	h.autoLinkPartner(req.NodeID, user)

	// 6. Issue JWT
	token, err := middleware.GenerateToken(user.ID, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	log.Printf("[claw-auth] Claw login success: %s → user %s (%s)", req.NodeID, user.ID, user.Nickname)

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"email":    user.Email,
			"phone":    user.Phone,
			"nickname": user.Nickname,
			"avatar":   user.Avatar,
			"role":     user.Role,
			"claw_id":  req.NodeID,
		},
	})
}

// autoLinkPartner checks if the claw_id is whitelisted as a core partner or city partner.
// If found and not yet linked to a user, it binds the user and updates their role.
func (h *ClawAuthHandler) autoLinkPartner(clawID string, user *model.User) {
	db := database.DB

	// Check core partner whitelist
	var cp model.CorePartner
	if err := db.Where("claw_id = ? AND status = ?", clawID, "active").First(&cp).Error; err == nil {
		if cp.UserID == "" || cp.UserID != user.ID {
			db.Model(&cp).Updates(map[string]interface{}{"user_id": user.ID})
		}
		if user.Role != "partner" && user.Role != "admin" {
			db.Model(user).Update("role", "partner")
			user.Role = "partner"
		}
		log.Printf("[claw-auth] Auto-linked core partner %s → user %s", clawID, user.ID)
		return
	}

	// Check city partner whitelist
	var city model.CityPartner
	if err := db.Where("claw_id = ? AND status = ?", clawID, "approved").First(&city).Error; err == nil {
		if city.UserID == "" || city.UserID != user.ID {
			db.Model(&city).Updates(map[string]interface{}{"user_id": user.ID})
		}
		if user.Role != "city" && user.Role != "partner" && user.Role != "admin" {
			db.Model(user).Update("role", "city")
			user.Role = "city"
		}
		log.Printf("[claw-auth] Auto-linked city partner %s → user %s", clawID, user.ID)
	}
}

// findOrCreateClawUser finds a user by their Claw node binding, or creates a new one.
func (h *ClawAuthHandler) findOrCreateClawUser(clawID, publicKey string) *model.User {
	db := database.DB

	// First, check if there's an existing NodeBinding for this claw_id
	var binding model.NodeBinding
	if err := db.Where("node_id = ? AND status = ?", clawID, "active").First(&binding).Error; err == nil {
		// Found binding — load the linked user
		var user model.User
		if err := db.Where("id = ?", binding.QueenUserID).First(&user).Error; err == nil {
			return &user
		}
	}

	// Check if there's a user with this claw_id as oauth_id (from previous claw login)
	var user model.User
	if err := db.Where("o_auth_provider = ? AND o_auth_id = ?", "claw", clawID).First(&user).Error; err == nil {
		return &user
	}

	// Create new user with claw identity
	shortID := clawID
	if len(shortID) > 14 {
		shortID = shortID[:14] + "\u2026"
	}
	// Use claw_id as unique email placeholder to avoid uniqueIndex conflict
	clawEmail := clawID + "@claw.local"
	user = model.User{
		ID:            uuid.New().String(),
		Email:         clawEmail,
		Nickname:      shortID,
		Role:          "user",
		Status:        "active",
		OAuthProvider: "claw",
		OAuthID:       clawID,
	}
	if err := db.Create(&user).Error; err != nil {
		log.Printf("[claw-auth] Failed to create user for %s: %v", clawID, err)
		return nil
	}

	// Also create a NodeBinding so the user is linked to this Claw
	binding = model.NodeBinding{
		ID:          uuid.New().String(),
		NodeID:      clawID,
		QueenUserID: user.ID,
		Status:      "active",
	}
	db.Create(&binding)

	log.Printf("[claw-auth] Created new user %s for claw %s", user.ID, clawID)
	return &user
}
