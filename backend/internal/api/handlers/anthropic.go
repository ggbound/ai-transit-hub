package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AnthropicCompatibleHandler struct {
	BaseHandler
}

func NewAnthropicCompatibleHandler() *AnthropicCompatibleHandler {
	return &AnthropicCompatibleHandler{}
}

func (h *AnthropicCompatibleHandler) ListModels(c *gin.Context) {
	// 公开模型列表经 60s TTL 缓存，减少每请求全量查库
	modelsList, ok := getCachedPublicModels()
	if !ok {
		respondV1Error(c, http.StatusInternalServerError, "Failed to fetch models", "api_error", "", "models_fetch_failed")
		return
	}

	data := make([]map[string]interface{}, 0, len(modelsList))
	for _, m := range modelsList {
		providerCode := "unknown"
		if m.Provider.ID > 0 {
			providerCode = m.Provider.Code
		}
		data = append(data, map[string]interface{}{
			"id":           providerCode + "/" + m.Code,
			"display_name": m.Name,
			"created_at":   m.CreatedAt.Unix(),
			"provider":     providerCode,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     data,
		"has_more": false,
		"first_id": func() interface{} {
			if len(data) > 0 {
				return data[0]["id"]
			}
			return nil
		}(),
		"last_id": func() interface{} {
			if len(data) > 0 {
				return data[len(data)-1]["id"]
			}
			return nil
		}(),
	})
}

func (h *AnthropicCompatibleHandler) Messages(c *gin.Context) {
	// 先读取原始 body（只能读一次）
	body, err := c.GetRawData()
	if err != nil {
		respondV1Error(c, http.StatusBadRequest, "failed to read request body: "+err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}

	// 解析请求 - 一次 Unmarshal 到 map，同时用于校验与透传（保留所有字段）
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

	// 替换模型ID，使用实际的模型code而不是带提供商前缀的格式
	requestData["model"] = model.Code

	// 提取流式标记
	stream, _ := requestData["stream"].(bool)

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, requestData, "anthropic", "messages", "api", RequestTypeMessages, "[V1-Messages]")
	if !ok {
		return
	}

	if stream {
		// Anthropic 流式 usage 分散在多个事件中，需要合并提取：
		//   - data 根级别的 usage（OpenAI 格式，或 message_delta 格式）
		//   - message 下的 usage（Anthropic message_start）
		//   - delta 下的 usage（兼容格式，output/input 增量合并）
		executeV1Stream(c, v, requestData, "messages", "[V1-Messages]", func(decoded map[string]interface{}, finalUsage *map[string]interface{}) {
			if usage, ok := decoded["usage"].(map[string]interface{}); ok {
				*finalUsage = usage
			}
			if message, ok := decoded["message"].(map[string]interface{}); ok {
				if usage, ok := message["usage"].(map[string]interface{}); ok {
					*finalUsage = usage
				}
			}
			if delta, ok := decoded["delta"].(map[string]interface{}); ok {
				if usage, ok := delta["usage"].(map[string]interface{}); ok {
					if *finalUsage == nil {
						*finalUsage = usage
					} else {
						if outputTokens, ok := usage["output_tokens"].(float64); ok {
							(*finalUsage)["output_tokens"] = outputTokens
						}
						if inputTokens, ok := usage["input_tokens"].(float64); ok {
							(*finalUsage)["input_tokens"] = inputTokens
						}
					}
				}
			}
		})
	} else {
		executeV1NonStream(c, v, requestData, "messages", "[V1-Messages]")
	}
}
