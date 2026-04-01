package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ExeDir returns the directory containing the running executable.
// Returns "" if it cannot be determined (e.g. go run with temp binary).
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	// Skip temp directories (go run builds to temp)
	if strings.Contains(dir, os.TempDir()) {
		return ""
	}
	return dir
}

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	OpenAI      OpenAIConfig      `mapstructure:"openai"`
	OAuth       OAuthConfig       `mapstructure:"oauth"`
	Node        NodeConfig        `mapstructure:"node"`
	Swarm       SwarmConfig       `mapstructure:"swarm"`
	Overlord    OverlordConfig    `mapstructure:"overlord"`
	Contributor ContributorConfig `mapstructure:"contributor"`
	Nydus       NydusConfig       `mapstructure:"nydus"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Hive        HiveConfig        `mapstructure:"hive"`
	Forge       ForgeConfig       `mapstructure:"forge"`
	Trading     TradingConfig     `mapstructure:"trading"`
}

type StorageConfig struct {
	DataDir string `mapstructure:"data_dir"` // root data directory, default /app (container) or ./data (host)
}

type NodeConfig struct {
	Address string `mapstructure:"address"` // public address of this Claw API, e.g. https://starclaw.me:8080
	WebURL  string `mapstructure:"web_url"` // browser-accessible Web UI URL, e.g. https://starclaw.me
	Name    string `mapstructure:"name"`    // display name, default hostname
	Region  string `mapstructure:"region"`  // e.g. cn-east, us-west
}

type SwarmConfig struct {
	Enabled           bool   `mapstructure:"enabled"`            // enable swarm registration
	QueenURL          string `mapstructure:"queen_url"`          // e.g. https://api.starclaw.me
	NodeToken         string `mapstructure:"node_token"`         // Queen internal API token (X-Node-Token); falls back to jwt.secret
	NodeName          string `mapstructure:"node_name"`          // display name for this Claw
	Region            string `mapstructure:"region"`             // e.g. cn-east, us-west
	HeartbeatInterval int    `mapstructure:"heartbeat_interval"` // seconds, default 30
}

type OverlordConfig struct {
	Enabled           bool   `mapstructure:"enabled"`            // enable overlord monitoring
	OverlordURL       string `mapstructure:"overlord_url"`       // e.g. https://overlord.starclaw.me
	NodeName          string `mapstructure:"node_name"`          // display name for this Claw
	Region            string `mapstructure:"region"`             // e.g. Shanghai, CN
	InviteCode        string `mapstructure:"invite_code"`        // optional invite code for brood join
	HeartbeatInterval int    `mapstructure:"heartbeat_interval"` // seconds, default 30
}

type ContributorConfig struct {
	Enabled      bool   `mapstructure:"enabled"`       // opt-in to contribute compute to the swarm
	OllamaURL    string `mapstructure:"ollama_url"`    // e.g. http://localhost:11434
	MaxJobs      int    `mapstructure:"max_jobs"`      // max concurrent inference jobs (default 2)
	ExternalAddr string `mapstructure:"external_addr"` // address other nodes can reach this node at
}

type HiveConfig struct {
	URL string `mapstructure:"url"` // Hive Controller URL, e.g. http://hive:9090 (set by Hive when creating containers)
}

type ForgeConfig struct {
	URL string `mapstructure:"url"` // Forge API URL, e.g. http://forge-api:8099 (when set, proxy /v1/forge/* to this)
}

type NydusConfig struct {
	Enabled     bool     `mapstructure:"enabled"`      // enable NAT traversal (default: auto when contributor.enabled)
	STUNServers []string `mapstructure:"stun_servers"` // custom STUN servers (default: Google + Cloudflare)
	RelayURLs   []string `mapstructure:"relay_urls"`   // relay fallback URLs (default: peer addresses)
	EnableRelay bool     `mapstructure:"enable_relay"` // serve as relay for other nodes (default: true if public IP)
}

type TradingConfig struct {
	Enabled   bool                    `mapstructure:"enabled"`
	Role      string                  `mapstructure:"role"`
	BridgeURL string                  `mapstructure:"bridge_url"`
	Mode      string                  `mapstructure:"mode"`
	Master    TradingMasterConfig     `mapstructure:"master"`
	Auto      TradingAutonomousPolicy `mapstructure:"autonomous_policy"`
}

type TradingMasterConfig struct {
	HeartbeatURL      string `mapstructure:"heartbeat_url"`
	HeartbeatInterval int    `mapstructure:"heartbeat_interval"`
	HeartbeatTimeout  int    `mapstructure:"heartbeat_timeout"`
	AutoAutonomous    bool   `mapstructure:"auto_autonomous"`
}

type TradingAutonomousPolicy struct {
	AllowNewPositions bool    `mapstructure:"allow_new_positions"`
	MaxPositionPct    float64 `mapstructure:"max_position_pct"`
	StopLossPct       float64 `mapstructure:"stop_loss_pct"`
	MinConfidence     float64 `mapstructure:"min_confidence"`
	ScanInterval      int     `mapstructure:"scan_interval"`
}

type OAuthConfig struct {
	GitHub OAuthProviderConfig `mapstructure:"github"`
	Google OAuthProviderConfig `mapstructure:"google"`
}

type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type ServerConfig struct {
	Port       int    `mapstructure:"port"`
	Mode       string `mapstructure:"mode"`        // debug, release
	DeployMode string `mapstructure:"deploy_mode"` // hosted, opensource
}

type DatabaseConfig struct {
	Driver     string `mapstructure:"driver"`      // mysql or sqlite (default: mysql)
	SQLitePath string `mapstructure:"sqlite_path"` // path for sqlite db file
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	User       string `mapstructure:"user"`
	Password   string `mapstructure:"password"`
	DBName     string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"` // false = use in-memory cache
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	ExpireHour int    `mapstructure:"expire_hour"`
}

