package broodnet

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ════════════════════════════════════════════════════════════
// BroodOS Network — TaskMarket (去中心任务撮合)
//
// A decentralized task marketplace where Claw nodes:
//   - Post tasks they need done (compute, data, inference, etc.)
//   - Bid on tasks they can fulfill
//   - Get matched by the market engine (best price/capability fit)
//   - Settle payment via star energy (Queen CreditClient)
//
// Task lifecycle: posted → bidding → matched → executing → completed/failed → settled
//
// Integration:
//   - Hive Discovery: find capable nodes for task routing
//   - Hydralisk: heavy tasks get routed to Hydralisk workers
//   - Pheromone: real-time task events
//   - CreditClient: star energy payment and settlement
// ════════════════════════════════════════════════════════════

// ── Types ──

// TaskCategory classifies what kind of work a task requires
type TaskCategory string

const (
	CatCompute   TaskCategory = "compute"   // general compute (agent execution)
	CatInference TaskCategory = "inference" // LLM inference
	CatData      TaskCategory = "data"      // data processing / ETL
	CatStorage   TaskCategory = "storage"   // file storage / retrieval
	CatGPU       TaskCategory = "gpu"       // GPU-intensive work
	CatCustom    TaskCategory = "custom"    // user-defined
)

// MarketTaskStatus tracks the task lifecycle
type MarketTaskStatus string

const (
	TaskPosted    MarketTaskStatus = "posted"
	TaskBidding   MarketTaskStatus = "bidding"
	TaskMatched   MarketTaskStatus = "matched"
	TaskExecuting MarketTaskStatus = "executing"
	TaskCompleted MarketTaskStatus = "completed"
	TaskFailed    MarketTaskStatus = "failed"
	TaskSettled   MarketTaskStatus = "settled"
	TaskCancelled MarketTaskStatus = "cancelled"
	TaskExpired   MarketTaskStatus = "expired"
)

// MarketTask represents a task posted to the marketplace
type MarketTask struct {
	ID          string           `json:"id"`
	PostedBy    string           `json:"posted_by"`    // node ID of requester
	Category    TaskCategory     `json:"category"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Payload     json.RawMessage  `json:"payload,omitempty"`
	Requirements TaskRequirements `json:"requirements"`
	Budget      int64            `json:"budget"`       // max star energy willing to pay
	BudgetStars float64          `json:"budget_stars"` // human-readable (1 Star = 10000 units)
	Status      MarketTaskStatus `json:"status"`
	AssignedTo  string           `json:"assigned_to,omitempty"` // winning bidder
	BidCount    int              `json:"bid_count"`
	Result      string           `json:"result,omitempty"`
	Error       string           `json:"error,omitempty"`
	SettleTxnID string           `json:"settle_txn_id,omitempty"`
	PostedAt    time.Time        `json:"posted_at"`
	ExpiresAt   time.Time        `json:"expires_at"`
	MatchedAt   *time.Time       `json:"matched_at,omitempty"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	SettledAt   *time.Time       `json:"settled_at,omitempty"`
}

// TaskRequirements specifies what the executor needs
type TaskRequirements struct {
	MinCPU     float64  `json:"min_cpu,omitempty"`     // cores
	MinMemory  int64    `json:"min_memory,omitempty"`   // MB
	NeedsGPU   bool     `json:"needs_gpu,omitempty"`
	MinGPUMem  int64    `json:"min_gpu_mem,omitempty"`  // MB
	Capabilities []string `json:"capabilities,omitempty"` // e.g., "python", "docker", "llm"
	MaxLatencyMs int64  `json:"max_latency_ms,omitempty"`
}

// Bid represents a node's offer to execute a task
type Bid struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	BidderID  string    `json:"bidder_id"`  // node ID of executor
	Price     int64     `json:"price"`      // star energy asking price
	PriceStars float64  `json:"price_stars"`
	ETA       int64     `json:"eta_seconds"` // estimated completion time
	Score     float64   `json:"score"`       // market-computed match score
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ── TaskMarket Engine ──

// MarketConfig holds marketplace settings
type MarketConfig struct {
	MaxTasks       int           `json:"max_tasks"`
	MaxBidsPerTask int           `json:"max_bids_per_task"`
	DefaultTTL     time.Duration `json:"default_ttl"`
	AutoMatchDelay time.Duration `json:"auto_match_delay"` // wait this long for bids before matching
	MinBudget      int64         `json:"min_budget"`       // minimum task budget
	PlatformFee    float64       `json:"platform_fee"`     // 0.0 - 1.0 fraction
}

