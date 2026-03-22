package model

import (
	"time"

	"gorm.io/gorm"
)

// ClawInstance represents a managed Claw node in the Hive
type ClawInstance struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	Slug        string     `gorm:"uniqueIndex;size:30;not null" json:"slug"`
	DisplayName string     `gorm:"size:100" json:"display_name"`
	OwnerID     string     `gorm:"size:36;index" json:"owner_id"`
	OwnerEmail  string     `gorm:"size:255" json:"owner_email"`

	// Deployment
	DeployMode  string `gorm:"size:20;default:hive" json:"deploy_mode"` // hive, ecs, spore
	Port        int    `json:"port"`                                     // internal port (hive mode)
	ContainerID string `gorm:"size:80" json:"container_id"`
	ECSID       string `gorm:"size:80" json:"ecs_id"`
	PublicIP    string `gorm:"size:45" json:"public_ip"`

	// Identity
	ClawID string `gorm:"size:100" json:"claw_id"` // claw:{hex} address
	NodeID string `gorm:"size:36" json:"node_id"`  // Overlord-assigned node ID

	// Status
	Status      string `gorm:"size:20;default:creating" json:"status"` // creating, running, stopped, error, destroying
	DBName      string `gorm:"size:60" json:"db_name"`                 // claw_{slug}
	DBUser      string `gorm:"size:60" json:"-"`
	DBPassword  string `gorm:"size:100" json:"-"`
	StorageUsed int64  `json:"storage_used"` // bytes

	// Resource limits
	CPULimit    float64 `gorm:"default:0.5" json:"cpu_limit"`
	MemoryLimit int64   `gorm:"default:536870912" json:"memory_limit"` // 512MB default
	StorageMax  int64   `gorm:"default:2147483648" json:"storage_max"` // 2GB default

	// JWT secret for this instance
	JWTSecret string `gorm:"size:64" json:"-"`

	// Timestamps
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ExpiresAt    *time.Time `json:"expires_at"`
	LastActiveAt *time.Time `json:"last_active_at"`

	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (ClawInstance) TableName() string { return "claw_instances" }

// SubdomainBlacklist holds reserved subdomains that cannot be used
type SubdomainBlacklist struct {
	Subdomain string `gorm:"primaryKey;size:50" json:"subdomain"`
	Reason    string `gorm:"size:50" json:"reason"` // system, infrastructure, brand, service
	CreatedAt time.Time `json:"created_at"`
}

func (SubdomainBlacklist) TableName() string { return "subdomain_blacklist" }

// DefaultBlacklist returns the initial set of reserved subdomains
func DefaultBlacklist() []SubdomainBlacklist {
	entries := map[string][]string{
		"system": {
			"app", "api", "www", "admin", "console", "root", "system", "null",
			"localhost", "local", "internal", "private", "public",
		},
		"infrastructure": {
			"overlord", "queen", "nydus", "swarm", "hive", "spore", "creep",
			"mail", "mx", "smtp", "imap", "pop", "ftp", "ssh", "dns", "ns1", "ns2",
			"vpn", "proxy", "gateway", "relay",
		},
		"service": {
			"cdn", "static", "assets", "img", "images", "media", "files", "uploads",
			"docs", "help", "support", "status", "blog", "forum", "wiki", "community",
			"store", "shop", "pay", "billing", "auth", "sso", "login", "register",
			"download", "downloads", "release", "releases", "update", "updates",
			"git", "repo", "registry", "mirror",
		},
		"devops": {
			"dev", "staging", "test", "demo", "sandbox", "preview", "beta", "alpha",
			"monitor", "grafana", "prometheus", "kibana", "elastic",
			"ci", "cd", "jenkins", "travis", "drone",
		},
		"brand": {
			"starclaw", "star-claw", "yinhe", "yinheai", "starai", "star-ai",
		},
	}

	var list []SubdomainBlacklist
	for reason, slugs := range entries {
		for _, s := range slugs {
			list = append(list, SubdomainBlacklist{
				Subdomain: s,
				Reason:    reason,
				CreatedAt: time.Now(),
			})
		}
	}
	return list
}
