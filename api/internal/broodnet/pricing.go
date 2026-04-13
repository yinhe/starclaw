package broodnet

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ════════════════════════════════════════════════════════════
// BroodOS Network — PricingEngine (动态星能定价)
//
// Computes fair star energy prices for tasks based on:
//   - Base cost: category-dependent baseline
//   - Supply/demand: ratio of open tasks to available bidders
//   - Capability multiplier: GPU, high-memory, specialized skills
//   - Urgency surcharge: time-sensitive tasks pay more
//   - Reputation discount: high-rep nodes get better rates
//   - Historical baseline: rolling average of settled prices
//
// Used by:
//   - TaskMarket: suggest budget when posting, validate bids
//   - Orchestrator: estimate workflow cost before execution
//   - Nodes: decide whether a task is worth bidding on
// ════════════════════════════════════════════════════════════

// ── Base Costs (in internal energy units, 10000 = 1 Star) ──

var baseCostByCategory = map[TaskCategory]int64{
	CatCompute:   500,   // 0.05 Stars
	CatInference: 2000,  // 0.20 Stars
	CatData:      300,   // 0.03 Stars
	CatStorage:   100,   // 0.01 Stars
	CatGPU:       5000,  // 0.50 Stars
	CatCustom:    1000,  // 0.10 Stars
}

// ── Capability Multipliers ──

var capabilityMultipliers = map[string]float64{
	"gpu":        2.0,
	"gpu_a100":   5.0,
	"gpu_h100":   8.0,
	"llm":        1.5,
	"docker":     1.2,
	"python":     1.0,
	"high_mem":   1.3,  // > 16GB
	"large_disk": 1.2,  // > 100GB
}

// ── Data Types ──

// PriceQuote is a computed price recommendation
type PriceQuote struct {
	Category        TaskCategory `json:"category"`
	BaseCost        int64        `json:"base_cost"`
	BaseStars       float64      `json:"base_stars"`
	SupplyDemand    float64      `json:"supply_demand_ratio"`
	DemandMultiplier float64     `json:"demand_multiplier"`
	CapMultiplier   float64      `json:"capability_multiplier"`
	UrgencyMult     float64      `json:"urgency_multiplier"`
	RepDiscount     float64      `json:"reputation_discount"`
	HistoryBaseline int64        `json:"history_baseline"`
	RecommendedMin  int64        `json:"recommended_min"`
	RecommendedMax  int64        `json:"recommended_max"`
	RecommendedAvg  int64        `json:"recommended_avg"`
	MinStars        float64      `json:"min_stars"`
	MaxStars        float64      `json:"max_stars"`
	AvgStars        float64      `json:"avg_stars"`
	ComputedAt      time.Time    `json:"computed_at"`
}

// PriceRecord tracks settled task prices for historical baseline
type PriceRecord struct {
	Category   TaskCategory `json:"category"`
	Price      int64        `json:"price"`
	SettledAt  time.Time    `json:"settled_at"`
}

// ── PricingEngine ──

// PricingConfig holds engine settings
type PricingConfig struct {
	MaxDemandMultiplier  float64 `json:"max_demand_multiplier"`  // cap on demand surge
	UrgencyThresholdMin  int64   `json:"urgency_threshold_min"`  // minutes: below this = urgent
	UrgencyMaxMultiplier float64 `json:"urgency_max_multiplier"` // max urgency surcharge
	RepDiscountMax       float64 `json:"rep_discount_max"`       // max discount for legendary
	HistoryWindowHours   int     `json:"history_window_hours"`   // rolling window for baseline
	MaxHistory           int     `json:"max_history"`
}

// DefaultPricingConfig returns production defaults
func DefaultPricingConfig() *PricingConfig {
	return &PricingConfig{
		MaxDemandMultiplier:  3.0,
		UrgencyThresholdMin:  5,
		UrgencyMaxMultiplier: 2.0,
		RepDiscountMax:       0.15, // 15% discount for legendary
		HistoryWindowHours:   24,
		MaxHistory:           5000,
	}
}

