package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// bucket tracks request counts per sliding window
type bucket struct {
	count    int
	resetAt  time.Time
}

// RateLimiter is an in-memory sliding-window rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    int           // max requests per window
	window   time.Duration // window size
	lastGC   time.Time
}

// NewRateLimiter creates a rate limiter with given limit per window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
		lastGC:  time.Now(),
	}
}

// allow checks if the key is within rate limit
func (rl *RateLimiter) allow(key string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Periodic GC: clean expired buckets every 5 minutes
	if now.Sub(rl.lastGC) > 5*time.Minute {
		for k, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, k)
			}
		}
		rl.lastGC = now
	}

	b, exists := rl.buckets[key]
	if !exists || now.After(b.resetAt) {
		rl.buckets[key] = &bucket{count: 1, resetAt: now.Add(rl.window)}
		return true, rl.limit - 1, now.Add(rl.window)
	}

	if b.count >= rl.limit {
		return false, 0, b.resetAt
	}

	b.count++
	return true, rl.limit - b.count, b.resetAt
}

// Middleware returns a Gin middleware that rate-limits by client IP
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()

		// If authenticated, use user_id for finer-grained limiting
		if uid := c.GetString("user_id"); uid != "" {
			key = "u:" + uid
		}

		allowed, remaining, resetAt := rl.allow(key)
		c.Header("X-RateLimit-Limit", intToStr(rl.limit))
		c.Header("X-RateLimit-Remaining", intToStr(remaining))
		c.Header("X-RateLimit-Reset", resetAt.Format(time.RFC3339))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, APIResponse{
				Code:    CodeRateLimited,
				Message: ErrMsg[CodeRateLimited],
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// UserRateLimit applies rate limiting keyed by user_id only (for write endpoints)
func (rl *RateLimiter) UserRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.GetString("user_id")
		if uid == "" {
			c.Next()
			return
		}

		key := "w:" + uid
		allowed, remaining, resetAt := rl.allow(key)
		c.Header("X-RateLimit-Limit", intToStr(rl.limit))
		c.Header("X-RateLimit-Remaining", intToStr(remaining))
		c.Header("X-RateLimit-Reset", resetAt.Format(time.RFC3339))

		if !allowed {
			c.JSON(http.StatusTooManyRequests, APIResponse{
				Code:    CodeRateLimited,
				Message: ErrMsg[CodeRateLimited],
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func intToStr(n int) string {
	if n < 0 {
		n = 0
	}
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
