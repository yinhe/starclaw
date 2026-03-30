package model

import "time"

type Order struct {
	ID           string    `json:"id" gorm:"primaryKey;size:36"`
	StrategyID   string    `json:"strategy_id" gorm:"size:36;index"`
	Account      string    `json:"account" gorm:"size:50;index"`
	Code         string    `json:"code" gorm:"size:20;index"` // stock code e.g. 600519
	Name         string    `json:"name" gorm:"size:50"`
	Direction    string    `json:"direction" gorm:"size:10"` // buy, sell
	OrderType    string    `json:"order_type" gorm:"size:20;default:limit"` // limit, market
	Price        float64   `json:"price"`
	Volume       int       `json:"volume"`
	FilledVolume int       `json:"filled_volume"`
	FilledPrice  float64   `json:"filled_price"`
	Status       string    `json:"status" gorm:"size:20;index;default:pending"` // pending, submitted, partial, filled, cancelled, rejected
	QMTOrderID   string    `json:"qmt_order_id" gorm:"size:100"`
	Remark       string    `json:"remark" gorm:"size:255"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Trade struct {
	ID         string    `json:"id" gorm:"primaryKey;size:36"`
	OrderID    string    `json:"order_id" gorm:"size:36;index"`
	StrategyID string    `json:"strategy_id" gorm:"size:36;index"`
	Account    string    `json:"account" gorm:"size:50;index"`
	Code       string    `json:"code" gorm:"size:20;index"`
	Name       string    `json:"name" gorm:"size:50"`
	Direction  string    `json:"direction" gorm:"size:10"`
	Price      float64   `json:"price"`
	Volume     int       `json:"volume"`
	Amount     float64   `json:"amount"`     // price * volume
	Commission float64  `json:"commission"`
	StampTax   float64   `json:"stamp_tax"`  // only on sell
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
}

type Position struct {
	ID          string    `json:"id" gorm:"primaryKey;size:36"`
	Account     string    `json:"account" gorm:"size:50;index"`
	Code        string    `json:"code" gorm:"size:20;index"`
	Name        string    `json:"name" gorm:"size:50"`
	Volume      int       `json:"volume"`
	AvailVolume int       `json:"avail_volume"` // T+1 sellable
	CostPrice   float64   `json:"cost_price"`
	MarketPrice float64   `json:"market_price"`
	PnLFloat    float64   `json:"pnl_float"`
	PnLRatio    float64   `json:"pnl_ratio"` // float pnl / cost
	UpdatedAt   time.Time `json:"updated_at"`
}
