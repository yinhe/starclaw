package model

import "time"

// PredictMarket represents a prediction market event (Polymarket, etc.)
type PredictMarket struct {
	ID        string     `json:"id" gorm:"primaryKey;size:36"`
	Source    string     `json:"source" gorm:"size:30;index"` // polymarket, crypto
	Title     string     `json:"title" gorm:"size:500"`
	Slug      string     `json:"slug" gorm:"size:200"`
	ExpiresAt *time.Time `json:"expires_at"`
	YesPrice  float64    `json:"yes_price"` // 0.00 - 1.00
	NoPrice   float64    `json:"no_price"`
	Volume    float64    `json:"volume"`
	Status    string     `json:"status" gorm:"size:20;default:active"` // active, resolved_yes, resolved_no, expired
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// PredictPosition represents a position in a prediction market
type PredictPosition struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	MarketID   string    `json:"market_id" gorm:"size:36;index"`
	Direction  string    `json:"direction" gorm:"size:5"` // yes, no
	Shares     float64   `json:"shares"`
	CostBasis  float64   `json:"cost_basis"`
	SettledPnL float64   `json:"settled_pnl"`
	Status     string    `json:"status" gorm:"size:20;default:open"` // open, closed, settled
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
