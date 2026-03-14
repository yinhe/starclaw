package model

import "time"

// Marketplace item statuses
const (
	ItemStatusDraft         = "draft"          // saved but not submitted
	ItemStatusPendingReview = "pending_review" // submitted, awaiting admin review
	ItemStatusApproved      = "approved"       // reviewed and approved → visible in marketplace
	ItemStatusRejected      = "rejected"       // reviewed and rejected
	ItemStatusPublished     = "published"      // legacy: directly published (pre-review era)
	ItemStatusRemoved       = "removed"        // taken down by admin
)

// MarketplaceItem is a generic marketplace entry (agent / skill / workflow / mcp)
type MarketplaceItem struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index"`
	Type        string    `json:"type" gorm:"type:varchar(20);index"` // agent / skill / workflow / mcp
	Name        string    `json:"name" gorm:"type:varchar(200)"`
	Description string    `json:"description" gorm:"type:text"`
	Icon        string    `json:"icon" gorm:"type:varchar(500)"`
	Version     string    `json:"version" gorm:"type:varchar(20);default:1.0.0"`
	Tags        string    `json:"tags" gorm:"type:varchar(500)"`                    // comma separated
	Config      string    `json:"config" gorm:"type:longtext"`                      // JSON blob
	Status      string    `json:"status" gorm:"type:varchar(20);default:published"` // draft / pending_review / approved / rejected / published / removed
	Downloads   int       `json:"downloads" gorm:"default:0"`
	Rating      float64   `json:"rating" gorm:"default:0"`
	RatingCount int       `json:"rating_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Review workflow
	ReviewStatus string     `json:"review_status" gorm:"type:varchar(20);index"` // pending / approved / rejected
	ReviewerID   string     `json:"reviewer_id" gorm:"type:varchar(36)"`
	ReviewNote   string     `json:"review_note" gorm:"type:text"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	SubmittedAt  *time.Time `json:"submitted_at"`

	// Virtual
	Author   *User `json:"author,omitempty" gorm:"foreignKey:UserID"`
	Reviewer *User `json:"reviewer,omitempty" gorm:"foreignKey:ReviewerID"`
}

type MarketplaceReview struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ItemID    string    `json:"item_id" gorm:"type:varchar(36);index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	Rating    int       `json:"rating" gorm:"type:tinyint"`
	Comment   string    `json:"comment" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey for developer Open API / star-ai.net gateway access
type APIKey struct {
	ID            string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID        string     `json:"user_id" gorm:"type:varchar(36);index"`
	Name          string     `json:"name" gorm:"type:varchar(100)"`
	Key           string     `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	KeyPrefix     string     `json:"key_prefix" gorm:"type:varchar(12)"` // sk-xxxx... (safe to display)
	Enabled       bool       `json:"enabled" gorm:"default:true"`
	RateLimit     int        `json:"rate_limit" gorm:"default:60"` // requests per minute, 0=unlimited
	TotalRequests int64      `json:"total_requests" gorm:"default:0"`
	TotalTokens   int64      `json:"total_tokens" gorm:"default:0"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
