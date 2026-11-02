package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type DashboardHandler struct {
	BaseHandler
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{}
}

// Stats 获取管理后台仪表盘统计数据
func (h *DashboardHandler) Stats(c *gin.Context) {
	// 基础统计数据
	var totalUsers, totalModels, totalProviders, totalSessions int64
	repository.DB.Model(&models.User{}).Count(&totalUsers)
	repository.DB.Model(&models.AiModel{}).Count(&totalModels)
	repository.DB.Model(&models.AiProvider{}).Count(&totalProviders)
	repository.DB.Model(&models.ChatSession{}).Count(&totalSessions)

	stats := map[string]interface{}{
		"totalUsers":     int(totalUsers),
		"totalModels":    int(totalModels),
		"totalProviders": int(totalProviders),
		"totalSessions":  int(totalSessions),
	}

	// Token 和费用统计
	type UsageStats struct {
		TotalTokens int64
		TotalCost   float64
	}
	var usageStats UsageStats
	repository.DB.Model(&models.UsageLog{}).Select(
		"COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost), 0) as total_cost",
	).Scan(&usageStats)

	stats["totalTokens"] = int(usageStats.TotalTokens)
	stats["totalCost"] = usageStats.TotalCost

	// 今日统计（按本地时区零点对齐；原 Truncate 按 UTC 对齐导致时区偏移 8 小时）
	y, m, d := time.Now().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	type TodayStats struct {
		CallCount int64
		Tokens    int64
		Cost      float64
	}
	var todayStats TodayStats
	repository.DB.Model(&models.UsageLog{}).Where("request_time >= ?", today).Select(
		"COALESCE(COUNT(*), 0) as call_count, COALESCE(SUM(total_tokens), 0) as tokens, COALESCE(SUM(cost), 0) as cost",
	).Scan(&todayStats)

	stats["todayCalls"] = int(todayStats.CallCount)
	stats["todayTokens"] = int(todayStats.Tokens)
	stats["todayCost"] = todayStats.Cost

	// 本月统计
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())
	type MonthStats struct {
		CallCount int64
		Tokens    int64
		Cost      float64
	}
	var monthStats MonthStats
	repository.DB.Model(&models.UsageLog{}).Where("request_time >= ?", monthStart).Select(
		"COALESCE(COUNT(*), 0) as call_count, COALESCE(SUM(total_tokens), 0) as tokens, COALESCE(SUM(cost), 0) as cost",
	).Scan(&monthStats)

	stats["monthCalls"] = int(monthStats.CallCount)
	stats["monthTokens"] = int(monthStats.Tokens)
	stats["monthCost"] = monthStats.Cost

	// 最近7天趋势数据
	trendData := []map[string]interface{}{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		dy, dm, dd := date.Date()
		dateStart := time.Date(dy, dm, dd, 0, 0, 0, 0, time.Local)
		dateEnd := dateStart.AddDate(0, 0, 1)

		type DayStats struct {
			Calls  int64
			Tokens int64
			Cost   float64
		}
		var dayStats DayStats
		repository.DB.Model(&models.UsageLog{}).Where("request_time >= ? AND request_time < ?", dateStart, dateEnd).Select(
			"COALESCE(COUNT(*), 0) as calls, COALESCE(SUM(total_tokens), 0) as tokens, COALESCE(SUM(cost), 0) as cost",
		).Scan(&dayStats)

		trendData = append(trendData, map[string]interface{}{
			"date":   date.Format("01-02"),
			"calls":  int(dayStats.Calls),
			"tokens": int(dayStats.Tokens),
			"cost":   dayStats.Cost,
		})
	}

	// 热门模型排行（只统计最近 30 天，避免全表聚合）
	rankSince := today.AddDate(0, 0, -30)
	type TopModelRow struct {
		ModelID     uint
		CallCount   int64
		TotalTokens int64
	}
	var topModelRows []TopModelRow
	repository.DB.Model(&models.UsageLog{}).Select(
		"model_id, COUNT(*) as call_count, COALESCE(SUM(total_tokens), 0) as total_tokens",
	).Where("request_time >= ? AND model_id IS NOT NULL", rankSince).Group("model_id").Order("call_count DESC").Limit(5).Scan(&topModelRows)

	topModels := []map[string]interface{}{}
	if len(topModelRows) > 0 {
		// 批量取模型（含 Provider），避免逐行 First+Preload 的 N+1
		modelIDs := make([]uint, 0, len(topModelRows))
		for _, row := range topModelRows {
			modelIDs = append(modelIDs, row.ModelID)
		}
		var aiModels []models.AiModel
		repository.DB.Preload("Provider").Where("id IN ?", modelIDs).Find(&aiModels)
		modelMap := make(map[uint]models.AiModel, len(aiModels))
		for _, m := range aiModels {
			modelMap[m.ID] = m
		}
		for _, row := range topModelRows {
			model := modelMap[row.ModelID]
			fullCode := "-"
			if model.ID > 0 && model.Provider.ID > 0 {
				fullCode = model.Provider.Code + "/" + model.Code
			}
			topModels = append(topModels, map[string]interface{}{
				"id":         row.ModelID,
				"name":       model.Name,
				"code":       model.Code,
				"full_code":  fullCode,
				"call_count": int(row.CallCount),
				"tokens":     int(row.TotalTokens),
			})
		}
	}

	// 活跃用户排行（同上，只统计最近 30 天）
	type TopUserRow struct {
		UserID      uint
		CallCount   int64
		TotalTokens int64
		TotalCost   float64
	}
	var topUserRows []TopUserRow
	repository.DB.Model(&models.UsageLog{}).Select(
		"user_id, COUNT(*) as call_count, COALESCE(SUM(total_tokens), 0) as total_tokens, COALESCE(SUM(cost), 0) as total_cost",
	).Where("request_time >= ? AND user_id IS NOT NULL", rankSince).Group("user_id").Order("call_count DESC").Limit(5).Scan(&topUserRows)

	topUsers := []map[string]interface{}{}
	if len(topUserRows) > 0 {
		// 批量取用户，避免逐行 First 的 N+1
		userIDs := make([]uint, 0, len(topUserRows))
		for _, row := range topUserRows {
			userIDs = append(userIDs, row.UserID)
		}
		var users []models.User
		repository.DB.Select("id", "name", "email").Where("id IN ?", userIDs).Find(&users)
		userMap := make(map[uint]models.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for _, row := range topUserRows {
			user := userMap[row.UserID]
			topUsers = append(topUsers, map[string]interface{}{
				"id":         row.UserID,
				"name":       user.Name,
				"email":      user.Email,
				"call_count": int(row.CallCount),
				"tokens":     int(row.TotalTokens),
				"cost":       row.TotalCost,
			})
		}
	}

	// 系统状态（providers/models 总数复用开头已统计的 totalProviders/totalModels，避免重复 Count）
	var providersEnabled, modelsEnabled int64
	repository.DB.Model(&models.AiProvider{}).Where("status = ?", true).Count(&providersEnabled)
	repository.DB.Model(&models.AiModel{}).Where("status = ?", true).Count(&modelsEnabled)

	systemStatus := map[string]interface{}{
		"providers": map[string]interface{}{
			"total":   int(totalProviders),
			"enabled": int(providersEnabled),
		},
		"models": map[string]interface{}{
			"total":   int(totalModels),
			"enabled": int(modelsEnabled),
		},
	}

	h.SuccessResponse(c, gin.H{
		"stats":        stats,
		"trend":        trendData,
		"topModels":    topModels,
		"topUsers":     topUsers,
		"systemStatus": systemStatus,
	})
}
