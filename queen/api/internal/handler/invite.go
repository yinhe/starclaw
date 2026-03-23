package handler

import (
	"crypto/rand"
	"encoding/hex"
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
	"gorm.io/gorm"
)

type InviteHandler struct{}

// generateInviteCode produces a code like "SC-A3F8-K9M2"
func generateInviteCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	h := strings.ToUpper(hex.EncodeToString(b))
	return fmt.Sprintf("SC-%s-%s", h[:4], h[4:])
}

// siteBaseURL returns the public website base URL for join links.
func siteBaseURL() string {
	if u := os.Getenv("SITE_BASE_URL"); u != "" {
		return u
	}
	return "https://starclaw.net"
}

// inviteResponse enriches an invite with join_url and display_code.
func inviteResponse(invite *model.PartnerInvite) gin.H {
	resp := gin.H{
		"id":           invite.ID,
		"code":         invite.Code,
		"alias":        invite.Alias,
		"display_code": invite.DisplayCode(),
		"type":         invite.Type,
		"creator_id":   invite.CreatorID,
		"creator_type": invite.CreatorType,
		"creator_name": invite.CreatorName,
		"label":        invite.Label,
		"max_uses":     invite.MaxUses,
		"used_count":   invite.UsedCount,
		"region":       invite.Region,
		"comm_rate":    invite.CommRate,
		"level":        invite.Level,
		"base_salary":  invite.BaseSalary,
		"preset_name":  invite.PresetName,
		"preset_phone": invite.PresetPhone,
		"preset_email": invite.PresetEmail,
		"expires_at":   invite.ExpiresAt,
		"status":       invite.Status,
		"join_url":     invite.JoinURL(siteBaseURL()),
		"created_at":   invite.CreatedAt,
		"updated_at":   invite.UpdatedAt,
	}
	return resp
}

// validateAlias ensures alias format: uppercase, alphanumeric + hyphens, 4-50 chars.
func validateAlias(alias string) (string, error) {
	alias = strings.TrimSpace(strings.ToUpper(alias))
	if alias == "" {
		return "", nil
	}
	if len(alias) < 4 || len(alias) > 50 {
		return "", fmt.Errorf("别名长度须在 4-50 字符之间")
	}
	// Check uniqueness
	var count int64
	database.DB.Model(&model.PartnerInvite{}).Where("alias = ?", alias).Count(&count)
	if count > 0 {
		return "", fmt.Errorf("别名 %s 已被使用", alias)
	}
	return alias, nil
}

// ══════════════════════════════════════════════════════════════
// Admin endpoints
// ══════════════════════════════════════════════════════════════

