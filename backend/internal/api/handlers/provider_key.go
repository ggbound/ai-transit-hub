package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// maskApiKey masks the API key for display
func maskApiKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// enhanceApiKeys enhances API key data with masked display
func enhanceApiKeys(keys []models.ProviderApiKey) []map[string]interface{} {
	result := make([]map[string]interface{}, len(keys))
	for i, key := range keys {
		item := make(map[string]interface{})
		item["id"] = key.ID
		item["ai_provider_id"] = key.AiProviderID
		item["api_key"] = key.ApiKey
		item["api_key_masked"] = maskApiKey(key.ApiKey)
		item["key_name"] = key.KeyName
		item["weight"] = key.Weight
		item["status"] = key.Status == 1
		item["usage_count"] = key.UsageCount
		item["created_at"] = key.CreatedAt
		item["updated_at"] = key.UpdatedAt
		item["provider"] = key.Provider
		result[i] = item
	}
	return result
}

// enhanceApiKey enhances a single API key
func enhanceApiKey(key *models.ProviderApiKey) map[string]interface{} {
	result := make(map[string]interface{})
	result["id"] = key.ID
	result["ai_provider_id"] = key.AiProviderID
	result["api_key"] = key.ApiKey
	result["api_key_masked"] = maskApiKey(key.ApiKey)
	result["key_name"] = key.KeyName
	result["weight"] = key.Weight
	result["status"] = key.Status == 1
	result["usage_count"] = key.UsageCount
	result["created_at"] = key.CreatedAt
	result["updated_at"] = key.UpdatedAt
	result["provider"] = key.Provider
	return result
}

type ProviderKeyHandler struct {
	BaseHandler
}

func NewProviderKeyHandler() *ProviderKeyHandler {
	return &ProviderKeyHandler{}
}

// ListProviderKeys lists all provider API keys
func (h *ProviderKeyHandler) ListProviderKeys(c *gin.Context) {
	var keys []models.ProviderApiKey
	if err := repository.DB.Preload("Provider").Find(&keys).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch provider keys", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, enhanceApiKeys(keys))
}

// CreateProviderKey creates a new provider API key
func (h *ProviderKeyHandler) CreateProviderKey(c *gin.Context) {
	var req struct {
		AiProviderID uint   `json:"ai_provider_id"`
		ApiKey       string `json:"api_key"`
		KeyName      string `json:"key_name"`
		Weight       int    `json:"weight"`
		Status       bool   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// Convert boolean status to integer
	statusInt := 0
	if req.Status {
		statusInt = 1
	}

	key := models.ProviderApiKey{
		AiProviderID: req.AiProviderID,
		ApiKey:       req.ApiKey,
		KeyName:      req.KeyName,
		Weight:       req.Weight,
		Status:       statusInt,
	}

	if err := repository.DB.Create(&key).Error; err != nil {
		h.ErrorResponse(c, "Failed to create provider key", http.StatusInternalServerError)
		return
	}

	// Reload with provider association
	repository.DB.Preload("Provider").First(&key, key.ID)
	h.SuccessResponse(c, enhanceApiKey(&key))
}

// GetProviderKey gets a provider API key by ID
func (h *ProviderKeyHandler) GetProviderKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var key models.ProviderApiKey
	if err := repository.DB.Preload("Provider").First(&key, id).Error; err != nil {
		h.ErrorResponse(c, "Provider key not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, enhanceApiKey(&key))
}

// UpdateProviderKey updates a provider API key
func (h *ProviderKeyHandler) UpdateProviderKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var key models.ProviderApiKey
	if err := repository.DB.First(&key, id).Error; err != nil {
		h.ErrorResponse(c, "Provider key not found", http.StatusNotFound)
		return
	}

	// Create a map for partial updates
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// Don't allow changing provider ID
	delete(updateData, "ai_provider_id")
	delete(updateData, "provider")

	// Convert boolean status to integer for database
	if status, ok := updateData["status"]; ok {
		if statusBool, ok := status.(bool); ok {
			if statusBool {
				updateData["status"] = 1
			} else {
				updateData["status"] = 0
			}
		}
	}

	// Update only the provided fields
	if len(updateData) > 0 {
		if err := repository.DB.Model(&key).Updates(updateData).Error; err != nil {
			h.ErrorResponse(c, "Failed to update provider key", http.StatusInternalServerError)
			return
		}
	}

	// Reload with provider association
	repository.DB.Preload("Provider").First(&key, id)
	h.SuccessResponse(c, enhanceApiKey(&key))
}

// DeleteProviderKey deletes a provider API key
func (h *ProviderKeyHandler) DeleteProviderKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	if err := repository.DB.Delete(&models.ProviderApiKey{}, id).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete provider key", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{"message": "Provider key deleted successfully"})
}

// ToggleProviderKeyStatus toggles provider API key status
func (h *ProviderKeyHandler) ToggleProviderKeyStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var key models.ProviderApiKey
	if err := repository.DB.First(&key, id).Error; err != nil {
		h.ErrorResponse(c, "Provider key not found", http.StatusNotFound)
		return
	}

	if key.Status == 1 {
		key.Status = 0
	} else {
		key.Status = 1
	}
	if err := repository.DB.Save(&key).Error; err != nil {
		h.ErrorResponse(c, "Failed to update provider key", http.StatusInternalServerError)
		return
	}

	// Reload with provider association
	repository.DB.Preload("Provider").First(&key, id)
	h.SuccessResponse(c, enhanceApiKey(&key))
}

// GetProviderKeysByProvider gets provider API keys by provider ID
func (h *ProviderKeyHandler) GetProviderKeysByProvider(c *gin.Context) {
	providerID, _ := strconv.Atoi(c.Param("providerId"))

	var keys []models.ProviderApiKey
	if err := repository.DB.Where("ai_provider_id = ?", providerID).Preload("Provider").Find(&keys).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch provider keys", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, enhanceApiKeys(keys))
}
