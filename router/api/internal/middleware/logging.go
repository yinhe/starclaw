package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs each API request with timing and user info
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		userID := c.GetString("user_id")
		status := c.Writer.Status()

		log.Printf("[star-ai] %s %s → %d (%v) user=%s",
			c.Request.Method, path, status, duration.Round(time.Millisecond), userID)
	}
}
