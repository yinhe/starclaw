package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit limits requests per API key using Redis sliding window
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyID := c.GetString("api_key_id")
		if keyID == "" {
			c.Next()
			return
		}

		ctx := context.Background()
		redisKey := fmt.Sprintf("ratelimit:%s", keyID)
		now := time.Now().UnixMilli()

		pipe := rdb.Pipeline()
		// Remove old entries outside window
		pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", now-window.Milliseconds()))
		// Count current entries
		countCmd := pipe.ZCard(ctx, redisKey)
		// Add current request
		pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: now})
		// Set expiry on the key
		pipe.Expire(ctx, redisKey, window+time.Second)

		if _, err := pipe.Exec(ctx); err != nil {
			// Redis error — allow request through (fail open)
			c.Next()
			return
		}

		count := countCmd.Val()
		remaining := int64(maxRequests) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if count >= int64(maxRequests) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"message": "rate limit exceeded",
					"type":    "rate_limit_error",
				},
			})
			return
		}

		c.Next()
	}
}
