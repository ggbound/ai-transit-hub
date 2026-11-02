package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/utils"
	"gorm.io/gorm"
)

const (
	// 每次处理的任务数量限制
	batchSize = 100
	// 并发处理的任务数量
	concurrency = 10
	// 任务超时时间
	taskTimeout = 4 * time.Minute
	// 只查询最近7天的任务
	daysToLookBack = 7
	// provider/model 解析缓存的 TTL
	resolveCacheTTL = 60 * time.Second
)

// VideoTaskPoller 视频任务轮询器
type VideoTaskPoller struct {
	resolveCacheMu sync.Mutex
	providerCache  map[string]cachedEntry[models.AiProvider] // key: provider code
	modelCache     map[string]cachedEntry[models.AiModel]    // key: provider+model 缓存键
}

// cachedEntry 带过期时间的进程内缓存条目
type cachedEntry[T any] struct {
	value   T
	expires time.Time
}

// NewVideoTaskPoller 创建视频任务轮询器
func NewVideoTaskPoller() *VideoTaskPoller {
	return &VideoTaskPoller{}
}

// Name 返回任务名称
func (v *VideoTaskPoller) Name() string {
	return "VideoTaskPoller"
}

// Interval 返回执行间隔
func (v *VideoTaskPoller) Interval() time.Duration {
	return 5 * time.Minute
}

// Timeout 返回任务超时时间
func (v *VideoTaskPoller) Timeout() time.Duration {
	return taskTimeout
}

// Run 执行任务
func (v *VideoTaskPoller) Run(ctx context.Context) error {
	logger.Info("[VideoTaskPoller] Starting video task polling")

	// 计算时间范围：只查询最近N天的任务
	cutoffTime := time.Now().AddDate(0, 0, -daysToLookBack)

	// 使用 id 游标分批处理：批处理会更新记录状态使其移出 WHERE 条件，
	// offset 分页会因此漂移导致跳过未处理记录
	lastID := uint(0)
	for {
		select {
		case <-ctx.Done():
			logger.Info("[VideoTaskPoller] Context cancelled, stopping")
			return ctx.Err()
		default:
		}

		// 查询一批任务
		var usageLogs []models.UsageLog
		err := repository.DB.Where("usage_logs.total_tokens = ? AND usage_logs.status = ?", 0, 1).
			Where("usage_logs.created_at >= ?", cutoffTime).
			Joins("JOIN api_test_requests ON api_test_requests.usage_log_id = usage_logs.id").
			Where("api_test_requests.type = ?", "video").
			Where("usage_logs.id > ?", lastID).
			Order("usage_logs.id ASC").
			Preload("Model").
			Limit(batchSize).
			Find(&usageLogs).Error

		if err != nil {
			logger.Error("[VideoTaskPoller] Failed to query usage logs (last_id: %d): %v", lastID, err)
			return err
		}

		if len(usageLogs) == 0 {
			break
		}

		logger.Info("[VideoTaskPoller] Processing batch of %d tasks (last_id: %d)", len(usageLogs), lastID)

		// 批内预取 api_test_requests：一次 IN 查询替代每任务一次单查
		logIDs := make([]uint, 0, len(usageLogs))
		for i := range usageLogs {
			logIDs = append(logIDs, usageLogs[i].ID)
		}
		var preloadedRequests []models.ApiTestRequest
		if err := repository.DB.Where("usage_log_id IN ?", logIDs).Order("id ASC").Find(&preloadedRequests).Error; err != nil {
			logger.Error("[VideoTaskPoller] Failed to preload api_test_requests: %v", err)
			return err
		}
		requestByLogID := make(map[uint]models.ApiTestRequest, len(preloadedRequests))
		for _, req := range preloadedRequests {
			// 保留首次出现（id 升序第一条），与原 First 查询语义一致
			if _, exists := requestByLogID[req.UsageLogID]; !exists {
				requestByLogID[req.UsageLogID] = req
			}
		}

		// 并发处理任务
		semaphore := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for i := range usageLogs {
			select {
			case <-ctx.Done():
				logger.Info("[VideoTaskPoller] Context cancelled during batch processing")
				wg.Wait()
				return ctx.Err()
			default:
			}

			semaphore <- struct{}{}
			wg.Add(1)

			go func(usageLog models.UsageLog) {
				defer wg.Done()
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						logger.Error("[VideoTaskPoller] Panic processing usage_log_id %d: %v", usageLog.ID, r)
					}
				}()

				v.processTask(ctx, &usageLog, requestByLogID)
			}(usageLogs[i])
		}

		wg.Wait()
		lastID = usageLogs[len(usageLogs)-1].ID
	}

	logger.Info("[VideoTaskPoller] Polling completed")
	return nil
}

