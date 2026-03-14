package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-pay/gopay"
	"github.com/go-pay/gopay/alipay"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/config"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BillingHandler struct {
	alipayClient *alipay.Client
	wechatClient *wechat.ClientV3
}

// NewBillingHandler initializes Alipay + WeChat Pay clients
func NewBillingHandler() *BillingHandler {
	h := &BillingHandler{}
	h.initAlipay()
	h.initWechatPay()
	SeedDefaultPackages()
	return h
}

func (h *BillingHandler) initAlipay() {
	cfg := config.C.Pay.Alipay
	if cfg.AppID == "" {
		log.Println("[billing] Alipay not configured, skipping")
		return
	}

	privateKey, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		log.Printf("[billing] Failed to read Alipay private key: %v", err)
		return
	}

	client, err := alipay.NewClient(cfg.AppID, string(privateKey), cfg.IsProduction)
	if err != nil {
		log.Printf("[billing] Failed to create Alipay client: %v", err)
		return
	}

	client.SetReturnUrl(cfg.ReturnURL)
	client.SetNotifyUrl(cfg.NotifyURL)

	// Load certs
	if err := client.SetCertSnByPath(cfg.AppCertPath, cfg.AlipayCertPath, cfg.RootCertPath); err != nil {
		log.Printf("[billing] Failed to load Alipay certs: %v", err)
		return
	}

	h.alipayClient = client
	log.Printf("[billing] Alipay client initialized (appid=%s, production=%v)", cfg.AppID, cfg.IsProduction)
}

func (h *BillingHandler) initWechatPay() {
	cfg := config.C.Pay.WechatPay
	if cfg.MchID == "" {
		log.Println("[billing] WeChat Pay not configured, skipping")
		return
	}

	// Read merchant private key
	privKeyBytes, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		log.Printf("[billing] Failed to read WeChat Pay private key: %v", err)
		return
	}

	client, err := wechat.NewClientV3(cfg.MchID, cfg.MerchantSerialNumber, cfg.APIv3Key, string(privKeyBytes))
	if err != nil {
		log.Printf("[billing] Failed to create WeChat Pay client: %v", err)
		return
	}

	// Use platform public key for signature verification
	if cfg.PublicKeyPath != "" {
		pubKeyBytes, err := os.ReadFile(cfg.PublicKeyPath)
		if err != nil {
			log.Printf("[billing] Failed to read WeChat Pay public key: %v", err)
		} else {
			if err := client.AutoVerifySignByPublicKey(pubKeyBytes, cfg.PublicKeyID); err != nil {
				log.Printf("[billing] Failed to set WeChat Pay auto verify: %v", err)
			}
		}
	}

	h.wechatClient = client
	log.Printf("[billing] WeChat Pay client initialized (mchid=%s)", cfg.MchID)
}

// ---------- Recharge Packages ----------

// GET /pay/packages — list available recharge packages
func (h *BillingHandler) ListPackages(c *gin.Context) {
	var packages []model.RechargePackage
	database.DB.Where("enabled = ?", true).Order("sort_order ASC, amount ASC").Find(&packages)
	c.JSON(http.StatusOK, gin.H{"packages": packages})
}

// ---------- Balance ----------

// GET /pay/balance — get current user balance
func (h *BillingHandler) GetBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	bal := ensureBalance(userID)
	c.JSON(http.StatusOK, gin.H{
		"balance":   bal.Balance,
		"frozen":    bal.Frozen,
		"total_in":  bal.TotalIn,
		"total_out": bal.TotalOut,
	})
}

// GET /pay/transactions — list balance transactions
func (h *BillingHandler) ListTransactions(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size > 100 {
		size = 100
	}

	var total int64
	database.DB.Model(&model.BalanceTransaction{}).Where("user_id = ?", userID).Count(&total)

	var txns []model.BalanceTransaction
	database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&txns)

	c.JSON(http.StatusOK, gin.H{
		"transactions": txns,
		"total":        total,
		"page":         page,
		"size":         size,
	})
}

