package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type BillingHandler struct {
	db *gorm.DB
}

func NewBillingHandler(db *gorm.DB) *BillingHandler {
	return &BillingHandler{db: db}
}

// SeedDefaultPlans creates default subscription plans if none exist
func (h *BillingHandler) SeedDefaultPlans() {
	var count int64
	h.db.Model(&model.Plan{}).Count(&count)
	if count > 0 {
		return
	}
	plans := []model.Plan{
		{Name: "community", DisplayName: "Community", PriceMonthly: 0, PriceYearly: 0, MaxNodes: 10, MaxTeams: 1, MaxTokensDay: 0, Features: `{"usage_stats":"basic","audit_log":false,"sso":false,"budget_alert":false}`, SortOrder: 0, Active: true},
		{Name: "starter", DisplayName: "Starter", PriceMonthly: 49900, PriceYearly: 39900, MaxNodes: 20, MaxTeams: 3, MaxTokensDay: 0, Features: `{"usage_stats":"standard","audit_log":false,"sso":false,"budget_alert":true}`, SortOrder: 1, Active: true},
		{Name: "pro", DisplayName: "Pro", PriceMonthly: 199900, PriceYearly: 159900, MaxNodes: 100, MaxTeams: 0, MaxTokensDay: 0, Features: `{"usage_stats":"advanced","audit_log":true,"sso":true,"budget_alert":true,"model_routing":true}`, SortOrder: 2, Active: true},
		{Name: "enterprise", DisplayName: "Enterprise", PriceMonthly: 499900, PriceYearly: 399900, MaxNodes: 500, MaxTeams: 0, MaxTokensDay: 0, Features: `{"usage_stats":"advanced","audit_log":true,"sso":true,"budget_alert":true,"model_routing":true,"compliance_dashboard":true}`, SortOrder: 3, Active: true},
		{Name: "whitelabel", DisplayName: "White-Label", PriceMonthly: 999900, PriceYearly: 0, MaxNodes: 0, MaxTeams: 0, MaxTokensDay: 0, Features: `{"usage_stats":"advanced","audit_log":true,"sso":true,"budget_alert":true,"model_routing":true,"compliance_dashboard":true,"branding":true,"custom_domain":true,"feature_toggles":true}`, SortOrder: 4, Active: true},
	}
	for i := range plans {
		h.db.Create(&plans[i])
	}
}

// ==================== Plans ====================

// ListPlans GET /brood/billing/plans
func (h *BillingHandler) ListPlans(c *gin.Context) {
	var plans []model.Plan
	q := h.db.Order("sort_order ASC")
	if c.Query("active_only") == "true" {
		q = q.Where("active = ?", true)
	}
	q.Find(&plans)
	c.JSON(http.StatusOK, plans)
}

// GetPlan GET /brood/billing/plans/:id
func (h *BillingHandler) GetPlan(c *gin.Context) {
	var plan model.Plan
	if err := h.db.First(&plan, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}
	c.JSON(http.StatusOK, plan)
}

// CreatePlan POST /brood/billing/plans
func (h *BillingHandler) CreatePlan(c *gin.Context) {
	var plan model.Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Create(&plan).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	audit(h.db, c, "plan.create", plan.ID, fmt.Sprintf("plan=%s price=%d", plan.Name, plan.PriceMonthly))
	c.JSON(http.StatusCreated, plan)
}

// UpdatePlan PUT /brood/billing/plans/:id
func (h *BillingHandler) UpdatePlan(c *gin.Context) {
	var plan model.Plan
	if err := h.db.First(&plan, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plan not found"})
		return
	}
	var req model.Plan
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&plan).Updates(map[string]interface{}{
		"display_name":   req.DisplayName,
		"price_monthly":  req.PriceMonthly,
		"price_yearly":   req.PriceYearly,
		"max_nodes":      req.MaxNodes,
		"max_teams":      req.MaxTeams,
		"max_tokens_day": req.MaxTokensDay,
		"features":       req.Features,
		"sort_order":     req.SortOrder,
		"active":         req.Active,
	})
	h.db.First(&plan, "id = ?", c.Param("id"))
	audit(h.db, c, "plan.update", plan.ID, plan.Name)
	c.JSON(http.StatusOK, plan)
}

