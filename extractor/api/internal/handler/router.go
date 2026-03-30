package handler

import (
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/engine"
)

func Setup(r *gin.Engine, db *gorm.DB, bridgeURL string) {
	bc := bridge.NewClient(bridgeURL)
	riskCtrl := engine.NewRiskController(db, bc)
	scheduler := engine.NewScheduler(db, bc, riskCtrl)

	// Claw AI client (optional — if configured)
	var clawClient *engine.ClawClient
	clawURL := os.Getenv("EXTRACTOR_CLAW_URL")
	clawKey := os.Getenv("EXTRACTOR_CLAW_API_KEY")
	if clawURL != "" {
		clawClient = engine.NewClawClient(clawURL, clawKey)
	}

	strategyH := &StrategyHandler{DB: db, Bridge: bc, Scheduler: scheduler}
	tradeH := &TradeHandler{DB: db, Bridge: bc}
	riskH := &RiskHandler{DB: db, RiskCtrl: riskCtrl}
	pnlH := &PnLHandler{DB: db}
	backtestH := &BacktestHandler{DB: db}
	monitorH := &MonitorHandler{DB: db, Bridge: bc}
	settlementH := &SettlementHandler{DB: db}
	callbackH := &CallbackHandler{DB: db, RiskCtrl: riskCtrl}
	clawH := &ClawConfirmHandler{DB: db, Claw: clawClient}
	scanH := &ScanHandler{DB: db, Bridge: bc}

	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "extractor-api"})
	})

	v1 := r.Group("/v1")
	{
		// Strategy management
		v1.GET("/strategies", strategyH.List)
		v1.POST("/strategies", strategyH.Create)
		v1.GET("/strategies/:id", strategyH.Get)
		v1.PUT("/strategies/:id", strategyH.Update)
		v1.DELETE("/strategies/:id", strategyH.Delete)
		v1.POST("/strategies/:id/start", strategyH.Start)
		v1.POST("/strategies/:id/stop", strategyH.Stop)

		// Trading
		v1.GET("/orders", tradeH.ListOrders)
		v1.POST("/orders", tradeH.SubmitOrder)
		v1.POST("/orders/:id/cancel", tradeH.CancelOrder)
		v1.GET("/trades", tradeH.ListTrades)
		v1.GET("/positions", tradeH.ListPositions)
		v1.GET("/positions/:account", tradeH.GetAccountPositions)

		// Risk
		v1.GET("/risk/rules", riskH.ListRules)
		v1.POST("/risk/rules", riskH.CreateRule)
		v1.PUT("/risk/rules/:id", riskH.UpdateRule)
		v1.DELETE("/risk/rules/:id", riskH.DeleteRule)
		v1.GET("/risk/alerts", riskH.ListAlerts)
		v1.POST("/risk/alerts/:id/resolve", riskH.ResolveAlert)

		// P&L
		v1.GET("/pnl/daily", pnlH.ListDaily)
		v1.GET("/pnl/summary", pnlH.Summary)
		v1.GET("/pnl/curve", pnlH.Curve)

		// Backtest
		v1.POST("/backtest", backtestH.Submit)
		v1.GET("/backtest", backtestH.List)
		v1.GET("/backtest/:id", backtestH.Get)

		// Monitor
		v1.GET("/accounts", monitorH.ListAccounts)
		v1.GET("/accounts/:account", monitorH.GetAccount)
		v1.GET("/monitor/overview", monitorH.Overview)

		// Settlement
		v1.POST("/settlement/run", settlementH.RunDaily)
		v1.GET("/settlement/history", settlementH.History)

		// Scan (trigger Python strategy executor)
		v1.POST("/scan", scanH.TriggerScan)
		v1.GET("/scan/status", scanH.ScanStatus)
	}

	// Callbacks from Python Bridge
	cb := r.Group("/callback")
	{
		cb.POST("/order_filled", callbackH.OrderFilled)
		cb.POST("/market_tick", callbackH.MarketTick)
		cb.POST("/risk_alert", callbackH.RiskAlert)
		cb.POST("/strategy_signal", callbackH.StrategySignal)
	}

	// Claw AI secondary confirmation
	claw := r.Group("/v1/claw")
	{
		claw.POST("/confirm", clawH.Confirm)
		claw.GET("/status", clawH.Status)
	}
}
