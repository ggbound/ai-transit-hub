package models

import (
	"time"
)

// ChatMessage model
type ChatMessage struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionID    uint      `gorm:"index;not null" json:"session_id"`
	Role         string    `gorm:"size:50;not null" json:"role"`
	Content      string    `gorm:"type:text" json:"content"`
	Tokens       int       `gorm:"default:0" json:"tokens"`
	ToolCalls    JSON      `gorm:"type:json" json:"tool_calls,omitempty"`
	ToolCallID   string    `gorm:"size:255" json:"tool_call_id,omitempty"`
	FinishReason string    `gorm:"size:100" json:"finish_reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Associations
	Session ChatSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// TableName specifies the table name
func (ChatMessage) TableName() string {
	return "chat_messages"
}
