package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── Plugin Listing (第三方插件上架) ──

// PluginListing represents a third-party tool plugin in the marketplace.
type PluginListing struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	CreatorID    string    `json:"creator_id" gorm:"type:varchar(36);index;not null"`
	Name         string    `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	DisplayName  string    `json:"display_name" gorm:"type:varchar(200);not null"`
	Description  string    `json:"description" gorm:"type:text"`
	Category     string    `json:"category" gorm:"type:varchar(50);index"` // api, data, productivity, dev, media, finance, social
	Version      string    `json:"version" gorm:"type:varchar(20);default:1.0.0"`
	Icon         string    `json:"icon" gorm:"type:varchar(50)"`
	Readme       string    `json:"readme" gorm:"type:longtext"`
	SpecJSON     string    `json:"spec_json" gorm:"type:json;not null"`       // PluginSpec JSON (same format as example_weather.json)
	Pricing      string    `json:"pricing" gorm:"type:varchar(20);default:free"` // free, paid
	PriceCents   int       `json:"price_cents" gorm:"default:0"`
	Currency     string    `json:"currency" gorm:"type:varchar(10);default:CNY"`
	Status       string    `json:"status" gorm:"type:varchar(20);default:draft;index"` // draft, pending_review, published, suspended
	ReviewNote   string    `json:"review_note" gorm:"type:text"`
	InstallCount int       `json:"install_count" gorm:"default:0"`
	Rating       float64   `json:"rating" gorm:"default:0"`
	RatingCount  int       `json:"rating_count" gorm:"default:0"`
	SalesCount   int       `json:"sales_count" gorm:"default:0"`
	Revenue      int64     `json:"revenue" gorm:"default:0"`
	Featured     bool      `json:"featured" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Creator User `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
}

func (p *PluginListing) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// PluginInstall tracks which users have installed which plugins.
type PluginInstall struct {
	ID        string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	PluginID  string    `json:"plugin_id" gorm:"type:varchar(36);index;not null"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Version   string    `json:"version" gorm:"type:varchar(20)"`
	CreatedAt time.Time `json:"created_at"`

	Plugin PluginListing `json:"plugin,omitempty" gorm:"foreignKey:PluginID"`
}

func (i *PluginInstall) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// PluginRating stores user ratings for plugins.
type PluginRating struct {
	ID       string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	PluginID string    `json:"plugin_id" gorm:"type:varchar(36);index;not null"`
	UserID   string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Score    int       `json:"score" gorm:"not null"` // 1-5
	Comment  string    `json:"comment" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *PluginRating) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// ── API Playground (在线调试记录) ──

// PlaygroundRequest records API playground executions.
type PlaygroundRequest struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	UserID       string    `json:"user_id" gorm:"type:varchar(36);index;not null"`
	Method       string    `json:"method" gorm:"type:varchar(10);not null"`
	Path         string    `json:"path" gorm:"type:varchar(500);not null"`
	RequestBody  string    `json:"request_body" gorm:"type:text"`
	RequestHeaders string  `json:"request_headers" gorm:"type:json"`
	ResponseCode int       `json:"response_code"`
	ResponseBody string    `json:"response_body" gorm:"type:mediumtext"`
	DurationMs   int64     `json:"duration_ms"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}

func (p *PlaygroundRequest) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}
