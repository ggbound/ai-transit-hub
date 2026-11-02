package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/your-org/ai-transit-hub/internal/cache"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// upstreamTransport 共享的 Transport：显式配置连接池参数，
// 避免 Go 默认值（MaxIdleConns=100、MaxIdleConnsPerHost=2）在高并发下频繁重建 TCP+TLS 连接。
var upstreamTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// upstreamHTTPClient 包级共享的 HTTP 客户端：所有适配器复用同一连接池，
// 避免每次创建适配器都新建 client 导致连接无法复用。
// 超时 10 分钟，覆盖视频生成等长耗时任务。非流式请求专用（整体超时兜底）。
var upstreamHTTPClient = &http.Client{
	Timeout:   600 * time.Second,
	Transport: upstreamTransport,
}

const (
	// streamReadIdleTimeout 流式读空闲超时：单次 Read 阻塞期间超过该时长无任何字节到达
	// 则中止请求（视作瞬时错误，由调用方重试）。回调处理耗时不算空闲。
	streamReadIdleTimeout = 180 * time.Second
	// streamOverallTimeout 流式请求总时长上限：兜底防止病态慢流无限占用连接。
	streamOverallTimeout = 30 * time.Minute
)

// streamHTTPClient 专用于流式（SSE）请求：不设整体 Client.Timeout——整体超时会在
// 读 body 阶段把健康的长生成流式连接整条掐断（报 context deadline exceeded while
// reading body），改为 ResponseHeaderTimeout + 读空闲看门狗（idleCancelReader）
// + 总时长上限三重兜底。
var streamHTTPClient = func() *http.Client {
	tr := upstreamTransport.Clone()
	tr.ResponseHeaderTimeout = 120 * time.Second
	return &http.Client{Transport: tr}
}()

type GenericAdapter struct {
	client      *http.Client
	provider    *models.AiProvider
	apiKey      *models.ProviderApiKey
	modelConfig *models.AiModel
}

func NewGenericAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *GenericAdapter {
	// 使用工厂传入的 API Key（与各具名适配器行为一致）。
	// 修复原先忽略传入 Key、直接查库取第一个 Key 导致多 Key 负载均衡失效的问题
	var providerKey *models.ProviderApiKey
	if apiKey != "" {
		providerKey = &models.ProviderApiKey{
			AiProviderID: provider.ID,
			ApiKey:       apiKey,
			Status:       1,
		}
	}

	return &GenericAdapter{
		client:      upstreamHTTPClient,
		provider:    provider,
		apiKey:      providerKey,
		modelConfig: modelConfig,
	}
}

