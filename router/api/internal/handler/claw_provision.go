package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

// ClawProvisionHandler handles API key provisioning for Claw instances.
// Claw authenticates via Ed25519 signature, gets an API key bound 1:1 to its claw_id.
type ClawProvisionHandler struct {
	db *gorm.DB
}

func NewClawProvisionHandler(db *gorm.DB) *ClawProvisionHandler {
	return &ClawProvisionHandler{db: db}
}

// POST /v1/claw/provision — auto-provision API key for a Claw instance.
// Requires Claw Ed25519 signature auth (X-Claw-* headers).
// Returns existing active key or generates a new one.
func (h *ClawProvisionHandler) Provision(c *gin.Context) {
	clawID := c.GetString("claw_id")
	if clawID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "claw_id required (use Ed25519 signature auth)"})
		return
	}

	// Find or create user for this claw_id
	user := h.findOrCreateUser(clawID)
	if user == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user account"})
		return
	}

	// Check for existing active API key bound to this claw_id
	var existingKey model.APIKey
	err := h.db.Where("claw_id = ? AND is_enabled = ?", clawID, true).First(&existingKey).Error
	if err == nil {
		// Key exists and is active — return info (but NOT the key itself, it was shown only once)
		c.JSON(http.StatusOK, gin.H{
			"status":     "existing",
			"key_prefix": existingKey.KeyPrefix,
			"key_id":     existingKey.ID,
			"user_id":    user.ID,
			"claw_id":    clawID,
			"balance":    user.Balance,
			"message":    "API key already provisioned. Use rotate-key if you need a new one.",
		})
		return
	}

	// Generate new API key
	plainKey := model.GenerateAPIKey()
	keyHash := sha256Hash(plainKey)
	prefix := plainKey[:16] // "sk-star-a1b2c3d4"

	shortID := clawID
	if len(shortID) > 18 {
		shortID = shortID[:18] + "…"
	}

	apiKey := model.APIKey{
		UserID:    user.ID,
		ClawID:    clawID,
		Name:      "Claw Auto (" + shortID + ")",
		KeyHash:   keyHash,
		KeyPrefix: prefix,
		KeyEnc:    model.EncryptKey(plainKey),
		IsEnabled: true,
	}
	if err := h.db.Create(&apiKey).Error; err != nil {
		log.Printf("[claw-provision] failed to create API key for %s: %v", clawID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
		return
	}

	log.Printf("[claw-provision] new API key provisioned: %s → user %s, key %s", clawID, user.ID, prefix)

	c.JSON(http.StatusOK, gin.H{
		"status":     "created",
		"api_key":    plainKey,
		"key_prefix": prefix,
		"key_id":     apiKey.ID,
		"user_id":    user.ID,
		"claw_id":    clawID,
		"balance":    user.Balance,
		"message":    "API key created and bound to your Claw instance.",
	})
}

// POST /v1/claw/rotate-key — revoke old key, generate new one.
// Requires Claw Ed25519 signature auth.
func (h *ClawProvisionHandler) RotateKey(c *gin.Context) {
	clawID := c.GetString("claw_id")
	if clawID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "claw_id required"})
		return
	}

	// Find user
	var user model.User
	if err := h.db.Where("claw_id = ?", clawID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no account found for this Claw instance"})
		return
	}

	// Revoke all existing keys for this claw_id
	now := time.Now()
	h.db.Model(&model.APIKey{}).
		Where("claw_id = ? AND is_enabled = ?", clawID, true).
		Updates(map[string]interface{}{"is_enabled": false, "deleted_at": now})

	// Generate new key
	plainKey := model.GenerateAPIKey()
	keyHash := sha256Hash(plainKey)
	prefix := plainKey[:16]

	shortID := clawID
	if len(shortID) > 18 {
		shortID = shortID[:18] + "…"
	}

	apiKey := model.APIKey{
		UserID:    user.ID,
		ClawID:    clawID,
		Name:      "Claw Auto (" + shortID + ")",
		KeyHash:   keyHash,
		KeyPrefix: prefix,
		KeyEnc:    model.EncryptKey(plainKey),
		IsEnabled: true,
	}
	if err := h.db.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create new API key"})
		return
	}

	log.Printf("[claw-provision] key rotated for %s → new key %s", clawID, prefix)

	c.JSON(http.StatusOK, gin.H{
		"status":     "rotated",
		"api_key":    plainKey,
		"key_prefix": prefix,
		"key_id":     apiKey.ID,
		"user_id":    user.ID,
		"claw_id":    clawID,
		"balance":    user.Balance,
		"message":    "Old key revoked, new key generated.",
	})
}

// GET /v1/claw/sync — check key status, balance, account info.
// Requires Claw Ed25519 signature auth.
func (h *ClawProvisionHandler) Sync(c *gin.Context) {
	clawID := c.GetString("claw_id")
	if clawID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "claw_id required"})
		return
	}

	var user model.User
	if err := h.db.Where("claw_id = ?", clawID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no account found"})
		return
	}

	// Find active key
	var activeKey model.APIKey
	hasKey := h.db.Where("claw_id = ? AND is_enabled = ?", clawID, true).First(&activeKey).Error == nil

	// Check if key was rotated via web (old key revoked, new key exists but Claw doesn't have it)
	needsRotation := false
	if !hasKey {
		// No active key — might have been revoked via web UI
		needsRotation = true
	}

	resp := gin.H{
		"user_id":        user.ID,
		"claw_id":        clawID,
		"balance":        user.Balance,
		"has_active_key": hasKey,
		"needs_rotation": needsRotation,
	}
	if hasKey {
		resp["key_prefix"] = activeKey.KeyPrefix
		resp["key_id"] = activeKey.ID
	}

	c.JSON(http.StatusOK, resp)
}

func (h *ClawProvisionHandler) findOrCreateUser(clawID string) *model.User {
	var user model.User
	if err := h.db.Where("claw_id = ?", clawID).First(&user).Error; err == nil {
		return &user
	}

	// Build display name from claw_id
	shortID := clawID
	if len(shortID) > 14 {
		shortID = shortID[:14] + "…"
	}

	idHash := sha256.Sum256([]byte(clawID))
	placeholder := hex.EncodeToString(idHash[:6])

	user = model.User{
		Name:      shortID,
		Email:     placeholder + "@claw.local",
		Phone:     "c" + placeholder,
		ClawID:    clawID,
		FreeQuota: 0,
		Status:    "active",
	}
	if err := h.db.Create(&user).Error; err != nil {
		// Might be duplicate — try to find again
		if h.db.Where("claw_id = ?", clawID).First(&user).Error == nil {
			return &user
		}
		log.Printf("[claw-provision] failed to create user for %s: %v", clawID, err)
		return nil
	}
	log.Printf("[claw-provision] created user %s for claw %s", user.ID, clawID)
	return &user
}

func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// hashAPIKey is exported for use by other handlers
func HashAPIKey(key string) string {
	return sha256Hash(key)
}

// StripKeyPrefix extracts the display prefix from a full API key
func StripKeyPrefix(key string) string {
	if len(key) > 16 {
		return key[:16]
	}
	return key
}

// IsClawBoundKey checks if an API key path indicates it's for key sync
func IsClawProvisionPath(path string) bool {
	return strings.HasPrefix(path, "/v1/claw/")
}
