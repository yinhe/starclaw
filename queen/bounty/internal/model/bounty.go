package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BountyStatus string

const (
	BountyOpen      BountyStatus = "open"      // waiting for someone to claim
	BountyClaimed   BountyStatus = "claimed"   // human claimed, working on it
	BountyDelivered BountyStatus = "delivered" // human submitted deliverable
	BountyCompleted BountyStatus = "completed" // Claw accepted delivery
	BountyDisputed  BountyStatus = "disputed"  // under arbitration
	BountyCancelled BountyStatus = "cancelled" // cancelled by poster
	BountyExpired   BountyStatus = "expired"   // deadline passed, unclaimed
)

type BountyCategory string

const (
	CatDataLabeling   BountyCategory = "data_labeling"
	CatContentReview  BountyCategory = "content_review"
	CatCreativeDesign BountyCategory = "creative_design"
	CatRealWorld      BountyCategory = "real_world"
	CatExpertConsult  BountyCategory = "expert_consult"
	CatCodeReview     BountyCategory = "code_review"
	CatOther          BountyCategory = "other"
)

// Bounty represents a bounty task posted by a Claw (AI) for humans to complete
type Bounty struct {
	ID           string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	NodeID       string         `json:"node_id" gorm:"type:varchar(36);index"` // which Claw posted this
	UserID       string         `json:"user_id" gorm:"type:varchar(36);index"` // owner of that Claw
	Title        string         `json:"title" gorm:"type:varchar(300);not null"`
	Description  string         `json:"description" gorm:"type:longtext"`
	Category     BountyCategory `json:"category" gorm:"type:varchar(30);index"`
	Requirements string         `json:"requirements" gorm:"type:text"` // what the deliverable must include
	Reward       float64        `json:"reward" gorm:"not null"`        // amount in CNY
	Currency     string         `json:"currency" gorm:"type:varchar(10);default:CNY"`
	Status       BountyStatus   `json:"status" gorm:"type:varchar(20);index;default:open"`
	Visibility   string         `json:"visibility" gorm:"type:varchar(20);index;default:public"` // public = all users, partner = team/city partners only
	Deadline     *time.Time     `json:"deadline"`                                                // optional deadline

	// Claim info
	ClaimedBy string     `json:"claimed_by" gorm:"type:varchar(36);index"` // human user ID
	ClaimedAt *time.Time `json:"claimed_at"`

	// Delivery info
	DeliveryNote string     `json:"delivery_note" gorm:"type:longtext"`
	DeliveryURL  string     `json:"delivery_url" gorm:"type:varchar(500)"` // link to deliverable
	DeliveredAt  *time.Time `json:"delivered_at"`

	// Completion
	CompletedAt *time.Time `json:"completed_at"`
	Rating      int        `json:"rating" gorm:"default:0"`   // 1-5 rating of the deliverable
	Feedback    string     `json:"feedback" gorm:"type:text"` // poster's feedback

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (b *Bounty) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// BountyUser represents a human who participates in bounty tasks
type BountyUser struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	Username       string    `json:"username" gorm:"type:varchar(100);uniqueIndex"`
	Email          string    `json:"email" gorm:"type:varchar(200)"`
	PasswordHash   string    `json:"-" gorm:"type:varchar(200)"`
	CompletedCount int       `json:"completed_count" gorm:"default:0"`
	TotalEarned    float64   `json:"total_earned" gorm:"default:0"`
	Rating         float64   `json:"rating" gorm:"default:0"`
	RatingCount    int       `json:"rating_count" gorm:"default:0"`
	Level          string    `json:"level" gorm:"type:varchar(20);default:bronze"` // bronze, silver, gold, platinum
	CreatedAt      time.Time `json:"created_at"`
}

func (u *BountyUser) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}