func parseJSON(j models.JSON) (map[string]interface{}, error) {
	if len(j) == 0 {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	err := json.Unmarshal(j, &result)
	if err != nil {
		return map[string]interface{}{}, err
	}
	return result, nil
}

// buildUrl builds complete request URL (same as PHP buildUrl)
func (a *GenericAdapter) buildUrl(endpoint string, requestType string) string {
	// 方案 1：优先使用简化的 custom_endpoint（推荐）
	if a.modelConfig.CustomEndpoint != "" {
		modelEndpoint := a.modelConfig.CustomEndpoint

		// 检查是否是完整 URL
		if !isValidURL(modelEndpoint) {
			// 只是路径，与提供商 base_url 拼接
			baseUrl := a.provider.BaseUrl
			finalUrl := strings.TrimRight(baseUrl, "/") + "/" + strings.TrimLeft(modelEndpoint, "/")
			logger.Info("Using model custom_endpoint (path): type=%s, model_code=%s, base_url=%s, endpoint=%s, final_url=%s", requestType, a.modelConfig.Code, baseUrl, modelEndpoint, finalUrl)
			return finalUrl
		}

		// 完整 URL
		logger.Info("Using model custom_endpoint (full URL): type=%s, model_code=%s, endpoint=%s", requestType, a.modelConfig.Code, modelEndpoint)
		return modelEndpoint
	}

	// 方案 2：兼容旧的类型特定端点（向后兼容；image/images、video/videos 为同一语义的两种写法）
	var modelEndpoint string
	switch requestType {
	case "embeddings":
		modelEndpoint = a.modelConfig.EmbeddingsEndpoint
	case "image", "images":
		modelEndpoint = a.modelConfig.ImageEndpoint
	case "video", "videos":
		modelEndpoint = a.modelConfig.VideoEndpoint
	case "audio":
		modelEndpoint = a.modelConfig.AudioEndpoint
	case "response":
		modelEndpoint = a.modelConfig.ResponseEndpoint
	case "messages":
		modelEndpoint = a.modelConfig.MessagesEndpoint
	case "chat":
		fallthrough
	default:
		modelEndpoint = a.modelConfig.ChatEndpoint
	}

	if modelEndpoint != "" {
		if !isValidURL(modelEndpoint) {
			baseUrl := a.provider.BaseUrl
			finalUrl := strings.TrimRight(baseUrl, "/") + "/" + strings.TrimLeft(modelEndpoint, "/")
			logger.Info("Using model-level endpoint with base_url: type=%s, model_code=%s, base_url=%s, endpoint=%s, final_url=%s", requestType, a.modelConfig.Code, baseUrl, modelEndpoint, finalUrl)
			return finalUrl
		}

		logger.Info("Using model-level endpoint (full URL): type=%s, model_code=%s, endpoint=%s", requestType, a.modelConfig.Code, modelEndpoint)
		return modelEndpoint
	}

	// 回退到提供商的 request_format
	baseUrl := a.provider.BaseUrl

	// 根据功能类型选择对应的端点配置（模型级端点缺失时按类型回退到 provider 同类配置，
	// 而非一律 chat_endpoint——image/video 请求落 chat/completions 必然失败）
	var endpointConfigKey string
	switch requestType {
	case "embeddings":
		endpointConfigKey = "embeddings_endpoint"
	case "image", "images":
		endpointConfigKey = "images_endpoint"
	case "video", "videos":
		endpointConfigKey = "videos_endpoint"
	case "response":
		endpointConfigKey = "response_endpoint"
	default:
		endpointConfigKey = "chat_endpoint"
	}

	// 获取 request_format
	requestFormat, _ := parseJSON(a.provider.RequestFormat)
	customEndpoint, _ := requestFormat[endpointConfigKey].(string)

	if customEndpoint != "" {
		if isValidURL(customEndpoint) {
			logger.Info("Using custom %s endpoint as full URL: %s", requestType, customEndpoint)
			return customEndpoint
		}

		finalUrl := strings.TrimRight(baseUrl, "/") + "/" + strings.TrimLeft(customEndpoint, "/")
		logger.Info("Using custom %s endpoint with base url: %s", requestType, finalUrl)
		return finalUrl
	}

	// 如果 base_url 已经包含特定端点，直接使用
	if requestType == "chat" && strings.Contains(baseUrl, "/chat/completions") {
		logger.Info("Base url already contains endpoint, using directly: %s", baseUrl)
		return baseUrl
	}

	// 默认方式拼接
	finalUrl := strings.TrimRight(baseUrl, "/") + "/" + strings.TrimLeft(endpoint, "/")
	logger.Info("Using default url construction: %s (base_url: %s, endpoint: %s, type: %s)", finalUrl, baseUrl, endpoint, requestType)
	return finalUrl
}

func (a *GenericAdapter) buildHeaders() map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// 添加自定义请求头
	if a.modelConfig.CustomHeaders != nil {
		customHeaders, _ := parseJSON(a.modelConfig.CustomHeaders)
		for key, value := range customHeaders {
			if strVal, ok := value.(string); ok {
				headers[key] = strVal
			}
		}
	}

	// 根据认证类型添加认证信息
	authType := a.provider.AuthType
	if authType == "" {
		authType = "bearer"
	}

	switch authType {
	case "bearer":
		if a.apiKey != nil {
			headers["Authorization"] = "Bearer " + a.apiKey.ApiKey
		}
	case "api_key":
		if a.apiKey != nil {
			headers["Authorization"] = a.apiKey.ApiKey
		}
	case "api_key_header":
		authConfig, _ := parseJSON(a.provider.AuthConfig)
		headerName, _ := authConfig["header_name"].(string)
		if headerName == "" {
			headerName = "X-API-Key"
		}
		if a.apiKey != nil {
			headers[headerName] = a.apiKey.ApiKey
		}
	}

	return headers
}