// DefaultMarketConfig returns production defaults
func DefaultMarketConfig() *MarketConfig {
	return &MarketConfig{
		MaxTasks:       5000,
		MaxBidsPerTask: 20,
		DefaultTTL:     30 * time.Minute,
		AutoMatchDelay: 10 * time.Second,
		MinBudget:      100,     // 0.01 Stars minimum
		PlatformFee:    0.02,    // 2% platform fee
	}
}

// TaskMarket is the decentralized task marketplace
type TaskMarket struct {
	mu        sync.RWMutex
	config    *MarketConfig
	nodeID    string
	tasks     []*MarketTask
	byID      map[string]*MarketTask
	bids      map[string][]*Bid // taskID → bids
	stats     MarketStats
}

// MarketStats tracks marketplace metrics
type MarketStats struct {
	TotalPosted    int                        `json:"total_posted"`
	TotalMatched   int                        `json:"total_matched"`
	TotalCompleted int                        `json:"total_completed"`
	TotalFailed    int                        `json:"total_failed"`
	TotalSettled   int                        `json:"total_settled"`
	TotalBids      int                        `json:"total_bids"`
	TotalVolume    int64                      `json:"total_volume"`     // total star energy transacted
	VolumeStars    float64                    `json:"volume_stars"`
	ByCategory     map[TaskCategory]int       `json:"by_category"`
	ByStatus       map[MarketTaskStatus]int   `json:"by_status"`
	AvgBidsPerTask float64                    `json:"avg_bids_per_task"`
	AvgSettleMs    int64                      `json:"avg_settle_ms"`
	totalSettleMs  int64
}

// NewTaskMarket creates a new task marketplace
func NewTaskMarket(nodeID string, cfg *MarketConfig) *TaskMarket {
	if cfg == nil {
		cfg = DefaultMarketConfig()
	}
	return &TaskMarket{
		config: cfg,
		nodeID: nodeID,
		tasks:  make([]*MarketTask, 0, cfg.MaxTasks),
		byID:   make(map[string]*MarketTask),
		bids:   make(map[string][]*Bid),
		stats: MarketStats{
			ByCategory: make(map[TaskCategory]int),
			ByStatus:   make(map[MarketTaskStatus]int),
		},
	}
}

// ── Post / Bid / Match / Complete / Settle ──

