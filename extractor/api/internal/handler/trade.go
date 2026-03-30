package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/model"
)

type TradeHandler struct {
	DB     *gorm.DB
	Bridge *bridge.Client
}

func (h *TradeHandler) ListOrders(c *gin.Context) {
	var list []model.Order
	q := h.DB.Order("created_at desc").Limit(100)
	if account := c.Query("account"); account != "" {
		q = q.Where("account = ?", account)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *TradeHandler) SubmitOrder(c *gin.Context) {
	var req struct {
		StrategyID string  `json:"strategy_id"`
		Account    string  `json:"account"`
		Code       string  `json:"code"`
		Direction  string  `json:"direction"`
		Price      float64 `json:"price"`
		Volume     int     `json:"volume"`
		OrderType  string  `json:"order_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := model.Order{
		ID:         uuid.New().String(),
		StrategyID: req.StrategyID,
		Account:    req.Account,
		Code:       req.Code,
		Direction:  req.Direction,
		OrderType:  req.OrderType,
		Price:      req.Price,
		Volume:     req.Volume,
		Status:     "pending",
	}

	// Submit to QMT via bridge
	resp, err := h.Bridge.SubmitOrder(bridge.SubmitOrderReq{
		Account:   req.Account,
		Code:      req.Code,
		Direction: req.Direction,
		Price:     req.Price,
		Volume:    req.Volume,
		OrderType: req.OrderType,
	})
	if err != nil {
		order.Status = "rejected"
		order.Remark = err.Error()
		h.DB.Create(&order)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	order.QMTOrderID = resp.QMTOrderID
	order.Status = "submitted"
	h.DB.Create(&order)
	c.JSON(http.StatusOK, order)
}

func (h *TradeHandler) CancelOrder(c *gin.Context) {
	var order model.Order
	if err := h.DB.First(&order, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.Bridge.CancelOrder(order.Account, order.QMTOrderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	order.Status = "cancelled"
	h.DB.Save(&order)
	c.JSON(http.StatusOK, order)
}

func (h *TradeHandler) ListTrades(c *gin.Context) {
	var list []model.Trade
	q := h.DB.Order("created_at desc").Limit(100)
	if account := c.Query("account"); account != "" {
		q = q.Where("account = ?", account)
	}
	q.Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *TradeHandler) ListPositions(c *gin.Context) {
	var list []model.Position
	q := h.DB
	if account := c.Query("account"); account != "" {
		q = q.Where("account = ?", account)
	}
	q.Where("volume > 0").Find(&list)
	c.JSON(http.StatusOK, list)
}

func (h *TradeHandler) GetAccountPositions(c *gin.Context) {
	var list []model.Position
	h.DB.Where("account = ? AND volume > 0", c.Param("account")).Find(&list)
	c.JSON(http.StatusOK, list)
}
