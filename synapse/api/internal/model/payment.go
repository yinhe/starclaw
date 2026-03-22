package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentOrder tracks recharge orders (Alipay / WeChat Pay)
type PaymentOrder struct {
	ID              string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID          string         `json:"user_id" gorm:"type:varchar(36);index;not null"`
	OrderNo         string         `json:"order_no" gorm:"type:varchar(64);uniqueIndex"`           // our order number
	Channel         string         `json:"channel" gorm:"type:varchar(20)"`                        // alipay, wechat
	AmountCents     int64          `json:"amount_cents"`                                           // paid amount in cents (分)
	BonusCents      int64          `json:"bonus_cents" gorm:"default:0"`                           // bonus amount
	TotalCents      int64          `json:"total_cents"`                                            // amount + bonus credited
	Status          string         `json:"status" gorm:"type:varchar(20);default:'pending';index"` // pending, paid, failed, expired
	TradeNo         string         `json:"trade_no" gorm:"type:varchar(100)"`                      // third-party transaction ID
	PayURL          string         `json:"pay_url,omitempty" gorm:"-"`                             // payment URL (not stored)
	Purpose         string         `json:"purpose" gorm:"type:varchar(20);default:'recharge'"`     // recharge, invest
	ExternalOrderNo string         `json:"external_order_no" gorm:"type:varchar(64)"`              // Queen's order_no (for invest)
	CallbackURL     string         `json:"-" gorm:"type:varchar(255)"`                             // callback URL on payment success
	PaidAt          *time.Time     `json:"paid_at"`
	CreatedAt       time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

func (o *PaymentOrder) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// RechargePackage defines a recharge option
type RechargePackage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AmountCents int64  `json:"amount_cents"` // pay this
	BonusCents  int64  `json:"bonus_cents"`  // get this extra
	TotalCents  int64  `json:"total_cents"`  // credited = amount + bonus
}

// DefaultPackages returns the available recharge packages
func DefaultPackages() []RechargePackage {
	return []RechargePackage{
		{ID: "pkg_10", Name: "¥10", AmountCents: 1000, BonusCents: 0, TotalCents: 1000},
		{ID: "pkg_50", Name: "¥50", AmountCents: 5000, BonusCents: 0, TotalCents: 5000},
		{ID: "pkg_100", Name: "¥100", AmountCents: 10000, BonusCents: 0, TotalCents: 10000},
		{ID: "pkg_500", Name: "¥500", AmountCents: 50000, BonusCents: 0, TotalCents: 50000},
		{ID: "pkg_1000", Name: "¥1000", AmountCents: 100000, BonusCents: 0, TotalCents: 100000},
	}
}