func (h *InviteHandler) AdminCreateInvite(c *gin.Context) {
	var req struct {
		Type        string     `json:"type" binding:"required"` // team_partner / city_partner / referral
		Alias       string     `json:"alias"`
		Label       string     `json:"label"`
		MaxUses     int        `json:"max_uses"`
		Region      string     `json:"region"`
		CommRate    float64    `json:"comm_rate"`
		Level       string     `json:"level"`
		BaseSalary  int64      `json:"base_salary"`
		PresetName  string     `json:"preset_name"`
		PresetPhone string     `json:"preset_phone"`
		PresetEmail string     `json:"preset_email"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	validTypes := map[string]bool{"team_partner": true, "city_partner": true, "referral": true}
	if !validTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be team_partner, city_partner, or referral"})
		return
	}

	alias, err := validateAlias(req.Alias)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}

	invite := model.PartnerInvite{
		ID:          uuid.New().String(),
		Code:        generateInviteCode(),
		Alias:       alias,
		Type:        req.Type,
		CreatorID:   "admin",
		CreatorType: "admin",
		CreatorName: "admin",
		Label:       req.Label,
		MaxUses:     req.MaxUses,
		Region:      req.Region,
		CommRate:    req.CommRate,
		Level:       req.Level,
		BaseSalary:  req.BaseSalary,
		PresetName:  req.PresetName,
		PresetPhone: req.PresetPhone,
		PresetEmail: req.PresetEmail,
		ExpiresAt:   req.ExpiresAt,
		Status:      "active",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[invite] Admin created %s invite: %s alias=%s (max=%d)", req.Type, invite.Code, alias, req.MaxUses)
	c.JSON(http.StatusCreated, gin.H{"invite": inviteResponse(&invite)})
}

func (h *InviteHandler) AdminListInvites(c *gin.Context) {
	var invites []model.PartnerInvite
	q := database.DB.Order("created_at DESC")
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	if creator := c.Query("creator_id"); creator != "" {
		q = q.Where("creator_id = ?", creator)
	}
	q.Limit(200).Find(&invites)

	results := make([]gin.H, len(invites))
	for i := range invites {
		results[i] = inviteResponse(&invites[i])
	}
	c.JSON(http.StatusOK, gin.H{"invites": results, "total": len(results)})
}

func (h *InviteHandler) AdminRevokeInvite(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Model(&model.PartnerInvite{}).Where("id = ?", id).
		Update("status", "revoked").Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite revoked"})
}

func (h *InviteHandler) AdminListInviteUses(c *gin.Context) {
	var uses []model.PartnerInviteUse
	q := database.DB.Order("created_at DESC")
	if inviteID := c.Query("invite_id"); inviteID != "" {
		q = q.Where("invite_id = ?", inviteID)
	}
	if code := c.Query("code"); code != "" {
		q = q.Where("code = ?", code)
	}
	q.Limit(200).Find(&uses)
	c.JSON(http.StatusOK, gin.H{"uses": uses, "total": len(uses)})
}

// ── Admin: invite statistics dashboard ──

func (h *InviteHandler) AdminInviteStats(c *gin.Context) {
	db := database.DB

	// Overall counts
	var totalInvites, activeInvites, totalUses int64
	db.Model(&model.PartnerInvite{}).Count(&totalInvites)
	db.Model(&model.PartnerInvite{}).Where("status = ?", "active").Count(&activeInvites)
	db.Model(&model.PartnerInviteUse{}).Count(&totalUses)

	// By type breakdown
	type typeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var byType []typeCount
	db.Model(&model.PartnerInvite{}).Select("type, COUNT(*) as count").Group("type").Scan(&byType)

	var usesByType []typeCount
	db.Model(&model.PartnerInviteUse{}).Select("type, COUNT(*) as count").Group("type").Scan(&usesByType)

	// Top creators (by usage count)
	type creatorStat struct {
		CreatorID   string `json:"creator_id"`
		CreatorName string `json:"creator_name"`
		CreatorType string `json:"creator_type"`
		Invites     int64  `json:"invites"`
		TotalUsed   int64  `json:"total_used"`
	}
	var topCreators []creatorStat
	db.Model(&model.PartnerInvite{}).Where("used_count > 0").
		Select("creator_id, creator_name, creator_type, COUNT(*) as invites, SUM(used_count) as total_used").
		Group("creator_id, creator_name, creator_type").
		Order("total_used DESC").Limit(20).Scan(&topCreators)

	// 7-day trend
	type dayCount struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var trend []dayCount
	sevenDaysAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	db.Model(&model.PartnerInviteUse{}).Where("created_at >= ?", sevenDaysAgo).
		Select("DATE(created_at) as day, COUNT(*) as count").
		Group("DATE(created_at)").Order("day").Scan(&trend)

	// Conversion rate
	var usedInvites int64
	db.Model(&model.PartnerInvite{}).Where("used_count > 0").Count(&usedInvites)
	conversionRate := float64(0)
	if totalInvites > 0 {
		conversionRate = float64(usedInvites) / float64(totalInvites) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_invites":   totalInvites,
		"active_invites":  activeInvites,
		"total_uses":      totalUses,
		"conversion_rate": fmt.Sprintf("%.1f%%", conversionRate),
		"by_type":         byType,
		"uses_by_type":    usesByType,
		"top_creators":    topCreators,
		"trend_7d":        trend,
	})
}

// ══════════════════════════════════════════════════════════════
// TeamPartner endpoints
// ══════════════════════════════════════════════════════════════

func (h *InviteHandler) PartnerCreateInvite(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.TeamPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		Alias       string     `json:"alias"`
		Label       string     `json:"label"`
		MaxUses     int        `json:"max_uses"`
		Region      string     `json:"region"`
		CommRate    float64    `json:"comm_rate"`
		PresetName  string     `json:"preset_name"`
		PresetPhone string     `json:"preset_phone"`
		PresetEmail string     `json:"preset_email"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	alias, err := validateAlias(req.Alias)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}

	invite := model.PartnerInvite{
		ID:          uuid.New().String(),
		Code:        generateInviteCode(),
		Alias:       alias,
		Type:        "city_partner",
		CreatorID:   partnerID,
		CreatorType: "team_partner",
		CreatorName: partner.Name,
		Label:       req.Label,
		MaxUses:     req.MaxUses,
		Region:      req.Region,
		CommRate:    req.CommRate,
		PresetName:  req.PresetName,
		PresetPhone: req.PresetPhone,
		PresetEmail: req.PresetEmail,
		ExpiresAt:   req.ExpiresAt,
		Status:      "active",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[invite] TeamPartner %s created city invite: %s", partner.Name, invite.DisplayCode())
	c.JSON(http.StatusCreated, gin.H{"invite": inviteResponse(&invite)})
}

func (h *InviteHandler) PartnerListInvites(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var invites []model.PartnerInvite
	database.DB.Where("creator_id = ?", partnerID).Order("created_at DESC").Limit(100).Find(&invites)

	results := make([]gin.H, len(invites))
	for i := range invites {
		results[i] = inviteResponse(&invites[i])
	}
	c.JSON(http.StatusOK, gin.H{"invites": results, "total": len(results)})
}

func (h *InviteHandler) PartnerRevokeInvite(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	id := c.Param("id")

	result := database.DB.Model(&model.PartnerInvite{}).
		Where("id = ? AND creator_id = ?", id, partnerID).
		Update("status", "revoked")
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found or not yours"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite revoked"})
}