// resolveEndpoint 根据 requestType 从 provider 的 request_format 中解析上游端点路径，
// 未配置时回退到各类型的默认路径。chat 及未知类型统一走 chat_endpoint。
// image/images、video/videos 为同一语义的两种写法（调用方两种都有使用）
func resolveEndpoint(requestType string, requestFormat map[string]interface{}) string {
	var formatKey, fallback string
	switch requestType {
	case "embeddings":
		formatKey, fallback = "embeddings_endpoint", "/embeddings"
	case "image", "images":
		formatKey, fallback = "images_endpoint", "/images/generations"
	case "video", "videos":
		formatKey, fallback = "videos_endpoint", "/contents/generations/tasks"
	case "response":
		formatKey, fallback = "response_endpoint", "/responses"
	case "messages":
		formatKey, fallback = "messages_endpoint", "/v1/messages"
	default: // chat 与未知类型
		formatKey, fallback = "chat_endpoint", "/chat/completions"
	}

	if endpoint, _ := requestFormat[formatKey].(string); endpoint != "" {
		return endpoint
	}
	return fallback
}

// newUpstreamRequest 构建上游 POST 请求：解析端点、记录请求日志、序列化请求体并附加认证头。
// PassthroughRequest / PassthroughStreamRequest / PassthroughStreamRequestRaw 共用此前置流程。
func (a *GenericAdapter) newUpstreamRequest(ctx context.Context, requestData map[string]interface{}, requestType string) (*http.Request, error) {
	requestFormat, _ := parseJSON(a.provider.RequestFormat)
	endpoint := resolveEndpoint(requestType, requestFormat)
	fullUrl := a.buildUrl(endpoint, requestType)

	// 打印实际请求的地址信息
	modelInfo := fmt.Sprintf("Provider: %s, Model: %s", a.provider.Code, a.modelConfig.Code)
	if modelCode, ok := requestData["model"].(string); ok {
		modelInfo = fmt.Sprintf("Provider: %s, Model: %s, Request Model: %s", a.provider.Code, a.modelConfig.Code, modelCode)
	}
	logger.Debug(fmt.Sprintf("[API-REQUEST-URL] %s | URL: %s | Type: %s", modelInfo, fullUrl, requestType))

	jsonBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	for k, v := range a.buildHeaders() {
		req.Header.Add(k, v)
	}

	return req, nil
}

func (a *GenericAdapter) PassthroughRequest(ctx context.Context, requestData map[string]interface{}, requestType string) (map[string]interface{}, error) {
	req, err := a.newUpstreamRequest(ctx, requestData, requestType)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if uerr := json.Unmarshal(body, &result); uerr != nil {
		// 响应体不是合法 JSON：区分上游错误与异常成功响应，避免把错误静默吞掉
		if resp.StatusCode >= 400 {
			return nil, &UpstreamError{StatusCode: resp.StatusCode, Raw: truncateForLog(body)}
		}
		return nil, fmt.Errorf("invalid JSON response from upstream (status %d)", resp.StatusCode)
	}

	// 透传上游真实状态码，由调用方决定如何映射给客户端
	if resp.StatusCode >= 400 {
		return result, &UpstreamError{StatusCode: resp.StatusCode, Body: result}
	}

	return result, nil
}

func (a *GenericAdapter) PassthroughStreamRequest(ctx context.Context, requestData map[string]interface{}, callback func(string), requestType string) error {
	req, err := a.newUpstreamRequest(ctx, requestData, requestType)
	if err != nil {
		return err
	}

	// 流式不走整体超时 client：总时长上限挂在 ctx 上，读空闲由看门狗中止
	sctx, cancel := context.WithTimeout(ctx, streamOverallTimeout)
	defer cancel()
	req = req.WithContext(sctx)

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 上游返回非 2xx：读取错误体直接返回，不触发任何 callback，
	// 调用方此时尚未向客户端写出任何数据，可以返回真实的 HTTP 状态码
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var parsed map[string]interface{}
		json.Unmarshal(errBody, &parsed)
		return &UpstreamError{StatusCode: resp.StatusCode, Body: parsed, Raw: truncateForLog(errBody)}
	}

	// 原始字节流透传，同时解析内容用于日志；
	// 包一层空闲看门狗：阻塞读期间持续无字节到达则中止（触发调用方重试）
	reader := bufio.NewReader(newIdleCancelReader(resp.Body, streamReadIdleTimeout, cancel))

	for {
		// 按行读取，但保持原始格式
		line, err := reader.ReadString('\n')

		if line != "" {
			// 移除换行符，保持与 PHP 实现一致
			trimmedLine := strings.TrimSuffix(line, "\n")
			trimmedLine = strings.TrimSuffix(trimmedLine, "\r")
			callback(trimmedLine)
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
	}

	return nil
}

