package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"starclaw.net/extractor/api/internal/bridge"
	"starclaw.net/extractor/api/internal/model"
)

// BillingHandler handles client management, monthly settlement, payment, and star energy injection.
type BillingHandler struct {
	DB     *gorm.DB
	Bridge *bridge.Client
}

// ── Client CRUD ──

func (h *BillingHandler) ListClients(c *gin.Context) {
	var clients []model.ClientAccount
	q := h.DB.Order("created_at DESC")
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&clients)
	c.JSON(200, gin.H{"clients": clients, "total": len(clients)})
}

func (h *BillingHandler) GetClient(c *gin.Context) {
	var client model.ClientAccount
	if err := h.DB.First(&client, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "client not found"})
		return
	}
	// Include recent bills
	var bills []model.MonthlyBill
	h.DB.Where("client_id = ?", client.ID).Order("month DESC").Limit(12).Find(&bills)
	c.JSON(200, gin.H{"client": client, "bills": bills})
}

func (h *BillingHandler) CreateClient(c *gin.Context) {
	var req struct {
		ClawID         string  `json:"claw_id"`
		UserID         string  `json:"user_id"`
		QMTAccount     string  `json:"qmt_account" binding:"required"`
		Name           string  `json:"name"`
		Phone          string  `json:"phone"`
		CommissionRate float64 `json:"commission_rate"`
		Source         string  `json:"source"`
		TemplateID     string  `json:"template_id"`
		Note           string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check duplicate QMT account
	var existing model.ClientAccount
	if h.DB.Where("qmt_account = ?", req.QMTAccount).First(&existing).Error == nil {
		c.JSON(409, gin.H{"error": "qmt_account already registered", "client_id": existing.ID})
		return
	}

	rate := req.CommissionRate
	if rate <= 0 || rate > 1 {
		rate = 0.20
	}
	source := req.Source
	if source == "" {
		source = "direct"
	}

	// Fetch current NAV from Bridge as initial values
	initialNAV := 0.0
	if info, err := h.Bridge.GetAccountInfo(req.QMTAccount); err == nil {
		initialNAV = info.TotalAssets
	}

	now := time.Now()
	client := model.ClientAccount{
		ClawID:         req.ClawID,
		UserID:         req.UserID,
		QMTAccount:     req.QMTAccount,
		Name:           req.Name,
		Phone:          req.Phone,
		CommissionRate: rate,
		HighWaterMark:  initialNAV,
		InitialNAV:     initialNAV,
		Source:         source,
		TemplateID:     req.TemplateID,
		Status:         "active",
		ActivatedAt:    &now,
		Note:           req.Note,
	}
	if err := h.DB.Create(&client).Error; err != nil {
		c.JSON(500, gin.H{"error": "create failed: " + err.Error()})
		return
	}

	log.Printf("[billing] client created: %s account=%s rate=%.0f%% nav=%.2f source=%s",
		client.ID, client.QMTAccount, rate*100, initialNAV, source)
	c.JSON(201, gin.H{"client": client})
}

func (h *BillingHandler) UpdateClient(c *gin.Context) {
	var client model.ClientAccount
	if err := h.DB.First(&client, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "client not found"})
		return
	}
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	allowed := []string{"name", "phone", "commission_rate", "strategy", "status", "note"}
	updates := map[string]interface{}{}
	for _, k := range allowed {
		if v, ok := req[k]; ok {
			updates[k] = v
		}
	}
	if status, ok := updates["status"].(string); ok && status == "terminated" {
		now := time.Now()
		updates["terminated_at"] = &now
	}
	h.DB.Model(&client).Updates(updates)
	h.DB.First(&client, "id = ?", client.ID)
	c.JSON(200, gin.H{"client": client})
}

// ── NAV Snapshot ──

