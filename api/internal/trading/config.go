package trading

// Config holds all trading plugin configuration.
type Config struct {
	Enabled  bool         `yaml:"enabled" mapstructure:"enabled"`
	Role     string       `yaml:"role" mapstructure:"role"` // "worker" or "master"
	BridgeURL string     `yaml:"bridge_url" mapstructure:"bridge_url"`
	Mode     string       `yaml:"mode" mapstructure:"mode"` // "follower", "collaborator", "autonomous"
	Master   MasterConfig `yaml:"master" mapstructure:"master"`
	Auto     AutoPolicy   `yaml:"autonomous_policy" mapstructure:"autonomous_policy"`
}

// MasterConfig defines how this node connects to the Master.
type MasterConfig struct {
	HeartbeatURL      string `yaml:"heartbeat_url" mapstructure:"heartbeat_url"`
	HeartbeatInterval int    `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"` // seconds
	HeartbeatTimeout  int    `yaml:"heartbeat_timeout" mapstructure:"heartbeat_timeout"`   // seconds before switching to autonomous
	AutoAutonomous    bool   `yaml:"auto_autonomous" mapstructure:"auto_autonomous"`
}

// AutoPolicy defines constraints when running in autonomous mode.
type AutoPolicy struct {
	AllowNewPositions bool    `yaml:"allow_new_positions" mapstructure:"allow_new_positions"`
	MaxPositionPct    float64 `yaml:"max_position_pct" mapstructure:"max_position_pct"`
	StopLossPct       float64 `yaml:"stop_loss_pct" mapstructure:"stop_loss_pct"`
	MinConfidence     float64 `yaml:"min_confidence" mapstructure:"min_confidence"`
	ScanInterval      int     `yaml:"scan_interval" mapstructure:"scan_interval"` // seconds
}

// DefaultConfig returns sensible defaults for a worker node.
func DefaultConfig() Config {
	return Config{
		Enabled:   false,
		Role:      "worker",
		BridgeURL: "http://localhost:8098",
		Mode:      "follower",
		Master: MasterConfig{
			HeartbeatURL:      "",
			HeartbeatInterval: 10,
			HeartbeatTimeout:  30,
			AutoAutonomous:    true,
		},
		Auto: AutoPolicy{
			AllowNewPositions: false,
			MaxPositionPct:    50,
			StopLossPct:       3.0,
			MinConfidence:     0.90,
			ScanInterval:      300,
		},
	}
}
