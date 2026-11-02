package service

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// AccountingService 负责请求记账：usage_log / api_test_request / api_test_detail 的创建与更新。
// 从 handlers 与 utils 中收敛 DB 记账职责，形成独立服务层。
type AccountingService struct{}

// Accounting 包级单例，调用点形如 service.Accounting.UpdateSuccessRecords(...)
var Accounting = &AccountingService{}

// RecordIDs 保存创建的记录ID和开始时间
type RecordIDs struct {
	UsageLogID       uint
	ApiTestRequestID uint
	StartTime        time.Time
}

// TruncateUTF8 按字节上限截断字符串，但不切断 UTF-8 多字节字符：
// 从上限位置向前回退到最近一个 rune 起始字节，避免产生非法 UTF-8（坏 JSON）
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	// 回退直到落在 rune 起始字节（UTF-8 续字节形如 10xxxxxx）
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// CreateInitialRecords 请求前创建初始记录（匹配 PHP createInitialRecords）
// 任一记录创建失败即返回错误，调用方应中止请求，避免无记录的“幽灵”请求
func (s *AccountingService) CreateInitialRecords(userID uint, model *models.AiModel, provider *models.AiProvider,
	apiType string, endpoint string, apiKeyID uint, sessionID uint, externalTaskID string, testType string) (*RecordIDs, error) {

	now := time.Now()

	logger.Debug("CreateInitialRecords called - userID: %d, modelID: %d, providerID: %d, apiKeyID: %d, sessionID: %d, endpoint: %s",
		userID, model.ID, provider.ID, apiKeyID, sessionID, endpoint)

	// 1. 创建 UsageLog（状态 2=处理中）
	usageLog := models.UsageLog{
		UserID:           userID,
		ModelID:          model.ID,
		PromptTokens:     0,
		CompletionTokens: 0,
		TotalTokens:      0,
		Cost:             0,
		RequestTime:      &now,
		ResponseTime:     nil,
		Status:           2, // 处理中
	}

	// 处理可选字段 - 只有非0值才设置
	if apiKeyID > 0 {
		usageLog.ApiKeyID = &apiKeyID
	}
	if sessionID > 0 {
		usageLog.SessionID = &sessionID
	}

	result := repository.DB.Create(&usageLog)
	if result.Error != nil {
		logger.Error("Failed to create usage_log - error: %v", result.Error)
		return nil, result.Error
	}
	logger.Debug("Created usage_log - id: %d", usageLog.ID)

	// 2. 创建 ApiTestRequest
	modelStr := provider.Code
	if model.Code != "" {
		modelStr = provider.Code + "/" + model.Code
	}

	apiTestRequest := models.ApiTestRequest{
		UserID:         userID,
		ApiType:        apiType,
		Endpoint:       endpoint,
		Model:          modelStr,
		Status:         0, // 初始状态
		Type:           testType,
		UsageLogID:     usageLog.ID,
		UserApiKeyID:   apiKeyID,
		ExternalTaskID: externalTaskID,
	}

	result = repository.DB.Create(&apiTestRequest)
	if result.Error != nil {
		logger.Error("Failed to create api_test_request - error: %v", result.Error)
		return nil, result.Error
	}
	logger.Debug("Created api_test_request - id: %d, usage_log_id: %d", apiTestRequest.ID, usageLog.ID)

	return &RecordIDs{
		UsageLogID:       usageLog.ID,
		ApiTestRequestID: apiTestRequest.ID,
		StartTime:        time.Now(),
	}, nil
}

// extractTokensFromStreamResponse 从流式响应字符串中提取 tokens
func extractTokensFromStreamResponse(response interface{}) (promptTokens, completionTokens, totalTokens int) {
	promptTokens = 0
	completionTokens = 0
	totalTokens = 0

	// 先检查是不是 map 类型，里面有 raw_stream
	if respMap, ok := response.(map[string]interface{}); ok {
		if rawStream, ok := respMap["raw_stream"].([]string); ok {
			// 遍历每一行
			for _, line := range rawStream {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "data: ") {
					data := strings.TrimPrefix(trimmed, "data: ")
					if data == "[DONE]" {
						continue
					}

					var decoded map[string]interface{}
					if err := json.Unmarshal([]byte(data), &decoded); err == nil {
						// 检查 1: 顶层的 usage (message_delta)
						if usage, ok := decoded["usage"].(map[string]interface{}); ok {
							pTokens, cTokens, tTokens := extractUsageFromMap(usage)
							if tTokens > 0 {
								promptTokens = pTokens
								completionTokens = cTokens
								totalTokens = tTokens
							}
						}
						// 检查 2: message.usage (message_start)
						if message, ok := decoded["message"].(map[string]interface{}); ok {
							if usage, ok := message["usage"].(map[string]interface{}); ok {
								pTokens, cTokens, tTokens := extractUsageFromMap(usage)
								if tTokens > 0 {
									promptTokens = pTokens
									completionTokens = cTokens
									totalTokens = tTokens
								}
							}
						}
					}
				}
			}
		}
	}

	return promptTokens, completionTokens, totalTokens
}

