package inference

import (
	"log"
	"sort"
	"sync"
	"time"
)

// MinerInfo represents a registered miner node and its capabilities.
type MinerInfo struct {
	NodeID      string   `json:"node_id"`
	PublicKey   string   `json:"public_key"`
	Address     string   `json:"address"`      // base URL e.g. "http://10.0.0.5:8080"
	Models      []string `json:"models"`       // supported model names
	MaxTokens   int      `json:"max_tokens"`   // max tokens per request
	GPUMemoryMB int      `json:"gpu_memory_mb"`
	ActiveJobs  int      `json:"active_jobs"`  // current concurrent requests
	MaxJobs     int      `json:"max_jobs"`     // max concurrent requests
	Region      string   `json:"region"`
	Status      string   `json:"status"`       // "online", "busy", "offline"
	LastSeen    int64    `json:"last_seen"`    // Unix timestamp
	Latency     int64    `json:"latency_ms"`   // avg response latency in ms
	TotalServed int64    `json:"total_served"` // lifetime requests served
}

// MinerRegistry tracks all known miner nodes.
type MinerRegistry struct {
	miners map[string]*MinerInfo // node_id -> info
	mu     sync.RWMutex
}

// NewMinerRegistry creates an empty registry.
func NewMinerRegistry() *MinerRegistry {
	r := &MinerRegistry{
		miners: make(map[string]*MinerInfo),
	}
	// Start background reaper for stale miners
	go r.reapLoop()
	return r
}

// Register adds or updates a miner in the registry.
func (r *MinerRegistry) Register(info *MinerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastSeen = time.Now().Unix()
	if info.Status == "" {
		info.Status = "online"
	}
	if info.MaxJobs <= 0 {
		info.MaxJobs = 1
	}
	r.miners[info.NodeID] = info
	log.Printf("[inference/registry] registered miner %s (%s) models=%v jobs=%d/%d",
		info.NodeID[:16], info.Address, info.Models, info.ActiveJobs, info.MaxJobs)
}

// Heartbeat updates a miner's status and active job count.
func (r *MinerRegistry) Heartbeat(nodeID string, activeJobs int, latencyMs int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.miners[nodeID]
	if !ok {
		return false
	}
	m.LastSeen = time.Now().Unix()
	m.ActiveJobs = activeJobs
	m.Status = "online"
	if latencyMs > 0 {
		// Exponential moving average
		m.Latency = (m.Latency*7 + latencyMs*3) / 10
	}
	return true
}

// Unregister removes a miner from the registry.
func (r *MinerRegistry) Unregister(nodeID string) {
	r.mu.Lock()
	delete(r.miners, nodeID)
	r.mu.Unlock()
	log.Printf("[inference/registry] unregistered miner %s", nodeID[:min(16, len(nodeID))])
}

// SelectMiner picks the best available miner for a given model.
// Strategy: filter by model → filter online + has capacity → sort by (load ratio, latency).
func (r *MinerRegistry) SelectMiner(model string) *MinerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*MinerInfo
	for _, m := range r.miners {
		if m.Status == "offline" {
			continue
		}
		if m.ActiveJobs >= m.MaxJobs {
			continue // at capacity
		}
		// Check model support
		for _, supported := range m.Models {
			if supported == model || supported == "*" {
				candidates = append(candidates, m)
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Sort by load ratio (ascending), then latency (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		loadI := float64(candidates[i].ActiveJobs) / float64(candidates[i].MaxJobs)
		loadJ := float64(candidates[j].ActiveJobs) / float64(candidates[j].MaxJobs)
		if loadI != loadJ {
			return loadI < loadJ
		}
		return candidates[i].Latency < candidates[j].Latency
	})

	return candidates[0]
}

// ListMiners returns all registered miners.
func (r *MinerRegistry) ListMiners() []*MinerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*MinerInfo, 0, len(r.miners))
	for _, m := range r.miners {
		copy := *m
		result = append(result, &copy)
	}
	return result
}

// Stats returns aggregate registry statistics.
func (r *MinerRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	online, busy, offline := 0, 0, 0
	totalJobs, totalCapacity := 0, 0
	modelSet := make(map[string]int)

	for _, m := range r.miners {
		switch m.Status {
		case "online":
			online++
		case "busy":
			busy++
		default:
			offline++
		}
		totalJobs += m.ActiveJobs
		totalCapacity += m.MaxJobs
		for _, model := range m.Models {
			modelSet[model]++
		}
	}

	return map[string]interface{}{
		"total_miners":  len(r.miners),
		"online":        online,
		"busy":          busy,
		"offline":       offline,
		"active_jobs":   totalJobs,
		"total_capacity": totalCapacity,
		"models":        modelSet,
	}
}

// reapLoop marks miners as offline if they haven't sent a heartbeat recently.
func (r *MinerRegistry) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now().Unix()
		for id, m := range r.miners {
			if now-m.LastSeen > 120 && m.Status != "offline" {
				m.Status = "offline"
				log.Printf("[inference/registry] miner %s marked offline (no heartbeat for %ds)", id[:min(16, len(id))], now-m.LastSeen)
			}
			// Remove miners offline for > 1 hour
			if now-m.LastSeen > 3600 {
				delete(r.miners, id)
				log.Printf("[inference/registry] reaped stale miner %s", id[:min(16, len(id))])
			}
		}
		r.mu.Unlock()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
