package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count       int
	windowStart time.Time
}

type rateLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	max      int
	window   time.Duration
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitor),
		max:      max,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(rl.window)
		rl.mu.Lock()
		for k, v := range rl.visitors {
			if time.Since(v.windowStart) > rl.window {
				delete(rl.visitors, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(key string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[key]
	if !exists || time.Since(v.windowStart) > rl.window {
		rl.visitors[key] = &visitor{count: 1, windowStart: time.Now()}
		return true, rl.max - 1
	}

	v.count++
	remaining := rl.max - v.count
	if remaining < 0 {
		remaining = 0
	}
	return v.count <= rl.max, remaining
}

// RateLimit applies per-IP rate limiting
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	rl := newRateLimiter(maxRequests, window)
	return func(c *gin.Context) {
		key := c.ClientIP()
		allowed, remaining := rl.allow(key)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// UserRateLimit applies per-user rate limiting (requires AuthRequired middleware first)
func UserRateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	rl := newRateLimiter(maxRequests, window)
	return func(c *gin.Context) {
		key := c.GetString("user_id")
		if key == "" {
			key = c.ClientIP()
		}
		allowed, remaining := rl.allow(key)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded, please slow down"})
			c.Abort()
			return
		}
		c.Next()
	}
}