// GET /pay/orders — list user's orders
func (h *BillingHandler) ListOrders(c *gin.Context) {
	userID := c.GetString("user_id")
	var orders []model.RechargeOrder
	database.DB.Where("user_id = ?", userID).Order("created_at DESC").Limit(50).Find(&orders)
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// ---------- Create Order ----------

// POST /pay/create — create a recharge order
func (h *BillingHandler) CreateOrder(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		PackageID string `json:"package_id" binding:"required"`
		PayMethod string `json:"pay_method" binding:"required"` // alipay / wechatpay
		PayForm   string `json:"pay_form"`                      // pc / h5 / native
		ClawID    string `json:"claw_id"`                       // optional: target claw for Star Energy
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.PayForm == "" {
		req.PayForm = "h5"
	}

	// Find package
	var pkg model.RechargePackage
	if err := database.DB.First(&pkg, "id = ? AND enabled = ?", req.PackageID, true).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "套餐不存在"})
		return
	}

	// Auto-resolve claw_id from user's bound nodes if not provided
	clawID := req.ClawID
	if clawID == "" {
		var binding model.NodeBinding
		if err := database.DB.Where("queen_user_id = ? AND status = ?", userID, "active").
			Order("last_seen DESC").First(&binding).Error; err == nil {
			clawID = binding.NodeID
		}
	}

	// Create order
	orderNo := fmt.Sprintf("SC%s%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()/1000000)
	expire := time.Now().Add(30 * time.Minute)
	order := model.RechargeOrder{
		ID:          uuid.New().String(),
		OrderNo:     orderNo,
		UserID:      userID,
		ClawID:      clawID,
		Amount:      pkg.Amount,
		BonusAmount: pkg.BonusAmount,
		PayMethod:   req.PayMethod,
		PayForm:     req.PayForm,
		Status:      "pending",
		Subject:     fmt.Sprintf("StarClaw 充值 - %s", pkg.Name),
		ExpireAt:    &expire,
	}
	if err := database.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建订单失败"})
		return
	}

	// Call payment gateway
	switch req.PayMethod {
	case "alipay":
		h.createAlipayOrder(c, &order)
	case "wechatpay":
		h.createWechatOrder(c, &order)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的支付方式"})
	}
}

// ---------- Alipay ----------

func (h *BillingHandler) createAlipayOrder(c *gin.Context, order *model.RechargeOrder) {
	if h.alipayClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "支付宝尚未配置"})
		return
	}

	amountYuan := fmt.Sprintf("%.2f", float64(order.Amount)/100.0)

	switch order.PayForm {
	case "pc":
		// Desktop page pay
		bm := gopay.BodyMap{}
		bm.Set("subject", order.Subject)
		bm.Set("out_trade_no", order.OrderNo)
		bm.Set("total_amount", amountYuan)
		bm.Set("product_code", "FAST_INSTANT_TRADE_PAY")

		payURL, err := h.alipayClient.TradePagePay(context.Background(), bm)
		if err != nil {
			log.Printf("[billing] Alipay page pay error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":   order.OrderNo,
			"pay_method": "alipay",
			"pay_form":   "pc",
			"pay_url":    payURL,
		})

	default:
		// H5 / WAP pay
		bm := gopay.BodyMap{}
		bm.Set("subject", order.Subject)
		bm.Set("out_trade_no", order.OrderNo)
		bm.Set("total_amount", amountYuan)
		bm.Set("product_code", "QUICK_WAP_WAY")
		bm.Set("quit_url", config.C.Pay.Alipay.ReturnURL)

		payURL, err := h.alipayClient.TradeWapPay(context.Background(), bm)
		if err != nil {
			log.Printf("[billing] Alipay wap pay error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":   order.OrderNo,
			"pay_method": "alipay",
			"pay_form":   "h5",
			"pay_url":    payURL,
		})
	}
}

