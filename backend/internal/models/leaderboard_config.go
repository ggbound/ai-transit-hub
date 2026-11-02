package models

import (
	"time"
)

// LeaderboardConfig model (matches PHP backend)
type LeaderboardConfig struct {
	ID          uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string             `gorm:"size:255;not null" json:"name"`
	DisplayName string             `gorm:"size:255;not null" json:"display_name"`
	Metric      string             `gorm:"size:50;not null" json:"metric"` // total_tokens, total_cost, request_count
	Period      string             `gorm:"size:20;not null" json:"period"` // daily, weekly, monthly
	Limit       int                `gorm:"default:10" json:"limit"`
	Enabled     bool               `gorm:"default:true" json:"enabled"`
	Description string             `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`

	// Associations
	Snapshots []LeaderboardSnapshot `json:"snapshots,omitempty"`
}

// TableName specifies the table name
func (LeaderboardConfig) TableName() string {
	return "leaderboard_configs"
}
