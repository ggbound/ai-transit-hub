package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/utils"
)

type VideosHandler struct {
	BaseHandler
}

func NewVideosHandler() *VideosHandler {
	return &VideosHandler{}
}

func (h *VideosHandler) Generations(c *gin.Context) {
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

	// L3 语义冲突显式化：prompt（网关标准字段）与 content（豆包原生透传字段）语义重叠，
	// 同时存在时无法判定客户端意图，显式 400 而非猜测
	if _, hasPrompt := reqData["prompt"]; hasPrompt {
		if _, hasContent := reqData["content"]; hasContent {
			respondV1Error(c, http.StatusBadRequest,
				"'prompt' and 'content' are mutually exclusive: 'prompt' is the standard field, 'content' is the native passthrough field; use exactly one of them",
				"invalid_request_error", "prompt", "prompt_content_conflict")
			return
		}
		if _, isStr := reqData["prompt"].(string); !isStr {
			respondV1Error(c, http.StatusBadRequest,
				"'prompt' must be a string",
				"invalid_request_error", "prompt", "invalid_prompt_type")
			return
		}
	}

	// 解析并校验 provider/model
	modelStr, _ := reqData["model"].(string)
	provider, model, ok := parseV1Model(c, modelStr)
	if !ok {
		return
	}

	// 替换模型ID，使用实际的模型code
	reqData["model"] = model.Code

	// API Key 选择、初始记录、适配器创建
	v, ok := prepareV1Request(c, provider, model, reqData, "videos", "videos/generations/tasks", "video", RequestTypeVideoGen, "[V1-Videos]")
	if !ok {
		return
	}

	executeV1NonStream(c, v, reqData, "video", "[V1-Videos]")
}