// POST /pay/webhook/alipay — Alipay async notification
func (h *BillingHandler) AlipayWebhook(c *gin.Context) {
	if h.alipayClient == nil {
		c.String(http.StatusOK, "fail")
		return
	}

	notifyReq, err := alipay.ParseNotifyToBodyMap(c.Request)
	if err != nil {
		log.Printf("[billing] Alipay parse notify error: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	// Verify signature
	ok, err := alipay.VerifySignWithCert(config.C.Pay.Alipay.AlipayCertPath, notifyReq)
	if err != nil || !ok {
		log.Printf("[billing] Alipay verify sign failed: %v", err)
		c.String(http.StatusOK, "fail")
		return
	}

	orderNo := notifyReq.Get("out_trade_no")
	tradeNo := notifyReq.Get("trade_no")
	tradeStatus := notifyReq.Get("trade_status")

	log.Printf("[billing] Alipay notify: order=%s, trade=%s, status=%s", orderNo, tradeNo, tradeStatus)

	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		h.completeOrder(orderNo, tradeNo, notifyReq.JsonBody())
	}

	c.String(http.StatusOK, "success")
}

// ---------- WeChat Pay ----------

func (h *BillingHandler) createWechatOrder(c *gin.Context, order *model.RechargeOrder) {
	if h.wechatClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "微信支付尚未配置"})
		return
	}

	cfg := config.C.Pay.WechatPay
	expire := time.Now().Add(30 * time.Minute).Format(time.RFC3339)

	switch order.PayForm {
	case "native":
		// Native scan-to-pay (PC)
		bm := gopay.BodyMap{}
		bm.Set("appid", cfg.AppID)
		bm.Set("mchid", cfg.MchID)
		bm.Set("description", order.Subject)
		bm.Set("out_trade_no", order.OrderNo)
		bm.Set("time_expire", expire)
		bm.Set("notify_url", cfg.NotifyURL)
		bm.SetBodyMap("amount", func(am gopay.BodyMap) {
			am.Set("total", order.Amount)
			am.Set("currency", "CNY")
		})

		resp, err := h.wechatClient.V3TransactionNative(context.Background(), bm)
		if err != nil {
			log.Printf("[billing] WeChat native pay error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":   order.OrderNo,
			"pay_method": "wechatpay",
			"pay_form":   "native",
			"code_url":   resp.Response.CodeUrl,
		})

	default:
		// H5 pay
		bm := gopay.BodyMap{}
		bm.Set("appid", cfg.AppID)
		bm.Set("mchid", cfg.MchID)
		bm.Set("description", order.Subject)
		bm.Set("out_trade_no", order.OrderNo)
		bm.Set("time_expire", expire)
		bm.Set("notify_url", cfg.NotifyURL)
		bm.SetBodyMap("amount", func(am gopay.BodyMap) {
			am.Set("total", order.Amount)
			am.Set("currency", "CNY")
		})
		bm.SetBodyMap("scene_info", func(si gopay.BodyMap) {
			si.Set("payer_client_ip", c.ClientIP())
			si.SetBodyMap("h5_info", func(h5 gopay.BodyMap) {
				h5.Set("type", "Wap")
				h5.Set("wap_url", cfg.H5WapURL)
				h5.Set("wap_name", cfg.H5WapName)
			})
		})

		resp, err := h.wechatClient.V3TransactionH5(context.Background(), bm)
		if err != nil {
			log.Printf("[billing] WeChat H5 pay error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建支付失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"order_no":   order.OrderNo,
			"pay_method": "wechatpay",
			"pay_form":   "h5",
			"h5_url":     resp.Response.H5Url,
		})
	}
}

