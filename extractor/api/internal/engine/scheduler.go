package engine

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/model"
)

// Scheduler manages strategy lifecycle: start, stop, monitor.
type Scheduler struct {
	DB       *gorm.DB
	Bridge   *bridge.Client
	RiskCtrl *RiskController
	mu       sync.RWMutex
	running  map[string]bool // strategy_id → is_running
	stopCh   chan struct{}
}

func NewScheduler(db *gorm.DB, bc *bridge.Client, rc *RiskController) *Scheduler {
	s := &Scheduler{
		DB:       db,
		Bridge:   bc,
		RiskCtrl: rc,
		running:  make(map[string]bool),
		stopCh:   make(chan struct{}),
	}
	go s.monitorLoop()
	return s
}

func (s *Scheduler) StartStrategy(strategy *model.Strategy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running[strategy.ID] {
		return nil
	}

	// TODO: parse accounts from strategy.Accounts JSON, start on each account via Bridge
	log.Printf("[scheduler] starting strategy %s (%s)", strategy.Name, strategy.ID)
	s.running[strategy.ID] = true

	strategy.Status = "running"
	s.DB.Model(strategy).Update("status", "running")
	return nil
}

func (s *Scheduler) StopStrategy(strategyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.Bridge.StopStrategy(strategyID); err != nil {
		log.Printf("[scheduler] bridge stop error: %v", err)
	}

	delete(s.running, strategyID)
	s.DB.Model(&model.Strategy{}).Where("id = ?", strategyID).Update("status", "stopped")
	log.Printf("[scheduler] stopped strategy %s", strategyID)
	return nil
}

func (s *Scheduler) IsRunning(strategyID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running[strategyID]
}

// monitorLoop periodically checks strategy health and risk.
func (s *Scheduler) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.checkHealth()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) checkHealth() {
	if err := s.Bridge.Ping(); err != nil {
		log.Printf("[scheduler] bridge health check failed: %v", err)
		s.RiskCtrl.OnBridgeDisconnect()
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}
