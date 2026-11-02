package models

import (
	"time"
)

// AiModel model
type AiModel struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderID         uint      `gorm:"index:idx_ai_models_provider_code,priority:1;not null" json:"provider_id"`
	Code               string    `gorm:"size:255;not null;index:idx_ai_models_provider_code,priority:2" json:"code"`
	Type               string    `gorm:"size:100" json:"type"`
	Name               string    `gorm:"size:255;not null" json:"name"`
	Description        string    `gorm:"type:text" json:"description"`
	MaxTokens          int       `gorm:"default:4096" json:"max_tokens"`
	Capabilities       JSON      `gorm:"type:json" json:"capabilities"`
	InputPrice         float64   `gorm:"type:decimal(10,6);default:0" json:"input_price"`
	OutputPrice        float64   `gorm:"type:decimal(10,6);default:0" json:"output_price"`
	Parameters         JSON      `gorm:"type:json" json:"parameters"`
	CustomHeaders      JSON      `gorm:"type:json" json:"custom_headers"`
	Config             JSON      `gorm:"type:json" json:"config"`
	CustomEndpoint     string    `gorm:"size:500" json:"custom_endpoint"`
	ChatEndpoint       string    `gorm:"size:500" json:"chat_endpoint"`
	EmbeddingsEndpoint string    `gorm:"size:500" json:"embeddings_endpoint"`
	ImageEndpoint      string    `gorm:"size:500" json:"image_endpoint"`
	VideoEndpoint      string    `gorm:"size:500" json:"video_endpoint"`
	AudioEndpoint      string    `gorm:"size:500" json:"audio_endpoint"`
	SupportsResponse   bool      `gorm:"default:false" json:"supports_response"`
	ResponseEndpoint   string    `gorm:"size:500" json:"response_endpoint"`
	SupportsMessages   bool      `gorm:"default:false" json:"supports_messages"`
	MessagesEndpoint   string    `gorm:"size:500" json:"messages_endpoint"`
	IsPublic           int       `gorm:"default:1" json:"is_public"`
	Status             int       `gorm:"default:1" json:"status"`
	SortOrder          int       `gorm:"default:0" json:"sort_order"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	// Associations
	Provider AiProvider `gorm:"foreignKey:ProviderID" json:"provider"`
}

// TableName specifies the table name
func (AiModel) TableName() string {
	return "ai_models"
}

// GetFullModelId gets full model ID (provider_code/model_code)
func (m *AiModel) GetFullModelId() string {
	return m.Provider.Code + "/" + m.Code
}

// CalculatePrice calculates the price based on tokens
func (m *AiModel) CalculatePrice(inputTokens, outputTokens int) float64 {
	inputCost := (float64(inputTokens) / 1000) * m.InputPrice
	outputCost := (float64(outputTokens) / 1000) * m.OutputPrice
	return inputCost + outputCost
}

// GetDisplayName gets display name (for compatibility)
func (m *AiModel) GetDisplayName() string {
	return m.Name
}