// ==================== Subscriptions ====================

// ListSubscriptions GET /brood/billing/subscriptions
func (h *BillingHandler) ListSubscriptions(c *gin.Context) {
	var subs []model.Subscription
	q := h.db.Order("created_at DESC")
	if team := c.Query("team_id"); team != "" {
		q = q.Where("team_id = ?", team)
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}
	q.Find(&subs)
	c.JSON(http.StatusOK, subs)
}

// GetSubscription GET /brood/billing/subscriptions/:id
func (h *BillingHandler) GetSubscription(c *gin.Context) {
	var sub model.Subscription
	if err := h.db.First(&sub, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	c.JSON(http.StatusOK, sub)
}

// CreateSubscription POST /brood/billing/subscriptions
func (h *BillingHandler) CreateSubscription(c *gin.Context) {
	var req struct {
		TeamID       string `json:"team_id"`
		PlanID       string `json:"plan_id" binding:"required"`
		BillingCycle string `json:"billing_cycle"` // monthly or yearly
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate plan
	var plan model.Plan
	if err := h.db.First(&plan, "id = ?", req.PlanID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	cycle := req.BillingCycle
	if cycle == "" {
		cycle = "monthly"
	}

	now := time.Now()
	var periodEnd time.Time
	if cycle == "yearly" {
		periodEnd = now.AddDate(1, 0, 0)
	} else {
		periodEnd = now.AddDate(0, 1, 0)
	}

	// Cancel any existing active subscription for this team
	h.db.Model(&model.Subscription{}).
		Where("team_id = ? AND status = ?", req.TeamID, "active").
		Updates(map[string]interface{}{"status": "cancelled", "cancelled_at": now})

	sub := model.Subscription{
		TeamID:             req.TeamID,
		PlanID:             plan.ID,
		PlanName:           plan.Name,
		Status:             "active",
		BillingCycle:       cycle,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   periodEnd,
	}
	h.db.Create(&sub)
	audit(h.db, c, "subscription.create", sub.ID, fmt.Sprintf("plan=%s team=%s cycle=%s", plan.Name, req.TeamID, cycle))
	c.JSON(http.StatusCreated, sub)
}

// CancelSubscription POST /brood/billing/subscriptions/:id/cancel
func (h *BillingHandler) CancelSubscription(c *gin.Context) {
	var sub model.Subscription
	if err := h.db.First(&sub, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subscription not found"})
		return
	}
	now := time.Now()
	h.db.Model(&sub).Updates(map[string]interface{}{"status": "cancelled", "cancelled_at": &now})
	audit(h.db, c, "subscription.cancel", sub.ID, sub.PlanName)
	h.db.First(&sub, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, sub)
}

// ==================== Usage Recording ====================

// RecordUsage POST /brood/billing/usage
func (h *BillingHandler) RecordUsage(c *gin.Context) {
	var req model.UsageRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TotalTokens == 0 {
		req.TotalTokens = req.InputTokens + req.OutputTokens
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	req.CreatedAt = time.Now()
	h.db.Create(&req)

	// Update daily summary
	h.updateDailySummary(req.TeamID, req.Date)

	// Check budget alerts
	go h.checkBudgetAlerts(req.TeamID, req.Date)

	c.JSON(http.StatusCreated, req)
}

// updateDailySummary recalculates the daily summary for a team+date
func (h *BillingHandler) updateDailySummary(teamID, date string) {
	var summary model.UsageDailySummary
	result := h.db.Where("team_id = ? AND date = ?", teamID, date).First(&summary)

	var stats struct {
		TotalRequests   int64
		TotalTokens     int64
		InputTokens     int64
		OutputTokens    int64
		TotalCostCents  int
		TotalStarEnergy int
		AvgLatencyMs    int
		UniqueUsers     int
		UniqueModels    int
	}

	h.db.Model(&model.UsageRecord{}).
		Where("team_id = ? AND date = ?", teamID, date).
		Select(`
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(cost_cents), 0) as total_cost_cents,
			COALESCE(SUM(star_energy), 0) as total_star_energy,
			COALESCE(AVG(duration_ms), 0) as avg_latency_ms,
			COUNT(DISTINCT user_id) as unique_users,
			COUNT(DISTINCT model_name) as unique_models
		`).Scan(&stats)

	if result.Error != nil {
		// Create new
		summary = model.UsageDailySummary{
			TeamID:          teamID,
			Date:            date,
			TotalRequests:   stats.TotalRequests,
			TotalTokens:     stats.TotalTokens,
			InputTokens:     stats.InputTokens,
			OutputTokens:    stats.OutputTokens,
			TotalCostCents:  stats.TotalCostCents,
			TotalStarEnergy: stats.TotalStarEnergy,
			UniqueUsers:     stats.UniqueUsers,
			UniqueModels:    stats.UniqueModels,
			AvgLatencyMs:    stats.AvgLatencyMs,
		}
		h.db.Create(&summary)
	} else {
		h.db.Model(&summary).Updates(map[string]interface{}{
			"total_requests":    stats.TotalRequests,
			"total_tokens":      stats.TotalTokens,
			"input_tokens":      stats.InputTokens,
			"output_tokens":     stats.OutputTokens,
			"total_cost_cents":  stats.TotalCostCents,
			"total_star_energy": stats.TotalStarEnergy,
			"unique_users":      stats.UniqueUsers,
			"unique_models":     stats.UniqueModels,
			"avg_latency_ms":    stats.AvgLatencyMs,
		})
	}
}

// ==================== Usage Statistics ====================

// UsageStats GET /brood/billing/usage/stats
func (h *BillingHandler) UsageStats(c *gin.Context) {
	teamID := c.Query("team_id")
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))

	var summaries []model.UsageDailySummary
	q := h.db.Where("date >= ? AND date <= ?", from, to).Order("date ASC")
	if teamID != "" {
		q = q.Where("team_id = ?", teamID)
	}
	q.Find(&summaries)

	// Compute totals
	var totals struct {
		TotalRequests   int64 `json:"total_requests"`
		TotalTokens     int64 `json:"total_tokens"`
		InputTokens     int64 `json:"input_tokens"`
		OutputTokens    int64 `json:"output_tokens"`
		TotalCostCents  int   `json:"total_cost_cents"`
		TotalStarEnergy int   `json:"total_star_energy"`
	}
	for _, s := range summaries {
		totals.TotalRequests += s.TotalRequests
		totals.TotalTokens += s.TotalTokens
		totals.InputTokens += s.InputTokens
		totals.OutputTokens += s.OutputTokens
		totals.TotalCostCents += s.TotalCostCents
		totals.TotalStarEnergy += s.TotalStarEnergy
	}

	c.JSON(http.StatusOK, gin.H{
		"from":    from,
		"to":      to,
		"team_id": teamID,
		"totals":  totals,
		"daily":   summaries,
	})
}

// UsageByModel GET /brood/billing/usage/by-model
func (h *BillingHandler) UsageByModel(c *gin.Context) {
	teamID := c.Query("team_id")
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))

	type ModelUsage struct {
		ModelName     string `json:"model_name"`
		TotalRequests int64  `json:"total_requests"`
		TotalTokens   int64  `json:"total_tokens"`
		TotalCost     int    `json:"total_cost_cents"`
		AvgLatencyMs  int    `json:"avg_latency_ms"`
	}

	var results []ModelUsage
	q := h.db.Model(&model.UsageRecord{}).
		Where("date >= ? AND date <= ?", from, to).
		Select(`
			model_name,
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost_cents), 0) as total_cost,
			COALESCE(AVG(duration_ms), 0) as avg_latency_ms
		`).
		Group("model_name").
		Order("total_tokens DESC")

	if teamID != "" {
		q = q.Where("team_id = ?", teamID)
	}
	q.Find(&results)

	c.JSON(http.StatusOK, gin.H{
		"from":   from,
		"to":     to,
		"models": results,
	})
}