// PassthroughStreamRequestRaw 原始流式透传 - 直接传递 io.Reader 用于完全保持格式
func (a *GenericAdapter) PassthroughStreamRequestRaw(ctx context.Context, requestData map[string]interface{}, requestType string) (io.ReadCloser, error) {
	req, err := a.newUpstreamRequest(ctx, requestData, requestType)
	if err != nil {
		return nil, err
	}

	// 与 PassthroughStreamRequest 一致：流式不走整体超时 client
	sctx, cancel := context.WithTimeout(ctx, streamOverallTimeout)
	req = req.WithContext(sctx)

	resp, err := streamHTTPClient.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}

	// 上游返回非 2xx：返回结构化错误，调用方可透传真实状态码
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var parsed map[string]interface{}
		json.Unmarshal(errBody, &parsed)
		resp.Body.Close()
		cancel()
		return nil, &UpstreamError{StatusCode: resp.StatusCode, Body: parsed, Raw: truncateForLog(errBody)}
	}

	// 返回带空闲看门狗的 body：cancel 所有权随 reader 交给调用方（Close 时释放）
	return newIdleCancelReader(resp.Body, streamReadIdleTimeout, cancel), nil
}

// idleCancelReader 包装流式响应体：每次阻塞 Read 期间挂看门狗定时器，
// 超过 idle 无任何字节到达则调用 cancel 中止请求（本次 Read 返回错误，
// 视作瞬时错误交由调用方重试）。回调处理耗时不算空闲（计时仅覆盖阻塞中的 Read）。
type idleCancelReader struct {
	rc     io.ReadCloser
	idle   time.Duration
	cancel context.CancelFunc
	timer  *time.Timer
}

func newIdleCancelReader(rc io.ReadCloser, idle time.Duration, cancel context.CancelFunc) *idleCancelReader {
	return &idleCancelReader{rc: rc, idle: idle, cancel: cancel}
}

func (r *idleCancelReader) Read(p []byte) (int, error) {
	if r.timer == nil {
		r.timer = time.AfterFunc(r.idle, r.cancel)
	} else {
		r.timer.Reset(r.idle)
	}
	n, err := r.rc.Read(p)
	r.timer.Stop()
	return n, err
}

func (r *idleCancelReader) Close() error {
	if r.timer != nil {
		r.timer.Stop()
	}
	r.cancel()
	return r.rc.Close()
}

// providerModelPair 缓存的 provider/model 组合（值类型，命中时返回副本避免调用方修改污染缓存）
type providerModelPair struct {
	provider models.AiProvider
	model    models.AiModel
}

// GetProviderAndModel 按 "provider/model" 解析并查询，带 60s TTL 缓存。
// 低频变更的配置数据缓存后，V1 每请求可省去 2 次 DB 往返。
func GetProviderAndModel(modelID string) (*models.AiProvider, *models.AiModel, error) {
	// 只支持 provider/model 格式
	if !strings.Contains(modelID, "/") {
		return nil, nil, fmt.Errorf("model must be in 'provider/model' format")
	}

	cacheKey := "pm:" + modelID
	if v, ok := cache.Config.Get(cacheKey); ok {
		if pair, ok := v.(*providerModelPair); ok {
			p, m := pair.provider, pair.model
			return &p, &m, nil
		}
	}

	parts := strings.SplitN(modelID, "/", 2)
	providerCode := parts[0]
	simpleCode := parts[1]

	var p models.AiProvider
	if err := repository.DB.Where("code = ? AND status = ?", providerCode, 1).First(&p).Error; err != nil {
		return nil, nil, fmt.Errorf("provider '%s' not found", providerCode)
	}

	var m models.AiModel
	if err := repository.DB.Where("provider_id = ? AND code = ? AND status = ?", p.ID, simpleCode, 1).First(&m).Error; err != nil {
		return nil, nil, fmt.Errorf("model '%s' not found", modelID)
	}

	cache.Config.Set(cacheKey, &providerModelPair{provider: p, model: m})

	return &p, &m, nil
}

