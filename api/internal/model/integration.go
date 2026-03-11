package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// IntegrationType represents the messaging platform type
type IntegrationType string

const (
	IntegrationFeishu    IntegrationType = "feishu"
	IntegrationDingtalk  IntegrationType = "dingtalk"
	IntegrationWeCom     IntegrationType = "wecom"
	IntegrationSlack     IntegrationType = "slack"
	IntegrationDiscord   IntegrationType = "discord"
	IntegrationTelegram  IntegrationType = "telegram"
)

// Integration stores credentials and configuration for a messaging platform
type Integration struct {
	ID        string          `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID    string          `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Type      IntegrationType `json:"type" gorm:"type:varchar(30);index;not null"`
	Name      string          `json:"name" gorm:"type:varchar(200);not null"`
	Config    string          `json:"config" gorm:"type:json;not null"`  // JSON: platform-specific credentials
	Enabled   bool            `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt gorm.DeletedAt  `json:"-" gorm:"index"`
}

func (i *Integration) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// FeishuConfig holds Feishu-specific credentials
// Stored as JSON in Integration.Config
//
// For App Bot mode: AppID + AppSecret (full API access)
// For Webhook mode: WebhookURL only (send-only, no auth needed)
type FeishuConfig struct {
	AppID      string `json:"app_id,omitempty"`
	AppSecret  string `json:"app_secret,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"` // Custom Bot webhook URL
}

// DingtalkConfig holds DingTalk-specific credentials
type DingtalkConfig struct {
	AppKey     string `json:"app_key,omitempty"`
	AppSecret  string `json:"app_secret,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
	SignSecret string `json:"sign_secret,omitempty"` // webhook signing secret
}

// SlackConfig holds Slack-specific credentials
type SlackConfig struct {
	BotToken   string `json:"bot_token,omitempty"`   // xoxb-...
	WebhookURL string `json:"webhook_url,omitempty"` // Incoming Webhook URL
}

// DiscordConfig holds Discord-specific credentials
type DiscordConfig struct {
	BotToken   string `json:"bot_token,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// TelegramConfig holds Telegram-specific credentials
type TelegramConfig struct {
	BotToken string `json:"bot_token,omitempty"` // from @BotFather
	ChatID   string `json:"chat_id,omitempty"`   // default chat to send to
}

// WeComConfig holds WeCom (企业微信) credentials
type WeComConfig struct {
	CorpID     string `json:"corp_id,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	Secret     string `json:"secret,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"` // Group robot webhook
}
