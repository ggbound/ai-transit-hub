package models

import (
	"time"
)

// OperationType model
type OperationType struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"size:255" json:"display_name,omitempty"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName specifies the table name
func (OperationType) TableName() string {
	return "operation_types"
}
