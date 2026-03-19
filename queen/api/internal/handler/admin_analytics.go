package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

type AdminAnalyticsHandler struct{}

// QueenAnalytics returns the global business metrics dashboard
// GET /v1/admin/analytics
func (h *AdminAnalyticsHandler) QueenAnalytics(c *gin.Context) {
	now := time.Now()
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")

	// --- Recharge GMV (actual payments) ---
	var rechargeTotal int64
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(amount), 0)").Scan(&rechargeTotal)

	var rechargeMonth int64
	database.DB.Model(&model.RechargeOrder{}).
		Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", thisMonth+"-01", nextMonthStr(thisMonth)+"-01").
		Select("COALESCE(SUM(amount), 0)").Scan(&rechargeMonth)

	var rechargeLastMonth int64
	database.DB.Model(&model.RechargeOrder{}).
		Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", lastMonth+"-01", thisMonth+"-01").
		Select("COALESCE(SUM(amount), 0)").Scan(&rechargeLastMonth)

	var rechargeOrderCount int64
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "paid").Count(&rechargeOrderCount)

	// --- Star Energy stats ---
	var totalEnergyGranted int64
	database.DB.Model(&model.CreditTransaction{}).Where("type IN ?", []string{"recharge", "grant"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalEnergyGranted)

	var totalEnergyConsumed int64
	database.DB.Model(&model.CreditTransaction{}).Where("type = ?", "consume").
		Select("COALESCE(SUM(amount), 0)").Scan(&totalEnergyConsumed)

	var monthEnergyConsumed int64
	database.DB.Model(&model.CreditTransaction{}).
		Where("type = ? AND created_at >= ?", "consume", thisMonth+"-01").
		Select("COALESCE(SUM(amount), 0)").Scan(&monthEnergyConsumed)

	// --- User stats ---
	var totalUsers int64
	database.DB.Model(&model.User{}).Count(&totalUsers)

	var monthNewUsers int64
	database.DB.Model(&model.User{}).Where("created_at >= ?", thisMonth+"-01").Count(&monthNewUsers)

	var payingUsers int64
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "paid").
		Distinct("user_id").Count(&payingUsers)

	// ARPU = total recharge / paying users
	var arpu float64
	if payingUsers > 0 {
		arpu = float64(rechargeTotal) / float64(payingUsers) / 100.0 // in ¥
	}

	// --- CRM GMV (deal pipeline) ---
	var totalGMV int64
	database.DB.Model(&model.CRMDeal{}).
		Where("stage IN ?", []string{"signed", "delivery", "active", "renewal"}).
		Select("COALESCE(SUM(deal_value), 0)").Scan(&totalGMV)

	var monthGMV int64
	database.DB.Model(&model.CRMDeal{}).
		Where("stage IN ? AND updated_at >= ? AND updated_at < ?",
			[]string{"signed", "delivery", "active", "renewal"},
			thisMonth+"-01", nextMonthStr(thisMonth)+"-01",
		).Select("COALESCE(SUM(deal_value), 0)").Scan(&monthGMV)

	// --- MRR (Monthly Recurring Revenue) from active clients ---
	var mrr int64
	database.DB.Model(&model.CityClient{}).
		Where("status = ?", "active").
		Select("COALESCE(SUM(mrr), 0)").Scan(&mrr)

	// Also add CRM active deals as annualized → monthly
	var activeDealValue int64
	database.DB.Model(&model.CRMDeal{}).
		Where("stage = ?", "active").
		Select("COALESCE(SUM(deal_value), 0)").Scan(&activeDealValue)
	mrr += activeDealValue / 12

	arr := mrr * 12

	// --- Client stats ---
	var totalClients int64
	database.DB.Model(&model.CRMDeal{}).
		Where("stage IN ?", []string{"signed", "delivery", "active", "renewal"}).
		Count(&totalClients)

	var activeClients int64
	database.DB.Model(&model.CRMDeal{}).Where("stage = ?", "active").Count(&activeClients)

	var totalCityClients int64
	database.DB.Model(&model.CityClient{}).Where("status = ?", "active").Count(&totalCityClients)

	// --- Renewal rate ---
	var renewalDeals int64
	database.DB.Model(&model.CRMDeal{}).Where("stage = ?", "renewal").Count(&renewalDeals)

	var churnedDeals int64
	database.DB.Model(&model.CRMDeal{}).Where("stage = ?", "churned").Count(&churnedDeals)

	renewalRate := float64(0)
	if renewalDeals+churnedDeals > 0 {
		renewalRate = float64(renewalDeals) / float64(renewalDeals+churnedDeals) * 100
	}

	// --- Partner stats ---
	var corePartners int64
	database.DB.Model(&model.TeamPartner{}).Where("status = ?", "active").Count(&corePartners)

	var cityPartners int64
	database.DB.Model(&model.CityPartner{}).Where("status = ?", "approved").Count(&cityPartners)

	var pendingCityPartners int64
	database.DB.Model(&model.CityPartner{}).Where("status = ?", "pending").Count(&pendingCityPartners)

	// --- Commission stats ---
	var totalCommPaid int64
	database.DB.Model(&model.SettlementBill{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(total_amount), 0)").Scan(&totalCommPaid)

	var monthComm int64
	database.DB.Model(&model.SettlementBill{}).Where("month = ?", thisMonth).
		Select("COALESCE(SUM(total_amount), 0)").Scan(&monthComm)

	// --- Pipeline funnel ---
	type StageCount struct {
		Stage string `json:"stage"`
		Count int64  `json:"count"`
		Value int64  `json:"value"`
	}
	var pipeline []StageCount
	database.DB.Model(&model.CRMDeal{}).
		Select("stage, COUNT(*) as count, COALESCE(SUM(deal_value),0) as value").
		Group("stage").Order("FIELD(stage, 'lead','opportunity','negotiation','signed','delivery','active','renewal','churned')").
		Find(&pipeline)

	// --- Monthly trend (last 6 months GMV) ---
	type MonthTrend struct {
		Month string `json:"month"`
		GMV   int64  `json:"gmv"`
		Deals int64  `json:"deals"`
	}
	var trend []MonthTrend
	for i := 5; i >= 0; i-- {
		m := now.AddDate(0, -i, 0).Format("2006-01")
		var gmv int64
		var deals int64
		database.DB.Model(&model.CRMDeal{}).
			Where("stage IN ? AND updated_at >= ? AND updated_at < ?",
				[]string{"signed", "delivery", "active", "renewal"},
				m+"-01", nextMonthStr(m)+"-01",
			).Select("COALESCE(SUM(deal_value), 0)").Scan(&gmv)
		database.DB.Model(&model.CRMDeal{}).
			Where("stage IN ? AND updated_at >= ? AND updated_at < ?",
				[]string{"signed", "delivery", "active", "renewal"},
				m+"-01", nextMonthStr(m)+"-01",
			).Count(&deals)
		trend = append(trend, MonthTrend{Month: m, GMV: gmv, Deals: deals})
	}

	// --- Channel health (top city partners by revenue) ---
	type ChannelHealth struct {
		ID           string  `json:"id"`
		Name         string  `json:"name"`
		City         string  `json:"city"`
		TotalEarned  int64   `json:"total_earned"`
		TotalClients int     `json:"total_clients"`
		CommRate     float64 `json:"comm_rate"`
	}
	var channels []ChannelHealth
	database.DB.Model(&model.CityPartner{}).
		Where("status = ?", "approved").
		Order("total_earned DESC").Limit(10).
		Find(&channels)

	// --- Monthly recharge trend (last 6 months) ---
	type RechargeTrend struct {
		Month    string `json:"month"`
		Recharge int64  `json:"recharge"`
		Orders   int64  `json:"orders"`
		Users    int64  `json:"users"`
	}
	var rechargeTrend []RechargeTrend
	for i := 5; i >= 0; i-- {
		m := now.AddDate(0, -i, 0).Format("2006-01")
		var recharge int64
		var orders int64
		var users int64
		database.DB.Model(&model.RechargeOrder{}).
			Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", m+"-01", nextMonthStr(m)+"-01").
			Select("COALESCE(SUM(amount), 0)").Scan(&recharge)
		database.DB.Model(&model.RechargeOrder{}).
			Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", m+"-01", nextMonthStr(m)+"-01").
			Count(&orders)
		database.DB.Model(&model.RechargeOrder{}).
			Where("status = ? AND paid_at >= ? AND paid_at < ?", "paid", m+"-01", nextMonthStr(m)+"-01").
			Distinct("user_id").Count(&users)
		rechargeTrend = append(rechargeTrend, RechargeTrend{Month: m, Recharge: recharge, Orders: orders, Users: users})
	}

	// MoM growth
	rechargeGrowth := float64(0)
	if rechargeLastMonth > 0 {
		rechargeGrowth = float64(rechargeMonth-rechargeLastMonth) / float64(rechargeLastMonth) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		// Recharge metrics
		"recharge_total":  rechargeTotal,
		"recharge_month":  rechargeMonth,
		"recharge_growth": rechargeGrowth,
		"recharge_orders": rechargeOrderCount,
		"recharge_trend":  rechargeTrend,
		// Star Energy
		"energy_granted":   totalEnergyGranted,
		"energy_consumed":  totalEnergyConsumed,
		"energy_month":     monthEnergyConsumed,
		"energy_retention": float64(totalEnergyGranted-totalEnergyConsumed) / float64(max(totalEnergyGranted, 1)) * 100,
		// User metrics
		"total_users":     totalUsers,
		"month_new_users": monthNewUsers,
		"paying_users":    payingUsers,
		"arpu":            arpu,
		// CRM pipeline
		"gmv":            totalGMV,
		"month_gmv":      monthGMV,
		"mrr":            mrr,
		"arr":            arr,
		"total_clients":  totalClients,
		"active_clients": activeClients,
		"city_clients":   totalCityClients,
		"renewal_rate":   renewalRate,
		// Partners
		"core_partners":    corePartners,
		"city_partners":    cityPartners,
		"pending_cities":   pendingCityPartners,
		"total_comm_paid":  totalCommPaid,
		"month_commission": monthComm,
		// Detailed breakdowns
		"pipeline": pipeline,
		"trend":    trend,
		"channels": channels,
	})
}

