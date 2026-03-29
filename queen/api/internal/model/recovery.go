package model

import "time"

// PhoneBinding links a Claw node_id to a phone number for recovery verification.
type PhoneBinding struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	NodeID    string    `json:"node_id" gorm:"type:varchar(50);uniqueIndex"`
	Phone     string    `json:"phone" gorm:"type:varchar(20);index"`
	Verified  bool      `json:"verified" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CloudBackup stores encrypted Claw backup blobs (Queen cannot decrypt them).
type CloudBackup struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	LookupKey string    `json:"lookup_key" gorm:"type:varchar(64);uniqueIndex"` // SHA256 hash derived from mnemonic
	NodeID    string    `json:"node_id" gorm:"type:varchar(50);index"`
	Data      []byte    `json:"-" gorm:"type:longblob"`                         // encrypted blob (AES-256-GCM)
	DataSize  int64     `json:"data_size"`
	Version   int       `json:"version" gorm:"default:1"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SMSVerification stores pending SMS verification codes.
type SMSVerification struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Phone     string    `json:"phone" gorm:"type:varchar(20);index"`
	Code      string    `json:"-" gorm:"type:varchar(10)"`
	Purpose   string    `json:"purpose" gorm:"type:varchar(20)"` // "bind_phone", "recovery"
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

func (PhoneBinding) TableName() string    { return "phone_bindings" }
func (CloudBackup) TableName() string     { return "cloud_backups" }
func (SMSVerification) TableName() string { return "sms_verifications" }
