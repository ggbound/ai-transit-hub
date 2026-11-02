package models

import (
	"database/sql/driver"
	"encoding/json"
	"strings"
	"time"
)

// AiProvider model
type AiProvider struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code             string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name             string    `gorm:"size:255;not null" json:"name"`
	AdapterClass     string    `gorm:"size:255" json:"adapter_class"`
	BaseUrl          string    `gorm:"size:500" json:"base_url"`
	AuthType         string    `gorm:"size:100" json:"auth_type"`
	AuthConfig       JSON      `gorm:"type:json" json:"auth_config"`
	RequestFormat    JSON      `gorm:"type:json" json:"request_format"`
	ResponseFormat   JSON      `gorm:"type:json" json:"response_format"`
	ParametersSchema JSON      `gorm:"type:json" json:"parameters_schema"`
	StreamSupport    bool      `gorm:"default:false" json:"stream_support"`
	ToolSupport      bool      `gorm:"default:false" json:"tool_support"`
	DocUrl           string    `gorm:"size:500" json:"doc_url"`
	Status           int       `gorm:"default:1" json:"status"`
	SortOrder        int       `gorm:"default:0" json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Associations - 保留关联关系但不会自动查询
	Models  []AiModel        `gorm:"foreignKey:ProviderID" json:"-"`
	ApiKeys []ProviderApiKey `gorm:"foreignKey:AiProviderID" json:"-"`
}

// TableName specifies the table name
func (AiProvider) TableName() string {
	return "ai_providers"
}

// JSON type for JSON columns
type JSON json.RawMessage

// MarshalJSON implements json.Marshaler interface
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("[]"), nil
	}
	if string(j) == "null" {
		return []byte("[]"), nil
	}
	// 验证是否为有效的 JSON
	var test interface{}
	if err := json.Unmarshal(j, &test); err != nil {
		// 如果不是有效的 JSON，返回空数组
		return []byte("[]"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON implements json.Unmarshaler interface
func (j *JSON) UnmarshalJSON(data []byte) error {
	*j = JSON(data)
	return nil
}

// Value implements driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("[]")
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	}

	if len(bytes) == 0 {
		*j = JSON("[]")
		return nil
	}

	// 处理各种 null 格式（小写、大写、混合）
	str := strings.TrimSpace(string(bytes))
	if strings.EqualFold(str, "null") {
		*j = JSON("[]")
		return nil
	}

	// 必须复制数据，避免引用底层驱动的内存
	*j = JSON(append([]byte(nil), bytes...))
	return nil
}

// GetFullUrl gets full URL with endpoint
func (p *AiProvider) GetFullUrl(endpoint string) string {
	if endpoint == "" {
		return p.BaseUrl
	}
	if p.BaseUrl[len(p.BaseUrl)-1] == '/' {
		if endpoint[0] == '/' {
			return p.BaseUrl + endpoint[1:]
		}
		return p.BaseUrl + endpoint
	}
	if endpoint[0] == '/' {
		return p.BaseUrl + endpoint
	}
	return p.BaseUrl + "/" + endpoint
}
