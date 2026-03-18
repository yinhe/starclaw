package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides a Redis-backed caching layer for hot query paths.
type Cache struct {
	rdb *redis.Client
}

// NewCache creates a cache backed by the given Redis client.
// Returns nil if rdb is nil (caching disabled gracefully).
func NewCache(rdb *redis.Client) *Cache {
	if rdb == nil {
		return nil
	}
	return &Cache{rdb: rdb}
}

// Get retrieves a cached value and unmarshals it into dest.
// Returns false if not found or on error.
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) bool {
	if c == nil {
		return false
	}
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, dest); err != nil {
		log.Printf("[Cache] unmarshal %s: %v", key, err)
		return false
	}
	return true
}

// Set stores a value in cache with the given TTL.
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if c == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	c.rdb.Set(ctx, key, data, ttl)
}

// Delete removes a key from cache.
func (c *Cache) Delete(ctx context.Context, keys ...string) {
	if c == nil {
		return
	}
	c.rdb.Del(ctx, keys...)
}

// InvalidatePrefix removes all keys matching a prefix pattern.
func (c *Cache) InvalidatePrefix(ctx context.Context, prefix string) {
	if c == nil {
		return
	}
	iter := c.rdb.Scan(ctx, 0, prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		c.rdb.Del(ctx, keys...)
	}
}

// ── Key builders ──

func KeyAgentList(userID string) string {
	return fmt.Sprintf("cache:agents:user:%s", userID)
}

func KeyAgent(agentID string) string {
	return fmt.Sprintf("cache:agent:%s", agentID)
}

func KeyConversationList(userID string) string {
	return fmt.Sprintf("cache:convs:user:%s", userID)
}

func KeyModelList() string {
	return "cache:models:all"
}

func KeyDashboardStats(userID string) string {
	return fmt.Sprintf("cache:dashboard:%s", userID)
}

func KeyObserveStats(userID string) string {
	return fmt.Sprintf("cache:observe:stats:%s", userID)
}

func KeyWebhookRules(userID string) string {
	return fmt.Sprintf("cache:webhook:rules:%s", userID)
}

func KeyPluginList(category string) string {
	return fmt.Sprintf("cache:plugins:%s", category)
}

// ── TTL presets ──

const (
	TTLShort  = 30 * time.Second  // volatile data (stats, counts)
	TTLMedium = 5 * time.Minute   // semi-stable (agent lists, model lists)
	TTLLong   = 30 * time.Minute  // stable data (plugin catalog)
)
