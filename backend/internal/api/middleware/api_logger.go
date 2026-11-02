package middleware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/logger"
)

// APILoggerMiddleware 记录 API 请求的详细日志
func APILoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录 /v1/ 或 /api/v1/ 路径的请求
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/") && !strings.HasPrefix(path, "/api/v1/") {
			c.Next()
			return
		}

		startTime := time.Now()

		// 读取请求体（限制大小，防止超大请求体耗尽内存）
		var requestBody []byte
		if c.Request.Body != nil {
			var readErr error
			requestBody, readErr = io.ReadAll(io.LimitReader(c.Request.Body, maxRequestBodySize+1))
			if readErr != nil {
				c.Request.Body.Close()
				logger.Error("Failed to read request body: %v", readErr)
			} else if len(requestBody) > maxRequestBodySize {
				// v1 契约统一 OpenAI 错误信封（本中间件仅挂在 /v1 与 /api/v1 路径）
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{
					"message": "request body too large",
					"type":    "invalid_request_error",
					"code":    "request_too_large",
				}})
				return
			} else {
				// 重新设置请求体，以便后续处理
				c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		}

		// 创建响应捕获器
		responseWriter := &responseBodyWriter{
			body:           &bytes.Buffer{},
			maxBytes:       maxCapturedBodySize,
			ResponseWriter: c.Writer,
		}
		originalWriter := c.Writer
		c.Writer = responseWriter

		// 获取 API Key（从 header 中）
		apiKey := c.GetHeader("Authorization")
		if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
			apiKey = apiKey[7:]
		}
		if len(apiKey) > 20 {
			apiKey = apiKey[:10] + "..." + apiKey[len(apiKey)-5:]
		}

		// 获取用户（如果已认证）
		userID := "unknown"
		if user, exists := c.Get("user"); exists {
			if u, ok := user.(interface{ GetID() uint }); ok {
				userID = fmt.Sprintf("%d", u.GetID())
			}
		}

		// 记录请求开始
		logger.Debug("[API-START] Path: %s | Method: %s | ClientIP: %s | APIKey: %s | UserID: %s",
			path,
			c.Request.Method,
			c.ClientIP(),
			apiKey,
			userID,
		)

		// 记录请求体（如果是 POST/PUT）
		if len(requestBody) > 0 && (c.Request.Method == "POST" || c.Request.Method == "PUT") {
			// 截断过长的请求体
			bodyStr := string(requestBody)
			if len(bodyStr) > 1000 {
				bodyStr = bodyStr[:1000] + "...(truncated)"
			}
			// 提取 model 字段只需解析前 4KB，避免为 Debug 日志全量 Unmarshal 大请求体
			sample := requestBody
			if len(sample) > 4096 {
				sample = sample[:4096]
			}
			var jsonData map[string]interface{}
			if err := json.Unmarshal(sample, &jsonData); err == nil {
				model, _ := jsonData["model"].(string)
				logger.Debug("[API-REQUEST] Model: %s | Body: %s", model, bodyStr)
			} else {
				logger.Debug("[API-REQUEST] Body: %s", bodyStr)
			}
		}

		// 处理请求
		c.Next()

		// 恢复原始 writer
		c.Writer = originalWriter

		// 计算处理时间
		duration := time.Since(startTime)

		// 获取响应体
		responseBody := responseWriter.body.Bytes()
		responseStr := string(responseBody)
		if len(responseStr) > 500 {
			responseStr = responseStr[:500] + "...(truncated)"
		}

		// 获取错误信息
		statusCode := responseWriter.Status()
		errorMsg := ""
		if len(c.Errors) > 0 {
			errorMsg = c.Errors.String()
		}

		// 记录响应
		if statusCode >= 400 || errorMsg != "" {
			logger.Error("[API-ERROR] Path: %s | Status: %d | Duration: %v | Error: %s | Response: %s",
				path,
				statusCode,
				duration,
				errorMsg,
				responseStr,
			)
		} else {
			logger.Debug("[API-SUCCESS] Path: %s | Status: %d | Duration: %v | Response: %s",
				path,
				statusCode,
				duration,
				responseStr,
			)
		}
	}
}

// maxCapturedBodySize 响应体捕获上限：日志只需前 64KB，
// 防止大响应/长 SSE 流被完整复制进内存（原先无上限，流式响应会存 3 份）
const maxCapturedBodySize = 64 * 1024

// maxRequestBodySize 请求体读取上限：请求体是后续 handler 要解析的真实业务数据，
// 超过 10MB 直接拒绝（413），避免无上限读入内存
const maxRequestBodySize = 10 << 20

// responseBodyWriter 捕获响应体的 writer（带捕获上限）
type responseBodyWriter struct {
	gin.ResponseWriter
	body     *bytes.Buffer
	maxBytes int
}

// capture 捕获响应体片段到日志缓冲（达到上限后停止捕获，但不影响响应本身写出）
func (w *responseBodyWriter) capture(b []byte) {
	if w.body.Len() < w.maxBytes {
		remaining := w.maxBytes - w.body.Len()
		if len(b) > remaining {
			w.body.Write(b[:remaining])
		} else {
			w.body.Write(b)
		}
	}
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

// WriteString 只捕获一次并写出一次。
// 注意：不能先调 w.Write 再调 ResponseWriter.WriteString——前者已向客户端写出，
// 再调后者会导致流式响应（SSE 走 WriteString）每行重复输出两次。
func (w *responseBodyWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// Flush 实现 http.Flusher 接口，支持流式响应
func (w *responseBodyWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// CloseNotify 实现 http.CloseNotifier 接口
func (w *responseBodyWriter) CloseNotify() <-chan bool {
	if notifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	// 返回一个永不关闭的 channel 作为 fallback
	return make(<-chan bool)
}

// Hijack 实现 http.Hijacker 接口
func (w *responseBodyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijack not supported")
}
