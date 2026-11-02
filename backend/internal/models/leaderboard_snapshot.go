package models

import (
	"time"
)

// LeaderboardSnapshot model
type LeaderboardSnapshot struct {
	ID                  uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	LeaderboardConfigID uint      `gorm:"index;not null" json:"leaderboard_config_id"`
	SnapshotDate        string    `gorm:"size:20;index;not null" json:"snapshot_date"` // YYYY-MM-DD
	Data                JSON      `gorm:"type:json" json:"data,omitempty"`
	CreatedAt           time.Time `json:"created_at"`

	// Associations
	LeaderboardConfig LeaderboardConfig `gorm:"foreignKey:LeaderboardConfigID" json:"leaderboard_config,omitempty"`
}

// TableName specifies the table name
func (LeaderboardSnapshot) TableName() string {
	return "leaderboard_snapshots"
}
