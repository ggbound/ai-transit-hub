package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	log "github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/metrics"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/service"
	"github.com/your-org/ai-transit-hub/internal/utils"
)

// 本文件抽取 6 个 V1 透传 handler（chat/messages/responses/images/videos/embeddings）
// 共用的请求流水线，消除各 handler 中重复的：
//   - OpenAI 风格错误响应组装
//   - provider/model 解析与校验
//   - API Key 选择、用户信息提取、初始记录创建、适配器创建
//   - 非流式/流式的错误检测、记账与响应写出

// respondV1Error 统一返回 OpenAI 风格错误：{"error": {"message", "type", "param"?, "code"?}}
// param 为空时不输出 param 字段；code 为 nil 时不输出 code 字段
func respondV1Error(c *gin.Context, status int, message, errType, param string, code interface{}) {
	errBody := gin.H{
		"message": message,
		"type":    errType,
	}
	if param != "" {
		errBody["param"] = param
	}
	if code != nil {
		errBody["code"] = code
	}
	c.JSON(status, gin.H{"error": errBody})
}

// parseV1Model 解析并校验 V1 请求的 model 字段（仅支持 "provider/model" 格式）。
// 解析统一走 ai.GetProviderAndModel（带 60s TTL 缓存，省去每请求 2 次 DB 往返）。
// 校验失败时已向客户端写出错误响应，调用方直接 return。
//   - 缺少 "/"           -> 400 invalid_model_format
//   - provider 不存在    -> 404 provider_not_found
//   - model 不存在       -> 404 model_not_found
func parseV1Model(c *gin.Context, modelID string) (*models.AiProvider, *models.AiModel, bool) {
	provider, model, err := ai.GetProviderAndModel(modelID)
	if err == nil {
		return provider, model, true
	}

	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "provider '"):
		respondV1Error(c, http.StatusNotFound,
			msg,
			"invalid_request_error", "model", "provider_not_found")
	case strings.HasPrefix(msg, "model '"):
		respondV1Error(c, http.StatusNotFound,
			msg,
			"invalid_request_error", "model", "model_not_found")
	default:
		// 缺少 "/" 等格式错误
		respondV1Error(c, http.StatusBadRequest,
			msg,
			"invalid_request_error", "model", "invalid_model_format")
	}
	return nil, nil, false
}

// v1PassthroughAdapter 是 V1 管线实际依赖的适配器能力（消费者侧小接口），
// 解耦对 ai.Adapter 胖接口的依赖：适配器新增能力不影响 V1 管线，也便于测试替身。
type v1PassthroughAdapter interface {
	// PassthroughRequest 透传请求（上游非 2xx 时返回 *UpstreamError）
	PassthroughRequest(ctx context.Context, requestData map[string]interface{}, requestType string) (map[string]interface{}, error)
	// PassthroughStreamRequest 流式透传请求（上游非 2xx 时在任何 callback 前返回 *UpstreamError）
	PassthroughStreamRequest(ctx context.Context, requestData map[string]interface{}, callback func(string), requestType string) error
	// ExtractTokenUsage 从响应中提取 token 使用量
	ExtractTokenUsage(response map[string]interface{}) (int, int, int)
	// Embeddings 简单文本输入的 embeddings 处理（V1-Embeddings 简单路径使用）
	Embeddings(ctx context.Context, inputs []string, options map[string]interface{}) (map[string]interface{}, error)
}

// v1Context 携带一次 V1 透传请求的公共上下文
type v1Context struct {
	Provider  *models.AiProvider
	Model     *models.AiModel
	RecordIDs *service.RecordIDs
	Adapter   v1PassthroughAdapter
}

