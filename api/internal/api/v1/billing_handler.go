package v1

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw/internal/billing"
	"github.com/yinhe/starclaw/internal/model"
	"github.com/yinhe/starclaw/internal/swarm"
	"github.com/yinhe/starclaw/internal/tool"
	"gorm.io/gorm"
)

// queenBilling is the centralized billing client for hosted mode.
// When set, checkQuota and recordUsage will also call Queen API.
var queenBilling *swarm.BillingClient

// billingQueenClient is the billing gateway's QueenClient (set from router setup)
var billingQueenClient billingChecker

// billingChecker is the interface for checking balance via Queen
type billingChecker interface {
	CheckBalance(userID string) (bool, int64, error)
	IsEnabled() bool
}

// SetQueenBilling sets the centralized billing client (called from router setup)
func SetQueenBilling(bc *swarm.BillingClient) {
	queenBilling = bc
}

// SetBillingQueenClient sets the billing gateway's queen client for balance queries
func SetBillingQueenClient(c billingChecker) {
	billingQueenClient = c
}

// Resource pricing (兀per unit)
var resourcePrice = map[string]float64{
	"tokens": 0.00001, // ¥0.01 per 1K tokens
	"video":  2.0,     // ¥2 per video
	"image":  0.5,     // ¥0.5 per image
	"music":  1.0,     // ¥1 per music
}

type BillingHandler struct {
	db *gorm.DB
}

func NewBillingHandler(db *gorm.DB) *BillingHandler {
	return &BillingHandler{db: db}
}

// SeedPlans creates default recharge packages
func (h *BillingHandler) SeedPlans() {
	plans := []model.Plan{
		{ID: "pkg-10", Name: "pkg-10", DisplayName: "¥10", Price: 10, Credits: 10, BonusPct: 0, Tag: "", SortOrder: 0},
		{ID: "pkg-50", Name: "pkg-50", DisplayName: "¥50", Price: 50, Credits: 55, BonusPct: 10, Tag: "", SortOrder: 1},
		{ID: "pkg-100", Name: "pkg-100", DisplayName: "¥100", Price: 100, Credits: 120, BonusPct: 20, Tag: "热门", SortOrder: 2},
		{ID: "pkg-500", Name: "pkg-500", DisplayName: "¥500", Price: 500, Credits: 650, BonusPct: 30, Tag: "超值", SortOrder: 3},
		{ID: "pkg-1000", Name: "pkg-1000", DisplayName: "¥1000", Price: 1000, Credits: 1400, BonusPct: 40, Tag: "至尊", SortOrder: 4},
	}
	for _, p := range plans {
		h.db.Where("id = ?", p.ID).FirstOrCreate(&p)
	}
}

// EnsureTenant ensures the user has a tenant; creates one with 0 balance if not
func (h *BillingHandler) EnsureTenant(userID, username string) {
	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		return
	}
	if user.TenantID != "" {
		return
	}
	tenant := model.Tenant{
		Name:    username + " 的团队",
		OwnerID: userID,
		Balance: 0,
	}
	h.db.Create(&tenant)
	h.db.Model(&user).Update("tenant_id", tenant.ID)
	h.db.Create(&model.TenantMember{
		TenantID: tenant.ID,
		UserID:   userID,
		Role:     "owner",
	})
}

// ---------- Recharge / Balance ----------

// ListPlans returns available recharge packages
func (h *BillingHandler) ListPlans(c *gin.Context) {
	var plans []model.Plan
	h.db.Order("sort_order ASC").Find(&plans)
	c.JSON(200, gin.H{"plans": plans, "pricing": resourcePrice})
}

// GetCurrentPlan returns the tenant balance + current month usage
func (h *BillingHandler) GetCurrentPlan(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	month := time.Now().Format("2006-01")
	usage := h.getMonthUsage(tenant.ID, userID, month)
	cost := h.getMonthCost(tenant.ID, userID, month)
	usageBySrc := h.getMonthUsageBySource(tenant.ID, userID, month)
	if summary, err := tool.GetStarAIUsageSummary(); err == nil && summary != nil {
		if len(summary.Usage) > 0 {
			usage = summary.Usage
		}
		if len(summary.Cost) > 0 {
			cost = summary.Cost
		}
		if len(summary.UsageBySource) > 0 {
			usageBySrc = summary.UsageBySource
		}
		if summary.Period != "" {
			month = summary.Period
		}
	}

	// Use Queen balance as authoritative source if available
	balance := tenant.Balance
	if billingQueenClient != nil && billingQueenClient.IsEnabled() {
		if _, queenBalance, err := billingQueenClient.CheckBalance(userID); err == nil {
			balance = float64(queenBalance) // Queen returns balance in 分 (stars)
		}
	}

	c.JSON(200, gin.H{
		"tenant":          tenant,
		"balance":         balance,
		"usage":           usage,
		"cost":            cost,
		"usage_by_source": usageBySrc,
		"period":          month,
		"pricing":         resourcePrice,
	})
}

