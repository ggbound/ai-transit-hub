package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/metrics"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/service"
	"github.com/your-org/ai-transit-hub/internal/utils"
)

type ChatController struct {
	BaseHandler
}

func NewChatController() *ChatController {
	return &ChatController{}
}

// SendMessageRequest 聊天请求结构
type SendMessageRequest struct {
	SessionID    *uint                    `json:"session_id"`
	ModelID      uint                     `json:"model_id" binding:"required"`
	Messages     []map[string]interface{} `json:"messages" binding:"required"`
	Stream       bool                     `json:"stream"`
	SystemPrompt *string                  `json:"system_prompt"`
}

// SendMessage 发送聊天消息
func (h *ChatController) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	// 查找模型
	modelPtr, err := h.findModelByID(req.ModelID)
	if err != nil {
		h.ErrorResponse(c, "Model not found", http.StatusNotFound)
		return
	}
	model := *modelPtr

	var provider models.AiProvider
	if err := repository.DB.First(&provider, model.ProviderID).Error; err != nil || provider.Status != 1 {
		h.ErrorResponse(c, "Provider not available", http.StatusBadRequest)
		return
	}

	// 获取或创建会话
	var session models.ChatSession
	if req.SessionID != nil {
		if err := repository.DB.Where("id = ? AND user_id = ?", *req.SessionID, user.ID).First(&session).Error; err != nil {
			h.ErrorResponse(c, "Session not found", http.StatusNotFound)
			return
		}
		session.ModelID = model.ID
		repository.DB.Save(&session)
	} else {
		session = models.ChatSession{
			UserID:        user.ID,
			ModelID:       model.ID,
			Title:         h.generateTitle(req.Messages),
			StreamEnabled: req.Stream,
		}
		if req.SystemPrompt != nil {
			session.SystemPrompt = *req.SystemPrompt
		}
		if err := repository.DB.Create(&session).Error; err != nil {
			h.ErrorResponse(c, "Failed to create session", http.StatusInternalServerError)
			return
		}
	}

	// 保存用户消息
	var userMessageContent string
	if len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]
		content, _ := lastMsg["content"].(string)
		userMessageContent = content
		userMessage := models.ChatMessage{
			SessionID: session.ID,
			Role:      "user",
			Content:   content,
		}
		repository.DB.Create(&userMessage)
	}

	// 获取API密钥：走统一的多 Key 负载均衡（带 60s TTL 缓存），避免每次请求裸查 DB
	apiKey, err := ai.GetRandomApiKey(provider.ID)
	if err != nil {
		h.ErrorResponse(c, "No API key available for this provider", http.StatusInternalServerError)
		return
	}

	// 准备消息列表
	messages := h.prepareMessages(&session, userMessageContent)

	// 创建初始记录
	userApiKeyID := uint(0)
	if id, exists := c.Get("api_key_id"); exists {
		if idUint, ok := id.(uint); ok {
			userApiKeyID = idUint
		}
	}
	recordIDs, err := service.Accounting.CreateInitialRecords(
		user.ID,
		&model,
		&provider,
		"openai",
		"chat/completions",
		userApiKeyID,
		session.ID,
		"",
		"api",
	)
	if err != nil {
		h.ErrorResponse(c, "Failed to create request records", http.StatusInternalServerError)
		return
	}

	if req.Stream {
		h.streamResponse(c, &model, &provider, apiKey, &session, messages, user, recordIDs)
	} else {
		h.nonStreamResponse(c, &model, &provider, apiKey, &session, messages, user, recordIDs)
	}
}

// prepareMessages 准备消息列表，包含系统提示和历史消息
func (h *ChatController) prepareMessages(session *models.ChatSession, userMessage string) []map[string]interface{} {
	messages := []map[string]interface{}{}

	// 添加系统提示
	if session.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": session.SystemPrompt,
		})
	}

	// 加载历史消息：只取最近 50 条，防止长会话把上下文与内存撑爆
	const maxHistoryMessages = 50
	var historyMessages []models.ChatMessage
	if err := repository.DB.Where("session_id = ?", session.ID).
		Order("created_at desc").Limit(maxHistoryMessages).Find(&historyMessages).Error; err != nil {
		logger.Error("Failed to load chat history for session %d: %v", session.ID, err)
	}
	// desc 取出的记录翻回时间正序
	for i, j := 0, len(historyMessages)-1; i < j; i, j = i+1, j-1 {
		historyMessages[i], historyMessages[j] = historyMessages[j], historyMessages[i]
	}

	for _, msg := range historyMessages {
		messages = append(messages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// 如果最后一条不是用户消息（刚刚保存的），添加当前用户消息
	if len(messages) == 0 || messages[len(messages)-1]["role"] != "user" {
		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": userMessage,
		})
	}

	return messages
}

