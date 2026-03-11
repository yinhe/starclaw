package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NodeRole defines what role a node plays in the swarm
type NodeRole string

const (
	RoleClaw     NodeRole = "claw"
	RoleOverlord NodeRole = "overlord"
)

// NodeStatus represents the current health state of a node
type NodeStatus string

const (
	StatusOnline  NodeStatus = "online"
	StatusOffline NodeStatus = "offline"
	StatusFeral   NodeStatus = "feral" // disconnected but alive
)

// Node represents a registered Claw or Overlord in the swarm
type Node struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name         string     `json:"name" gorm:"type:varchar(200)"`
	Role         NodeRole   `json:"role" gorm:"type:varchar(20);index;not null"`
	Status       NodeStatus `json:"status" gorm:"type:varchar(20);default:online"`
	Version      string     `json:"version" gorm:"type:varchar(20)"`
	Address      string     `json:"address" gorm:"type:varchar(255)"` // host:port
	Region       string     `json:"region" gorm:"type:varchar(50)"`
	ClawID       string     `json:"claw_id" gorm:"type:varchar(60);index"`     // claw:xxxx (Ed25519-derived)
	OverlordID   string     `json:"overlord_id" gorm:"type:varchar(36);index"` // parent Overlord (empty for Queen-direct)
	Token        string     `json:"-" gorm:"type:varchar(128);uniqueIndex"`    // registration token for auth
	Capabilities string     `json:"capabilities" gorm:"type:json"`             // JSON: models, tools, etc.

	// Metrics (from latest heartbeat)
	CPUPercent    float64 `json:"cpu_percent" gorm:"default:0"`
	MemPercent    float64 `json:"mem_percent" gorm:"default:0"`
	TasksRunning  int     `json:"tasks_running" gorm:"default:0"`
	TasksQueued   int     `json:"tasks_queued" gorm:"default:0"`
	TokensUsed30d int64   `json:"tokens_used_30d" gorm:"default:0"`
	ErrorRate     float64 `json:"error_rate" gorm:"default:0"`

	LastHeartbeat time.Time      `json:"last_heartbeat"`
	RegisteredAt  time.Time      `json:"registered_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (n *Node) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.Capabilities == "" {
		n.Capabilities = "{}"
	}
	if n.RegisteredAt.IsZero() {
		n.RegisteredAt = time.Now()
	}
	return nil
}

// SwarmConfig is the config payload sent to nodes
type SwarmConfig struct {
	Models     []ModelInfo    `json:"models"`
	Policies   map[string]any `json:"policies"`
	Version    string         `json:"latest_version"`
	VersionURL string         `json:"latest_version_url"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
}