func (h *BillingHandler) SnapshotNAV(c *gin.Context) {
	// Snapshot all active clients' NAV
	var clients []model.ClientAccount
	h.DB.Where("status = ?", "active").Find(&clients)

	today := time.Now().Format("2006-01-02")
	results := []gin.H{}

	for _, client := range clients {
		info, err := h.Bridge.GetAccountInfo(client.QMTAccount)
		if err != nil {
			results = append(results, gin.H{"account": client.QMTAccount, "error": err.Error()})
			continue
		}
		nav := info.TotalAssets
		avail := info.Available
		mktVal := info.MarketValue

		// Upsert: one snapshot per account per day
		var snap model.NAVSnapshot
		if h.DB.Where("qmt_account = ? AND date = ?", client.QMTAccount, today).First(&snap).Error != nil {
			snap = model.NAVSnapshot{
				ClientID:   client.ID,
				QMTAccount: client.QMTAccount,
				Date:       today,
				TotalNAV:   nav,
				Available:  avail,
				MarketVal:  mktVal,
			}
			h.DB.Create(&snap)
		} else {
			h.DB.Model(&snap).Updates(map[string]interface{}{
				"total_nav": nav, "available": avail, "market_val": mktVal,
			})
		}
		results = append(results, gin.H{"account": client.QMTAccount, "nav": nav, "date": today})
	}

	c.JSON(200, gin.H{"snapshots": results, "date": today})
}

// ── Monthly Settlement ──

func (h *BillingHandler) GenerateBills(c *gin.Context) {
	month := c.DefaultQuery("month", time.Now().AddDate(0, -1, 0).Format("2006-01"))

	var clients []model.ClientAccount
	h.DB.Where("status = ?", "active").Find(&clients)

	// Parse month boundaries
	monthStart, _ := time.Parse("2006-01", month)
	monthEnd := monthStart.AddDate(0, 1, 0)
	startDate := monthStart.Format("2006-01-02")
	endDate := monthEnd.AddDate(0, 0, -1).Format("2006-01-02")

	bills := []model.MonthlyBill{}
	for _, client := range clients {
		// Check if bill already exists
		var existing model.MonthlyBill
		if h.DB.Where("client_id = ? AND month = ?", client.ID, month).First(&existing).Error == nil {
			bills = append(bills, existing)
			continue
		}

		// Get start/end NAV from snapshots
		var startSnap, endSnap model.NAVSnapshot
		h.DB.Where("client_id = ? AND date >= ?", client.ID, startDate).Order("date ASC").First(&startSnap)
		h.DB.Where("client_id = ? AND date <= ?", client.ID, endDate).Order("date DESC").First(&endSnap)

		startNAV := startSnap.TotalNAV
		endNAV := endSnap.TotalNAV
		if startNAV == 0 {
			startNAV = client.InitialNAV
		}
		if endNAV == 0 {
			// Try live query
			if info, err := h.Bridge.GetAccountInfo(client.QMTAccount); err == nil {
				endNAV = info.TotalAssets
			}
		}

		grossProfit := endNAV - startNAV
		hwm := client.HighWaterMark
		if hwm < startNAV {
			hwm = startNAV
		}

		// High Water Mark: only bill on profit above HWM
		billableProfit := 0.0
		if endNAV > hwm {
			billableProfit = endNAV - hwm
		}

		serviceFee := billableProfit * client.CommissionRate
		serviceFeeCents := int64(serviceFee * 100)

		bill := model.MonthlyBill{
			ClientID:        client.ID,
			ClientName:      client.Name,
			QMTAccount:      client.QMTAccount,
			Month:           month,
			StartNAV:        startNAV,
			EndNAV:          endNAV,
			GrossProfit:     grossProfit,
			HighWaterMark:   hwm,
			BillableProfit:  billableProfit,
			CommissionRate:  client.CommissionRate,
			ServiceFee:      serviceFee,
			ServiceFeeCents: serviceFeeCents,
			Status:          "draft",
		}
		h.DB.Create(&bill)

		log.Printf("[billing] bill generated: client=%s month=%s profit=%.2f billable=%.2f fee=%.2f",
			client.QMTAccount, month, grossProfit, billableProfit, serviceFee)
		bills = append(bills, bill)
	}

	c.JSON(200, gin.H{"month": month, "bills": bills, "total": len(bills)})
}

func (h *BillingHandler) ListBills(c *gin.Context) {
	var bills []model.MonthlyBill
	q := h.DB.Order("month DESC, created_at DESC")
	if month := c.Query("month"); month != "" {
		q = q.Where("month = ?", month)
	}
	if clientID := c.Query("client_id"); clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Limit(200).Find(&bills)
	c.JSON(200, gin.H{"bills": bills, "total": len(bills)})
}

