package models

import (
	"time"
)

// UserApiKey model
type UserApiKey struct {
	ID         uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint       `gorm:"index;not null" json:"user_id"`
	KeyName    string     `gorm:"size:255" json:"key_name,omitempty"`
	ApiKey     string     `gorm:"size:255;uniqueIndex;not null" json:"api_key,omitempty"`
	KeyHash    string     `gorm:"size:255" json:"key_hash,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Status     bool       `gorm:"default:true" json:"status"`
	QuotaLimit int        `gorm:"default:0" json:"quota_limit"`
	RateLimit  int        `gorm:"default:0" json:"rate_limit"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (UserApiKey) TableName() string {
	return "user_api_keys"
}
