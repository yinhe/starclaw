package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
	"starclaw.net/synapse/api/internal/config"
	"starclaw.net/synapse/api/internal/model"
	"gorm.io/gorm"
)

type PaymentHandler struct {
	db        *gorm.DB
	aliCfg    config.AlipayConfig
	wxCfg     config.WechatConfig
	queenCfg  config.QueenConfig
	aliClient *alipay.Client
	wxClient  *WechatPayClient
}

func NewPaymentHandler(db *gorm.DB, aliCfg config.AlipayConfig, wxCfg config.WechatConfig, queenCfg config.QueenConfig) *PaymentHandler {
	h := &PaymentHandler{
		db:       db,
		aliCfg:   aliCfg,
		wxCfg:    wxCfg,
		queenCfg: queenCfg,
	}

	// Initialize Alipay client if configured
	if aliCfg.AppID != "" {
		// Read private key from file
		keyBytes, err := os.ReadFile(aliCfg.PrivateKeyPath)
		if err != nil {
			log.Printf("[star-ai] Alipay private key file not found: %v", err)
		} else {
			privateKey := string(keyBytes)
			client, err := alipay.New(aliCfg.AppID, privateKey, aliCfg.IsProduction)
			if err != nil {
				log.Printf("[star-ai] Alipay client init error: %v", err)
			} else {
				// Load certificates
				if aliCfg.AppCertPath != "" {
					if err := client.LoadAppCertPublicKeyFromFile(aliCfg.AppCertPath); err != nil {
						log.Printf("[star-ai] Alipay load app cert: %v", err)
					}
					if err := client.LoadAliPayRootCertFromFile(aliCfg.RootCertPath); err != nil {
						log.Printf("[star-ai] Alipay load root cert: %v", err)
					}
					if err := client.LoadAlipayCertPublicKeyFromFile(aliCfg.AliCertPath); err != nil {
						log.Printf("[star-ai] Alipay load ali cert: %v", err)
					}
				}
				h.aliClient = client
				log.Printf("[star-ai] Alipay client initialized (appID=%s, production=%v)", aliCfg.AppID, aliCfg.IsProduction)
			}
		}
	}

	// Initialize WeChat Pay client if configured
	if wxCfg.MchID != "" && wxCfg.PrivateKeyPath != "" {
		h.wxClient = NewWechatPayClient(wxCfg)
	}

	return h
}

// Packages returns available recharge packages
func (h *PaymentHandler) Packages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"packages": model.DefaultPackages(),
	})
}

