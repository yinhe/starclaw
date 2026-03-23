package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BranchProtection defines rules for a protected branch.
// Protected branches require PRs to merge — no direct push allowed.
type BranchProtection struct {
	ID              string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	RepoID          string    `json:"repo_id" gorm:"type:varchar(36);index;not null"`
	Branch          string    `json:"branch" gorm:"type:varchar(100);not null"`           // branch name or pattern (e.g. "master", "release/*")
	RequirePR       bool      `json:"require_pr" gorm:"default:true"`                     // must go through PR
	RequireReview   bool      `json:"require_review" gorm:"default:false"`                // at least 1 approval
	MinReviewers    int       `json:"min_reviewers" gorm:"default:0"`                     // minimum approvals needed
	RequireCI       bool      `json:"require_ci" gorm:"default:false"`                    // CI must pass
	AllowForcePush  bool      `json:"allow_force_push" gorm:"default:false"`              // allow force push
	AllowDelete     bool      `json:"allow_delete" gorm:"default:false"`                  // allow branch deletion
	RestrictPushTo  string    `json:"restrict_push_to" gorm:"type:varchar(200)"`          // comma-separated node_ids allowed to push
	CreatedBy       string    `json:"created_by" gorm:"type:varchar(80)"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (b *BranchProtection) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}
