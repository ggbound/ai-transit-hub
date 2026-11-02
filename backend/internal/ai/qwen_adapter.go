package ai

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/your-org/ai-transit-hub/internal/models"
)

// QwenAdapter 通义千问适配器
// 公共能力（认证、透传、视频任务等）继承自嵌入的 *GenericAdapter，
// 此处仅保留千问厂商差异逻辑：原生 input.texts / input.prompt 格式、
// 响应格式转换、流式行解析。
type QwenAdapter struct {
	*GenericAdapter
}

// NewQwenAdapter 创建通义千问适配器
func NewQwenAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *QwenAdapter {
	providerKey := &models.ProviderApiKey{
		AiProviderID: provider.ID,
		ApiKey:       apiKey,
		Status:       1,
	}

	return &QwenAdapter{
		GenericAdapter: &GenericAdapter{
			client:      upstreamHTTPClient,
			provider:    provider,
			apiKey:      providerKey,
			modelConfig: modelConfig,
		},
	}
}

// ==================== Adapter 接口实现（厂商覆写） ====================

// Chat 发送聊天请求（非流式）
func (a *QwenAdapter) Chat(ctx context.Context, messages []Message, options map[string]interface{}) (map[string]interface{}, error) {
	// 构建 OpenAI 格式请求（复用 Generic 公共构建逻辑）
	openaiRequest := a.buildRequestData(messages, options, false)

	// 发送到通用适配器
	resp, err := a.GenericAdapter.PassthroughRequest(ctx, openaiRequest, "chat")
	if err != nil {
		return nil, err
	}

	// 转换通义千问响应为 OpenAI 格式
	return a.TransformResponse(resp, "chat"), nil
}

// ChatStream 发送聊天请求（流式）
func (a *QwenAdapter) ChatStream(ctx context.Context, messages []Message, options map[string]interface{}, callback StreamCallback) error {
	openaiRequest := a.buildRequestData(messages, options, true)

	return a.GenericAdapter.PassthroughStreamRequest(ctx, openaiRequest, func(line string) {
		data := a.parseQwenStreamLine(line)
		if data != nil {
			callback(data)
		}
	}, "chat")
}

// Embeddings 获取文本嵌入向量
func (a *QwenAdapter) Embeddings(ctx context.Context, inputs []string, options map[string]interface{}) (map[string]interface{}, error) {
	// 构建通义千问格式请求
	qwenRequest := map[string]interface{}{
		"input": map[string]interface{}{
			"texts": inputs,
		},
		"model": a.modelConfig.Code,
	}

	if model, ok := options["model"].(string); ok && model != "" {
		qwenRequest["model"] = model
	}

	resp, err := a.GenericAdapter.PassthroughRequest(ctx, qwenRequest, "embeddings")
	if err != nil {
		return nil, err
	}

	// 转换通义千问响应为 OpenAI 格式
	return a.transformQwenEmbeddingsResponse(resp), nil
}

// ImagesGenerations 生成图片
func (a *QwenAdapter) ImagesGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error) {
	// 通义千问图片生成格式
	qwenRequest := map[string]interface{}{
		"model": a.modelConfig.Code,
		"input": map[string]interface{}{
			"prompt": prompt,
		},
	}

	if model, ok := options["model"].(string); ok && model != "" {
		qwenRequest["model"] = model
	}

	// 复制可选参数
	input := qwenRequest["input"].(map[string]interface{})
	optionalParams := []string{"size", "n", "seed", "negative_prompt"}
	for _, param := range optionalParams {
		if val, ok := options[param]; ok {
			input[param] = val
		}
	}

	resp, err := a.GenericAdapter.PassthroughRequest(ctx, qwenRequest, "images")
	if err != nil {
		return nil, err
	}

	// 转换通义千问响应为 OpenAI 格式
	return a.transformQwenImagesResponse(resp), nil
}

// TransformRequest 转换请求格式
func (a *QwenAdapter) TransformRequest(requestData map[string]interface{}, requestType string) (map[string]interface{}, error) {
	if requestType == "embeddings" {
		return a.transformEmbeddingsRequest(requestData), nil
	}
	return requestData, nil
}

