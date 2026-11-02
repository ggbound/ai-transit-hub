package models

import (
	"time"
)

// ProviderApiKey model
type ProviderApiKey struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AiProviderID uint      `gorm:"index;not null" json:"ai_provider_id"`
	ApiKey       string    `gorm:"size:500;not null" json:"api_key"`
	KeyName      string    `gorm:"size:255" json:"key_name"`
	Weight       int       `gorm:"default:1" json:"weight"`
	Status       int       `gorm:"default:1" json:"status"`
	UsageCount   int       `gorm:"default:0" json:"usage_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Associations
	Provider AiProvider `gorm:"foreignKey:AiProviderID" json:"provider"`
}

// TableName specifies the table name
func (ProviderApiKey) TableName() string {
	return "provider_api_keys"
}
