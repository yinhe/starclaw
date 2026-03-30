package trading

import (
	"log"
	"sync"
)

// Mode represents the current operating mode of this trading node.
type Mode string

const (
	ModeFollower     Mode = "follower"
	ModeCollaborator Mode = "collaborator"
	ModeAutonomous   Mode = "autonomous"
)

// NodeState tracks the runtime state of the trading node.
type NodeState struct {
	mu           sync.RWMutex
	mode         Mode
	masterOnline bool
	clawActive   bool
	cfg          Config
	onModeChange func(old, new Mode)
}

// NewNodeState creates a new node state from config.
func NewNodeState(cfg Config) *NodeState {
	m := Mode(cfg.Mode)
	if m != ModeFollower && m != ModeCollaborator && m != ModeAutonomous {
		m = ModeFollower
	}
	return &NodeState{
		mode:         m,
		masterOnline: true,
		clawActive:   m != ModeFollower,
		cfg:          cfg,
	}
}

// OnModeChange registers a callback fired when the mode changes.
func (s *NodeState) OnModeChange(fn func(old, new Mode)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onModeChange = fn
}

// Mode returns the current operating mode.
func (s *NodeState) CurrentMode() Mode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// IsClawActive returns whether the local Claw AI brain should be running.
func (s *NodeState) IsClawActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clawActive
}

// IsMasterOnline returns whether the Master heartbeat is alive.
func (s *NodeState) IsMasterOnline() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.masterOnline
}

// SetMasterOnline updates master connectivity and may trigger mode switch.
func (s *NodeState) SetMasterOnline(online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.masterOnline
	s.masterOnline = online

	if prev && !online && s.cfg.Master.AutoAutonomous {
		// Master went offline → switch to autonomous
		old := s.mode
		s.mode = ModeAutonomous
		s.clawActive = true
		log.Printf("[trading] AUTONOMOUS MODE ACTIVATED — Master offline, Claw taking over")
		if s.onModeChange != nil {
			go s.onModeChange(old, ModeAutonomous)
		}
	} else if !prev && online && s.mode == ModeAutonomous {
		// Master came back → restore previous mode
		old := s.mode
		s.mode = Mode(s.cfg.Mode)
		s.clawActive = s.mode != ModeFollower
		log.Printf("[trading] Master recovered — restoring mode: %s", s.mode)
		if s.onModeChange != nil {
			go s.onModeChange(old, s.mode)
		}
	}
}

// SetMode manually sets the operating mode (e.g. from API or config reload).
func (s *NodeState) SetMode(m Mode) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mode == m {
		return
	}
	old := s.mode
	s.mode = m
	s.clawActive = m != ModeFollower
	log.Printf("[trading] Mode changed: %s → %s (claw_active=%v)", old, m, s.clawActive)
	if s.onModeChange != nil {
		go s.onModeChange(old, m)
	}
}

// Policy returns the autonomous policy config.
func (s *NodeState) Policy() AutoPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Auto
}