// streamCapture 以滑动窗口方式保留原始流数据：头尾各若干行，中间行以计数占位，
// 防止超长流式响应把内存与 DB 记录撑爆（与 v1 透传的 streamRelay 同一策略）
type streamCapture struct {
	headLines []string // 头部行：通常含 role/开始帧
	tailLines []string // 尾部行：通常含结尾帧与 usage
	dropped   int      // 被裁掉的中间行数
}

const (
	maxCaptureHeadLines = 50
	maxCaptureTailLines = 100
)

func (s *streamCapture) append(line string) {
	if len(s.headLines) < maxCaptureHeadLines {
		s.headLines = append(s.headLines, line)
		return
	}
	if len(s.tailLines) < maxCaptureTailLines {
		s.tailLines = append(s.tailLines, line)
		return
	}
	// 尾部窗口已满：丢弃最旧的一行，腾出位置继续滚动保留最新的行
	s.dropped++
	s.tailLines = append(s.tailLines[1:], line)
}

// joined 输出用于存库回填的原始流数据：头 + 丢弃计数占位 + 尾，行间以空行分隔
func (s *streamCapture) joined() string {
	parts := make([]string, 0, len(s.headLines)+1+len(s.tailLines))
	parts = append(parts, s.headLines...)
	if s.dropped > 0 {
		parts = append(parts, fmt.Sprintf(": %d lines dropped", s.dropped))
	}
	parts = append(parts, s.tailLines...)
	return strings.Join(parts, "\n\n")
}

