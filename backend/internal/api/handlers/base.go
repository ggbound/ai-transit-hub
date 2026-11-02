package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/service"
	"github.com/your-org/ai-transit-hub/internal/utils"
)

// BaseHandler is a base handler struct
type BaseHandler struct{}

// GetAuthUser gets authenticated user from context
func GetAuthUser(c *gin.Context) (*models.User, bool) {
	if user, exists := c.Get("user"); exists {
		if u, ok := user.(*models.User); ok {
			return u, true
		}
	}
	return nil, false
}

// SuccessResponse returns a success response
func (h *BaseHandler) SuccessResponse(c *gin.Context, data interface{}, message ...string) {
	msg := "Success"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	code := 200
	if len(message) > 1 {
		code = 201
	}

	c.JSON(code, gin.H{
		"success": true,
		"message": msg,
		"data":    data,
	})
}

// SuccessResponseWithCode returns a success response with custom status code
func (h *BaseHandler) SuccessResponseWithCode(c *gin.Context, data interface{}, message string, code int) {
	c.JSON(code, gin.H{
		"success": true,
		"message": message,
		"data":    data,
	})
}

// ErrorResponse returns an error response
func (h *BaseHandler) ErrorResponse(c *gin.Context, message string, status int, errors ...interface{}) {
	response := gin.H{
		"success": false,
		"message": message,
	}
	if len(errors) > 0 {
		response["errors"] = errors[0]
	}
	c.JSON(status, response)
}

// ValidationErrorResponse returns validation error response
func (h *BaseHandler) ValidationErrorResponse(c *gin.Context, errors interface{}) {
	h.ErrorResponse(c, "Validation Error", http.StatusBadRequest, errors)
}

// streamRelay 封装 V1 流式透传的公共逻辑：
//   - SSE 响应头惰性设置：上游在流出数据前就失败时，调用方仍可用 c.JSON 返回真实状态码的 JSON 错误
//   - 逐行透传并实时 Flush
//   - 收集原始行用于记录回填：仅保留头尾各若干行（防超长流式响应把内存吃爆），中间行以计数占位
//   - 基于解析后的 JSON 结构检测错误，替代原先的关键词字符串匹配（避免正常内容包含 "BadRequest" 等词被误判）
type streamRelay struct {
	c          *gin.Context
	headersSet bool
	headLines  []string // 头部行：通常含 role/开始帧，对排查最有价值
	tailLines  []string // 尾部行：通常含结尾帧与 usage，对排查最有价值
	dropped    int      // 被裁掉的中间行数
	HasError   bool
	ErrMsg     string
	GotDone    bool // 已向客户端透传 [DONE] 终止帧（OpenAI 风格流式正常结束的标志）
}

// statusClientClosedRequest 客户端主动断开连接（nginx 惯例状态码）
const statusClientClosedRequest = 499

const (
	relayMaxHeadLines = 50
	relayMaxTailLines = 100
)

func newStreamRelay(c *gin.Context) *streamRelay {
	return &streamRelay{c: c}
}

// appendLine 以滑动窗口方式保留头尾行，中间行只计数不暂存
func (r *streamRelay) appendLine(line string) {
	if len(r.headLines) < relayMaxHeadLines {
		r.headLines = append(r.headLines, line)
		return
	}
	if len(r.tailLines) < relayMaxTailLines {
		r.tailLines = append(r.tailLines, line)
		return
	}
	// 尾部窗口已满：丢弃最旧的一行，腾出位置继续滚动保留最新的行
	r.dropped++
	r.tailLines = append(r.tailLines[1:], line)
}

// AllLines 返回用于记账回填的流式原始行：头 + 丢弃计数占位 + 尾。
// 占位行是 SSE 注释格式，usage 提取逻辑会按非 data 行自动跳过。
func (r *streamRelay) AllLines() []string {
	if r.dropped == 0 {
		out := make([]string, 0, len(r.headLines)+len(r.tailLines))
		out = append(out, r.headLines...)
		out = append(out, r.tailLines...)
		return out
	}
	out := make([]string, 0, len(r.headLines)+1+len(r.tailLines))
	out = append(out, r.headLines...)
	out = append(out, fmt.Sprintf(": %d lines dropped", r.dropped))
	out = append(out, r.tailLines...)
	return out
}