// updateUsageLogRows 单次 UPDATE 更新 usage_log（替代原先 SELECT+SAVE 的 2 次往返）
func updateUsageLogRows(id uint, updates map[string]interface{}) {
	result := repository.DB.Model(&models.UsageLog{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update usage_log - id: %d, error: %v", id, result.Error)
	} else if result.RowsAffected == 0 {
		logger.Error("UsageLog not found for update - id: %d", id)
	} else {
		logger.Debug("Updated usage_log successfully - id: %d", id)
	}
}

// updateApiTestRequestRows 单次 UPDATE 更新 api_test_request（替代原先 SELECT+SAVE 的 2 次往返）
func updateApiTestRequestRows(id uint, updates map[string]interface{}) {
	result := repository.DB.Model(&models.ApiTestRequest{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update api_test_request - id: %d, error: %v", id, result.Error)
	} else if result.RowsAffected == 0 {
		logger.Error("ApiTestRequest not found for update - id: %d", id)
	} else {
		logger.Debug("Updated api_test_request successfully - id: %d", id)
	}
}

// UpdateSuccessRecords 请求成功后更新记录（匹配 PHP updateSuccessRecords）
func (s *AccountingService) UpdateSuccessRecords(recordIDs *RecordIDs, status int, response interface{},
	promptTokens int, completionTokens int, totalTokens int, cost float64,
	method string, endpointURL string, requestHeaders map[string][]string, requestBody interface{},
	model *models.AiModel) {

	duration := int(time.Since(recordIDs.StartTime).Milliseconds())
	now := time.Now()

	logger.Debug("UpdateSuccessRecords called - usage_log_id: %d, api_test_request_id: %d, prompt_tokens: %d, completion_tokens: %d, total_tokens: %d, cost: %f",
		recordIDs.UsageLogID, recordIDs.ApiTestRequestID,
		promptTokens, completionTokens, totalTokens, cost)

	// 如果传入的 tokens 都是 0，尝试从 response 中重新提取
	if promptTokens == 0 && completionTokens == 0 && totalTokens == 0 {
		logger.Debug("Tokens are all 0, trying to extract from response...")
		promptTokens, completionTokens, totalTokens = extractTokensFromStreamResponse(response)
		logger.Debug("Extracted from response - prompt: %d, completion: %d, total: %d", promptTokens, completionTokens, totalTokens)

		// 如果重新提取到了 tokens，重新计算 cost
		if totalTokens > 0 && model != nil {
			cost = ai.CalculateCost(promptTokens, completionTokens, model)
			logger.Debug("Recalculated cost: %f", cost)
		}
	}

	// 提取外部任务 ID
	externalTaskID := extractExternalTaskID(response)

	// 更新 UsageLog（单次 UPDATE）
	updateUsageLogRows(recordIDs.UsageLogID, map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      totalTokens,
		"cost":              cost,
		"response_time":     now,
		"status":            1, // 成功
	})

	// 更新 ApiTestRequest（单次 UPDATE）
	apiTestUpdates := map[string]interface{}{
		"status":       status,
		"duration":     duration,
		"method":       method,
		"endpoint_url": endpointURL,
	}
	if externalTaskID != "" {
		apiTestUpdates["external_task_id"] = externalTaskID
	}
	updateApiTestRequestRows(recordIDs.ApiTestRequestID, apiTestUpdates)

	// 编码响应数据
	responseBody, _ := json.Marshal(response)
	responseBody = []byte(TruncateUTF8(string(responseBody), 65535)) // 限制大小（rune 边界安全）

	// 编码请求数据
	requestHeadersJSON, _ := json.Marshal(requestHeaders)
	requestBodyJSON, _ := json.Marshal(requestBody)

	// 创建 ApiTestDetail
	apiTestDetail := models.ApiTestDetail{
		RequestID:       recordIDs.ApiTestRequestID,
		RequestHeaders:  string(requestHeadersJSON),
		RequestBody:     string(requestBodyJSON),
		ResponseHeaders: "{}", // 暂时为空
		ResponseBody:    string(responseBody),
	}
	if err := repository.DB.Create(&apiTestDetail).Error; err != nil {
		logger.Error("Failed to create ApiTestDetail for request %d: %v", recordIDs.ApiTestRequestID, err)
	}
}

// UpdateErrorRecords 请求失败后更新记录（匹配 PHP updateErrorRecords）
func (s *AccountingService) UpdateErrorRecords(recordIDs *RecordIDs, status int, errorMessage string,
	method string, endpointURL string, requestHeaders map[string][]string, requestBody interface{}) {

	duration := int(time.Since(recordIDs.StartTime).Milliseconds())
	now := time.Now()

	logger.Debug("UpdateErrorRecords called - usage_log_id: %d, api_test_request_id: %d, status: %d, error_message: %s",
		recordIDs.UsageLogID, recordIDs.ApiTestRequestID,
		status, errorMessage)

	// 更新 UsageLog（单次 UPDATE，状态 0=失败）
	updateUsageLogRows(recordIDs.UsageLogID, map[string]interface{}{
		"response_time": now,
		"status":        0, // 失败
		"error_message": errorMessage,
	})

	// 更新 ApiTestRequest（单次 UPDATE）
	updateApiTestRequestRows(recordIDs.ApiTestRequestID, map[string]interface{}{
		"status":        status,
		"duration":      duration,
		"error_message": errorMessage,
		"method":        method,
		"endpoint_url":  endpointURL,
	})

	// 编码请求数据
	requestHeadersJSON, _ := json.Marshal(requestHeaders)
	requestBodyJSON, _ := json.Marshal(requestBody)

	// 创建 ApiTestDetail
	apiTestDetail := models.ApiTestDetail{
		RequestID:       recordIDs.ApiTestRequestID,
		RequestHeaders:  string(requestHeadersJSON),
		RequestBody:     string(requestBodyJSON),
		ResponseHeaders: "{}",
		ResponseBody:    `{"error": "` + errorMessage + `"}`,
	}
	if err := repository.DB.Create(&apiTestDetail).Error; err != nil {
		logger.Error("Failed to create ApiTestDetail for request %d: %v", recordIDs.ApiTestRequestID, err)
	}
}

// UpdateErrorRecordsWithUsage 更新失败记录，同时回填已产生的 token 用量与费用。
// 用于流式中途失败、客户端中途断开等场景——上游已消耗的 tokens 不应丢失记账。
// 单次 UPDATE 完成（原先先 UpdateErrorRecords 再 First+Save 补 token，共 5 次往返）。
func (s *AccountingService) UpdateErrorRecordsWithUsage(recordIDs *RecordIDs, status int, errorMessage string,
	promptTokens int, completionTokens int, totalTokens int, cost float64,
	method string, endpointURL string, requestHeaders map[string][]string, requestBody interface{}) {

	duration := int(time.Since(recordIDs.StartTime).Milliseconds())
	now := time.Now()

	logger.Debug("UpdateErrorRecordsWithUsage called - usage_log_id: %d, api_test_request_id: %d, status: %d, tokens: %d/%d/%d",
		recordIDs.UsageLogID, recordIDs.ApiTestRequestID,
		status, promptTokens, completionTokens, totalTokens)

	// 更新 UsageLog（单次 UPDATE：失败状态 + 已产生的 token 用量与费用）
	updateUsageLogRows(recordIDs.UsageLogID, map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      totalTokens,
		"cost":              cost,
		"response_time":     now,
		"status":            0, // 失败
		"error_message":     errorMessage,
	})

	// 更新 ApiTestRequest（单次 UPDATE）
	updateApiTestRequestRows(recordIDs.ApiTestRequestID, map[string]interface{}{
		"status":        status,
		"duration":      duration,
		"error_message": errorMessage,
		"method":        method,
		"endpoint_url":  endpointURL,
	})

	// 编码请求数据
	requestHeadersJSON, _ := json.Marshal(requestHeaders)
	requestBodyJSON, _ := json.Marshal(requestBody)

	// 创建 ApiTestDetail
	apiTestDetail := models.ApiTestDetail{
		RequestID:       recordIDs.ApiTestRequestID,
		RequestHeaders:  string(requestHeadersJSON),
		RequestBody:     string(requestBodyJSON),
		ResponseHeaders: "{}",
		ResponseBody:    `{"error": "` + errorMessage + `"}`,
	}
	if err := repository.DB.Create(&apiTestDetail).Error; err != nil {
		logger.Error("Failed to create ApiTestDetail for request %d: %v", recordIDs.ApiTestRequestID, err)
	}
}