// CreateAlipay creates an Alipay payment order
func (h *PaymentHandler) CreateAlipay(c *gin.Context) {
	if h.aliClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Alipay not configured"})
		return
	}

	userID := c.GetString("user_id")

	var req struct {
		PackageID string `json:"package_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package_id required"})
		return
	}

	pkg := findPackage(req.PackageID)
	if pkg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package"})
		return
	}

	orderNo := generateOrderNo("AL")

	// Create DB order
	order := model.PaymentOrder{
		UserID:      userID,
		OrderNo:     orderNo,
		Channel:     "alipay",
		AmountCents: pkg.AmountCents,
		BonusCents:  pkg.BonusCents,
		TotalCents:  pkg.TotalCents,
		Status:      "pending",
	}
	if err := h.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	// Create Alipay trade
	amountYuan := fmt.Sprintf("%.2f", float64(pkg.AmountCents)/100.0)
	trade := alipay.TradePagePay{
		Trade: alipay.Trade{
			NotifyURL:   h.aliCfg.NotifyURL,
			ReturnURL:   h.aliCfg.ReturnURL,
			Subject:     fmt.Sprintf("Star-AI 充值 %s", pkg.Name),
			OutTradeNo:  orderNo,
			TotalAmount: amountYuan,
			ProductCode: "FAST_INSTANT_TRADE_PAY",
		},
	}

	url, err := h.aliClient.TradePagePay(trade)
	if err != nil {
		log.Printf("[star-ai] Alipay create trade error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no": orderNo,
		"pay_url":  url.String(),
		"channel":  "alipay",
		"amount":   amountYuan,
	})
}

// CreateWechat creates a WeChat Pay Native order (QR code)
func (h *PaymentHandler) CreateWechat(c *gin.Context) {
	if h.wxClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WeChat Pay not configured"})
		return
	}

	userID := c.GetString("user_id")

	var req struct {
		PackageID string `json:"package_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package_id required"})
		return
	}

	pkg := findPackage(req.PackageID)
	if pkg == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package"})
		return
	}

	orderNo := generateOrderNo("WX")

	order := model.PaymentOrder{
		UserID:      userID,
		OrderNo:     orderNo,
		Channel:     "wechat",
		AmountCents: pkg.AmountCents,
		BonusCents:  pkg.BonusCents,
		TotalCents:  pkg.TotalCents,
		Status:      "pending",
	}
	if err := h.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	// Create WeChat Pay Native order (V3 API)
	amountYuan := fmt.Sprintf("%.2f", float64(pkg.AmountCents)/100.0)
	description := fmt.Sprintf("Star-AI 充值 %s", pkg.Name)

	codeURL, err := h.wxClient.CreateNativeOrder(description, orderNo, int(pkg.AmountCents))
	if err != nil {
		log.Printf("[star-ai] WeChat Pay create order error: %v", err)
		// Mark order as failed
		h.db.Model(&order).Update("status", "failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create WeChat payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no": orderNo,
		"channel":  "wechat",
		"amount":   amountYuan,
		"code_url": codeURL,
	})
}

