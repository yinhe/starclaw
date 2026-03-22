package middleware

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// RequestLogger logs each API request as structured JSON
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		userID := c.GetString("user_id")
		clawID := c.GetString("claw_id")
		authType := c.GetString("auth_type")

		attrs := []slog.Attr{
			slog.String("service", "star-ai"),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.Int("size", c.Writer.Size()),
			slog.String("client_ip", c.ClientIP()),
		}
		if authType != "" {
			attrs = append(attrs, slog.String("auth_type", authType))
		}
		if userID != "" {
			attrs = append(attrs, slog.String("user_id", userID))
		}
		if clawID != "" {
			attrs = append(attrs, slog.String("claw_id", clawID))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		logger.LogAttrs(c.Request.Context(), level, "http_request", attrs...)
	}
}