// GetRandomApiKey 在 provider 的可用 Key 中随机选择（多 Key 负载分摊）。
// Key 列表带 60s TTL 缓存，且查询直接过滤 status=1（原先查全量后内存过滤）。
func GetRandomApiKey(providerID uint) (*models.ProviderApiKey, error) {
	cacheKey := fmt.Sprintf("keys:%d", providerID)
	if v, ok := cache.Config.Get(cacheKey); ok {
		if keys, ok := v.([]models.ProviderApiKey); ok && len(keys) > 0 {
			k := keys[rand.Intn(len(keys))]
			return &k, nil
		}
		// 空结果同样命中缓存（防穿透）：len==0 时走下方返回错误
		if keys, ok := v.([]models.ProviderApiKey); ok && len(keys) == 0 {
			return nil, fmt.Errorf("no api keys available")
		}
	}

	var keys []models.ProviderApiKey
	if err := repository.DB.Where("ai_provider_id = ? AND status = ?", providerID, 1).Find(&keys).Error; err != nil {
		logger.Error("Failed to query API keys in GetRandomApiKey: %v", err)
		return nil, err
	}

	cache.Config.Set(cacheKey, keys)

	if len(keys) == 0 {
		return nil, fmt.Errorf("no api keys available")
	}

	// 在可用 Key 中随机选择，实现多 Key 之间的负载分摊
	k := keys[rand.Intn(len(keys))]
	return &k, nil
}

func ExtractTokenUsage(response map[string]interface{}) (int, int, int) {
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0

	// 方式1：response["usage"]（OpenAI 标准格式与 Responses API 格式相同，一个分支覆盖）
	if usage, ok := response["usage"].(map[string]interface{}); ok {
		promptTokens, completionTokens, totalTokens = extractUsageFromMap(usage)
		if totalTokens > 0 {
			return promptTokens, completionTokens, totalTokens
		}
	}

	// 方式3：检查是否有 output 数组 (Responses API 流式格式)
	if output, ok := response["output"].([]interface{}); ok && len(output) > 0 {
		for _, outItem := range output {
			if outMap, ok := outItem.(map[string]interface{}); ok {
				if usage, ok := outMap["usage"].(map[string]interface{}); ok {
					pTokens, cTokens, tTokens := extractUsageFromMap(usage)
					if tTokens > 0 {
						promptTokens = pTokens
						completionTokens = cTokens
						totalTokens = tTokens
						return promptTokens, completionTokens, totalTokens
					}
				}
			}
		}
	}

	// 方式4：检查 data 嵌套格式
	if data, ok := response["data"].(map[string]interface{}); ok {
		if usage, ok := data["usage"].(map[string]interface{}); ok {
			promptTokens, completionTokens, totalTokens = extractUsageFromMap(usage)
			if totalTokens > 0 {
				return promptTokens, completionTokens, totalTokens
			}
		}
	}

	return promptTokens, completionTokens, totalTokens
}

func extractUsageFromMap(usage map[string]interface{}) (int, int, int) {
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0

	if usage == nil {
		return promptTokens, completionTokens, totalTokens
	}

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

	return promptTokens, completionTokens, totalTokens
}

// ExtractIntFromMap 从 map 中提取 int 值，支持多个 key
func ExtractIntFromMap(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if v, ok := m[key].(float64); ok {
			return int(v)
		}
		if v, ok := m[key].(int); ok {
			return v
		}
	}
	return 0
}

func CalculateCost(promptTokens, completionTokens int, model *models.AiModel) float64 {
	inputPrice := 0.0
	outputPrice := 0.0

	if model.InputPrice > 0 {
		inputPrice = float64(model.InputPrice)
	}
	if model.OutputPrice > 0 {
		outputPrice = float64(model.OutputPrice)
	}

	cost := (float64(promptTokens) * inputPrice / 1000) + (float64(completionTokens) * outputPrice / 1000)
	return cost
}

