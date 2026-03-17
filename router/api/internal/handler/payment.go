package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
	"github.com/yinhe/starclaw-router/internal/config"
	"github.com/yinhe/starclaw-router/internal/model"
	"gorm.io/gorm"
)

type PaymentHandler struct {
	db        *gorm.DB
	aliCfg    config.AlipayConfig
	wxCfg     config.WechatConfig
	aliClient *alipay.Client
	wxClient  *WechatPayClient
}

func NewPaymentHandler(db *gorm.DB, aliCfg config.AlipayConfig, wxCfg config.WechatConfig) *PaymentHandler {
	h := &PaymentHandler{
		db:     db,
		aliCfg: aliCfg,
		wxCfg:  wxCfg,
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
	return h.db.Transaction(func(tx *gorm.DB) error {
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
