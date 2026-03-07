package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClawNode represents a Claw managed by this Overlord
type ClawNode struct {
	ID            string  `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name          string  `json:"name" gorm:"type:varchar(200)"`
	Address       string  `json:"address" gorm:"type:varchar(255);not null"` // host:port
	Version       string  `json:"version" gorm:"type:varchar(20)"`
	Status        string  `json:"status" gorm:"type:varchar(20);default:online;index"` // online, feral, offline
	Token         string  `json:"-" gorm:"type:varchar(128);uniqueIndex"`
	Team          string  `json:"team" gorm:"type:varchar(100);index"`     // multi-tenant team tag
	Tags          string  `json:"tags" gorm:"type:varchar(500)"`           // JSON array for routing

	// Quotas
	MaxConcurrent int   `json:"max_concurrent" gorm:"default:10"`
	MaxTokensDay  int64 `json:"max_tokens_day" gorm:"default:0"`          // 0 = unlimited

	// Metrics (latest heartbeat)
	CPUPercent   float64 `json:"cpu_percent" gorm:"default:0"`
	MemPercent   float64 `json:"mem_percent" gorm:"default:0"`
	TasksRunning int     `json:"tasks_running" gorm:"default:0"`
	TasksQueued  int     `json:"tasks_queued" gorm:"default:0"`
	TokensToday  int64   `json:"tokens_today" gorm:"default:0"`
	ErrorRate    float64 `json:"error_rate" gorm:"default:0"`
	AvgLatencyMs int     `json:"avg_latency_ms" gorm:"default:0"`

	LastHeartbeat time.Time  `json:"last_heartbeat"`
	RegisteredAt  time.Time  `json:"registered_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (n *ClawNode) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.RegisteredAt.IsZero() {
		n.RegisteredAt = time.Now()
	}
	return nil
}

// TaskAssignment records which Claw was assigned which task
type TaskAssignment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	TaskID     string    `json:"task_id" gorm:"type:varchar(36);index"`
	ClawID     string    `json:"claw_id" gorm:"type:varchar(36);index"`
	Status     string    `json:"status" gorm:"type:varchar(20);default:assigned"` // assigned, running, completed, failed
	AssignedAt time.Time `json:"assigned_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// AuditLog records management operations for compliance
type AuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Actor     string    `json:"actor" gorm:"type:varchar(100);index"`  // who performed the action
	Action    string    `json:"action" gorm:"type:varchar(50);index"`  // register, remove, update_quota, assign_task, etc.
	TargetID  string    `json:"target_id" gorm:"type:varchar(36)"`
	Detail    string    `json:"detail" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}
