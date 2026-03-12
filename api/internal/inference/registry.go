package inference

import (
	"log"
	"sort"
	"sync"
	"time"
)

// ContributorInfo represents a registered compute contributor node and its capabilities.
type ContributorInfo struct {
	NodeID      string      `json:"node_id"`
	PublicKey   string      `json:"public_key"`
	Address     string      `json:"address"`    // base URL e.g. "http://10.0.0.5:8080"
	Models      []string    `json:"models"`     // supported model names
	MaxTokens   int         `json:"max_tokens"` // max tokens per request
	GPUMemoryMB int         `json:"gpu_memory_mb"`
	ActiveJobs  int         `json:"active_jobs"` // current concurrent requests
	MaxJobs     int         `json:"max_jobs"`    // max concurrent requests
	Region      string      `json:"region"`
	Status      string      `json:"status"`       // "online", "busy", "offline"
	LastSeen    int64       `json:"last_seen"`    // Unix timestamp
	Latency     int64       `json:"latency_ms"`   // avg response latency in ms
	TotalServed int64       `json:"total_served"` // lifetime requests served
	Trust       *TrustScore `json:"trust"`        // reputation score
}

// ContributorRegistry tracks all known compute contributor nodes.
type ContributorRegistry struct {
	contributors map[string]*ContributorInfo // node_id -> info
	mu           sync.RWMutex
}

// NewContributorRegistry creates an empty registry.
func NewContributorRegistry() *ContributorRegistry {
	r := &ContributorRegistry{
		contributors: make(map[string]*ContributorInfo),
	}
	// Start background reaper for stale contributors
	go r.reapLoop()
	return r
}

// Register adds or updates a contributor in the registry.
func (r *ContributorRegistry) Register(info *ContributorInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastSeen = time.Now().Unix()
	if info.Status == "" {
		info.Status = "online"
	}
	if info.MaxJobs <= 0 {
		info.MaxJobs = 1
	}
	// Preserve existing trust score on re-registration
	if existing, ok := r.contributors[info.NodeID]; ok && existing.Trust != nil {
		info.Trust = existing.Trust
	}
	if info.Trust == nil {
		info.Trust = DefaultTrustScore()
	}
	r.contributors[info.NodeID] = info
	log.Printf("[inference/registry] registered contributor %s (%s) models=%v jobs=%d/%d",
		info.NodeID[:16], info.Address, info.Models, info.ActiveJobs, info.MaxJobs)
}

// Heartbeat updates a contributor's status and active job count.
func (r *ContributorRegistry) Heartbeat(nodeID string, activeJobs int, latencyMs int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.contributors[nodeID]
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
	if m.Trust != nil {
		m.Trust.RecordHeartbeat()
	}
	return true
}

// Unregister removes a contributor from the registry.
func (r *ContributorRegistry) Unregister(nodeID string) {
	r.mu.Lock()
	delete(r.contributors, nodeID)
	r.mu.Unlock()
	log.Printf("[inference/registry] unregistered contributor %s", nodeID[:min(16, len(nodeID))])
}

// SelectContributor picks the best available contributor for a given model.
// Strategy: filter by model → filter online + has capacity → trust-weighted sort.
func (r *ContributorRegistry) SelectContributor(model string) *ContributorInfo {
	return r.SelectContributorWithTrust(model, TrustAny)
}

// SelectContributorWithTrust picks the best contributor that meets the minimum trust level.
// Scoring formula: score = (1 - loadRatio)*0.3 + (1 - latencyNorm)*0.2 + trustComposite*0.5
func (r *ContributorRegistry) SelectContributorWithTrust(model string, minTrust TrustLevel) *ContributorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*ContributorInfo
	for _, m := range r.contributors {
		if m.Status == "offline" {
			continue
		}
		if m.ActiveJobs >= m.MaxJobs {
			continue // at capacity
		}
		// Check trust level
		if m.Trust != nil && !m.Trust.MeetsLevel(minTrust) {
			continue
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

	// Trust-weighted scoring: lower score = better
	sort.Slice(candidates, func(i, j int) bool {
		return contributorScore(candidates[i]) > contributorScore(candidates[j])
	})

	return candidates[0]
}

// contributorScore computes a composite selection score (higher = better).
func contributorScore(c *ContributorInfo) float64 {
	// Load component: less loaded = higher score
	loadScore := 1.0 - float64(c.ActiveJobs)/float64(c.MaxJobs)

	// Latency component: lower latency = higher score (normalize to 0-1, cap at 10s)
	latencyScore := 1.0
	if c.Latency > 0 {
		latencyScore = 1.0 - float64(min64(c.Latency, 10000))/10000.0
	}

	// Trust component
	trustScore := 0.5 // default for new contributors
	if c.Trust != nil {
		trustScore = c.Trust.Composite
	}

	return loadScore*0.30 + latencyScore*0.20 + trustScore*0.50
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ListContributors returns all registered contributors.
func (r *ContributorRegistry) ListContributors() []*ContributorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ContributorInfo, 0, len(r.contributors))
	for _, m := range r.contributors {
		copy := *m
		result = append(result, &copy)
	}
	return result
}

// Stats returns aggregate registry statistics.
func (r *ContributorRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	online, busy, offline := 0, 0, 0
	totalJobs, totalCapacity := 0, 0
	modelSet := make(map[string]int)

	for _, m := range r.contributors {
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
		"total_contributors": len(r.contributors),
		"online":             online,
		"busy":               busy,
		"offline":            offline,
		"active_jobs":        totalJobs,
		"total_capacity":     totalCapacity,
		"models":             modelSet,
	}
}

// reapLoop marks contributors as offline if they haven't sent a heartbeat recently.
func (r *ContributorRegistry) reapLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now().Unix()
		for id, m := range r.contributors {
			if now-m.LastSeen > 120 && m.Status != "offline" {
				m.Status = "offline"
				log.Printf("[inference/registry] contributor %s marked offline (no heartbeat for %ds)", id[:min(16, len(id))], now-m.LastSeen)
			}
			// Remove contributors offline for > 1 hour
			if now-m.LastSeen > 3600 {
				delete(r.contributors, id)
				log.Printf("[inference/registry] reaped stale contributor %s", id[:min(16, len(id))])
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
