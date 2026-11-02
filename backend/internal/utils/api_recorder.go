package utils

import (
	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// CalculateCost 计算成本：统一委托 ai.CalculateCost（唯一实现，含价格 >0 保护）
func CalculateCost(promptTokens, completionTokens int, model *models.AiModel) float64 {
	return ai.CalculateCost(promptTokens, completionTokens, model)
}

// IsErrorResponse 检查响应是否包含错误
func IsErrorResponse(response interface{}) (bool, string) {
	if response == nil {
		return true, "Empty response"
	}

	if respMap, ok := response.(map[string]interface{}); ok {
		// 检查 error 字段（需要确保 error 不是 null/nil）
		if err, ok := respMap["error"]; ok {
			// 明确跳过 null/nil 值
			if err == nil {
				logger.Debug("Found error field but it's null, skipping")
			} else {
				errStr := ""
				switch e := err.(type) {
				case string:
					errStr = e
				case map[string]interface{}:
					if msg, ok := e["message"].(string); ok {
						errStr = msg
					} else if msg, ok := e["code"].(string); ok {
						errStr = msg
					}
				default:
					errStr = "Unknown error format"
				}
				if errStr == "" {
					errStr = "API returned error"
				}
				return true, errStr
			}
		}

		// 检查 success 字段为 false
		if success, ok := respMap["success"].(bool); ok && !success {
			if msg, ok := respMap["message"].(string); ok {
				return true, msg
			}
			return true, "Success is false"
		}

		// 检查 status 字段为 error
		if status, ok := respMap["status"].(string); ok && status == "error" {
			if msg, ok := respMap["message"].(string); ok {
				return true, msg
			}
			return true, "Status is error"
		}

		// 检查 data.error
		if data, ok := respMap["data"].(map[string]interface{}); ok {
			if err, ok := data["error"]; ok && err != nil {
				return true, "Data contains error"
			}
		}
	}

	return false, ""
}
