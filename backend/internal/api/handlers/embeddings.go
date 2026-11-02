package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/metrics"
)

type EmbeddingHandler struct {
	BaseHandler
}

func NewEmbeddingHandler() *EmbeddingHandler {
	return &EmbeddingHandler{}
}

func (h *EmbeddingHandler) Create(c *gin.Context) {
	// 先读取原始 body（保留所有字段）
	body, err := c.GetRawData()
	if err != nil {
		respondV1Error(c, http.StatusBadRequest, "failed to read request body: "+err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}

	// 一次 Unmarshal 到 map，同时用于校验、提取 input 与透传（保留所有字段）
	bodyMap := map[string]interface{}{}
	if err := json.Unmarshal(body, &bodyMap); err != nil {
		respondV1Error(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "body", "invalid_body")
		return
	}
	input := bodyMap["input"]

	// 解析并校验 provider/model
	modelStr, _ := bodyMap["model"].(string)
	provider, model, ok := parseV1Model(c, modelStr)
	if !ok {
		return
	}

	// 准备请求数据 - 修正模型编码（只保留 modelCode，去掉 providerCode），并保留其他可选参数
	reqData := map[string]interface{}{
		"model": model.Code,
		"input": input,
	}
	for k, v := range bodyMap {
		if k != "model" && k != "input" {
			reqData[k] = v
		}
	}

	log.Info("[V1-Embeddings] 修正模型编码: %s -> %s", modelStr, model.Code)

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, reqData, "embeddings", "embeddings", "api", RequestTypeEmbeddings, "[V1-Embeddings]")
	if !ok {
		return
	}

	// 检查输入是否为简单格式（纯文本）
	// 注：JSON 解码后 Input 只可能是 string/[]interface{}/map 等，不存在 []string
	isSimpleInput := true
	if inputList, ok := input.([]interface{}); ok {
		for _, item := range inputList {
			if _, ok := item.(map[string]interface{}); ok {
				isSimpleInput = false
				break
			}
		}
	}

	var response map[string]interface{}
	var adapterErr error

	if isSimpleInput {
		// 简单输入：使用标准 Embeddings 方法
		var inputs []string
		switch inputValue := input.(type) {
		case string:
			inputs = []string{inputValue}
		case []string:
			inputs = inputValue
		case []interface{}:
			for _, item := range inputValue {
				if str, ok := item.(string); ok {
					inputs = append(inputs, str)
				}
			}
		}

		options := map[string]interface{}{
			"model": modelStr,
		}
		if encodingFormat, ok := reqData["encoding_format"]; ok {
			options["encoding_format"] = encodingFormat
		}
		if dimensions, ok := reqData["dimensions"]; ok {
			options["dimensions"] = dimensions
		}
		if modality, ok := reqData["modality"]; ok {
			options["modality"] = modality
		}

		log.Info("[V1-Embeddings] 使用标准 Embeddings 方法处理简单输入")
		response, adapterErr = v.Adapter.Embeddings(c.Request.Context(), inputs, options)
	} else {
		// 复杂输入（多模态）：直接透传原始请求
		log.Info("[V1-Embeddings] 使用透传方式处理多模态输入")
		response, adapterErr = v.Adapter.PassthroughRequest(c.Request.Context(), reqData, "embeddings")
	}

	if adapterErr != nil {
		// 上游错误透传真实状态码与错误体
		metrics.V1RequestsTotal.WithLabelValues("embeddings", "error").Inc()
		handlePassthroughError(c, adapterErr, v.RecordIDs, "POST", c.Request.URL.String(), c.Request.Header, reqData)
		return
	}

	// 出口归一化：无论上游返回什么形状，客户端始终拿到 OpenAI 标准结构
	// （data 为数组、每项含 object/index；顶层补 object=list 与 model）
	response = normalizeEmbeddingsResponse(response, model.Code)

	if respondV1Success(c, v, reqData, response, "[V1-Embeddings]") {
		metrics.V1RequestsTotal.WithLabelValues("embeddings", "success").Inc()
	} else {
		metrics.V1RequestsTotal.WithLabelValues("embeddings", "error").Inc()
	}
}

// normalizeEmbeddingsResponse 将上游 embeddings 响应归一化为 OpenAI 标准结构：
//   - data 为单对象（豆包多模态等上游的单向量语义）时包装为 [{...index:0}]
//   - 数组项缺失 index 时按顺序补齐，缺失 object 时补 "embedding"
//   - 顶层缺失 object 补 "list"、缺失 model 补上游真实模型编码
//
// 仅做结构归一化，不改动向量数据与 usage；对已是标准结构的响应幂等无害。
func normalizeEmbeddingsResponse(response map[string]interface{}, modelCode string) map[string]interface{} {
	if response == nil {
		return response
	}

	if _, ok := response["object"]; !ok {
		response["object"] = "list"
	}
	if _, ok := response["model"]; !ok {
		response["model"] = modelCode
	}

	switch d := response["data"].(type) {
	case map[string]interface{}:
		normalizeEmbeddingItem(d, 0)
		response["data"] = []interface{}{d}
	case []interface{}:
		for i, item := range d {
			if m, ok := item.(map[string]interface{}); ok {
				normalizeEmbeddingItem(m, i)
			}
		}
	}

	return response
}

// normalizeEmbeddingItem 补齐单条 embedding 项的标准字段（object/index）
func normalizeEmbeddingItem(item map[string]interface{}, index int) {
	if _, ok := item["object"]; !ok {
		item["object"] = "embedding"
	}
	if _, ok := item["index"]; !ok {
		item["index"] = index
	}
}