// TransformResponse 转换响应格式
func (a *QwenAdapter) TransformResponse(responseData map[string]interface{}, requestType string) map[string]interface{} {
	if requestType == "chat" {
		return a.transformChatResponse(responseData)
	}
	if requestType == "embeddings" {
		return a.transformQwenEmbeddingsResponse(responseData)
	}
	if requestType == "images" {
		return a.transformQwenImagesResponse(responseData)
	}
	return responseData
}

// ExtractTokenUsage 提取 Token 使用量
func (a *QwenAdapter) ExtractTokenUsage(response map[string]interface{}) (int, int, int) {
	// 通义千问响应格式
	if usage, ok := response["usage"]; ok {
		if usageMap, ok := usage.(map[string]interface{}); ok {
			promptTokens := ExtractIntFromMap(usageMap, "input_tokens", "prompt_tokens")
			completionTokens := ExtractIntFromMap(usageMap, "output_tokens", "completion_tokens")
			totalTokens := ExtractIntFromMap(usageMap, "total_tokens")
			if totalTokens == 0 {
				totalTokens = promptTokens + completionTokens
			}
			return promptTokens, completionTokens, totalTokens
		}
	}

	// 回退到通用处理
	return ExtractTokenUsage(response)
}

// ==================== 内部辅助方法 ====================

func (a *QwenAdapter) transformChatResponse(qwenResp map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"object": "chat.completion",
		"model":  qwenResp["model"],
	}

	if choices, ok := qwenResp["choices"]; ok {
		result["choices"] = choices
	} else if output, ok := qwenResp["output"]; ok {
		// 通义千问格式
		if outputMap, ok := output.(map[string]interface{}); ok {
			if choices, ok := outputMap["choices"]; ok {
				result["choices"] = choices
			}
		}
	}

	if usage, ok := qwenResp["usage"]; ok {
		result["usage"] = usage
	}

	return result
}

func (a *QwenAdapter) transformEmbeddingsRequest(openaiRequest map[string]interface{}) map[string]interface{} {
	// OpenAI: {"model": "text-embedding-v2", "input": ["text1", "text2"]}
	// 通义千问: {"model": "text-embedding-v2", "input": {"texts": ["text1", "text2"]}}
	result := map[string]interface{}{
		"model": openaiRequest["model"],
	}

	if input, ok := openaiRequest["input"]; ok {
		if inputSlice, ok := input.([]interface{}); ok {
			result["input"] = map[string]interface{}{
				"texts": inputSlice,
			}
		}
	}

	return result
}

func (a *QwenAdapter) transformQwenEmbeddingsResponse(qwenResp map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"object": "list",
		"model":  qwenResp["model"],
	}

	// 转换通义千问响应
	if output, ok := qwenResp["output"]; ok {
		if outputMap, ok := output.(map[string]interface{}); ok {
			if embeddings, ok := outputMap["embeddings"]; ok {
				result["data"] = embeddings
			}
		}
	}

	if usage, ok := qwenResp["usage"]; ok {
		result["usage"] = usage
	}

	return result
}

func (a *QwenAdapter) transformQwenImagesResponse(qwenResp map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"created": time.Now().Unix(),
		"data":    []interface{}{},
	}

	// 通义千问响应格式: {"output": {"results": [{"url": "..."}]}}
	if output, ok := qwenResp["output"]; ok {
		if outputMap, ok := output.(map[string]interface{}); ok {
			if results, ok := outputMap["results"]; ok {
				result["data"] = results
			}
		}
	}

	return result
}

func (a *QwenAdapter) parseQwenStreamLine(line string) map[string]interface{} {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// 处理 SSE 格式
	if strings.HasPrefix(line, "data: ") {
		data := line[6:]
		if data == "[DONE]" {
			return map[string]interface{}{"done": true}
		}
		// 直接返回原始数据，由通用处理逻辑转换
		var result map[string]interface{}
		json.Unmarshal([]byte(data), &result)
		return a.TransformResponse(result, "chat")
	}

	return nil
}
