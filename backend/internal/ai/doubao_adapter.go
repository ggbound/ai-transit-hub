package ai

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// DoubaoAdapter 豆包/火山引擎适配器
// 公共能力（请求构建、认证、透传等）继承自嵌入的 *GenericAdapter，
// 此处仅保留豆包厂商差异逻辑：output.usage 提升、多模态 embeddings 逐条适配、
// Seedance 视频格式、流式 usage 重编码。
type DoubaoAdapter struct {
	*GenericAdapter
}

// NewDoubaoAdapter 创建豆包适配器
func NewDoubaoAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *DoubaoAdapter {
	providerKey := &models.ProviderApiKey{
		AiProviderID: provider.ID,
		ApiKey:       apiKey,
		Status:       1,
	}

	return &DoubaoAdapter{
		GenericAdapter: &GenericAdapter{
			client:      upstreamHTTPClient,
			provider:    provider,
			apiKey:      providerKey,
			modelConfig: modelConfig,
		},
	}
}

// ==================== Adapter 接口实现（厂商覆写） ====================

// ChatStream 发送聊天请求（流式）
// 豆包特殊处理：流式分片中的 output.usage 字段提升到顶层 usage，便于通用记账逻辑提取
func (a *DoubaoAdapter) ChatStream(ctx context.Context, messages []Message, options map[string]interface{}, callback StreamCallback) error {
	return a.GenericAdapter.ChatStream(ctx, messages, options, func(data map[string]interface{}) {
		if output, ok := data["output"]; ok {
			if outputMap, ok := output.(map[string]interface{}); ok {
				if usage, ok := outputMap["usage"]; ok {
					data["usage"] = usage
				}
			}
		}
		callback(data)
	})
}

// Embeddings 获取文本嵌入向量
func (a *DoubaoAdapter) Embeddings(ctx context.Context, inputs []string, options map[string]interface{}) (map[string]interface{}, error) {
	// 豆包多模态嵌入特殊处理
	if a.modelConfig != nil {
		customEndpoint := a.modelConfig.CustomEndpoint
		if customEndpoint == "" {
			customEndpoint = a.modelConfig.EmbeddingsEndpoint
		}
		if customEndpoint != "" && (strings.Contains(customEndpoint, "multimodal") || strings.Contains(customEndpoint, "vision")) {
			return a.multimodalEmbeddings(ctx, inputs)
		}
	}

	return a.GenericAdapter.Embeddings(ctx, inputs, options)
}

// multimodalEmbeddings 豆包多模态嵌入的 OpenAI 语义适配。
// 豆包多模态端点的 input 语义是"单个向量的内容项列表"（N 项共同构成 1 条向量），
// 与 OpenAI 的"N 个独立文本 → N 条向量"语义冲突；若把 N 个文本放入同一次请求，
// 上游只会返回 1 条向量（静默丢数据）。
// 因此逐条文本单独请求（每条 input 仅含一个内容项，并发受限），再按 index 合并为标准 data[]。
func (a *DoubaoAdapter) multimodalEmbeddings(ctx context.Context, inputs []string) (map[string]interface{}, error) {
	logger.Debug("DoubaoAdapter: multimodal embeddings, issuing %d per-item requests", len(inputs))

	// 单文本：直接单请求，省去合并开销
	if len(inputs) == 1 {
		return a.GenericAdapter.PassthroughRequest(ctx, a.multimodalEmbeddingRequest(inputs[0]), "embeddings")
	}

	type embedResult struct {
		response map[string]interface{}
		err      error
	}
	results := make([]embedResult, len(inputs))

	// 并发上限，避免大数组打爆上游
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, text := range inputs {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, err := a.GenericAdapter.PassthroughRequest(ctx, a.multimodalEmbeddingRequest(t), "embeddings")
			results[idx] = embedResult{response: resp, err: err}
		}(i, text)
	}
	wg.Wait()

	// 任一子请求失败即返回该错误（与单请求语义一致）
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
	}

	// 合并为标准 data[]，usage 逐条累加
	dataArr := make([]interface{}, 0, len(inputs))
	mergedUsage := map[string]interface{}{}
	hasUsage := false
	for i, r := range results {
		dataArr = append(dataArr, map[string]interface{}{
			"object":    "embedding",
			"index":     i,
			"embedding": extractEmbeddingVector(r.response),
		})
		if u, ok := r.response["usage"].(map[string]interface{}); ok {
			hasUsage = true
			mergeUsageTokens(mergedUsage, u)
		}
	}

	merged := map[string]interface{}{
		"object": "list",
		"data":   dataArr,
		"model":  a.modelConfig.Code,
	}
	if hasUsage {
		merged["usage"] = mergedUsage
	}
	return merged, nil
}

