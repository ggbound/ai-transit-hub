package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type ProviderHandler struct {
	BaseHandler
}

func NewProviderHandler() *ProviderHandler {
	return &ProviderHandler{}
}

// ListProviders lists all providers
func (h *ProviderHandler) ListProviders(c *gin.Context) {
	var providers []models.AiProvider
	if err := repository.DB.Order("sort_order ASC, created_at DESC").Find(&providers).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch providers", http.StatusInternalServerError)
		return
	}

	// 转换 status 为布尔值
	var response []map[string]interface{}
	for _, p := range providers {
		response = append(response, map[string]interface{}{
			"id":                p.ID,
			"code":              p.Code,
			"name":              p.Name,
			"adapter_class":     p.AdapterClass,
			"base_url":          p.BaseUrl,
			"auth_type":         p.AuthType,
			"auth_config":       p.AuthConfig,
			"request_format":    p.RequestFormat,
			"response_format":   p.ResponseFormat,
			"parameters_schema": p.ParametersSchema,
			"stream_support":    p.StreamSupport,
			"tool_support":      p.ToolSupport,
			"doc_url":           p.DocUrl,
			"status":            p.Status == 1,
			"sort_order":        p.SortOrder,
			"created_at":        p.CreatedAt,
			"updated_at":        p.UpdatedAt,
		})
	}

	h.SuccessResponse(c, response)
}

