package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"starclaw.net/queen/api/internal/config"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

// MarketplacePaymentHandler handles direct Alipay/WeChat payment for marketplace Agent purchases.
// Flow: Claw → Queen (this) → Synapse (CreateInvestOrder) → pay_url → user pays → callback → done
type MarketplacePaymentHandler struct{}

func NewMarketplacePaymentHandler() *MarketplacePaymentHandler {
	return &MarketplacePaymentHandler{}
}

// POST /internal/marketplace/create-payment — Called by Claw nodes to create a payment order
func (h *MarketplacePaymentHandler) CreatePayment(c *gin.Context) {
	var req struct {
		ClawID       string `json:"claw_id" binding:"required"`
		UserID       string `json:"user_id"`
		TemplateID   string `json:"template_id" binding:"required"`
		TemplateName string `json:"template_name"`
		Amount       int64  `json:"amount" binding:"required"` // CNY in 分
		PayMethod    string `json:"pay_method" binding:"required"` // alipay / wechatpay
		PayForm      string `json:"pay_form"` // pc / h5
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields: " + err.Error()})
		return
	}
	if req.PayForm == "" {
		req.PayForm = "pc"
	}
	if req.Amount < 100 { // min ¥1
		c.JSON(http.StatusBadRequest, gin.H{"error": "金额不能低于 ¥1"})
		return
	}

	db := database.DB

	// Check for existing pending order for same template (dedup)
	var existing model.MarketplacePayOrder
	if err := db.Where("claw_id = ? AND template_id = ? AND status = ?", req.ClawID, req.TemplateID, "pending").
		First(&existing).Error; err == nil {
		// Existing pending order — check if expired
		if existing.ExpireAt != nil && existing.ExpireAt.Before(time.Now()) {
			db.Model(&existing).Update("status", "expired")
		} else {
			// Return the existing order (idempotent)
			c.JSON(http.StatusOK, gin.H{
				"order_no": existing.OrderNo,
				"status":   "existing_pending",
				"message":  "已有待支付订单，请完成支付或等待过期",
			})
			return
		}
	}

	// Create order
	orderNo := fmt.Sprintf("MP%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()/1000000)
	expire := time.Now().Add(30 * time.Minute)
	subject := fmt.Sprintf("购买智能体「%s」", req.TemplateName)
	if subject == "购买智能体「」" {
		subject = fmt.Sprintf("智能体市场购买 %s", req.TemplateID[:8])
	}

	order := model.MarketplacePayOrder{
		OrderNo:      orderNo,
		ClawID:       req.ClawID,
		UserID:       req.UserID,
		TemplateID:   req.TemplateID,
		TemplateName: req.TemplateName,
		Amount:       req.Amount,
		PayMethod:    req.PayMethod,
		PayForm:      req.PayForm,
		Status:       "pending",
		Subject:      subject,
		ExpireAt:     &expire,
	}
	if err := db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订单失败"})
		return
	}

	// Proxy to Synapse (same pattern as investor.createDiamondPayOrder)
	starAI := config.C.StarAI
	if starAI.URL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "支付通道尚未配置"})
		return
	}

	callbackBase := strings.TrimRight(starAI.CallbackBase, "/")
	if callbackBase == "" {
		if port := config.C.Server.Port; port != "" {
			callbackBase = fmt.Sprintf("http://queen-api:%s", port)
		} else {
			callbackBase = "http://queen-api:8085"
		}
	}
	callbackURL := callbackBase + "/internal/marketplace/payment-confirmed"

	body, _ := json.Marshal(map[string]interface{}{
		"channel":           order.PayMethod,
		"amount_cents":      order.Amount,
		"subject":           order.Subject,
		"external_order_no": order.OrderNo,
		"callback_url":      callbackURL,
		"pay_form":          order.PayForm,
	})

	httpReq, _ := http.NewRequest("POST", starAI.URL+"/internal/payment/invest-order", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if starAI.Token != "" {
		httpReq.Header.Set("X-Internal-Token", starAI.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[marketplace-pay] StarAI payment proxy error: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "支付通道暂不可用"})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("[marketplace-pay] StarAI payment returned %d: %s", resp.StatusCode, string(respBody))
		c.JSON(http.StatusBadGateway, gin.H{"error": "创建支付失败"})
		return
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	out := gin.H{
		"order_no":    order.OrderNo,
		"amount_yuan": float64(order.Amount) / 100,
		"pay_method":  order.PayMethod,
		"status":      "pending",
	}
	if v, ok := result["pay_url"]; ok {
		out["pay_url"] = v
	}
	if v, ok := result["code_url"]; ok {
		out["code_url"] = v
	}

	log.Printf("[marketplace-pay] order created: %s claw=%s template=%s amount=¥%.2f method=%s",
		orderNo, req.ClawID, req.TemplateID, float64(order.Amount)/100, req.PayMethod)
	c.JSON(http.StatusOK, out)
}

// POST /internal/marketplace/payment-confirmed — Called by Synapse when payment succeeds
func (h *MarketplacePaymentHandler) PaymentConfirmed(c *gin.Context) {
	var req struct {
		ExternalOrderNo string `json:"external_order_no" binding:"required"`
		RouterOrderNo   string `json:"router_order_no"`
		TradeNo         string `json:"trade_no"`
		AmountCents     int64  `json:"amount_cents"`
		Channel         string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid callback"})
		return
	}

	log.Printf("[marketplace-pay] payment confirmed: order=%s trade=%s amount=%d channel=%s",
		req.ExternalOrderNo, req.TradeNo, req.AmountCents, req.Channel)

	db := database.DB
	var order model.MarketplacePayOrder
	if err := db.Where("order_no = ? AND status = ?", req.ExternalOrderNo, "pending").First(&order).Error; err != nil {
		log.Printf("[marketplace-pay] order not found or already processed: %s", req.ExternalOrderNo)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	now := time.Now()
	callbackRaw, _ := json.Marshal(req)
	db.Model(&order).Updates(map[string]interface{}{
		"status":       "paid",
		"trade_no":     req.TradeNo,
		"paid_at":      &now,
		"callback_raw": string(callbackRaw),
	})

	log.Printf("[marketplace-pay] order %s marked as paid (template=%s claw=%s)", order.OrderNo, order.TemplateName, order.ClawID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /internal/marketplace/order/:order_no — Called by Claw to poll order status
func (h *MarketplacePaymentHandler) QueryOrder(c *gin.Context) {
	orderNo := c.Param("order_no")

	var order model.MarketplacePayOrder
	if err := database.DB.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// Auto-expire
	if order.Status == "pending" && order.ExpireAt != nil && order.ExpireAt.Before(time.Now()) {
		database.DB.Model(&order).Update("status", "expired")
		order.Status = "expired"
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no":      order.OrderNo,
		"status":        order.Status,
		"template_id":   order.TemplateID,
		"template_name": order.TemplateName,
		"amount_yuan":   float64(order.Amount) / 100,
		"paid_at":       order.PaidAt,
	})
}