// PricingEngine computes dynamic task pricing
type PricingEngine struct {
	mu      sync.RWMutex
	config  *PricingConfig
	history []PriceRecord
	stats   PricingStats
}

// PricingStats tracks pricing metrics
type PricingStats struct {
	TotalQuotes    int                    `json:"total_quotes"`
	TotalSettled   int                    `json:"total_settled"`
	AvgPriceByCategory map[TaskCategory]float64 `json:"avg_price_by_category"`
	HistorySize    int                    `json:"history_size"`
}

var (
	globalPricing *PricingEngine
	pricingOnce   sync.Once
)

// InitPricing creates the global pricing engine
func InitPricing(cfg *PricingConfig) *PricingEngine {
	if cfg == nil {
		cfg = DefaultPricingConfig()
	}
	pricingOnce.Do(func() {
		globalPricing = &PricingEngine{
			config:  cfg,
			history: make([]PriceRecord, 0, cfg.MaxHistory),
			stats: PricingStats{
				AvgPriceByCategory: make(map[TaskCategory]float64),
			},
		}
		log.Printf("[broodnet/pricing] engine ready (demand_cap=%.1fx, urgency_cap=%.1fx, rep_disc=%.0f%%)",
			cfg.MaxDemandMultiplier, cfg.UrgencyMaxMultiplier, cfg.RepDiscountMax*100)
	})
	return globalPricing
}

// GetPricing returns the global engine
func GetPricing() *PricingEngine {
	return globalPricing
}

// ── Core Pricing ──

// Quote computes a price recommendation for a task
func (pe *PricingEngine) Quote(category TaskCategory, reqs TaskRequirements, urgencyMinutes int64, bidderRepScore float64) *PriceQuote {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	q := &PriceQuote{
		Category:   category,
		ComputedAt: time.Now(),
	}

	// 1. Base cost
	base, ok := baseCostByCategory[category]
	if !ok {
		base = baseCostByCategory[CatCustom]
	}
	q.BaseCost = base
	q.BaseStars = float64(base) / 10000.0

	// 2. Supply/demand multiplier
	q.SupplyDemand = pe.computeSupplyDemand(category)
	q.DemandMultiplier = math.Min(pe.config.MaxDemandMultiplier,
		math.Max(0.5, q.SupplyDemand))

	// 3. Capability multiplier
	q.CapMultiplier = pe.computeCapabilityMultiplier(reqs)

	// 4. Urgency multiplier
	q.UrgencyMult = 1.0
	if urgencyMinutes > 0 && urgencyMinutes <= pe.config.UrgencyThresholdMin {
		// Linear scale: 5min → 2x, 1min → 2x (capped)
		ratio := float64(pe.config.UrgencyThresholdMin) / float64(urgencyMinutes)
		q.UrgencyMult = math.Min(pe.config.UrgencyMaxMultiplier, 1.0+ratio*0.5)
	}

	// 5. Reputation discount (for the bidder/executor)
	q.RepDiscount = 0
	if bidderRepScore >= 800 {
		// Linear: 800→5%, 950→15%
		factor := (bidderRepScore - 800) / 150.0
		q.RepDiscount = factor * pe.config.RepDiscountMax
	}

	// 6. Historical baseline
	q.HistoryBaseline = pe.categoryBaseline(category)

	// Compute final range
	rawPrice := float64(base) * q.DemandMultiplier * q.CapMultiplier * q.UrgencyMult
	discountedPrice := rawPrice * (1.0 - q.RepDiscount)

	q.RecommendedMin = int64(discountedPrice * 0.8)
	q.RecommendedMax = int64(rawPrice * 1.2)
	q.RecommendedAvg = int64((discountedPrice + rawPrice) / 2.0)

	// Blend with historical baseline (30% history, 70% computed)
	if q.HistoryBaseline > 0 {
		q.RecommendedAvg = int64(float64(q.RecommendedAvg)*0.7 + float64(q.HistoryBaseline)*0.3)
	}

	q.MinStars = float64(q.RecommendedMin) / 10000.0
	q.MaxStars = float64(q.RecommendedMax) / 10000.0
	q.AvgStars = float64(q.RecommendedAvg) / 10000.0

	pe.stats.TotalQuotes++

	return q
}

