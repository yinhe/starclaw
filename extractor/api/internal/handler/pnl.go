package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/model"
)

type PnLHandler struct {
	DB *gorm.DB
}

func (h *PnLHandler) ListDaily(c *gin.Context) {
	var list []model.DailyPnL
	q := h.DB.Order("date desc").Limit(60)
	if account := c.Query("account"); account != "" {
		q = q.Where("account = ?", account)
	}
	if strategyID := c.Query("strategy_id"); strategyID != "" {
		q = q.Where("strategy_id = ?", strategyID)
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *PnLHandler) Summary(c *gin.Context) {
	type result struct {
		TotalPnL      float64 `json:"total_pnl"`
		TotalComm     float64 `json:"total_commission"`
		TotalTax      float64 `json:"total_stamp_tax"`
		TotalNet      float64 `json:"total_net"`
		TradingDays   int64   `json:"trading_days"`
	}
	var r result
	h.DB.Model(&model.DailyPnL{}).Select(
		"COALESCE(SUM(pnl_realized),0) as total_pnl, "+
			"COALESCE(SUM(commission),0) as total_comm, "+
			"COALESCE(SUM(stamp_tax),0) as total_tax, "+
			"COALESCE(SUM(net_pnl),0) as total_net, "+
			"COUNT(DISTINCT date) as trading_days",
	).Scan(&r)
	c.JSON(http.StatusOK, r)
}

func (h *PnLHandler) Curve(c *gin.Context) {
	days := c.DefaultQuery("days", "30")
	var list []model.DailyPnL
	h.DB.Where("date >= (SELECT MAX(date) FROM daily_pn_ls) - INTERVAL '"+days+" days'").
		Order("date asc").
		Find(&list)
	c.JSON(http.StatusOK, list)
}
