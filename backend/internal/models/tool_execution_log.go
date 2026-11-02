package models

import (
	"time"
)

// ToolExecutionLog model
type ToolExecutionLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	SessionID uint      `gorm:"index;not null" json:"session_id"`
	ToolID    uint      `gorm:"index;not null" json:"tool_id"`
	Input     JSON      `gorm:"type:json" json:"input,omitempty"`
	Output    JSON      `gorm:"type:json" json:"output,omitempty"`
	Status    string    `gorm:"size:50" json:"status,omitempty"`
	Error     string    `gorm:"type:text" json:"error,omitempty"`
	Duration  int       `gorm:"default:0" json:"duration"` // milliseconds
	CreatedAt time.Time `json:"created_at"`

	// Associations
	User    User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Session ChatSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Tool    Tool        `gorm:"foreignKey:ToolID" json:"tool,omitempty"`
}

// TableName specifies the table name
func (ToolExecutionLog) TableName() string {
	return "tool_execution_logs"
}
