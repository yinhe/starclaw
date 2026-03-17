package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SSOProvider stores the configuration for an SSO identity provider
type SSOProvider struct {
	ID           string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TeamID       string    `json:"team_id" gorm:"type:varchar(36);index"`                    // which team this provider serves; empty = global
	Name         string    `json:"name" gorm:"type:varchar(100);not null"`                   // human-readable name
	Type         string    `json:"type" gorm:"type:varchar(20);not null"`                    // oauth2, ldap, oidc
	Provider     string    `json:"provider" gorm:"type:varchar(30)"`                         // github, google, wecom, dingtalk, feishu, custom
	Enabled      bool      `json:"enabled" gorm:"default:true"`
	DefaultRole  string    `json:"default_role" gorm:"type:varchar(20);default:viewer"`      // role assigned to auto-provisioned users

	// OAuth2 / OIDC fields
	ClientID     string    `json:"client_id" gorm:"type:varchar(255)"`
	ClientSecret string    `json:"-" gorm:"type:varchar(255)"`                               // hidden in JSON responses
	AuthURL      string    `json:"auth_url" gorm:"type:varchar(500)"`
	TokenURL     string    `json:"token_url" gorm:"type:varchar(500)"`
	UserInfoURL  string    `json:"userinfo_url" gorm:"type:varchar(500)"`
	RedirectURL  string    `json:"redirect_url" gorm:"type:varchar(500)"`
	Scopes       string    `json:"scopes" gorm:"type:varchar(500);default:openid,profile,email"` // comma-separated

	// LDAP fields
	LDAPHost       string `json:"ldap_host" gorm:"type:varchar(255)"`                       // host:port
	LDAPBaseDN     string `json:"ldap_base_dn" gorm:"type:varchar(500)"`                    // dc=example,dc=com
	LDAPBindDN     string `json:"ldap_bind_dn" gorm:"type:varchar(500)"`                    // cn=admin,dc=example,dc=com
	LDAPBindPass   string `json:"-" gorm:"type:varchar(255)"`                               // hidden
	LDAPUserFilter string `json:"ldap_user_filter" gorm:"type:varchar(500);default:(uid=%s)"` // %s = username
	LDAPUseTLS     bool   `json:"ldap_use_tls" gorm:"default:false"`
	LDAPAttrEmail  string `json:"ldap_attr_email" gorm:"type:varchar(50);default:mail"`
	LDAPAttrName   string `json:"ldap_attr_name" gorm:"type:varchar(50);default:cn"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *SSOProvider) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// SSOSession tracks an SSO login session (state parameter for OAuth2, or LDAP session)
type SSOSession struct {
	ID          string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	ProviderID  string     `json:"provider_id" gorm:"type:varchar(36);index;not null"`
	State       string     `json:"state" gorm:"type:varchar(128);uniqueIndex"`              // OAuth2 state parameter
	ExternalID  string     `json:"external_id" gorm:"type:varchar(255);index"`              // user ID from the IdP
	ExternalName string    `json:"external_name" gorm:"type:varchar(255)"`
	ExternalEmail string   `json:"external_email" gorm:"type:varchar(255)"`
	AdminUserID string     `json:"admin_user_id" gorm:"type:varchar(36);index"`             // linked local AdminUser
	Status      string     `json:"status" gorm:"type:varchar(20);default:pending"`          // pending, completed, failed, expired
	IPAddress   string     `json:"ip_address" gorm:"type:varchar(45)"`
	UserAgent   string     `json:"user_agent" gorm:"type:varchar(500)"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (s *SSOSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}