// processTask 处理单个任务
func (v *VideoTaskPoller) processTask(ctx context.Context, usageLog *models.UsageLog, requestByLogID map[uint]models.ApiTestRequest) {
	logger.Info("[VideoTaskPoller] Processing usage_log_id: %d", usageLog.ID)

	// 查找对应的 api_test_request：优先用批内预取结果，miss 时兜底单查
	var apiTestRequest models.ApiTestRequest
	if req, ok := requestByLogID[usageLog.ID]; ok {
		apiTestRequest = req
	} else if err := repository.DB.Where("usage_log_id = ?", usageLog.ID).First(&apiTestRequest).Error; err != nil {
		logger.Error("[VideoTaskPoller] Failed to find api_test_request for usage_log_id %d: %v", usageLog.ID, err)
		return
	}

	// 检查是否有 external_task_id
	if apiTestRequest.ExternalTaskID == "" {
		logger.Debug("[VideoTaskPoller] No external_task_id found for api_test_request_id: %d", apiTestRequest.ID)
		return
	}

	// 解析 model 字段获取 provider 和 model
	modelParts := strings.Split(apiTestRequest.Model, "/")
	if len(modelParts) < 1 {
		logger.Error("[VideoTaskPoller] Invalid model format: %s", apiTestRequest.Model)
		return
	}

	providerCode := modelParts[0]
	modelCode := ""
	if len(modelParts) >= 2 {
		modelCode = modelParts[1]
	}

	// 查找 provider（带 60s TTL 进程内缓存）
	provider, err := v.getCachedProvider(providerCode)
	if err != nil {
		logger.Error("[VideoTaskPoller] Failed to find provider: %s, error: %v", providerCode, err)
		return
	}

	// 查找 model（带 60s TTL 进程内缓存；无 model code 时取该 provider 的第一个启用视频模型）
	model, err := v.getCachedModel(provider, modelCode)
	if err != nil {
		logger.Error("[VideoTaskPoller] Failed to find model: %s/%s, error: %v", providerCode, modelCode, err)
		return
	}

	// 获取随机 API key
	apiKey, err := ai.GetRandomApiKey(provider.ID)
	if err != nil {
		logger.Error("[VideoTaskPoller] Failed to get API key for provider: %s, error: %v", providerCode, err)
		return
	}

	// 创建适配器
	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(&provider, apiKey.ApiKey, &model)
	if err != nil {
		logger.Error("[VideoTaskPoller] Failed to create adapter: %v", err)
		return
	}

	// 查询任务状态
	logger.Info("[VideoTaskPoller] Querying video task: %s", apiTestRequest.ExternalTaskID)
	data, err := adapter.GetVideoTask(ctx, apiTestRequest.ExternalTaskID)
	if err != nil {
		logger.Error("[VideoTaskPoller] Failed to query task: %v", err)
		return
	}

	// 记录响应
	responseJSON, _ := json.Marshal(data)
	logger.Debug("[VideoTaskPoller] Task response: %s", string(responseJSON))

	// 检查任务状态
	status, ok := data["status"].(string)
	if !ok {
		logger.Warn("[VideoTaskPoller] No status found in response")
		return
	}

	// 只有当状态是 succeeded 时才更新
	if status != "succeeded" {
		logger.Debug("[VideoTaskPoller] Task not yet succeeded, status: %s", status)
		return
	}

	logger.Info("[VideoTaskPoller] Task succeeded, updating records")

	// 提取 token 信息
	promptTokens, completionTokens, totalTokens := extractTokenUsageFromData(data)

	// 计算 cost
	cost := utils.CalculateCost(promptTokens, completionTokens, &model)

	logger.Info("[VideoTaskPoller] Token usage - prompt: %d, completion: %d, total: %d, cost: %f",
		promptTokens, completionTokens, totalTokens, cost)

	// 使用事务更新多个表
	tx := repository.DB.Begin()
	if tx.Error != nil {
		logger.Error("[VideoTaskPoller] Failed to begin transaction: %v", tx.Error)
		return
	}

	// 更新 usage_log
	usageUpdates := map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      totalTokens,
		"cost":              cost,
	}

	if err := tx.Model(usageLog).Updates(usageUpdates).Error; err != nil {
		logger.Error("[VideoTaskPoller] Failed to update usage_log: %v", err)
		tx.Rollback()
		return
	}

	// 注意：不得用 videoUrl 覆盖 external_task_id——该字段必须保持为上游任务 ID
	//（提交时写入，供轮询查询与用户端复用），视频结果地址完整保存在 api_test_details.response_body 中。
	// 此前在此处覆盖为视频 URL，导致后续以 external_task_id 查询任务时报 404。

	// 更新 api_test_details 的 response_body
	var detail models.ApiTestDetail
	err = tx.Where("request_id = ?", apiTestRequest.ID).First(&detail).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 无 detail 记录属正常情况，跳过更新
		logger.Warn("[VideoTaskPoller] No api_test_detail found for request_id %d, skipping response_body update", apiTestRequest.ID)
	} else if err != nil {
		logger.Error("[VideoTaskPoller] Failed to query api_test_detail for request_id %d: %v", apiTestRequest.ID, err)
		tx.Rollback()
		return
	} else if err := tx.Model(&detail).Update("response_body", string(responseJSON)).Error; err != nil {
		logger.Error("[VideoTaskPoller] Failed to update api_test_detail: %v", err)
		tx.Rollback()
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Error("[VideoTaskPoller] Failed to commit transaction: %v", err)
		return
	}

	logger.Info("[VideoTaskPoller] Successfully updated usage_log_id: %d", usageLog.ID)
}

