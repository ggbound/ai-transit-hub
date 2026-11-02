package models

import (
	"time"
)

// UsageLog model - 严格匹配PHP数据库表结构
type UsageLog struct {
	ID               uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID           uint       `gorm:"index;not null" json:"user_id"`
	ApiKeyID         *uint      `gorm:"index" json:"api_key_id,omitempty"` // 用户API密钥ID - 使用指针可以为null
	ModelID          uint       `gorm:"index;not null" json:"model_id"`
	SessionID        *uint      `gorm:"index" json:"session_id,omitempty"` // 会话ID - 使用指针可以为null
	PromptTokens     int        `gorm:"default:0" json:"prompt_tokens"`
	CompletionTokens int        `gorm:"default:0" json:"completion_tokens"`
	TotalTokens      int        `gorm:"default:0" json:"total_tokens"`
	Cost             float64    `gorm:"type:decimal(10,6);default:0" json:"cost"`
	RequestTime      *time.Time `gorm:"index" json:"request_time,omitempty"`      // 请求时间
	ResponseTime     *time.Time `json:"response_time,omitempty"`                  // 响应时间
	Status           int        `gorm:"index;default:1" json:"status"`            // 0=失败，1=成功，2=处理中
	ErrorMessage     *string    `gorm:"type:text" json:"error_message,omitempty"` // 错误信息 - 使用指针可以为null
	CreatedAt        time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	// Associations
	User  User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Model AiModel `gorm:"foreignKey:ModelID" json:"model,omitempty"`
}

// TableName specifies the table name
func (UsageLog) TableName() string {
	return "usage_logs"
}
