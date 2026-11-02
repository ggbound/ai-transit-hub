package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/cache"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type ModelController struct {
	BaseHandler
}

func NewModelController() *ModelController {
	return &ModelController{}
}

func getBoolOrDefault(value bool, defaultValue bool) bool {
	return value
}

// getCachedPublicModels 返回启用且公开的模型列表（60s TTL 进程内缓存），
// 供 openai/anthropic 的 ListModels 共用，避免每次请求全量查库。
// 查询失败时返回 false，由调用方决定错误响应。
func getCachedPublicModels() ([]models.AiModel, bool) {
	const cacheKey = "models:public:all"
	if v, ok := cache.Config.Get(cacheKey); ok {
		if list, ok := v.([]models.AiModel); ok {
			return list, true
		}
	}
	var list []models.AiModel
	if err := repository.DB.Where("status = ? AND is_public = ?", 1, 1).Preload("Provider").Find(&list).Error; err != nil {
		logger.Error("Failed to fetch public models: %v", err)
		return nil, false
	}
	cache.Config.Set(cacheKey, list)
	return list, true
}

// Index 获取可用模型列表（用户端）
func (h *ModelController) Index(c *gin.Context) {
	query := repository.DB.Model(&models.AiModel{}).
		Where("status = ?", 1).
		Where("is_public = ?", 1)

	if providerID := c.Query("provider_id"); providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	if providerCode := c.Query("provider_code"); providerCode != "" {
		var provider models.AiProvider
		if repository.DB.Where("code = ?", providerCode).First(&provider).Error == nil {
			query = query.Where("provider_id = ?", provider.ID)
		}
	}

	var rawModels []models.AiModel
	if err := query.Order("sort_order").Order("id").Find(&rawModels).Error; err != nil {
		logger.Error("Failed to fetch models: %v", err)
		h.ErrorResponse(c, "Failed to fetch models", http.StatusInternalServerError)
		return
	}

	logger.Debug("Found %d models", len(rawModels))

	// 预加载所有 provider 数据
	providerMap := make(map[uint]models.AiProvider)
	var providerIDs []uint
	for _, m := range rawModels {
		if m.ProviderID > 0 {
			providerIDs = append(providerIDs, m.ProviderID)
		}
	}
	if len(providerIDs) > 0 {
		var providers []models.AiProvider
		repository.DB.Where("id IN ?", providerIDs).Find(&providers)
		for _, p := range providers {
			providerMap[p.ID] = p
		}
	}

	modelsList := make([]map[string]interface{}, 0)
	for _, m := range rawModels {
		// 从 map 中获取 provider
		provider, exists := providerMap[m.ProviderID]
		if !exists {
			logger.Warn("Skipping model %d: provider (ID: %d) not found", m.ID, m.ProviderID)
			continue
		}
		m.Provider = provider

		modelData := map[string]interface{}{
			"id":                  m.ID,
			"code":                m.Provider.Code + "/" + m.Code,
			"name":                m.Name,
			"description":         m.Description,
			"type":                m.Type,
			"max_tokens":          m.MaxTokens,
			"capabilities":        json.RawMessage(m.Capabilities),
			"supports_response":   getBoolOrDefault(m.SupportsResponse, false),
			"supports_messages":   getBoolOrDefault(m.SupportsMessages, false),
			"supports_openai":     true,
			"supports_anthropic":  false,
			"supports_embeddings": false,
			"supports_images":     false,
			"supports_videos":     false,
			"supports_audio":      false,
			"provider": map[string]interface{}{
				"id":        m.Provider.ID,
				"code":      m.Provider.Code,
				"name":      m.Provider.Name,
				"base_url":  m.Provider.BaseUrl,
				"auth_type": m.Provider.AuthType,
			},
			"full_model_id": m.Provider.Code + "/" + m.Code,
		}

		modelsList = append(modelsList, modelData)
	}

	grouped := make([]map[string]interface{}, 0)
	groupProviderMap := make(map[string][]map[string]interface{})
	for _, m := range modelsList {
		providerName := m["provider"].(map[string]interface{})["name"].(string)
		groupProviderMap[providerName] = append(groupProviderMap[providerName], m)
	}

	for name, models := range groupProviderMap {
		if len(models) > 0 {
			grouped = append(grouped, map[string]interface{}{
				"provider_name": name,
				"provider_code": models[0]["provider"].(map[string]interface{})["code"],
				"models":        models,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data": gin.H{
			"models":  modelsList,
			"grouped": grouped,
		},
	})
}

// AdminIndex 获取所有模型列表（管理端）
func (h *ModelController) AdminIndex(c *gin.Context) {
	query := repository.DB.Model(&models.AiModel{})

	if providerID := c.Query("provider_id"); providerID != "" {
		query = query.Where("provider_id = ?", providerID)
	}

	if providerCode := c.Query("provider_code"); providerCode != "" {
		var provider models.AiProvider
		if repository.DB.Select("id").Where("code = ?", providerCode).First(&provider).Error == nil {
			query = query.Where("provider_id = ?", provider.ID)
		}
	}

	// 可选分页：仅当请求带 per_page 参数时启用（不带参数保持全量，兼容现有前端）
	if perPageStr := c.Query("per_page"); perPageStr != "" {
		perPage, err := strconv.Atoi(perPageStr)
		if err != nil || perPage < 1 {
			perPage = 50
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
		query = query.Offset((page - 1) * perPage).Limit(perPage)
	}

	var rawModels []models.AiModel
	if err := query.Order("sort_order").Order("id").Find(&rawModels).Error; err != nil {
		logger.Error("Failed to fetch models: %v", err)
		h.ErrorResponse(c, "Failed to fetch models", http.StatusInternalServerError)
		return
	}

	logger.Debug("Found %d models", len(rawModels))

	// 预加载所有 provider 数据
	providerMap := make(map[uint]models.AiProvider)
	var providerIDs []uint
	for _, m := range rawModels {
		if m.ProviderID > 0 {
			providerIDs = append(providerIDs, m.ProviderID)
		}
	}
	if len(providerIDs) > 0 {
		var providers []models.AiProvider
		repository.DB.Where("id IN ?", providerIDs).Find(&providers)
		for _, p := range providers {
			providerMap[p.ID] = p
		}
	}

	modelsList := make([]map[string]interface{}, 0)
	for _, m := range rawModels {
		// 从 map 中获取 provider
		provider, exists := providerMap[m.ProviderID]
		if !exists {
			logger.Warn("Skipping model %d: provider (ID: %d) not found", m.ID, m.ProviderID)
			continue
		}
		m.Provider = provider

		modelData := map[string]interface{}{
			"id":                  m.ID,
			"provider_id":         m.ProviderID,
			"code":                m.Code,
			"name":                m.Name,
			"description":         m.Description,
			"type":                m.Type,
			"max_tokens":          m.MaxTokens,
			"capabilities":        json.RawMessage(m.Capabilities),
			"input_price":         m.InputPrice,
			"output_price":        m.OutputPrice,
			"config":              json.RawMessage(m.Config),
			"custom_endpoint":     m.CustomEndpoint,
			"chat_endpoint":       m.ChatEndpoint,
			"embeddings_endpoint": m.EmbeddingsEndpoint,
			"image_endpoint":      m.ImageEndpoint,
			"video_endpoint":      m.VideoEndpoint,
			"audio_endpoint":      m.AudioEndpoint,
			"supports_response":   m.SupportsResponse,
			"response_endpoint":   m.ResponseEndpoint,
			"supports_messages":   m.SupportsMessages,
			"messages_endpoint":   m.MessagesEndpoint,
			"supports_openai":     true,
			"supports_anthropic":  false,
			"supports_embeddings": false,
			"supports_images":     false,
			"supports_videos":     false,
			"supports_audio":      false,
			"is_public":           m.IsPublic == 1,
			"status":              m.Status == 1,
			"sort_order":          m.SortOrder,
			"created_at":          m.CreatedAt,
			"updated_at":          m.UpdatedAt,
		}
		if m.Provider.ID > 0 {
			modelData["provider"] = map[string]interface{}{
				"id":         m.Provider.ID,
				"code":       m.Provider.Code,
				"name":       m.Provider.Name,
				"base_url":   m.Provider.BaseUrl,
				"auth_type":  m.Provider.AuthType,
				"status":     m.Provider.Status == 1,
				"sort_order": m.Provider.SortOrder,
			}
		}
		modelsList = append(modelsList, modelData)
	}

	h.SuccessResponse(c, modelsList)
}

// Show 获取单个模型详情
func (h *ModelController) Show(c *gin.Context) {
	id := c.Param("id")
	var model models.AiModel
	if err := repository.DB.Preload("Provider").First(&model, id).Error; err != nil {
		h.ErrorResponse(c, "Model not found", http.StatusNotFound)
		return
	}

	if model.Provider.ID == 0 {
		h.ErrorResponse(c, "该模型没有关联的提供商", http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data": map[string]interface{}{
			"id":                  model.ID,
			"code":                model.Code,
			"name":                model.Name,
			"description":         model.Description,
			"type":                model.Type,
			"max_tokens":          model.MaxTokens,
			"capabilities":        model.Capabilities,
			"supports_response":   getBoolOrDefault(model.SupportsResponse, false),
			"supports_messages":   getBoolOrDefault(model.SupportsMessages, false),
			"supports_openai":     true,
			"supports_anthropic":  false,
			"supports_embeddings": false,
			"supports_images":     false,
			"supports_videos":     false,
			"supports_audio":      false,
			"provider": map[string]interface{}{
				"id":        model.Provider.ID,
				"code":      model.Provider.Code,
				"name":      model.Provider.Name,
				"base_url":  model.Provider.BaseUrl,
				"auth_type": model.Provider.AuthType,
			},
			"full_model_id": model.Provider.Code + "/" + model.Code,
		},
	})
}

// Store 预留管理端接口
func (h *ModelController) Store(c *gin.Context) {
	h.ErrorResponse(c, "请使用管理后台接口创建模型", http.StatusBadRequest)
}

// Update 更新模型
func (h *ModelController) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var model models.AiModel
	if err := repository.DB.First(&model, id).Error; err != nil {
		h.ErrorResponse(c, "Model not found", http.StatusNotFound)
		return
	}

	var req struct {
		Type               string      `json:"type"`
		Name               string      `json:"name"`
		Description        string      `json:"description"`
		MaxTokens          int         `json:"max_tokens"`
		Capabilities       models.JSON `json:"capabilities"`
		IsPublic           bool        `json:"is_public"`
		Status             bool        `json:"status"`
		Config             models.JSON `json:"config"`
		CustomEndpoint     string      `json:"custom_endpoint"`
		ChatEndpoint       string      `json:"chat_endpoint"`
		EmbeddingsEndpoint string      `json:"embeddings_endpoint"`
		ImageEndpoint      string      `json:"image_endpoint"`
		VideoEndpoint      string      `json:"video_endpoint"`
		AudioEndpoint      string      `json:"audio_endpoint"`
		SupportsResponse   bool        `json:"supports_response"`
		ResponseEndpoint   string      `json:"response_endpoint"`
		SupportsMessages   bool        `json:"supports_messages"`
		MessagesEndpoint   string      `json:"messages_endpoint"`
		InputPrice         float64     `json:"input_price"`
		OutputPrice        float64     `json:"output_price"`
		SortOrder          int         `json:"sort_order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	updateData := map[string]interface{}{}
	if req.Type != "" {
		updateData["type"] = req.Type
	}
	if req.Name != "" {
		updateData["name"] = req.Name
	}
	if req.Description != "" {
		updateData["description"] = req.Description
	}
	if req.MaxTokens > 0 {
		updateData["max_tokens"] = req.MaxTokens
	}
	if len(req.Capabilities) > 0 {
		updateData["capabilities"] = req.Capabilities
	}
	// 转换布尔值为int
	if req.IsPublic {
		updateData["is_public"] = 1
	} else {
		updateData["is_public"] = 0
	}
	if req.Status {
		updateData["status"] = 1
	} else {
		updateData["status"] = 0
	}
	if len(req.Config) > 0 {
		updateData["config"] = req.Config
	}
	if req.CustomEndpoint != "" {
		updateData["custom_endpoint"] = req.CustomEndpoint
	}
	if req.ChatEndpoint != "" {
		updateData["chat_endpoint"] = req.ChatEndpoint
	}
	if req.EmbeddingsEndpoint != "" {
		updateData["embeddings_endpoint"] = req.EmbeddingsEndpoint
	}
	if req.ImageEndpoint != "" {
		updateData["image_endpoint"] = req.ImageEndpoint
	}
	if req.VideoEndpoint != "" {
		updateData["video_endpoint"] = req.VideoEndpoint
	}
	if req.AudioEndpoint != "" {
		updateData["audio_endpoint"] = req.AudioEndpoint
	}
	updateData["supports_response"] = req.SupportsResponse
	if req.ResponseEndpoint != "" {
		updateData["response_endpoint"] = req.ResponseEndpoint
	}
	updateData["supports_messages"] = req.SupportsMessages
	if req.MessagesEndpoint != "" {
		updateData["messages_endpoint"] = req.MessagesEndpoint
	}
	if req.InputPrice >= 0 {
		updateData["input_price"] = req.InputPrice
	}
	if req.OutputPrice >= 0 {
		updateData["output_price"] = req.OutputPrice
	}
	updateData["sort_order"] = req.SortOrder

	if err := repository.DB.Model(&model).Updates(updateData).Error; err != nil {
		logger.Error("Failed to update model: %v", err)
		h.ErrorResponse(c, "Failed to update model: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := repository.DB.Preload("Provider").First(&model, id).Error; err == nil {
		// 转换为 map 以确保返回布尔值
		response := map[string]interface{}{
			"id":                  model.ID,
			"provider_id":         model.ProviderID,
			"code":                model.Code,
			"name":                model.Name,
			"description":         model.Description,
			"type":                model.Type,
			"max_tokens":          model.MaxTokens,
			"capabilities":        model.Capabilities,
			"input_price":         model.InputPrice,
			"output_price":        model.OutputPrice,
			"config":              model.Config,
			"custom_endpoint":     model.CustomEndpoint,
			"chat_endpoint":       model.ChatEndpoint,
			"embeddings_endpoint": model.EmbeddingsEndpoint,
			"image_endpoint":      model.ImageEndpoint,
			"video_endpoint":      model.VideoEndpoint,
			"audio_endpoint":      model.AudioEndpoint,
			"supports_response":   model.SupportsResponse,
			"response_endpoint":   model.ResponseEndpoint,
			"supports_messages":   model.SupportsMessages,
			"messages_endpoint":   model.MessagesEndpoint,
			"supports_openai":     true,
			"supports_anthropic":  false,
			"supports_embeddings": false,
			"supports_images":     false,
			"supports_videos":     false,
			"supports_audio":      false,
			"is_public":           model.IsPublic == 1,
			"status":              model.Status == 1,
			"sort_order":          model.SortOrder,
			"created_at":          model.CreatedAt,
			"updated_at":          model.UpdatedAt,
		}
		if model.Provider.ID > 0 {
			response["provider"] = map[string]interface{}{
				"id":         model.Provider.ID,
				"code":       model.Provider.Code,
				"name":       model.Provider.Name,
				"base_url":   model.Provider.BaseUrl,
				"auth_type":  model.Provider.AuthType,
				"status":     model.Provider.Status == 1,
				"sort_order": model.Provider.SortOrder,
			}
		}
		h.SuccessResponse(c, response)
	} else {
		h.ErrorResponse(c, "Failed to reload model", http.StatusInternalServerError)
	}
}

// Destroy 预留管理端接口
func (h *ModelController) Destroy(c *gin.Context) {
	h.ErrorResponse(c, "请使用管理后台接口删除模型", http.StatusBadRequest)
}

// ToggleStatus 切换模型状态
func (h *ModelController) ToggleStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var model models.AiModel
	if err := repository.DB.First(&model, id).Error; err != nil {
		h.ErrorResponse(c, "Model not found", http.StatusNotFound)
		return
	}

	if model.Status == 1 {
		model.Status = 0
	} else {
		model.Status = 1
	}

	if err := repository.DB.Save(&model).Error; err != nil {
		logger.Error("Failed to update model status: %v", err)
		h.ErrorResponse(c, "Failed to update model status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 转换为 map 以确保返回布尔值
	response := map[string]interface{}{
		"id":          model.ID,
		"provider_id": model.ProviderID,
		"code":        model.Code,
		"name":        model.Name,
		"status":      model.Status == 1,
		"is_public":   model.IsPublic == 1,
		"sort_order":  model.SortOrder,
	}
	h.SuccessResponse(c, response)
}
