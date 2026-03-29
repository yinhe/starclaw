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

// Commission rate tiers for the settlement engine.
// Tiers are based on cumulative actual consumption margin (from profit-split),
// NOT on CRM deal values or recharge amounts.
//
// DirectRate:  applied to margin from directly-signed clients (Team Partner)
// MgmtFeeRate: applied to city partner commissions under this team partner
//
// Bronze:  direct 10% / mgmt 3%
// Silver:  direct 15% / mgmt 5%  (cumulative margin ≥ ¥50,000)
// Gold:    direct 20% / mgmt 8%  (cumulative margin ≥ ¥200,000)
// Diamond: direct 25% / mgmt 10% (cumulative margin ≥ ¥500,000)

type CommissionTier struct {
	Name        string
	MinMargin   int64   // minimum cumulative actual margin to qualify (分)
	DirectRate  float64 // commission rate on margin for directly-signed clients
	MgmtFeeRate float64 // management fee rate on city partner commissions
}

// DefaultCommissionTiers returns the tiered commission rate schedule
func DefaultCommissionTiers() []CommissionTier {
	return []CommissionTier{
		{Name: "bronze", MinMargin: 0, DirectRate: 0.10, MgmtFeeRate: 0.03},
		{Name: "silver", MinMargin: 50000_00, DirectRate: 0.15, MgmtFeeRate: 0.05},   // ¥50,000+
		{Name: "gold", MinMargin: 200000_00, DirectRate: 0.20, MgmtFeeRate: 0.08},    // ¥200,000+
		{Name: "diamond", MinMargin: 500000_00, DirectRate: 0.25, MgmtFeeRate: 0.10}, // ¥500,000+
	}
}

// GetCommissionTier returns the applicable tier for a given cumulative margin
func GetCommissionTier(cumulativeMargin int64) CommissionTier {
	tiers := DefaultCommissionTiers()
	result := tiers[0]
	for _, t := range tiers {
		if cumulativeMargin >= t.MinMargin {
			result = t
		}
	}
	return result
}