// CallbackAlipay handles Alipay async notification (no auth — called by Alipay servers)
func (h *PaymentHandler) CallbackAlipay(c *gin.Context) {
	if h.aliClient == nil {
		c.String(http.StatusOK, "fail")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		log.Printf("[star-ai] Alipay callback parse form error: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	ctx := context.Background()
	noti, err := h.aliClient.DecodeNotification(ctx, c.Request.Form)
	if err != nil {
		log.Printf("[star-ai] Alipay callback verify error: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	orderNo := noti.OutTradeNo
	tradeStatus := noti.TradeStatus

	log.Printf("[star-ai] Alipay callback: order=%s status=%s trade_no=%s", orderNo, tradeStatus, noti.TradeNo)

	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		c.String(http.StatusOK, "success")
		return
	}

	// Credit balance
	if err := h.creditOrder(orderNo, noti.TradeNo); err != nil {
		log.Printf("[star-ai] Alipay credit error: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// CallbackWechat handles WeChat Pay V3 async notification (no auth — called by WeChat servers)
func (h *PaymentHandler) CallbackWechat(c *gin.Context) {
	if h.wxClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "WeChat Pay not configured"})
		return
	}

	// Read callback headers and body
	timestamp := c.GetHeader("Wechatpay-Timestamp")
	nonce := c.GetHeader("Wechatpay-Nonce")
	signature := c.GetHeader("Wechatpay-Signature")

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[star-ai] WeChat callback read body error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "read body failed"})
		return
	}

	log.Printf("[star-ai] WeChat callback received: ts=%s nonce=%s body_len=%d", timestamp, nonce, len(bodyBytes))

	// Verify signature and decrypt
	result, err := h.wxClient.VerifyAndDecryptCallback(timestamp, nonce, string(bodyBytes), signature)
	if err != nil {
		log.Printf("[star-ai] WeChat callback verify/decrypt error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "verification failed"})
		return
	}

	log.Printf("[star-ai] WeChat callback: order=%s state=%s transaction=%s",
		result.OutTradeNo, result.TradeState, result.TransactionID)

	// Only process successful payments
	if result.TradeState != "SUCCESS" {
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
		return
	}

	// Credit balance
	if err := h.creditOrder(result.OutTradeNo, result.TransactionID); err != nil {
		log.Printf("[star-ai] WeChat credit error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "credit failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}

// QueryOrder actively queries Alipay/WeChat for trade status and credits if paid
func (h *PaymentHandler) QueryOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	orderNo := c.Query("order_no")
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_no required"})
		return
	}

	// Find the order
	var order model.PaymentOrder
	if err := h.db.Where("order_no = ? AND user_id = ?", orderNo, userID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	// Already paid — just return
	if order.Status == "paid" {
		c.JSON(http.StatusOK, gin.H{"status": "paid", "order_no": orderNo})
		return
	}

	if order.Status != "pending" {
		c.JSON(http.StatusOK, gin.H{"status": order.Status, "order_no": orderNo})
		return
	}

	// Active query based on channel
	if order.Channel == "alipay" && h.aliClient != nil {
		trade := alipay.TradeQuery{
			OutTradeNo: orderNo,
		}
		result, err := h.aliClient.TradeQuery(context.Background(), trade)
		if err != nil {
			log.Printf("[star-ai] Alipay query error for %s: %v", orderNo, err)
			c.JSON(http.StatusOK, gin.H{"status": "pending", "order_no": orderNo, "message": "查询中"})
			return
		}

		log.Printf("[star-ai] Alipay query: order=%s trade_status=%s trade_no=%s",
			orderNo, result.TradeStatus, result.TradeNo)

		if result.TradeStatus == "TRADE_SUCCESS" || result.TradeStatus == "TRADE_FINISHED" {
			if err := h.creditOrder(orderNo, result.TradeNo); err != nil {
				log.Printf("[star-ai] Alipay query credit error: %v", err)
			}
			c.JSON(http.StatusOK, gin.H{"status": "paid", "order_no": orderNo})
			return
		}
	}

	// WeChat active query
	if order.Channel == "wechat" && h.wxClient != nil {
		result, err := h.wxClient.QueryOrder(orderNo)
		if err != nil {
			log.Printf("[star-ai] WeChat query error for %s: %v", orderNo, err)
			c.JSON(http.StatusOK, gin.H{"status": "pending", "order_no": orderNo, "message": "查询中"})
			return
		}

		log.Printf("[star-ai] WeChat query: order=%s trade_state=%s transaction=%s",
			orderNo, result.TradeState, result.TransactionID)

		if result.TradeState == "SUCCESS" {
			if err := h.creditOrder(orderNo, result.TransactionID); err != nil {
				log.Printf("[star-ai] WeChat query credit error: %v", err)
			}
			c.JSON(http.StatusOK, gin.H{"status": "paid", "order_no": orderNo})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "pending", "order_no": orderNo})
}

// SyncPendingOrders queries payment providers for all pending orders of the user
func (h *PaymentHandler) SyncPendingOrders(c *gin.Context) {
	userID := c.GetString("user_id")

	var orders []model.PaymentOrder
	h.db.Where("user_id = ? AND status = ?", userID, "pending").Find(&orders)

	synced := 0
	for _, order := range orders {
		if order.Channel == "alipay" && h.aliClient != nil {
			trade := alipay.TradeQuery{
				OutTradeNo: order.OrderNo,
			}
			result, err := h.aliClient.TradeQuery(context.Background(), trade)
			if err != nil {
				log.Printf("[star-ai] Alipay sync query error for %s: %v", order.OrderNo, err)
				continue
			}
			if result.TradeStatus == "TRADE_SUCCESS" || result.TradeStatus == "TRADE_FINISHED" {
				if err := h.creditOrder(order.OrderNo, result.TradeNo); err == nil {
					synced++
				}
			}
		}
		if order.Channel == "wechat" && h.wxClient != nil {
			result, err := h.wxClient.QueryOrder(order.OrderNo)
			if err != nil {
				log.Printf("[star-ai] WeChat sync query error for %s: %v", order.OrderNo, err)
				continue
			}
			if result.TradeState == "SUCCESS" {
				if err := h.creditOrder(order.OrderNo, result.TransactionID); err == nil {
					synced++
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"synced":  synced,
		"checked": len(orders),
	})
}

// Orders returns the user's payment orders
func (h *PaymentHandler) Orders(c *gin.Context) {
	userID := c.GetString("user_id")

	var orders []model.PaymentOrder
	h.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(100).
		Find(&orders)

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
	})
}

// creditOrder credits the user's balance after successful payment
func (h *PaymentHandler) creditOrder(orderNo, tradeNo string) error {
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var order model.PaymentOrder
		if err := tx.Where("order_no = ? AND status = ?", orderNo, "pending").First(&order).Error; err != nil {
			return fmt.Errorf("order not found or already processed: %w", err)
		}

		now := time.Now()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status":   "paid",
			"trade_no": tradeNo,
			"paid_at":  &now,
		}).Error; err != nil {
			return err
		}

		// Credit user balance
		if err := tx.Model(&model.User{}).
			Where("id = ?", order.UserID).
			Update("balance", gorm.Expr("balance + ?", order.TotalCents)).Error; err != nil {
			return err
		}

		log.Printf("[star-ai] credited %d cents to user %s (order %s)", order.TotalCents, order.UserID, orderNo)
		return nil
	})

	if err == nil {
		// Check if this is an invest order → call Queen callback
		var completed model.PaymentOrder
		h.db.Where("order_no = ?", orderNo).First(&completed)
		if completed.Purpose == "invest" && completed.CallbackURL != "" {
			go h.notifyInvestCallback(completed)
		} else {
			// Sync star energy to Queen credit ledger for Claw
			go h.syncStarEnergy(orderNo)
		}
	}

	return err
}

