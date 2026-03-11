package model

import "time"

// UserBalance tracks user's recharge balance
type UserBalance struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);uniqueIndex"`
	Balance   int64     `json:"balance" gorm:"default:0"`   // 可用余额，单位：分
	Frozen    int64     `json:"frozen" gorm:"default:0"`    // 冻结金额（赏金托管），单位：分
	TotalIn   int64     `json:"total_in" gorm:"default:0"`  // 累计充值，单位：分
	TotalOut  int64     `json:"total_out" gorm:"default:0"` // 累计消费，单位：分
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RechargeOrder represents a payment order
type RechargeOrder struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(36)"`
	OrderNo     string     `json:"order_no" gorm:"type:varchar(64);uniqueIndex"` // 商户订单号
	UserID      string     `json:"user_id" gorm:"type:varchar(36);index"`
	Amount      int64      `json:"amount" gorm:"not null"`                         // 支付金额，单位：分
	BonusAmount int64      `json:"bonus_amount" gorm:"default:0"`                  // 赠送金额，单位：分
	PayMethod   string     `json:"pay_method" gorm:"type:varchar(20)"`             // alipay / wechatpay
	PayForm     string     `json:"pay_form" gorm:"type:varchar(20);default:h5"`    // pc / h5 / native / app
	Status      string     `json:"status" gorm:"type:varchar(20);default:pending"` // pending / paid / failed / closed / refunded
	TradeNo     string     `json:"trade_no" gorm:"type:varchar(128)"`              // 第三方交易号
	Subject     string     `json:"subject" gorm:"type:varchar(200)"`               // 订单标题
	PaidAt      *time.Time `json:"paid_at"`
	ExpireAt    *time.Time `json:"expire_at"`          // 订单过期时间
	CallbackRaw string     `json:"-" gorm:"type:text"` // 回调原始数据
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BalanceTransaction records every balance change
type BalanceTransaction struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	OrderNo   string    `json:"order_no" gorm:"type:varchar(64);index"` // 关联订单号（充值时）
	Type      string    `json:"type" gorm:"type:varchar(20)"`           // recharge / consume / bonus / refund / admin_adjust
	Amount    int64     `json:"amount"`                                 // 变动金额（正=入，负=出），单位：分
	Before    int64     `json:"before"`                                 // 变动前余额
	After     int64     `json:"after"`                                  // 变动后余额
	Remark    string    `json:"remark" gorm:"type:varchar(500)"`
	CreatedAt time.Time `json:"created_at"`
}

// RechargePackage defines available recharge options
type RechargePackage struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name        string    `json:"name" gorm:"type:varchar(100)"` // 套餐名称
	Amount      int64     `json:"amount"`                        // 充值金额，单位：分
	BonusAmount int64     `json:"bonus_amount" gorm:"default:0"` // 赠送金额，单位：分
	BonusRate   float64   `json:"bonus_rate" gorm:"default:0"`   // 赠送比例 0.1 = 10%
	SortOrder   int       `json:"sort_order" gorm:"default:0"`
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