// CreateProvider creates a new provider
func (h *ProviderHandler) CreateProvider(c *gin.Context) {
	// 使用中间结构接收布尔值
	var req struct {
		Code             string      `json:"code"`
		Name             string      `json:"name"`
		BaseUrl          string      `json:"base_url"`
		AuthType         string      `json:"auth_type"`
		Status           bool        `json:"status"`
		SortOrder        int         `json:"sort_order"`
		AuthConfig       models.JSON `json:"auth_config"`
		RequestFormat    models.JSON `json:"request_format"`
		ResponseFormat   models.JSON `json:"response_format"`
		ParametersSchema models.JSON `json:"parameters_schema"`
		StreamSupport    bool        `json:"stream_support"`
		ToolSupport      bool        `json:"tool_support"`
		DocUrl           string      `json:"doc_url"`
		AdapterClass     string      `json:"adapter_class"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 转换布尔值为 int
	provider := models.AiProvider{
		Code:             req.Code,
		Name:             req.Name,
		BaseUrl:          req.BaseUrl,
		AuthType:         req.AuthType,
		SortOrder:        req.SortOrder,
		AuthConfig:       req.AuthConfig,
		RequestFormat:    req.RequestFormat,
		ResponseFormat:   req.ResponseFormat,
		ParametersSchema: req.ParametersSchema,
		StreamSupport:    req.StreamSupport,
		ToolSupport:      req.ToolSupport,
		DocUrl:           req.DocUrl,
		AdapterClass:     req.AdapterClass,
	}
	if req.Status {
		provider.Status = 1
	} else {
		provider.Status = 0
	}

	if err := repository.DB.Create(&provider).Error; err != nil {
		h.ErrorResponse(c, "Failed to create provider", http.StatusInternalServerError)
		return
	}

	// 返回转换为布尔值的响应
	response := map[string]interface{}{
		"id":         provider.ID,
		"code":       provider.Code,
		"name":       provider.Name,
		"base_url":   provider.BaseUrl,
		"auth_type":  provider.AuthType,
		"status":     provider.Status == 1,
		"sort_order": provider.SortOrder,
	}
	h.SuccessResponse(c, response)
}

// GetProvider gets provider by ID
func (h *ProviderHandler) GetProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var provider models.AiProvider
	if err := repository.DB.First(&provider, id).Error; err != nil {
		h.ErrorResponse(c, "Provider not found", http.StatusNotFound)
		return
	}

	// 返回转换为布尔值的响应
	response := map[string]interface{}{
		"id":                provider.ID,
		"code":              provider.Code,
		"name":              provider.Name,
		"adapter_class":     provider.AdapterClass,
		"base_url":          provider.BaseUrl,
		"auth_type":         provider.AuthType,
		"auth_config":       provider.AuthConfig,
		"request_format":    provider.RequestFormat,
		"response_format":   provider.ResponseFormat,
		"parameters_schema": provider.ParametersSchema,
		"stream_support":    provider.StreamSupport,
		"tool_support":      provider.ToolSupport,
		"doc_url":           provider.DocUrl,
		"status":            provider.Status == 1,
		"sort_order":        provider.SortOrder,
		"created_at":        provider.CreatedAt,
		"updated_at":        provider.UpdatedAt,
	}
	h.SuccessResponse(c, response)
}

// UpdateProvider updates a provider
func (h *ProviderHandler) UpdateProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var provider models.AiProvider
	if err := repository.DB.First(&provider, id).Error; err != nil {
		h.ErrorResponse(c, "Provider not found", http.StatusNotFound)
		return
	}

	// 使用中间结构接收布尔值
	var req struct {
		Code             string      `json:"code"`
		Name             string      `json:"name"`
		BaseUrl          string      `json:"base_url"`
		AuthType         string      `json:"auth_type"`
		Status           bool        `json:"status"`
		SortOrder        int         `json:"sort_order"`
		AuthConfig       models.JSON `json:"auth_config"`
		RequestFormat    models.JSON `json:"request_format"`
		ResponseFormat   models.JSON `json:"response_format"`
		ParametersSchema models.JSON `json:"parameters_schema"`
		StreamSupport    bool        `json:"stream_support"`
		ToolSupport      bool        `json:"tool_support"`
		DocUrl           string      `json:"doc_url"`
		AdapterClass     string      `json:"adapter_class"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 更新字段
	if req.Code != "" {
		provider.Code = req.Code
	}
	if req.Name != "" {
		provider.Name = req.Name
	}
	if req.BaseUrl != "" {
		provider.BaseUrl = req.BaseUrl
	}
	if req.AuthType != "" {
		provider.AuthType = req.AuthType
	}
	if req.AuthConfig != nil {
		provider.AuthConfig = req.AuthConfig
	}
	if req.RequestFormat != nil {
		provider.RequestFormat = req.RequestFormat
	}
	if req.ResponseFormat != nil {
		provider.ResponseFormat = req.ResponseFormat
	}
	if req.ParametersSchema != nil {
		provider.ParametersSchema = req.ParametersSchema
	}
	provider.StreamSupport = req.StreamSupport
	provider.ToolSupport = req.ToolSupport
	if req.DocUrl != "" {
		provider.DocUrl = req.DocUrl
	}
	if req.AdapterClass != "" {
		provider.AdapterClass = req.AdapterClass
	}
	if req.SortOrder != 0 {
		provider.SortOrder = req.SortOrder
	}

	// 转换布尔值为 int
	if req.Status {
		provider.Status = 1
	} else {
		provider.Status = 0
	}

	if err := repository.DB.Save(&provider).Error; err != nil {
		h.ErrorResponse(c, "Failed to update provider", http.StatusInternalServerError)
		return
	}

	// 返回转换为布尔值的响应
	response := map[string]interface{}{
		"id":                provider.ID,
		"code":              provider.Code,
		"name":              provider.Name,
		"adapter_class":     provider.AdapterClass,
		"base_url":          provider.BaseUrl,
		"auth_type":         provider.AuthType,
		"auth_config":       provider.AuthConfig,
		"request_format":    provider.RequestFormat,
		"response_format":   provider.ResponseFormat,
		"parameters_schema": provider.ParametersSchema,
		"stream_support":    provider.StreamSupport,
		"tool_support":      provider.ToolSupport,
		"doc_url":           provider.DocUrl,
		"status":            provider.Status == 1,
		"sort_order":        provider.SortOrder,
		"created_at":        provider.CreatedAt,
		"updated_at":        provider.UpdatedAt,
	}
	h.SuccessResponse(c, response)
}

// DeleteProvider deletes a provider
func (h *ProviderHandler) DeleteProvider(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	if err := repository.DB.Delete(&models.AiProvider{}, id).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete provider", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{"message": "Provider deleted successfully"})
}

// ToggleProviderStatus toggles provider status
func (h *ProviderHandler) ToggleProviderStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var provider models.AiProvider
	if err := repository.DB.First(&provider, id).Error; err != nil {
		h.ErrorResponse(c, "Provider not found", http.StatusNotFound)
		return
	}

	// 切换状态
	if provider.Status == 1 {
		provider.Status = 0
	} else {
		provider.Status = 1
	}

	if err := repository.DB.Save(&provider).Error; err != nil {
		h.ErrorResponse(c, "Failed to update provider status", http.StatusInternalServerError)
		return
	}

	// 转换为 map 以确保返回布尔值
	response := map[string]interface{}{
		"id":         provider.ID,
		"code":       provider.Code,
		"name":       provider.Name,
		"base_url":   provider.BaseUrl,
		"auth_type":  provider.AuthType,
		"status":     provider.Status == 1,
		"sort_order": provider.SortOrder,
	}
	h.SuccessResponse(c, response, "Provider status updated successfully")
}

