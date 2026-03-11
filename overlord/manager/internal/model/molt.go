package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MoltRelease represents a Claw software update that needs approval before rollout
type MoltRelease struct {
	ID          string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Version     string `json:"version" gorm:"type:varchar(30);not null"`
	Channel     string `json:"channel" gorm:"type:varchar(20);default:stable"` // stable, beta, nightly
	Title       string `json:"title" gorm:"type:varchar(300)"`
	ReleaseNotes string `json:"release_notes" gorm:"type:text"`
	DownloadURL string `json:"download_url" gorm:"type:varchar(500)"`
	Checksum    string `json:"checksum" gorm:"type:varchar(128)"`              // SHA256

	// Approval workflow
	Status      string     `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending, approved, rejected, rolling, completed
	SubmittedBy string     `json:"submitted_by" gorm:"type:varchar(100)"`
	ReviewedBy  string     `json:"reviewed_by" gorm:"type:varchar(100)"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	ReviewNote  string     `json:"review_note" gorm:"type:text"`

	// Rollout config
	TargetTeam  string `json:"target_team" gorm:"type:varchar(100)"`      // empty = all teams
	RolloutPct  int    `json:"rollout_pct" gorm:"default:100"`            // percentage of nodes to update
	MaxFailures int    `json:"max_failures" gorm:"default:1"`             // auto-halt after N failures

	// Rollout progress
	TotalNodes   int `json:"total_nodes" gorm:"default:0"`
	UpdatedNodes int `json:"updated_nodes" gorm:"default:0"`
	FailedNodes  int `json:"failed_nodes" gorm:"default:0"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (r *MoltRelease) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// MoltNodeStatus tracks per-node update progress
type MoltNodeStatus struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	ReleaseID   string     `json:"release_id" gorm:"type:varchar(36);index;not null"`
	ClawNodeID  string     `json:"claw_node_id" gorm:"type:varchar(36);index;not null"`
	ClawName    string     `json:"claw_name" gorm:"type:varchar(200)"`
	OldVersion  string     `json:"old_version" gorm:"type:varchar(30)"`
	Status      string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending, downloading, installing, completed, failed
	ErrorDetail string     `json:"error_detail" gorm:"type:text"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