func PreparePassthroughRequest(requestData map[string]interface{}, model *models.AiModel, requestType string) map[string]interface{} {
	if _, ok := requestData["model"].(string); ok {
		requestData["model"] = model.Code
	}

	if requestType == "chat" || requestType == "messages" || requestType == "" {
		if stream, ok := requestData["stream"].(bool); ok && stream {
			if _, ok := requestData["stream_options"]; !ok {
				requestData["stream_options"] = map[string]interface{}{}
			}
			so := requestData["stream_options"].(map[string]interface{})
			so["include_usage"] = true
		}
	}

	// 特殊处理豆包多模态嵌入接口
	if requestType == "embeddings" {
		// 通过 ProviderID 间接获取 provider code，因为 Provider 是值类型
		// 先检查端点是否包含 multimodal 或 vision，这更可靠
		customEndpoint := model.CustomEndpoint
		if customEndpoint == "" {
			customEndpoint = model.EmbeddingsEndpoint
		}
		if strings.Contains(customEndpoint, "multimodal") || strings.Contains(customEndpoint, "vision") {
			if input, ok := requestData["input"]; ok {
				// 处理输入参数转换
				if inputSlice, ok := input.([]interface{}); ok {
					isSimpleTextArray := true
					for _, item := range inputSlice {
						if _, ok := item.(string); !ok {
							isSimpleTextArray = false
							break
						}
					}
					if isSimpleTextArray {
						formattedInput := make([]map[string]interface{}, len(inputSlice))
						for i, text := range inputSlice {
							formattedInput[i] = map[string]interface{}{
								"type": "text",
								"text": text,
							}
						}
						requestData["input"] = formattedInput
						logger.Debug("Converted simple text array to multimodal format")
					}
				} else if textInput, ok := input.(string); ok {
					requestData["input"] = []map[string]interface{}{
						{
							"type": "text",
							"text": textInput,
						},
					}
					logger.Debug("Converted single string to multimodal format")
				}
			}
		}
	}

	return requestData
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// truncateForLog 截断原始响应体用于错误信息展示，避免超长错误体占满日志与 DB
func truncateForLog(body []byte) string {
	s := string(body)
	if len(s) > 1024 {
		s = s[:1024]
	}
	return s
}

func GetEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultValue
}

// ==================== Adapter 接口实现 ====================

// Chat 发送聊天请求（非流式）
func (a *GenericAdapter) Chat(ctx context.Context, messages []Message, options map[string]interface{}) (map[string]interface{}, error) {
	// 转换消息格式
	requestData := a.buildRequestData(messages, options, false)

	// 转换请求格式
	transformedData, err := a.TransformRequest(requestData, "chat")
	if err != nil {
		return nil, err
	}

	// 发送请求
	result, err := a.PassthroughRequest(ctx, transformedData, "chat")
	if err != nil {
		return nil, err
	}

	// 转换响应格式
	return a.TransformResponse(result, "chat"), nil
}

// ChatStream 发送聊天请求（流式）
func (a *GenericAdapter) ChatStream(ctx context.Context, messages []Message, options map[string]interface{}, callback StreamCallback) error {
	// 转换消息格式
	requestData := a.buildRequestData(messages, options, true)

	// 转换请求格式
	transformedData, err := a.TransformRequest(requestData, "chat")
	if err != nil {
		return err
	}

	// 发送流式请求
	return a.PassthroughStreamRequest(ctx, transformedData, func(line string) {
		// 解析流式数据
		data := a.parseStreamLine(line)
		if data != nil {
			// 转换响应格式
			transformedData := a.TransformResponse(data, "chat")
			callback(transformedData)
		}
	}, "chat")
}

// Embeddings 获取文本嵌入向量
func (a *GenericAdapter) Embeddings(ctx context.Context, inputs []string, options map[string]interface{}) (map[string]interface{}, error) {
	requestData := map[string]interface{}{
		"input": inputs,
	}

	// 添加模型参数
	if model, ok := options["model"].(string); ok && model != "" {
		requestData["model"] = model
	} else if a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	// 添加可选参数
	if encodingFormat, ok := options["encoding_format"].(string); ok {
		requestData["encoding_format"] = encodingFormat
	}
	if dimensions, ok := options["dimensions"].(float64); ok {
		requestData["dimensions"] = dimensions
	}

	// 转换请求格式
	transformedData, err := a.TransformRequest(requestData, "embeddings")
	if err != nil {
		return nil, err
	}

	// 发送请求
	result, err := a.PassthroughRequest(ctx, transformedData, "embeddings")
	if err != nil {
		return nil, err
	}

	// 转换响应格式
	return a.TransformResponse(result, "embeddings"), nil
}

