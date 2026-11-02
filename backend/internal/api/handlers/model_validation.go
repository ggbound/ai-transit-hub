package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// 模型类型常量
const (
	ModelTypeText      = "text"
	ModelTypeEmbedding = "embedding"
	ModelTypeImage     = "image"
	ModelTypeVideo     = "video"
	ModelTypeAudio     = "audio"
)

// 请求类型常量
const (
	RequestTypeChat       = "chat"
	RequestTypeEmbeddings = "embeddings"
	RequestTypeImageGen   = "image_generations"
	RequestTypeVideoGen   = "video_generations"
	RequestTypeVideoTask  = "video_task"
	RequestTypeResponses  = "responses"
	RequestTypeMessages   = "messages"
)

// ValidateModelType 验证模型类型是否与请求类型匹配
func ValidateModelType(c *gin.Context, model *models.AiModel, requestType string) bool {
	// 特殊处理：Messages 接口 - 检查 SupportsMessages 字段
	if requestType == RequestTypeMessages {
		if !model.SupportsMessages {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Model does not support Messages API. This model '" + model.Code + "' does not support Messages endpoint.",
					"type":    "invalid_request_error",
					"param":   "model",
					"code":    "model_not_supported",
				},
			})
			return false
		}
		return true
	}

	// 特殊处理：Responses 接口 - 检查 SupportsResponse 字段
	if requestType == RequestTypeResponses {
		if !model.SupportsResponse {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Model does not support Responses API. This model '" + model.Code + "' does not support Responses endpoint.",
					"type":    "invalid_request_error",
					"param":   "model",
					"code":    "model_not_supported",
				},
			})
			return false
		}
		return true
	}

	// 其他接口 - 按模型类型验证
	allowedTypes := getModelTypesForRequest(requestType)
	modelType := model.Type
	if modelType == "" {
		modelType = ModelTypeText // 默认类型为文本对话
	}
	for _, allowedType := range allowedTypes {
		if modelType == allowedType {
			return true
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"message": "Model type mismatch. This endpoint requires a " + getRequestTypeLabel(requestType) + " model, but the model '" + model.Code + "' is of type " + getModelTypeLabel(modelType),
			"type":    "invalid_request_error",
			"param":   "model",
			"code":    "model_type_invalid",
		},
	})
	return false
}

// getModelTypesForRequest 获取请求类型允许的模型类型
func getModelTypesForRequest(requestType string) []string {
	switch requestType {
	case RequestTypeChat:
		return []string{ModelTypeText}
	case RequestTypeEmbeddings:
		return []string{ModelTypeEmbedding}
	case RequestTypeImageGen:
		return []string{ModelTypeImage}
	case RequestTypeVideoGen, RequestTypeVideoTask:
		return []string{ModelTypeVideo}
	default:
		return []string{ModelTypeText}
	}
}

// getRequestTypeLabel 获取请求类型的友好显示名称
func getRequestTypeLabel(requestType string) string {
	switch requestType {
	case RequestTypeChat:
		return "text chat"
	case RequestTypeEmbeddings:
		return "embeddings"
	case RequestTypeImageGen:
		return "image generation"
	case RequestTypeVideoGen:
		return "video generation"
	case RequestTypeVideoTask:
		return "video task query"
	case RequestTypeResponses:
		return "responses"
	case RequestTypeMessages:
		return "messages"
	default:
		return requestType
	}
}

// getModelTypeLabel 获取模型类型的友好显示名称
func getModelTypeLabel(modelType string) string {
	switch modelType {
	case ModelTypeText:
		return "text chat"
	case ModelTypeEmbedding:
		return "embeddings"
	case ModelTypeImage:
		return "image generation"
	case ModelTypeVideo:
		return "video generation"
	case ModelTypeAudio:
		return "audio generation"
	default:
		return modelType
	}
}