// PostTask creates a new task listing
func (tm *TaskMarket) PostTask(postedBy string, category TaskCategory, title, description string,
	payload json.RawMessage, reqs TaskRequirements, budget int64) (*MarketTask, error) {

	if budget < tm.config.MinBudget {
		return nil, fmt.Errorf("budget %d below minimum %d", budget, tm.config.MinBudget)
	}
	if title == "" {
		return nil, fmt.Errorf("title required")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &MarketTask{
		ID:           "task:" + uuid.New().String()[:8],
		PostedBy:     postedBy,
		Category:     category,
		Title:        title,
		Description:  description,
		Payload:      payload,
		Requirements: reqs,
		Budget:       budget,
		BudgetStars:  float64(budget) / 10000.0,
		Status:       TaskPosted,
		PostedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(tm.config.DefaultTTL),
	}

	tm.tasks = append(tm.tasks, task)
	tm.byID[task.ID] = task
	tm.bids[task.ID] = make([]*Bid, 0)
	tm.stats.TotalPosted++
	tm.stats.ByCategory[category]++
	tm.stats.ByStatus[TaskPosted]++

	// Evict expired/old tasks
	tm.evict()

	log.Printf("[broodnet/market] task posted: %s %q by %s (budget=%.2f⚡)",
		task.ID, title, postedBy, task.BudgetStars)

	return task, nil
}

// PlaceBid submits a bid on a posted task
func (tm *TaskMarket) PlaceBid(taskID, bidderID string, price int64, etaSeconds int64, reason string) (*Bid, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskPosted && task.Status != TaskBidding {
		return nil, fmt.Errorf("task %s not accepting bids (status=%s)", taskID, task.Status)
	}
	if bidderID == task.PostedBy {
		return nil, fmt.Errorf("cannot bid on your own task")
	}
	if price > task.Budget {
		return nil, fmt.Errorf("bid price %d exceeds budget %d", price, task.Budget)
	}
	if len(tm.bids[taskID]) >= tm.config.MaxBidsPerTask {
		return nil, fmt.Errorf("max bids reached for task %s", taskID)
	}

	// Check for duplicate bidder
	for _, b := range tm.bids[taskID] {
		if b.BidderID == bidderID {
			return nil, fmt.Errorf("already bid on task %s", taskID)
		}
	}

	bid := &Bid{
		ID:         "bid:" + uuid.New().String()[:8],
		TaskID:     taskID,
		BidderID:   bidderID,
		Price:      price,
		PriceStars: float64(price) / 10000.0,
		ETA:        etaSeconds,
		Reason:     reason,
		CreatedAt:  time.Now(),
	}

	// Score the bid: lower price + faster ETA = better
	bid.Score = scoreBid(bid, task)

	tm.bids[taskID] = append(tm.bids[taskID], bid)
	task.Status = TaskBidding
	task.BidCount++
	tm.stats.TotalBids++

	log.Printf("[broodnet/market] bid %s on %s by %s (price=%.2f⚡, eta=%ds, score=%.2f)",
		bid.ID, taskID, bidderID, bid.PriceStars, etaSeconds, bid.Score)

	return bid, nil
}

// scoreBid computes a match score (higher is better)
func scoreBid(bid *Bid, task *MarketTask) float64 {
	score := 0.0

	// Price efficiency: how much under budget (0-40 points)
	if task.Budget > 0 {
		savings := float64(task.Budget-bid.Price) / float64(task.Budget)
		score += savings * 40.0
	}

	// Speed: faster ETA = better (0-30 points)
	if bid.ETA > 0 {
		if bid.ETA <= 10 {
			score += 30.0
		} else if bid.ETA <= 60 {
			score += 20.0
		} else if bid.ETA <= 300 {
			score += 10.0
		}
	}

	// Base reliability score (placeholder — future: use node reputation)
	score += 30.0

	return score
}

// MatchBest selects the best bid for a task and assigns it
func (tm *TaskMarket) MatchBest(taskID string) (*Bid, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskBidding && task.Status != TaskPosted {
		return nil, fmt.Errorf("task %s cannot be matched (status=%s)", taskID, task.Status)
	}

	bids := tm.bids[taskID]
	if len(bids) == 0 {
		return nil, fmt.Errorf("no bids on task %s", taskID)
	}

	// Sort by score descending
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Score > bids[j].Score
	})

	winner := bids[0]
	now := time.Now()
	task.Status = TaskMatched
	task.AssignedTo = winner.BidderID
	task.MatchedAt = &now
	tm.stats.TotalMatched++
	tm.stats.ByStatus[TaskMatched]++

	log.Printf("[broodnet/market] matched %s → %s (bid=%s, price=%.2f⚡, score=%.2f)",
		taskID, winner.BidderID, winner.ID, winner.PriceStars, winner.Score)

	return winner, nil
}

// StartExecution marks a matched task as executing
func (tm *TaskMarket) StartExecution(taskID, executorID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskMatched {
		return fmt.Errorf("task %s not in matched state", taskID)
	}
	if task.AssignedTo != executorID {
		return fmt.Errorf("task %s not assigned to %s", taskID, executorID)
	}

	task.Status = TaskExecuting
	tm.stats.ByStatus[TaskExecuting]++
	return nil
}

// CompleteTask marks a task as completed with result
func (tm *TaskMarket) CompleteTask(taskID, executorID, result string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.AssignedTo != executorID {
		return fmt.Errorf("task %s not assigned to %s", taskID, executorID)
	}

	now := time.Now()
	task.Status = TaskCompleted
	task.Result = result
	task.CompletedAt = &now
	tm.stats.TotalCompleted++
	tm.stats.ByStatus[TaskCompleted]++

	log.Printf("[broodnet/market] task %s completed by %s", taskID, executorID)
	return nil
}

// FailTask marks a task as failed
func (tm *TaskMarket) FailTask(taskID, executorID, errMsg string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	task.Status = TaskFailed
	task.Error = errMsg
	task.CompletedAt = &now
	tm.stats.TotalFailed++
	tm.stats.ByStatus[TaskFailed]++

	log.Printf("[broodnet/market] task %s failed: %s", taskID, errMsg)
	return nil
}