// Recharge adds credits to tenant balance
func (h *BillingHandler) Recharge(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		PlanID string `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var plan model.Plan
	if err := h.db.First(&plan, "id = ?", req.PlanID).Error; err != nil {
		c.JSON(404, gin.H{"error": "recharge package not found"})
		return
	}

	// Add credits to balance
	newBalance := tenant.Balance + plan.Credits
	h.db.Model(&tenant).Update("balance", newBalance)

	// Record transaction
	h.db.Create(&model.Transaction{
		TenantID: tenant.ID,
		UserID:   userID,
		Type:     "recharge",
		Amount:   plan.Credits,
		Balance:  newBalance,
		Remark:   fmt.Sprintf("充值 %s（支付¥%.0f，到账¥%.0f）", plan.DisplayName, plan.Price, plan.Credits),
	})

	c.JSON(200, gin.H{
		"message": "充值成功",
		"credits": plan.Credits,
		"balance": newBalance,
	})
}

// ---------- Usage endpoints ----------

// GetUsageHistory returns monthly usage + cost summary
func (h *BillingHandler) GetUsageHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if summary, err := tool.GetStarAIUsageSummary(); err == nil && summary != nil && len(summary.History) > 0 {
		c.JSON(200, gin.H{"usage": summary.History})
		return
	}

	var results []struct {
		Month        string  `json:"month"`
		ResourceType string  `json:"resource_type"`
		Total        int64   `json:"total"`
		TotalCost    float64 `json:"total_cost"`
	}
	h.db.Model(&model.UsageRecord{}).
		Select("LEFT(date, 7) as month, resource_type, SUM(quantity) as total, SUM(cost) as total_cost").
		Where("tenant_id = ? AND date >= ?", tenant.ID, time.Now().AddDate(0, -6, 0).Format("2006-01-02")).
		Group("month, resource_type").
		Order("month DESC").
		Find(&results)

	c.JSON(200, gin.H{"usage": results})
}

// GetDailyUsage returns daily usage for a month
func (h *BillingHandler) GetDailyUsage(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	month := c.DefaultQuery("month", time.Now().Format("2006-01"))
	var results []struct {
		Date         string  `json:"date"`
		ResourceType string  `json:"resource_type"`
		Total        int64   `json:"total"`
		TotalCost    float64 `json:"total_cost"`
	}
	h.db.Model(&model.UsageRecord{}).
		Select("date, resource_type, SUM(quantity) as total, SUM(cost) as total_cost").
		Where("tenant_id = ? AND date LIKE ?", tenant.ID, month+"%").
		Group("date, resource_type").
		Order("date ASC").
		Find(&results)

	c.JSON(200, gin.H{"daily_usage": results, "month": month})
}

// ListTransactions returns recent balance changes
func (h *BillingHandler) ListTransactions(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var txns []model.Transaction
	h.db.Where("tenant_id = ?", tenant.ID).Order("created_at DESC").Limit(50).Find(&txns)
	c.JSON(200, gin.H{"transactions": txns})
}

// ---------- Tenant endpoints ----------

// GetTenant returns the current tenant info with members
func (h *BillingHandler) GetTenant(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	type MemberInfo struct {
		ID       string    `json:"id"`
		UserID   string    `json:"user_id"`
		Username string    `json:"username"`
		Email    string    `json:"email"`
		Role     string    `json:"role"`
		JoinedAt time.Time `json:"joined_at"`
	}
	var members []MemberInfo
	h.db.Model(&model.TenantMember{}).
		Select("tenant_members.id, tenant_members.user_id, users.username, users.email, tenant_members.role, tenant_members.joined_at").
		Joins("JOIN users ON users.id = tenant_members.user_id").
		Where("tenant_members.tenant_id = ?", tenant.ID).
		Find(&members)

	c.JSON(200, gin.H{"tenant": tenant, "members": members})
}

// UpdateTenant updates tenant info
func (h *BillingHandler) UpdateTenant(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if tenant.OwnerID != userID {
		c.JSON(403, gin.H{"error": "only owner can update tenant"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		h.db.Model(&tenant).Update("name", req.Name)
	}
	c.JSON(200, gin.H{"tenant": tenant})
}

// AddMember adds a user to the tenant by email
func (h *BillingHandler) AddMember(c *gin.Context) {
	userID := c.GetString("user_id")
	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var myMember model.TenantMember
	if err := h.db.Where("tenant_id = ? AND user_id = ?", tenant.ID, userID).First(&myMember).Error; err != nil {
		c.JSON(403, gin.H{"error": "not a member"})
		return
	}
	if myMember.Role != "owner" && myMember.Role != "admin" {
		c.JSON(403, gin.H{"error": "only owner/admin can add members"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required"`
		Role  string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}

	var targetUser model.User
	if err := h.db.Where("email = ?", req.Email).First(&targetUser).Error; err != nil {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	var existing model.TenantMember
	if err := h.db.Where("tenant_id = ? AND user_id = ?", tenant.ID, targetUser.ID).First(&existing).Error; err == nil {
		c.JSON(409, gin.H{"error": "user already a member"})
		return
	}

	member := model.TenantMember{
		TenantID: tenant.ID,
		UserID:   targetUser.ID,
		Role:     req.Role,
	}
	h.db.Create(&member)
	h.db.Model(&targetUser).Update("tenant_id", tenant.ID)

	c.JSON(201, gin.H{"member": member})
}

