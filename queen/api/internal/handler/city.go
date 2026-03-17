package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
	"gorm.io/gorm"
)

type CityHandler struct{}

// ── Public: Apply to become a city partner ──

func (h *CityHandler) Apply(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name       string `json:"name" binding:"required"`
		Company    string `json:"company"`
		City       string `json:"city" binding:"required"`
		Phone      string `json:"phone" binding:"required"`
		Email      string `json:"email" binding:"required"`
		Experience string `json:"experience"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	// Check if already applied
	var existing model.CityPartner
	if err := database.DB.Where("user_id = ?", userID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"partner": existing, "message": "application already exists"})
		return
	}

	partner := model.CityPartner{
		ID:         uuid.New().String(),
		UserID:     userID,
		Name:       req.Name,
		Company:    req.Company,
		City:       req.City,
		Phone:      req.Phone,
		Email:      req.Email,
		Experience: req.Experience,
		RefCode:    generateRefCode(),
		CommRate:   0.20,
		Status:     "pending",
	}

	if err := database.DB.Create(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"partner": partner})
}

// ── City Partner Dashboard ──

func (h *CityHandler) Dashboard(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.CityPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	// Current month stats
	month := time.Now().Format("2006-01")

	var monthCommission int64
	database.DB.Model(&model.Commission{}).
		Where("partner_id = ? AND month = ? AND status IN ?", partnerID, month, []string{"approved", "paid"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&monthCommission)

	var monthClients int64
	database.DB.Model(&model.CityClient{}).
		Where("partner_id = ? AND created_at >= ?", partnerID, time.Now().Format("2006-01")+"-01").
		Count(&monthClients)

	var totalClients int64
	database.DB.Model(&model.CityClient{}).Where("partner_id = ?", partnerID).Count(&totalClients)

	var activeClients int64
	database.DB.Model(&model.CityClient{}).Where("partner_id = ? AND status = ?", partnerID, "active").Count(&activeClients)

	// Pending commission (not yet paid)
	var pendingCommission int64
	database.DB.Model(&model.Commission{}).
		Where("partner_id = ? AND status IN ?", partnerID, []string{"pending", "approved"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&pendingCommission)

	c.JSON(http.StatusOK, gin.H{
		"partner":            partner,
		"month":              month,
		"month_commission":   monthCommission,
		"month_new_clients":  monthClients,
		"total_clients":      totalClients,
		"active_clients":     activeClients,
		"total_earned":       partner.TotalEarned,
		"pending_commission": pendingCommission,
		"ref_url":            fmt.Sprintf("https://starclaw.me/download?ref=%s", partner.RefCode),
	})
}

// ── Clients ──

func (h *CityHandler) ListClients(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var clients []model.CityClient
	query := database.DB.Where("partner_id = ?", partnerID)

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	query.Order("created_at DESC").Limit(200).Find(&clients)

	c.JSON(http.StatusOK, gin.H{"clients": clients})
}

func (h *CityHandler) AddClient(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var req struct {
		ClientName  string `json:"client_name" binding:"required"`
		ContactInfo string `json:"contact_info"`
		Plan        string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	client := model.CityClient{
		ID:          uuid.New().String(),
		PartnerID:   partnerID,
		ClientName:  req.ClientName,
		ContactInfo: req.ContactInfo,
		Plan:        req.Plan,
		Status:      "lead",
	}

	if err := database.DB.Create(&client).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"client": client})
}

func (h *CityHandler) UpdateClient(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	clientID := c.Param("id")

	var client model.CityClient
	if err := database.DB.Where("id = ? AND partner_id = ?", clientID, partnerID).First(&client).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		ClientName  string `json:"client_name"`
		ContactInfo string `json:"contact_info"`
		Plan        string `json:"plan"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	updates := map[string]interface{}{}
	if req.ClientName != "" {
		updates["client_name"] = req.ClientName
	}
	if req.ContactInfo != "" {
		updates["contact_info"] = req.ContactInfo
	}
	if req.Plan != "" {
		updates["plan"] = req.Plan
	}
	if req.Status != "" {
		updates["status"] = req.Status
		if req.Status == "active" && client.SignedAt == nil {
			now := time.Now()
			updates["signed_at"] = &now
		}
	}

	database.DB.Model(&client).Updates(updates)

	database.DB.Where("id = ?", clientID).First(&client)
	c.JSON(http.StatusOK, gin.H{"client": client})
}

// ── Commissions ──

func (h *CityHandler) ListCommissions(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var commissions []model.Commission
	query := database.DB.Where("partner_id = ?", partnerID)

	if month := c.Query("month"); month != "" {
		query = query.Where("month = ?", month)
	}

	query.Order("created_at DESC").Limit(500).Find(&commissions)

	// Monthly summary
	type monthSummary struct {
		Month string `json:"month"`
		Total int64  `json:"total"`
		Count int64  `json:"count"`
	}
	var summaries []monthSummary
	database.DB.Model(&model.Commission{}).
		Where("partner_id = ? AND status IN ?", partnerID, []string{"approved", "paid"}).
		Select("month, SUM(amount) as total, COUNT(*) as count").
		Group("month").Order("month DESC").Limit(12).
		Find(&summaries)

	c.JSON(http.StatusOK, gin.H{
		"commissions":     commissions,
		"monthly_summary": summaries,
	})
}

// ── Payouts ──

func (h *CityHandler) ListPayouts(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var payouts []model.Payout
	database.DB.Where("partner_id = ?", partnerID).
		Order("created_at DESC").Limit(100).Find(&payouts)

	c.JSON(http.StatusOK, gin.H{"payouts": payouts})
}

// ── Marketing Materials ──

