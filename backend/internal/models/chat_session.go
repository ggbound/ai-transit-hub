package models

import (
	"time"
)

// ChatSession model
type ChatSession struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uint      `gorm:"index;not null" json:"user_id"`
	ModelID       uint      `gorm:"index;not null" json:"model_id"`
	Title         string    `gorm:"size:500" json:"title,omitempty"`
	StreamEnabled bool      `gorm:"default:true" json:"stream_enabled"`
	EnabledTools  JSON      `gorm:"type:json" json:"enabled_tools,omitempty"`
	SystemPrompt  string    `gorm:"type:text" json:"system_prompt,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Associations
	User     User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Model    AiModel      `gorm:"foreignKey:ModelID" json:"model,omitempty"`
	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// TableName specifies the table name
func (ChatSession) TableName() string {
	return "chat_sessions"
}
