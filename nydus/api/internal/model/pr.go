package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PullRequest represents a merge request from one branch to another.
// Supports same-repo PRs and cross-fork PRs.
type PullRequest struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	RepoID       string     `json:"repo_id" gorm:"type:varchar(36);index;not null"`       // target repo
	Number       int        `json:"number" gorm:"index"`                                   // repo-scoped PR number (auto-increment)
	Title        string     `json:"title" gorm:"type:varchar(500);not null"`
	Body         string     `json:"body" gorm:"type:text"`
	AuthorNodeID string     `json:"author_node_id" gorm:"type:varchar(80);index"`          // who opened the PR
	SourceRepo   string     `json:"source_repo" gorm:"type:varchar(100)"`                  // source repo name (for cross-fork)
	SourceBranch string     `json:"source_branch" gorm:"type:varchar(100);not null"`       // feature branch
	TargetBranch string     `json:"target_branch" gorm:"type:varchar(100);not null"`       // e.g. master
	Status       string     `json:"status" gorm:"type:varchar(20);default:open;index"`     // open / merged / closed
	MergeCommit  string     `json:"merge_commit" gorm:"type:varchar(40)"`                  // merge commit hash
	MergedBy     string     `json:"merged_by" gorm:"type:varchar(80)"`                     // who merged
	MergedAt     *time.Time `json:"merged_at"`
	ClosedAt     *time.Time `json:"closed_at"`
	Labels       string     `json:"labels" gorm:"type:varchar(500)"`                       // comma-separated labels
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (pr *PullRequest) BeforeCreate(tx *gorm.DB) error {
	if pr.ID == "" {
		pr.ID = uuid.New().String()
	}
	return nil
}

// PRReview records a review on a pull request.
type PRReview struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	PRID         string    `json:"pr_id" gorm:"type:varchar(36);index;not null"`
	ReviewerNode string    `json:"reviewer_node" gorm:"type:varchar(80);not null"` // claw:xxx or "ai:architect"
	Verdict      string    `json:"verdict" gorm:"type:varchar(20);not null"`       // approve / request_changes / comment
	Body         string    `json:"body" gorm:"type:text"`
	FilePath     string    `json:"file_path" gorm:"type:varchar(500)"`             // optional: inline comment on file
	LineNumber   int       `json:"line_number" gorm:"default:0"`                   // optional: line number for inline
	CreatedAt    time.Time `json:"created_at"`
}

func (r *PRReview) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