// prepareV1Request 完成 GetRandomApiKey -> 用户信息提取 -> CreateInitialRecords -> 适配器创建。
// 任一步骤失败时已写出错误响应，调用方直接 return。
// 参数：
//   - requestData: 透传请求体（用于错误记录回填）
//   - apiType/endpoint/testType: CreateInitialRecords 的记账维度
//   - requestType: ValidateModelType 的模型类型校验维度
//   - tag: 日志前缀，如 "[V1-Chat]"
func prepareV1Request(c *gin.Context, provider *models.AiProvider, model *models.AiModel, requestData map[string]interface{},
	apiType, endpoint, testType, requestType, tag string) (*v1Context, bool) {

	// 验证模型类型是否匹配
	if !ValidateModelType(c, model, requestType) {
		return nil, false
	}

	// 获取 API Key（在可用 Key 中随机选择，实现多 Key 负载分摊）
	apiKey, err := ai.GetRandomApiKey(provider.ID)
	if err != nil {
		log.Error("%s No active API keys found for provider_id=%d: %v", tag, provider.ID, err)
		respondV1Error(c, http.StatusInternalServerError,
			err.Error(),
			"api_error", "api_key", "no_api_key")
		return nil, false
	}

	// 获取当前用户和 API Key ID
	user, _ := GetAuthUser(c)
	var userID uint = 0
	if user != nil {
		userID = user.ID
	}

	// 从 context 获取 API Key ID（由 ApiKeyAuthMiddleware 设置）
	userApiKeyID := uint(0)
	if id, exists := c.Get("api_key_id"); exists {
		if idUint, ok := id.(uint); ok {
			userApiKeyID = idUint
		}
	}

	// 创建初始记录（请求开始前）
	recordIDs, err := service.Accounting.CreateInitialRecords(
		userID,
		model,
		provider,
		apiType,
		endpoint,
		userApiKeyID,
		0,  // sessionID
		"", // externalTaskID
		testType,
	)
	if err != nil {
		log.Error("%s Failed to create initial records: %v", tag, err)
		respondV1Error(c, http.StatusInternalServerError,
			"failed to create request records",
			"api_error", "server", "record_create_failed")
		return nil, false
	}

	// -------------------- 使用 AdapterFactory 创建适配器 --------------------
	log.Debug("%s 开始处理请求, model=%s, provider=%s", tag, model.Code, provider.Code)

	// 获取 AdapterClass，默认是 generic
	adapterClass := provider.AdapterClass
	if adapterClass == "" {
		adapterClass = "generic"
	}
	log.Debug("%s Provider AdapterClass: %s", tag, adapterClass)

	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		log.Error("%s 适配器创建失败: %v", tag, err)
		service.Accounting.UpdateErrorRecords(
			recordIDs,
			500,
			"Failed to create adapter: "+err.Error(),
			"POST",
			c.Request.URL.String(),
			c.Request.Header,
			requestData,
		)
		respondV1Error(c, http.StatusInternalServerError,
			"Adapter creation failed: "+err.Error(),
			"api_error", "adapter", "adapter_creation_failed")
		return nil, false
	}

	// 记录适配器信息
	adapterProvider := adapter.GetProvider()
	adapterModel := adapter.GetModelConfig()
	log.Debug("%s 适配器创建成功", tag)
	log.Debug("%s Adapter Provider: %s (AdapterClass=%s)", tag, adapterProvider.Code, adapterProvider.AdapterClass)
	log.Debug("%s Adapter Model: %s", tag, adapterModel.Code)
	// ------------------------------------------------------------------------

	return &v1Context{
		Provider:  provider,
		Model:     model,
		RecordIDs: recordIDs,
		Adapter:   adapter,
	}, true
}

// respondV1Success 非流式成功路径：错误检测 -> 写出响应 -> 记账。
// 记账（2 SELECT + 2 UPDATE + 1 INSERT）移到响应写出之后，避免数据库往返阻塞 TTFB。
func respondV1Success(c *gin.Context, v *v1Context, reqData, response map[string]interface{}, tag string) bool {
	// 检查响应是否包含错误
	if isError, errMsg := utils.IsErrorResponse(response); isError {
		log.Error("%s API returned error response: %s", tag, errMsg)
		// error 为对象形状时已是标准信封，原样透传保真；其余形状（error 字符串、
		// success:false、status:error 等）包装为统一 OpenAI 信封，避免内部风格泄漏
		if _, isObj := response["error"].(map[string]interface{}); isObj {
			c.JSON(http.StatusInternalServerError, response)
		} else {
			respondV1Error(c, http.StatusInternalServerError, errMsg, "api_error", "", "upstream_error")
		}
		service.Accounting.UpdateErrorRecords(
			v.RecordIDs,
			500,
			errMsg,
			"POST",
			c.Request.URL.String(),
			c.Request.Header,
			reqData,
		)
		return false
	}

	c.JSON(http.StatusOK, response)

	// 提取 token 使用量并更新成功记录（响应已写出，不阻塞客户端）
	promptTokens, completionTokens, totalTokens := v.Adapter.ExtractTokenUsage(response)
	cost := utils.CalculateCost(promptTokens, completionTokens, v.Model)
	observeTokens(v.Model.Code, promptTokens, completionTokens, totalTokens)
	service.Accounting.UpdateSuccessRecords(
		v.RecordIDs,
		200,
		response,
		promptTokens,
		completionTokens,
		totalTokens,
		cost,
		"POST",
		c.Request.URL.String(),
		c.Request.Header,
		reqData,
		v.Model,
	)
	return true
}

