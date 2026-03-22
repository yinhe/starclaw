package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yinhe/starclaw-queen/api/internal/database"
	"github.com/yinhe/starclaw-queen/api/internal/middleware"
	"github.com/yinhe/starclaw-queen/api/internal/model"
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

// ── Admin: create team_partner invite ──

func (h *InviteHandler) AdminCreateInvite(c *gin.Context) {
	var req struct {
		Type       string     `json:"type" binding:"required"` // team_partner / city_partner
		Label      string     `json:"label"`
		MaxUses    int        `json:"max_uses"`
		Region     string     `json:"region"`
		CommRate   float64    `json:"comm_rate"`
		Level      string     `json:"level"`
		BaseSalary int64      `json:"base_salary"`
		ExpiresAt  *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	if req.Type != "team_partner" && req.Type != "city_partner" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be team_partner or city_partner"})
		return
	}

	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}

	invite := model.PartnerInvite{
		ID:          uuid.New().String(),
		Code:        generateInviteCode(),
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
		ExpiresAt:   req.ExpiresAt,
		Status:      "active",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[invite] Admin created %s invite: %s (max=%d)", req.Type, invite.Code, req.MaxUses)
	c.JSON(http.StatusCreated, gin.H{"invite": invite})
}

// ── Admin: list all invites ──

func (h *InviteHandler) AdminListInvites(c *gin.Context) {
	var invites []model.PartnerInvite
	q := database.DB.Order("created_at DESC")
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	if s := c.Query("status"); s != "" {
		q = q.Where("status = ?", s)
	}
	q.Limit(200).Find(&invites)
	c.JSON(http.StatusOK, gin.H{"invites": invites, "total": len(invites)})
}

// ── Admin: revoke an invite ──

func (h *InviteHandler) AdminRevokeInvite(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Model(&model.PartnerInvite{}).Where("id = ?", id).
		Update("status", "revoked").Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite revoked"})
}

// ── Admin: list invite usage records ──

func (h *InviteHandler) AdminListInviteUses(c *gin.Context) {
	var uses []model.PartnerInviteUse
	q := database.DB.Order("created_at DESC")
	if inviteID := c.Query("invite_id"); inviteID != "" {
		q = q.Where("invite_id = ?", inviteID)
	}
	q.Limit(200).Find(&uses)
	c.JSON(http.StatusOK, gin.H{"uses": uses, "total": len(uses)})
}

// ── TeamPartner: create city_partner invite ──

