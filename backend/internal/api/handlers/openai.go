package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type OpenAICompatibleHandler struct {
	BaseHandler
}

func NewOpenAICompatibleHandler() *OpenAICompatibleHandler {
	return &OpenAICompatibleHandler{}
}

// ListModels 获取模型列表（公开模型经 60s TTL 缓存，减少每请求全量查库）
func (h *OpenAICompatibleHandler) ListModels(c *gin.Context) {
	modelsList, ok := getCachedPublicModels()
	if !ok {
		respondV1Error(c, http.StatusInternalServerError, "Failed to fetch models", "api_error", "", "models_fetch_failed")
		return
	}

	data := make([]map[string]interface{}, 0, len(modelsList))
	for _, m := range modelsList {
		// 如果 Provider 没有预加载成功，尝试手动加载
		if m.Provider.ID == 0 && m.ProviderID > 0 {
			var provider models.AiProvider
			if err := repository.DB.First(&provider, m.ProviderID).Error; err == nil {
				m.Provider = provider
			}
		}

		providerCode := "unknown"
		if m.Provider.ID > 0 {
			providerCode = m.Provider.Code
		} else {
			logger.Warn("Model %d has no provider (provider_id: %d)", m.ID, m.ProviderID)
		}
		data = append(data, map[string]interface{}{
			"id":                  providerCode + "/" + m.Code,
			"object":              "model",
			"created_at":          m.CreatedAt.Unix(),
			"owner":               providerCode,
			"permission":          []interface{}{},
			"capabilities":        json.RawMessage(m.Capabilities),
			"max_tokens":          m.MaxTokens,
			"supports_response":   m.SupportsResponse,
			"supports_messages":   m.SupportsMessages,
			"supports_openai":     true,
			"supports_anthropic":  false,
			"supports_embeddings": m.Type == "embedding",
			"supports_images":     m.Type == "image",
			"supports_videos":     m.Type == "video",
			"supports_audio":      m.Type == "audio",
			"input_price":         m.InputPrice,
			"output_price":        m.OutputPrice,
			"compatibility":       modelCompatibility(&m.Provider, &m),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// modelCompatibility 构建模型的调用契约元数据（能力元数据）：
// 客户端可据此程序化发现端点、请求形状与网关响应保证，而非靠文档或试错。
// 仅声明网关代码实际保证的行为（P1 embeddings 归一化、P2 videos 适配），
// 不重复 admin 配置的自由格式 capabilities 字段。
func modelCompatibility(provider *models.AiProvider, m *models.AiModel) map[string]interface{} {
	compat := map[string]interface{}{
		"endpoints": map[string]interface{}{},
		"request":   map[string]interface{}{},
	}
	endpoints := compat["endpoints"].(map[string]interface{})
	request := compat["request"].(map[string]interface{})

	switch m.Type {
	case ModelTypeText:
		endpoints["chat"] = "POST /v1/chat/completions"
		if m.SupportsResponse {
			endpoints["responses"] = "POST /v1/responses"
		}
		if m.SupportsMessages {
			endpoints["messages"] = "POST /v1/messages"
		}
		request["messages"] = "array"
		request["stream"] = "bool"

	case ModelTypeEmbedding:
		endpoints["create"] = "POST /v1/embeddings"
		// 多模态嵌入模型（生效端点指向 multimodal）额外接受内容项数组；
		// 生效端点解析与适配器 buildUrl 一致：custom_endpoint 优先，embeddings_endpoint 兜底
		effectiveEndpoint := m.CustomEndpoint
		if effectiveEndpoint == "" {
			effectiveEndpoint = m.EmbeddingsEndpoint
		}
		if strings.Contains(strings.ToLower(effectiveEndpoint), "multimodal") {
			request["input"] = []string{"string", "string[]", "multimodal_content[]"}
		} else {
			request["input"] = []string{"string", "string[]"}
		}
		compat["response"] = map[string]interface{}{
			"data":        "array",                // data 恒为数组，每项含 object/index（含上游返回单对象时）
			"multi_input": "one_vector_per_input", // N 个输入 → N 条向量
		}

	case ModelTypeImage:
		endpoints["create"] = "POST /v1/images/generations"
		request["prompt"] = "string"

	case ModelTypeVideo:
		endpoints["create"] = "POST /v1/videos/generations/tasks"
		endpoints["query"] = "GET /v1/videos/generations/tasks/{task_id}"
		request["prompt"] = "string"
		// 豆包系适配器支持原生 content 数组透传，且与 prompt 互斥（并存 400）
		if ai.NewAdapterFactory().NormalizedAdapterClass(provider.AdapterClass) == ai.AdapterClassDoubao {
			request["content"] = "multimodal_content[]"
			request["exclusive_fields"] = []string{"prompt", "content"}
		}
		compat["response"] = map[string]interface{}{
			"status":    "top_level",
			"video_url": "top_level_alias", // content.video_url 提升为顶层别名
		}
	}

	return compat
}

// ChatCompletions 对话补全 - 完全按照 PHP 逻辑重写
func (h *OpenAICompatibleHandler) ChatCompletions(c *gin.Context) {
	// 解析请求 - 一次 Unmarshal 到 map，同时用于校验与透传（保留所有字段）
	body, err := c.GetRawData()
	if err != nil {
		respondV1Error(c, http.StatusBadRequest, "failed to read request body: "+err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}

	requestData := map[string]interface{}{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		respondV1Error(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "model", nil)
		return
	}

	// 解析并校验 provider/model
	modelStr, _ := requestData["model"].(string)
	provider, model, ok := parseV1Model(c, modelStr)
	if !ok {
		return
	}

	// 替换模型ID，使用实际的模型code
	requestData["model"] = model.Code

	// 提取流式标记
	stream, _ := requestData["stream"].(bool)

	// 透传请求处理
	requestData = ai.PreparePassthroughRequest(requestData, model, "chat")

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, requestData, "openai", "chat/completions", "api", RequestTypeChat, "[V1-Chat]")
	if !ok {
		return
	}

	if stream {
		executeV1Stream(c, v, requestData, "chat", "[V1-Chat]", nil)
	} else {
		executeV1NonStream(c, v, requestData, "chat", "[V1-Chat]")
	}
}
