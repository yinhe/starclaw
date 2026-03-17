package middleware

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yinhe/starclaw-overlord/api/internal/model"
	"gorm.io/gorm"
)

// LicenseCache caches the active license to avoid DB queries on every request.
type LicenseCache struct {
	mu      sync.RWMutex
	license *model.LicenseKey
	loadedAt time.Time
	ttl     time.Duration
}

var licenseCache = &LicenseCache{ttl: 60 * time.Second}

// GetActiveLicense returns the current active license (cached).
func GetActiveLicense(db *gorm.DB) *model.LicenseKey {
	licenseCache.mu.RLock()
	if licenseCache.license != nil && time.Since(licenseCache.loadedAt) < licenseCache.ttl {
		l := licenseCache.license
		licenseCache.mu.RUnlock()
		return l
	}
	licenseCache.mu.RUnlock()

	// Cache miss — load from DB
	licenseCache.mu.Lock()
	defer licenseCache.mu.Unlock()

	// Double-check after acquiring write lock
	if licenseCache.license != nil && time.Since(licenseCache.loadedAt) < licenseCache.ttl {
		return licenseCache.license
	}

	var lic model.LicenseKey
	if err := db.Where("status = ?", "active").Order("created_at DESC").First(&lic).Error; err != nil {
		// No license found — return nil (community tier)
		licenseCache.license = nil
		licenseCache.loadedAt = time.Now()
		return nil
	}

	// Check expiration
	if lic.ExpiresAt != nil && lic.ExpiresAt.Before(time.Now()) {
		db.Model(&lic).Update("status", "expired")
		licenseCache.license = nil
		licenseCache.loadedAt = time.Now()
		return nil
	}

	licenseCache.license = &lic
	licenseCache.loadedAt = time.Now()
	return &lic
}

// InvalidateLicenseCache forces a reload on next access
func InvalidateLicenseCache() {
	licenseCache.mu.Lock()
	defer licenseCache.mu.Unlock()
	licenseCache.license = nil
	licenseCache.loadedAt = time.Time{}
}

// GetCurrentTier returns the current license tier string
func GetCurrentTier(db *gorm.DB) string {
	lic := GetActiveLicense(db)
	if lic == nil {
		return model.TierCommunity
	}
	return lic.Tier
}

// RequireTier returns middleware that blocks requests if the current license is below the required tier.
func RequireTier(db *gorm.DB, minTier string) gin.HandlerFunc {
	return func(c *gin.Context) {
		current := GetCurrentTier(db)
		if model.TierLevel(current) < model.TierLevel(minTier) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":        "license_insufficient",
				"message":      "此功能需要更高级别的许可证",
				"current_tier": current,
				"required_tier": minTier,
			})
			return
		}
		c.Set("license_tier", current)
		c.Next()
	}
}

// RequireFeature returns middleware that checks if a specific feature is enabled for the current license.
func RequireFeature(db *gorm.DB, featureKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentTier := GetCurrentTier(db)

		var ft model.FeatureToggle
		if err := db.Where("`key` = ?", featureKey).First(&ft).Error; err != nil {
			// Feature not found in DB — allow by default
			c.Set("license_tier", currentTier)
			c.Next()
			return
		}

		// Check manual override
		if !ft.Enabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "feature_disabled",
				"message": ft.Name + " 功能已被管理员关闭",
				"feature": featureKey,
			})
			return
		}

		// Check tier requirement
		if model.TierLevel(currentTier) < model.TierLevel(ft.MinTier) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":         "feature_tier_insufficient",
				"message":       ft.Name + " 需要 " + ft.MinTier + " 或更高许可证",
				"feature":       featureKey,
				"current_tier":  currentTier,
				"required_tier": ft.MinTier,
			})
			return
		}

		c.Set("license_tier", currentTier)
		c.Next()
	}
}

// InjectLicenseInfo is a lightweight middleware that injects license info into context (no blocking).
func InjectLicenseInfo(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lic := GetActiveLicense(db)
		if lic != nil {
			c.Set("license", lic)
			c.Set("license_tier", lic.Tier)
		} else {
			c.Set("license_tier", model.TierCommunity)
		}
		c.Next()
	}
}

// GetLicenseSecret returns the signing secret for license verification
func GetLicenseSecret() string {
	if s := os.Getenv("OVERLORD_LICENSE_SECRET"); s != "" {
		return s
	}
	return "starclaw-overlord-license-secret-default"
}
