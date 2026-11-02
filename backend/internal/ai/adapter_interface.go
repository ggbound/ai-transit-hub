package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/your-org/ai-transit-hub/internal/models"
)

// UpstreamError 表示上游 AI 提供商返回了非 2xx 状态码。
// 携带上游真实的 HTTP 状态码与错误响应体，调用方应将状态码透传给客户端。
type UpstreamError struct {
	StatusCode int                    // 上游 HTTP 状态码
	Body       map[string]interface{} // 上游返回的 JSON 错误体（可能为 nil）
	Raw        string                 // 上游返回的原始错误体（非 JSON 时保留，已截断）
}

func (e *UpstreamError) Error() string {
	if e.Body != nil {
		if errObj, ok := e.Body["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok {
				return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, msg)
			}
		}
		if msg, ok := e.Body["message"].(string); ok {
			return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, msg)
		}
	}
	if e.Raw != "" {
		return fmt.Sprintf("upstream returned %d: %s", e.StatusCode, e.Raw)
	}
	return fmt.Sprintf("upstream returned status %d", e.StatusCode)
}

// AsUpstreamError 判断 err 是否为上游错误（非 2xx），并提取状态码与响应体
func AsUpstreamError(err error) (*UpstreamError, bool) {
	var ue *UpstreamError
	if errors.As(err, &ue) {
		return ue, true
	}
	return nil, false
}

// ToolCall 模型返回的工具调用（OpenAI 格式）
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 固定 "function"
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc 工具调用函数体
type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// Message 表示聊天消息
type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []ContentPart for multimodal
	// ToolCalls assistant 消息携带的工具调用请求（Agent 工具环用）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID role=tool 消息对应的工具调用 ID
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ContentPart 多模态消息内容部分
type ContentPart struct {
	Type     string `json:"type"` // "text", "image_url"
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
}

// StreamCallback 流式响应回调函数
type StreamCallback func(data map[string]interface{})

// Adapter AI模型适配器接口
// 每个厂商的适配器需要实现此接口，处理厂商特定的请求/响应格式转换
type Adapter interface {
	// Chat 发送聊天请求（非流式）
	// messages: 聊天消息列表
	// options: 请求选项（model, temperature, max_tokens等）
	// 返回标准OpenAI格式的响应
	Chat(ctx context.Context, messages []Message, options map[string]interface{}) (map[string]interface{}, error)

	// ChatStream 发送聊天请求（流式）
	// callback: 每收到一个数据块时调用
	ChatStream(ctx context.Context, messages []Message, options map[string]interface{}, callback StreamCallback) error

	// Embeddings 获取文本嵌入向量
	// inputs: 输入文本列表
	// 返回标准OpenAI格式的响应
	Embeddings(ctx context.Context, inputs []string, options map[string]interface{}) (map[string]interface{}, error)

	// ImagesGenerations 生成图片
	// prompt: 图片描述
	ImagesGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error)

	// VideosGenerations 生成视频任务
	VideosGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error)

	// GetVideoTask 查询视频生成任务状态
	GetVideoTask(ctx context.Context, taskID string) (map[string]interface{}, error)

	// GetProvider 获取当前适配器使用的提供商信息
	GetProvider() *models.AiProvider

	// GetModelConfig 获取当前适配器使用的模型配置
	GetModelConfig() *models.AiModel

	// GetApiKey 获取当前使用的API Key
	GetApiKey() string

	// TransformRequest 转换请求格式（将OpenAI格式转为厂商特定格式）
	// requestData: OpenAI格式的请求数据
	// requestType: 请求类型 (chat, embeddings, images, videos等)
	// 返回厂商特定格式的请求数据
	TransformRequest(requestData map[string]interface{}, requestType string) (map[string]interface{}, error)

	// TransformResponse 转换响应格式（将厂商特定格式转为OpenAI格式）
	// responseData: 厂商返回的原始响应数据
	// requestType: 请求类型
	// 返回标准OpenAI格式的响应数据
	TransformResponse(responseData map[string]interface{}, requestType string) map[string]interface{}

	// ExtractTokenUsage 从响应中提取token使用量
	// 返回: promptTokens, completionTokens, totalTokens
	ExtractTokenUsage(response map[string]interface{}) (int, int, int)

	// PassthroughRequest 透传请求（保持向后兼容，直接转发到厂商API）
	// 上游返回非 2xx 时返回 *UpstreamError（携带上游状态码与错误体）
	PassthroughRequest(ctx context.Context, requestData map[string]interface{}, requestType string) (map[string]interface{}, error)

	// PassthroughStreamRequest 流式透传请求（保持向后兼容）
	// 上游返回非 2xx 时在触发任何 callback 之前返回 *UpstreamError，调用方可向客户端返回真实状态码
	PassthroughStreamRequest(ctx context.Context, requestData map[string]interface{}, callback func(string), requestType string) error
}