// ImagesGenerations 生成图片
func (a *GenericAdapter) ImagesGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error) {
	requestData := map[string]interface{}{
		"prompt": prompt,
	}

	// 添加模型参数
	if model, ok := options["model"].(string); ok && model != "" {
		requestData["model"] = model
	} else if a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	// 添加可选参数
	optionalParams := []string{"n", "size", "quality", "style", "response_format"}
	for _, param := range optionalParams {
		if val, ok := options[param]; ok {
			requestData[param] = val
		}
	}

	// 转换请求格式
	transformedData, err := a.TransformRequest(requestData, "images")
	if err != nil {
		return nil, err
	}

	// 发送请求
	result, err := a.PassthroughRequest(ctx, transformedData, "images")
	if err != nil {
		return nil, err
	}

	// 转换响应格式
	return a.TransformResponse(result, "images"), nil
}

// VideosGenerations 生成视频任务
func (a *GenericAdapter) VideosGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error) {
	requestData := map[string]interface{}{
		"prompt": prompt,
	}

	// 添加模型参数
	if model, ok := options["model"].(string); ok && model != "" {
		requestData["model"] = model
	} else if a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	// 转换请求格式
	transformedData, err := a.TransformRequest(requestData, "videos")
	if err != nil {
		return nil, err
	}

	// 发送请求
	result, err := a.PassthroughRequest(ctx, transformedData, "videos")
	if err != nil {
		return nil, err
	}

	// 转换响应格式
	return a.TransformResponse(result, "videos"), nil
}

// GetVideoTask 查询视频生成任务状态
func (a *GenericAdapter) GetVideoTask(ctx context.Context, taskID string) (map[string]interface{}, error) {
	// 构建URL
	fullUrl := a.buildVideoTaskUrl(taskID)

	// 任务轮询是短请求：在传入 ctx 基础上叠加 30s 短超时，
	// 避免复用 600s 长超时客户端配置拖慢失败反馈
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fullUrl, nil)
	if err != nil {
		return nil, err
	}

	// 添加请求头
	for k, v := range a.buildHeaders() {
		req.Header.Add(k, v)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	// 上游返回非 2xx：返回 UpstreamError，避免把错误响应体当成功数据解析
	// 导致任务查询失败被静默吞掉
	if resp.StatusCode >= 400 {
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		return nil, &UpstreamError{StatusCode: resp.StatusCode, Body: parsed, Raw: truncateForLog(body)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// 转换响应格式
	return a.TransformResponse(result, "video_task"), nil
}

// GetProvider 获取当前适配器使用的提供商信息
func (a *GenericAdapter) GetProvider() *models.AiProvider {
	return a.provider
}

// GetModelConfig 获取当前适配器使用的模型配置
func (a *GenericAdapter) GetModelConfig() *models.AiModel {
	return a.modelConfig
}

// GetApiKey 获取当前使用的API Key
func (a *GenericAdapter) GetApiKey() string {
	if a.apiKey != nil {
		return a.apiKey.ApiKey
	}
	return ""
}

// TransformRequest 转换请求格式（通用适配器：透传，不做转换）
func (a *GenericAdapter) TransformRequest(requestData map[string]interface{}, requestType string) (map[string]interface{}, error) {
	// 通用适配器直接透传，不做格式转换
	// 但会处理一些通用逻辑，如设置模型代码等

	if _, ok := requestData["model"].(string); ok {
		requestData["model"] = a.modelConfig.Code
	}

	// 流式请求的特殊处理
	if requestType == "chat" || requestType == "messages" || requestType == "" {
		if stream, ok := requestData["stream"].(bool); ok && stream {
			if _, ok := requestData["stream_options"]; !ok {
				requestData["stream_options"] = map[string]interface{}{}
			}
			so := requestData["stream_options"].(map[string]interface{})
			so["include_usage"] = true
		}
	}

	return requestData, nil
}

// TransformResponse 转换响应格式（通用适配器：透传，不做转换）
func (a *GenericAdapter) TransformResponse(responseData map[string]interface{}, requestType string) map[string]interface{} {
	// 通用适配器直接透传，不做格式转换
	return responseData
}

// ExtractTokenUsage 提取 token 使用量
func (a *GenericAdapter) ExtractTokenUsage(response map[string]interface{}) (int, int, int) {
	return ExtractTokenUsage(response)
}

// buildRequestData 构建请求数据
func (a *GenericAdapter) buildRequestData(messages []Message, options map[string]interface{}, forStream bool) map[string]interface{} {
	requestData := map[string]interface{}{}

	// 转换消息格式
	msgList := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		// Agent 工具环：assistant 的工具调用请求与 tool 结果消息需要回传上游
		if len(msg.ToolCalls) > 0 {
			m["tool_calls"] = msg.ToolCalls
		}
		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}
		msgList[i] = m
	}
	requestData["messages"] = msgList

	// 添加模型参数
	if model, ok := options["model"].(string); ok && model != "" {
		requestData["model"] = model
	} else if a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	// 添加可选参数
	optionalParams := []string{"temperature", "max_tokens", "top_p", "stream", "tool_choice"}
	for _, param := range optionalParams {
		if val, ok := options[param]; ok {
			requestData[param] = val
		}
	}

	// 添加工具调用参数
	if tools, ok := options["tools"]; ok && tools != nil {
		requestData["tools"] = tools
	}

	// 流式请求
	if forStream {
		requestData["stream"] = true
	}

	return requestData
}

