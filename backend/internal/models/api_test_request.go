package models

import (
	"time"
)

// ApiTestRequest model (修正后匹配 PHP 原项目)
type ApiTestRequest struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	UserApiKeyID   uint       `gorm:"index" json:"user_api_key_id,omitempty"`
	UsageLogID     uint       `gorm:"index" json:"usage_log_id,omitempty"`
	ApiType        string     `gorm:"size:100" json:"api_type,omitempty"`
	Endpoint       string     `gorm:"size:255" json:"endpoint,omitempty"`
	EndpointUrl    string     `gorm:"size:500" json:"endpoint_url,omitempty"`
	Method         string     `gorm:"size:20" json:"method,omitempty"`
	Model          string     `gorm:"size:255" json:"model,omitempty"`
	Type           string     `gorm:"size:50;index:idx_api_test_requests_type_status,priority:1" json:"type,omitempty"`
	ExternalTaskID string     `gorm:"size:255" json:"external_task_id,omitempty"`
	Status         int        `gorm:"index:idx_api_test_requests_type_status,priority:2;default:200" json:"status"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	Duration       int        `gorm:"default:0" json:"duration,omitempty"` // milliseconds
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Associations
	User     User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	UsageLog UsageLog     `gorm:"foreignKey:UsageLogID" json:"usage_log,omitempty"`
}

// TableName specifies the table name
func (ApiTestRequest) TableName() string {
	return "api_test_requests"
}