// RemoveMember removes a user from the tenant
func (h *BillingHandler) RemoveMember(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("user_id")

	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if targetUserID == tenant.OwnerID {
		c.JSON(403, gin.H{"error": "cannot remove tenant owner"})
		return
	}

	var myMember model.TenantMember
	if err := h.db.Where("tenant_id = ? AND user_id = ?", tenant.ID, userID).First(&myMember).Error; err != nil {
		c.JSON(403, gin.H{"error": "not a member"})
		return
	}
	if myMember.Role != "owner" && myMember.Role != "admin" {
		c.JSON(403, gin.H{"error": "only owner/admin can remove members"})
		return
	}

	h.db.Where("tenant_id = ? AND user_id = ?", tenant.ID, targetUserID).Delete(&model.TenantMember{})
	h.db.Model(&model.User{}).Where("id = ?", targetUserID).Update("tenant_id", "")

	c.JSON(200, gin.H{"message": "member removed"})
}

// UpdateMemberRole changes a member's role
func (h *BillingHandler) UpdateMemberRole(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("user_id")

	tenant, err := h.getTenant(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if tenant.OwnerID != userID {
		c.JSON(403, gin.H{"error": "only owner can change roles"})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&model.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenant.ID, targetUserID).
		Update("role", req.Role)

	c.JSON(200, gin.H{"message": "role updated"})
}

// ---------- Usage Recording (called by other handlers) ----------

// recordUsage records usage and optionally deducts cost from tenant balance.
// When platformKey is true (using platform shared key), cost is calculated and balance deducted.
// When platformKey is false (BYOK), usage is recorded for stats only, no cost or deduction.
// If Queen billing client is enabled, consumption is also reported to Queen centrally.
func recordUsage(db *gorm.DB, userID, resourceType string, quantity int64, platformKey bool) {
	// Report to Queen billing if available (centralized billing)
	if platformKey && queenBilling != nil && queenBilling.IsEnabled() {
		remark := fmt.Sprintf("%s x%d", resourceType, quantity)
		if _, err := queenBilling.Consume(userID, resourceType, quantity, remark); err != nil {
			log.Printf("[billing] Queen consume failed: %v (falling back to local)", err)
		}
	}

	// Always record locally for stats
	var user model.User
	if err := db.First(&user, "id = ?", userID).Error; err != nil || user.TenantID == "" {
		return
	}

	var cost float64
	if platformKey {
		price, ok := resourcePrice[resourceType]
		if !ok {
			price = 0
		}
		cost = math.Round(float64(quantity)*price*10000) / 10000
	}

	// Source: "starai" when using platform/StarAI key, "self" when using user's own API key
	source := "self"
	if platformKey {
		source = "starai"
	}

	today := time.Now().Format("2006-01-02")

	var existing model.UsageRecord
	result := db.Where("tenant_id = ? AND resource_type = ? AND date = ? AND user_id = ? AND source = ?",
		user.TenantID, resourceType, today, userID, source).First(&existing)

	if result.Error == nil {
		db.Model(&existing).Updates(map[string]interface{}{
			"quantity": gorm.Expr("quantity + ?", quantity),
			"cost":     gorm.Expr("cost + ?", cost),
		})
	} else {
		db.Create(&model.UsageRecord{
			TenantID:     user.TenantID,
			UserID:       userID,
			ResourceType: resourceType,
			Source:       source,
			Quantity:     quantity,
			Cost:         cost,
			Date:         today,
		})
	}

	// Deduct from local balance only when using platform key
	if platformKey && cost > 0 {
		db.Model(&model.Tenant{}).Where("id = ?", user.TenantID).
			Update("balance", gorm.Expr("GREATEST(balance - ?, 0)", cost))
	}

	// Sync star energy: deduct local stardust when using star-ai (platform key)
	// Rate: 1 stardust per 1000 tokens (minimum 1 if any tokens used)
	if platformKey && resourceType == "tokens" && quantity > 0 {
		stardustCost := int(quantity / 1000)
		if stardustCost < 1 {
			stardustCost = 1
		}
		db.Model(&model.NodeGrowth{}).
			Where("user_id = ? AND stardust_balance >= ?", userID, stardustCost).
			Update("stardust_balance", gorm.Expr("stardust_balance - ?", stardustCost))
		db.Create(&model.StardustTransaction{
			UserID: userID,
			Amount: -stardustCost,
			Type:   "spend_ai_tokens",
			Note:   fmt.Sprintf("%s x%d tokens", resourceType, quantity),
		})
	}
}

// ---------- Helpers ----------

func (h *BillingHandler) getTenant(userID string) (*model.Tenant, error) {
	var user model.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.TenantID == "" {
		h.EnsureTenant(userID, user.Username)
		h.db.First(&user, "id = ?", userID)
	}

	var tenant model.Tenant
	if err := h.db.First(&tenant, "id = ?", user.TenantID).Error; err != nil {
		return nil, fmt.Errorf("tenant not found")
	}

	return &tenant, nil
}

func (h *BillingHandler) getMonthUsage(tenantID, userID, month string) map[string]int64 {
	usage := map[string]int64{"tokens": 0, "video": 0, "image": 0, "music": 0}

	var results []struct {
		ResourceType string
		Total        int64
	}
	h.db.Model(&model.UsageRecord{}).
		Select("resource_type, SUM(quantity) as total").
		Where("tenant_id = ? AND date LIKE ?", tenantID, month+"%").
		Group("resource_type").
		Find(&results)

	for _, r := range results {
		usage[r.ResourceType] = r.Total
	}

	// Also aggregate tool usage records (image/video/music generation)
	var toolResults []struct {
		ResourceType string
		Total        int64
	}
	h.db.Model(&billing.ToolUsageRecord{}).
		Select("resource_type, COUNT(*) as total").
		Where("user_id = ? AND created_at >= ? AND created_at < ? AND success = true", userID, month+"-01", h.nextMonth(month)+"-01").
		Group("resource_type").
		Find(&toolResults)
	for _, r := range toolResults {
		usage[r.ResourceType] += r.Total
	}
	return usage
}

func (h *BillingHandler) getMonthCost(tenantID, userID, month string) map[string]float64 {
	cost := map[string]float64{"tokens": 0, "video": 0, "image": 0, "music": 0}

	var results []struct {
		ResourceType string
		TotalCost    float64
	}
	h.db.Model(&model.UsageRecord{}).
		Select("resource_type, SUM(cost) as total_cost").
		Where("tenant_id = ? AND date LIKE ?", tenantID, month+"%").
		Group("resource_type").
		Find(&results)

	for _, r := range results {
		cost[r.ResourceType] = r.TotalCost
	}

	// Also aggregate tool usage cost
	var toolResults []struct {
		ResourceType string
		TotalCost    float64
	}
	h.db.Model(&billing.ToolUsageRecord{}).
		Select("resource_type, SUM(cost_fen) / 100.0 as total_cost").
		Where("user_id = ? AND created_at >= ? AND created_at < ? AND success = true", userID, month+"-01", h.nextMonth(month)+"-01").
		Group("resource_type").
		Find(&toolResults)
	for _, r := range toolResults {
		cost[r.ResourceType] += r.TotalCost
	}
	return cost
}

// getMonthUsageBySource returns usage split by source (starai vs self)
func (h *BillingHandler) getMonthUsageBySource(tenantID, _, month string) map[string]map[string]int64 {
	result := map[string]map[string]int64{
		"starai": {"tokens": 0, "video": 0, "image": 0, "music": 0},
		"self":   {"tokens": 0, "video": 0, "image": 0, "music": 0},
	}

	var rows []struct {
		Source       string
		ResourceType string
		Total        int64
	}
	h.db.Model(&model.UsageRecord{}).
		Select("COALESCE(source, 'self') as source, resource_type, SUM(quantity) as total").
		Where("tenant_id = ? AND date LIKE ?", tenantID, month+"%").
		Group("source, resource_type").
		Find(&rows)

	for _, r := range rows {
		src := r.Source
		if src == "" {
			src = "self"
		}
		if _, ok := result[src]; !ok {
			result[src] = map[string]int64{}
		}
		result[src][r.ResourceType] = r.Total
	}
	return result
}

// nextMonth returns the next month in "2006-01" format
func (h *BillingHandler) nextMonth(month string) string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return month
	}
	return t.AddDate(0, 1, 0).Format("2006-01")
}
