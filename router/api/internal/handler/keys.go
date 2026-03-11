package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

type KeysHandler struct {
	db *gorm.DB
}

func NewKeysHandler(db *gorm.DB) *KeysHandler {
	return &KeysHandler{db: db}
}

// List returns all API keys for the authenticated user (keys are masked)
func (h *KeysHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")

	var keys []model.APIKey
	h.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys)

	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// Create generates a new API key
func (h *KeysHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req)
	if req.Name == "" {
		req.Name = "Default"
	}

	// Generate key
	rawKey := model.GenerateAPIKey()

	// Store hash only
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	apiKey := model.APIKey{
		UserID:    userID,
		Name:      req.Name,
		KeyHash:   keyHash,
		KeyPrefix: rawKey[:16] + "...", // "sk-star-a1b2c3d4..."
		IsEnabled: true,
	}

	if err := h.db.Create(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create key"})
		return
	}

	// Return the raw key ONCE — it cannot be retrieved again
	c.JSON(http.StatusCreated, gin.H{
		"key":        rawKey,
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key_prefix": apiKey.KeyPrefix,
		"message":    "Save this key — it will not be shown again",
	})
}

// Delete revokes an API key
func (h *KeysHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id")
	keyID := c.Param("id")

	result := h.db.Where("id = ? AND user_id = ?", keyID, userID).Delete(&model.APIKey{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "key deleted"})
}