func (h *CityHandler) ListMaterials(c *gin.Context) {
	var materials []model.MarketingMaterial

	query := database.DB
	if cat := c.Query("category"); cat != "" {
		query = query.Where("category = ?", cat)
	}

	query.Order("sort_order ASC, created_at DESC").Find(&materials)

	c.JSON(http.StatusOK, gin.H{"materials": materials})
}

// ── Referral link ──

func (h *CityHandler) RefLink(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.CityPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ref_code": partner.RefCode,
		"ref_url":  fmt.Sprintf("https://starclaw.me/download?ref=%s", partner.RefCode),
		"utm_url":  fmt.Sprintf("https://starclaw.me/download?utm_source=%s&utm_medium=partner&utm_campaign=city", partner.RefCode),
	})
}

// ── Admin: manage city partners ──

func (h *CityHandler) AdminListPartners(c *gin.Context) {
	var partners []model.CityPartner

	query := database.DB
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if city := c.Query("city"); city != "" {
		query = query.Where("city LIKE ?", "%"+city+"%")
	}

	query.Order("created_at DESC").Limit(200).Find(&partners)
	c.JSON(http.StatusOK, gin.H{"partners": partners})
}

func (h *CityHandler) AdminReviewPartner(c *gin.Context) {
	partnerID := c.Param("id")

	var req struct {
		Status   string  `json:"status" binding:"required"` // approved / rejected / suspended
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

		// Update user role to "city"
		var partner model.CityPartner
		if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err == nil {
			database.DB.Model(&model.User{}).Where("id = ?", partner.UserID).Update("role", "city")
		}
	}
	if req.CommRate > 0 {
		updates["comm_rate"] = req.CommRate
	}

	if err := database.DB.Model(&model.CityPartner{}).Where("id = ?", partnerID).Updates(updates).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *CityHandler) AdminListCommissions(c *gin.Context) {
	var commissions []model.Commission

	query := database.DB
	if month := c.Query("month"); month != "" {
		query = query.Where("month = ?", month)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	query.Order("created_at DESC").Limit(500).Find(&commissions)
	c.JSON(http.StatusOK, gin.H{"commissions": commissions})
}

func (h *CityHandler) AdminApproveCommission(c *gin.Context) {
	commID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"` // approved / rejected
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	if err := database.DB.Model(&model.Commission{}).Where("id = ?", commID).
		Update("status", req.Status).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *CityHandler) AdminCreateMaterial(c *gin.Context) {
	var req struct {
		Title       string `json:"title" binding:"required"`
		Category    string `json:"category" binding:"required"`
		Description string `json:"description"`
		FileURL     string `json:"file_url" binding:"required"`
		FileSize    int64  `json:"file_size"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	mat := model.MarketingMaterial{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Category:    req.Category,
		Description: req.Description,
		FileURL:     req.FileURL,
		FileSize:    req.FileSize,
		SortOrder:   req.SortOrder,
	}

	if err := database.DB.Create(&mat).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"material": mat})
}

// ── Middleware: CityPartnerRequired ──

func CityPartnerRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		role := c.GetString("role")

		// Admin can access city routes too
		if role == "admin" {
			c.Next()
			return
		}

		var partner model.CityPartner
		if err := database.DB.Where("user_id = ? AND status = ?", userID, "approved").First(&partner).Error; err != nil {
			middleware.Fail(c, http.StatusForbidden, middleware.CodeForbidden, "not an approved city partner")
			c.Abort()
			return
		}

		c.Set("partner_id", partner.ID)
		c.Next()
	}
}

// ── Commission auto-generation (called from billing.completeOrder) ──

func generateCityCommission(tx *gorm.DB, userID, orderNo string, amount int64) {
	// Find if this user was referred by a city partner
	var client model.CityClient
	if err := tx.Where("contact_info LIKE ?", "%"+userID+"%").First(&client).Error; err != nil {
		// Try by ref_source matching through join: find partner's client whose contact matches user email
		var user model.User
		if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
			return
		}
		searchTerm := user.Email
		if searchTerm == "" {
			searchTerm = user.Phone
		}
		if searchTerm == "" {
			return
		}
		if err := tx.Where("contact_info LIKE ?", "%"+searchTerm+"%").First(&client).Error; err != nil {
			return // user not referred by any partner
		}
	}

	// Get partner and their commission rate
	var partner model.CityPartner
	if err := tx.Where("id = ? AND status = ?", client.PartnerID, "approved").First(&partner).Error; err != nil {
		return
	}

	commAmount := int64(float64(amount) * partner.CommRate)
	if commAmount <= 0 {
		return
	}

	month := time.Now().Format("2006-01")

	comm := model.Commission{
		ID:         uuid.New().String(),
		PartnerID:  partner.ID,
		ClientID:   client.ID,
		OrderNo:    orderNo,
		Type:       "renewal",
		Amount:     commAmount,
		Rate:       partner.CommRate,
		BaseAmount: amount,
		Status:     "pending",
		Month:      month,
	}

	// First order from this client? Mark as signup commission
	var existingComm int64
	tx.Model(&model.Commission{}).Where("partner_id = ? AND client_id = ?", partner.ID, client.ID).Count(&existingComm)
	if existingComm == 0 {
		comm.Type = "signup"
		// Auto-upgrade client status to active on first payment
		tx.Model(&client).Updates(map[string]interface{}{"status": "active", "signed_at": time.Now()})
	}

	tx.Create(&comm)

	// Update partner total earned
	tx.Model(&partner).UpdateColumn("total_earned", gorm.Expr("total_earned + ?", commAmount))

	log.Printf("[city] Commission generated: partner=%s, client=%s, order=%s, amount=%d (rate=%.0f%%)",
		partner.ID, client.ID, orderNo, commAmount, partner.CommRate*100)
}

// ── Helpers ──

func generateRefCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "SC" + hex.EncodeToString(b)
}
