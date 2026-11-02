package models

import (
	"time"
)

// ApiTestDetail model (修正后匹配 PHP 原项目)
type ApiTestDetail struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	RequestID      uint       `gorm:"index;not null" json:"request_id"`
	RequestHeaders string     `gorm:"type:text" json:"request_headers,omitempty"`
	RequestBody    string     `gorm:"type:text" json:"request_body,omitempty"`
	ResponseHeaders string     `gorm:"type:text" json:"response_headers,omitempty"`
	ResponseBody   string     `gorm:"type:text" json:"response_body,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Associations
	Request ApiTestRequest `gorm:"foreignKey:RequestID" json:"request,omitempty"`
}

// TableName specifies the table name
func (ApiTestDetail) TableName() string {
	return "api_test_details"
}
