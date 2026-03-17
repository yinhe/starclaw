package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Plan represents a subscription plan (Community, Starter, Pro, Enterprise, White-Label)
type Plan struct {
	ID           string  `json:"id" gorm:"type:varchar(36);primaryKey"`
	Name         string  `json:"name" gorm:"type:varchar(50);uniqueIndex;not null"`       // community, starter, pro, enterprise, whitelabel
	DisplayName  string  `json:"display_name" gorm:"type:varchar(100)"`                   // "Community", "Starter", ...
	PriceMonthly int     `json:"price_monthly" gorm:"default:0"`                          // ¥ cents (e.g. 49900 = ¥499)
	PriceYearly  int     `json:"price_yearly" gorm:"default:0"`                           // ¥ cents per month when paid yearly
	MaxNodes     int     `json:"max_nodes" gorm:"default:10"`                             // node cap
	MaxTeams     int     `json:"max_teams" gorm:"default:1"`                              // team cap; 0 = unlimited
	MaxTokensDay int64   `json:"max_tokens_day" gorm:"default:0"`                         // daily token limit; 0 = unlimited
	Features     string  `json:"features" gorm:"type:text"`                               // JSON feature flags
	SortOrder    int     `json:"sort_order" gorm:"default:0"`
	Active       bool    `json:"active" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (p *Plan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// Subscription links a team (or global instance) to a plan
type Subscription struct {
	ID            string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	TeamID        string     `json:"team_id" gorm:"type:varchar(36);index"`           // empty = instance-level subscription
	PlanID        string     `json:"plan_id" gorm:"type:varchar(36);index;not null"`
	PlanName      string     `json:"plan_name" gorm:"type:varchar(50)"`               // denormalized for convenience
	Status        string     `json:"status" gorm:"type:varchar(20);default:active"`   // active, cancelled, expired, trial
	BillingCycle  string     `json:"billing_cycle" gorm:"type:varchar(10);default:monthly"` // monthly, yearly
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	CancelledAt   *time.Time `json:"cancelled_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (s *Subscription) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// UsageRecord tracks token/request consumption per team/user/model
type UsageRecord struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TeamID       string    `json:"team_id" gorm:"type:varchar(36);index"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);index"`       // AdminUser.ID or web user
	ClawID       string    `json:"claw_id" gorm:"type:varchar(36);index"`       // which Claw node served
	ModelName    string    `json:"model_name" gorm:"type:varchar(100);index"`   // e.g. gpt-4o, deepseek-v3
	InputTokens  int64     `json:"input_tokens" gorm:"default:0"`
	OutputTokens int64     `json:"output_tokens" gorm:"default:0"`
	TotalTokens  int64     `json:"total_tokens" gorm:"default:0"`
	CostCents    int       `json:"cost_cents" gorm:"default:0"`                 // cost in ¥ cents
	StarEnergy   int       `json:"star_energy" gorm:"default:0"`                // star energy units consumed
	RequestType  string    `json:"request_type" gorm:"type:varchar(30)"`        // chat, completion, embedding, tool_call
	DurationMs   int       `json:"duration_ms" gorm:"default:0"`
	Date         string    `json:"date" gorm:"type:varchar(10);index"`          // YYYY-MM-DD for daily aggregation
	CreatedAt    time.Time `json:"created_at"`
}

// UsageDailySummary is an aggregated daily summary per team
type UsageDailySummary struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TeamID       string    `json:"team_id" gorm:"type:varchar(36);index"`
	Date         string    `json:"date" gorm:"type:varchar(10);index"`
	TotalRequests int64    `json:"total_requests" gorm:"default:0"`
	TotalTokens  int64     `json:"total_tokens" gorm:"default:0"`
	InputTokens  int64     `json:"input_tokens" gorm:"default:0"`
	OutputTokens int64     `json:"output_tokens" gorm:"default:0"`
	TotalCostCents int     `json:"total_cost_cents" gorm:"default:0"`
	TotalStarEnergy int    `json:"total_star_energy" gorm:"default:0"`
	UniqueUsers  int       `json:"unique_users" gorm:"default:0"`
	UniqueModels int       `json:"unique_models" gorm:"default:0"`
	AvgLatencyMs int       `json:"avg_latency_ms" gorm:"default:0"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BudgetAlert defines a spending/usage threshold alert
type BudgetAlert struct {
	ID            string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TeamID        string    `json:"team_id" gorm:"type:varchar(36);index"`
	Name          string    `json:"name" gorm:"type:varchar(100);not null"`
	MetricType    string    `json:"metric_type" gorm:"type:varchar(30);not null"` // tokens, cost, star_energy, requests
	ThresholdValue int64   `json:"threshold_value" gorm:"not null"`               // threshold amount
	Period        string    `json:"period" gorm:"type:varchar(10);default:daily"` // daily, monthly
	NotifyEmail   string    `json:"notify_email" gorm:"type:varchar(255)"`
	NotifyWebhook string    `json:"notify_webhook" gorm:"type:varchar(500)"`      // webhook URL
	Enabled       bool      `json:"enabled" gorm:"default:true"`
	LastTriggered *time.Time `json:"last_triggered"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (b *BudgetAlert) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}