// getCachedProvider 按 code 获取 provider，带 60s TTL 进程内缓存（只缓存命中，不缓存未命中）
func (v *VideoTaskPoller) getCachedProvider(code string) (models.AiProvider, error) {
	v.resolveCacheMu.Lock()
	defer v.resolveCacheMu.Unlock()

	if v.providerCache == nil {
		v.providerCache = make(map[string]cachedEntry[models.AiProvider])
	}
	if entry, ok := v.providerCache[code]; ok && time.Now().Before(entry.expires) {
		return entry.value, nil
	}

	var provider models.AiProvider
	if err := repository.DB.Where("code = ?", code).First(&provider).Error; err != nil {
		return provider, err
	}
	v.providerCache[code] = cachedEntry[models.AiProvider]{value: provider, expires: time.Now().Add(resolveCacheTTL)}
	return provider, nil
}

// getCachedModel 按 provider+code 获取 model，带 60s TTL 进程内缓存（只缓存命中，不缓存未命中）；
// modelCode 为空时回退为该 provider 的第一个启用视频模型
func (v *VideoTaskPoller) getCachedModel(provider models.AiProvider, modelCode string) (models.AiModel, error) {
	key := fmt.Sprintf("code:%d:%s", provider.ID, modelCode)
	if modelCode == "" {
		key = fmt.Sprintf("videodefault:%d", provider.ID)
	}

	v.resolveCacheMu.Lock()
	defer v.resolveCacheMu.Unlock()

	if v.modelCache == nil {
		v.modelCache = make(map[string]cachedEntry[models.AiModel])
	}
	if entry, ok := v.modelCache[key]; ok && time.Now().Before(entry.expires) {
		return entry.value, nil
	}

	var model models.AiModel
	var err error
	if modelCode != "" {
		err = repository.DB.Where("provider_id = ? AND code = ?", provider.ID, modelCode).First(&model).Error
	} else {
		err = repository.DB.Where("provider_id = ? AND type = ? AND status = ?", provider.ID, "video", 1).First(&model).Error
	}
	if err != nil {
		return model, err
	}
	v.modelCache[key] = cachedEntry[models.AiModel]{value: model, expires: time.Now().Add(resolveCacheTTL)}
	return model, nil
}

// extractTokenUsageFromData 从响应数据中提取 token 使用量
func extractTokenUsageFromData(data map[string]interface{}) (promptTokens, completionTokens, totalTokens int) {
	promptTokens = 0
	completionTokens = 0
	totalTokens = 0

	// 尝试从 data["usage"] 获取
	if usage, ok := data["usage"].(map[string]interface{}); ok {
		promptTokens, completionTokens, totalTokens = extractUsageFromMap(usage)
		if totalTokens > 0 {
			return
		}
	}

	// 尝试从 data["data"]["usage"] 获取
	if dataMap, ok := data["data"].(map[string]interface{}); ok {
		if usage, ok := dataMap["usage"].(map[string]interface{}); ok {
			promptTokens, completionTokens, totalTokens = extractUsageFromMap(usage)
			if totalTokens > 0 {
				return
			}
		}
	}

	return
}

// extractUsageFromMap 从 usage map 中提取 tokens
func extractUsageFromMap(usage map[string]interface{}) (promptTokens, completionTokens, totalTokens int) {
	promptTokens = 0
	completionTokens = 0
	totalTokens = 0

	// 提取 prompt tokens
	if v, ok := usage["prompt_tokens"].(float64); ok {
		promptTokens = int(v)
	} else if v, ok := usage["input_tokens"].(float64); ok {
		promptTokens = int(v)
	}

	// 提取 completion tokens
	if v, ok := usage["completion_tokens"].(float64); ok {
		completionTokens = int(v)
	} else if v, ok := usage["output_tokens"].(float64); ok {
		completionTokens = int(v)
	}

	// 提取 total tokens
	if v, ok := usage["total_tokens"].(float64); ok {
		totalTokens = int(v)
	} else {
		totalTokens = promptTokens + completionTokens
	}

	return promptTokens, completionTokens, totalTokens
}
