package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"starclaw.net/queen/api/internal/database"
	"starclaw.net/queen/api/internal/middleware"
	"starclaw.net/queen/api/internal/model"
)

type PartnerHandler struct{}

// ── Middleware: TeamPartnerRequired ──

func TeamPartnerRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		role := c.GetString("role")

		if role == "admin" {
			c.Next()
			return
		}

		var partner model.TeamPartner
		if err := database.DB.Where("user_id = ? AND status = ?", userID, "active").First(&partner).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, middleware.CodeForbidden, "not a core partner")
			c.Abort()
			return
		}

		c.Set("partner_id", partner.ID)
		c.Next()
	}
}

// ── Dashboard ──

func (h *PartnerHandler) Dashboard(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.TeamPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	month := time.Now().Format("2006-01")

	// Monthly commission
	var monthComm int64
	database.DB.Model(&model.PartnerCommission{}).
		Where("partner_id = ? AND month = ? AND status IN ?", partnerID, month, []string{"approved", "paid"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&monthComm)

	// Pipeline funnel
	type stageStat struct {
		Stage string `json:"stage"`
		Count int64  `json:"count"`
		Value int64  `json:"value"`
	}
	var funnel []stageStat
	database.DB.Model(&model.CRMDeal{}).
		Where("partner_id = ?", partnerID).
		Select("stage, COUNT(*) as count, COALESCE(SUM(deal_value), 0) as value").
		Group("stage").Find(&funnel)

	// Deals needing attention (next_date <= today)
	var urgentCount int64
	database.DB.Model(&model.CRMDeal{}).
		Where("partner_id = ? AND next_date <= ? AND stage NOT IN ?", partnerID, time.Now(), []string{"churned"}).
		Count(&urgentCount)

	// Managed city partners
	var cityCount int64
	database.DB.Model(&model.CityPartner{}).Where("city IN (SELECT region FROM core_partners WHERE id = ?)", partnerID).Count(&cityCount)

	// Equity info
	var equity model.EquityGrant
	database.DB.Where("partner_id = ? AND status = ?", partnerID, "active").First(&equity)

	c.JSON(http.StatusOK, gin.H{
		"partner":          partner,
		"month":            month,
		"month_commission": monthComm,
		"funnel":           funnel,
		"urgent_actions":   urgentCount,
		"city_partners":    cityCount,
		"equity":           equity,
	})
}

// ── CRM Deals ──

func (h *PartnerHandler) ListDeals(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var deals []model.CRMDeal
	query := database.DB.Where("partner_id = ?", partnerID)

	if stage := c.Query("stage"); stage != "" {
		query = query.Where("stage = ?", stage)
	}
	if priority := c.Query("priority"); priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if search := c.Query("q"); search != "" {
		query = query.Where("company_name LIKE ? OR contact_name LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Order("updated_at DESC").Limit(500).Find(&deals)

	c.JSON(http.StatusOK, gin.H{"deals": deals})
}

func (h *PartnerHandler) CreateDeal(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var req struct {
		CompanyName string `json:"company_name" binding:"required"`
		ContactName string `json:"contact_name"`
		ContactInfo string `json:"contact_info"`
		Industry    string `json:"industry"`
		DealValue   int64  `json:"deal_value"`
		Plan        string `json:"plan"`
		Source      string `json:"source"`
		Priority    string `json:"priority"`
		Notes       string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	deal := model.CRMDeal{
		ID:          uuid.New().String(),
		PartnerID:   partnerID,
		CompanyName: req.CompanyName,
		ContactName: req.ContactName,
		ContactInfo: req.ContactInfo,
		Industry:    req.Industry,
		Stage:       "lead",
		DealValue:   req.DealValue,
		Plan:        req.Plan,
		Source:      req.Source,
		Priority:    req.Priority,
		Notes:       req.Notes,
	}

	if err := database.DB.Create(&deal).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"deal": deal})
}

func (h *PartnerHandler) UpdateDeal(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	dealID := c.Param("id")

	var deal model.CRMDeal
	if err := database.DB.Where("id = ? AND partner_id = ?", dealID, partnerID).First(&deal).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		CompanyName string `json:"company_name"`
		ContactName string `json:"contact_name"`
		ContactInfo string `json:"contact_info"`
		Industry    string `json:"industry"`
		Stage       string `json:"stage"`
		DealValue   int64  `json:"deal_value"`
		Plan        string `json:"plan"`
		Priority    string `json:"priority"`
		Notes       string `json:"notes"`
		NextAction  string `json:"next_action"`
		NextDate    string `json:"next_date"` // YYYY-MM-DD
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	updates := map[string]interface{}{}
	if req.CompanyName != "" {
		updates["company_name"] = req.CompanyName
	}
	if req.ContactName != "" {
		updates["contact_name"] = req.ContactName
	}
	if req.ContactInfo != "" {
		updates["contact_info"] = req.ContactInfo
	}
	if req.Industry != "" {
		updates["industry"] = req.Industry
	}
	if req.Stage != "" {
		updates["stage"] = req.Stage
		now := time.Now()
		if req.Stage == "signed" && deal.SignedAt == nil {
			updates["signed_at"] = &now
		}
		if req.Stage == "delivery" || req.Stage == "active" {
			if deal.DeliveredAt == nil {
				updates["delivered_at"] = &now
			}
		}
	}
	if req.DealValue > 0 {
		updates["deal_value"] = req.DealValue
	}
	if req.Plan != "" {
		updates["plan"] = req.Plan
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.NextAction != "" {
		updates["next_action"] = req.NextAction
	}
	if req.NextDate != "" {
		if t, err := time.Parse("2006-01-02", req.NextDate); err == nil {
			updates["next_date"] = &t
		}
	}

	database.DB.Model(&deal).Updates(updates)
	database.DB.Where("id = ?", dealID).First(&deal)

	c.JSON(http.StatusOK, gin.H{"deal": deal})
}

func (h *PartnerHandler) GetDeal(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	dealID := c.Param("id")

	var deal model.CRMDeal
	if err := database.DB.Where("id = ? AND partner_id = ?", dealID, partnerID).First(&deal).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deal": deal})
}

// ── City Partner Management ──

func (h *PartnerHandler) ListCityPartners(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.TeamPartner
	database.DB.Where("id = ?", partnerID).First(&partner)

	var cities []model.CityPartner
	query := database.DB
	if partner.Region != "" {
		query = query.Where("city LIKE ?", "%"+partner.Region+"%")
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	query.Order("created_at DESC").Limit(200).Find(&cities)

	c.JSON(http.StatusOK, gin.H{"city_partners": cities})
}

func (h *PartnerHandler) ReviewCityPartner(c *gin.Context) {
	cityPartnerID := c.Param("id")

	var req struct {
		Status   string  `json:"status" binding:"required"`
		CommRate float64 `json:"comm_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	updates := map[string]interface{}{"status": req.Status}
	if req.Status == "approved" {
		now := time.Now()
		updates["approved_at"] = &now

		var cp model.CityPartner
		if err := database.DB.Where("id = ?", cityPartnerID).First(&cp).Error; err == nil {
			database.DB.Model(&model.User{}).Where("id = ?", cp.UserID).Update("role", "city")
		}
	}
	if req.CommRate > 0 {
		updates["comm_rate"] = req.CommRate
	}

	if err := database.DB.Model(&model.CityPartner{}).Where("id = ?", cityPartnerID).Updates(updates).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// ── Commissions (dual-track) ──

func (h *PartnerHandler) ListCommissions(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var commissions []model.PartnerCommission
	query := database.DB.Where("partner_id = ?", partnerID)
	if month := c.Query("month"); month != "" {
		query = query.Where("month = ?", month)
	}
	if commType := c.Query("type"); commType != "" {
		query = query.Where("type = ?", commType)
	}
	query.Order("created_at DESC").Limit(500).Find(&commissions)

	// Monthly breakdown by type
	type monthType struct {
		Month string `json:"month"`
		Type  string `json:"type"`
		Total int64  `json:"total"`
	}
	var breakdown []monthType
	database.DB.Model(&model.PartnerCommission{}).
		Where("partner_id = ? AND status IN ?", partnerID, []string{"approved", "paid"}).
		Select("month, type, SUM(amount) as total").
		Group("month, type").Order("month DESC").Limit(60).Find(&breakdown)

	c.JSON(http.StatusOK, gin.H{
		"commissions": commissions,
		"breakdown":   breakdown,
	})
}

// ── Equity ──

func (h *PartnerHandler) GetEquity(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var grants []model.EquityGrant
	database.DB.Where("partner_id = ?", partnerID).Order("grant_date DESC").Find(&grants)

	// Calculate vesting progress for active grants
	now := time.Now()
	for i := range grants {
		if grants[i].Status != "active" {
			continue
		}
		if now.Before(grants[i].CliffDate) {
			grants[i].VestedShares = 0
		} else if now.After(grants[i].FullVestDate) {
			grants[i].VestedShares = grants[i].TotalShares
		} else {
			totalDays := grants[i].FullVestDate.Sub(grants[i].GrantDate).Hours() / 24
			elapsedDays := now.Sub(grants[i].GrantDate).Hours() / 24
			grants[i].VestedShares = int64(float64(grants[i].TotalShares) * (elapsedDays / totalDays))
		}
	}

	c.JSON(http.StatusOK, gin.H{"grants": grants})
}

// ── One-Click Deployment ──

func (h *PartnerHandler) ListDeployments(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var deployments []model.Deployment
	database.DB.Where("partner_id = ?", partnerID).Order("created_at DESC").Limit(100).Find(&deployments)

	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

func (h *PartnerHandler) CreateDeployment(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var req struct {
		DealID     string `json:"deal_id"`
		ClientName string `json:"client_name" binding:"required"`
		Type       string `json:"type" binding:"required"` // docker / k8s / cloud
		Region     string `json:"region"`
		Domain     string `json:"domain"`
		AdminEmail string `json:"admin_email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	dep := model.Deployment{
		ID:         uuid.New().String(),
		PartnerID:  partnerID,
		DealID:     req.DealID,
		ClientName: req.ClientName,
		Type:       req.Type,
		Region:     req.Region,
		Domain:     req.Domain,
		AdminEmail: req.AdminEmail,
		Version:    "latest",
		Status:     "pending",
	}

	if err := database.DB.Create(&dep).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	// In production, this would trigger an async provisioning job.
	// For now, simulate by setting to "provisioning".
	go func(id string) {
		time.Sleep(2 * time.Second)
		now := time.Now()
		database.DB.Model(&model.Deployment{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "running",
			"started_at": &now,
			"health_url": fmt.Sprintf("https://%s/health", dep.Domain),
		})
		log.Printf("[deploy] Deployment %s provisioned for %s", id, dep.ClientName)
	}(dep.ID)

	c.JSON(http.StatusCreated, gin.H{"deployment": dep})
}

func (h *PartnerHandler) GetDeployment(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	depID := c.Param("id")

	var dep model.Deployment
	if err := database.DB.Where("id = ? AND partner_id = ?", depID, partnerID).First(&dep).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deployment": dep})
}

func (h *PartnerHandler) StopDeployment(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	depID := c.Param("id")

	var dep model.Deployment
	if err := database.DB.Where("id = ? AND partner_id = ?", depID, partnerID).First(&dep).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	database.DB.Model(&dep).Update("status", "stopped")

	c.JSON(http.StatusOK, gin.H{"message": "deployment stopped"})
}

// ── Admin: manage core partners ──

func (h *PartnerHandler) AdminListPartners(c *gin.Context) {
	var partners []model.TeamPartner
	database.DB.Order("created_at DESC").Find(&partners)
	c.JSON(http.StatusOK, gin.H{"partners": partners})
}

func (h *PartnerHandler) AdminCreatePartner(c *gin.Context) {
	var req struct {
		UserID         string  `json:"user_id"`
		ClawID         string  `json:"claw_id" binding:"required"`
		Name           string  `json:"name" binding:"required"`
		Phone          string  `json:"phone"`
		Email          string  `json:"email"`
		Region         string  `json:"region"`
		Level          string  `json:"level"`
		BaseSalary     int64   `json:"base_salary"`
		DirectCommRate float64 `json:"direct_comm_rate"`
		ManageFeeRate  float64 `json:"manage_fee_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	partner := model.TeamPartner{
		ID:             uuid.New().String(),
		UserID:         req.UserID,
		ClawID:         req.ClawID,
		Name:           req.Name,
		Phone:          req.Phone,
		Email:          req.Email,
		Region:         req.Region,
		Level:          req.Level,
		Status:         "active",
		BaseSalary:     req.BaseSalary,
		DirectCommRate: req.DirectCommRate,
		ManageFeeRate:  req.ManageFeeRate,
		JoinedAt:       time.Now(),
	}
	if partner.Level == "" {
		partner.Level = "partner"
	}
	if partner.DirectCommRate == 0 {
		partner.DirectCommRate = 0.30
	}
	if partner.ManageFeeRate == 0 {
		partner.ManageFeeRate = 0.05
	}

	if err := database.DB.Create(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	// Update user role to "partner" if user_id is provided
	if req.UserID != "" {
		database.DB.Model(&model.User{}).Where("id = ?", req.UserID).Update("role", "partner")
	}

	c.JSON(http.StatusCreated, gin.H{"partner": partner})
}

func (h *PartnerHandler) AdminUpdatePartner(c *gin.Context) {
	partnerID := c.Param("id")

	var req struct {
		Level          string  `json:"level"`
		Status         string  `json:"status"`
		Region         string  `json:"region"`
		BaseSalary     *int64  `json:"base_salary"`
		DirectCommRate float64 `json:"direct_comm_rate"`
		ManageFeeRate  float64 `json:"manage_fee_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	updates := map[string]interface{}{}
	if req.Level != "" {
		updates["level"] = req.Level
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Region != "" {
		updates["region"] = req.Region
	}
	if req.BaseSalary != nil {
		updates["base_salary"] = *req.BaseSalary
	}
	if req.DirectCommRate > 0 {
		updates["direct_comm_rate"] = req.DirectCommRate
	}
	if req.ManageFeeRate > 0 {
		updates["manage_fee_rate"] = req.ManageFeeRate
	}

	if err := database.DB.Model(&model.TeamPartner{}).Where("id = ?", partnerID).Updates(updates).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// AdminDeletePartner deletes a team or city partner
func (h *PartnerHandler) AdminDeletePartner(c *gin.Context) {
	partnerID := c.Param("id")

	// Try team partner first
	if err := database.DB.Where("id = ?", partnerID).Delete(&model.TeamPartner{}).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
		return
	}

	// Try city partner
	if err := database.DB.Where("id = ?", partnerID).Delete(&model.CityPartner{}).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
		return
	}

	middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
}

// AdminSuspendPartner suspends a partner
func (h *PartnerHandler) AdminSuspendPartner(c *gin.Context) {
	partnerID := c.Param("id")
	db := database.DB

	if err := db.Model(&model.TeamPartner{}).Where("id = ?", partnerID).Update("status", "suspended").Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "suspended"})
		return
	}
	if err := db.Model(&model.CityPartner{}).Where("id = ?", partnerID).Update("status", "suspended").Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "suspended"})
		return
	}
	middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
}

