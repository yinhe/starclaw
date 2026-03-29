package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/model"
)

type SettlementHandler struct{}

// GenerateBills auto-generates settlement bills for a given month.
// POST /v1/admin/settlement/generate  { "month": "2026-03" }
func (h *SettlementHandler) GenerateBills(c *gin.Context) {
	var req struct {
		Month string `json:"month" binding:"required"` // YYYY-MM
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate month format
	_, err := time.Parse("2006-01", req.Month)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "month 格式错误，需要 YYYY-MM"})
		return
	}

	// Check if bills already exist for this month
	var existing int64
	database.DB.Model(&model.SettlementBill{}).Where("month = ?", req.Month).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("月份 %s 已有 %d 张账单，请先删除再重新生成", req.Month, existing)})
		return
	}

	var bills []model.SettlementBill

	// 1. Generate bills for core partners
	var corePartners []model.TeamPartner
	database.DB.Where("status = ?", "active").Find(&corePartners)

	for _, cp := range corePartners {
		bill := h.generateTeamPartnerBill(cp, req.Month)
		if bill != nil {
			bills = append(bills, *bill)
		}
	}

	// 2. Generate bills for city partners
	var cityPartners []model.CityPartner
	database.DB.Where("status = ?", "approved").Find(&cityPartners)

	for _, city := range cityPartners {
		bill := h.generateCityPartnerBill(city, req.Month)
		if bill != nil {
			bills = append(bills, *bill)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已生成 %d 张结算账单", len(bills)),
		"bills":   bills,
		"month":   req.Month,
	})
}

func (h *SettlementHandler) generateTeamPartnerBill(cp model.TeamPartner, month string) *model.SettlementBill {
	db := database.DB
	billID := uuid.New().String()
	var items []model.SettlementLineItem
	var totalAmount, salaryAmount, directAmount, manageAmount, equityAmount int64

	// A. Base salary
	if cp.BaseSalary > 0 {
		salaryAmount = cp.BaseSalary
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
			SourceType: "salary", Amount: salaryAmount,
			Description: fmt.Sprintf("月度底薪 (%s)", month),
		})
	}

	// B. Direct commissions from actual consumption (profit-split records)
	var directCommissions []model.PartnerCommission
	db.Where("partner_id = ? AND type = ? AND month = ? AND status = ?",
		cp.ID, "direct", month, "pending",
	).Find(&directCommissions)

	for _, dc := range directCommissions {
		directAmount += dc.Amount
		remark := "直签客户消费分润"
		if dc.Remark != "" {
			remark = dc.Remark
		}
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
			SourceType: "direct", SourceID: dc.DealID, BaseAmount: dc.BaseAmount,
			Rate: dc.Rate, Amount: dc.Amount,
			Description: fmt.Sprintf("%s (%.0f%% × ¥%.2f)", remark, dc.Rate*100, float64(dc.BaseAmount)/100),
		})
	}

	// C. Management fee from city partners under this core partner
	var cityCommissions []model.PartnerCommission
	db.Where("partner_id = ? AND type = ? AND month = ? AND status = ?",
		cp.ID, "manage_fee", month, "pending",
	).Find(&cityCommissions)

	for _, cc := range cityCommissions {
		manageAmount += cc.Amount
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
			SourceType: "manage_fee", SourceID: cc.CityID, BaseAmount: cc.BaseAmount,
			Rate: cc.Rate, Amount: cc.Amount,
			Description: fmt.Sprintf("城市合伙人管理费 (%.0f%%)", cc.Rate*100),
		})
	}

	// D. Equity profit sharing: net_income × equity_ratio
	var equity model.EquityGrant
	if err := db.Where("partner_id = ? AND status = ?", cp.ID, "active").First(&equity).Error; err == nil {
		if equity.TotalShares > 0 {
			netIncome := h.calculateMonthlyNetIncome(month)
			if netIncome > 0 {
				var totalShares int64
				db.Model(&model.EquityGrant{}).Where("status = ?", "active").
					Select("COALESCE(SUM(total_shares), 0)").Scan(&totalShares)

				if totalShares > 0 {
					equityRatio := float64(equity.TotalShares) / float64(totalShares)
					equityAmount = int64(float64(netIncome) * equityRatio)
					if equityAmount > 0 {
						items = append(items, model.SettlementLineItem{
							ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
							SourceType: "equity_share", BaseAmount: netIncome,
							Rate: equityRatio, Amount: equityAmount,
							Description: fmt.Sprintf("股权分润 %d/%d 股 (%.1f%%) × 净利润 ¥%.2f",
								equity.TotalShares, totalShares, equityRatio*100, float64(netIncome)/100),
						})
					}
				}
			}
		}
	}

	totalAmount = salaryAmount + directAmount + manageAmount + equityAmount
	if totalAmount == 0 && len(items) == 0 {
		return nil // skip empty bills
	}

	bill := model.SettlementBill{
		ID: billID, PartnerID: cp.ID, PartnerType: "core", PartnerName: cp.Name,
		Month: month, TotalAmount: totalAmount, SalaryAmount: salaryAmount,
		DirectAmount: directAmount, ManageAmount: manageAmount, EquityAmount: equityAmount,
		ItemCount: len(items), Status: "draft",
	}

	db.Create(&bill)
	for _, item := range items {
		db.Create(&item)
	}

	return &bill
}

