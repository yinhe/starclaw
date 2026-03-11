package model

import "time"

// MarketplaceItem is a generic marketplace entry (agent / skill / workflow / mcp)
type MarketplaceItem struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID      string    `json:"user_id" gorm:"type:varchar(36);index"`
	Type        string    `json:"type" gorm:"type:varchar(20);index"` // agent / skill / workflow / mcp
	Name        string    `json:"name" gorm:"type:varchar(200)"`
	Description string    `json:"description" gorm:"type:text"`
	Icon        string    `json:"icon" gorm:"type:varchar(500)"`
	Version     string    `json:"version" gorm:"type:varchar(20);default:1.0.0"`
	Tags        string    `json:"tags" gorm:"type:varchar(500)"`        // comma separated
	Config      string    `json:"config" gorm:"type:longtext"`          // JSON blob
	Status      string    `json:"status" gorm:"type:varchar(20);default:published"` // draft / published / removed
	Downloads   int       `json:"downloads" gorm:"default:0"`
	Rating      float64   `json:"rating" gorm:"default:0"`
	RatingCount int       `json:"rating_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Virtual
	Author *User `json:"author,omitempty" gorm:"foreignKey:UserID"`
}

type MarketplaceReview struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ItemID    string    `json:"item_id" gorm:"type:varchar(36);index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	Rating    int       `json:"rating" gorm:"type:tinyint"`
	Comment   string    `json:"comment" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

// APIKey for developer Open API access
type APIKey struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	UserID    string    `json:"user_id" gorm:"type:varchar(36);index"`
	Name      string    `json:"name" gorm:"type:varchar(100)"`
	Key       string    `json:"key" gorm:"type:varchar(64);uniqueIndex"`
	LastUsed  *time.Time `json:"last_used"`
	CreatedAt time.Time  `json:"created_at"`
}