// extractUsageFromMap 从单个 usage map 中提取 tokens：
// 委托 ai.ExtractIntFromMap（与 ai 包 extractUsageFromMap 同一逻辑，兼容 OpenAI/Anthropic 字段命名）
func extractUsageFromMap(usage map[string]interface{}) (promptTokens, completionTokens, totalTokens int) {
	promptTokens = ai.ExtractIntFromMap(usage, "prompt_tokens", "input_tokens")
	completionTokens = ai.ExtractIntFromMap(usage, "completion_tokens", "output_tokens")
	totalTokens = ai.ExtractIntFromMap(usage, "total_tokens")
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	return promptTokens, completionTokens, totalTokens
}

// extractExternalTaskID 从响应中提取外部任务 ID
func extractExternalTaskID(response interface{}) string {
	if response == nil {
		return ""
	}

	if respMap, ok := response.(map[string]interface{}); ok {
		// 尝试各种可能的字段
		if id, ok := respMap["id"].(string); ok && id != "" {
			return id
		}
		if id, ok := respMap["task_id"].(string); ok && id != "" {
			return id
		}
		if id, ok := respMap["taskId"].(string); ok && id != "" {
			return id
		}

		// 检查 data 嵌套
		if data, ok := respMap["data"].(map[string]interface{}); ok {
			if id, ok := data["id"].(string); ok && id != "" {
				return id
			}
			if id, ok := data["task_id"].(string); ok && id != "" {
				return id
			}
			if id, ok := data["taskId"].(string); ok && id != "" {
				return id
			}
		}
	}

	return ""
}
