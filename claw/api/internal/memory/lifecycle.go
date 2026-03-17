package memory

import (
	"log"
	"sync"
	"time"

	"github.com/yinhe/starclaw/internal/model"
	"gorm.io/gorm"
)

// LifecycleManager handles memory decay, deduplication, and capacity limits.
type LifecycleManager struct {
	db     *gorm.DB
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager(db *gorm.DB) *LifecycleManager {
	return &LifecycleManager{db: db, stopCh: make(chan struct{})}
}

// Start begins the background lifecycle loop (runs daily checks).
func (lm *LifecycleManager) Start() {
	go lm.loop()
	log.Println("[cerebrate/lifecycle] started")
}

// Stop halts the lifecycle manager.
func (lm *LifecycleManager) Stop() {
	select {
	case <-lm.stopCh:
	default:
		close(lm.stopCh)
	}
}

func (lm *LifecycleManager) loop() {
	// Run once at startup (after a short delay)
	time.Sleep(30 * time.Second)
	lm.runCycle()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lm.runCycle()
		case <-lm.stopCh:
			return
		}
	}
}

func (lm *LifecycleManager) runCycle() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	decayed := lm.decayMemories()
	purged := lm.purgeStaleMemories()
	capped := lm.enforceCapacity()

	if decayed > 0 || purged > 0 || capped > 0 {
		log.Printf("[cerebrate/lifecycle] cycle complete: decayed=%d purged=%d capped=%d", decayed, purged, capped)
	}
}

// ── Decay ──

// decayMemories reduces importance of unused context/skill memories.
// Returns number of memories affected.
func (lm *LifecycleManager) decayMemories() int64 {
	now := time.Now()
	var total int64

	// context memories: decay 0.01 per day since last access
	result := lm.db.Model(&model.Memory{}).
		Where("category = ? AND importance > 0.1 AND last_access_at < ?",
			model.MemCatContext, now.Add(-24*time.Hour)).
		Update("importance", gorm.Expr(
			"GREATEST(0.05, importance - 0.01 * DATEDIFF(?, COALESCE(last_access_at, created_at)))", now))
	total += result.RowsAffected

	// skill memories: decay slower (0.005 per day)
	result = lm.db.Model(&model.Memory{}).
		Where("category = ? AND importance > 0.1 AND last_access_at < ?",
			model.MemCatSkill, now.Add(-48*time.Hour)).
		Update("importance", gorm.Expr(
			"GREATEST(0.05, importance - 0.005 * DATEDIFF(?, COALESCE(last_access_at, created_at)))", now))
	total += result.RowsAffected

	// summary memories: decay 0.008 per day
	result = lm.db.Model(&model.Memory{}).
		Where("category = ? AND importance > 0.1 AND last_access_at < ?",
			model.MemCatSummary, now.Add(-48*time.Hour)).
		Update("importance", gorm.Expr(
			"GREATEST(0.05, importance - 0.008 * DATEDIFF(?, COALESCE(last_access_at, created_at)))", now))
	total += result.RowsAffected

	// fact, preference, instruct: NO decay (stable long-term memories)

	return total
}

// ── Purge stale ──

// purgeStaleMemories deletes memories with very low importance that haven't been accessed in 30 days.
func (lm *LifecycleManager) purgeStaleMemories() int64 {
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	result := lm.db.Where(
		"importance < 0.1 AND last_access_at < ? AND category IN ?",
		cutoff, []string{model.MemCatContext, model.MemCatSkill, model.MemCatSummary},
	).Delete(&model.Memory{})
	return result.RowsAffected
}

// ── Capacity enforcement ──

// enforceCapacity limits each user+agent to maxMemories entries.
// Excess memories are removed starting from lowest importance context/skill.
func (lm *LifecycleManager) enforceCapacity() int64 {
	const maxPerAgent = 200

	// Find user+agent combos that exceed limit
	type combo struct {
		UserID  string
		AgentID string
		Count   int64
	}
	var overLimit []combo
	lm.db.Model(&model.Memory{}).
		Select("user_id, agent_id, COUNT(*) as count").
		Group("user_id, agent_id").
		Having("COUNT(*) > ?", maxPerAgent).
		Find(&overLimit)

	var totalDeleted int64
	for _, c := range overLimit {
		excess := c.Count - maxPerAgent
		if excess <= 0 {
			continue
		}

		// Delete the lowest-importance, oldest memories (context/skill/summary first)
		var toDelete []model.Memory
		lm.db.Where("user_id = ? AND agent_id = ? AND category IN ?",
			c.UserID, c.AgentID, []string{model.MemCatContext, model.MemCatSkill, model.MemCatSummary}).
			Order("importance ASC, last_access_at ASC").
			Limit(int(excess)).
			Find(&toDelete)

		if len(toDelete) > 0 {
			ids := make([]string, len(toDelete))
			for i, m := range toDelete {
				ids[i] = m.ID
			}
			result := lm.db.Where("id IN ?", ids).Delete(&model.Memory{})
			totalDeleted += result.RowsAffected
		}
	}

	return totalDeleted
}
