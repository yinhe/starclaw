package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/synapse/api/internal/model"
	"gorm.io/gorm"
)

// APIKeyAuth validates sk-star-xxx API keys from Authorization header
func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "missing Authorization header",
					"type":    "authentication_error",
				},
			})
			return
		}

		// Support both "Bearer sk-star-xxx" and raw "sk-star-xxx"
		key := strings.TrimPrefix(auth, "Bearer ")
		key = strings.TrimSpace(key)

		if !strings.HasPrefix(key, "sk-star-") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "invalid API key format, expected sk-star-xxx",
					"type":    "authentication_error",
				},
			})
			return
		}

		// Hash the key and look up in DB
		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		var apiKey model.APIKey
		if err := db.Where("key_hash = ? AND is_enabled = ?", keyHash, true).First(&apiKey).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "invalid or disabled API key",
					"type":    "authentication_error",
				},
			})
			return
		}

		// Update last used (async, don't block request)
		go func() {
			now := time.Now()
			db.Model(&apiKey).Update("last_used", &now)
		}()

		// Set user context
		c.Set("user_id", apiKey.UserID)
		c.Set("api_key_id", apiKey.ID)
		c.Next()
	}
}
