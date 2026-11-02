package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/handlers"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/config"
)

// SetupRoutes sets all API routes, exactly matching PHP backend
func SetupRoutes(r *gin.Engine, cfg *config.Config) {
	// CORS middleware
	r.Use(middleware.CORS())

	// Initialize all handlers
	authHandler := handlers.NewAuthHandler(cfg)
	chatController := handlers.NewChatController()
	modelController := handlers.NewModelController()
	sessionController := handlers.NewChatSessionController()
	userApiKeyController := handlers.NewUserApiKeyController()
	logController := handlers.NewLogController()
	toolController := handlers.NewToolController()
	userController := handlers.NewUserController()
	openAICompatibleController := handlers.NewOpenAICompatibleHandler()
	providerHandler := handlers.NewProviderHandler()
	providerKeyHandler := handlers.NewProviderKeyHandler()
	leaderboardHandler := handlers.NewLeaderboardHandler()
	embeddingHandler := handlers.NewEmbeddingHandler()
	imagesHandler := handlers.NewImagesHandler()
	videosHandler := handlers.NewVideosHandler()
	anthropicHandler := handlers.NewAnthropicCompatibleHandler()
	responsesHandler := handlers.NewResponsesHandler()
	dashboardHandler := handlers.NewDashboardHandler()
	apiTestHandler := handlers.NewApiTestHandler()
	billingHandler := handlers.NewBillingHandler()
	skillHandler := handlers.NewSkillHandler(cfg)
	agentTaskHandler := handlers.NewAgentTaskHandler(cfg)

	// Empty handler function
	emptyHandler := func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	}

	// Public routes - auth
	r.POST("/api/register", authHandler.Register)
	r.POST("/api/login", authHandler.Login)

	// Protected routes
	api := r.Group("/api")
	api.Use(middleware.AuthMiddleware(cfg))
	{
		// Auth routes
		api.POST("/logout", authHandler.Logout)
		api.GET("/user", authHandler.User)
		api.PUT("/user/profile", authHandler.UpdateProfile)
		api.POST("/user/avatar", authHandler.UploadAvatar)
		api.POST("/user/change-password", authHandler.ChangePassword)

		// Models
		api.GET("/models", modelController.Index)
		api.GET("/models/:id", modelController.Show)

		// Tools
		api.GET("/tools", func(c *gin.Context) {
			enabled := "true"
			c.Request.URL.RawQuery = "enabled=" + enabled
			toolController.Index(c)
		})

		// Chat
		api.POST("/chat/send", chatController.SendMessage)

		// Sessions
		sessions := api.Group("/sessions")
		{
			sessions.GET("", sessionController.Index)
			sessions.POST("", sessionController.Store)

			// Session messages (more specific routes first!)
			sessions.GET("/:id/messages", sessionController.MessageIndex)
			sessions.GET("/:id/messages/:message_id", sessionController.MessageShow)
			sessions.DELETE("/:id/messages/:message_id", sessionController.MessageDestroy)
			sessions.DELETE("/:id/messages", sessionController.ClearMessages)

			// Single session routes
			sessions.GET("/:id", sessionController.Show)
			sessions.PUT("/:id", sessionController.Update)
			sessions.DELETE("/:id", sessionController.Destroy)
		}

		// User API Keys
		userApiKeys := api.Group("/user/api-keys")
		{
			userApiKeys.GET("", userApiKeyController.Index)
			userApiKeys.POST("", userApiKeyController.Store)
			userApiKeys.GET("/:id", userApiKeyController.Show)
			userApiKeys.PUT("/:id", userApiKeyController.Update)
			userApiKeys.PUT("/:id/toggle-status", userApiKeyController.ToggleStatus)
			userApiKeys.DELETE("/:id", userApiKeyController.Destroy)
		}

		// User Logs
		userLogs := api.Group("/user/logs")
		{
			userLogs.GET("/usage", logController.UserUsageLogs)
			userLogs.GET("/operation", logController.UserOperationLogs)
			userLogs.GET("/stats", logController.UserUsageStats)
		}

		// Skills（用户端只读：查看管理后台配置的可用技能，不可编辑/修改状态）
		userSkills := api.Group("/skills")
		{
			userSkills.GET("", skillHandler.UserIndex)
			userSkills.GET("/:id", skillHandler.UserShow)
		}

		// Leaderboards
		leaderboards := api.Group("/leaderboards")
		{
			leaderboards.GET("", leaderboardHandler.Index)
			leaderboards.GET("/:id", leaderboardHandler.Show)
		}

		// Api Test
		apiTest := api.Group("/api-test")
		{
			apiTest.POST("/", apiTestHandler.Store)
			apiTest.GET("/history", apiTestHandler.History)
			apiTest.GET("/:id", apiTestHandler.Show)
			apiTest.PUT("/:id", apiTestHandler.Update)
			apiTest.DELETE("/:id", apiTestHandler.Destroy)
		}

		// Admin routes
		admin := api.Group("/admin")
		admin.Use(middleware.RoleMiddleware("admin"))
		{
			// Dashboard stats
			admin.GET("/dashboard/stats", dashboardHandler.Stats)
			// Admin AI Providers
			adminProviders := admin.Group("/ai-providers")
			{
				adminProviders.GET("", providerHandler.ListProviders)
				adminProviders.POST("", providerHandler.CreateProvider)
				adminProviders.GET("/presets", emptyHandler)
				adminProviders.GET("/presets/:code", emptyHandler)
				adminProviders.GET("/:id", providerHandler.GetProvider)
				adminProviders.PUT("/:id", providerHandler.UpdateProvider)
				adminProviders.PUT("/:id/toggle-status", providerHandler.ToggleProviderStatus)
				adminProviders.DELETE("/:id", providerHandler.DeleteProvider)

				// Provider Models
				adminProviders.GET("/:id/models", providerHandler.GetProviderModels)
				adminProviders.POST("/:id/models", providerHandler.CreateProviderModel)
				adminProviders.PUT("/:id/models/:modelId", providerHandler.UpdateProviderModel)
				adminProviders.DELETE("/:id/models/:modelId", providerHandler.DeleteProviderModel)
			}

			// Admin Provider Keys
			adminProviderKeys := admin.Group("/provider-keys")
			{
				adminProviderKeys.GET("", providerKeyHandler.ListProviderKeys)
				adminProviderKeys.POST("", providerKeyHandler.CreateProviderKey)
				adminProviderKeys.GET("/:id", providerKeyHandler.GetProviderKey)
				adminProviderKeys.PUT("/:id", providerKeyHandler.UpdateProviderKey)
				adminProviderKeys.DELETE("/:id", providerKeyHandler.DeleteProviderKey)
				adminProviderKeys.PUT("/:id/toggle-status", providerKeyHandler.ToggleProviderKeyStatus)
				adminProviderKeys.GET("/by-provider/:providerId", providerKeyHandler.GetProviderKeysByProvider)
			}

			// Admin Models
			adminModels := admin.Group("/models")
			{
				adminModels.GET("", modelController.AdminIndex)
				adminModels.POST("", modelController.Store)
				adminModels.GET("/:id", modelController.Show)
				adminModels.PUT("/:id", modelController.Update)
				adminModels.PUT("/:id/toggle-status", modelController.ToggleStatus)
				adminModels.DELETE("/:id", modelController.Destroy)
			}

			// Admin tools
			adminTools := admin.Group("/tools")
			{
				adminTools.GET("", toolController.Index)
				adminTools.POST("", toolController.Store)
				adminTools.GET("/:id", toolController.Show)
				adminTools.PUT("/:id", toolController.Update)
				adminTools.PUT("/:id/toggle-status", toolController.ToggleStatus)
				adminTools.DELETE("/:id", toolController.Destroy)
			}

			// Admin billing
			adminBilling := admin.Group("/billing")
			{
				adminBilling.GET("/stats", billingHandler.Stats)
				adminBilling.GET("/rates", billingHandler.Rates)
				adminBilling.PUT("/rates/:id", billingHandler.UpdateRate)
				adminBilling.GET("/records", billingHandler.Records)
			}

			// Admin skills (SKILL.md 技能包管理)
			adminSkills := admin.Group("/skills")
			{
				adminSkills.GET("", skillHandler.Index)
				adminSkills.POST("", skillHandler.Store)
				adminSkills.POST("/upload", skillHandler.Upload)
				adminSkills.GET("/:id", skillHandler.Show)
				adminSkills.GET("/:id/download", skillHandler.Download)
				adminSkills.PUT("/:id", skillHandler.Update)
				adminSkills.PUT("/:id/toggle-status", skillHandler.ToggleStatus)
				adminSkills.DELETE("/:id", skillHandler.Destroy)
			}

			// Admin users
			adminUsers := admin.Group("/users")
			{
				adminUsers.GET("", userController.Index)
				adminUsers.POST("", userController.Store)
				adminUsers.GET("/:id", userController.Show)
				adminUsers.PUT("/:id", userController.Update)
				adminUsers.PUT("/:id/toggle-status", userController.ToggleStatus)
			}

			// Admin logs
			adminLogs := admin.Group("/logs")
			{
				adminLogs.GET("/usage", logController.AllUsageLogs)
				adminLogs.GET("/operation", logController.AllOperationLogs)
			}

			// Admin Leaderboards
			adminLeaderboards := admin.Group("/leaderboards")
			{
				adminLeaderboards.GET("/configs", leaderboardHandler.Configs)
				adminLeaderboards.POST("/configs", leaderboardHandler.StoreConfig)
				adminLeaderboards.PUT("/configs/:id", leaderboardHandler.UpdateConfig)
				adminLeaderboards.PUT("/configs/:id/toggle-status", leaderboardHandler.ToggleConfigStatus)
				adminLeaderboards.DELETE("/configs/:id", leaderboardHandler.DestroyConfig)
			}
		}
	}

	// OpenAI Compatible API (external access with API keys)
	// 支持 /v1/ 和 /api/v1/ 两种路径
	for _, v1Prefix := range []string{"/v1", "/api/v1"} {
		v1 := r.Group(v1Prefix)
		v1.Use(middleware.ApiKeyAuthMiddleware())
		v1.Use(middleware.APILoggerMiddleware())
		{
			v1.GET("/models", openAICompatibleController.ListModels)
			v1.POST("/chat/completions", openAICompatibleController.ChatCompletions)
			v1.POST("/embeddings", embeddingHandler.Create)
			v1.POST("/images/generations", imagesHandler.Generations)
			v1.POST("/videos/generations/tasks", videosHandler.Generations)
			v1.GET("/videos/generations/tasks/:taskId", videosHandler.GetTask)
			v1.POST("/messages", anthropicHandler.Messages)
			v1.GET("/anthropic/models", anthropicHandler.ListModels)
			v1.POST("/anthropic/messages", anthropicHandler.Messages)
			v1.POST("/responses", responsesHandler.Create)

			// Agent Tasks（虚拟 Agent 任务：本地宿主机跑 SKILL 循环并产出资产）
			v1.POST("/agent/tasks", agentTaskHandler.Create)
			v1.POST("/agent/dispatch", agentTaskHandler.Dispatch)
			v1.GET("/agent/tasks/:taskId", agentTaskHandler.Show)
			v1.GET("/agent/tasks/:taskId/assets/*assetPath", agentTaskHandler.DownloadAsset)
		}
	}

	// v1 未匹配路由返回 OpenAI 错误信封（替代 gin 默认的纯文本 "404 page not found"）；
	// 其余前缀（管理后台等）保持默认行为
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/v1" || path == "/api/v1" ||
			strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/api/v1/") {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{
				"message": "Invalid URL (" + c.Request.Method + " " + path + ")",
				"type":    "invalid_request_error",
				"code":    "unknown_url",
			}})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "404 page not found"})
	})
}