// UsageByUser GET /brood/billing/usage/by-user
func (h *BillingHandler) UsageByUser(c *gin.Context) {
	teamID := c.Query("team_id")
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	type UserUsage struct {
		UserID        string `json:"user_id"`
		TotalRequests int64  `json:"total_requests"`
		TotalTokens   int64  `json:"total_tokens"`
		TotalCost     int    `json:"total_cost_cents"`
	}

	var results []UserUsage
	q := h.db.Model(&model.UsageRecord{}).
		Where("date >= ? AND date <= ?", from, to).
		Select(`
			user_id,
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost_cents), 0) as total_cost
		`).
		Group("user_id").
		Order("total_tokens DESC").
		Limit(limit)

	if teamID != "" {
		q = q.Where("team_id = ?", teamID)
	}
	q.Find(&results)

	c.JSON(http.StatusOK, gin.H{
		"from":  from,
		"to":    to,
		"users": results,
	})
}

// UsageRecent GET /brood/billing/usage/recent
func (h *BillingHandler) UsageRecent(c *gin.Context) {
	teamID := c.Query("team_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var records []model.UsageRecord
	q := h.db.Order("created_at DESC").Limit(limit)
	if teamID != "" {
		q = q.Where("team_id = ?", teamID)
	}
	q.Find(&records)
	c.JSON(http.StatusOK, records)
}

// ==================== Budget Alerts ====================

// ListBudgetAlerts GET /brood/billing/alerts
func (h *BillingHandler) ListBudgetAlerts(c *gin.Context) {
	var alerts []model.BudgetAlert
	q := h.db.Order("created_at DESC")
	if team := c.Query("team_id"); team != "" {
		q = q.Where("team_id = ?", team)
	}
	q.Find(&alerts)
	c.JSON(http.StatusOK, alerts)
}

// CreateBudgetAlert POST /brood/billing/alerts
func (h *BillingHandler) CreateBudgetAlert(c *gin.Context) {
	var alert model.BudgetAlert
	if err := c.ShouldBindJSON(&alert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.db.Create(&alert)
	audit(h.db, c, "budget_alert.create", alert.ID, fmt.Sprintf("metric=%s threshold=%d period=%s", alert.MetricType, alert.ThresholdValue, alert.Period))
	c.JSON(http.StatusCreated, alert)
}

// UpdateBudgetAlert PUT /brood/billing/alerts/:id
func (h *BillingHandler) UpdateBudgetAlert(c *gin.Context) {
	var alert model.BudgetAlert
	if err := h.db.First(&alert, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	var req model.BudgetAlert
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.db.Model(&alert).Updates(map[string]interface{}{
		"name":            req.Name,
		"metric_type":     req.MetricType,
		"threshold_value": req.ThresholdValue,
		"period":          req.Period,
		"notify_email":    req.NotifyEmail,
		"notify_webhook":  req.NotifyWebhook,
		"enabled":         req.Enabled,
	})
	h.db.First(&alert, "id = ?", c.Param("id"))
	c.JSON(http.StatusOK, alert)
}

// DeleteBudgetAlert DELETE /brood/billing/alerts/:id
func (h *BillingHandler) DeleteBudgetAlert(c *gin.Context) {
	if err := h.db.Delete(&model.BudgetAlert{}, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	audit(h.db, c, "budget_alert.delete", c.Param("id"), "")
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// checkBudgetAlerts checks if any budget alerts should fire for the given team+date
func (h *BillingHandler) checkBudgetAlerts(teamID, date string) {
	var alerts []model.BudgetAlert
	h.db.Where("team_id = ? AND enabled = ?", teamID, true).Find(&alerts)

	if len(alerts) == 0 {
		return
	}

	// Get current daily totals
	var daily model.UsageDailySummary
	if err := h.db.Where("team_id = ? AND date = ?", teamID, date).First(&daily).Error; err != nil {
		return
	}

	for _, alert := range alerts {
		var currentValue int64
		switch alert.MetricType {
		case "tokens":
			currentValue = daily.TotalTokens
		case "cost":
			currentValue = int64(daily.TotalCostCents)
		case "star_energy":
			currentValue = int64(daily.TotalStarEnergy)
		case "requests":
			currentValue = daily.TotalRequests
		default:
			continue
		}

		if currentValue >= alert.ThresholdValue {
			now := time.Now()
			// Avoid re-triggering within the same period
			if alert.LastTriggered != nil && alert.Period == "daily" && alert.LastTriggered.Format("2006-01-02") == date {
				continue
			}
			h.db.Model(&alert).Update("last_triggered", &now)
			// Log the trigger as audit
			h.db.Create(&model.AuditLog{
				Actor:    "system",
				Action:   "budget_alert.triggered",
				TargetID: alert.ID,
				Detail:   fmt.Sprintf("alert=%s metric=%s value=%d threshold=%d", alert.Name, alert.MetricType, currentValue, alert.ThresholdValue),
			})
			// TODO: send email / webhook notification
		}
	}
}

// ==================== Billing Overview ====================

// BillingOverview GET /brood/billing/overview
func (h *BillingHandler) BillingOverview(c *gin.Context) {
	teamID := c.Query("team_id")

	// Current subscription
	var sub model.Subscription
	subQ := h.db.Where("status = ?", "active")
	if teamID != "" {
		subQ = subQ.Where("team_id = ?", teamID)
	}
	subQ.Order("created_at DESC").First(&sub)

	// Current plan
	var plan model.Plan
	if sub.PlanID != "" {
		h.db.First(&plan, "id = ?", sub.PlanID)
	}

	// Current month usage
	monthStart := time.Now().Format("2006-01") + "-01"
	monthEnd := time.Now().Format("2006-01-02")

	var monthUsage struct {
		TotalRequests  int64 `json:"total_requests"`
		TotalTokens    int64 `json:"total_tokens"`
		TotalCostCents int   `json:"total_cost_cents"`
	}
	uq := h.db.Model(&model.UsageRecord{}).
		Where("date >= ? AND date <= ?", monthStart, monthEnd).
		Select("COUNT(*) as total_requests, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost_cents), 0) as total_cost_cents")
	if teamID != "" {
		uq = uq.Where("team_id = ?", teamID)
	}
	uq.Scan(&monthUsage)

	// Today usage
	today := time.Now().Format("2006-01-02")
	var todayUsage struct {
		TotalRequests  int64 `json:"total_requests"`
		TotalTokens    int64 `json:"total_tokens"`
		TotalCostCents int   `json:"total_cost_cents"`
	}
	tq := h.db.Model(&model.UsageRecord{}).
		Where("date = ?", today).
		Select("COUNT(*) as total_requests, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost_cents), 0) as total_cost_cents")
	if teamID != "" {
		tq = tq.Where("team_id = ?", teamID)
	}
	tq.Scan(&todayUsage)

	// Active alerts count
	var alertCount int64
	aq := h.db.Model(&model.BudgetAlert{}).Where("enabled = ?", true)
	if teamID != "" {
		aq = aq.Where("team_id = ?", teamID)
	}
	aq.Count(&alertCount)

	c.JSON(http.StatusOK, gin.H{
		"subscription":  sub,
		"plan":          plan,
		"month_usage":   monthUsage,
		"today_usage":   todayUsage,
		"active_alerts": alertCount,
	})
}