// streamResponse 流式响应
func (h *ChatController) streamResponse(c *gin.Context, model *models.AiModel, provider *models.AiProvider, apiKey *models.ProviderApiKey, session *models.ChatSession, messages []map[string]interface{}, user *models.User, recordIDs *service.RecordIDs) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// 使用 AdapterFactory 选择适配器
	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		h.ErrorResponse(c, "Failed to create adapter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 转换消息格式为 Message 数组
	aiMessages := make([]ai.Message, len(messages))
	for i, msg := range messages {
		aiMessages[i] = ai.Message{
			Role:    msg["role"].(string),
			Content: msg["content"],
		}
	}

	var fullContent strings.Builder // 流式内容累积（避免 O(n²) 字符串拼接）
	inputTokens := 0
	outputTokens := 0
	var rawStreamData streamCapture // 原始流数据捕获（滑动窗口限长）

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		h.ErrorResponse(c, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 准备选项
	options := map[string]interface{}{
		"model": model.Code,
	}

	upstreamStart := time.Now()
	err = adapter.ChatStream(c.Request.Context(), aiMessages, options, func(data map[string]interface{}) {
		// 检查是否完成
		if done, ok := data["done"].(bool); ok && done {
			c.Writer.WriteString("data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		// 将转换后的数据序列化为 JSON 并发送
		if jsonLine, err := json.Marshal(data); err == nil {
			line := "data: " + string(jsonLine)
			// 发送给客户端
			c.Writer.WriteString(line + "\n")
			flusher.Flush()

			// 保存原始数据（滑动窗口：头尾保留，中间以计数占位）
			rawStreamData.append(line)

			// 提取内容
			content := h.extractContent(data)
			if content != "" {
				fullContent.WriteString(content)
			}

			// 提取 token 使用量
			pt, ct, _ := adapter.ExtractTokenUsage(data)
			if pt > 0 {
				inputTokens = pt
			}
			if ct > 0 {
				outputTokens = ct
			}
		}
	})

	// 构建完整的请求数据（用于记录）
	requestData := map[string]interface{}{
		"model":    model.Code,
		"messages": messages,
		"stream":   true,
	}

	if err != nil {
		metrics.V1RequestsTotal.WithLabelValues("chat", "error").Inc()
		metrics.UpstreamDuration.WithLabelValues(provider.Code, "chat").Observe(time.Since(upstreamStart).Seconds())
		// 更新错误记录
		service.Accounting.UpdateErrorRecords(
			recordIDs,
			500,
			err.Error(),
			"POST",
			c.Request.URL.String(),
			c.Request.Header,
			requestData,
		)
		errorPayload, _ := json.Marshal(gin.H{"error": err.Error()})
		c.Writer.WriteString("data: " + string(errorPayload) + "\n\n")
		return
	}

	// 如果没有获取到token，进行估算
	if inputTokens == 0 {
		var inputText strings.Builder
		for _, msg := range messages {
			if content, ok := msg["content"].(string); ok {
				inputText.WriteString(content)
			}
		}
		inputTokens = inputText.Len() / 4 // 粗略估算
	}
	if outputTokens == 0 {
		outputTokens = fullContent.Len() / 4 // 粗略估算
	}
	totalTokens := inputTokens + outputTokens
	cost := utils.CalculateCost(inputTokens, outputTokens, model)

	metrics.V1RequestsTotal.WithLabelValues("chat", "success").Inc()
	metrics.UpstreamDuration.WithLabelValues(provider.Code, "chat").Observe(time.Since(upstreamStart).Seconds())
	observeTokens(model.Code, inputTokens, outputTokens, totalTokens)

	// 保存助手消息
	assistantMessage := models.ChatMessage{
		SessionID: session.ID,
		Role:      "assistant",
		Content:   fullContent.String(),
		Tokens:    totalTokens,
	}
	repository.DB.Create(&assistantMessage)

	// 构建完整响应
	finalResponse := map[string]interface{}{
		"type":       "streaming",
		"raw_stream": rawStreamData.joined(),
		"structured": map[string]interface{}{
			"id":      "chatcmpl-" + fmt.Sprintf("%d", time.Now().Unix()),
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   model.Code,
			"choices": []map[string]interface{}{{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": fullContent.String(),
				},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{
				"prompt_tokens":     inputTokens,
				"completion_tokens": outputTokens,
				"total_tokens":      totalTokens,
			},
		},
	}

	// 更新成功记录
	service.Accounting.UpdateSuccessRecords(
		recordIDs,
		200,
		finalResponse,
		inputTokens,
		outputTokens,
		totalTokens,
		cost,
		"POST",
		c.Request.URL.String(),
		c.Request.Header,
		requestData,
		model,
	)

	// 发送完成标记
	c.Writer.WriteString(fmt.Sprintf("data: {\"done\":true,\"message_id\":%d}\n\n", assistantMessage.ID))
	flusher.Flush()
}

// nonStreamResponse 非流式响应
func (h *ChatController) nonStreamResponse(c *gin.Context, model *models.AiModel, provider *models.AiProvider, apiKey *models.ProviderApiKey, session *models.ChatSession, messages []map[string]interface{}, user *models.User, recordIDs *service.RecordIDs) {
	// 使用 AdapterFactory 选择适配器
	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		h.ErrorResponse(c, "Failed to create adapter: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 转换消息格式为 Message 数组
	aiMessages := make([]ai.Message, len(messages))
	for i, msg := range messages {
		aiMessages[i] = ai.Message{
			Role:    msg["role"].(string),
			Content: msg["content"],
		}
	}

	// 准备选项
	options := map[string]interface{}{
		"model": model.Code,
	}

	upstreamStart := time.Now()
	response, err := adapter.Chat(c.Request.Context(), aiMessages, options)

	// 构建完整的请求数据（用于记录）
	requestData := map[string]interface{}{
		"model":    model.Code,
		"messages": messages,
		"stream":   false,
	}

	if err != nil {
		metrics.V1RequestsTotal.WithLabelValues("chat", "error").Inc()
		metrics.UpstreamDuration.WithLabelValues(provider.Code, "chat").Observe(time.Since(upstreamStart).Seconds())
		// 更新错误记录
		service.Accounting.UpdateErrorRecords(
			recordIDs,
			500,
			err.Error(),
			"POST",
			c.Request.URL.String(),
			c.Request.Header,
			requestData,
		)
		h.ErrorResponse(c, "Chat failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 解析响应
	result := h.parseResponse(response, provider)
	content := result["content"].(string)
	inputTokens := result["input_tokens"].(int)
	outputTokens := result["output_tokens"].(int)
	totalTokens := inputTokens + outputTokens
	cost := utils.CalculateCost(inputTokens, outputTokens, model)

	metrics.V1RequestsTotal.WithLabelValues("chat", "success").Inc()
	metrics.UpstreamDuration.WithLabelValues(provider.Code, "chat").Observe(time.Since(upstreamStart).Seconds())
	observeTokens(model.Code, inputTokens, outputTokens, totalTokens)

	// 保存助手消息
	assistantMessage := models.ChatMessage{
		SessionID: session.ID,
		Role:      "assistant",
		Content:   content,
		Tokens:    totalTokens,
	}
	repository.DB.Create(&assistantMessage)

	// 更新成功记录
	service.Accounting.UpdateSuccessRecords(
		recordIDs,
		200,
		response,
		inputTokens,
		outputTokens,
		totalTokens,
		cost,
		"POST",
		c.Request.URL.String(),
		c.Request.Header,
		requestData,
		model,
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data": gin.H{
			"session":      session,
			"message":      assistantMessage,
			"tool_results": []interface{}{},
		},
	})
}

// extractContent 从流式响应中提取内容
func (h *ChatController) extractContent(data map[string]interface{}) string {
	// OpenAI/阿里云格式: choices[0].delta.content
	if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok {
					return content
				}
			}
			// 非流式格式: choices[0].message.content
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					return content
				}
			}
		}
	}

	// 阿里云格式: output.choices[0].message.content
	if output, ok := data["output"].(map[string]interface{}); ok {
		if choices, ok := output["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if message, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						return content
					}
				}
			}
		}
	}

	return ""
}

