package middleware

import (
	"github.com/gin-gonic/gin"
	"starclaw.net/nydus/api/internal/config"
)

// SecretAuth validates the X-Nydus-Secret header or ?secret query param.
func SecretAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Nydus-Secret")
		if token == "" {
			token = c.Query("secret")
		}
		if token != config.C.Server.Secret {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// WormAuth validates the X-Nydus-Secret header for Worm agent.
func WormAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Nydus-Secret")
		if token != config.W.Secret {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
