package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           int    // Hive Controller API port
	Domain         string // Base domain, e.g. starclaw.me
	DataDir        string // Base data directory
	NginxConfDir   string // Nginx config directory for generated confs
	SSLCertPath    string // Wildcard SSL cert path
	SSLKeyPath     string // Wildcard SSL key path
	ClawImage      string // Docker image for Claw API (full stack)
	ClawLiteImage  string // Docker image for Claw Lite (Spark tier, SQLite)
	HivePort       int    // Hive Controller port (injected into containers for Molt→Hive notifications)
	WebImage       string // Docker image for Claw Web (shared)
	ClawWebPort    int    // Port of shared Claw Web container (for nginx routing)
	NetworkName    string // Docker network name
	PortRangeStart int    // Starting port for Claw instances
	PortRangeEnd   int    // Ending port

	// Shared MySQL for hive instances
	MySQLHost     string
	MySQLPort     int
	MySQLRootUser string
	MySQLRootPass string

	// Shared Redis
	RedisHost     string
	RedisPort     int
	RedisPassword string

	// Overlord
	OverlordURL   string
	OverlordToken string

	// Queen billing (StarAI payment)
	QueenURL   string
	QueenToken string

	// Aliyun ECS
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
	AliyunRegionID        string
	AliyunVPCID           string
	AliyunVSwitchID       string
	AliyunSecurityGroupID string
	AliyunImageID         string

	// Aliyun DNS
	AliyunDNSDomain string // same as Domain by default

	// SSH for ECS remote updates
	SSHKeyPath string // path to SSH private key for ECS access
	SSHUser    string // SSH user on ECS instances (default: root)

	// Hive server public IP (for DNS A records in hive mode)
	HivePublicIP string

	// Hive admin token
	AdminToken string

	// Free tier limits
	FreeTierExpireDays int
	MaxFreeInstances   int
}

func Load() *Config {
	return &Config{
		Port:           envInt("HIVE_PORT", 9090),
		Domain:         envStr("HIVE_DOMAIN", "starclaw.me"),
		DataDir:        envStr("HIVE_DATA_DIR", "/opt/starclaw-hive"),
		NginxConfDir:   envStr("HIVE_NGINX_CONF_DIR", "/opt/starclaw-hive/nginx/conf.d"),
		SSLCertPath:    envStr("HIVE_SSL_CERT", "/etc/letsencrypt/live/starclaw.me/fullchain.pem"),
		SSLKeyPath:     envStr("HIVE_SSL_KEY", "/etc/letsencrypt/live/starclaw.me/privkey.pem"),
		ClawImage:      envStr("HIVE_CLAW_IMAGE", "starclaw-api:latest"),
		ClawLiteImage:  envStr("HIVE_CLAW_LITE_IMAGE", "starclaw-claw-lite:latest"),
		HivePort:       envInt("HIVE_PORT", 9090),
		WebImage:       envStr("HIVE_WEB_IMAGE", "starclaw-web:latest"),
		ClawWebPort:    envInt("HIVE_CLAW_WEB_PORT", 8083),
		NetworkName:    envStr("HIVE_NETWORK", "hive-net"),
		PortRangeStart: envInt("HIVE_PORT_START", 9001),
		PortRangeEnd:   envInt("HIVE_PORT_END", 9999),

		MySQLHost:     envStr("HIVE_MYSQL_HOST", "127.0.0.1"),
		MySQLPort:     envInt("HIVE_MYSQL_PORT", 3306),
		MySQLRootUser: envStr("HIVE_MYSQL_ROOT_USER", "root"),
		MySQLRootPass: envStr("HIVE_MYSQL_ROOT_PASS", ""),

		RedisHost:     envStr("HIVE_REDIS_HOST", "127.0.0.1"),
		RedisPort:     envInt("HIVE_REDIS_PORT", 6379),
		RedisPassword: envStr("HIVE_REDIS_PASSWORD", ""),

		OverlordURL:   envStr("HIVE_OVERLORD_URL", "https://overlord.starclaw.net"),
		OverlordToken: envStr("HIVE_OVERLORD_TOKEN", ""),

		QueenURL:   envStr("HIVE_QUEEN_URL", "https://queen.starclaw.net"),
		QueenToken: envStr("HIVE_QUEEN_TOKEN", ""),

		AliyunAccessKeyID:     envStr("ALIYUN_ACCESS_KEY_ID", ""),
		AliyunAccessKeySecret: envStr("ALIYUN_ACCESS_KEY_SECRET", ""),
		AliyunRegionID:        envStr("ALIYUN_REGION_ID", "cn-shanghai"),
		AliyunVPCID:           envStr("ALIYUN_VPC_ID", ""),
		AliyunVSwitchID:       envStr("ALIYUN_VSWITCH_ID", ""),
		AliyunSecurityGroupID: envStr("ALIYUN_SECURITY_GROUP_ID", ""),
		AliyunImageID:         envStr("ALIYUN_IMAGE_ID", ""),

		AliyunDNSDomain: envStr("ALIYUN_DNS_DOMAIN", envStr("HIVE_DOMAIN", "starclaw.me")),
		SSHKeyPath:      envStr("HIVE_SSH_KEY", "/root/.ssh/id_ed25519"),
		SSHUser:         envStr("HIVE_SSH_USER", "root"),
		HivePublicIP:    envStr("HIVE_PUBLIC_IP", ""),

		AdminToken:         envStr("HIVE_ADMIN_TOKEN", ""),
		FreeTierExpireDays: envInt("HIVE_FREE_EXPIRE_DAYS", 7),
		MaxFreeInstances:   envInt("HIVE_MAX_FREE", 100),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
