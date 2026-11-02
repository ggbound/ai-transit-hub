package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type UserApiKeyController struct {
	BaseHandler
}

func NewUserApiKeyController() *UserApiKeyController {
	return &UserApiKeyController{}
}

// Index gets user's API keys
func (h *UserApiKeyController) Index(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	var apiKeys []models.UserApiKey
	var total int64

	query := repository.DB.Where("user_id = ?", user.ID)
	query.Model(&models.UserApiKey{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&apiKeys)

	result := gin.H{
		"data": apiKeys,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// Show gets a single API key
func (h *UserApiKeyController) Show(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	var apiKey models.UserApiKey
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiKey).Error; err != nil {
		h.ErrorResponse(c, "API key not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, apiKey)
}

// Store creates a new API key
func (h *UserApiKeyController) Store(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	var req struct {
		KeyName   string  `json:"key_name" binding:"required,max=191"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	logger.Info("Creating API key - KeyName: %s, ExpiresAt: %v", req.KeyName, req.ExpiresAt)

	keyString := generateApiKey()
	hash := sha256.Sum256([]byte(keyString))
	keyHash := hex.EncodeToString(hash[:])

	apiKey := models.UserApiKey{
		UserID:  user.ID,
		KeyName: req.KeyName,
		ApiKey:  keyString,
		KeyHash: keyHash,
		Status:  true,
	}

	// 处理过期时间 - 支持多种格式
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		logger.Info("Parsing expires_at: %s", *req.ExpiresAt)

		var expiresTime time.Time
		var err error

		// 尝试多种格式（使用本地时区 UTC+8）
		loc, _ := time.LoadLocation("Asia/Shanghai")

		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05.000Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			expiresTime, err = time.ParseInLocation(format, *req.ExpiresAt, loc)
			if err == nil {
				logger.Info("Parsed expires_at with format %s (Asia/Shanghai): %v", format, expiresTime)
				break
			}
		}

		if err == nil {
			apiKey.ExpiresAt = &expiresTime
			logger.Info("Set ExpiresAt successfully: %v", apiKey.ExpiresAt)
		} else {
			logger.Error("Failed to parse expires_at: %s, error: %v", *req.ExpiresAt, err)
		}
	}

	if err := repository.DB.Create(&apiKey).Error; err != nil {
		logger.Error("Failed to create API key in database: %v", err)
		h.ErrorResponse(c, "Failed to create API key", http.StatusInternalServerError)
		return
	}

	logger.Info("API key created - ID: %d, ExpiresAt: %v", apiKey.ID, apiKey.ExpiresAt)

	result := gin.H{
		"id":         apiKey.ID,
		"key_name":   apiKey.KeyName,
		"api_key":    apiKey.ApiKey,
		"created_at": apiKey.CreatedAt,
		"expires_at": apiKey.ExpiresAt,
	}

	h.SuccessResponseWithCode(c, result, "API key created successfully", http.StatusCreated)
}

// Update updates an API key
func (h *UserApiKeyController) Update(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	var apiKey models.UserApiKey
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiKey).Error; err != nil {
		h.ErrorResponse(c, "API key not found", http.StatusNotFound)
		return
	}

	var req struct {
		KeyName   string  `json:"key_name"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	if req.KeyName != "" {
		apiKey.KeyName = req.KeyName
	}

	// 处理过期时间更新
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			// 清空过期时间
			apiKey.ExpiresAt = nil
			logger.Info("Clearing expires_at for API key %d", apiKey.ID)
		} else {
			// 更新过期时间
			logger.Info("Parsing expires_at for update: %s", *req.ExpiresAt)

			var expiresTime time.Time
			var err error

			// 尝试多种格式（使用本地时区 UTC+8）
			loc, _ := time.LoadLocation("Asia/Shanghai")

			formats := []string{
				time.RFC3339,
				time.RFC3339Nano,
				"2006-01-02T15:04:05",
				"2006-01-02T15:04:05.000Z",
				"2006-01-02 15:04:05",
				"2006-01-02",
			}

			for _, format := range formats {
				expiresTime, err = time.ParseInLocation(format, *req.ExpiresAt, loc)
				if err == nil {
					logger.Info("Parsed expires_at with format %s (Asia/Shanghai): %v", format, expiresTime)
					break
				}
			}

			if err == nil {
				apiKey.ExpiresAt = &expiresTime
				logger.Info("Updated ExpiresAt for API key %d: %v", apiKey.ID, apiKey.ExpiresAt)
			} else {
				logger.Error("Failed to parse expires_at for update: %s, error: %v", *req.ExpiresAt, err)
			}
		}
	}

	if err := repository.DB.Save(&apiKey).Error; err != nil {
		h.ErrorResponse(c, "Failed to update API key", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, apiKey, "API key updated successfully")
}

// Destroy deletes an API key
func (h *UserApiKeyController) Destroy(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.UserApiKey{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete API key", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, nil, "API key deleted successfully")
}

// ToggleStatus toggles API key status
func (h *UserApiKeyController) ToggleStatus(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Invalid ID", http.StatusBadRequest)
		return
	}

	var apiKey models.UserApiKey
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&apiKey).Error; err != nil {
		h.ErrorResponse(c, "API key not found", http.StatusNotFound)
		return
	}

	// Toggle status
	apiKey.Status = !apiKey.Status

	if err := repository.DB.Save(&apiKey).Error; err != nil {
		h.ErrorResponse(c, "Failed to update API key status", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, apiKey, "API key status updated successfully")
}

func generateApiKey() string {
	// 生成32字节随机数
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		// 如果随机生成失败，使用当前时间戳作为备选
		return fmt.Sprintf("sk-%d", time.Now().UnixNano())
	}

	// 使用URL安全的base64编码，替换掉特殊字符
	encoded := base64.URLEncoding.EncodeToString(random)
	// 去掉填充字符 =，并转换为小写
	encoded = strings.ToLower(strings.TrimRight(encoded, "="))
	// 取前43位，添加sk-前缀
	if len(encoded) > 43 {
		encoded = encoded[:43]
	}
	return fmt.Sprintf("sk_%s", encoded)
}
