package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── Agent Listing (付费 Agent 上架) ──

// PricingModel defines how an Agent is priced.
type PricingModel string

const (
	PricingFree         PricingModel = "free"
	PricingOneTime      PricingModel = "one_time"
	PricingSubscription PricingModel = "subscription"
)

// AgentListing extends AgentTemplate with commercial attributes.
type AgentListing struct {
	ID             string       `json:"id" gorm:"type:varchar(36);primaryKey"`
	TemplateID     string       `json:"template_id" gorm:"type:varchar(36);uniqueIndex;not null"` // references AgentTemplate
	CreatorID      string       `json:"creator_id" gorm:"type:varchar(36);index;not null"`
	Pricing        PricingModel `json:"pricing" gorm:"type:varchar(20);default:free"`
	PriceCents     int          `json:"price_cents" gorm:"default:0"`       // price in cents (¥0.01 units)
	MonthlyPricing int          `json:"monthly_pricing" gorm:"default:0"`   // monthly subscription in cents
	Currency       string       `json:"currency" gorm:"type:varchar(10);default:CNY"`
	Screenshots    string       `json:"screenshots" gorm:"type:json"`       // JSON array of image URLs
	DemoURL        string       `json:"demo_url" gorm:"type:varchar(500)"`
	ChangelogURL   string       `json:"changelog_url" gorm:"type:varchar(500)"`
	Version        string       `json:"version" gorm:"type:varchar(20);default:1.0.0"`
	MinClawVersion string       `json:"min_claw_version" gorm:"type:varchar(20)"` // minimum compatible version
	Status         string       `json:"status" gorm:"type:varchar(20);default:draft;index"` // draft, pending_review, published, suspended
	ReviewNote     string       `json:"review_note" gorm:"type:text"`
	SalesCount     int          `json:"sales_count" gorm:"default:0"`
	Revenue        int64        `json:"revenue" gorm:"default:0"` // total revenue in cents
	Featured       bool         `json:"featured" gorm:"default:false"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`

	Template AgentTemplate `json:"template,omitempty" gorm:"foreignKey:TemplateID"`
	Creator  User          `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
}

func (l *AgentListing) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	if l.Screenshots == "" {
		l.Screenshots = "[]"
	}
	return nil
}

// ── Agent Purchase (购买记录) ──

type AgentPurchase struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ListingID  string    `json:"listing_id" gorm:"type:varchar(36);index;not null"`
	BuyerID    string    `json:"buyer_id" gorm:"type:varchar(36);index;not null"`
	CreatorID  string    `json:"creator_id" gorm:"type:varchar(36);index;not null"`
	PriceCents int       `json:"price_cents" gorm:"not null"`
	Currency   string    `json:"currency" gorm:"type:varchar(10);default:CNY"`
	Status     string    `json:"status" gorm:"type:varchar(20);default:completed;index"` // completed, refunded
	RefundedAt *time.Time `json:"refunded_at"`
	ExpiresAt  *time.Time `json:"expires_at"` // for subscriptions
	CreatedAt  time.Time  `json:"created_at"`

	Listing AgentListing `json:"listing,omitempty" gorm:"foreignKey:ListingID"`
	Buyer   User         `json:"buyer,omitempty" gorm:"foreignKey:BuyerID"`
}

func (p *AgentPurchase) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// ── Creator Revenue (创作者收益) ──

type CreatorRevenue struct {
	ID            string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	CreatorID     string    `json:"creator_id" gorm:"type:varchar(36);index;not null"`
	PurchaseID    string    `json:"purchase_id" gorm:"type:varchar(36);index;not null"`
	ListingID     string    `json:"listing_id" gorm:"type:varchar(36);index;not null"`
	GrossAmount   int       `json:"gross_amount" gorm:"not null"`   // total payment in cents
	PlatformFee   int       `json:"platform_fee" gorm:"not null"`   // 15% platform cut
	ReferralFee   int       `json:"referral_fee" gorm:"default:0"`  // 5% to referrer
	NetAmount     int       `json:"net_amount" gorm:"not null"`     // 80% to creator
	Currency      string    `json:"currency" gorm:"type:varchar(10);default:CNY"`
	Status        string    `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending, settled, paid_out
	SettledAt     *time.Time `json:"settled_at"`
	PaidOutAt     *time.Time `json:"paid_out_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (r *CreatorRevenue) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// ── Agent Rating (评价) ──

type AgentRating struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ListingID  string    `json:"listing_id" gorm:"type:varchar(36);index;not null"`
	UserID     string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Score      int       `json:"score" gorm:"not null"`       // 1-5 stars
	Title      string    `json:"title" gorm:"type:varchar(200)"`
	Comment    string    `json:"comment" gorm:"type:text"`
	Helpful    int       `json:"helpful" gorm:"default:0"`    // helpful votes
	Reported   bool      `json:"reported" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	User    User         `json:"user,omitempty" gorm:"foreignKey:UserID"`
	Listing AgentListing `json:"listing,omitempty" gorm:"foreignKey:ListingID"`
}

func (r *AgentRating) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// ── Agent Version (版本管理) ──

type AgentVersion struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	ListingID  string    `json:"listing_id" gorm:"type:varchar(36);index;not null"`
	Version    string    `json:"version" gorm:"type:varchar(20);not null"`
	Changelog  string    `json:"changelog" gorm:"type:text"`
	Config     string    `json:"config" gorm:"type:json"`       // snapshot of agent config at this version
	CreatedAt  time.Time `json:"created_at"`
}

func (v *AgentVersion) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.Config == "" {
		v.Config = "{}"
	}
	return nil
}

// ── Creator Profile (创作者资料) ──

type CreatorProfile struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);uniqueIndex;not null"`
	DisplayName  string    `json:"display_name" gorm:"type:varchar(100)"`
	Bio          string    `json:"bio" gorm:"type:text"`
	AvatarURL    string    `json:"avatar_url" gorm:"type:varchar(500)"`
	Website      string    `json:"website" gorm:"type:varchar(500)"`
	Verified     bool      `json:"verified" gorm:"default:false"`
	TotalEarned  int64     `json:"total_earned" gorm:"default:0"`  // lifetime earnings in cents
	AgentCount   int       `json:"agent_count" gorm:"default:0"`
	AvgRating    float64   `json:"avg_rating" gorm:"default:0"`
	PayoutMethod string    `json:"payout_method" gorm:"type:varchar(20)"` // alipay, bank
	PayoutInfo   string    `json:"payout_info" gorm:"type:text"`          // encrypted payout details
	Status       string    `json:"status" gorm:"type:varchar(20);default:active;index"` // active, suspended
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `json:"user,omitempty" gorm:"foreignKey:UserID"`
}

func (c *CreatorProfile) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
