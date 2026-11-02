package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"gorm.io/gorm"
)

// ApiTestHandler 处理 API 测试相关请求
type ApiTestHandler struct {
	BaseHandler
}

// NewApiTestHandler 创建 ApiTestHandler 实例
func NewApiTestHandler() *ApiTestHandler {
	return &ApiTestHandler{}
}

// Store 保存接口测试请求记录
func (h *ApiTestHandler) Store(c *gin.Context) {
	var req struct {
		ApiType         string      `json:"api_type"`
		Endpoint        string      `json:"endpoint"`
		EndpointUrl     string      `json:"endpoint_url"`
		Method          string      `json:"method"`
		Model           string      `json:"model"`
		Status          int         `json:"status"`
		Duration        int         `json:"duration"`
		ErrorMessage    string      `json:"error_message"`
		RequestHeaders  interface{} `json:"request_headers"`
		RequestBody     interface{} `json:"request_body"`
		ResponseHeaders interface{} `json:"response_headers"`
		ResponseBody    interface{} `json:"response_body"`
		Type            string      `json:"type"`
		UserApiKeyID    uint        `json:"user_api_key_id"`
		UsageLogID      uint        `json:"usage_log_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// endpoint_url 落库列为 varchar(191)，超长直接报错（截断 URL 无意义）
	if len([]rune(req.EndpointUrl)) > 191 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "endpoint_url 长度不能超过 191 个字符"})
		return
	}

	user, _ := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
		return
	}

	externalTaskId := ""
	if req.ResponseBody != nil {
		if responseBody, ok := req.ResponseBody.(map[string]interface{}); ok {
			if id, ok := responseBody["id"].(string); ok {
				externalTaskId = id
			} else if id, ok := responseBody["task_id"].(string); ok {
				externalTaskId = id
			} else if id, ok := responseBody["taskId"].(string); ok {
				externalTaskId = id
			}
		}
	}

	var requestHeadersStr string
	if req.RequestHeaders != nil {
		if b, err := json.Marshal(req.RequestHeaders); err == nil {
			requestHeadersStr = string(b)
		}
	}

	var requestBodyStr string
	if req.RequestBody != nil {
		if b, err := json.Marshal(req.RequestBody); err == nil {
			requestBodyStr = string(b)
		}
	}

	var responseHeadersStr string
	if req.ResponseHeaders != nil {
		if b, err := json.Marshal(req.ResponseHeaders); err == nil {
			responseHeadersStr = string(b)
		}
	}

	var responseBodyStr string
	if req.ResponseBody != nil {
		if b, err := json.Marshal(req.ResponseBody); err == nil {
			responseBodyStr = string(b)
		}
	}

	usageLogId := req.UsageLogID
	if usageLogId == 0 {
		// request_time 为 NOT NULL 列，必须显式赋值，否则 Error 1048
		now := time.Now()
		usageLog := models.UsageLog{
			UserID:      user.ID,
			Status:      2,
			RequestTime: &now,
		}
		if err := repository.DB.Create(&usageLog).Error; err != nil {
			h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
			return
		}
		usageLogId = usageLog.ID
	}

	apiTestRequest := models.ApiTestRequest{
		UserID:         user.ID,
		ApiType:        req.ApiType,
		Endpoint:       req.Endpoint,
		EndpointUrl:    req.EndpointUrl,
		Method:         req.Method,
		Model:          req.Model,
		Status:         req.Status,
		Duration:       req.Duration,
		ErrorMessage:   req.ErrorMessage,
		Type:           req.Type,
		ExternalTaskID: externalTaskId,
		UserApiKeyID:   req.UserApiKeyID,
		UsageLogID:     usageLogId,
	}

	if err := repository.DB.Create(&apiTestRequest).Error; err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTestDetail := models.ApiTestDetail{
		RequestID:       apiTestRequest.ID,
		RequestHeaders:  requestHeadersStr,
		RequestBody:     requestBodyStr,
		ResponseHeaders: responseHeadersStr,
		ResponseBody:    responseBodyStr,
	}

	if err := repository.DB.Create(&apiTestDetail).Error; err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		return
	}

	// 同步 usage_log 状态：单次 Updates 替代 First+Save 两趟往返
	if usageLogId > 0 {
		updates := map[string]interface{}{
			"status": 0,
		}
		if apiTestRequest.Status < 400 {
			updates["status"] = 1
		}
		if apiTestRequest.ErrorMessage != "" {
			updates["error_message"] = apiTestRequest.ErrorMessage
		}
		if err := repository.DB.Model(&models.UsageLog{}).Where("id = ?", usageLogId).Updates(updates).Error; err != nil {
			logger.Error("Failed to sync usage_log %d status: %v", usageLogId, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           apiTestRequest.ID,
			"usage_log_id": usageLogId,
		},
	})
}

// History 获取用户的测试历史记录
func (h *ApiTestHandler) History(c *gin.Context) {
	user, _ := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
		return
	}

	perPageStr := c.Query("per_page")
	pageStr := c.Query("page")
	apiType := c.Query("api_type")
	statusFilter := c.Query("status")
	typeFilter := c.Query("type")

	perPage := 10
	if perPageStr != "" {
		if p, err := strconv.Atoi(perPageStr); err == nil {
			perPage = p
		}
	}

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			page = p
		}
	}

	var count int64
	query := repository.DB.Model(&models.ApiTestRequest{}).Where("user_id = ?", user.ID)

	if apiType != "" {
		query = query.Where("api_type = ?", apiType)
	}
	if statusFilter != "" {
		if statusFilter == "success" {
			query = query.Where("status < 400")
		} else if statusFilter == "error" {
			query = query.Where("status >= 400")
		}
	}
	if typeFilter != "" {
		query = query.Where("type = ?", typeFilter)
	}

	query.Count(&count)

	var requests []models.ApiTestRequest
	offset := (page - 1) * perPage
	query.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&requests)

	totalPages := (int(count) + perPage - 1) / perPage

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"current_page":   page,
			"data":           requests,
			"first_page_url": nil,
			"from":           offset + 1,
			"last_page":      totalPages,
			"last_page_url":  nil,
			"links":          []interface{}{},
			"next_page_url":  nil,
			"path":           c.Request.URL.Path,
			"per_page":       perPage,
			"prev_page_url":  nil,
			"to":             offset + len(requests),
			"total":          count,
		},
	})
}

// Show 获取详细的请求和响应信息
func (h *ApiTestHandler) Show(c *gin.Context) {
	user, _ := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "无效的 ID"})
		return
	}

	var apiRequest models.ApiTestRequest
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiRequest).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "记录不存在"})
		} else {
			h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var detail models.ApiTestDetail
	repository.DB.Where("request_id = ?", apiRequest.ID).First(&detail)

	result := map[string]interface{}{
		"id":               apiRequest.ID,
		"user_id":          apiRequest.UserID,
		"api_type":         apiRequest.ApiType,
		"endpoint":         apiRequest.Endpoint,
		"endpoint_url":     apiRequest.EndpointUrl,
		"method":           apiRequest.Method,
		"model":            apiRequest.Model,
		"status":           apiRequest.Status,
		"duration":         apiRequest.Duration,
		"error_message":    apiRequest.ErrorMessage,
		"type":             apiRequest.Type,
		"external_task_id": apiRequest.ExternalTaskID,
		"user_api_key_id":  apiRequest.UserApiKeyID,
		"usage_log_id":     apiRequest.UsageLogID,
		"created_at":       apiRequest.CreatedAt,
		"updated_at":       apiRequest.UpdatedAt,
		"detail":           detail,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// Update 更新请求记录
func (h *ApiTestHandler) Update(c *gin.Context) {
	user, _ := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "无效的 ID"})
		return
	}

	var req struct {
		Status          int         `json:"status"`
		Duration        int         `json:"duration"`
		ErrorMessage    string      `json:"error_message"`
		ResponseHeaders interface{} `json:"response_headers"`
		ResponseBody    interface{} `json:"response_body"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	var apiRequest models.ApiTestRequest
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiRequest).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "记录不存在"})
		} else {
			h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if req.Status != 0 {
		apiRequest.Status = req.Status
	}
	if req.Duration != 0 {
		apiRequest.Duration = req.Duration
	}
	if req.ErrorMessage != "" {
		apiRequest.ErrorMessage = req.ErrorMessage
	}

	if err := repository.DB.Save(&apiRequest).Error; err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		return
	}

	detailUpdate := map[string]interface{}{}
	if req.ResponseHeaders != nil {
		if b, err := json.Marshal(req.ResponseHeaders); err == nil {
			detailUpdate["response_headers"] = string(b)
		}
	}
	if req.ResponseBody != nil {
		if b, err := json.Marshal(req.ResponseBody); err == nil {
			detailUpdate["response_body"] = string(b)
		}
	}

	if len(detailUpdate) > 0 {
		repository.DB.Model(&models.ApiTestDetail{}).Where("request_id = ?", apiRequest.ID).Updates(detailUpdate)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
	})
}

// Destroy 删除记录
func (h *ApiTestHandler) Destroy(c *gin.Context) {
	user, _ := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "未登录"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "无效的 ID"})
		return
	}

	var apiRequest models.ApiTestRequest
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiRequest).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "记录不存在"})
		} else {
			h.ErrorResponse(c, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	repository.DB.Where("request_id = ?", apiRequest.ID).Delete(&models.ApiTestDetail{})
	repository.DB.Delete(&apiRequest)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}
