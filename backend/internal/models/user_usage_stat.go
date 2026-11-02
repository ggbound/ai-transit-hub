package models

import (
	"time"
)

// UserUsageStat model
type UserUsageStat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	Date         string    `gorm:"size:20;index;not null" json:"date"` // YYYY-MM-DD
	TotalTokens  int       `gorm:"default:0" json:"total_tokens"`
	TotalCost    float64   `gorm:"type:decimal(15,6);default:0" json:"total_cost"`
	RequestCount int       `gorm:"default:0" json:"request_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName specifies the table name
func (UserUsageStat) TableName() string {
	return "user_usage_stats"
}