// parseResponse 解析响应，提取内容和token使用量
func (h *ChatController) parseResponse(response map[string]interface{}, provider *models.AiProvider) map[string]interface{} {
	result := map[string]interface{}{
		"content":       "",
		"input_tokens":  0,
		"output_tokens": 0,
	}

	// 提取内容
	result["content"] = h.extractContent(response)

	// 提取token使用量
	if usage, ok := response["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			result["input_tokens"] = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			result["output_tokens"] = int(ct)
		}
		// 火山引擎格式
		if it, ok := usage["input_tokens"].(float64); ok {
			result["input_tokens"] = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok {
			result["output_tokens"] = int(ot)
		}
	}

	// 如果只有total_tokens，按比例估算
	if result["input_tokens"] == 0 && result["output_tokens"] == 0 {
		if usage, ok := response["usage"].(map[string]interface{}); ok {
			if total, ok := usage["total_tokens"].(float64); ok && total > 0 {
				// 假设输出占75%
				result["output_tokens"] = int(total * 0.75)
				result["input_tokens"] = int(total) - result["output_tokens"].(int)
			}
		}
	}

	return result
}

// findModelByID 通过ID查找模型
func (h *ChatController) findModelByID(modelID uint) (*models.AiModel, error) {
	var model models.AiModel
	if err := repository.DB.Where("id = ? AND status = ?", modelID, true).Preload("Provider").First(&model).Error; err != nil {
		return nil, fmt.Errorf("model not found")
	}
	return &model, nil
}

// findModel 通过字符串ID查找模型（支持 provider/model_code 格式）
func (h *ChatController) findModel(modelID string) (*models.AiModel, error) {
	// 支持 provider/model_code 格式
	if strings.Contains(modelID, "/") {
		parts := strings.SplitN(modelID, "/", 2)
		providerCode, modelCode := parts[0], parts[1]
		var provider models.AiProvider
		if err := repository.DB.Where("code = ? AND status = ?", providerCode, true).First(&provider).Error; err == nil {
			var model models.AiModel
			if err := repository.DB.Where("provider_id = ? AND code = ? AND status = ?", provider.ID, modelCode, true).Preload("Provider").First(&model).Error; err == nil {
				return &model, nil
			}
		}
	}

	// 尝试通过 code 查找
	var model models.AiModel
	if err := repository.DB.Where("code = ? AND status = ?", modelID, true).Preload("Provider").First(&model).Error; err == nil {
		return &model, nil
	}

	return nil, fmt.Errorf("model not found")
}

// generateTitle 生成会话标题
func (h *ChatController) generateTitle(messages []map[string]interface{}) string {
	for _, msg := range messages {
		if role, ok := msg["role"].(string); ok && role == "user" {
			if content, ok := msg["content"].(string); ok {
				content = strings.TrimSpace(content)
				if len(content) > 50 {
					// rune 边界安全截断，避免切断 UTF-8 多字节字符
					return service.TruncateUTF8(content, 50) + "..."
				}
				return content
			}
		}
	}
	return "新会话"
}
