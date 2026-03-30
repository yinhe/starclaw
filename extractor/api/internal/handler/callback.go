package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/engine"
	"starclaw.net/extractor/api/internal/model"
)

// CallbackHandler receives async callbacks from the Python Bridge.
type CallbackHandler struct {
	DB       *gorm.DB
	RiskCtrl *engine.RiskController
}

func (h *CallbackHandler) OrderFilled(c *gin.Context) {
	var req struct {
		QMTOrderID string  `json:"qmt_order_id"`
		OrderID    string  `json:"order_id"`
		FillPrice  float64 `json:"fill_price"`
		FillVolume int     `json:"fill_volume"`
		Commission float64 `json:"commission"`
		StampTax   float64 `json:"stamp_tax"`
		Timestamp  string  `json:"timestamp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find and update the order
	var order model.Order
	if req.OrderID != "" {
		h.DB.First(&order, "id = ?", req.OrderID)
	} else if req.QMTOrderID != "" {
		h.DB.First(&order, "qmt_order_id = ?", req.QMTOrderID)
	}

	if order.ID != "" {
		order.FilledPrice = req.FillPrice
		order.FilledVolume += req.FillVolume
		if order.FilledVolume >= order.Volume {
			order.Status = "filled"
		} else {
			order.Status = "partial"
		}
		h.DB.Save(&order)

		// Create trade record
		trade := model.Trade{
			ID:         uuid.New().String(),
			OrderID:    order.ID,
			StrategyID: order.StrategyID,
			Account:    order.Account,
			Code:       order.Code,
			Name:       order.Name,
			Direction:  order.Direction,
			Price:      req.FillPrice,
			Volume:     req.FillVolume,
			Amount:     req.FillPrice * float64(req.FillVolume),
			Commission: req.Commission,
			StampTax:   req.StampTax,
			CreatedAt:  time.Now(),
		}
		h.DB.Create(&trade)

		// Risk check
		h.RiskCtrl.OnOrderFilled(&trade)

		log.Printf("[callback] order filled: %s %s %s %.2f x %d", order.Account, order.Direction, order.Code, req.FillPrice, req.FillVolume)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *CallbackHandler) MarketTick(c *gin.Context) {
	// TODO: update position market prices, check risk thresholds
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *CallbackHandler) RiskAlert(c *gin.Context) {
	var req struct {
		Account string `json:"account"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alert := model.RiskAlert{
		ID:        uuid.New().String(),
		Account:   req.Account,
		Message:   req.Message,
		Severity:  "warn",
		CreatedAt: time.Now(),
	}
	h.DB.Create(&alert)
	log.Printf("[callback] risk alert: %s - %s", req.Account, req.Message)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *CallbackHandler) StrategySignal(c *gin.Context) {
	// TODO: receive strategy signals and route to order execution
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