func (h *BillingHandler) GetBill(c *gin.Context) {
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}
	c.JSON(200, gin.H{"bill": bill})
}

// ── Payment: Online (Synapse Alipay/WeChat) ──

func (h *BillingHandler) CreatePayment(c *gin.Context) {
	billID := c.Param("id")
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", billID).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}
	if bill.ServiceFeeCents <= 0 {
		c.JSON(400, gin.H{"error": "no_fee", "message": "本月无需付费（未产生可计费利润）"})
		return
	}
	if bill.Status == "paid" || bill.Status == "star_injected" {
		c.JSON(409, gin.H{"error": "already_paid"})
		return
	}

	var req struct {
		PayMethod string `json:"pay_method"` // alipay / wechat
	}
	c.ShouldBindJSON(&req)
	if req.PayMethod == "" {
		req.PayMethod = "alipay"
	}

	// Call Queen → Synapse to create payment order
	queenURL := getEnvOrDefault("QUEEN_URL", "https://api.starclaw.net")
	queenToken := getEnvOrDefault("QUEEN_TOKEN", "")

	orderDesc := fmt.Sprintf("Q8bot %s 交易服务费", bill.Month)
	payload, _ := json.Marshal(map[string]interface{}{
		"claw_id":       "extractor",
		"user_id":       bill.ClientID,
		"template_id":   "q8bot-service-fee",
		"template_name": orderDesc,
		"amount":        bill.ServiceFeeCents,
		"pay_method":    req.PayMethod,
		"pay_form":      "pc",
	})

	httpReq, _ := http.NewRequest("POST", queenURL+"/internal/marketplace/create-payment", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Token", queenToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[billing] payment creation failed: %v", err)
		c.JSON(502, gin.H{"error": "payment_service_unavailable", "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		log.Printf("[billing] payment creation error: HTTP %d body=%s", resp.StatusCode, string(body))
		c.JSON(502, gin.H{"error": "payment_failed", "detail": string(body)})
		return
	}

	var result struct {
		OrderNo    string  `json:"order_no"`
		PayURL     string  `json:"pay_url"`
		PayMethod  string  `json:"pay_method"`
		AmountYuan float64 `json:"amount_yuan"`
	}
	json.Unmarshal(body, &result)

	// Update bill with payment info
	h.DB.Model(&bill).Updates(map[string]interface{}{
		"payment_method":   req.PayMethod,
		"payment_order_no": result.OrderNo,
		"pay_url":          result.PayURL,
		"status":           "pending_payment",
	})

	log.Printf("[billing] payment order created: bill=%s order=%s method=%s amount=¥%.2f",
		bill.ID, result.OrderNo, result.PayMethod, result.AmountYuan)

	c.JSON(200, gin.H{
		"order_no":   result.OrderNo,
		"pay_url":    result.PayURL,
		"pay_method": result.PayMethod,
		"amount":     result.AmountYuan,
		"bill_id":    bill.ID,
	})
}

// PollPayment checks Synapse payment status and auto-confirms.
func (h *BillingHandler) PollPayment(c *gin.Context) {
	billID := c.Param("id")
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", billID).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}
	if bill.Status == "paid" || bill.Status == "star_injected" {
		c.JSON(200, gin.H{"status": bill.Status, "bill": bill})
		return
	}
	if bill.PaymentOrderNo == "" {
		c.JSON(400, gin.H{"error": "no payment order"})
		return
	}

	// Query Queen for order status
	queenURL := getEnvOrDefault("QUEEN_URL", "https://api.starclaw.net")
	queenToken := getEnvOrDefault("QUEEN_TOKEN", "")

	httpReq, _ := http.NewRequest("GET", queenURL+"/internal/marketplace/order/"+bill.PaymentOrderNo, nil)
	httpReq.Header.Set("X-Node-Token", queenToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(502, gin.H{"error": "query_failed"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		c.JSON(200, gin.H{"status": "pending_payment", "bill": bill})
		return
	}

	var orderStatus struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&orderStatus)

	if orderStatus.Status == "paid" {
		now := time.Now()
		h.DB.Model(&bill).Updates(map[string]interface{}{
			"status":  "paid",
			"paid_at": &now,
		})
		bill.Status = "paid"
		bill.PaidAt = &now

		// Auto-inject star energy
		go h.injectStarEnergy(&bill)
	}

	c.JSON(200, gin.H{"status": bill.Status, "bill": bill})
}

// ── Payment: Offline (manual transfer confirmation) ──

func (h *BillingHandler) ConfirmOfflinePayment(c *gin.Context) {
	billID := c.Param("id")
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", billID).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}

	var req struct {
		PaymentRef  string `json:"payment_ref" binding:"required"`
		ConfirmedBy string `json:"confirmed_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	h.DB.Model(&bill).Updates(map[string]interface{}{
		"payment_method": "offline",
		"payment_ref":    req.PaymentRef,
		"confirmed_by":   req.ConfirmedBy,
		"status":         "paid",
		"paid_at":        &now,
	})
	bill.Status = "paid"

	log.Printf("[billing] offline payment confirmed: bill=%s ref=%s by=%s", bill.ID, req.PaymentRef, req.ConfirmedBy)

	// Auto-inject star energy
	go h.injectStarEnergy(&bill)

	h.DB.First(&bill, "id = ?", bill.ID)
	c.JSON(200, gin.H{"bill": bill})
}

// ── Star Energy Injection (Queen) ──

func (h *BillingHandler) injectStarEnergy(bill *model.MonthlyBill) {
	if bill.ServiceFeeCents <= 0 {
		return
	}

	queenURL := getEnvOrDefault("QUEEN_URL", "https://api.starclaw.net")
	queenToken := getEnvOrDefault("QUEEN_TOKEN", "")
	if queenToken == "" {
		log.Printf("[billing] star energy injection skipped (test mode): QUEEN_TOKEN not configured, bill=%s fee=¥%.2f", bill.ID, bill.ServiceFee)
		// Stay at "paid" — will be injected later via POST /v1/billing/inject-retry
		return
	}

	// ¥1 = 100 star energy units = 1,000,000 internal units
	starEnergy := bill.ServiceFeeCents * 10000 // cents → internal units (1分 = 10000 units)

	payload, _ := json.Marshal(map[string]interface{}{
		"source":        "extractor",
		"source_id":     bill.ID,
		"client_id":     bill.ClientID,
		"amount":        starEnergy,
		"amount_cny":    bill.ServiceFee,
		"month":         bill.Month,
		"description":   fmt.Sprintf("Q8bot %s trading service fee from %s", bill.Month, bill.QMTAccount),
		"settlement_id": bill.ID,
	})

	httpReq, _ := http.NewRequest("POST", queenURL+"/internal/credits/grant", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Node-Token", queenToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Printf("[billing] star energy injection failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var result struct {
			TxID string `json:"tx_id"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		h.DB.Model(bill).Updates(map[string]interface{}{
			"star_energy_units": starEnergy,
			"queen_tx_id":       result.TxID,
			"status":            "star_injected",
		})

		// Update client high water mark
		if bill.EndNAV > bill.HighWaterMark {
			h.DB.Model(&model.ClientAccount{}).Where("id = ?", bill.ClientID).
				Update("high_water_mark", bill.EndNAV)
		}

		log.Printf("[billing] ⚡ star energy injected: bill=%s energy=%d tx=%s", bill.ID, starEnergy, result.TxID)
	} else {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[billing] star energy injection error: HTTP %d body=%s", resp.StatusCode, string(body))
	}
}

// ── Star Energy Retry (for testing phase / Queen offline) ──

// RetryInjectOne retries star energy injection for a single paid bill.
func (h *BillingHandler) RetryInjectOne(c *gin.Context) {
	billID := c.Param("id")
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", billID).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}
	if bill.Status == "star_injected" {
		c.JSON(200, gin.H{"status": "already_injected", "bill": bill})
		return
	}
	if bill.Status != "paid" {
		c.JSON(400, gin.H{"error": "bill not paid yet", "status": bill.Status})
		return
	}

	h.injectStarEnergy(&bill)
	h.DB.First(&bill, "id = ?", bill.ID)
	c.JSON(200, gin.H{"bill": bill})
}

// RetryInjectAll retries star energy injection for ALL paid but not-yet-injected bills.
func (h *BillingHandler) RetryInjectAll(c *gin.Context) {
	var bills []model.MonthlyBill
	h.DB.Where("status = ? AND service_fee_cents > 0", "paid").Find(&bills)

	if len(bills) == 0 {
		c.JSON(200, gin.H{"message": "no pending bills to inject", "count": 0})
		return
	}

	success, failed := 0, 0
	for i := range bills {
		h.injectStarEnergy(&bills[i])
		// Re-read to check if injection succeeded
		var updated model.MonthlyBill
		h.DB.First(&updated, "id = ?", bills[i].ID)
		if updated.Status == "star_injected" {
			success++
		} else {
			failed++
		}
	}

	log.Printf("[billing] inject retry: %d success, %d failed out of %d", success, failed, len(bills))
	c.JSON(200, gin.H{"total": len(bills), "success": success, "failed": failed})
}

// ── Send Bill (notification to client) ──

func (h *BillingHandler) SendBill(c *gin.Context) {
	billID := c.Param("id")
	var bill model.MonthlyBill
	if err := h.DB.First(&bill, "id = ?", billID).Error; err != nil {
		c.JSON(404, gin.H{"error": "bill not found"})
		return
	}
	if bill.Status != "draft" {
		c.JSON(400, gin.H{"error": "bill already sent", "status": bill.Status})
		return
	}

	now := time.Now()
	h.DB.Model(&bill).Updates(map[string]interface{}{
		"status":  "sent",
		"sent_at": &now,
	})

	// TODO: send notification to client via Claw / WeChat / SMS

	log.Printf("[billing] bill sent: %s client=%s fee=¥%.2f", bill.ID, bill.ClientName, bill.ServiceFee)
	c.JSON(200, gin.H{"bill": bill, "message": "账单已发送"})
}

// ── Dashboard Stats ──

func (h *BillingHandler) BillingStats(c *gin.Context) {
	var clientCount int64
	h.DB.Model(&model.ClientAccount{}).Where("status = ?", "active").Count(&clientCount)

	var totalFee, totalPaid float64
	h.DB.Model(&model.MonthlyBill{}).Select("COALESCE(SUM(service_fee), 0)").Scan(&totalFee)
	h.DB.Model(&model.MonthlyBill{}).Where("status IN ?", []string{"paid", "star_injected"}).
		Select("COALESCE(SUM(service_fee), 0)").Scan(&totalPaid)

	var pendingCount int64
	h.DB.Model(&model.MonthlyBill{}).Where("status IN ?", []string{"sent", "pending_payment"}).Count(&pendingCount)

	var totalEnergy int64
	h.DB.Model(&model.MonthlyBill{}).Where("status = ?", "star_injected").
		Select("COALESCE(SUM(star_energy_units), 0)").Scan(&totalEnergy)

	c.JSON(200, gin.H{
		"active_clients":    clientCount,
		"total_billed":      totalFee,
		"total_collected":   totalPaid,
		"pending_bills":     pendingCount,
		"star_energy_total": totalEnergy,
		"collection_rate":   safeDiv(totalPaid, totalFee),
	})
}

// ── Marketplace Webhook: Q8bot purchase → auto-create ClientAccount ──

func (h *BillingHandler) OnMarketplacePurchase(c *gin.Context) {
	var req struct {
		ClawID     string `json:"claw_id"`
		UserID     string `json:"user_id"`
		TemplateID string `json:"template_id"`
		QMTAccount string `json:"qmt_account"`
		Name       string `json:"name"`
		Phone      string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check if already registered
	var existing model.ClientAccount
	if req.QMTAccount != "" {
		if h.DB.Where("qmt_account = ?", req.QMTAccount).First(&existing).Error == nil {
			c.JSON(200, gin.H{"client": existing, "message": "already_registered"})
			return
		}
	}

	now := time.Now()
	client := model.ClientAccount{
		ClawID:         req.ClawID,
		UserID:         req.UserID,
		QMTAccount:     req.QMTAccount,
		Name:           req.Name,
		Phone:          req.Phone,
		CommissionRate: 0.20,
		Source:         "marketplace",
		TemplateID:     req.TemplateID,
		Status:         "active",
		ActivatedAt:    &now,
	}
	h.DB.Create(&client)

	log.Printf("[billing] marketplace client created: %s claw=%s account=%s", client.ID, req.ClawID, req.QMTAccount)
	c.JSON(201, gin.H{"client": client})
}

// ── Helpers ──

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
