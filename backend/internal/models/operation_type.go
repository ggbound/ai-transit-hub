package models

import (
	"time"
)

// OperationType model - 严格匹配数据库 operation_types 表结构
type OperationType struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TypeCode    string    `gorm:"size:191;uniqueIndex;not null" json:"type_code"`
	TypeName    string    `gorm:"size:191;not null" json:"type_name"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	ModuleName  string    `gorm:"size:191;not null" json:"module_name"`
	IsSensitive bool      `gorm:"default:false" json:"is_sensitive"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TableName specifies the table name
func (OperationType) TableName() string {
	return "operation_types"
}
