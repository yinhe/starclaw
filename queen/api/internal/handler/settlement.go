package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
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
	var corePartners []model.CorePartner
	database.DB.Where("status = ?", "active").Find(&corePartners)

	for _, cp := range corePartners {
		bill := h.generateCorePartnerBill(cp, req.Month)
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

func (h *SettlementHandler) generateCorePartnerBill(cp model.CorePartner, month string) *model.SettlementBill {
	db := database.DB
	billID := uuid.New().String()
	var items []model.SettlementLineItem
	var totalAmount, salaryAmount, directAmount, manageAmount, equityAmount int64

	tier := model.GetCommissionTier(cp.TotalRevenue)

	// A. Base salary
	if cp.BaseSalary > 0 {
		salaryAmount = cp.BaseSalary
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
			SourceType: "salary", Amount: salaryAmount,
			Description: fmt.Sprintf("月度底薪 (%s)", month),
		})
	}

	// B. Direct commissions from CRM deals active/signed this month
	var deals []model.CRMDeal
	db.Where("partner_id = ? AND stage IN ? AND updated_at >= ? AND updated_at < ?",
		cp.ID, []string{"signed", "delivery", "active", "renewal"},
		month+"-01", nextMonth(month)+"-01",
	).Find(&deals)

	for _, d := range deals {
		if d.DealValue <= 0 {
			continue
		}
		isRenewal := d.Stage == "renewal"
		rate := tier.FirstYearRate
		sourceType := "direct_new"
		if isRenewal {
			rate = tier.RenewalRate
			sourceType = "direct_renew"
		}
		// Override with partner's custom rate if set
		if cp.DirectCommRate > 0 {
			rate = cp.DirectCommRate
		}
		amt := int64(float64(d.DealValue) * rate)
		directAmount += amt
		items = append(items, model.SettlementLineItem{
			ID: uuid.New().String(), BillID: billID, PartnerID: cp.ID,
			SourceType: sourceType, SourceID: d.ID, ClientName: d.CompanyName,
			BaseAmount: d.DealValue, Rate: rate, Amount: amt,
			Description: fmt.Sprintf("%s - %s佣金 (%.0f%%)", d.CompanyName, map[bool]string{true: "续费", false: "首年"}[isRenewal], rate*100),
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
	// net_income = total recharge - upstream API cost (approximated) - city commissions - operating cost
	var equity model.EquityGrant
	if err := db.Where("partner_id = ? AND status = ?", cp.ID, "active").First(&equity).Error; err == nil {
		if equity.TotalShares > 0 {
			// Calculate platform monthly net income
			netIncome := h.calculateMonthlyNetIncome(month)
			if netIncome > 0 {
				// equity_ratio = partner's shares / total platform shares (10,000 base)
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

	// Upstream API cost estimate (30% of recharge)
	upstreamCost := totalRecharge * 30 / 100

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
