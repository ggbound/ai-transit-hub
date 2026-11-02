package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"gorm.io/gorm"
)

// LeaderboardHandler handles leaderboard operations
type LeaderboardHandler struct {
	SuccessResponse func(*gin.Context, any, string, int)
	ErrorResponse   func(*gin.Context, string, int)
}

// NewLeaderboardHandler creates a new LeaderboardHandler
func NewLeaderboardHandler() *LeaderboardHandler {
	return &LeaderboardHandler{
		SuccessResponse: func(c *gin.Context, data any, message string, status int) {
			c.JSON(status, gin.H{
				"success": true,
				"message": message,
				"data":    data,
			})
		},
		ErrorResponse: func(c *gin.Context, message string, status int) {
			c.JSON(status, gin.H{
				"success": false,
				"message": message,
			})
		},
	}
}

// Index returns all leaderboards
func (h *LeaderboardHandler) Index(c *gin.Context) {
	var configs []models.LeaderboardConfig
	if err := repository.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch leaderboards", http.StatusInternalServerError)
		return
	}

	leaderboards := make([]map[string]any, 0)
	for _, config := range configs {
		data := h.getLeaderboardData(&config)
		leaderboards = append(leaderboards, map[string]any{
			"config": config,
			"data":   data,
		})
	}

	h.SuccessResponse(c, leaderboards, "Success", http.StatusOK)
}

// Show returns a single leaderboard
func (h *LeaderboardHandler) Show(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.LeaderboardConfig
	if err := repository.DB.Where("id = ? AND enabled = ?", id, true).First(&config).Error; err != nil {
		h.ErrorResponse(c, "Leaderboard not found", http.StatusNotFound)
		return
	}

	data := h.getLeaderboardData(&config)
	h.SuccessResponse(c, gin.H{
		"config": config,
		"data":   data,
	}, "Success", http.StatusOK)
}

// Configs returns all leaderboard configs (admin only)
func (h *LeaderboardHandler) Configs(c *gin.Context) {
	perPage := 20
	if pp := c.Query("per_page"); pp != "" {
		if p, err := strconv.Atoi(pp); err == nil && p > 0 {
			perPage = p
		}
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if pg, err := strconv.Atoi(p); err == nil && pg > 0 {
			page = pg
		}
	}

	var configs []models.LeaderboardConfig
	var total int64
	offset := (page - 1) * perPage

	if err := repository.DB.Model(&models.LeaderboardConfig{}).Count(&total).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch configs", http.StatusInternalServerError)
		return
	}

	if err := repository.DB.Order("created_at DESC").Offset(offset).Limit(perPage).Find(&configs).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch configs", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{
		"data":  configs,
		"total": total,
		"page":  page,
		"per_page": perPage,
	}, "Success", http.StatusOK)
}