// multimodalEmbeddingRequest 构建豆包多模态嵌入的单条请求（input 仅含一个文本内容项）
func (a *DoubaoAdapter) multimodalEmbeddingRequest(text string) map[string]interface{} {
	return map[string]interface{}{
		"model": a.modelConfig.Code,
		"input": []map[string]interface{}{
			{"type": "text", "text": text},
		},
	}
}

// extractEmbeddingVector 从上游 embeddings 响应中提取向量（兼容 data 为单对象或数组两种形状）
func extractEmbeddingVector(response map[string]interface{}) interface{} {
	if response == nil {
		return nil
	}
	switch d := response["data"].(type) {
	case map[string]interface{}:
		return d["embedding"]
	case []interface{}:
		if len(d) > 0 {
			if item, ok := d[0].(map[string]interface{}); ok {
				return item["embedding"]
			}
		}
	}
	return nil
}

// mergeUsageTokens 将单次请求的 token 用量累加进合并统计（JSON 解码后数值均为 float64）
func mergeUsageTokens(total, u map[string]interface{}) {
	for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		if v, ok := u[key].(float64); ok {
			cur, _ := total[key].(float64)
			total[key] = cur + v
		}
	}
}

// VideosGenerations 生成视频任务
func (a *DoubaoAdapter) VideosGenerations(ctx context.Context, prompt string, options map[string]interface{}) (map[string]interface{}, error) {
	// 豆包 Seedance 视频生成任务 API 格式（官方强校验传法）：
	// {"model": <Model ID>, "content": [{"type":"text","text": prompt}],
	//  ratio/duration/seed/watermark/generate_audio 为顶层参数}
	// 注意：顶层 "prompt" 字段会被 Ark 忽略，提示词必须放 content 数组
	requestData := map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": prompt},
		},
	}
	if a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	// 处理参考图片
	if referenceImages, ok := options["reference_images"]; ok {
		requestData["reference_images"] = referenceImages
	}

	// 处理 size 到 ratio 转换
	if size, ok := options["size"].(string); ok {
		ratio := a.convertSizeToRatio(size)
		requestData["ratio"] = ratio
	} else if ratio, ok := options["ratio"].(string); ok {
		requestData["ratio"] = ratio
	}

	// 处理 duration
	if duration, ok := options["duration"].(float64); ok {
		requestData["duration"] = int(duration)
	} else {
		requestData["duration"] = 5
	}

	// 处理 watermark
	if watermark, ok := options["watermark"]; ok {
		requestData["watermark"] = watermark
	} else {
		requestData["watermark"] = false
	}

	// 处理 generate_audio
	if generateAudio, ok := options["generate_audio"]; ok {
		requestData["generate_audio"] = generateAudio
	}

	// 处理 seed
	if seed, ok := options["seed"].(float64); ok {
		requestData["seed"] = int(seed)
	}

	// 转换请求
	transformed, err := a.TransformRequest(requestData, "videos")
	if err != nil {
		return nil, err
	}

	return a.GenericAdapter.PassthroughRequest(ctx, transformed, "videos")
}

// TransformRequest 转换请求格式
func (a *DoubaoAdapter) TransformRequest(requestData map[string]interface{}, requestType string) (map[string]interface{}, error) {
	// 豆包主要是透传，但需要确保模型代码正确
	if _, ok := requestData["model"].(string); ok && a.modelConfig != nil {
		requestData["model"] = a.modelConfig.Code
	}

	return requestData, nil
}

// TransformResponse 转换响应格式
func (a *DoubaoAdapter) TransformResponse(responseData map[string]interface{}, requestType string) map[string]interface{} {
	// 处理豆包的特殊响应格式

	// 1. 处理 output.usage 到顶层 usage
	if output, ok := responseData["output"]; ok {
		if outputMap, ok := output.(map[string]interface{}); ok {
			if usage, ok := outputMap["usage"]; ok {
				responseData["usage"] = usage
			}
		}
	}

	// 2. 处理图片生成响应
	if requestType == "images" {
		if data, ok := responseData["data"]; ok {
			if dataArray, ok := data.([]interface{}); ok && len(dataArray) > 0 {
				// 确保 OpenAI 兼容格式
			}
		}
	}

	return responseData
}

