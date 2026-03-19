package model

import "time"

// SettlementBill represents a monthly settlement bill for a partner (core or city)
type SettlementBill struct {
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	PartnerID    string     `json:"partner_id" gorm:"type:varchar(36);index;not null"`
	PartnerType  string     `json:"partner_type" gorm:"type:varchar(20)"` // core / city
	PartnerName  string     `json:"partner_name" gorm:"type:varchar(200)"`
	Month        string     `json:"month" gorm:"type:varchar(7);index;not null"`  // YYYY-MM
	TotalAmount  int64      `json:"total_amount"`                                 // total settlement amount (分)
	SalaryAmount int64      `json:"salary_amount"`                                // base salary portion (分), core only
	DirectAmount int64      `json:"direct_amount"`                                // direct commission (分)
	ManageAmount int64      `json:"manage_amount"`                                // management fee from city partners (分), core only
	EquityAmount int64      `json:"equity_amount"`                                // equity profit share (分), core only
	CityAmount   int64      `json:"city_amount"`                                  // city partner commission (分), city only
	ItemCount    int        `json:"item_count"`                                   // number of line items
	Status       string     `json:"status" gorm:"type:varchar(20);default:draft"` // draft / pending_review / approved / paid / rejected
	ReviewedBy   string     `json:"reviewed_by" gorm:"type:varchar(100)"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	ReviewNote   string     `json:"review_note" gorm:"type:varchar(500)"`
	PaidAt       *time.Time `json:"paid_at"`
	PayMethod    string     `json:"pay_method" gorm:"type:varchar(20)"` // bank / alipay
	PayAccount   string     `json:"pay_account" gorm:"type:varchar(200)"`
	PayRef       string     `json:"pay_ref" gorm:"type:varchar(200)"` // payment reference/transaction ID
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SettlementLineItem represents one commission line in a settlement bill
type SettlementLineItem struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	BillID      string    `json:"bill_id" gorm:"type:varchar(36);index;not null"`
	PartnerID   string    `json:"partner_id" gorm:"type:varchar(36);index"`
	SourceType  string    `json:"source_type" gorm:"type:varchar(20)"` // salary / direct_new / direct_renew / manage_fee / city_new / city_renew
	SourceID    string    `json:"source_id" gorm:"type:varchar(36)"`   // CRMDeal ID or CityClient ID or Commission ID
	ClientName  string    `json:"client_name" gorm:"type:varchar(200)"`
	BaseAmount  int64     `json:"base_amount"` // original revenue (分)
	Rate        float64   `json:"rate"`        // commission rate applied
	Amount      int64     `json:"amount"`      // calculated commission (分)
	Description string    `json:"description" gorm:"type:varchar(500)"`
	CreatedAt   time.Time `json:"created_at"`
}

// SettlementConfig stores settlement engine configuration
type SettlementConfig struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Key         string    `json:"key" gorm:"type:varchar(100);uniqueIndex;not null"`
	Value       string    `json:"value" gorm:"type:varchar(500)"`
	Description string    `json:"description" gorm:"type:varchar(500)"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Commission rate tiers for the settlement engine
// Bronze: first-year 40% / renewal 25%
// Silver: first-year 45% / renewal 30%
// Gold:   first-year 50% / renewal 35%
// Diamond: first-year 55% / renewal 40%

type CommissionTier struct {
	Name          string
	MinRevenue    int64 // minimum lifetime revenue to qualify (分)
	FirstYearRate float64
	RenewalRate   float64
}

// DefaultCommissionTiers returns the tiered commission rate schedule
func DefaultCommissionTiers() []CommissionTier {
	return []CommissionTier{
		{Name: "bronze", MinRevenue: 0, FirstYearRate: 0.40, RenewalRate: 0.25},
		{Name: "silver", MinRevenue: 50000_00, FirstYearRate: 0.45, RenewalRate: 0.30},   // ¥50,000+
		{Name: "gold", MinRevenue: 200000_00, FirstYearRate: 0.50, RenewalRate: 0.35},    // ¥200,000+
		{Name: "diamond", MinRevenue: 500000_00, FirstYearRate: 0.55, RenewalRate: 0.40}, // ¥500,000+
	}
}

// GetCommissionTier returns the applicable tier for a given lifetime revenue
func GetCommissionTier(lifetimeRevenue int64) CommissionTier {
	tiers := DefaultCommissionTiers()
	result := tiers[0]
	for _, t := range tiers {
		if lifetimeRevenue >= t.MinRevenue {
			result = t
		}
	}
	return result
}
