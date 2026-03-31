package trading

import (
	"log"
)

// Plugin is the main entry point for the trading subsystem within Claw.
// It manages the node state, heartbeat, and lifecycle.
type Plugin struct {
	cfg       Config
	state     *NodeState
	heartbeat *Heartbeat
}

// NewPlugin creates and initializes the trading plugin.
// Call Start() to begin heartbeat monitoring and mode management.
func NewPlugin(cfg Config) *Plugin {
	state := NewNodeState(cfg)
	hb := NewHeartbeat(state, cfg.Master)

	state.OnModeChange(func(old, new Mode) {
		log.Printf("[trading] Mode transition: %s -> %s", old, new)
	})

	return &Plugin{
		cfg:       cfg,
		state:     state,
		heartbeat: hb,
	}
}

// Start begins the trading plugin's background processes.
func (p *Plugin) Start() {
	if !p.cfg.Enabled {
		log.Printf("[trading] Plugin disabled, skipping start")
		return
	}
	log.Printf("[trading] Plugin starting: role=%s mode=%s bridge=%s",
		p.cfg.Role, p.cfg.Mode, p.cfg.BridgeURL)
	p.heartbeat.Start()
}

// Stop gracefully shuts down the trading plugin.
func (p *Plugin) Stop() {
	p.heartbeat.Stop()
	log.Printf("[trading] Plugin stopped")
}

// State returns the current node state (for API handlers / status endpoints).
func (p *Plugin) State() *NodeState {
	return p.state
}

// Config returns the trading configuration.
func (p *Plugin) Config() Config {
	return p.cfg
}