// computeSupplyDemand estimates the supply/demand ratio for a category
func (pe *PricingEngine) computeSupplyDemand(category TaskCategory) float64 {
	m := GetMarket()
	if m == nil {
		return 1.0
	}

	// Count open tasks in this category
	openTasks := m.OpenTasks(category, 100)
	demand := float64(len(openTasks))

	// Estimate supply from recent bid activity
	stats := m.Stats()
	avgBids := stats.AvgBidsPerTask
	if avgBids <= 0 {
		avgBids = 1.0
	}

	// demand / supply: > 1 means more tasks than bidders (prices go up)
	if demand == 0 {
		return 0.8 // low demand = slight discount
	}
	return math.Max(0.5, demand/avgBids)
}

// computeCapabilityMultiplier computes multiplier from requirements
func (pe *PricingEngine) computeCapabilityMultiplier(reqs TaskRequirements) float64 {
	mult := 1.0

	if reqs.NeedsGPU {
		mult *= capabilityMultipliers["gpu"]
	}
	if reqs.MinGPUMem >= 80000 { // 80GB = A100/H100 class
		mult *= capabilityMultipliers["gpu_h100"] / capabilityMultipliers["gpu"]
	} else if reqs.MinGPUMem >= 40000 { // 40GB = A100 class
		mult *= capabilityMultipliers["gpu_a100"] / capabilityMultipliers["gpu"]
	}

	if reqs.MinMemory >= 16384 {
		mult *= capabilityMultipliers["high_mem"]
	}

	for _, cap := range reqs.Capabilities {
		if m, ok := capabilityMultipliers[cap]; ok {
			mult *= m
		}
	}

	return mult
}

// categoryBaseline computes rolling average price for a category
func (pe *PricingEngine) categoryBaseline(category TaskCategory) int64 {
	cutoff := time.Now().Add(-time.Duration(pe.config.HistoryWindowHours) * time.Hour)
	var sum int64
	var count int
	for i := len(pe.history) - 1; i >= 0; i-- {
		r := pe.history[i]
		if r.SettledAt.Before(cutoff) {
			break
		}
		if r.Category == category {
			sum += r.Price
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / int64(count)
}

// RecordSettlement records a settled price for baseline computation
func (pe *PricingEngine) RecordSettlement(category TaskCategory, price int64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pe.history = append(pe.history, PriceRecord{
		Category:  category,
		Price:     price,
		SettledAt: time.Now(),
	})
	if len(pe.history) > pe.config.MaxHistory {
		pe.history = pe.history[1:]
	}
	pe.stats.TotalSettled++

	// Update running average
	baseline := pe.categoryBaseline(category)
	if baseline > 0 {
		pe.stats.AvgPriceByCategory[category] = float64(baseline) / 10000.0
	}
}

// ── Query ──

// Stats returns engine metrics
func (pe *PricingEngine) Stats() *PricingStats {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	s := pe.stats
	s.HistorySize = len(pe.history)
	return &s
}

// PriceConfig returns current config
func (pe *PricingEngine) PriceConfig() PricingConfig {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return *pe.config
}

// ValidateBid checks if a bid price is within reasonable range
func (pe *PricingEngine) ValidateBid(category TaskCategory, price int64) (bool, string) {
	base, ok := baseCostByCategory[category]
	if !ok {
		return true, ""
	}
	floor := base / 10 // 10% of base
	ceiling := base * 100 // 100x base

	if price < floor {
		return false, fmt.Sprintf("price %d below floor %d for category %s", price, floor, category)
	}
	if price > ceiling {
		return false, fmt.Sprintf("price %d above ceiling %d for category %s", price, ceiling, category)
	}
	return true, ""
}
