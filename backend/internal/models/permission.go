package models

import (
	"time"
)

// Permission model
type Permission struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DisplayName string    `gorm:"size:255" json:"display_name,omitempty"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Associations
	Roles []Role `gorm:"many2many:permission_role" json:"roles,omitempty"`
}

// TableName specifies the table name
func (Permission) TableName() string {
	return "permissions"
}