func (h *VideosHandler) GetTask(c *gin.Context) {
	taskId := c.Param("taskId")
	recordId := c.Query("record_id")

	logger.Debug("[V1-Videos] Querying Video Task: task_id=%s, record_id=%s", taskId, recordId)

	var model *models.AiModel
	var provider *models.AiProvider
	var apiKey *models.ProviderApiKey

	// 尝试从历史记录获取模型信息
	if recordId != "" {
		recordIdInt, err := strconv.Atoi(recordId)
		if err == nil {
			var apiTest models.ApiTestRequest
			if err := repository.DB.First(&apiTest, recordIdInt).Error; err == nil {
				// 属主校验：非本人记录按不存在处理，防止越权查询与更新他人记录（IDOR）
				currentUser, authed := GetAuthUser(c)
				if !authed || apiTest.UserID != currentUser.ID {
					logger.Warn("[V1-Videos] Forbidden task query: record_id=%d not owned by current user", recordIdInt)
					respondV1Error(c, http.StatusNotFound,
						"Record not found",
						"invalid_request_error", "record_id", "record_not_found")
					return
				}
				if apiTest.Model != "" {
					logger.Debug("[V1-Videos] Found API Test Record: model=%s, usage_log_id=%d", apiTest.Model, apiTest.UsageLogID)
					// apiTest.Model 格式为 "provider/model_code"，复用带缓存的解析逻辑；
					// 解析失败时 fall through 到默认视频模型兜底
					if p, m, err := ai.GetProviderAndModel(apiTest.Model); err == nil {
						provider = p
						model = m
						apiKey, _ = ai.GetRandomApiKey(provider.ID)
					}
				}
			}
		}
	}

	// 如果没有记录，尝试获取默认的视频模型
	if model == nil || provider == nil || apiKey == nil {
		var defaultModel models.AiModel
		if err := repository.DB.Where("type = ? AND status = ?", "video", 1).First(&defaultModel).Error; err == nil {
			model = &defaultModel
			var prov models.AiProvider
			if err := repository.DB.First(&prov, defaultModel.ProviderID).Error; err == nil {
				provider = &prov
				apiKey, _ = ai.GetRandomApiKey(provider.ID)
				logger.Debug("[V1-Videos] Using default model: provider=%s, model=%s", provider.Code, model.Code)
			}
		}
	}

	if model == nil || provider == nil || apiKey == nil {
		logger.Warn("[V1-Videos] Could not determine model or provider for task")
		// 记录缺失且无可用视频模型兜底，属网关侧配置问题而非客户端可修复错误
		respondV1Error(c, http.StatusInternalServerError,
			"Could not determine model or provider for task",
			"api_error", "model", "model_unresolved")
		return
	}

	logger.Debug("[V1-Videos] Using provider=%s, model=%s for task query", provider.Code, model.Code)

	// 验证模型类型是否匹配
	if !ValidateModelType(c, model, RequestTypeVideoTask) {
		return
	}

	// -------------------- 使用 AdapterFactory 创建适配器 --------------------
	logger.Debug("[V1-Videos] Provider AdapterClass: %s", provider.AdapterClass)

	// 使用 AdapterFactory 创建适配器
	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		logger.Error("[V1-Videos] 适配器创建失败: %v", err)
		respondV1Error(c, http.StatusInternalServerError,
			"Failed to create adapter: "+err.Error(),
			"api_error", "adapter", "adapter_creation_failed")
		return
	}

	// 记录适配器信息
	adapterProvider := adapter.GetProvider()
	adapterModel := adapter.GetModelConfig()
	logger.Debug("[V1-Videos] 适配器创建成功")
	logger.Debug("[V1-Videos] Adapter Provider: %s (AdapterClass=%s)", adapterProvider.Code, adapterProvider.AdapterClass)
	logger.Debug("[V1-Videos] Adapter Model: %s", adapterModel.Code)
	// ------------------------------------------------------------------------

	// 调用适配器的 GetVideoTask 方法查询任务
	logger.Debug("[V1-Videos] Calling GetVideoTask with task_id=%s", taskId)
	data, err := adapter.GetVideoTask(c.Request.Context(), taskId)
	if err != nil {
		logger.Error("[V1-Videos] 查询任务失败: %v", err)
		// 上游明确返回 404 时按任务不存在处理（对齐 agent/tasks 的 404 语义），其余错误仍为 500
		var upstreamErr *ai.UpstreamError
		if errors.As(err, &upstreamErr) && upstreamErr.StatusCode == http.StatusNotFound {
			respondV1Error(c, http.StatusNotFound,
				"Task not found",
				"invalid_request_error", "task_id", "task_not_found")
			return
		}
		respondV1Error(c, http.StatusInternalServerError,
			"Failed to query task status: "+err.Error(),
			"api_error", "task_id", "task_query_failed")
		return
	}

	// 出口归一化：在顶层补 status/video_url 标准别名，客户端可统一从顶层读取，
	// 无需关心上游是否把结果嵌在 content 里（如 Ark）
	normalizeVideoTaskResponse(data)

	// 记录完整响应
	responseJSON, _ := json.Marshal(data)
	logger.Debug("[V1-Videos] Video Task Query Response - status=%s, full_response=%s",
		data["status"], string(responseJSON))

	// 提取并记录 token 信息
	var promptTokens, completionTokens, totalTokens int

	if usage, ok := data["usage"].(map[string]interface{}); ok {
		logger.Debug("[V1-Videos] Found usage info in response: %v", usage)

		// 提取 token 数量
		if v, ok := usage["prompt_tokens"].(float64); ok {
			promptTokens = int(v)
		} else if v, ok := usage["input_tokens"].(float64); ok {
			promptTokens = int(v)
		}
		if v, ok := usage["completion_tokens"].(float64); ok {
			completionTokens = int(v)
		} else if v, ok := usage["output_tokens"].(float64); ok {
			completionTokens = int(v)
		}
		if v, ok := usage["total_tokens"].(float64); ok {
			totalTokens = int(v)
		} else {
			totalTokens = promptTokens + completionTokens
		}
	} else {
		// 尝试其他可能的格式
		logger.Debug("[V1-Videos] No usage field found, checking alternative formats")
		// 检查 data.usage
		if dataMap, ok := data["data"].(map[string]interface{}); ok {
			if usage, ok := dataMap["usage"].(map[string]interface{}); ok {
				logger.Debug("[V1-Videos] Found usage in data field: %v", usage)
				if v, ok := usage["prompt_tokens"].(float64); ok {
					promptTokens = int(v)
				}
				if v, ok := usage["completion_tokens"].(float64); ok {
					completionTokens = int(v)
				}
				if v, ok := usage["total_tokens"].(float64); ok {
					totalTokens = int(v)
				}
			}
		}
	}

	logger.Debug("[V1-Videos] Token Usage: prompt=%d, completion=%d, total=%d",
		promptTokens, completionTokens, totalTokens)

	// 如果状态是 succeeded 且有 record_id，更新记录
	if recordId != "" && data["status"] == "succeeded" {
		recordIdInt, _ := strconv.Atoi(recordId)
		logger.Debug("[V1-Videos] Task succeeded, updating record_id=%d with token info", recordIdInt)
		if user, ok := GetAuthUser(c); ok {
			updateExistingRecord(recordIdInt, data, model, user.ID)
		}
	} else if recordId != "" {
		logger.Debug("[V1-Videos] Task not yet succeeded, skipping record update - status=%v", data["status"])
	}

	c.JSON(http.StatusOK, data)
}

