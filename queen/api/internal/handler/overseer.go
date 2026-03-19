package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/model"
)

// OverseerHandler provides monitoring endpoints for the Overseer dashboard
type OverseerHandler struct {
	promURL  string
	swarmURL string
	httpC    *http.Client
}

func NewOverseerHandler() *OverseerHandler {
	promURL := os.Getenv("PROMETHEUS_URL")
	if promURL == "" {
		promURL = "http://prometheus:9090"
	}
	swarmURL := os.Getenv("SWARM_URL")
	if swarmURL == "" {
		swarmURL = "http://swarm:8090"
	}
	return &OverseerHandler{
		promURL:  promURL,
		swarmURL: swarmURL,
		httpC:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Dashboard returns aggregated overview stats
func (h *OverseerHandler) Dashboard(c *gin.Context) {
	// Node stats from swarm
	var nodeStats struct {
		Total  int64 `json:"total"`
		Online int64 `json:"online"`
	}
	resp, err := h.httpC.Get(h.swarmURL + "/v1/nodes/stats")
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if stats, ok := result["stats"].(map[string]interface{}); ok {
			if v, ok := stats["total"].(float64); ok {
				nodeStats.Total = int64(v)
			}
			if v, ok := stats["online"].(float64); ok {
				nodeStats.Online = int64(v)
			}
		}
	}

	// Star energy stats
	var energyStats struct {
		TotalAccounts int64   `json:"total_accounts"`
		TotalBalance  float64 `json:"total_balance"`
		TotalGranted  float64 `json:"total_granted"`
		TotalConsumed float64 `json:"total_consumed"`
	}
	database.DB.Model(&model.CreditAccount{}).Count(&energyStats.TotalAccounts)
	var sumBalance, sumIn, sumOut int64
	database.DB.Model(&model.CreditAccount{}).Select("COALESCE(SUM(balance),0)").Scan(&sumBalance)
	database.DB.Model(&model.CreditAccount{}).Select("COALESCE(SUM(total_in),0)").Scan(&sumIn)
	database.DB.Model(&model.CreditAccount{}).Select("COALESCE(SUM(total_out),0)").Scan(&sumOut)
	energyStats.TotalBalance = float64(sumBalance) / 10000
	energyStats.TotalGranted = float64(sumIn) / 10000
	energyStats.TotalConsumed = float64(sumOut) / 10000

	// User stats
	var userCount int64
	database.DB.Model(&model.User{}).Count(&userCount)

	// Marketplace stats
	var itemCount int64
	database.DB.Model(&model.MarketplaceItem{}).Where("status IN ?", []string{"approved", "published"}).Count(&itemCount)

	thisMonth := time.Now().Format("2006-01")

	// Recharge stats
	var rechargeStats struct {
		TotalAmount   int64 `json:"total_amount"`
		MonthAmount   int64 `json:"month_amount"`
		TotalOrders   int64 `json:"total_orders"`
		PendingOrders int64 `json:"pending_orders"`
	}
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(amount), 0)").Scan(&rechargeStats.TotalAmount)
	database.DB.Model(&model.RechargeOrder{}).
		Where("status = ? AND paid_at >= ?", "paid", thisMonth+"-01").
		Select("COALESCE(SUM(amount), 0)").Scan(&rechargeStats.MonthAmount)
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "paid").Count(&rechargeStats.TotalOrders)
	database.DB.Model(&model.RechargeOrder{}).Where("status = ?", "pending").Count(&rechargeStats.PendingOrders)

	// Commission stats
	var commissionStats struct {
		TotalPaid    int64 `json:"total_paid"`
		MonthPending int64 `json:"month_pending"`
		CityPartners int64 `json:"city_partners"`
		TeamPartners int64 `json:"core_partners"`
	}
	database.DB.Model(&model.Commission{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(amount), 0)").Scan(&commissionStats.TotalPaid)
	database.DB.Model(&model.Commission{}).Where("status = ? AND month = ?", "pending", thisMonth).
		Select("COALESCE(SUM(amount), 0)").Scan(&commissionStats.MonthPending)
	database.DB.Model(&model.CityPartner{}).Where("status = ?", "approved").Count(&commissionStats.CityPartners)
	database.DB.Model(&model.TeamPartner{}).Where("status = ?", "active").Count(&commissionStats.TeamPartners)

	// Settlement stats
	var settlementStats struct {
		PendingBills int64 `json:"pending_bills"`
		PendingTotal int64 `json:"pending_total"`
		PaidThisYear int64 `json:"paid_this_year"`
	}
	database.DB.Model(&model.SettlementBill{}).Where("status IN ?", []string{"draft", "pending_review"}).
		Count(&settlementStats.PendingBills)
	database.DB.Model(&model.SettlementBill{}).Where("status IN ?", []string{"draft", "pending_review"}).
		Select("COALESCE(SUM(total_amount), 0)").Scan(&settlementStats.PendingTotal)
	yearStart := time.Now().Format("2006") + "-01-01"
	database.DB.Model(&model.SettlementBill{}).Where("status = ? AND paid_at >= ?", "paid", yearStart).
		Select("COALESCE(SUM(total_amount), 0)").Scan(&settlementStats.PaidThisYear)

	c.JSON(200, gin.H{
		"nodes":       nodeStats,
		"energy":      energyStats,
		"users":       userCount,
		"marketplace": itemCount,
		"recharge":    rechargeStats,
		"commissions": commissionStats,
		"settlement":  settlementStats,
	})
}

