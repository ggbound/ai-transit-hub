package models

import (
	"time"
)

// OperationLog model
type OperationLog struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint      `gorm:"index;not null" json:"user_id"`
	OperationTypeID uint      `gorm:"index;not null" json:"operation_type_id"`
	ResourceType    string    `gorm:"size:100" json:"resource_type,omitempty"`
	ResourceID      uint      `gorm:"index" json:"resource_id,omitempty"`
	Action          string    `gorm:"size:100" json:"action,omitempty"`
	Details         JSON      `gorm:"type:json" json:"details,omitempty"`
	IpAddress       string    `gorm:"size:100" json:"ip_address,omitempty"`
	UserAgent       string    `gorm:"size:500" json:"user_agent,omitempty"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (OperationLog) TableName() string {
	return "operation_logs"
}
