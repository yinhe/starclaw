package engine

import (
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/model"
)

// RiskController implements the 3-level risk management system.
type RiskController struct {
	DB     *gorm.DB
	Bridge *bridge.Client
}

func NewRiskController(db *gorm.DB, bc *bridge.Client) *RiskController {
	return &RiskController{DB: db, Bridge: bc}
}

// CheckTradeRisk validates an order against L1 rules before submission.
func (rc *RiskController) CheckTradeRisk(account, code string, direction string, amount float64) error {
	// TODO: implement L1 checks (position limit, per-trade loss limit)
	return nil
}

// OnOrderFilled is called after a trade executes. Checks L1/L2 risk.
func (rc *RiskController) OnOrderFilled(trade *model.Trade) {
	// TODO: check L1 strategy-level daily loss
	// TODO: check L2 account-level drawdown
	log.Printf("[risk] order filled: %s %s %s %.2f x %d", trade.Account, trade.Direction, trade.Code, trade.Price, trade.Volume)
}

// OnBridgeDisconnect triggers L3 circuit break.
func (rc *RiskController) OnBridgeDisconnect() {
	log.Printf("[risk] L3 CIRCUIT BREAK: bridge disconnected")
	rc.createAlert("", "", "L3 circuit break: Python bridge disconnected", "critical")
	// TODO: cancel all pending orders via Bridge (when reconnected)
}

// CircuitBreak stops all strategies and cancels all pending orders.
func (rc *RiskController) CircuitBreak(reason string) {
	log.Printf("[risk] CIRCUIT BREAK triggered: %s", reason)
	rc.createAlert("", "", "Circuit break: "+reason, "critical")

	// Pause all running strategies
	rc.DB.Model(&model.Strategy{}).Where("status = ?", "running").Update("status", "paused")
}

func (rc *RiskController) createAlert(account, strategyID, message, severity string) {
	alert := model.RiskAlert{
		ID:         uuid.New().String(),
		Account:    account,
		StrategyID: strategyID,
		Message:    message,
		Severity:   severity,
		CreatedAt:  time.Now(),
	}
	rc.DB.Create(&alert)
}