// GetProviderModels gets models for a provider
func (h *ProviderHandler) GetProviderModels(c *gin.Context) {
	providerID, _ := strconv.Atoi(c.Param("id"))

	var rawModels []models.AiModel
	if err := repository.DB.Where("provider_id = ?", providerID).Preload("Provider").Find(&rawModels).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch models", http.StatusInternalServerError)
		return
	}

	modelsList := make([]map[string]interface{}, 0)
	for _, m := range rawModels {
		modelData := map[string]interface{}{
			"id":                  m.ID,
			"provider_id":         m.ProviderID,
			"code":                m.Code,
			"name":                m.Name,
			"description":         m.Description,
			"type":                m.Type,
			"max_tokens":          m.MaxTokens,
			"capabilities":        m.Capabilities,
			"input_price":         m.InputPrice,
			"output_price":        m.OutputPrice,
			"config":              m.Config,
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

// CreateProviderModel creates a model for a provider
func (h *ProviderHandler) CreateProviderModel(c *gin.Context) {
	providerID, _ := strconv.Atoi(c.Param("id"))

	// 使用中间结构接收布尔值
	var req struct {
		Type               string      `json:"type"`
		Code               string      `json:"code"`
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

	// 转换布尔值为 int
	model := models.AiModel{
		ProviderID:         uint(providerID),
		Code:               req.Code,
		Type:               req.Type,
		Name:               req.Name,
		Description:        req.Description,
		MaxTokens:          req.MaxTokens,
		Capabilities:       req.Capabilities,
		InputPrice:         req.InputPrice,
		OutputPrice:        req.OutputPrice,
		Config:             req.Config,
		CustomEndpoint:     req.CustomEndpoint,
		ChatEndpoint:       req.ChatEndpoint,
		EmbeddingsEndpoint: req.EmbeddingsEndpoint,
		ImageEndpoint:      req.ImageEndpoint,
		VideoEndpoint:      req.VideoEndpoint,
		AudioEndpoint:      req.AudioEndpoint,
		SupportsResponse:   req.SupportsResponse,
		ResponseEndpoint:   req.ResponseEndpoint,
		SupportsMessages:   req.SupportsMessages,
		MessagesEndpoint:   req.MessagesEndpoint,
		SortOrder:          req.SortOrder,
	}

	// 转换布尔值为 int
	if req.IsPublic {
		model.IsPublic = 1
	} else {
		model.IsPublic = 0
	}
	if req.Status {
		model.Status = 1
	} else {
		model.Status = 0
	}

	if err := repository.DB.Create(&model).Error; err != nil {
		h.ErrorResponse(c, "Failed to create model", http.StatusInternalServerError)
		return
	}

	// 返回转换为布尔值的响应
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

// UpdateProviderModel updates a provider model
func (h *ProviderHandler) UpdateProviderModel(c *gin.Context) {
	providerID, _ := strconv.Atoi(c.Param("id"))
	modelID, _ := strconv.Atoi(c.Param("modelId"))

	var model models.AiModel
	if err := repository.DB.Where("id = ? AND provider_id = ?", modelID, providerID).First(&model).Error; err != nil {
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
	// 转换布尔值为整数
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

	if err := repository.DB.First(&model, modelID).Error; err == nil {
		h.SuccessResponse(c, model)
	} else {
		h.ErrorResponse(c, "Failed to reload model", http.StatusInternalServerError)
	}
}

// DeleteProviderModel deletes a provider model
func (h *ProviderHandler) DeleteProviderModel(c *gin.Context) {
	providerID, _ := strconv.Atoi(c.Param("id"))
	modelID, _ := strconv.Atoi(c.Param("modelId"))

	if err := repository.DB.Where("id = ? AND provider_id = ?", modelID, providerID).Delete(&models.AiModel{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete model", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{"message": "Model deleted successfully"})
}
