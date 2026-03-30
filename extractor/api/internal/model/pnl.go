package model

import "time"

type DailyPnL struct {
	ID            string    `json:"id" gorm:"primaryKey;size:36"`
	Date          string    `json:"date" gorm:"size:10;index"`     // YYYY-MM-DD
	Account       string    `json:"account" gorm:"size:50;index"`
	StrategyID    string    `json:"strategy_id" gorm:"size:36;index"`
	PnLRealized   float64   `json:"pnl_realized"`
	PnLUnrealized float64   `json:"pnl_unrealized"`
	Commission    float64   `json:"commission"`
	StampTax      float64   `json:"stamp_tax"`
	NetPnL        float64   `json:"net_pnl"`   // realized - commission - stamp_tax
	NAV           float64   `json:"nav"`        // net asset value
	BenchReturn   float64   `json:"bench_return"` // benchmark (e.g. CSI300) return %
	CreatedAt     time.Time `json:"created_at"`
}
