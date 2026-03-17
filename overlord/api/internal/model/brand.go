package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BrandConfig stores the white-label brand configuration for this Overlord instance.
// Only one active config exists at any time (singleton pattern, keyed by id="default" or first row).
type BrandConfig struct {
	ID             string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	BrandName      string    `json:"brand_name" gorm:"type:varchar(200);not null;default:StarClaw"`
	LogoURL        string    `json:"logo_url" gorm:"type:varchar(500)"`
	FaviconURL     string    `json:"favicon_url" gorm:"type:varchar(500)"`
	PrimaryColor   string    `json:"primary_color" gorm:"type:varchar(20);default:#6d28d9"`   // hex
	SecondaryColor string    `json:"secondary_color" gorm:"type:varchar(20);default:#4f46e5"` // hex
	BgColor        string    `json:"bg_color" gorm:"type:varchar(20);default:#0a0a0a"`
	AccentColor    string    `json:"accent_color" gorm:"type:varchar(20);default:#8b5cf6"`
	Domain         string    `json:"domain" gorm:"type:varchar(255)"`           // custom domain e.g. ai.client.com
	LoginTitle     string    `json:"login_title" gorm:"type:varchar(200)"`      // e.g. "欢迎使用智脑AI"
	LoginSubtitle  string    `json:"login_subtitle" gorm:"type:varchar(500)"`   // login page subtitle
	CopyrightText  string    `json:"copyright_text" gorm:"type:varchar(500)"`   // footer copyright
	ICPNumber      string    `json:"icp_number" gorm:"type:varchar(100)"`       // 粤ICP备XXXXXXX号
	SupportEmail   string    `json:"support_email" gorm:"type:varchar(255)"`    // noreply@your-brand.com
	CustomCSS      string    `json:"custom_css" gorm:"type:text"`               // optional extra CSS
	PoweredBy      bool      `json:"powered_by" gorm:"default:true"`            // show "Powered by StarClaw"
	Enabled        bool      `json:"enabled" gorm:"default:false"`              // white-label active
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (b *BrandConfig) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// FeatureToggle represents a feature module that can be enabled/disabled per license tier.
type FeatureToggle struct {
	ID          string `json:"id" gorm:"type:varchar(36);primaryKey"`
	Key         string `json:"key" gorm:"type:varchar(100);uniqueIndex;not null"` // e.g. "ai_chat", "knowledge_base"
	Name        string `json:"name" gorm:"type:varchar(200);not null"`            // display name
	Description string `json:"description" gorm:"type:varchar(500)"`
	Category    string `json:"category" gorm:"type:varchar(50)"`        // core, advanced, enterprise, whitelabel
	MinTier     string `json:"min_tier" gorm:"type:varchar(20)"`        // minimum license tier: community/starter/pro/enterprise/whitelabel
	Enabled     bool   `json:"enabled" gorm:"default:true"`             // manual override
	SortOrder   int    `json:"sort_order" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (f *FeatureToggle) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

// LicenseTier constants
const (
	TierCommunity  = "community"
	TierStarter    = "starter"
	TierPro        = "pro"
	TierEnterprise = "enterprise"
	TierWhiteLabel = "whitelabel"
)

// TierLevel returns numeric level for comparison (higher = more features)
func TierLevel(tier string) int {
	switch tier {
	case TierCommunity:
		return 0
	case TierStarter:
		return 1
	case TierPro:
		return 2
	case TierEnterprise:
		return 3
	case TierWhiteLabel:
		return 4
	default:
		return 0
	}
}

// TierLimits defines per-tier resource limits
type TierLimits struct {
	MaxNodes      int
	MaxTeams      int
	SSOEnabled    bool
	AuditDays     int  // 0 = permanent
	AdvancedUsage bool
	Compliance    bool
	BrandCustom   bool
	FeatureToggle bool
}

// GetTierLimits returns the limits for a given tier
func GetTierLimits(tier string) TierLimits {
	switch tier {
	case TierStarter:
		return TierLimits{MaxNodes: 20, MaxTeams: 3, AuditDays: 30, AdvancedUsage: false}
	case TierPro:
		return TierLimits{MaxNodes: 100, MaxTeams: 0, SSOEnabled: true, AuditDays: 180, AdvancedUsage: true}
	case TierEnterprise:
		return TierLimits{MaxNodes: 500, MaxTeams: 0, SSOEnabled: true, AuditDays: 0, AdvancedUsage: true, Compliance: true}
	case TierWhiteLabel:
		return TierLimits{MaxNodes: 0, MaxTeams: 0, SSOEnabled: true, AuditDays: 0, AdvancedUsage: true, Compliance: true, BrandCustom: true, FeatureToggle: true}
	default: // community
		return TierLimits{MaxNodes: 10, MaxTeams: 1, AuditDays: 7}
	}
}

// DefaultFeatures returns the seed list of feature toggles
func DefaultFeatures() []FeatureToggle {
	return []FeatureToggle{
		{Key: "ai_chat", Name: "AI 对话", Category: "core", MinTier: TierCommunity, Enabled: true, SortOrder: 1},
		{Key: "agent_market", Name: "Agent 模板市场", Category: "core", MinTier: TierCommunity, Enabled: true, SortOrder: 2},
		{Key: "knowledge_base", Name: "知识库", Category: "core", MinTier: TierStarter, Enabled: true, SortOrder: 3},
		{Key: "image_gen", Name: "图片生成", Category: "advanced", MinTier: TierStarter, Enabled: true, SortOrder: 4},
		{Key: "video_gen", Name: "视频生成", Category: "advanced", MinTier: TierPro, Enabled: true, SortOrder: 5},
		{Key: "tool_call", Name: "工具调用", Category: "advanced", MinTier: TierStarter, Enabled: true, SortOrder: 6},
		{Key: "compute_market", Name: "算力市场", Category: "advanced", MinTier: TierPro, Enabled: false, SortOrder: 7},
		{Key: "community_forum", Name: "社区功能", Category: "advanced", MinTier: TierStarter, Enabled: true, SortOrder: 8},
		{Key: "sso", Name: "企业 SSO", Category: "enterprise", MinTier: TierPro, Enabled: true, SortOrder: 9},
		{Key: "audit_log", Name: "审计日志", Category: "enterprise", MinTier: TierCommunity, Enabled: true, SortOrder: 10},
		{Key: "usage_billing", Name: "用量计费", Category: "enterprise", MinTier: TierStarter, Enabled: true, SortOrder: 11},
		{Key: "compliance", Name: "合规面板", Category: "enterprise", MinTier: TierEnterprise, Enabled: true, SortOrder: 12},
		{Key: "brand_custom", Name: "品牌定制", Category: "whitelabel", MinTier: TierWhiteLabel, Enabled: true, SortOrder: 13},
		{Key: "feature_toggle", Name: "功能开关", Category: "whitelabel", MinTier: TierWhiteLabel, Enabled: true, SortOrder: 14},
	}
}