// Nodes returns node list from swarm
func (h *OverseerHandler) Nodes(c *gin.Context) {
	url := fmt.Sprintf("%s/v1/nodes?page=%s&size=%s&status=%s",
		h.swarmURL, c.DefaultQuery("page", "1"), c.DefaultQuery("size", "50"), c.Query("status"))
	h.proxyGet(c, url)
}

// NodeDetail returns single node info
func (h *OverseerHandler) NodeDetail(c *gin.Context) {
	url := fmt.Sprintf("%s/v1/nodes/%s", h.swarmURL, c.Param("id"))
	h.proxyGet(c, url)
}

// Services checks health of all queen services
func (h *OverseerHandler) Services(c *gin.Context) {
	services := []struct {
		Name string `json:"name"`
		URL  string `json:"-"`
	}{
		{"queen-api", "http://localhost:8085/health"},
		{"swarm", h.swarmURL + "/health"},
		{"bounty", os.Getenv("BOUNTY_URL") + "/health"},
		{"forum", os.Getenv("FORUM_URL") + "/health"},
		{"arena", os.Getenv("ARENA_URL") + "/health"},
		{"prometheus", h.promURL + "/-/healthy"},
	}

	type ServiceStatus struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Latency int64  `json:"latency_ms"`
	}

	results := make([]ServiceStatus, len(services))
	ch := make(chan struct {
		idx int
		ss  ServiceStatus
	}, len(services))

	for i, svc := range services {
		go func(idx int, name, url string) {
			start := time.Now()
			resp, err := h.httpC.Get(url)
			latency := time.Since(start).Milliseconds()
			status := "down"
			if err == nil && resp.StatusCode == 200 {
				status = "up"
				resp.Body.Close()
			}
			ch <- struct {
				idx int
				ss  ServiceStatus
			}{idx, ServiceStatus{name, status, latency}}
		}(i, svc.Name, svc.URL)
	}

	for range services {
		r := <-ch
		results[r.idx] = r.ss
	}

	c.JSON(200, gin.H{"services": results})
}

// Energy returns star energy economy details
func (h *OverseerHandler) Energy(c *gin.Context) {
	// Top accounts
	var topAccounts []model.CreditAccount
	database.DB.Order("balance DESC").Limit(20).Find(&topAccounts)

	// Recent transactions
	var recentTx []model.CreditTransaction
	database.DB.Order("created_at DESC").Limit(50).Find(&recentTx)

	// Stats by type
	type TypeStat struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
		Total int64  `json:"total"`
	}
	var typeStats []TypeStat
	database.DB.Model(&model.CreditTransaction{}).
		Select("type, COUNT(*) as count, COALESCE(SUM(amount),0) as total").
		Group("type").
		Scan(&typeStats)

	// HP distribution
	type HPBucket struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	hpBuckets := []HPBucket{}
	rows, _ := database.DB.Model(&model.CreditAccount{}).
		Select(`CASE 
			WHEN balance/10000 > 1000 THEN 'full'
			WHEN balance/10000 > 100 THEN 'healthy'
			WHEN balance/10000 > 10 THEN 'low'
			WHEN balance > 0 THEN 'critical'
			ELSE 'hibernated'
		END as status, COUNT(*) as count`).
		Group("status").Rows()
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var b HPBucket
			rows.Scan(&b.Status, &b.Count)
			hpBuckets = append(hpBuckets, b)
		}
	}

	c.JSON(200, gin.H{
		"top_accounts":    topAccounts,
		"recent_tx":       recentTx,
		"type_stats":      typeStats,
		"hp_distribution": hpBuckets,
	})
}

// MetricsQuery proxies a Prometheus instant query
func (h *OverseerHandler) MetricsQuery(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(400, gin.H{"error": "query parameter required"})
		return
	}
	url := fmt.Sprintf("%s/api/v1/query?query=%s&time=%s", h.promURL, query, c.Query("time"))
	h.proxyGet(c, url)
}

// MetricsQueryRange proxies a Prometheus range query
func (h *OverseerHandler) MetricsQueryRange(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(400, gin.H{"error": "query parameter required"})
		return
	}
	url := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%s&end=%s&step=%s",
		h.promURL, query, c.Query("start"), c.Query("end"), c.DefaultQuery("step", "60"))
	h.proxyGet(c, url)
}

// Alerts returns active alerts from Prometheus
func (h *OverseerHandler) Alerts(c *gin.Context) {
	h.proxyGet(c, h.promURL+"/api/v1/alerts")
}

// proxyGet fetches a URL and forwards the JSON response
func (h *OverseerHandler) proxyGet(c *gin.Context, url string) {
	resp, err := h.httpC.Get(url)
	if err != nil {
		c.JSON(502, gin.H{"error": fmt.Sprintf("upstream error: %v", err)})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", body)
}
