package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
)

type ResponsesHandler struct {
	BaseHandler
}

func NewResponsesHandler() *ResponsesHandler {
	return &ResponsesHandler{}
}

func (h *ResponsesHandler) Create(c *gin.Context) {
	// 先读取原始body以保留所有字段
	body, err := c.GetRawData()
	if err != nil {
		respondV1Error(c, http.StatusBadRequest, "failed to read request body: "+err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}

	// 一次 Unmarshal 到 map，同时用于校验与透传（保留所有字段）
	reqData := map[string]interface{}{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		respondV1Error(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}

	// 解析并校验 provider/model
	modelStr, _ := reqData["model"].(string)
	provider, model, ok := parseV1Model(c, modelStr)
	if !ok {
		return
	}

	// 替换模型ID，使用实际的模型code
	reqData["model"] = model.Code

	reqData = ai.PreparePassthroughRequest(reqData, model, "response")

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, reqData, "responses", "responses", "response", RequestTypeResponses, "[V1-Responses]")
	if !ok {
		return
	}

	// 提取流式标记
	stream, _ := reqData["stream"].(bool)
	if stream {
		// responses 流式 usage 在 response.completed 事件的 response.usage 中，
		// 兼容豆包的顶层 usage / output.usage 格式
		executeV1Stream(c, v, reqData, "response", "[V1-Responses]", func(decoded map[string]interface{}, finalUsage *map[string]interface{}) {
			if resp, ok := decoded["response"].(map[string]interface{}); ok {
				if usg, ok := resp["usage"].(map[string]interface{}); ok {
					*finalUsage = usg
					logger.Debug("[V1-Responses] Found usage in response.completed:", usg)
				}
			}
			if usg, ok := decoded["usage"].(map[string]interface{}); ok {
				*finalUsage = usg
			} else if output, ok := decoded["output"].(map[string]interface{}); ok {
				if usg, ok := output["usage"].(map[string]interface{}); ok {
					*finalUsage = usg
				}
			}
		})
	} else {
		executeV1NonStream(c, v, reqData, "response", "[V1-Responses]")
	}
}