// AdminListAllClients returns all enterprise clients (CRM deals + city clients combined)
// GET /v1/admin/clients
func (h *AdminAnalyticsHandler) AdminListAllClients(c *gin.Context) {
	type ClientRow struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		Source      string     `json:"source"` // "crm" or "city"
		PartnerName string     `json:"partner_name"`
		Plan        string     `json:"plan"`
		Stage       string     `json:"stage"`
		MRR         int64      `json:"mrr"`
		DealValue   int64      `json:"deal_value"`
		SignedAt    *time.Time `json:"signed_at"`
		RenewAt     *time.Time `json:"renew_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}

	var clients []ClientRow

	// CRM deals with partner name
	var deals []model.CRMDeal
	stageFilter := c.Query("stage")
	q := database.DB.Model(&model.CRMDeal{})
	if stageFilter != "" {
		q = q.Where("stage = ?", stageFilter)
	}
	q.Order("updated_at DESC").Find(&deals)

	// Build partner name map
	partnerMap := map[string]string{}
	var partners []model.TeamPartner
	database.DB.Find(&partners)
	for _, p := range partners {
		partnerMap[p.ID] = p.Name
	}

	for _, d := range deals {
		clients = append(clients, ClientRow{
			ID: d.ID, Name: d.CompanyName, Source: "crm",
			PartnerName: partnerMap[d.PartnerID], Plan: d.Plan,
			Stage: d.Stage, DealValue: d.DealValue,
			SignedAt: d.SignedAt, RenewAt: d.RenewAt,
			CreatedAt: d.CreatedAt,
		})
	}

	// City clients
	var cityClients []model.CityClient
	cq := database.DB.Model(&model.CityClient{})
	if stageFilter != "" {
		cq = cq.Where("status = ?", stageFilter)
	}
	cq.Order("updated_at DESC").Find(&cityClients)

	cityMap := map[string]string{}
	var cityPartners []model.CityPartner
	database.DB.Find(&cityPartners)
	for _, cp := range cityPartners {
		cityMap[cp.ID] = cp.Name
	}

	for _, cc := range cityClients {
		clients = append(clients, ClientRow{
			ID: cc.ID, Name: cc.ClientName, Source: "city",
			PartnerName: cityMap[cc.PartnerID], Plan: cc.Plan,
			Stage: cc.Status, MRR: cc.MRR,
			SignedAt: cc.SignedAt, RenewAt: cc.RenewAt,
			CreatedAt: cc.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"clients": clients, "total": len(clients)})
}

// AdminPartnerPerformance returns combined partner performance data
// GET /v1/admin/partners/performance
func (h *AdminAnalyticsHandler) AdminPartnerPerformance(c *gin.Context) {
	type PartnerPerf struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		Type            string  `json:"type"` // core / city
		Region          string  `json:"region"`
		Status          string  `json:"status"`
		TotalRevenue    int64   `json:"total_revenue"`
		TotalCommission int64   `json:"total_commission"`
		ActiveClients   int     `json:"active_clients"`
		DealCount       int64   `json:"deal_count"`
		Level           string  `json:"level"`
		CommRate        float64 `json:"comm_rate"`
	}

	var result []PartnerPerf

	// Core partners
	var corePartners []model.TeamPartner
	database.DB.Order("total_revenue DESC").Find(&corePartners)
	for _, p := range corePartners {
		var dealCount int64
		database.DB.Model(&model.CRMDeal{}).Where("partner_id = ?", p.ID).Count(&dealCount)
		result = append(result, PartnerPerf{
			ID: p.ID, Name: p.Name, Type: "core", Region: p.Region,
			Status: p.Status, TotalRevenue: p.TotalRevenue,
			TotalCommission: p.TotalCommission, ActiveClients: p.ActiveClients,
			DealCount: dealCount, Level: p.Level, CommRate: p.DirectCommRate,
		})
	}

	// City partners
	var cityPartners []model.CityPartner
	database.DB.Order("total_earned DESC").Find(&cityPartners)
	for _, cp := range cityPartners {
		result = append(result, PartnerPerf{
			ID: cp.ID, Name: cp.Name, Type: "city", Region: cp.City,
			Status: cp.Status, TotalCommission: cp.TotalEarned,
			ActiveClients: cp.TotalClients, CommRate: cp.CommRate,
		})
	}

	c.JSON(http.StatusOK, gin.H{"partners": result, "total": len(result)})
}

func nextMonthStr(month string) string {
	t, _ := time.Parse("2006-01", month)
	t = t.AddDate(0, 1, 0)
	return t.Format("2006-01")
}
