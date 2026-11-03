package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// BillingHandler 处理计费管理相关接口（/api/admin/billing/*）
type BillingHandler struct {
	BaseHandler
}

func NewBillingHandler() *BillingHandler {
	return &BillingHandler{}
}

// Stats 费用统计：今日/本周/本月/总费用（数据源 usage_logs.cost）
func (h *BillingHandler) Stats(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := now.AddDate(0, 0, -int(now.Weekday())).Truncate(24 * time.Hour)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	sumCost := func(start *time.Time) float64 {
		var total float64
		query := repository.DB.Model(&models.UsageLog{}).Select("COALESCE(SUM(cost), 0)")
		if start != nil {
			query = query.Where("created_at >= ?", *start)
		}
		query.Scan(&total)
		return total
	}

	h.SuccessResponse(c, gin.H{
		"todayCost": sumCost(&todayStart),
		"weekCost":  sumCost(&weekStart),
		"monthCost": sumCost(&monthStart),
		"totalCost": sumCost(nil),
	})
}

// Rates 模型费率列表（数据源 ai_models，费率编辑与模型管理同源）
func (h *BillingHandler) Rates(c *gin.Context) {
	var modelList []models.AiModel
	if err := repository.DB.Preload("Provider").Order("id ASC").Find(&modelList).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch billing rates", http.StatusInternalServerError)
		return
	}

	rates := make([]gin.H, 0, len(modelList))
	for _, m := range modelList {
		rates = append(rates, gin.H{
			"id":           m.ID,
			"model_name":   m.Name,
			"provider":     m.Provider.Name,
			"code":         m.Code,
			"input_price":  m.InputPrice,
			"output_price": m.OutputPrice,
		})
	}

	h.SuccessResponse(c, rates)
}

// UpdateRate 更新指定模型的费率（input_price/output_price，¥/百万 tokens）
func (h *BillingHandler) UpdateRate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req struct {
		InputPrice  *float64 `json:"input_price"`
		OutputPrice *float64 `json:"output_price"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.InputPrice != nil {
		updates["input_price"] = *req.InputPrice
	}
	if req.OutputPrice != nil {
		updates["output_price"] = *req.OutputPrice
	}
	if len(updates) == 0 {
		h.ErrorResponse(c, "No fields to update", http.StatusBadRequest)
		return
	}

	var m models.AiModel
	if err := repository.DB.First(&m, id).Error; err != nil {
		h.ErrorResponse(c, "Model not found", http.StatusNotFound)
		return
	}

	if err := repository.DB.Model(&m).Updates(updates).Error; err != nil {
		h.ErrorResponse(c, "Failed to update billing rate", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{
		"id":           m.ID,
		"input_price":  m.InputPrice,
		"output_price": m.OutputPrice,
	}, "Billing rate updated successfully")
}

// Records 费用明细分页列表（数据源 usage_logs，关联用户与模型）
func (h *BillingHandler) Records(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	var total int64
	if err := repository.DB.Model(&models.UsageLog{}).Count(&total).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch billing records", http.StatusInternalServerError)
		return
	}

	var logs []models.UsageLog
	if err := repository.DB.Preload("User").Preload("Model").
		Order("created_at DESC").
		Offset((page - 1) * perPage).Limit(perPage).
		Find(&logs).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch billing records", http.StatusInternalServerError)
		return
	}

	records := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		records = append(records, gin.H{
			"created_at":    l.CreatedAt.Format("2006-01-02 15:04:05"),
			"user":          l.User.Name,
			"model":         l.Model.Name,
			"input_tokens":  l.PromptTokens,
			"output_tokens": l.CompletionTokens,
			"cost":          l.Cost,
		})
	}

	h.SuccessResponse(c, gin.H{
		"records":    records,
		"pagination": gin.H{"total": total},
	})
}