// ── TeamPartner: own invite stats ──

func (h *InviteHandler) PartnerInviteStats(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	db := database.DB

	var totalInvites, totalUses int64
	db.Model(&model.PartnerInvite{}).Where("creator_id = ?", partnerID).Count(&totalInvites)
	db.Model(&model.PartnerInviteUse{}).Where("invite_id IN (?)",
		db.Model(&model.PartnerInvite{}).Select("id").Where("creator_id = ?", partnerID),
	).Count(&totalUses)

	c.JSON(http.StatusOK, gin.H{
		"total_invites": totalInvites,
		"total_uses":    totalUses,
	})
}

// ══════════════════════════════════════════════════════════════
// CityPartner endpoints (multi-level: city can create referral invites)
// ══════════════════════════════════════════════════════════════

func (h *InviteHandler) CityCreateInvite(c *gin.Context) {
	partnerID := c.GetString("partner_id") // set by CityPartnerRequired middleware

	var partner model.CityPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		Alias       string     `json:"alias"`
		Label       string     `json:"label"`
		MaxUses     int        `json:"max_uses"`
		PresetName  string     `json:"preset_name"`
		PresetPhone string     `json:"preset_phone"`
		PresetEmail string     `json:"preset_email"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	alias, err := validateAlias(req.Alias)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxUses <= 0 {
		req.MaxUses = 0 // unlimited by default for referral invites
	}

	invite := model.PartnerInvite{
		ID:          uuid.New().String(),
		Code:        generateInviteCode(),
		Alias:       alias,
		Type:        "referral",
		CreatorID:   partnerID,
		CreatorType: "city_partner",
		CreatorName: partner.Name,
		Label:       req.Label,
		MaxUses:     req.MaxUses,
		Region:      partner.City,
		PresetName:  req.PresetName,
		PresetPhone: req.PresetPhone,
		PresetEmail: req.PresetEmail,
		ExpiresAt:   req.ExpiresAt,
		Status:      "active",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[invite] CityPartner %s created referral invite: %s", partner.Name, invite.DisplayCode())
	c.JSON(http.StatusCreated, gin.H{"invite": inviteResponse(&invite)})
}

func (h *InviteHandler) CityListInvites(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var invites []model.PartnerInvite
	database.DB.Where("creator_id = ?", partnerID).Order("created_at DESC").Limit(100).Find(&invites)

	results := make([]gin.H, len(invites))
	for i := range invites {
		results[i] = inviteResponse(&invites[i])
	}
	c.JSON(http.StatusOK, gin.H{"invites": results, "total": len(results)})
}

func (h *InviteHandler) CityRevokeInvite(c *gin.Context) {
	partnerID := c.GetString("partner_id")
	id := c.Param("id")

	result := database.DB.Model(&model.PartnerInvite{}).
		Where("id = ? AND creator_id = ?", id, partnerID).
		Update("status", "revoked")
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "invite not found or not yours"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite revoked"})
}

// ══════════════════════════════════════════════════════════════
// Public endpoints
// ══════════════════════════════════════════════════════════════

func (h *InviteHandler) VerifyInvite(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
		return
	}

	invite, err := ValidateInviteCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
		return
	}

	remaining := 0
	if invite.MaxUses > 0 {
		remaining = invite.MaxUses - invite.UsedCount
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"type":         invite.Type,
		"creator_name": invite.CreatorName,
		"region":       invite.Region,
		"remaining":    remaining,
		"unlimited":    invite.MaxUses == 0,
		"join_url":     invite.JoinURL(siteBaseURL()),
	})
}

// ══════════════════════════════════════════════════════════════
// Core logic: validate + consume
// ══════════════════════════════════════════════════════════════

// ValidateInviteCode checks if an invite code (or alias) is valid and usable.
func ValidateInviteCode(code string) (*model.PartnerInvite, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	var invite model.PartnerInvite
	// Match by code OR alias
	if err := database.DB.Where("(code = ? OR alias = ?) AND status = ?", code, code, "active").
		First(&invite).Error; err != nil {
		return nil, fmt.Errorf("邀请码无效或已停用")
	}
	if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
		database.DB.Model(&invite).Update("status", "expired")
		return nil, fmt.Errorf("邀请码已过期")
	}
	if invite.MaxUses > 0 && invite.UsedCount >= invite.MaxUses {
		return nil, fmt.Errorf("邀请码已用完")
	}
	return &invite, nil
}

// ConsumeInviteCode uses an invite code to create a partner record.
// Called from ClawRegister when invite_code is present.
// Returns the created partner ID and type, or error.
func ConsumeInviteCode(invite *model.PartnerInvite, clawID string, user *model.User) (partnerID, partnerType string, err error) {
	db := database.DB

	// Apply preset info to user if available
	presetUpdates := map[string]interface{}{}
	if invite.PresetName != "" && user.Nickname == "" {
		presetUpdates["nickname"] = invite.PresetName
		user.Nickname = invite.PresetName
	}
	if invite.PresetEmail != "" && (user.Email == "" || strings.HasSuffix(user.Email, "@claw.local")) {
		presetUpdates["email"] = invite.PresetEmail
		user.Email = invite.PresetEmail
	}
	if invite.PresetPhone != "" && user.Phone == "" {
		presetUpdates["phone"] = invite.PresetPhone
		user.Phone = invite.PresetPhone
	}
	if len(presetUpdates) > 0 {
		db.Model(user).Updates(presetUpdates)
	}

	switch invite.Type {
	case "team_partner":
		partnerID, partnerType, err = consumeTeamPartner(db, invite, clawID, user)
	case "city_partner":
		partnerID, partnerType, err = consumeCityPartner(db, invite, clawID, user)
	case "referral":
		partnerID, partnerType, err = consumeReferral(db, invite, clawID, user)
	default:
		return "", "", fmt.Errorf("未知邀请类型: %s", invite.Type)
	}
	if err != nil {
		return "", "", err
	}

	// Record usage
	use := model.PartnerInviteUse{
		ID:        uuid.New().String(),
		InviteID:  invite.ID,
		Code:      invite.Code,
		ClawID:    clawID,
		UserID:    user.ID,
		PartnerID: partnerID,
		Type:      partnerType,
	}
	db.Create(&use)
	db.Model(invite).UpdateColumn("used_count", gorm.Expr("used_count + 1"))

	// Auto-expire if max uses reached
	if invite.MaxUses > 0 && invite.UsedCount+1 >= invite.MaxUses {
		db.Model(invite).Update("status", "expired")
	}

	return partnerID, partnerType, nil
}

func consumeTeamPartner(db *gorm.DB, invite *model.PartnerInvite, clawID string, user *model.User) (string, string, error) {
	var existing model.TeamPartner
	if err := db.Where("claw_id = ?", clawID).First(&existing).Error; err == nil {
		return existing.ID, "team_partner", nil
	}

	level := invite.Level
	if level == "" {
		level = "overlord"
	}
	commRate := invite.CommRate
	if commRate == 0 {
		commRate = 0.30
	}

	name := invite.PresetName
	if name == "" {
		name = user.Nickname
	}
	email := invite.PresetEmail
	if email == "" {
		email = user.Email
	}
	phone := invite.PresetPhone

	partner := model.TeamPartner{
		ID:             uuid.New().String(),
		UserID:         user.ID,
		ClawID:         clawID,
		Name:           name,
		Phone:          phone,
		Email:          email,
		Region:         invite.Region,
		Level:          level,
		Status:         "active",
		BaseSalary:     invite.BaseSalary,
		DirectCommRate: commRate,
		ManageFeeRate:  0.05,
		JoinedAt:       time.Now(),
	}
	if err := db.Create(&partner).Error; err != nil {
		return "", "", fmt.Errorf("创建团队合伙人失败: %v", err)
	}

	if user.Role != "partner" && user.Role != "admin" {
		db.Model(user).Update("role", "partner")
		user.Role = "partner"
	}

	log.Printf("[invite] Created TeamPartner %s via invite %s for claw %s", partner.ID, invite.Code, clawID)
	return partner.ID, "team_partner", nil
}

func consumeCityPartner(db *gorm.DB, invite *model.PartnerInvite, clawID string, user *model.User) (string, string, error) {
	var existing model.CityPartner
	if err := db.Where("claw_id = ?", clawID).First(&existing).Error; err == nil {
		return existing.ID, "city_partner", nil
	}

	commRate := invite.CommRate
	if commRate == 0 {
		commRate = 0.20
	}
	refCode := fmt.Sprintf("city_%s", uuid.New().String()[:8])

	teamPartnerID := ""
	if invite.CreatorType == "team_partner" {
		teamPartnerID = invite.CreatorID
	}

	name := invite.PresetName
	if name == "" {
		name = user.Nickname
	}
	email := invite.PresetEmail
	if email == "" {
		email = user.Email
	}

	partner := model.CityPartner{
		ID:            uuid.New().String(),
		UserID:        user.ID,
		ClawID:        clawID,
		Name:          name,
		Email:         email,
		Phone:         invite.PresetPhone,
		City:          invite.Region,
		TeamPartnerID: teamPartnerID,
		RefCode:       refCode,
		CommRate:      commRate,
		Status:        "approved",
	}
	now := time.Now()
	partner.ApprovedAt = &now

	if err := db.Create(&partner).Error; err != nil {
		return "", "", fmt.Errorf("创建城市合伙人失败: %v", err)
	}

	if user.Role != "city" && user.Role != "partner" && user.Role != "admin" {
		db.Model(user).Update("role", "city")
		user.Role = "city"
	}

	if teamPartnerID != "" {
		db.Model(&model.TeamPartner{}).Where("id = ?", teamPartnerID).
			UpdateColumn("managed_cities", gorm.Expr("managed_cities + 1"))
	}

	log.Printf("[invite] Created CityPartner %s via invite %s for claw %s (team=%s)", partner.ID, invite.Code, clawID, teamPartnerID)
	return partner.ID, "city_partner", nil
}

func consumeReferral(db *gorm.DB, invite *model.PartnerInvite, clawID string, user *model.User) (string, string, error) {
	// Referral invite: attribute user as a client of the creating CityPartner
	var cityPartner model.CityPartner
	if err := db.Where("id = ? AND status = ?", invite.CreatorID, "approved").First(&cityPartner).Error; err != nil {
		return "", "", fmt.Errorf("推荐人城市合伙人不存在或已停用")
	}

	// Check if already attributed
	var existingClient model.CityClient
	if err := db.Where("user_id = ?", user.ID).First(&existingClient).Error; err == nil {
		return existingClient.ID, "referral", nil
	}

	client := model.CityClient{
		ID:          uuid.New().String(),
		PartnerID:   cityPartner.ID,
		UserID:      user.ID,
		ClientName:  user.Nickname,
		ContactInfo: user.Email + " " + user.Phone,
		Status:      "lead",
		RefSource:   invite.Code,
	}
	db.Create(&client)
	db.Model(&cityPartner).UpdateColumn("total_clients", gorm.Expr("total_clients + 1"))

	log.Printf("[invite] Referral invite %s → user %s attributed to CityPartner %s", invite.Code, user.ID, cityPartner.ID)
	return client.ID, "referral", nil
}