// POST /pay/webhook/wechatpay — WeChat Pay V3 notification
func (h *BillingHandler) WechatPayWebhook(c *gin.Context) {
	if h.wechatClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "not configured"})
		return
	}

	notifyReq, err := wechat.V3ParseNotify(c.Request)
	if err != nil {
		log.Printf("[billing] WeChat parse notify error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "parse error"})
		return
	}

	// Decrypt pay notification with APIv3 key
	result, err := notifyReq.DecryptPayCipherText(config.C.Pay.WechatPay.APIv3Key)
	if err != nil {
		log.Printf("[billing] WeChat decrypt notify error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "decrypt error"})
		return
	}

	log.Printf("[billing] WeChat notify: order=%s, trade=%s, state=%s",
		result.OutTradeNo, result.TransactionId, result.TradeState)

	if result.TradeState == "SUCCESS" {
		h.completeOrder(result.OutTradeNo, result.TransactionId, notifyReq.Resource.Ciphertext)
	}

	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}

// ---------- Order Query ----------

// GET /pay/order/:order_no/status — query order payment status
func (h *BillingHandler) QueryOrderStatus(c *gin.Context) {
	orderNo := c.Param("order_no")
	userID := c.GetString("user_id")

	var order model.RechargeOrder
	if err := database.DB.First(&order, "order_no = ? AND user_id = ?", orderNo, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}

	// If still pending, query payment gateway for latest status
	if order.Status == "pending" {
		switch order.PayMethod {
		case "alipay":
			h.queryAlipayOrder(&order)
		case "wechatpay":
			h.queryWechatOrder(&order)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no": order.OrderNo,
		"status":   order.Status,
		"amount":   order.Amount,
		"paid_at":  order.PaidAt,
	})
}

func (h *BillingHandler) queryAlipayOrder(order *model.RechargeOrder) {
	if h.alipayClient == nil {
		return
	}
	bm := gopay.BodyMap{}
	bm.Set("out_trade_no", order.OrderNo)

	resp, err := h.alipayClient.TradeQuery(context.Background(), bm)
	if err != nil {
		log.Printf("[billing] Alipay query error: %v", err)
		return
	}
	if resp.Response.TradeStatus == "TRADE_SUCCESS" || resp.Response.TradeStatus == "TRADE_FINISHED" {
		h.completeOrder(order.OrderNo, resp.Response.TradeNo, "")
		order.Status = "paid"
	}
}

func (h *BillingHandler) queryWechatOrder(order *model.RechargeOrder) {
	if h.wechatClient == nil {
		return
	}

	resp, err := h.wechatClient.V3TransactionQueryOrder(context.Background(), wechat.OutTradeNo, order.OrderNo)
	if err != nil {
		log.Printf("[billing] WeChat query error: %v", err)
		return
	}
	if resp.Response.TradeState == "SUCCESS" {
		h.completeOrder(order.OrderNo, resp.Response.TransactionId, "")
		order.Status = "paid"
	}
}

// ---------- Complete Order (core billing logic) ----------

func (h *BillingHandler) completeOrder(orderNo, tradeNo, callbackRaw string) {
	db := database.DB

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock order
		var order model.RechargeOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return fmt.Errorf("order not found: %s", orderNo)
		}

		if order.Status == "paid" {
			return nil // idempotent
		}

		now := time.Now()
		order.Status = "paid"
		order.TradeNo = tradeNo
		order.PaidAt = &now
		if callbackRaw != "" {
			order.CallbackRaw = callbackRaw
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		// Ensure user balance
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", order.UserID).First(&bal).Error; err != nil {
			bal = model.UserBalance{
				ID:     uuid.New().String(),
				UserID: order.UserID,
			}
			tx.Create(&bal)
		}

		totalCredit := order.Amount + order.BonusAmount
		before := bal.Balance

		bal.Balance += totalCredit
		bal.TotalIn += totalCredit
		if err := tx.Save(&bal).Error; err != nil {
			return err
		}

		// Record recharge transaction
		tx.Create(&model.BalanceTransaction{
			ID:      uuid.New().String(),
			UserID:  order.UserID,
			OrderNo: order.OrderNo,
			Type:    "recharge",
			Amount:  order.Amount,
			Before:  before,
			After:   before + order.Amount,
			Remark:  fmt.Sprintf("充值 ¥%.2f", float64(order.Amount)/100.0),
		})

		// Record bonus transaction (if any)
		if order.BonusAmount > 0 {
			tx.Create(&model.BalanceTransaction{
				ID:      uuid.New().String(),
				UserID:  order.UserID,
				OrderNo: order.OrderNo,
				Type:    "bonus",
				Amount:  order.BonusAmount,
				Before:  before + order.Amount,
				After:   bal.Balance,
				Remark:  fmt.Sprintf("充值赠送 ¥%.2f", float64(order.BonusAmount)/100.0),
			})
		}

		// ── Grant Star Energy to bound claw_id ──
		if order.ClawID != "" {
			stars := cnyToEnergy(order.Amount + order.BonusAmount)
			grantStarEnergy(tx, order.ClawID, stars, fmt.Sprintf("recharge order %s", order.OrderNo))
			order.StarsGranted = stars
			tx.Model(&order).Update("stars_granted", stars)
			log.Printf("[billing] Granted %d star units to %s (order %s)", stars, order.ClawID, orderNo)
		}

		log.Printf("[billing] Order %s completed: amount=%d, bonus=%d, user=%s, claw=%s, balance=%d",
			orderNo, order.Amount, order.BonusAmount, order.UserID, order.ClawID, bal.Balance)

		return nil
	})

	if err != nil {
		log.Printf("[billing] completeOrder error: %v", err)
	}
}