// ExtractTokenUsage 提取 Token 使用量
func (a *DoubaoAdapter) ExtractTokenUsage(response map[string]interface{}) (int, int, int) {
	// 优先检查豆包的 output.usage 格式
	if output, ok := response["output"]; ok {
		if outputMap, ok := output.(map[string]interface{}); ok {
			if usage, ok := outputMap["usage"]; ok {
				if usageMap, ok := usage.(map[string]interface{}); ok {
					promptTokens := ExtractIntFromMap(usageMap, "prompt_tokens", "input_tokens")
					completionTokens := ExtractIntFromMap(usageMap, "completion_tokens", "output_tokens")
					totalTokens := ExtractIntFromMap(usageMap, "total_tokens")
					if totalTokens == 0 {
						totalTokens = promptTokens + completionTokens
					}
					return promptTokens, completionTokens, totalTokens
				}
			}
		}
	}

	// 回退到通用处理
	return ExtractTokenUsage(response)
}

// ==================== 内部辅助方法 ====================

func (a *DoubaoAdapter) convertSizeToRatio(size string) string {
	switch size {
	case "1280x720", "1920x1080", "2560x1440", "1024x576":
		return "16:9"
	case "720x1280", "1080x1920", "576x1024":
		return "9:16"
	case "1024x768", "1280x960", "1400x1050":
		return "4:3"
	case "768x1024", "960x1280", "1050x1400":
		return "3:4"
	default:
		return "adaptive"
	}
}

// ==================== 透传适配（厂商覆写） ====================

// PassthroughRequest 透传请求
func (a *DoubaoAdapter) PassthroughRequest(ctx context.Context, requestData map[string]interface{}, requestType string) (map[string]interface{}, error) {
	// 豆包视频生成的 L1 适配：Ark 会忽略顶层 prompt 字段，提示词必须放在 content 数组中（官方强校验）。
	// 网关标准字段是 prompt：客户端只传 prompt 时自动转换为官方要求的 content 数组；
	// 客户端传了 content（原生透传 escape hatch）时保持原样；两者并存已由 handler 层 400 拦截。
	// 注意：与 VideosGenerations（Seedance 完整参数格式）是两条不同调用链的路径，不可合并。
	if requestType == "video" {
		adaptVideoPromptToContent(requestData)
	}
	return a.GenericAdapter.PassthroughRequest(ctx, requestData, requestType)
}

// adaptVideoPromptToContent 将视频生成请求的标准 prompt 字段转换为豆包官方要求的 content 数组。
// 顶层其他参数（ratio/duration/seed/watermark/generate_audio 等）Ark 原生接受，保持不动。
func adaptVideoPromptToContent(requestData map[string]interface{}) {
	prompt, ok := requestData["prompt"].(string)
	if !ok || prompt == "" {
		return // 无 prompt（或已在 handler 层拦截的非法形态），交由上游原生校验
	}
	if _, hasContent := requestData["content"]; hasContent {
		return
	}
	requestData["content"] = []interface{}{
		map[string]interface{}{"type": "text", "text": prompt},
	}
	delete(requestData, "prompt")
}

// PassthroughStreamRequest 透传流式请求
func (a *DoubaoAdapter) PassthroughStreamRequest(ctx context.Context, requestData map[string]interface{}, callback func(string), requestType string) error {
	return a.GenericAdapter.PassthroughStreamRequest(ctx, requestData, func(line string) {
		// 豆包特殊处理：处理流式数据中的 output.usage 字段
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data: ") {
			data := strings.TrimPrefix(trimmed, "data: ")
			if data != "[DONE]" {
				var decoded map[string]interface{}
				if json.Unmarshal([]byte(data), &decoded) == nil {
					// 处理 output.usage 到顶层 usage
					if output, ok := decoded["output"]; ok {
						if outputMap, ok := output.(map[string]interface{}); ok {
							if usage, ok := outputMap["usage"]; ok {
								decoded["usage"] = usage
								// 重新编码后传递
								if modifiedBytes, err := json.Marshal(decoded); err == nil {
									callback("data: " + string(modifiedBytes))
									return
								}
							}
						}
					}
				}
			}
		}
		// 原样传递
		callback(line)
	}, requestType)
}
