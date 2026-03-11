package model

import "time"

// ContentReport represents a user report against content across services
type ContentReport struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ReporterID string    `json:"reporter_id" gorm:"type:varchar(36);index"`    // who filed the report
	TargetType string    `json:"target_type" gorm:"type:varchar(30);index"`    // forum_post / forum_reply / arena_thread / arena_reply / bounty
	TargetID   string    `json:"target_id" gorm:"type:varchar(36);index"`      // ID of the reported content
	TargetTitle string   `json:"target_title" gorm:"type:varchar(300)"`        // snapshot of content title
	AuthorID   string    `json:"author_id" gorm:"type:varchar(36);index"`      // content author
	Reason     string    `json:"reason" gorm:"type:varchar(50)"`               // spam / abuse / nsfw / illegal / other
	Detail     string    `json:"detail" gorm:"type:text"`                      // reporter's description
	Status     string    `json:"status" gorm:"type:varchar(20);index;default:pending"` // pending / reviewed / resolved / dismissed
	Resolution string    `json:"resolution" gorm:"type:varchar(20)"`           // warn / hide / delete / ban / none
	ReviewerID string    `json:"reviewer_id" gorm:"type:varchar(36)"`          // admin who reviewed
	ReviewNote string    `json:"review_note" gorm:"type:text"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Valid report reasons
var ReportReasons = []string{"spam", "abuse", "nsfw", "illegal", "other"}

// Valid target types
var ReportTargetTypes = []string{
	"forum_post", "forum_reply",
	"arena_thread", "arena_reply",
	"bounty",
}
