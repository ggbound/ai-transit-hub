package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ImagesHandler struct {
	BaseHandler
}

func NewImagesHandler() *ImagesHandler {
	return &ImagesHandler{}
}

func (h *ImagesHandler) Generations(c *gin.Context) {
	// 先读取原始 body
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

	// 提取流式标记
	stream, _ := reqData["stream"].(bool)

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, reqData, "images", "images/generations", "image", RequestTypeImageGen, "[V1-Images]")
	if !ok {
		return
	}

	if stream {
		executeV1Stream(c, v, reqData, "image", "[V1-Images]", nil)
	} else {
		executeV1NonStream(c, v, reqData, "image", "[V1-Images]")
	}
}