// buildVideoTaskUrl 构建视频任务查询URL
func (a *GenericAdapter) buildVideoTaskUrl(taskID string) string {
	var endpoint string

	// 优先从模型配置中获取自定义端点
	if a.modelConfig != nil {
		if a.modelConfig.VideoEndpoint != "" {
			endpoint = a.modelConfig.VideoEndpoint
		} else if a.modelConfig.CustomEndpoint != "" {
			// 如果模型有自定义端点且包含视频相关路径，也尝试使用
			endpoint = a.modelConfig.CustomEndpoint
		}
	}

	// 如果模型没有，尝试从提供商配置中获取
	if endpoint == "" && a.provider.RequestFormat != nil {
		var requestFormat map[string]interface{}
		if err := json.Unmarshal(a.provider.RequestFormat, &requestFormat); err == nil {
			if ep, ok := requestFormat["videos_endpoint"].(string); ok && ep != "" {
				endpoint = ep
			}
		}
	}

	// 默认端点
	if endpoint == "" {
		endpoint = "/contents/generations/tasks"
	}

	baseUrl := a.provider.BaseUrl
	var fullUrl string

	// 如果 endpoint 已经是完整的 URL，直接使用
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		fullUrl = endpoint
	} else {
		fullUrl = strings.TrimRight(baseUrl, "/") + "/" + strings.TrimLeft(endpoint, "/")
	}

	// 替换 taskId 到路径中（智能替换）
	if strings.Contains(fullUrl, "contents/generations/tasks") {
		// 豆包格式
		fullUrl = strings.TrimRight(fullUrl, "/") + "/" + taskID
	} else if strings.HasSuffix(fullUrl, "/tasks") {
		// 通用格式
		fullUrl += "/" + taskID
	} else if strings.Contains(fullUrl, "{task_id}") {
		// 模板格式
		fullUrl = strings.ReplaceAll(fullUrl, "{task_id}", taskID)
	} else {
		// 默认直接追加
		if strings.HasSuffix(fullUrl, "/") {
			fullUrl += taskID
		} else {
			fullUrl += "/" + taskID
		}
	}

	return fullUrl
}

// parseStreamLine 解析流式数据行
func (a *GenericAdapter) parseStreamLine(line string) map[string]interface{} {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// 处理 SSE 格式：data: {...}
	if strings.HasPrefix(line, "data: ") {
		data := line[6:]
		if data == "[DONE]" {
			return map[string]interface{}{"done": true}
		}

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(data), &result); err == nil {
			return result
		}
	}

	// 尝试直接解析
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(line), &result); err == nil {
		return result
	}

	return nil
}