func (h *InviteHandler) PartnerCreateInvite(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var partner model.TeamPartner
	if err := database.DB.Where("id = ?", partnerID).First(&partner).Error; err != nil {
		middleware.Fail(c, http.StatusNotFound, middleware.CodeNotFound)
		return
	}

	var req struct {
		Label     string     `json:"label"`
		MaxUses   int        `json:"max_uses"`
		Region    string     `json:"region"`
		CommRate  float64    `json:"comm_rate"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, http.StatusBadRequest, middleware.CodeBadRequest)
		return
	}

	if req.MaxUses <= 0 {
		req.MaxUses = 1
	}

	invite := model.PartnerInvite{
		ID:          uuid.New().String(),
		Code:        generateInviteCode(),
		Type:        "city_partner",
		CreatorID:   partnerID,
		CreatorType: "team_partner",
		CreatorName: partner.Name,
		Label:       req.Label,
		MaxUses:     req.MaxUses,
		Region:      req.Region,
		CommRate:    req.CommRate,
		ExpiresAt:   req.ExpiresAt,
		Status:      "active",
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		middleware.Fail(c, http.StatusInternalServerError, middleware.CodeInternal)
		return
	}

	log.Printf("[invite] TeamPartner %s created city invite: %s", partner.Name, invite.Code)
	c.JSON(http.StatusCreated, gin.H{"invite": invite})
}

// ── TeamPartner: list own invites ──

func (h *InviteHandler) PartnerListInvites(c *gin.Context) {
	partnerID := c.GetString("partner_id")

	var invites []model.PartnerInvite
	database.DB.Where("creator_id = ?", partnerID).Order("created_at DESC").Limit(100).Find(&invites)
	c.JSON(http.StatusOK, gin.H{"invites": invites, "total": len(invites)})
}

// ── TeamPartner: revoke own invite ──

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

// ── Public: verify invite code (no auth required) ──

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

	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"type":         invite.Type,
		"creator_name": invite.CreatorName,
		"region":       invite.Region,
		"remaining":    invite.MaxUses - invite.UsedCount,
	})
}

// ── Core logic: validate + consume invite code ──

// ValidateInviteCode checks if an invite code is valid and usable.
func ValidateInviteCode(code string) (*model.PartnerInvite, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	var invite model.PartnerInvite
	if err := database.DB.Where("code = ? AND status = ?", code, "active").First(&invite).Error; err != nil {
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

	switch invite.Type {
	case "team_partner":
		// Check if this claw_id already has a TeamPartner
		var existing model.TeamPartner
		if err := db.Where("claw_id = ?", clawID).First(&existing).Error; err == nil {
			return existing.ID, "team_partner", nil // already a partner
		}

		level := invite.Level
		if level == "" {
			level = "overlord"
		}
		commRate := invite.CommRate
		if commRate == 0 {
			commRate = 0.30
		}

		partner := model.TeamPartner{
			ID:             uuid.New().String(),
			UserID:         user.ID,
			ClawID:         clawID,
			Name:           user.Nickname,
			Email:          user.Email,
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

		// Upgrade user role
		if user.Role != "partner" && user.Role != "admin" {
			db.Model(user).Update("role", "partner")
			user.Role = "partner"
		}

		partnerID = partner.ID
		partnerType = "team_partner"
		log.Printf("[invite] Created TeamPartner %s via invite %s for claw %s", partner.ID, invite.Code, clawID)

	case "city_partner":
		// Check if this claw_id already has a CityPartner
		var existing model.CityPartner
		if err := db.Where("claw_id = ?", clawID).First(&existing).Error; err == nil {
			return existing.ID, "city_partner", nil
		}

		commRate := invite.CommRate
		if commRate == 0 {
			commRate = 0.20
		}
		refCode := fmt.Sprintf("city_%s", uuid.New().String()[:8])

		// Link to the creating TeamPartner
		teamPartnerID := ""
		if invite.CreatorType == "team_partner" {
			teamPartnerID = invite.CreatorID
		}

		partner := model.CityPartner{
			ID:            uuid.New().String(),
			UserID:        user.ID,
			ClawID:        clawID,
			Name:          user.Nickname,
			Email:         user.Email,
			City:          invite.Region,
			TeamPartnerID: teamPartnerID,
			RefCode:       refCode,
			CommRate:      commRate,
			Status:        "approved", // auto-approved via invite
		}
		now := time.Now()
		partner.ApprovedAt = &now

		if err := db.Create(&partner).Error; err != nil {
			return "", "", fmt.Errorf("创建城市合伙人失败: %v", err)
		}

		// Upgrade user role
		if user.Role != "city" && user.Role != "partner" && user.Role != "admin" {
			db.Model(user).Update("role", "city")
			user.Role = "city"
		}

		// Update TeamPartner's managed cities count
		if teamPartnerID != "" {
			db.Model(&model.TeamPartner{}).Where("id = ?", teamPartnerID).
				UpdateColumn("managed_cities", gorm.Expr("managed_cities + 1"))
		}

		partnerID = partner.ID
		partnerType = "city_partner"
		log.Printf("[invite] Created CityPartner %s via invite %s for claw %s (team=%s)", partner.ID, invite.Code, clawID, teamPartnerID)

	default:
		return "", "", fmt.Errorf("未知邀请类型: %s", invite.Type)
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
