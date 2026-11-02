package models

import (
	"time"
)

// Skill 表示一个 SKILL.md 技能包（管理后台独立模块）
// SKILL.md 遵循 Anthropic Agent Skills 规范：YAML frontmatter + Markdown 正文
type Skill struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:64;uniqueIndex;not null" json:"name"`
	Description  string    `gorm:"size:1024" json:"description,omitempty"`
	Version      string    `gorm:"size:32;default:'0.0.1'" json:"version,omitempty"`
	License      string    `gorm:"size:64" json:"license,omitempty"`
	AllowedTools string    `gorm:"type:text" json:"allowed_tools,omitempty"`
	Content      string    `gorm:"type:mediumtext;not null" json:"content"`
	Frontmatter  JSON      `gorm:"type:json" json:"frontmatter,omitempty"`
	Tags         JSON      `gorm:"type:json" json:"tags,omitempty"`
	FileCount    int       `gorm:"default:0" json:"file_count"`
	StoragePath  string    `gorm:"size:512" json:"storage_path,omitempty"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	CreatedBy    uint      `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Associations
	Files []SkillFile `gorm:"foreignKey:SkillID" json:"files,omitempty"`
}

// TableName specifies the table name
func (Skill) TableName() string {
	return "skills"
}

// SkillFile 技能包内附件文件索引（文件实体存放在磁盘存储目录）
type SkillFile struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SkillID   uint      `gorm:"index;not null" json:"skill_id"`
	Path      string    `gorm:"size:512;not null" json:"path"`
	Size      int64     `gorm:"default:0" json:"size"`
	MimeType  string    `gorm:"size:128" json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name
func (SkillFile) TableName() string {
	return "skill_files"
}
