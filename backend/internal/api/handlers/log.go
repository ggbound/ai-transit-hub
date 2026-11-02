package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type LogController struct {
	BaseHandler
}

func NewLogController() *LogController {
	return &LogController{}
}

// UserUsageLogs gets user's usage logs
func (h *LogController) UserUsageLogs(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var logs []models.UsageLog
	var total int64

	query := repository.DB.Where("user_id = ?", user.ID).Preload("Model").Preload("Model.Provider")
	query.Model(&models.UsageLog{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs)

	result := gin.H{
		"data": logs,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// UserOperationLogs gets user's operation logs
func (h *LogController) UserOperationLogs(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var logs []models.OperationLog
	var total int64

	query := repository.DB.Where("user_id = ?", user.ID).Preload("OperationType")
	query.Model(&models.OperationLog{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs)

	result := gin.H{
		"data": logs,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// AllUsageLogs gets all usage logs (admin only)
func (h *LogController) AllUsageLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var logs []models.UsageLog
	var total int64

	query := repository.DB.Preload("User").Preload("Model").Preload("Model.Provider")
	query.Model(&models.UsageLog{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs)

	result := gin.H{
		"data": logs,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// AllOperationLogs gets all operation logs (admin only)
func (h *LogController) AllOperationLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var logs []models.OperationLog
	var total int64

	query := repository.DB.Preload("User").Preload("OperationType")
	query.Model(&models.OperationLog{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs)

	result := gin.H{
		"data": logs,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// UserUsageStats gets user's usage statistics
func (h *LogController) UserUsageStats(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	now := time.Now()
	// 正确计算当天开始：00:00:00 而不是从现在往前推24小时
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())

	var todayCalls int64
	var todayTokens int64
	var monthCalls int64
	var monthTokens int64
	var totalCalls int64
	var totalTokens int64
	var totalSessions int64

	repository.DB.Model(&models.UsageLog{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", user.ID, today, today.Add(24*time.Hour)).Count(&todayCalls)
	repository.DB.Model(&models.UsageLog{}).Where("user_id = ? AND created_at >= ? AND created_at < ?", user.ID, today, today.Add(24*time.Hour)).Select("COALESCE(SUM(total_tokens), 0)").Scan(&todayTokens)

	repository.DB.Model(&models.UsageLog{}).Where("user_id = ? AND created_at >= ?", user.ID, startOfMonth).Count(&monthCalls)
	repository.DB.Model(&models.UsageLog{}).Where("user_id = ? AND created_at >= ?", user.ID, startOfMonth).Select("COALESCE(SUM(total_tokens), 0)").Scan(&monthTokens)

	repository.DB.Model(&models.UsageLog{}).Where("user_id = ?", user.ID).Count(&totalCalls)
	repository.DB.Model(&models.UsageLog{}).Where("user_id = ?", user.ID).Select("COALESCE(SUM(total_tokens), 0)").Scan(&totalTokens)

	repository.DB.Model(&models.ChatSession{}).Where("user_id = ?", user.ID).Count(&totalSessions)

	result := gin.H{
		"todayCalls":    int(todayCalls),
		"todayTokens":   int(todayTokens),
		"monthCalls":    int(monthCalls),
		"monthTokens":   int(monthTokens),
		"totalCalls":    int(totalCalls),
		"totalTokens":   int(totalTokens),
		"totalSessions": int(totalSessions),
	}

	h.SuccessResponse(c, result)
}