func (h *SettlementHandler) generateCityPartnerBill(city model.CityPartner, month string) *model.SettlementBill {
	db := database.DB
	billID := uuid.New().String()
	var items []model.SettlementLineItem
	var cityAmount int64

	// Find commissions for this city partner this month
	var commissions []model.Commission
	database.DB.Where("partner_id = ? AND month = ? AND status = ?",
		city.ID, month, "pending",
	).Find(&commissions)

	for _, comm := range commissions {
		cityAmount += comm.Amount
		label := "新签佣金"
		sourceType := "city_new"
		if comm.Type == "renewal" {
			label = "续费佣金"
			sourceType = "city_renew"
		}
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: city.ID,
			SourceType: sourceType, SourceID: comm.ClientID,
			BaseAmount: comm.BaseAmount, Rate: comm.Rate, Amount: comm.Amount,
			Description: fmt.Sprintf("%s (%.0f%%)", label, comm.Rate*100),
		})
	}

	if cityAmount == 0 && len(items) == 0 {
		return nil
	}

	bill := model.SettlementBill{
		ID: billID, PartnerID: city.ID, PartnerType: "city", PartnerName: city.Name,
		Month: month, TotalAmount: cityAmount, CityAmount: cityAmount,
		ItemCount: len(items), Status: "draft",
	}

	db.Create(&bill)
	for _, item := range items {
		db.Create(&item)
	}

	return &bill
}

// ListBills returns settlement bills with filters
// GET /v1/admin/settlement/bills?month=2026-03&status=draft&partner_type=core
func (h *SettlementHandler) ListBills(c *gin.Context) {
	q := database.DB.Model(&model.SettlementBill{})
	if m := c.Query("month"); m != "" {
		q = q.Where("month = ?", m)
	}
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if pt := c.Query("partner_type"); pt != "" {
		q = q.Where("partner_type = ?", pt)
	}

	var total int64
	q.Count(&total)

	var bills []model.SettlementBill
	q.Order("month DESC, partner_type ASC, partner_name ASC").Find(&bills)

	c.JSON(http.StatusOK, gin.H{"bills": bills, "total": total})
}