// AdminActivatePartner reactivates a suspended partner
func (h *PartnerHandler) AdminActivatePartner(c *gin.Context) {
	partnerID := c.Param("id")
	db := database.DB

	if err := db.Model(&model.TeamPartner{}).Where("id = ?", partnerID).Update("status", "active").Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "activated"})
		return
	}
	if err := db.Model(&model.CityPartner{}).Where("id = ?", partnerID).Update("status", "approved").Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "activated"})
		return
	}
	middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
}

func (h *PartnerHandler) AdminGrantEquity(c *gin.Context) {
	partnerID := c.Param("id")

	var req struct {
		TotalShares   int64   `json:"total_shares" binding:"required"`
		CliffMonths   int     `json:"cliff_months"`
		VestingMonths int     `json:"vesting_months"`
		StrikePrice   float64 `json:"strike_price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	cliff := 12
	if req.CliffMonths > 0 {
		cliff = req.CliffMonths
	}
	vesting := 48
	if req.VestingMonths > 0 {
		vesting = req.VestingMonths
	}

	now := time.Now()
	grant := model.EquityGrant{
		ID:            uuid.New().String(),
		PartnerID:     partnerID,
		TotalShares:   req.TotalShares,
		CliffMonths:   cliff,
		VestingMonths: vesting,
		GrantDate:     now,
		CliffDate:     now.AddDate(0, cliff, 0),
		FullVestDate:  now.AddDate(0, vesting, 0),
		StrikePrice:   req.StrikePrice,
		Status:        "active",
	}

	if err := database.DB.Create(&grant).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"grant": grant})
}

// ── Core Partner: node management (proxy to swarm) ──

func (h *PartnerHandler) ListMyNodes(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	// Collect claw_ids: core partner's own + all city partners under them
	var partner model.TeamPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	clawIDs := []string{}
	if partner.ClawID != "" {
		clawIDs = append(clawIDs, partner.ClawID)
	}

	var cityPartners []model.CityPartner
	database.DB.Where("status = ?", "approved").Find(&cityPartners)
	for _, cp := range cityPartners {
		if cp.ClawID != "" {
			clawIDs = append(clawIDs, cp.ClawID)
		}
	}

	if len(clawIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"nodes": []interface{}{}, "total": 0})
		return
	}

	swarmURL := os.Getenv("SWARM_URL")
	if swarmURL == "" {
		swarmURL = "http://localhost:8090"
	}

	// Build query with claw_ids filter
	joined := strings.Join(clawIDs, ",")
	path := "/swarm/nodes?claw_ids=" + joined
	if status := c.Query("status"); status != "" {
		path += "&status=" + status
	}

	resp, err := http.Get(swarmURL + path)
	if err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "failed to reach swarm")
		return
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "invalid swarm response")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *PartnerHandler) ListNodes(c *gin.Context) {
	swarmURL := os.Getenv("SWARM_URL")
	if swarmURL == "" {
		swarmURL = "http://localhost:8090"
	}

	path := "/swarm/nodes"
	q := c.Request.URL.Query()
	if qs := q.Encode(); qs != "" {
		path += "?" + qs
	}

	resp, err := http.Get(swarmURL + path)
	if err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "failed to reach swarm")
		return
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "invalid swarm response")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *PartnerHandler) GetNode(c *gin.Context) {
	swarmURL := os.Getenv("SWARM_URL")
	if swarmURL == "" {
		swarmURL = "http://localhost:8090"
	}

	id := c.Param("id")
	resp, err := http.Get(swarmURL + fmt.Sprintf("/swarm/nodes/%s", id))
	if err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "failed to reach swarm")
		return
	}
	defer resp.Body.Close()

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		middleware.Fail(c, http.StatusBadGateway, middleware.CodeInternal, "invalid swarm response")
		return
	}
	c.JSON(http.StatusOK, data)
}

// ── Core Partner: manage city partner Claw whitelist ──

func (h *PartnerHandler) AddCityPartnerClaw(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var req struct {
		ClawID   string  `json:"claw_id" binding:"required"`
		Name     string  `json:"name" binding:"required"`
		Company  string  `json:"company"`
		City     string  `json:"city"`
		Phone    string  `json:"phone"`
		Email    string  `json:"email"`
		CommRate float64 `json:"comm_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	// Verify this core partner's region
	var partner model.TeamPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	commRate := req.CommRate
	if commRate == 0 {
		commRate = 0.20
	}

	refCode := fmt.Sprintf("city_%s", uuid.New().String()[:8])

	cityPartner := model.CityPartner{
		ID:            uuid.New().String(),
		ClawID:        req.ClawID,
		Name:          req.Name,
		Company:       req.Company,
		City:          req.City,
		TeamPartnerID: partnerID,
		Phone:         req.Phone,
		Email:         req.Email,
		RefCode:       refCode,
		CommRate:      commRate,
		Status:        "approved",
	}

	if err := database.DB.Create(&cityPartner).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[partner] Core partner %s added city partner claw %s (%s)", partnerID, req.ClawID, req.Name)
	c.JSON(http.StatusCreated, gin.H{"city_partner": cityPartner})
}

func (h *PartnerHandler) RemoveCityPartnerClaw(c *gin.Context) {
	cityPartnerID := c.Param("id")

	if err := database.DB.Model(&model.CityPartner{}).Where("id = ?", cityPartnerID).
		Update("status", "suspended").Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "city partner suspended"})
}