// observeTokens 记录 token 消耗到 Prometheus（prompt/completion/total 三个 kind）
func observeTokens(model string, promptTokens, completionTokens, totalTokens int) {
	metrics.TokensTotal.WithLabelValues(model, "prompt").Add(float64(promptTokens))
	metrics.TokensTotal.WithLabelValues(model, "completion").Add(float64(completionTokens))
	metrics.TokensTotal.WithLabelValues(model, "total").Add(float64(totalTokens))
}

// executeV1NonStream 非流式透传的统一执行：请求 -> 错误处理 -> 错误检测/记账/响应
func executeV1NonStream(c *gin.Context, v *v1Context, reqData map[string]interface{}, requestType, tag string) {
	start := time.Now()
	response, err := v.Adapter.PassthroughRequest(c.Request.Context(), reqData, requestType)
	metrics.UpstreamDuration.WithLabelValues(v.Provider.Code, requestType).Observe(time.Since(start).Seconds())
	if err != nil {
		// 上游错误透传真实状态码与错误体
		metrics.V1RequestsTotal.WithLabelValues(requestType, "error").Inc()
		handlePassthroughError(c, err, v.RecordIDs, "POST", c.Request.URL.String(), c.Request.Header, reqData)
		return
	}
	if respondV1Success(c, v, reqData, response, tag) {
		metrics.V1RequestsTotal.WithLabelValues(requestType, "success").Inc()
	} else {
		metrics.V1RequestsTotal.WithLabelValues(requestType, "error").Inc()
	}
}