// notifyInvestCallback tells Queen that a diamond purchase payment succeeded.
func (h *PaymentHandler) notifyInvestCallback(order model.PaymentOrder) {
	body, _ := json.Marshal(map[string]interface{}{
		"external_order_no": order.ExternalOrderNo,
		"router_order_no":   order.OrderNo,
		"trade_no":          order.TradeNo,
		"amount_cents":      order.AmountCents,
		"channel":           order.Channel,
	})

	req, _ := http.NewRequest("POST", order.CallbackURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if h.queenCfg.Token != "" {
		req.Header.Set("X-Node-Token", h.queenCfg.Token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[star-ai] invest callback failed for %s → %s: %v", order.OrderNo, order.CallbackURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[star-ai] invest callback OK: router=%s queen=%s amount=¥%.2f",
			order.OrderNo, order.ExternalOrderNo, float64(order.AmountCents)/100)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[star-ai] invest callback returned %d for %s: %s", resp.StatusCode, order.ExternalOrderNo, string(respBody))
	}
}

// syncStarEnergy grants star energy to the user's Claw node via Queen's internal API.
// Conversion: 1 分 (cent) = 1 Star = 10000 internal units
func (h *PaymentHandler) syncStarEnergy(orderNo string) {
	queenURL := strings.TrimRight(h.queenCfg.URL, "/")
	if queenURL == "" {
		return
	}

	var order model.PaymentOrder
	if err := h.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		log.Printf("[star-ai] sync energy: order %s not found: %v", orderNo, err)
		return
	}

	// Look up user's Claw ID
	var user model.User
	if err := h.db.Where("id = ?", order.UserID).First(&user).Error; err != nil || user.ClawID == "" {
		log.Printf("[star-ai] sync energy: user %s has no claw_id, skipping", order.UserID)
		return
	}

	// 1 cent = 1 Star = 10000 energy units
	energyUnits := order.TotalCents * 10000

	body, _ := json.Marshal(map[string]interface{}{
		"claw_id": user.ClawID,
		"amount":  energyUnits,
		"type":    "recharge",
		"remark":  fmt.Sprintf("StarAI recharge order %s (¥%.2f)", orderNo, float64(order.TotalCents)/100),
	})

	req, _ := http.NewRequest("POST", queenURL+"/internal/credits/grant", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if h.queenCfg.Token != "" {
		req.Header.Set("X-Node-Token", h.queenCfg.Token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[star-ai] sync energy to Queen failed for %s: %v", user.ClawID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Printf("[star-ai] synced %d energy units (%.1f⚡) to claw %s (order %s)",
			energyUnits, float64(energyUnits)/10000, user.ClawID, orderNo)
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[star-ai] sync energy to Queen returned %d: %s", resp.StatusCode, string(respBody))
	}
}

// CreateInvestOrder creates an Alipay/WeChat payment order for Queen's star diamond investment.
// Called by Queen's internal API. Returns pay_url for the user to complete payment.
// POST /internal/payment/invest-order
func (h *PaymentHandler) CreateInvestOrder(c *gin.Context) {
	var req struct {
		Channel         string `json:"channel" binding:"required"`           // alipay / wechat
		AmountCents     int64  `json:"amount_cents" binding:"required"`      // CNY in 分
		Subject         string `json:"subject"`                              // order description
		ExternalOrderNo string `json:"external_order_no" binding:"required"` // Queen's diamond order_no
		CallbackURL     string `json:"callback_url" binding:"required"`      // Queen's callback URL
		PayForm         string `json:"pay_form"`                             // pc / h5 / native
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields: " + err.Error()})
		return
	}
	if req.PayForm == "" {
		req.PayForm = "pc"
	}
	if req.Subject == "" {
		req.Subject = fmt.Sprintf("星钻购买 ¥%.2f", float64(req.AmountCents)/100)
	}

	orderNo := generateOrderNo("IV") // IV = invest

	order := model.PaymentOrder{
		UserID:          "queen-invest", // placeholder; not a Router user
		OrderNo:         orderNo,
		Channel:         req.Channel,
		AmountCents:     req.AmountCents,
		BonusCents:      0,
		TotalCents:      req.AmountCents,
		Status:          "pending",
		Purpose:         "invest",
		ExternalOrderNo: req.ExternalOrderNo,
		CallbackURL:     req.CallbackURL,
	}
	if err := h.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}

	amountYuan := fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0)

	switch req.Channel {
	case "alipay":
		if h.aliClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Alipay not configured"})
			return
		}
		trade := alipay.TradePagePay{
			Trade: alipay.Trade{
				NotifyURL:   h.aliCfg.NotifyURL,
				ReturnURL:   h.aliCfg.ReturnURL,
				Subject:     req.Subject,
				OutTradeNo:  orderNo,
				TotalAmount: amountYuan,
				ProductCode: "FAST_INSTANT_TRADE_PAY",
			},
		}
		url, err := h.aliClient.TradePagePay(trade)
		if err != nil {
			log.Printf("[star-ai] invest Alipay error: %v", err)
			h.db.Model(&order).Update("status", "failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Alipay order failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":          orderNo,
			"external_order_no": req.ExternalOrderNo,
			"channel":           "alipay",
			"pay_url":           url.String(),
			"amount_yuan":       amountYuan,
		})

	case "wechat", "wechatpay":
		if h.wxClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WeChat Pay not configured"})
			return
		}
		codeURL, err := h.wxClient.CreateNativeOrder(req.Subject, orderNo, int(req.AmountCents))
		if err != nil {
			log.Printf("[star-ai] invest WeChat error: %v", err)
			h.db.Model(&order).Update("status", "failed")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "WeChat order failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":          orderNo,
			"external_order_no": req.ExternalOrderNo,
			"channel":           "wechat",
			"code_url":          codeURL,
			"amount_yuan":       amountYuan,
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel, use alipay or wechatpay"})
	}
}

// --- helpers ---

func findPackage(id string) *model.RechargePackage {
	for _, p := range model.DefaultPackages() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

func generateOrderNo(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s%s%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(b))
}
