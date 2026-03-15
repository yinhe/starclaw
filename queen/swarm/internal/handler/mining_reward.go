package handler

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yinhe/starclaw-queen/swarm/internal/model"
	"gorm.io/gorm"
)

// MiningRewardWorker periodically rewards online contributors with star energy.
//
// Reward schedule:
//   - Runs every 10 minutes
//   - Awards base reward to each contributor that has been online (heartbeat within last 2 min)
//   - Base reward: 1 ⚡ per 10 min online (= 10000 units / 10 = 1000 units per tick)
//   - Bonus for GPU quality (based on gpu_info field)
//   - Daily online_minutes_today reset at midnight UTC
type MiningRewardWorker struct {
	db       *gorm.DB
	interval time.Duration
	stopCh   chan struct{}
}

const (
	rewardTickUnits  = 1000            // 0.1 ⚡ per 10-min tick (conservative start)
	heartbeatTimeout = 2 * time.Minute // node must have heartbeat within this window
)

func NewMiningRewardWorker(db *gorm.DB) *MiningRewardWorker {
	return &MiningRewardWorker{
		db:       db,
		interval: 10 * time.Minute,
		stopCh:   make(chan struct{}),
	}
}

// Start runs the reward loop in a goroutine
func (w *MiningRewardWorker) Start() {
	go w.loop()
	go w.dailyReset()
	log.Println("[mining] MiningRewardWorker started (interval=10m)")
}

// Stop signals the worker to stop
func (w *MiningRewardWorker) Stop() {
	close(w.stopCh)
}

func (w *MiningRewardWorker) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.rewardTick()
		case <-w.stopCh:
			log.Println("[mining] MiningRewardWorker stopped")
			return
		}
	}
}

func (w *MiningRewardWorker) rewardTick() {
	cutoff := time.Now().Add(-heartbeatTimeout)

	// Find all online contributors
	var nodes []model.Node
	w.db.Where("is_contributor = ? AND status = ? AND last_heartbeat > ?",
		true, model.StatusOnline, cutoff).Find(&nodes)

	if len(nodes) == 0 {
		return
	}

	rewarded := 0
	for _, node := range nodes {
		if node.ClawID == "" {
			continue
		}

		reward := calculateReward(node)
		if reward <= 0 {
			continue
		}

		// Grant reward via Queen credit API
		if grantMiningReward(node.ClawID, reward, "算力共享保底奖励") {
			now := time.Now()
			w.db.Model(&model.Node{}).Where("id = ?", node.ID).Updates(map[string]interface{}{
				"mining_earnings": gorm.Expr("mining_earnings + ?", reward),
				"last_reward_at":  now,
			})
			rewarded++
		}
	}

	if rewarded > 0 {
		log.Printf("[mining] rewarded %d/%d contributors", rewarded, len(nodes))
	}
}

// calculateReward returns reward units for a single tick.
// Base: 1000 units (0.1 ⚡). GPU bonus: +50% for high-end GPUs.
func calculateReward(node model.Node) int64 {
	base := int64(rewardTickUnits)

	// GPU bonus
	gpu := node.GPUInfo
	switch {
	case containsAny(gpu, "A100", "H100", "H200"):
		base = base * 3 // 3x for datacenter GPUs
	case containsAny(gpu, "4090", "4080", "3090"):
		base = base * 2 // 2x for high-end consumer
	case containsAny(gpu, "4070", "4060", "3080", "3070"):
		base = base * 150 / 100 // 1.5x for mid-range
	}

	return base
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// grantMiningReward calls Queen API to grant star energy
func grantMiningReward(clawID string, amount int64, remark string) bool {
	queenAPI := os.Getenv("QUEEN_API_URL")
	if queenAPI == "" {
		queenAPI = "http://queen-api:8080"
	}
	secret := os.Getenv("QUEEN_JWT_SECRET")
	if secret == "" {
		return false
	}

	body, _ := json.Marshal(map[string]interface{}{
		"claw_id": clawID,
		"amount":  amount,
		"type":    "mining_reward",
		"remark":  remark,
	})

	req, err := http.NewRequest("POST", queenAPI+"/internal/credits/grant", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", secret)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[mining] grant reward to %s failed: %v", clawID, err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// dailyReset resets online_minutes_today at midnight UTC
func (w *MiningRewardWorker) dailyReset() {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		timer := time.NewTimer(next.Sub(now))

		select {
		case <-timer.C:
			result := w.db.Model(&model.Node{}).Where("is_contributor = ?", true).
				Update("online_minutes_today", 0)
			log.Printf("[mining] daily reset: %d contributor nodes", result.RowsAffected)
		case <-w.stopCh:
			timer.Stop()
			return
		}
	}
}