// ---------- Available Pay Methods ----------

// GET /pay/methods — list which payment methods are configured
func (h *BillingHandler) PayMethods(c *gin.Context) {
	methods := []gin.H{}
	if h.alipayClient != nil {
		methods = append(methods, gin.H{"id": "alipay", "name": "支付宝", "forms": []string{"pc", "h5"}})
	}
	if h.wechatClient != nil {
		methods = append(methods, gin.H{"id": "wechatpay", "name": "微信支付", "forms": []string{"native", "h5"}})
	}
	c.JSON(http.StatusOK, gin.H{"methods": methods})
}

// ---------- Star Energy Integration ----------

const (
	// Early promo rate: ¥0.01 = 1 Star = 10000 internal units
	// So ¥1 (100分) = 100 Stars = 1,000,000 units
	// Therefore: 1分 = 1 Star = 10000 units
	CnyFenToEnergyUnits = 10000
)

// cnyToEnergy converts CNY amount (分) to Star Energy internal units
func cnyToEnergy(amountFen int64) int64 {
	return amountFen * CnyFenToEnergyUnits
}

// grantStarEnergy adds Star Energy to a claw account within an existing DB transaction
func grantStarEnergy(tx *gorm.DB, clawID string, amount int64, remark string) {
	if clawID == "" || amount <= 0 {
		return
	}

	var acct model.CreditAccount
	if err := tx.Where("claw_id = ?", clawID).First(&acct).Error; err != nil {
		acct = model.CreditAccount{
			ID:     uuid.New().String(),
			ClawID: clawID,
			Status: "active",
		}
		tx.Create(&acct)
	}

	tx.Model(&acct).Updates(map[string]interface{}{
		"balance":  gorm.Expr("balance + ?", amount),
		"total_in": gorm.Expr("total_in + ?", amount),
	})

	tx.Create(&model.CreditTransaction{
		ID:       uuid.New().String(),
		FromClaw: "system",
		ToClaw:   clawID,
		Amount:   amount,
		Type:     "recharge",
		Remark:   remark,
		Status:   "confirmed",
	})
}

// ---------- Convert Balance to Star Energy ----------

