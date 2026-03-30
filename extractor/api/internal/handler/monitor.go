package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/model"
)

type MonitorHandler struct {
	DB     *gorm.DB
	Bridge *bridge.Client
}

func (h *MonitorHandler) ListAccounts(c *gin.Context) {
	var list []model.AccountBinding
	h.DB.Order("qmt_account asc").Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *MonitorHandler) GetAccount(c *gin.Context) {
	account := c.Param("account")

	var binding model.AccountBinding
	if err := h.DB.Where("qmt_account = ?", account).First(&binding).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	// Try to get live data from bridge
	info, err := h.Bridge.GetAccountInfo(account)
	if err == nil {
		binding.TotalAssets = info.TotalAssets
		binding.Available = info.Available
		binding.Frozen = info.Frozen
		h.DB.Save(&binding)
	}

	c.JSON(http.StatusOK, binding)
}

func (h *MonitorHandler) Overview(c *gin.Context) {
	var accountCount int64
	h.DB.Model(&model.AccountBinding{}).Where("status = ?", "active").Count(&accountCount)

	var strategyRunning int64
	h.DB.Model(&model.Strategy{}).Where("status = ?", "running").Count(&strategyRunning)

	var strategyStopped int64
	h.DB.Model(&model.Strategy{}).Where("status = ?", "stopped").Count(&strategyStopped)

	var todayTrades int64
	h.DB.Model(&model.Trade{}).Where("DATE(created_at) = CURRENT_DATE").Count(&todayTrades)

	var unresolvedAlerts int64
	h.DB.Model(&model.RiskAlert{}).Where("resolved = false").Count(&unresolvedAlerts)

	bridgeOK := h.Bridge.Ping() == nil

	c.JSON(http.StatusOK, gin.H{
		"accounts_active":    accountCount,
		"strategies_running": strategyRunning,
		"strategies_stopped": strategyStopped,
		"today_trades":       todayTrades,
		"unresolved_alerts":  unresolvedAlerts,
		"bridge_connected":   bridgeOK,
	})
}
