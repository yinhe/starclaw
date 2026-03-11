package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLogger logs structured request/response info for every API call
func AccessLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		userID := c.GetString("user_id")

		if query != "" {
			path = path + "?" + query
		}

		// Color status for readability
		var level string
		switch {
		case status >= 500:
			level = "ERROR"
		case status >= 400:
			level = "WARN"
		default:
			level = "INFO"
		}

		if userID != "" {
			log.Printf("[api] %s | %d | %12v | %s | %-7s %s | user=%s",
				level, status, latency, clientIP, method, path, userID)
		} else {
			log.Printf("[api] %s | %d | %12v | %s | %-7s %s",
				level, status, latency, clientIP, method, path)
		}
	}
}
