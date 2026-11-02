package models

import (
	"time"
)

// User model
type User struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	Email      string    `gorm:"size:191;uniqueIndex;not null" json:"email"`
	Password   string    `gorm:"size:255;not null" json:"-"`
	Avatar     string    `gorm:"size:255" json:"avatar,omitempty"`
	Department *string   `gorm:"size:255" json:"department,omitempty"`
	EmployeeID *string   `gorm:"size:255" json:"employee_id,omitempty"`
	Status     uint      `gorm:"default:1;comment:状态:1正常,0禁用" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Associations
	Roles          []Role          `gorm:"many2many:role_user" json:"roles,omitempty"`
	ApiKeys        []UserApiKey    `json:"api_keys,omitempty"`
	ChatSessions   []ChatSession   `json:"chat_sessions,omitempty"`
	UsageLogs      []UsageLog      `json:"usage_logs,omitempty"`
	OperationLogs  []OperationLog  `json:"operation_logs,omitempty"`
	UserUsageStats []UserUsageStat `json:"user_usage_stats,omitempty"`
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// HasRole checks if user has a role
func (u *User) HasRole(roleName string) bool {
	for _, role := range u.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// IsAdmin checks if user is admin
func (u *User) IsAdmin() bool {
	return u.HasRole("admin")
}
