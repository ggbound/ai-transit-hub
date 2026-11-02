package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/auth"
	"github.com/your-org/ai-transit-hub/internal/cache"
	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// lastUsedThrottle 记录每个 API key 最近一次 last_used_at 落库时间，60s 内不重复写
var lastUsedThrottle sync.Map

// updateLastUsedAtAsync 异步更新 last_used_at：60s 节流避免高频请求每次都写库，recover 防止 panic 扩散
func updateLastUsedAtAsync(apiKeyID uint) {
	if v, ok := lastUsedThrottle.Load(apiKeyID); ok {
		if time.Since(v.(time.Time)) < 60*time.Second {
			return
		}
	}
	lastUsedThrottle.Store(apiKeyID, time.Now())
	go func(id uint) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("update last_used_at panicked for api_key_id %d: %v", id, r)
			}
		}()
		repository.DB.Model(&models.UserApiKey{}).Where("id = ?", id).Update("last_used_at", time.Now())
	}(apiKeyID)
}

// AuthMiddleware handles JWT authentication
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(parts[1], cfg)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Get user from database
		var user models.User
		if err := repository.DB.Preload("Roles").First(&user, claims.UserID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", &user)
		c.Next()
	}
}

// ApiKeyAuthMiddleware handles API key authentication for external APIs
func ApiKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var apiKey string

		// 1. 优先检查 Authorization: Bearer 格式
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				apiKey = parts[1]
			}
		}

		// 2. 如果没有，检查 X-Api-Key 头
		if apiKey == "" {
			apiKey = c.GetHeader("X-Api-Key")
		}

		if apiKey == "" {
			// v1 契约统一 OpenAI 错误信封：{"error": {message, type, code}}
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"message": "authorization header or x-api-key header required",
				"type":    "invalid_request_error",
				"code":    "missing_api_key",
			}})
			c.Abort()
			return
		}

		// Validate API key（带 30s TTL 缓存，减少每请求 DB 往返；
		// key 被禁用后最多在 TTL 窗口内仍可用，属可接受的安全权衡）
		var userApiKey models.UserApiKey
		if v, ok := cache.Auth.Get("auth:" + apiKey); ok {
			userApiKey = v.(models.UserApiKey)
		} else {
			if err := repository.DB.Where("api_key = ? AND status = ?", apiKey, true).Preload("User").First(&userApiKey).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
					"message": "invalid api key",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				}})
				c.Abort()
				return
			}
			cache.Auth.Set("auth:"+apiKey, userApiKey)
		}

		// Check if API key has expired
		if userApiKey.ExpiresAt != nil && time.Now().After(*userApiKey.ExpiresAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{
				"message": "api key has expired",
				"type":    "invalid_request_error",
				"code":    "api_key_expired",
			}})
			c.Abort()
			return
		}

		// Set user and api key in context (使用指针类型)
		c.Set("user", &userApiKey.User)
		c.Set("api_key", &userApiKey)
		c.Set("api_key_id", userApiKey.ID)

		// Update last used time asynchronously
		updateLastUsedAtAsync(userApiKey.ID)

		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found in context"})
			c.Abort()
			return
		}

		u, ok := user.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user type"})
			c.Abort()
			return
		}

		if !u.HasRole(requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUser gets user from context
func GetUser(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	u, ok := user.(*models.User)
	return u, ok
}
