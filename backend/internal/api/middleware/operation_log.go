package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/service"
)

// 管理端审计捕获上限：管理表单请求/响应均很小，8KB 足够
const (
	opLogMaxBody   = 8 << 10
	opLogMaxParams = 6 << 10
)

// OperationLogMiddleware 管理后台写操作审计中间件：
// 自动记录 POST/PUT/PATCH/DELETE 请求的操作人、模块、动作、请求参数与响应结果。
// 只读请求（GET）不记录；记录失败不影响主流程。
func OperationLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
			c.Next()
			return
		}

		// 读取请求体（限流并还原，供后续 handler 正常解析）
		var reqBody []byte
		if c.Request.Body != nil {
			body, err := io.ReadAll(io.LimitReader(c.Request.Body, opLogMaxBody+1))
			if err == nil {
				if len(body) > opLogMaxBody {
					body = body[:opLogMaxBody]
				}
				reqBody = body
				c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			} else {
				c.Request.Body.Close()
				c.Request.Body = http.NoBody
			}
		}

		// 捕获响应体（截断）
		respWriter := &opLogResponseWriter{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		originalWriter := c.Writer
		c.Writer = respWriter

		c.Next()

		c.Writer = originalWriter

		recordOperationLog(c, method, reqBody, respWriter)
	}
}

// recordOperationLog 组装并异步落库一条管理端操作日志
func recordOperationLog(c *gin.Context, method string, reqBody []byte, respWriter *opLogResponseWriter) {
	entry := service.OperationLogEntry{
		Type:      deriveOperationType(method, c.Request.URL.Path),
		Module:    deriveOperationModule(c.Request.URL.Path),
		Action:    method + " " + strings.TrimPrefix(c.Request.URL.Path, "/api/admin"),
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}

	if user, ok := GetUser(c); ok {
		uid := user.ID
		entry.UserID = &uid
		entry.Username = user.Name
	}

	// 请求参数：合法 JSON 记对象，否则记原始文本
	if len(reqBody) > 0 {
		var params map[string]any
		if err := json.Unmarshal(reqBody, &params); err == nil {
			entry.Params = params
		} else {
			entry.Params = string(reqBody)
		}
	} else {
		// 无请求体的写操作（如 DELETE、toggle），记录路径参数便于定位资源
		entry.Params = gin.H{"path": c.Request.URL.Path, "method": method}
	}

	// 响应结果与状态
	respBody := respWriter.buf.Bytes()
	if len(respBody) > 0 {
		var result any
		if err := json.Unmarshal(respBody, &result); err == nil {
			entry.Result = result
			// 失败时尝试提取响应中的 message 作为错误信息
			if respWriter.Status() >= http.StatusBadRequest {
				entry.Status = "failed"
				entry.ErrorMsg = extractRespMessage(result)
			}
		} else if len(respBody) > opLogMaxParams {
			respBody = respBody[:opLogMaxParams]
		} else if respWriter.Status() >= http.StatusBadRequest {
			entry.Status = "failed"
			entry.ErrorMsg = string(respBody)
		}
	}
	if entry.Result != nil && len(respWriter.buf.Bytes()) > opLogMaxParams {
		// 超长响应仅保留截断标记，避免日志膨胀
		entry.Result = gin.H{"truncated": true, "raw": string(respWriter.buf.Bytes()[:opLogMaxParams])}
	}

	service.Operation.Record(entry)
}

// deriveOperationType 由请求方法与路径推导操作类型
func deriveOperationType(method, path string) string {
	if strings.HasSuffix(path, "/toggle-status") {
		return "toggle"
	}
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodDelete:
		return "delete"
	default:
		return "update"
	}
}

// deriveOperationModule 从路径解析操作模块：/api/admin/billing/rates/10 → billing
func deriveOperationModule(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/admin/")
	if trimmed == path {
		return "admin"
	}
	if idx := strings.Index(trimmed, "/"); idx > 0 {
		return trimmed[:idx]
	}
	if trimmed != "" {
		return trimmed
	}
	return "admin"
}

// extractRespMessage 从失败响应 JSON 中提取 message 字段
func extractRespMessage(result any) string {
	switch v := result.(type) {
	case map[string]any:
		if msg, ok := v["message"].(string); ok && msg != "" {
			return msg
		}
	case string:
		return v
	}
	return ""
}

// opLogResponseWriter 捕获响应体的 writer（带捕获上限）
type opLogResponseWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *opLogResponseWriter) capture(b []byte) {
	if w.buf.Len() < opLogMaxBody {
		remaining := opLogMaxBody - w.buf.Len()
		if len(b) > remaining {
			w.buf.Write(b[:remaining])
		} else {
			w.buf.Write(b)
		}
	}
}

func (w *opLogResponseWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *opLogResponseWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}
