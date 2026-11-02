package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type ToolController struct {
	BaseHandler
}

func NewToolController() *ToolController {
	return &ToolController{}
}

// Index gets tools list
func (h *ToolController) Index(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	enabled := c.Query("enabled")
	toolType := c.Query("tool_type")

	var tools []models.Tool
	var total int64

	query := repository.DB.Model(&models.Tool{})

	if enabled != "" {
		query = query.Where("enabled = ?", enabled == "true")
	}
	if toolType != "" {
		query = query.Where("tool_type = ?", toolType)
	}

	query.Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&tools)

	result := gin.H{
		"data": tools,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// Show gets a single tool
func (h *ToolController) Show(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var tool models.Tool
	if err := repository.DB.Preload("Versions").First(&tool, id).Error; err != nil {
		h.ErrorResponse(c, "Tool not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, tool)
}

// Store creates a new tool
func (h *ToolController) Store(c *gin.Context) {
	var tool models.Tool
	if err := c.ShouldBindJSON(&tool); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 必填校验：name/display_name/tool_type 为数据库非空列，空 body 不可创建
	if strings.TrimSpace(tool.Name) == "" || strings.TrimSpace(tool.DisplayName) == "" || strings.TrimSpace(tool.ToolType) == "" {
		h.ValidationErrorResponse(c, "name, display_name 和 tool_type 不能为空")
		return
	}

	if err := repository.DB.Create(&tool).Error; err != nil {
		h.ErrorResponse(c, "Failed to create tool", http.StatusInternalServerError)
		return
	}

	h.SuccessResponseWithCode(c, tool, "Tool created successfully", http.StatusCreated)
}

// Update updates a tool
func (h *ToolController) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var tool models.Tool
	if err := repository.DB.First(&tool, id).Error; err != nil {
		h.ErrorResponse(c, "Tool not found", http.StatusNotFound)
		return
	}

	if err := c.ShouldBindJSON(&tool); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	if err := repository.DB.Save(&tool).Error; err != nil {
		h.ErrorResponse(c, "Failed to update tool", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, tool, "Tool updated successfully")
}

// ToggleStatus 切换工具启用状态（Tools 页启用/禁用开关）
func (h *ToolController) ToggleStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	var tool models.Tool
	if err := repository.DB.First(&tool, id).Error; err != nil {
		h.ErrorResponse(c, "Tool not found", http.StatusNotFound)
		return
	}

	tool.Enabled = !tool.Enabled

	if err := repository.DB.Save(&tool).Error; err != nil {
		h.ErrorResponse(c, "Failed to update tool status", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, tool, "Tool status updated successfully")
}

// Destroy deletes a tool
func (h *ToolController) Destroy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	if err := repository.DB.Delete(&models.Tool{}, id).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete tool", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, nil, "Tool deleted successfully")
}