// GetBill returns a single bill with line items
// GET /v1/admin/settlement/bills/:id
func (h *SettlementHandler) GetBill(c *gin.Context) {
	id := c.Param("id")
	var bill model.SettlementBill
	if err := database.DB.First(&bill, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	var items []model.SettlementLineItem
	database.DB.Where("bill_id = ?", id).Order("source_type ASC").Find(&items)

	c.JSON(http.StatusOK, gin.H{"bill": bill, "items": items})
}

// ApproveBill approves a settlement bill
// POST /v1/admin/settlement/bills/:id/approve
func (h *SettlementHandler) ApproveBill(c *gin.Context) {
	id := c.Param("id")
	var bill model.SettlementBill
	if err := database.DB.First(&bill, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	if bill.Status != "draft" && bill.Status != "pending_review" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能审批 draft 或 pending_review 状态的账单"})
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&req)

	now := time.Now()
	database.DB.Model(&bill).Updates(map[string]interface{}{
		"status":      "approved",
		"reviewed_by": c.GetString("admin_email"),
		"reviewed_at": &now,
		"review_note": req.Note,
	})

	// Also update related commission records to approved
	if bill.PartnerType == "core" {
		database.DB.Model(&model.PartnerCommission{}).
			Where("partner_id = ? AND month = ? AND status = ?", bill.PartnerID, bill.Month, "pending").
			Update("status", "approved")
	} else {
		database.DB.Model(&model.Commission{}).
			Where("partner_id = ? AND month = ? AND status = ?", bill.PartnerID, bill.Month, "pending").
			Update("status", "approved")
	}

	c.JSON(http.StatusOK, gin.H{"message": "账单已审批", "bill": bill})
}

// RejectBill rejects a settlement bill
// POST /v1/admin/settlement/bills/:id/reject
func (h *SettlementHandler) RejectBill(c *gin.Context) {
	id := c.Param("id")
	var bill model.SettlementBill
	if err := database.DB.First(&bill, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	c.ShouldBindJSON(&req)

	now := time.Now()
	database.DB.Model(&bill).Updates(map[string]interface{}{
		"status":      "rejected",
		"reviewed_by": c.GetString("admin_email"),
		"reviewed_at": &now,
		"review_note": req.Note,
	})

	c.JSON(http.StatusOK, gin.H{"message": "账单已驳回"})
}

// MarkPaid marks a bill as paid
// POST /v1/admin/settlement/bills/:id/pay
func (h *SettlementHandler) MarkPaid(c *gin.Context) {
	id := c.Param("id")
	var bill model.SettlementBill
	if err := database.DB.First(&bill, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	if bill.Status != "approved" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能对已审批的账单标记打款"})
		return
	}

	var req struct {
		PayMethod  string `json:"pay_method"`
		PayAccount string `json:"pay_account"`
		PayRef     string `json:"pay_ref"`
	}
	c.ShouldBindJSON(&req)

	now := time.Now()
	database.DB.Model(&bill).Updates(map[string]interface{}{
		"status":      "paid",
		"paid_at":     &now,
		"pay_method":  req.PayMethod,
		"pay_account": req.PayAccount,
		"pay_ref":     req.PayRef,
	})

	// Update related commissions to paid
	if bill.PartnerType == "core" {
		database.DB.Model(&model.PartnerCommission{}).
			Where("partner_id = ? AND month = ? AND status = ?", bill.PartnerID, bill.Month, "approved").
			Update("status", "paid")
	} else {
		database.DB.Model(&model.Commission{}).
			Where("partner_id = ? AND month = ? AND status = ?", bill.PartnerID, bill.Month, "approved").
			Update("status", "paid")
	}

	c.JSON(http.StatusOK, gin.H{"message": "已标记打款完成"})
}

// DeleteBill deletes a draft bill and its line items
// DELETE /v1/admin/settlement/bills/:id
func (h *SettlementHandler) DeleteBill(c *gin.Context) {
	id := c.Param("id")
	var bill model.SettlementBill
	if err := database.DB.First(&bill, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账单不存在"})
		return
	}
	if bill.Status != "draft" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只能删除 draft 状态的账单"})
		return
	}

	database.DB.Where("bill_id = ?", id).Delete(&model.SettlementLineItem{})
	database.DB.Delete(&bill)

	c.JSON(http.StatusOK, gin.H{"message": "账单已删除"})
}

// SettlementStats returns settlement overview
// GET /v1/admin/settlement/stats
func (h *SettlementHandler) SettlementStats(c *gin.Context) {
	type MonthSummary struct {
		Month       string `json:"month"`
		TotalAmount int64  `json:"total_amount"`
		BillCount   int64  `json:"bill_count"`
		PaidCount   int64  `json:"paid_count"`
	}

	// Last 6 months
	var monthly []MonthSummary
	database.DB.Model(&model.SettlementBill{}).
		Select("month, SUM(total_amount) as total_amount, COUNT(*) as bill_count, SUM(CASE WHEN status='paid' THEN 1 ELSE 0 END) as paid_count").
		Group("month").Order("month DESC").Limit(6).
		Find(&monthly)

	// Status summary
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
		Amount int64  `json:"amount"`
	}
	var byStatus []StatusCount
	database.DB.Model(&model.SettlementBill{}).
		Select("status, COUNT(*) as count, COALESCE(SUM(total_amount),0) as amount").
		Group("status").Find(&byStatus)

	// Total lifetime
	var totalPaid int64
	database.DB.Model(&model.SettlementBill{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(total_amount),0)").Scan(&totalPaid)

	var pendingAmount int64
	database.DB.Model(&model.SettlementBill{}).Where("status IN ?", []string{"draft", "pending_review", "approved"}).
		Select("COALESCE(SUM(total_amount),0)").Scan(&pendingAmount)

	c.JSON(http.StatusOK, gin.H{
		"monthly":        monthly,
		"by_status":      byStatus,
		"total_paid":     totalPaid,
		"pending_amount": pendingAmount,
	})
}

// calculateMonthlyNetIncome computes platform net income for a given month:
// net = total_recharge - upstream_cost_estimate - city_commissions
// upstream_cost_estimate ≈ 30% of recharge (based on typical API cost ratio)
func (h *SettlementHandler) calculateMonthlyNetIncome(month string) int64 {
	db := database.DB

	// Total recharge revenue this month
	var totalRecharge int64
	db.Model(&model.RechargeOrder{}).
		Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", month+"-01", nextMonth(month)+"-01").
		Select("COALESCE(SUM(amount), 0)").Scan(&totalRecharge)

	if totalRecharge == 0 {
		return 0
	}

	// Upstream API cost estimate (fixed 30% ratio for rough estimation)
	upstreamCost := int64(float64(totalRecharge) * 0.30)

	// City partner commissions this month
	var cityCommissions int64
	db.Model(&model.Commission{}).
		Where("month = ? AND status IN ?", month, []string{"pending", "approved", "paid"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&cityCommissions)

	netIncome := totalRecharge - upstreamCost - cityCommissions
	if netIncome < 0 {
		netIncome = 0
	}

	return netIncome
}

func nextMonth(month string) string {
	t, _ := time.Parse("2006-01", month)
	t = t.AddDate(0, 1, 0)
	return t.Format("2006-01")
}

// ============================================================
// Profit Split Configuration — globally adjustable from admin
// ============================================================

// ProfitConfig holds the global profit split ratios (all as float64 percentages, e.g. 0.20 = 20%)
// ProfitConfig defines the 3-party profit split: Partner (dynamic) + OptionPool (fixed) + Platform (remainder).
// Partner commission rate is dynamic (10%~30%/20%) based on option investment — see CalcPartnerCommRate.
// These fields are the system-wide defaults configurable from the admin panel.
type ProfitConfig struct {
	BaseCommRate   float64 `json:"base_comm_rate"`   // base commission rate when partner has no investment (default 10%)
	CityMaxRate    float64 `json:"city_max_rate"`    // city partner max commission rate (default 30%)
	TeamMaxRate    float64 `json:"team_max_rate"`    // team partner max commission rate (default 20%)
	OptionPoolRate float64 `json:"option_pool_rate"` // option pool fixed share of margin (default 20%)
}

var defaultProfitConfig = ProfitConfig{
	BaseCommRate:   0.10,
	CityMaxRate:    0.30,
	TeamMaxRate:    0.20,
	OptionPoolRate: 0.20,
}

// configKeys maps JSON field names to DB keys
var configKeys = map[string]string{
	"base_comm_rate":   "profit_split.base_comm_rate",
	"city_max_rate":    "profit_split.city_max_rate",
	"team_max_rate":    "profit_split.team_max_rate",
	"option_pool_rate": "profit_split.option_pool_rate",
}

// LoadProfitConfig reads the global profit split config from settlement_configs table.
// Falls back to defaults for missing keys.
func LoadProfitConfig() ProfitConfig {
	cfg := defaultProfitConfig
	db := database.DB

	var rows []model.SettlementConfig
	db.Where("key LIKE ?", "profit_split.%").Find(&rows)

	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}

	if v, err := strconv.ParseFloat(m["profit_split.base_comm_rate"], 64); err == nil && v > 0 {
		cfg.BaseCommRate = v
	}
	if v, err := strconv.ParseFloat(m["profit_split.city_max_rate"], 64); err == nil && v > 0 {
		cfg.CityMaxRate = v
	}
	if v, err := strconv.ParseFloat(m["profit_split.team_max_rate"], 64); err == nil && v > 0 {
		cfg.TeamMaxRate = v
	}
	if v, err := strconv.ParseFloat(m["profit_split.option_pool_rate"], 64); err == nil && v >= 0 {
		cfg.OptionPoolRate = v
	}

	return cfg
}

// GET /v1/admin/settlement/profit-config
func (h *SettlementHandler) GetProfitConfig(c *gin.Context) {
	cfg := LoadProfitConfig()
	c.JSON(http.StatusOK, cfg)
}

// PUT /v1/admin/settlement/profit-config
func (h *SettlementHandler) UpdateProfitConfig(c *gin.Context) {
	var req ProfitConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate ranges (0–100%)
	for label, val := range map[string]float64{
		"base_comm_rate":   req.BaseCommRate,
		"city_max_rate":    req.CityMaxRate,
		"team_max_rate":    req.TeamMaxRate,
		"option_pool_rate": req.OptionPoolRate,
	} {
		if val < 0 || val > 1.0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s 必须在 0~1 之间", label)})
			return
		}
	}

	db := database.DB
	save := func(key string, value float64) {
		dbKey := configKeys[key]
		var row model.SettlementConfig
		if err := db.Where("key = ?", dbKey).First(&row).Error; err != nil {
			row = model.SettlementConfig{
				ID:          uuid.New().String(),
				Key:         dbKey,
				Value:       fmt.Sprintf("%.4f", value),
				Description: key,
			}
			db.Create(&row)
		} else {
			db.Model(&row).Updates(map[string]interface{}{
				"value":      fmt.Sprintf("%.4f", value),
				"updated_at": time.Now(),
			})
		}
	}

	save("base_comm_rate", req.BaseCommRate)
	save("city_max_rate", req.CityMaxRate)
	save("team_max_rate", req.TeamMaxRate)
	save("option_pool_rate", req.OptionPoolRate)

	c.JSON(http.StatusOK, gin.H{"message": "利润分配配置已更新", "config": req})
}
