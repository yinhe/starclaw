package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/middleware"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

type BrandHandler struct {
	DB *gorm.DB
}

func NewBrandHandler(db *gorm.DB) *BrandHandler {
	return &BrandHandler{DB: db}
}

// GetBrandConfig returns the current brand configuration (public — no auth required for web rendering)
func (h *BrandHandler) GetBrandConfig(c *gin.Context) {
	var brand model.BrandConfig
	if err := h.DB.First(&brand).Error; err != nil {
		// Return defaults if none configured
		c.JSON(http.StatusOK, gin.H{
			"brand": model.BrandConfig{
				BrandName:    "StarClaw",
				PrimaryColor: "#6d28d9",
				AccentColor:  "#8b5cf6",
				BgColor:      "#0a0a0a",
				PoweredBy:    true,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"brand": brand})
}

// UpdateBrandConfig creates or updates the brand configuration (requires whitelabel tier)
func (h *BrandHandler) UpdateBrandConfig(c *gin.Context) {
	var req model.BrandConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing model.BrandConfig
	if err := h.DB.First(&existing).Error; err != nil {
		// Create new
		if err := h.DB.Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建品牌配置失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"brand": req})
		return
	}

	// Update existing — only non-zero fields
	updates := map[string]interface{}{}
	if req.BrandName != "" {
		updates["brand_name"] = req.BrandName
	}
	if req.LogoURL != "" {
		updates["logo_url"] = req.LogoURL
	}
	if req.FaviconURL != "" {
		updates["favicon_url"] = req.FaviconURL
	}
	if req.PrimaryColor != "" {
		updates["primary_color"] = req.PrimaryColor
	}
	if req.SecondaryColor != "" {
		updates["secondary_color"] = req.SecondaryColor
	}
	if req.BgColor != "" {
		updates["bg_color"] = req.BgColor
	}
	if req.AccentColor != "" {
		updates["accent_color"] = req.AccentColor
	}
	if req.Domain != "" {
		updates["domain"] = req.Domain
	}
	if req.LoginTitle != "" {
		updates["login_title"] = req.LoginTitle
	}
	if req.LoginSubtitle != "" {
		updates["login_subtitle"] = req.LoginSubtitle
	}
	if req.CopyrightText != "" {
		updates["copyright_text"] = req.CopyrightText
	}
	if req.ICPNumber != "" {
		updates["icp_number"] = req.ICPNumber
	}
	if req.SupportEmail != "" {
		updates["support_email"] = req.SupportEmail
	}
	if req.CustomCSS != "" {
		updates["custom_css"] = req.CustomCSS
	}
	updates["powered_by"] = req.PoweredBy
	updates["enabled"] = req.Enabled

	h.DB.Model(&existing).Updates(updates)

	h.DB.First(&existing)
	c.JSON(http.StatusOK, gin.H{"brand": existing})
}

// --- License endpoints ---

// GetLicense returns the current active license
func (h *BrandHandler) GetLicense(c *gin.Context) {
	lic := middleware.GetActiveLicense(h.DB)
	tier := model.TierCommunity
	if lic != nil {
		tier = lic.Tier
	}
	limits := model.GetTierLimits(tier)

	c.JSON(http.StatusOK, gin.H{
		"license": lic,
		"tier":    tier,
		"limits":  limits,
	})
}

// ActivateLicense activates a license key
func (h *BrandHandler) ActivateLicense(c *gin.Context) {
	var req struct {
		Key         string `json:"key" binding:"required"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up the license key
	var lic model.LicenseKey
	if err := h.DB.Where("`key` = ?", req.Key).First(&lic).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "许可证密钥无效"})
		return
	}

	// Verify signature
	secret := middleware.GetLicenseSecret()
	if lic.Signature != "" && !model.VerifyLicenseSignature(&lic, secret) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "许可证签名验证失败"})
		return
	}

	// Check expiration
	if !lic.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "许可证已过期或已吊销"})
		return
	}

	// Deactivate previous licenses
	h.DB.Model(&model.LicenseKey{}).Where("status = ? AND id != ?", "active", lic.ID).Update("status", "superseded")

	// Activate
	updates := map[string]interface{}{
		"status":      "active",
		"fingerprint": req.Fingerprint,
	}
	h.DB.Model(&lic).Updates(updates)

	middleware.InvalidateLicenseCache()

	c.JSON(http.StatusOK, gin.H{
		"license": lic,
		"tier":    lic.Tier,
		"limits":  model.GetTierLimits(lic.Tier),
		"message": "许可证激活成功",
	})
}

// CreateLicense creates a new license key (superadmin/Queen-issued)
func (h *BrandHandler) CreateLicense(c *gin.Context) {
	var req model.LicenseKey
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Tier == "" {
		req.Tier = model.TierCommunity
	}
	if req.Status == "" {
		req.Status = "active"
	}

	// Generate signature
	secret := middleware.GetLicenseSecret()
	req.Signature = model.GenerateLicenseSignature(req.Key, req.Tier, req.Holder, req.MaxNodes, req.MaxTeams, req.ExpiresAt, secret)

	if err := h.DB.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建许可证失败"})
		return
	}

	middleware.InvalidateLicenseCache()
	c.JSON(http.StatusOK, gin.H{"license": req})
}

// RevokeLicense revokes a license
func (h *BrandHandler) RevokeLicense(c *gin.Context) {
	id := c.Param("id")
	var lic model.LicenseKey
	if err := h.DB.First(&lic, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "许可证不存在"})
		return
	}
	h.DB.Model(&lic).Update("status", "revoked")
	middleware.InvalidateLicenseCache()
	c.JSON(http.StatusOK, gin.H{"message": "许可证已吊销"})
}

// --- Feature toggle endpoints ---

// ListFeatures returns all feature toggles
func (h *BrandHandler) ListFeatures(c *gin.Context) {
	var features []model.FeatureToggle
	h.DB.Order("sort_order ASC").Find(&features)

	tier := middleware.GetCurrentTier(h.DB)

	type FeatureWithAccess struct {
		model.FeatureToggle
		HasAccess bool `json:"has_access"`
	}
	result := make([]FeatureWithAccess, len(features))
	for i, f := range features {
		result[i] = FeatureWithAccess{
			FeatureToggle: f,
			HasAccess:     f.Enabled && model.TierLevel(tier) >= model.TierLevel(f.MinTier),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"features":     result,
		"current_tier": tier,
	})
}

// UpdateFeature toggles a feature on/off
func (h *BrandHandler) UpdateFeature(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled *bool  `json:"enabled"`
		MinTier string `json:"min_tier"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ft model.FeatureToggle
	if err := h.DB.First(&ft, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "功能不存在"})
		return
	}

	updates := map[string]interface{}{}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.MinTier != "" {
		updates["min_tier"] = req.MinTier
	}

	h.DB.Model(&ft).Updates(updates)
	h.DB.First(&ft, "id = ?", id)
	c.JSON(http.StatusOK, gin.H{"feature": ft})
}

// SeedFeatures ensures default features exist in the DB
func (h *BrandHandler) SeedFeatures() {
	defaults := model.DefaultFeatures()
	for _, df := range defaults {
		var existing model.FeatureToggle
		if err := h.DB.Where("`key` = ?", df.Key).First(&existing).Error; err != nil {
			h.DB.Create(&df)
		}
	}
}
