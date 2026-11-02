package models

import (
	"time"
)

// OperationLog model - 严格匹配数据库 operation_logs 表结构
type OperationLog struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          *uint     `gorm:"index" json:"user_id,omitempty"` // 可为空（如登录失败时未知用户）
	Username        string    `gorm:"size:191" json:"username,omitempty"`
	OperationType   string    `gorm:"size:191;index;not null" json:"operation_type"` // login/logout/create/update/toggle/delete
	OperationModule string    `gorm:"size:191;not null" json:"operation_module"`     // auth/users/models/billing...
	OperationAction string    `gorm:"size:191;not null" json:"operation_action"`     // 如 "PUT /billing/rates/10"
	IpAddress       string    `gorm:"size:191" json:"ip_address,omitempty"`
	UserAgent       string    `gorm:"size:191" json:"user_agent,omitempty"`
	RequestParams   JSON      `gorm:"type:json" json:"request_params,omitempty"`
	ResponseResult  JSON      `gorm:"type:json" json:"response_result,omitempty"`
	Status          string    `gorm:"size:191;index;default:success" json:"status"` // success/failed
	ErrorMessage    *string   `gorm:"type:text" json:"error_message,omitempty"`
	BeforeData      JSON      `gorm:"type:json" json:"before_data,omitempty"`
	AfterData       JSON      `gorm:"type:json" json:"after_data,omitempty"`
	TraceID         string    `gorm:"size:191" json:"trace_id,omitempty"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"user"`
}

// TableName specifies the table name
func (OperationLog) TableName() string {
	return "operation_logs"
}
