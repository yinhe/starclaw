package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ---------- limiter interface ----------

type limiter interface {
	allow(key string) (bool, int)
}

// ---------- in-memory fallback ----------

type visitor struct {
	count       int
	windowStart time.Time
}

type memLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	max      int
	window   time.Duration
}

func newMemLimiter(max int, window time.Duration) *memLimiter {
	ml := &memLimiter{
		visitors: make(map[string]*visitor),
		max:      max,
		window:   window,
	}
	go ml.cleanup()
	return ml
}

func (ml *memLimiter) cleanup() {
	for {
		time.Sleep(ml.window)
		ml.mu.Lock()
		for k, v := range ml.visitors {
			if time.Since(v.windowStart) > ml.window {
				delete(ml.visitors, k)
			}
		}
		ml.mu.Unlock()
	}
}

func (ml *memLimiter) allow(key string) (bool, int) {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	v, exists := ml.visitors[key]
	if !exists || time.Since(v.windowStart) > ml.window {
		ml.visitors[key] = &visitor{count: 1, windowStart: time.Now()}
		return true, ml.max - 1
	}

	v.count++
	remaining := ml.max - v.count
	if remaining < 0 {
		remaining = 0
	}
	return v.count <= ml.max, remaining
}

// ---------- Redis-based limiter (distributed) ----------

type redisLimiter struct {
	rdb    *redis.Client
	max    int
	window time.Duration
	prefix string
}

func newRedisLimiter(rdb *redis.Client, max int, window time.Duration, prefix string) *redisLimiter {
	return &redisLimiter{rdb: rdb, max: max, window: window, prefix: prefix}
}

func (rl *redisLimiter) allow(key string) (bool, int) {
	ctx := context.Background()
	rkey := fmt.Sprintf("rl:%s:%s", rl.prefix, key)

	count, err := rl.rdb.Incr(ctx, rkey).Result()
	if err != nil {
		log.Printf("[ratelimit] redis error: %v, allowing request", err)
		return true, rl.max
	}
	if count == 1 {
		rl.rdb.Expire(ctx, rkey, rl.window)
	}

	remaining := rl.max - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return int(count) <= rl.max, remaining
}

// ---------- factory ----------

func newLimiter(rdb *redis.Client, max int, window time.Duration, prefix string) limiter {
	if rdb != nil {
		return newRedisLimiter(rdb, max, window, prefix)
	}
	return newMemLimiter(max, window)
}

// ---------- Gin middlewares ----------

// RateLimit applies per-IP rate limiting.
// Pass nil for rdb to use in-memory fallback.
func RateLimit(maxRequests int, window time.Duration, rdb ...*redis.Client) gin.HandlerFunc {
	var r *redis.Client
	if len(rdb) > 0 {
		r = rdb[0]
	}
	l := newLimiter(r, maxRequests, window, "ip")
	return func(c *gin.Context) {
		key := c.ClientIP()
		allowed, remaining := l.allow(key)
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

// UserRateLimit applies per-user rate limiting (requires AuthRequired middleware first).
// Pass nil for rdb to use in-memory fallback.
func UserRateLimit(maxRequests int, window time.Duration, rdb ...*redis.Client) gin.HandlerFunc {
	var r *redis.Client
	if len(rdb) > 0 {
		r = rdb[0]
	}
	l := newLimiter(r, maxRequests, window, "user")
	return func(c *gin.Context) {
		key := c.GetString("user_id")
		if key == "" {
			key = c.ClientIP()
		}
		allowed, remaining := l.allow(key)
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