// StoreConfig creates a new leaderboard config (admin only)
func (h *LeaderboardHandler) StoreConfig(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		DisplayName string `json:"display_name" binding:"required"`
		Metric      string `json:"metric" binding:"required,oneof=total_tokens total_cost request_count"`
		Period      string `json:"period" binding:"required,oneof=daily weekly monthly"`
		Limit       int    `json:"limit" binding:"required,min=1,max=100"`
		Enabled     bool   `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ErrorResponse(c, "Validation error", http.StatusBadRequest)
		return
	}

	config := models.LeaderboardConfig{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Metric:      req.Metric,
		Period:      req.Period,
		Limit:       req.Limit,
		Enabled:     req.Enabled,
		Description: req.Description,
	}

	if err := repository.DB.Create(&config).Error; err != nil {
		h.ErrorResponse(c, "Failed to create config", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, config, "Leaderboard config created successfully", http.StatusCreated)
}

// UpdateConfig updates a leaderboard config (admin only)
func (h *LeaderboardHandler) UpdateConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.LeaderboardConfig
	if err := repository.DB.First(&config, id).Error; err != nil {
		h.ErrorResponse(c, "Config not found", http.StatusNotFound)
		return
	}

	var req struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Metric      string `json:"metric"`
		Period      string `json:"period"`
		Limit       int    `json:"limit"`
		Enabled     bool   `json:"enabled"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ErrorResponse(c, "Validation error", http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		config.Name = req.Name
	}
	if req.DisplayName != "" {
		config.DisplayName = req.DisplayName
	}
	if req.Metric != "" {
		config.Metric = req.Metric
	}
	if req.Period != "" {
		config.Period = req.Period
	}
	if req.Limit > 0 {
		config.Limit = req.Limit
	}
	config.Enabled = req.Enabled
	config.Description = req.Description

	if err := repository.DB.Save(&config).Error; err != nil {
		h.ErrorResponse(c, "Failed to update config", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, config, "Leaderboard config updated successfully", http.StatusOK)
}

// ToggleConfigStatus 切换排行榜配置启用状态 (admin only)
func (h *LeaderboardHandler) ToggleConfigStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.LeaderboardConfig
	if err := repository.DB.First(&config, id).Error; err != nil {
		h.ErrorResponse(c, "Config not found", http.StatusNotFound)
		return
	}

	config.Enabled = !config.Enabled

	if err := repository.DB.Save(&config).Error; err != nil {
		h.ErrorResponse(c, "Failed to update config status", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, config, "Leaderboard config status updated successfully", http.StatusOK)
}

// DestroyConfig deletes a leaderboard config (admin only)
func (h *LeaderboardHandler) DestroyConfig(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var config models.LeaderboardConfig
	if err := repository.DB.First(&config, id).Error; err != nil {
		h.ErrorResponse(c, "Config not found", http.StatusNotFound)
		return
	}

	if err := repository.DB.Delete(&config).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete config", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, nil, "Leaderboard config deleted successfully", http.StatusOK)
}

func (h *LeaderboardHandler) getLeaderboardData(config *models.LeaderboardConfig) []map[string]any {
	var startDate time.Time
	now := time.Now()

	switch config.Period {
	case "daily":
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "weekly":
		startDate = now.AddDate(0, 0, -int(now.Weekday())).Truncate(24 * time.Hour)
	case "monthly":
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}

	var results []struct {
		UserID uint
		Total  float64
	}

	var query *gorm.DB

	switch config.Metric {
	case "total_tokens":
		query = repository.DB.Table("usage_logs").
			Select("user_id, SUM(total_tokens) as total").
			Where("user_id IS NOT NULL").
			Group("user_id").
			Order("total DESC").
			Limit(config.Limit)
	case "total_cost":
		query = repository.DB.Table("usage_logs").
			Select("user_id, SUM(cost) as total").
			Where("user_id IS NOT NULL").
			Group("user_id").
			Order("total DESC").
			Limit(config.Limit)
	case "request_count":
		query = repository.DB.Table("usage_logs").
			Select("user_id, COUNT(*) as total").
			Where("user_id IS NOT NULL").
			Group("user_id").
			Order("total DESC").
			Limit(config.Limit)
	default:
		query = repository.DB.Table("usage_logs").
			Select("user_id, COUNT(*) as total").
			Where("user_id IS NOT NULL").
			Group("user_id").
			Order("total DESC").
			Limit(config.Limit)
	}

	if !startDate.IsZero() {
		query = query.Where("created_at >= ?", startDate)
	}

	if err := query.Scan(&results).Error; err != nil {
		return []map[string]any{}
	}

	// 批量取用户名：收集 user_id 后一次 IN 查询，避免逐行 First 的 N+1
	userIDs := make([]uint, 0, len(results))
	for _, item := range results {
		userIDs = append(userIDs, item.UserID)
	}
	userNames := make(map[uint]string, len(userIDs))
	if len(userIDs) > 0 {
		var users []models.User
		if err := repository.DB.Select("id", "name").Where("id IN ?", userIDs).Find(&users).Error; err == nil {
			for _, u := range users {
				if u.Name != "" {
					userNames[u.ID] = u.Name
				}
			}
		}
	}

	leaderboard := make([]map[string]any, 0)
	for i, item := range results {
		userName, ok := userNames[item.UserID]
		if !ok {
			userName = "未知用户"
		}

		leaderboard = append(leaderboard, map[string]any{
			"rank":      i + 1,
			"user_id":   item.UserID,
			"user_name": userName,
			"total":     item.Total,
		})
	}

	return leaderboard
}
