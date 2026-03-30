package model

import (
	"time"

	"gorm.io/gorm"
)

type RiskRule struct {
	ID        string `json:"id" gorm:"primaryKey;size:36"`
	Level     string `json:"level" gorm:"size:10;index"` // L1(strategy), L2(account), L3(system)
	Type      string `json:"type" gorm:"size:30"`        // max_loss_per_trade, daily_drawdown, position_limit, circuit_break, bridge_disconnect
	Threshold float64 `json:"threshold"`
	Action    string `json:"action" gorm:"size:20"` // warn, pause, stop, circuit_break
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
}

type RiskAlert struct {
	ID         string     `json:"id" gorm:"primaryKey;size:36"`
	RuleID     string     `json:"rule_id" gorm:"size:36;index"`
	Account    string     `json:"account" gorm:"size:50"`
	StrategyID string     `json:"strategy_id" gorm:"size:36"`
	Message    string     `json:"message" gorm:"size:500"`
	Severity   string     `json:"severity" gorm:"size:10"` // info, warn, critical
	Resolved   bool       `json:"resolved" gorm:"default:false"`
	ResolvedAt *time.Time `json:"resolved_at"`
	CreatedAt  time.Time  `json:"created_at" gorm:"index"`
}

func SeedRiskRules(db *gorm.DB) {
	rules := []RiskRule{
		// L1 Strategy
		{ID: "risk-l1-loss", Level: "L1", Type: "max_loss_per_trade", Threshold: 2.0, Action: "stop", Enabled: true},
		{ID: "risk-l1-daily", Level: "L1", Type: "strategy_daily_loss", Threshold: 3.0, Action: "pause", Enabled: true},
		{ID: "risk-l1-position", Level: "L1", Type: "single_stock_position", Threshold: 25.0, Action: "warn", Enabled: true},
		// L2 Account
		{ID: "risk-l2-drawdown", Level: "L2", Type: "account_daily_drawdown", Threshold: 3.0, Action: "pause", Enabled: true},
		{ID: "risk-l2-margin", Level: "L2", Type: "available_margin", Threshold: 20.0, Action: "warn", Enabled: true},
		{ID: "risk-l2-streak", Level: "L2", Type: "consecutive_loss_days", Threshold: 3.0, Action: "pause", Enabled: true},
		// L3 System
		{ID: "risk-l3-circuit", Level: "L3", Type: "total_daily_drawdown", Threshold: 5.0, Action: "circuit_break", Enabled: true},
		{ID: "risk-l3-bridge", Level: "L3", Type: "bridge_disconnect", Threshold: 30.0, Action: "circuit_break", Enabled: true},
	}
	for _, r := range rules {
		db.Where("id = ?", r.ID).FirstOrCreate(&r)
	}
}