// normalizeVideoTaskResponse 在顶层补 status/video_url 标准别名：
// Ark 的 video_url 位于 content.video_url，status 通常已在顶层；
// 别名仅在顶层缺失且可从嵌套结构提取时补充，不覆盖上游原始字段。
func normalizeVideoTaskResponse(data map[string]interface{}) {
	if data == nil {
		return
	}
	content, ok := data["content"].(map[string]interface{})
	if !ok {
		return
	}
	if _, ok := data["video_url"]; !ok {
		if vurl, ok := content["video_url"].(string); ok && vurl != "" {
			data["video_url"] = vurl
		}
	}
	if _, ok := data["status"]; !ok {
		if st, ok := content["status"].(string); ok && st != "" {
			data["status"] = st
		}
	}
}

// updateExistingRecord 更新已存在的记录（类似 PHP 的 updateExistingRecord）
func updateExistingRecord(recordId int, data map[string]interface{}, model *models.AiModel, ownerID uint) {
	logger.Debug("[V1-Videos] updateExistingRecord starting: record_id=%d, model=%s", recordId, model.Code)

	var apiTest models.ApiTestRequest
	if err := repository.DB.First(&apiTest, recordId).Error; err != nil {
		logger.Error("[V1-Videos] Failed to find record for update: error=%v, record_id=%d", err, recordId)
		return
	}
	// 二次属主校验，防止该函数被其他路径调用时绕过鉴权
	if apiTest.UserID != ownerID {
		logger.Warn("[V1-Videos] Refused to update record_id=%d: not owned by user %d", recordId, ownerID)
		return
	}
	logger.Debug("[V1-Videos] Found API Test Record: usage_log_id=%d", apiTest.UsageLogID)

	// 更新记录
	updates := map[string]interface{}{}

	// 注意：不得把 video_url 写入 external_task_id——该字段必须保持为上游任务 ID
	//（提交时写入，供用户端复用查询），视频结果地址完整保存在 api_test_details.response_body 中。
	// 此前在此处覆盖为视频 URL，导致后续以 external_task_id 查询任务时报 404。

	if len(updates) > 0 {
		if err := repository.DB.Model(&apiTest).Updates(updates).Error; err != nil {
			logger.Error("[V1-Videos] Failed to update record: error=%v, record_id=%d", err, recordId)
		} else {
			logger.Debug("[V1-Videos] Successfully updated API Test Record: record_id=%d", recordId)
		}
	}

	// 更新 api_test_details 的 response_body（单次 Update 替代 First+Update 两趟往返；
	// RowsAffected=0 表示无 detail 记录，属正常情况）
	responseBody, _ := json.Marshal(data)
	res := repository.DB.Model(&models.ApiTestDetail{}).Where("request_id = ?", recordId).
		Update("response_body", string(responseBody))
	if res.Error != nil {
		logger.Error("[V1-Videos] Failed to update detail response_body: error=%v, record_id=%d", res.Error, recordId)
	} else if res.RowsAffected == 0 {
		logger.Debug("[V1-Videos] No API Test Detail found for record_id=%d", recordId)
	} else {
		logger.Debug("[V1-Videos] Successfully updated API Test Detail response_body")
	}

	// 更新 usage_logs 的 token 数据
	if apiTest.UsageLogID > 0 {
		logger.Debug("[V1-Videos] Updating usage_log: usage_log_id=%d", apiTest.UsageLogID)

		// 从返回数据中提取 usage 信息
		var promptTokens, completionTokens, totalTokens int
		foundUsage := false

		// 尝试从 data["usage"] 获取
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			logger.Debug("[V1-Videos] Extracting tokens from data.usage: %v", usage)
			foundUsage = true

			// 提取 completion_tokens
			if ct, ok := usage["completion_tokens"].(float64); ok {
				completionTokens = int(ct)
				logger.Debug("[V1-Videos] Found completion_tokens: %d", completionTokens)
			} else if ct, ok := usage["output_tokens"].(float64); ok {
				completionTokens = int(ct)
				logger.Debug("[V1-Videos] Found output_tokens: %d", completionTokens)
			}

			// 提取 total_tokens
			if tt, ok := usage["total_tokens"].(float64); ok {
				totalTokens = int(tt)
				logger.Debug("[V1-Videos] Found total_tokens: %d", totalTokens)
			}

			// 提取 prompt_tokens（可能没有，默认为 0）
			if pt, ok := usage["prompt_tokens"].(float64); ok {
				promptTokens = int(pt)
				logger.Debug("[V1-Videos] Found prompt_tokens: %d", promptTokens)
			} else if pt, ok := usage["input_tokens"].(float64); ok {
				promptTokens = int(pt)
				logger.Debug("[V1-Videos] Found input_tokens: %d", promptTokens)
			}
		}

		// 尝试从 data.data.usage 获取
		if !foundUsage {
			if dataMap, ok := data["data"].(map[string]interface{}); ok {
				if usage, ok := dataMap["usage"].(map[string]interface{}); ok {
					logger.Debug("[V1-Videos] Extracting tokens from data.data.usage: %v", usage)
					foundUsage = true
					if ct, ok := usage["completion_tokens"].(float64); ok {
						completionTokens = int(ct)
					}
					if tt, ok := usage["total_tokens"].(float64); ok {
						totalTokens = int(tt)
					}
					if pt, ok := usage["prompt_tokens"].(float64); ok {
						promptTokens = int(pt)
					}
				}
			}
		}

		// 如果还是没找到，记录警告
		if !foundUsage {
			logger.Warn("[V1-Videos] No usage information found in response, cannot update token counts")
		}

		// 计算 cost
		cost := utils.CalculateCost(promptTokens, completionTokens, model)
		logger.Debug("[V1-Videos] Calculated cost: cost=%f (input_price=%f, output_price=%f)",
			cost, model.InputPrice, model.OutputPrice)

		// 更新 usage_log
		usageUpdates := map[string]interface{}{
			"prompt_tokens":     promptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      totalTokens,
			"cost":              cost,
		}

		logger.Debug("[V1-Videos] Updating usage_log with: prompt=%d, completion=%d, total=%d, cost=%f",
			promptTokens, completionTokens, totalTokens, cost)

		if err := repository.DB.Model(&models.UsageLog{}).Where("id = ?", apiTest.UsageLogID).Updates(usageUpdates).Error; err != nil {
			logger.Error("[V1-Videos] Failed to update usage_log tokens: error=%v, usage_log_id=%d", err, apiTest.UsageLogID)
		} else {
			logger.Debug("[V1-Videos] Successfully updated usage_log tokens: usage_log_id=%d, prompt=%d, completion=%d, total=%d, cost=%f",
				apiTest.UsageLogID, promptTokens, completionTokens, totalTokens, cost)
		}
	} else {
		logger.Debug("[V1-Videos] No usage_log_id found in API test record")
	}

	logger.Debug("[V1-Videos] updateExistingRecord completed")
}
