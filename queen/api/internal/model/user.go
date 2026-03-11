package model

import "time"

type User struct {
	ID            string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Email         string    `json:"email" gorm:"type:varchar(255);uniqueIndex"`
	Phone         string    `json:"phone" gorm:"type:varchar(20);index"`
	Nickname      string    `json:"nickname" gorm:"type:varchar(100)"`
	Password      string    `json:"-" gorm:"type:varchar(255)"`
	Avatar        string    `json:"avatar" gorm:"type:varchar(500)"`
	Bio           string    `json:"bio" gorm:"type:text"`
	Role          string    `json:"role" gorm:"type:varchar(20);default:user"`     // user / developer / admin
	Status        string    `json:"status" gorm:"type:varchar(20);default:active"` // active / banned
	OAuthProvider string    `json:"oauth_provider" gorm:"type:varchar(20)"`        // google / github
	OAuthID       string    `json:"oauth_id" gorm:"type:varchar(255);index"`       // provider user id
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SMSCode struct {
	ID        uint   `gorm:"primaryKey"`
	Phone     string `gorm:"type:varchar(20);index"`
	Code      string `gorm:"type:varchar(6)"`
	Used      bool   `gorm:"default:false"`
	ExpiresAt time.Time
	CreatedAt time.Time
}
