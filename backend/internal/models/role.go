package models

import (
	"time"
)

// Role model
type Role struct {
	ID          uint         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string       `gorm:"size:100;uniqueIndex;not null" json:"name"`
	DisplayName string       `gorm:"size:255" json:"display_name,omitempty"`
	Description string       `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`

	// Associations
	Users       []User       `gorm:"many2many:role_user" json:"users,omitempty"`
	Permissions []Permission `gorm:"many2many:permission_role" json:"permissions,omitempty"`
}

// TableName specifies the table name
func (Role) TableName() string {
	return "roles"
}
