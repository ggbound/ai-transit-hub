package models

import (
	"time"
)

// ToolVersion model
type ToolVersion struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ToolID    uint      `gorm:"index;not null" json:"tool_id"`
	Version   string    `gorm:"size:100;not null" json:"version"`
	Code      string    `gorm:"type:text" json:"code,omitempty"`
	Config    JSON      `gorm:"type:json" json:"config,omitempty"`
	IsActive  bool      `gorm:"default:false" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Associations
	Tool Tool `gorm:"foreignKey:ToolID" json:"tool,omitempty"`
}

// TableName specifies the table name
func (ToolVersion) TableName() string {
	return "tool_versions"
}
