package browser

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// ScreenshotCache stores screenshots in memory with TTL
type ScreenshotCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

type cacheItem struct {
	Data      []byte
	MimeType  string
	CreatedAt time.Time
}

var globalCache = &ScreenshotCache{
	items: make(map[string]*cacheItem),
}

func init() {
	// Cleanup expired screenshots every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			globalCache.cleanup()
		}
	}()
}

// GetCache returns the global screenshot cache
func GetCache() *ScreenshotCache {
	return globalCache
}

// Store saves a screenshot and returns its ID
func (c *ScreenshotCache) Store(data []byte, mimeType string) string {
	id := uuid.New().String()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[id] = &cacheItem{
		Data:      data,
		MimeType:  mimeType,
		CreatedAt: time.Now(),
	}
	return id
}

// Get retrieves a screenshot by ID
func (c *ScreenshotCache) Get(id string) ([]byte, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[id]
	if !ok {
		return nil, "", false
	}
	return item.Data, item.MimeType, true
}

// cleanup removes screenshots older than 30 minutes
func (c *ScreenshotCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := time.Now().Add(-30 * time.Minute)
	for id, item := range c.items {
		if item.CreatedAt.Before(cutoff) {
			delete(c.items, id)
		}
	}
}
