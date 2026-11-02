package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"gorm.io/gorm"
)

type LogController struct {
	BaseHandler
}

func NewLogController() *LogController {
	return &LogController{}
}

// parsePageParams 解析分页参数
func (h *LogController) parsePageParams(c *gin.Context) (page, perPage int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ = strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

// parseDateRange 解析 start_date/end_date（YYYY-MM-DD）为时间范围，均可为空
func (h *LogController) parseDateRange(c *gin.Context) (start, end *time.Time) {
	if v := strings.TrimSpace(c.Query("start_date")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			start = &t
		}
	}
	if v := strings.TrimSpace(c.Query("end_date")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			// 结束日期取当天末尾（次日零点前），保证闭区间语义
			e := t.AddDate(0, 0, 1)
			end = &e
		}
	}
	return start, end
}

// paginate 执行 Count + 分页查询并返回统一响应
func (h *LogController) paginate(c *gin.Context, query *gorm.DB, dest any, page, perPage int) {
	var total int64
	query.Model(dest).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(dest)

	h.SuccessResponse(c, gin.H{
		"data": dest,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	})
}

// UserUsageLogs gets user's usage logs
func (h *LogController) UserUsageLogs(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, perPage := h.parsePageParams(c)

	query := repository.DB.Where("user_id = ?", user.ID).Preload("Model").Preload("Model.Provider")

	// 搜索筛选：模型 / 状态 / 时间范围
	if modelID, _ := strconv.Atoi(c.Query("model_id")); modelID > 0 {
		query = query.Where("model_id = ?", modelID)
	}
	if s := c.Query("status"); s != "" {
		if status, err := strconv.Atoi(s); err == nil {
			query = query.Where("status = ?", status)
		}
	}
	if start, _ := h.parseDateRange(c); start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if _, end := h.parseDateRange(c); end != nil {
		query = query.Where("created_at < ?", *end)
	}

	logs := make([]models.UsageLog, 0)
	h.paginate(c, query, &logs, page, perPage)
}

// UserOperationLogs gets user's operation logs
func (h *LogController) UserOperationLogs(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, perPage := h.parsePageParams(c)

	query := repository.DB.Where("user_id = ?", user.ID)

	// 搜索筛选：操作类型 / 模块 / 状态 / 时间范围
	if t := strings.TrimSpace(c.Query("operation_type")); t != "" {
		query = query.Where("operation_type = ?", t)
	}
	if m := strings.TrimSpace(c.Query("operation_module")); m != "" {
		query = query.Where("operation_module = ?", m)
	}
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		query = query.Where("status = ?", s)
	}
	if start, _ := h.parseDateRange(c); start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if _, end := h.parseDateRange(c); end != nil {
		query = query.Where("created_at < ?", *end)
	}

	logs := make([]models.OperationLog, 0)
	h.paginate(c, query, &logs, page, perPage)
}

// AllUsageLogs gets all usage logs (admin only)
func (h *LogController) AllUsageLogs(c *gin.Context) {
	page, perPage := h.parsePageParams(c)

	query := repository.DB.Preload("User").Preload("Model").Preload("Model.Provider")

	// 搜索筛选：关键字（用户名/邮箱/模型名）、用户、模型、状态、时间范围
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		kw := "%" + keyword + "%"
		userSub := repository.DB.Model(&models.User{}).Select("id").Where("name LIKE ? OR email LIKE ?", kw, kw)
		modelSub := repository.DB.Model(&models.AiModel{}).Select("id").Where("code LIKE ? OR name LIKE ?", kw, kw)
		query = query.Where("user_id IN (?) OR model_id IN (?)", userSub, modelSub)
	}
	if userID, _ := strconv.Atoi(c.Query("user_id")); userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if modelID, _ := strconv.Atoi(c.Query("model_id")); modelID > 0 {
		query = query.Where("model_id = ?", modelID)
	}
	if s := c.Query("status"); s != "" {
		if status, err := strconv.Atoi(s); err == nil {
			query = query.Where("status = ?", status)
		}
	}
	if start, _ := h.parseDateRange(c); start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if _, end := h.parseDateRange(c); end != nil {
		query = query.Where("created_at < ?", *end)
	}

	logs := make([]models.UsageLog, 0)
	h.paginate(c, query, &logs, page, perPage)
}

// AllOperationLogs gets all operation logs (admin only)
func (h *LogController) AllOperationLogs(c *gin.Context) {
	page, perPage := h.parsePageParams(c)

	query := repository.DB.Preload("User")

	// 搜索筛选：关键字（用户名/邮箱）、用户、操作类型、模块、状态、时间范围
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		kw := "%" + keyword + "%"
		userSub := repository.DB.Model(&models.User{}).Select("id").Where("name LIKE ? OR email LIKE ?", kw, kw)
		query = query.Where("username LIKE ? OR user_id IN (?)", kw, userSub)
	}
	if userID, _ := strconv.Atoi(c.Query("user_id")); userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if t := strings.TrimSpace(c.Query("operation_type")); t != "" {
		query = query.Where("operation_type = ?", t)
	}
	if m := strings.TrimSpace(c.Query("operation_module")); m != "" {
		query = query.Where("operation_module = ?", m)
	}
	if s := strings.TrimSpace(c.Query("status")); s != "" {
		query = query.Where("status = ?", s)
	}
	if start, _ := h.parseDateRange(c); start != nil {
		query = query.Where("created_at >= ?", *start)
	}
	if _, end := h.parseDateRange(c); end != nil {
		query = query.Where("created_at < ?", *end)
	}

	logs := make([]models.OperationLog, 0)
	h.paginate(c, query, &logs, page, perPage)
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