// POST /pay/convert-energy — convert ¥ balance to Star Energy for a bound claw
func (h *BillingHandler) ConvertToEnergy(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		Amount int64  `json:"amount" binding:"required"` // 金额，单位：分
		ClawID string `json:"claw_id"`                   // 目标 claw 地址，空则自动选择
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整，需要 amount（分）"})
		return
	}
	if req.Amount < 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "最低兑换 ¥1.00（100 分）"})
		return
	}

	// Auto-resolve claw_id from user's bound nodes if not provided
	clawID := req.ClawID
	if clawID == "" {
		var binding model.NodeBinding
		if err := database.DB.Where("queen_user_id = ? AND status = ?", userID, "active").
			Order("last_seen DESC").First(&binding).Error; err == nil {
			clawID = binding.NodeID
		}
	}
	if clawID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未绑定 Claw 节点，请先在设置中绑定"})
		return
	}

	db := database.DB
	stars := cnyToEnergy(req.Amount)
	var newBalance int64
	var newEnergy float64

	err := db.Transaction(func(tx *gorm.DB) error {
		// Lock user balance
		var bal model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", userID).First(&bal).Error; err != nil {
			return fmt.Errorf("用户余额账户不存在")
		}
		if bal.Balance < req.Amount {
			return fmt.Errorf("余额不足（可用 ¥%.2f，需要 ¥%.2f）",
				float64(bal.Balance)/100, float64(req.Amount)/100)
		}

		// Deduct ¥ balance
		before := bal.Balance
		tx.Model(&bal).Updates(map[string]interface{}{
			"balance":   gorm.Expr("balance - ?", req.Amount),
			"total_out": gorm.Expr("total_out + ?", req.Amount),
		})
		newBalance = before - req.Amount

		// Record balance transaction
		tx.Create(&model.BalanceTransaction{
			ID:     uuid.New().String(),
			UserID: userID,
			Type:   "consume",
			Amount: -req.Amount,
			Before: before,
			After:  newBalance,
			Remark: fmt.Sprintf("兑换星能 %.1f⚡ → %s", float64(stars)/float64(CnyFenToEnergyUnits), clawID),
		})

		// Grant star energy
		grantStarEnergy(tx, clawID, stars, fmt.Sprintf("balance convert by user %s", userID))
		newEnergy = float64(stars) / float64(CnyFenToEnergyUnits)

		return nil
	})

	if err != nil {
		log.Printf("[billing] convert-energy failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[billing] User %s converted ¥%.2f → %.1f⚡ for %s",
		userID, float64(req.Amount)/100, newEnergy, clawID)

	c.JSON(http.StatusOK, gin.H{
		"message":        "兑换成功",
		"amount_cny":     float64(req.Amount) / 100,
		"stars_granted":  stars,
		"energy_granted": newEnergy,
		"claw_id":        clawID,
		"new_balance":    float64(newBalance) / 100,
	})
}

// ---------- Helpers ----------

func ensureBalance(userID string) *model.UserBalance {
	var bal model.UserBalance
	if err := database.DB.Where("user_id = ?", userID).First(&bal).Error; err != nil {
		bal = model.UserBalance{
			ID:     uuid.New().String(),
			UserID: userID,
		}
		database.DB.Create(&bal)
	}
	return &bal
}

// SeedDefaultPackages creates default recharge packages if none exist
func SeedDefaultPackages() {
	var count int64
	database.DB.Model(&model.RechargePackage{}).Count(&count)
	if count > 0 {
		return
	}

	packages := []model.RechargePackage{
		{ID: uuid.New().String(), Name: "体验包", Amount: 1000, BonusAmount: 0, BonusRate: 0, SortOrder: 1, Enabled: true},
		{ID: uuid.New().String(), Name: "基础包", Amount: 5000, BonusAmount: 500, BonusRate: 0.10, SortOrder: 2, Enabled: true},
		{ID: uuid.New().String(), Name: "标准包", Amount: 10000, BonusAmount: 2000, BonusRate: 0.20, SortOrder: 3, Enabled: true},
		{ID: uuid.New().String(), Name: "高级包", Amount: 50000, BonusAmount: 15000, BonusRate: 0.30, SortOrder: 4, Enabled: true},
		{ID: uuid.New().String(), Name: "专业包", Amount: 100000, BonusAmount: 40000, BonusRate: 0.40, SortOrder: 5, Enabled: true},
	}

	for _, p := range packages {
		database.DB.Create(&p)
	}
	log.Println("[billing] Seeded default recharge packages")
}
