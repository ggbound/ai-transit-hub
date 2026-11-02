package models

import (
	"time"
)

// Tool model
type Tool struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:255;uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"size:255;not null" json:"display_name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	ToolType    string    `gorm:"size:255;not null" json:"tool_type"`
	Parameters  JSON      `gorm:"type:json" json:"parameters,omitempty"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Associations
	Versions []ToolVersion `json:"versions,omitempty"`
}

// TableName specifies the table name
func (Tool) TableName() string {
	return "tools"
}
