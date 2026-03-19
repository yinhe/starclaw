package billing

import "time"

// ToolUsageRecord tracks every tool execution for billing and analytics.
// Stored locally in Claw's database.
type ToolUsageRecord struct {
	ID               string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID           string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	ConversationID   string    `json:"conversation_id" gorm:"type:varchar(36);index"`
	ClawID           string    `json:"claw_id" gorm:"type:varchar(60);index"`
	ToolName         string    `json:"tool_name" gorm:"type:varchar(50);index;not null"`
	SubType          string    `json:"sub_type" gorm:"type:varchar(50)"`
	ResourceType     string    `json:"resource_type" gorm:"type:varchar(30);index;not null"`
	CostFen          int64     `json:"cost_fen" gorm:"default:0"`           // user pays (分)
	UpstreamFen      int64     `json:"upstream_fen" gorm:"default:0"`       // upstream cost (分)
	MarginFen        int64     `json:"margin_fen" gorm:"default:0"`         // margin = cost - upstream (分)
	CityPartnerID    string    `json:"city_partner_id" gorm:"type:varchar(36);index"`
	CorePartnerID    string    `json:"core_partner_id" gorm:"type:varchar(36);index"`
	CityShareFen     int64     `json:"city_share_fen" gorm:"default:0"`     // city partner share (分)
	CoreShareFen     int64     `json:"core_share_fen" gorm:"default:0"`     // core partner share (分)
	PlatformShareFen int64     `json:"platform_share_fen" gorm:"default:0"` // platform share (分)
	InvestorShareFen int64     `json:"investor_share_fen" gorm:"default:0"` // investor pool share (分)
	DurationMs       int64     `json:"duration_ms" gorm:"default:0"`
	Success          bool      `json:"success" gorm:"default:true"`
	ErrorMsg         string    `json:"error_msg,omitempty" gorm:"type:text"`
	CreatedAt        time.Time `json:"created_at" gorm:"index"`
}