// executeV1Stream 流式透传的统一执行：relay 逐行透传 -> 错误处理 -> 记账。
// usageExtract 为自定义 usage 提取回调（decoded 为解析后的 SSE JSON 块，
// finalUsage 指向累积的 usage 对象），nil 时默认提取顶层 usage 字段。
func executeV1Stream(c *gin.Context, v *v1Context, reqData map[string]interface{}, requestType, tag string,
	usageExtract func(decoded map[string]interface{}, finalUsage *map[string]interface{})) {

	// 流式响应处理（SSE 头由 relay 在首次写出时惰性设置，
	// 上游在流出数据前失败时仍可返回真实状态码的 JSON 错误）
	relay := newStreamRelay(c)
	var finalUsage map[string]interface{}

	start := time.Now()
	err := v.Adapter.PassthroughStreamRequest(c.Request.Context(), reqData, func(line string) {
		relay.OnLine(line, func(decoded map[string]interface{}) {
			if usageExtract != nil {
				usageExtract(decoded, &finalUsage)
			} else if usage, ok := decoded["usage"].(map[string]interface{}); ok {
				finalUsage = usage
			}
		})
	}, requestType)
	upstreamElapsed := time.Since(start)

	if err != nil {
		promptTokens, completionTokens, totalTokens := extractUsageTokens(finalUsage)
		hasUsage := promptTokens > 0 || completionTokens > 0 || totalTokens > 0

		// 客户端主动断开（context canceled）：常见于客户端收完完整内容后提前 abort 连接，
		// 而上游连接尚未关闭，后端继续等 EOF 的那次读被取消。这不是上游故障，需与真实错误区分。
		if isClientDisconnected(err) {
			metrics.UpstreamDuration.WithLabelValues(v.Provider.Code, requestType).Observe(upstreamElapsed.Seconds())
			if hasUsage || relay.GotDone {
				// 流已完整送达（已透传 [DONE] 或已捕获 usage 帧）：按成功记账，不计入错误
				metrics.V1RequestsTotal.WithLabelValues(requestType, "success").Inc()
				log.Warn("%s Client disconnected after stream completion (context canceled), recorded as success", tag)
				cost := utils.CalculateCost(promptTokens, completionTokens, v.Model)
				observeTokens(v.Model.Code, promptTokens, completionTokens, totalTokens)
				service.Accounting.UpdateSuccessRecords(
					v.RecordIDs,
					200,
					map[string]interface{}{
						"usage":      finalUsage,
						"raw_stream": relay.AllLines(),
					},
					promptTokens,
					completionTokens,
					totalTokens,
					cost,
					"POST",
					c.Request.URL.String(),
					c.Request.Header,
					reqData,
					v.Model,
				)
				return
			}
			// 流中途断开：按 499 记录（客户端关闭请求），与真实上游故障（5xx）区分开
			metrics.V1RequestsTotal.WithLabelValues(requestType, "error").Inc()
			log.Warn("%s Client disconnected mid-stream (context canceled)", tag)
			relay.HandleStreamError(fmt.Errorf("client disconnected mid-stream: %w", err),
				v.RecordIDs, "POST", c.Request.URL.String(), c.Request.Header, reqData)
			return
		}

		// 上游连接/流中失败：断开前上游已产生的 token 用量仍需入账，不能因请求失败而丢失记账
		metrics.V1RequestsTotal.WithLabelValues(requestType, "error").Inc()
		metrics.UpstreamDuration.WithLabelValues(v.Provider.Code, requestType).Observe(upstreamElapsed.Seconds())
		if hasUsage {
			log.Error("%s Stream request failed after partial output: %s", tag, err.Error())
			cost := utils.CalculateCost(promptTokens, completionTokens, v.Model)
			observeTokens(v.Model.Code, promptTokens, completionTokens, totalTokens)
			service.Accounting.UpdateErrorRecordsWithUsage(
				v.RecordIDs,
				passthroughErrStatus(err),
				err.Error(),
				promptTokens,
				completionTokens,
				totalTokens,
				cost,
				"POST",
				c.Request.URL.String(),
				c.Request.Header,
				reqData,
			)
			return
		}
		relay.HandleStreamError(err, v.RecordIDs, "POST", c.Request.URL.String(), c.Request.Header, reqData)
		return
	}

	// 流中途收到错误数据块：已产生的 token 用量仍需入账
	if relay.HasError {
		log.Error("%s Stream API returned error response: %s", tag, relay.ErrMsg)
		metrics.V1RequestsTotal.WithLabelValues(requestType, "error").Inc()
		promptTokens, completionTokens, totalTokens := extractUsageTokens(finalUsage)
		observeTokens(v.Model.Code, promptTokens, completionTokens, totalTokens)
		cost := utils.CalculateCost(promptTokens, completionTokens, v.Model)
		service.Accounting.UpdateErrorRecordsWithUsage(
			v.RecordIDs,
			500,
			relay.ErrMsg,
			promptTokens,
			completionTokens,
			totalTokens,
			cost,
			"POST",
			c.Request.URL.String(),
			c.Request.Header,
			reqData,
		)
		return
	}

	// 构建完整响应（仅保留原始流与 usage）
	fullResponse := map[string]interface{}{
		"usage":      finalUsage,
		"raw_stream": relay.AllLines(),
	}

	// 提取 token 使用量（无 usage 时 extractUsageTokens 返回 0，仍保存完整流）
	promptTokens, completionTokens, totalTokens := extractUsageTokens(finalUsage)
	cost := utils.CalculateCost(promptTokens, completionTokens, v.Model)
	metrics.V1RequestsTotal.WithLabelValues(requestType, "success").Inc()
	metrics.UpstreamDuration.WithLabelValues(v.Provider.Code, requestType).Observe(upstreamElapsed.Seconds())
	observeTokens(v.Model.Code, promptTokens, completionTokens, totalTokens)

	service.Accounting.UpdateSuccessRecords(
		v.RecordIDs,
		200,
		fullResponse,
		promptTokens,
		completionTokens,
		totalTokens,
		cost,
		"POST",
		c.Request.URL.String(),
		c.Request.Header,
		reqData,
		v.Model,
	)
}
