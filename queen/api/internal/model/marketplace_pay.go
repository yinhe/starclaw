package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MarketplacePayOrder tracks a payment order for purchasing a paid marketplace template.
// Created by Claw nodes via Queen internal API, payment processed by Synapse (Alipay/WeChat).
type MarketplacePayOrder struct {
	ID           string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	OrderNo      string     `json:"order_no" gorm:"type:varchar(64);uniqueIndex"`
	ClawID       string     `json:"claw_id" gorm:"type:varchar(60);index"`       // purchasing Claw node
	UserID       string     `json:"user_id" gorm:"type:varchar(36);index"`       // user on the Claw node
	TemplateID   string     `json:"template_id" gorm:"type:varchar(36);index"`   // marketplace template
	TemplateName string     `json:"template_name" gorm:"type:varchar(200)"`
	Amount       int64      `json:"amount" gorm:"not null"`                      // price in cents (分)
	PayMethod    string     `json:"pay_method" gorm:"type:varchar(20)"`          // alipay / wechatpay
	PayForm      string     `json:"pay_form" gorm:"type:varchar(10)"`            // pc / h5 / native
	Status       string     `json:"status" gorm:"type:varchar(20);default:pending;index"` // pending / paid / expired / failed
	Subject      string     `json:"subject" gorm:"type:varchar(200)"`
	TradeNo      string     `json:"trade_no" gorm:"type:varchar(100)"`           // third-party transaction ID
	CallbackRaw  string     `json:"-" gorm:"type:text"`
	ExpireAt     *time.Time `json:"expire_at"`
	PaidAt       *time.Time `json:"paid_at"`
	CreatedAt    time.Time  `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (o *MarketplacePayOrder) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}