type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	// Also search exe directory (handles Spore deployments where CWD ≠ exe dir)
	if ed := ExeDir(); ed != "" {
		viper.AddConfigPath(ed)
		viper.AddConfigPath(filepath.Join(ed, "configs"))
	}

	// Environment variable overrides
	viper.SetEnvPrefix("STARCLAW")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.deploy_mode", "opensource")
	viper.SetDefault("database.driver", "mysql")
	viper.SetDefault("database.sqlite_path", "./data/claw.db")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.password", "starclaw")
	viper.SetDefault("database.dbname", "starclaw")
	viper.SetDefault("redis.enabled", true)
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.secret", "starclaw-secret-key-change-me")
	viper.SetDefault("jwt.expire_hour", 72)
	viper.SetDefault("openai.api_key", "")
	viper.SetDefault("openai.base_url", "")
	viper.SetDefault("swarm.enabled", false)
	viper.SetDefault("swarm.queen_url", "")
	viper.SetDefault("swarm.node_token", "")
	viper.SetDefault("swarm.node_name", "")
	viper.SetDefault("swarm.region", "")
	viper.SetDefault("swarm.heartbeat_interval", 30)
	viper.SetDefault("overlord.enabled", false)
	viper.SetDefault("overlord.overlord_url", "")
	viper.SetDefault("overlord.node_name", "")
	viper.SetDefault("overlord.region", "")
	viper.SetDefault("node.address", "")
	viper.SetDefault("node.web_url", "")
	viper.SetDefault("node.name", "")
	viper.SetDefault("node.region", "")
	viper.SetDefault("overlord.heartbeat_interval", 30)
	viper.SetDefault("contributor.enabled", false)
	viper.SetDefault("contributor.ollama_url", "http://localhost:11434")
	viper.SetDefault("contributor.max_jobs", 2)
	viper.SetDefault("contributor.external_addr", "")
	viper.SetDefault("storage.data_dir", "/app")
	viper.SetDefault("hive.url", "")
	viper.SetDefault("forge.url", "")
	viper.SetDefault("trading.enabled", false)
	viper.SetDefault("trading.role", "worker")
	viper.SetDefault("trading.bridge_url", "http://localhost:8098")
	viper.SetDefault("trading.mode", "follower")
	viper.SetDefault("trading.master.heartbeat_url", "")
	viper.SetDefault("trading.master.heartbeat_interval", 10)
	viper.SetDefault("trading.master.heartbeat_timeout", 30)
	viper.SetDefault("trading.master.auto_autonomous", true)
	viper.SetDefault("trading.autonomous_policy.allow_new_positions", false)
	viper.SetDefault("trading.autonomous_policy.max_position_pct", 50.0)
	viper.SetDefault("trading.autonomous_policy.stop_loss_pct", 3.0)
	viper.SetDefault("trading.autonomous_policy.min_confidence", 0.90)
	viper.SetDefault("trading.autonomous_policy.scan_interval", 300)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// Config file not found, using defaults + env vars
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Spore integration: SPORE_DATA_DIR is the shared data directory that
	// persists across version upgrades. When set, override storage.data_dir
	// and sqlite_path so all data goes to the shared location.
	sporeDataDir := os.Getenv("SPORE_DATA_DIR")

	// Auto-detect: if exe is inside a Spore install dir (.spore/installed/<name>/v<ver>/),
	// derive shared data dir even without the env var (handles standalone restart).
	if sporeDataDir == "" {
		if ed := ExeDir(); ed != "" {
			// Pattern: .../.spore/installed/<name>/v<version>
			parent := filepath.Dir(ed)           // .../.spore/installed/<name>
			grandparent := filepath.Dir(parent)  // .../.spore/installed
			base := filepath.Base(ed)            // v<version>
			gpBase := filepath.Base(grandparent) // installed
			if gpBase == "installed" && len(base) > 1 && base[0] == 'v' && base[1] >= '0' && base[1] <= '9' {
				sporeDataDir = filepath.Join(parent, "data")
			}
		}
	}

	if sporeDataDir != "" {
		// Respect explicit config override: if user set storage.data_dir to something
		// other than the default "/app", honor their choice over auto-detected Spore path.
		if cfg.Storage.DataDir == "/app" || cfg.Storage.DataDir == "" {
			cfg.Storage.DataDir = sporeDataDir
		}
		if cfg.Database.Driver == "sqlite" && cfg.Database.SQLitePath == "./data/claw.db" {
			cfg.Database.SQLitePath = filepath.Join(cfg.Storage.DataDir, "claw.db")
		}
	}

	return &cfg, nil
}
