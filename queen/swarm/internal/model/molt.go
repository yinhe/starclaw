package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReleaseStatus string

const (
	ReleasePending  ReleaseStatus = "pending"
	ReleaseRolling  ReleaseStatus = "rolling"
	ReleaseComplete ReleaseStatus = "complete"
	ReleasePaused   ReleaseStatus = "paused"
	ReleaseFailed   ReleaseStatus = "failed"
)

type ReleaseStrategy string

const (
	StrategyAll       ReleaseStrategy = "all"
	StrategyGrayscale ReleaseStrategy = "grayscale"
	StrategyManual    ReleaseStrategy = "manual"
)

// MoltRelease represents a version update release pushed through the swarm
type MoltRelease struct {
	ID          string          `json:"id" gorm:"type:varchar(36);primaryKey"`
	Version     string          `json:"version" gorm:"type:varchar(30);not null;uniqueIndex"`
	VersionURL  string          `json:"version_url" gorm:"type:varchar(500)"`
	Changelog   string          `json:"changelog" gorm:"type:text"`
	Strategy    ReleaseStrategy `json:"strategy" gorm:"type:varchar(20);default:all"`
	Percent     int             `json:"percent" gorm:"default:100"`
	Status      ReleaseStatus   `json:"status" gorm:"type:varchar(20);default:pending"`
	TargetRole  NodeRole        `json:"target_role" gorm:"type:varchar(20);default:claw"`
	Mandatory   bool            `json:"mandatory" gorm:"default:false"`
	NodesTotal  int             `json:"nodes_total" gorm:"default:0"`
	NodesOK     int             `json:"nodes_ok" gorm:"default:0"`
	NodesFailed int             `json:"nodes_failed" gorm:"default:0"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at"`
}

func (r *MoltRelease) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.Strategy == "" {
		r.Strategy = StrategyAll
	}
	if r.Status == "" {
		r.Status = ReleasePending
	}
	if r.TargetRole == "" {
		r.TargetRole = RoleClaw
	}
	return nil
}

// MoltNodeStatus tracks per-node update progress
type MoltNodeStatus struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ReleaseID string    `json:"release_id" gorm:"type:varchar(36);index;not null"`
	NodeID    string    `json:"node_id" gorm:"type:varchar(36);index;not null"`
	NodeName  string    `json:"node_name" gorm:"type:varchar(200)"`
	OldVer    string    `json:"old_version" gorm:"type:varchar(30)"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:pending"` // pending, updating, ok, failed
	Error     string    `json:"error" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *MoltNodeStatus) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
