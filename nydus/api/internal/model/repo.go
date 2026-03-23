package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NydusRepo represents a Git repository managed by Nydus.
// Repos from YAML config are synced as "system" source on startup.
type NydusRepo struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	Description string    `json:"description" gorm:"type:varchar(500)"`
	OwnerNodeID string    `json:"owner_node_id" gorm:"type:varchar(80);index"`   // node_id of creator (empty for system repos)
	TeamID      string    `json:"team_id" gorm:"type:varchar(36);index"`         // team that owns this repo
	Public      bool      `json:"public" gorm:"default:false"`                   // visible on /v1 public routes
	DefaultBranch string  `json:"default_branch" gorm:"type:varchar(50);default:master"`
	ForkedFrom  string    `json:"forked_from" gorm:"type:varchar(36);index"`     // parent repo ID (if fork)
	Source      string    `json:"source" gorm:"type:varchar(20);default:dynamic"` // system (from YAML) / dynamic (API-created)
	Status      string    `json:"status" gorm:"type:varchar(20);default:active"`  // active / archived / deleted
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *NydusRepo) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// RepoAccess defines per-node access to a repository.
// Granularity: owner > write > read > none.
type RepoAccess struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RepoID    string    `json:"repo_id" gorm:"type:varchar(36);index;not null"`
	NodeID    string    `json:"node_id" gorm:"type:varchar(80);index;not null"`  // claw:xxxx
	TeamID    string    `json:"team_id" gorm:"type:varchar(36);index"`           // grant access to entire team
	Level     string    `json:"level" gorm:"type:varchar(20);not null"`          // owner / write / read
	GrantedBy string    `json:"granted_by" gorm:"type:varchar(80)"`             // who granted this
	CreatedAt time.Time `json:"created_at"`
}

func (a *RepoAccess) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
