package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs structured request info: method, path, status, latency, user
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		userID := c.GetString("user_id")
		if userID == "" {
			userID = "-"
		}

		log.Printf("[%s] %s %s | %d | %v | user=%s | ip=%s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Request.URL.RawQuery,
			c.Writer.Status(),
			latency.Round(time.Millisecond),
			userID,
			c.ClientIP(),
		)
	}
}