// OnLine 处理上游返回的一行：透传给客户端并做结构化错误检测。
// onChunk 在每解析出一个 JSON 数据块时回调（用于提取 usage 等），可为 nil。
func (r *streamRelay) OnLine(line string, onChunk func(decoded map[string]interface{})) {
	// 首次写出前才设置 SSE 头，保证上游先报错时还能返回 JSON 错误响应
	if !r.headersSet {
		r.c.Header("Content-Type", "text/event-stream")
		r.c.Header("Cache-Control", "no-cache")
		r.c.Header("Connection", "keep-alive")
		r.c.Header("X-Accel-Buffering", "no")
		r.headersSet = true
	}
	r.c.Writer.WriteString(line + "\n")
	if flusher, ok := r.c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}

	r.appendLine(line)

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if trimmed == "[DONE]" {
		r.GotDone = true
		return
	}
	payload := trimmed
	if strings.HasPrefix(payload, "data:") {
		payload = strings.TrimSpace(strings.TrimPrefix(payload, "data:"))
	}
	// 只解析 JSON 对象行，SSE 注释行（: keep-alive）等直接跳过
	if !strings.HasPrefix(payload, "{") {
		return
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return
	}
	if onChunk != nil {
		onChunk(decoded)
	}
	if !r.HasError {
		if isErr, errMsg := utils.IsErrorResponse(decoded); isErr {
			r.HasError = true
			r.ErrMsg = errMsg
		}
	}
}

// passthroughErrStatus 返回应记录/返回给客户端的状态码：上游错误用真实状态码，
// 客户端主动断开用 499（nginx 惯例的 client closed request），其余 500
func passthroughErrStatus(err error) int {
	if errors.Is(err, context.Canceled) {
		return statusClientClosedRequest
	}
	if ue, ok := ai.AsUpstreamError(err); ok {
		return ue.StatusCode
	}
	return http.StatusInternalServerError
}

// isClientDisconnected 判断错误是否由客户端断开连接引起（请求 context 被取消）
func isClientDisconnected(err error) bool {
	return errors.Is(err, context.Canceled)
}

// respondPassthroughErr 向客户端返回透传错误：上游错误透传真实状态码与错误体，其余返回 OpenAI 风格 500。
// 客户端已断开（context canceled）时无需也无法写响应，直接跳过。
func respondPassthroughErr(c *gin.Context, err error) {
	if isClientDisconnected(err) {
		return
	}
	if ue, ok := ai.AsUpstreamError(err); ok {
		if ue.Body != nil {
			c.JSON(ue.StatusCode, ue.Body)
		} else {
			// 上游无错误体时的兜底信封；code 用字符串对齐 OpenAI 官方惯例
			c.JSON(ue.StatusCode, gin.H{"error": gin.H{"message": "upstream error", "type": "api_error", "code": "upstream_error"}})
		}
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "api_error"}})
}

// handlePassthroughError 非流式透传失败时的统一处理：
// 更新错误记录（含上游真实状态码）并向客户端返回错误响应
func handlePassthroughError(c *gin.Context, err error, recordIDs *service.RecordIDs, method, endpointURL string, requestHeaders map[string][]string, requestBody interface{}) {
	service.Accounting.UpdateErrorRecords(recordIDs, passthroughErrStatus(err), err.Error(), method, endpointURL, requestHeaders, requestBody)
	respondPassthroughErr(c, err)
}

// HandleStreamError 统一处理流式请求失败：更新错误记录（含上游真实状态码）；
// 若尚未向客户端写出任何数据（headersSet=false），返回 JSON 错误——上游错误透传真实状态码
func (r *streamRelay) HandleStreamError(err error, recordIDs *service.RecordIDs, method, endpointURL string, requestHeaders map[string][]string, requestBody interface{}) {
	service.Accounting.UpdateErrorRecords(recordIDs, passthroughErrStatus(err), err.Error(), method, endpointURL, requestHeaders, requestBody)

	// SSE 流已开始（或客户端已断开）时无法再改写状态码
	if r.headersSet {
		return
	}
	respondPassthroughErr(r.c, err)
}

// extractUsageTokens 从 usage 对象提取 token 数（兼容 OpenAI 与 Anthropic 的字段命名）
func extractUsageTokens(usage map[string]interface{}) (promptTokens, completionTokens, totalTokens int) {
	if usage == nil {
		return 0, 0, 0
	}
	promptTokens = ai.ExtractIntFromMap(usage, "prompt_tokens", "input_tokens")
	completionTokens = ai.ExtractIntFromMap(usage, "completion_tokens", "output_tokens")
	totalTokens = ai.ExtractIntFromMap(usage, "total_tokens")
	if totalTokens == 0 {
		totalTokens = promptTokens + completionTokens
	}
	return
}
