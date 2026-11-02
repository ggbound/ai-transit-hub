package models

import "time"

// AgentTask 虚拟 Agent 任务记录（/v1/agent/tasks 线路）
// 生命周期：pending（已创建待执行）→ running（Agent 循环中）→ completed / partial / failed
// partial：循环耗尽（未给出最终回答）但工作区已有资产产出，资产仍可下载
type AgentTask struct {
	ID            uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID        string     `gorm:"size:64;uniqueIndex;not null" json:"task_id"` // 对外 ID（agt_ 前缀）
	UserID        uint       `gorm:"index;not null" json:"user_id"`
	ApiKeyID      *uint      `gorm:"index" json:"api_key_id,omitempty"`
	ModelID       uint       `gorm:"not null" json:"model_id"`
	ProviderID    uint       `gorm:"not null" json:"provider_id"`
	Task          string     `gorm:"type:text;not null" json:"task"`
	SkillIDs      JSON       `gorm:"type:json" json:"skill_ids,omitempty"`
	MaxIterations int        `gorm:"default:100" json:"max_iterations"`
	Status        string     `gorm:"size:32;index;default:'pending'" json:"status"`
	Iterations    int        `gorm:"default:0" json:"iterations"`
	Result        string     `gorm:"type:mediumtext" json:"result,omitempty"`
	ErrorMessage  string     `gorm:"type:text" json:"error_message,omitempty"`
	SkillsUsed    JSON       `gorm:"type:json" json:"skills_used,omitempty"`
	Assets        JSON       `gorm:"type:json" json:"assets,omitempty"`
	PromptTokens  int        `gorm:"default:0" json:"prompt_tokens"`
	CompletionTok int        `gorm:"column:completion_tokens;default:0" json:"completion_tokens"`
	TotalTokens   int        `gorm:"default:0" json:"total_tokens"`
	Cost          float64    `gorm:"type:decimal(10,6);default:0" json:"cost"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (AgentTask) TableName() string { return "agent_tasks" }

// AgentTaskStatus 任务状态常量
const (
	AgentTaskStatusPending   = "pending"
	AgentTaskStatusRunning   = "running"
	AgentTaskStatusCompleted = "completed"
	AgentTaskStatusPartial   = "partial" // 循环耗尽但有资产产出（可下载，缺最终总结）
	AgentTaskStatusFailed    = "failed"
)

// AgentTaskStep 任务执行步骤明细（工具调用 trace，用于可观测性与排障）
type AgentTaskStep struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID        string    `gorm:"size:64;index;not null" json:"task_id"`
	Seq           int       `json:"seq"`       // 任务内步序（1 起，跨轮累计）
	Iteration     int       `json:"iteration"` // 所属 Agent 循环轮次
	Tool          string    `gorm:"size:64" json:"tool"`
	ArgsExcerpt   string    `gorm:"type:text" json:"args_excerpt,omitempty"`
	OK            bool      `json:"ok"`
	Error         string    `gorm:"type:text" json:"error,omitempty"`
	OutputExcerpt string    `gorm:"type:text" json:"output_excerpt,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// TableName 指定表名
func (AgentTaskStep) TableName() string { return "agent_task_steps" }
