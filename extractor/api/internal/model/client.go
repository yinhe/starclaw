package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ClientAccount links a Claw user to their QMT trading account.
// Q8bot trades on the client's own account; monthly profit sharing is the revenue model.
type ClientAccount struct {
	ID             string     `json:"id" gorm:"primaryKey;size:36"`
	ClawID         string     `json:"claw_id" gorm:"size:100;index"`                   // Claw node identity
	UserID         string     `json:"user_id" gorm:"size:36;index"`                    // Claw user ID
	QMTAccount     string     `json:"qmt_account" gorm:"size:50;uniqueIndex"`          // client's QMT account
	Name           string     `json:"name" gorm:"size:100"`                            // client display name
	Phone          string     `json:"phone" gorm:"size:20"`                            // contact phone
	CommissionRate float64    `json:"commission_rate" gorm:"default:0.20"`             // platform profit share (e.g. 0.20 = 20%)
	HighWaterMark  float64    `json:"high_water_mark"`                                 // historical peak NAV for HWM billing
	InitialNAV     float64    `json:"initial_nav"`                                     // NAV at onboarding
	Source         string     `json:"source" gorm:"size:30;default:direct"`            // direct / marketplace (bought Q8bot agent)
	TemplateID     string     `json:"template_id" gorm:"size:36"`                      // marketplace template ID if source=marketplace
	Strategy       string     `json:"strategy" gorm:"size:50;default:trend_main_wave"` // assigned strategy
	Status         string     `json:"status" gorm:"size:20;default:active;index"`      // active / paused / terminated
	ActivatedAt    *time.Time `json:"activated_at"`
	TerminatedAt   *time.Time `json:"terminated_at"`
	Note           string     `json:"note" gorm:"size:500"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (c *ClientAccount) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// MonthlyBill is the monthly settlement record for a client.
// Billing uses High Water Mark: only profit above the historical peak is billable.
type MonthlyBill struct {
	ID              string     `json:"id" gorm:"primaryKey;size:36"`
	ClientID        string     `json:"client_id" gorm:"size:36;index"`
	ClientName      string     `json:"client_name" gorm:"size:100"`
	QMTAccount      string     `json:"qmt_account" gorm:"size:50"`
	Month           string     `json:"month" gorm:"size:7;index"` // "2026-04"
	StartNAV        float64    `json:"start_nav"`                 // NAV at month start
	EndNAV          float64    `json:"end_nav"`                   // NAV at month end
	GrossProfit     float64    `json:"gross_profit"`              // end_nav - start_nav
	HighWaterMark   float64    `json:"high_water_mark"`           // HWM at billing time
	BillableProfit  float64    `json:"billable_profit"`           // max(0, end_nav - high_water_mark)
	CommissionRate  float64    `json:"commission_rate"`            // snapshot of rate at billing
	ServiceFee      float64    `json:"service_fee"`               // billable_profit × commission_rate (¥)
	ServiceFeeCents int64      `json:"service_fee_cents"`         // service_fee in cents (分)
	TradeCount      int        `json:"trade_count"`               // number of trades this month
	WinRate         float64    `json:"win_rate"`                  // win rate this month
	PaymentMethod   string     `json:"payment_method" gorm:"size:20"` // synapse / offline
	PaymentOrderNo  string     `json:"payment_order_no" gorm:"size:100"`
	PaymentRef      string     `json:"payment_ref" gorm:"size:200"`       // offline transfer reference / screenshot
	PayURL          string     `json:"pay_url" gorm:"size:500"`           // Synapse payment URL
	StarEnergyUnits int64      `json:"star_energy_units"`                 // converted star energy
	QueenTxID       string     `json:"queen_tx_id" gorm:"size:100"`      // Queen credit grant tx ID
	Status          string     `json:"status" gorm:"size:30;default:draft;index"` // draft/sent/pending_payment/paid/star_injected/disputed/cancelled
	SentAt          *time.Time `json:"sent_at"`
	PaidAt          *time.Time `json:"paid_at"`
	ConfirmedBy     string     `json:"confirmed_by" gorm:"size:50"` // admin who confirmed offline payment
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (b *MonthlyBill) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// NAVSnapshot records daily NAV for a client account (for month-start/end reference).
type NAVSnapshot struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	ClientID   string    `json:"client_id" gorm:"size:36;index"`
	QMTAccount string    `json:"qmt_account" gorm:"size:50;index"`
	Date       string    `json:"date" gorm:"size:10;index"` // "2026-04-01"
	TotalNAV   float64   `json:"total_nav"`                 // total assets
	Available  float64   `json:"available"`
	MarketVal  float64   `json:"market_val"`
	Positions  int       `json:"positions"` // number of holdings
	CreatedAt  time.Time `json:"created_at"`
}

func (n *NAVSnapshot) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
