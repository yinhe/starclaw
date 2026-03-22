package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DualAuth accepts either:
//  1. API Key auth (Authorization: Bearer sk-star-xxx) — sets auth_type="api_key", user_id, api_key_id
//  2. Claw Ed25519 signature (X-Claw-ID + X-Claw-PubKey + X-Claw-Signature + X-Claw-Timestamp)
//     — sets auth_type="claw", claw_id, claw_pubkey
//
// If neither is present, returns 401.
func DualAuth(db *gorm.DB) gin.HandlerFunc {
	apiKeyHandler := APIKeyAuth(db)
	clawSigHandler := ClawSignatureAuth()

	return func(c *gin.Context) {
		// Check if request has Claw signature headers
		if HasClawSignature(c) {
			clawSigHandler(c)
			return
		}

		// Check if request has API key
		auth := c.GetHeader("Authorization")
		key := strings.TrimPrefix(auth, "Bearer ")
		key = strings.TrimSpace(key)
		if strings.HasPrefix(key, "sk-star-") {
			// Mark as API key auth type
			apiKeyHandler(c)
			if !c.IsAborted() {
				c.Set("auth_type", "api_key")
			}
			return
		}

		// Neither auth method provided
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "authentication required: provide API key (Authorization: Bearer sk-star-xxx) or Claw signature (X-Claw-ID + X-Claw-Signature headers)",
				"type":    "authentication_error",
			},
		})
	}
}