// SettleTask records payment settlement for a completed task
func (tm *TaskMarket) SettleTask(taskID, txnID string, amount int64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.Status != TaskCompleted {
		return fmt.Errorf("task %s not completed, cannot settle", taskID)
	}

	now := time.Now()
	task.Status = TaskSettled
	task.SettleTxnID = txnID
	task.SettledAt = &now
	tm.stats.TotalSettled++
	tm.stats.TotalVolume += amount
	tm.stats.VolumeStars = float64(tm.stats.TotalVolume) / 10000.0
	tm.stats.ByStatus[TaskSettled]++

	if task.MatchedAt != nil {
		settleMs := now.Sub(*task.MatchedAt).Milliseconds()
		tm.stats.totalSettleMs += settleMs
		tm.stats.AvgSettleMs = tm.stats.totalSettleMs / int64(tm.stats.TotalSettled)
	}

	log.Printf("[broodnet/market] task %s settled: txn=%s, amount=%.2f⚡",
		taskID, txnID, float64(amount)/10000.0)
	return nil
}

// CancelTask cancels a posted/bidding task
func (tm *TaskMarket) CancelTask(taskID, requesterID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.byID[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if task.PostedBy != requesterID {
		return fmt.Errorf("only poster can cancel")
	}
	if task.Status != TaskPosted && task.Status != TaskBidding {
		return fmt.Errorf("cannot cancel task in %s state", task.Status)
	}

	task.Status = TaskCancelled
	tm.stats.ByStatus[TaskCancelled]++
	return nil
}

// ── Query ──

// GetTask retrieves a task by ID
func (tm *TaskMarket) GetTask(id string) *MarketTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.byID[id]
}

// GetBids returns all bids for a task
func (tm *TaskMarket) GetBids(taskID string) []*Bid {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.bids[taskID]
}

// ListTasks returns tasks filtered by status/category
func (tm *TaskMarket) ListTasks(status MarketTaskStatus, category TaskCategory, limit int) []*MarketTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*MarketTask
	for i := len(tm.tasks) - 1; i >= 0 && len(result) < limit; i-- {
		t := tm.tasks[i]
		if status != "" && t.Status != status {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		result = append(result, t)
	}
	return result
}

// OpenTasks returns tasks available for bidding
func (tm *TaskMarket) OpenTasks(category TaskCategory, limit int) []*MarketTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}
	var result []*MarketTask
	now := time.Now()
	for i := len(tm.tasks) - 1; i >= 0 && len(result) < limit; i-- {
		t := tm.tasks[i]
		if t.Status != TaskPosted && t.Status != TaskBidding {
			continue
		}
		if now.After(t.ExpiresAt) {
			continue
		}
		if category != "" && t.Category != category {
			continue
		}
		result = append(result, t)
	}
	return result
}

// Stats returns marketplace metrics
func (tm *TaskMarket) Stats() *MarketStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	s := tm.stats
	if s.TotalPosted > 0 {
		s.AvgBidsPerTask = float64(s.TotalBids) / float64(s.TotalPosted)
	}
	return &s
}

// Config returns market config
func (tm *TaskMarket) MarketConfig() MarketConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return *tm.config
}

// ── Maintenance ──

// evict removes expired tasks and caps total count
func (tm *TaskMarket) evict() {
	now := time.Now()
	// Expire old posted/bidding tasks
	for _, t := range tm.tasks {
		if (t.Status == TaskPosted || t.Status == TaskBidding) && now.After(t.ExpiresAt) {
			t.Status = TaskExpired
			tm.stats.ByStatus[TaskExpired]++
		}
	}
	// Cap total
	for len(tm.tasks) > tm.config.MaxTasks {
		old := tm.tasks[0]
		tm.tasks = tm.tasks[1:]
		delete(tm.byID, old.ID)
		delete(tm.bids, old.ID)
	}
}

// ExpireSweep runs expiration on timer (call periodically)
func (tm *TaskMarket) ExpireSweep() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	expired := 0
	for _, t := range tm.tasks {
		if (t.Status == TaskPosted || t.Status == TaskBidding) && now.After(t.ExpiresAt) {
			t.Status = TaskExpired
			tm.stats.ByStatus[TaskExpired]++
			expired++
		}
	}
	return expired
}
