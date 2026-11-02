package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

type ChatSessionController struct {
	BaseHandler
}

func NewChatSessionController() *ChatSessionController {
	return &ChatSessionController{}
}

type CreateSessionRequest struct {
	ModelID       uint   `json:"model_id" binding:"required"`
	Title         string `json:"title"`
	SystemPrompt  string `json:"system_prompt"`
	StreamEnabled *bool  `json:"stream_enabled"`
}

type UpdateSessionRequest struct {
	Title         string `json:"title"`
	SystemPrompt  string `json:"system_prompt"`
	StreamEnabled *bool  `json:"stream_enabled"`
}

// Index 获取会话列表（支持 page/per_page 分页，默认 per_page=100，data 保持数组兼容前端）
func (h *ChatSessionController) Index(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "100"))
	if perPage < 1 || perPage > 500 {
		perPage = 100
	}

	var total int64
	if err := repository.DB.Model(&models.ChatSession{}).Where("user_id = ?", user.ID).Count(&total).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch sessions", http.StatusInternalServerError)
		return
	}

	var sessions []models.ChatSession
	if err := repository.DB.Where("user_id = ?", user.ID).Preload("Model").Order("created_at DESC").
		Offset((page - 1) * perPage).Limit(perPage).Find(&sessions).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch sessions", http.StatusInternalServerError)
		return
	}

	lastPage := (int(total) + perPage - 1) / perPage
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    sessions,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
			"last_page":    lastPage,
		},
	})
}

// Show 获取单个会话详情
func (h *ChatSessionController) Show(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).Preload("Model").Preload("Messages").First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    session,
	})
}

// Store 创建新会话
func (h *ChatSessionController) Store(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	title := truncateTitle(req.Title)
	if title == "" {
		title = "新会话"
	}

	streamEnabled := true
	if req.StreamEnabled != nil {
		streamEnabled = *req.StreamEnabled
	}

	session := models.ChatSession{
		UserID:        user.ID,
		ModelID:       req.ModelID,
		Title:         title,
		SystemPrompt:  req.SystemPrompt,
		StreamEnabled: streamEnabled,
	}

	if err := repository.DB.Create(&session).Error; err != nil {
		h.ErrorResponse(c, "Failed to create session", http.StatusInternalServerError)
		return
	}

	repository.DB.Preload("Model").First(&session, session.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Session created successfully",
		"data":    session,
	})
}

// Update 更新会话
func (h *ChatSessionController) Update(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	var req UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	if req.Title != "" {
		session.Title = truncateTitle(req.Title)
	}
	session.SystemPrompt = req.SystemPrompt
	if req.StreamEnabled != nil {
		session.StreamEnabled = *req.StreamEnabled
	}

	if err := repository.DB.Save(&session).Error; err != nil {
		h.ErrorResponse(c, "Failed to update session", http.StatusInternalServerError)
		return
	}

	repository.DB.Preload("Model").First(&session, session.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session updated successfully",
		"data":    session,
	})
}

// Destroy 删除会话
func (h *ChatSessionController) Destroy(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if err := repository.DB.Where("id = ? AND user_id = ?", id, user.ID).Delete(&models.ChatSession{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session deleted successfully",
		"data":    nil,
	})
}

// truncateTitle 将会话标题按 rune 截断到数据库列长（chat_sessions.title 为 varchar(191)），
// 避免超长标题触发 Error 1406 导致建会话/改名整体失败
func truncateTitle(s string) string {
	runes := []rune(s)
	if len(runes) > 191 {
		return string(runes[:191])
	}
	return s
}

// MessageIndex 获取会话消息列表
func (h *ChatSessionController) MessageIndex(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	sessionID, _ := strconv.Atoi(c.Param("id"))
	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", sessionID, user.ID).First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	var messages []models.ChatMessage
	if err := repository.DB.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&messages).Error; err != nil {
		h.ErrorResponse(c, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    messages,
	})
}

// MessageShow 获取单条消息
func (h *ChatSessionController) MessageShow(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	sessionID, _ := strconv.Atoi(c.Param("id"))
	messageID, _ := strconv.Atoi(c.Param("message_id"))

	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", sessionID, user.ID).First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	var message models.ChatMessage
	if err := repository.DB.Where("id = ? AND session_id = ?", messageID, sessionID).First(&message).Error; err != nil {
		h.ErrorResponse(c, "Message not found", http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Success",
		"data":    message,
	})
}

// MessageDestroy 删除消息
func (h *ChatSessionController) MessageDestroy(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	sessionID, _ := strconv.Atoi(c.Param("id"))
	messageID, _ := strconv.Atoi(c.Param("message_id"))

	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", sessionID, user.ID).First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	if err := repository.DB.Where("id = ? AND session_id = ?", messageID, sessionID).Delete(&models.ChatMessage{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete message", http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Message deleted successfully",
		"data":    nil,
	})
}

// ClearMessages 清空会话的全部消息（Chat 页"清空对话"，DELETE /sessions/:id/messages）
func (h *ChatSessionController) ClearMessages(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		h.ErrorResponse(c, "User not found", http.StatusUnauthorized)
		return
	}

	sessionID, _ := strconv.Atoi(c.Param("id"))

	var session models.ChatSession
	if err := repository.DB.Where("id = ? AND user_id = ?", sessionID, user.ID).First(&session).Error; err != nil {
		h.ErrorResponse(c, "Session not found", http.StatusNotFound)
		return
	}

	if err := repository.DB.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to clear messages", http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Messages cleared successfully",
		"data":    nil,
	})
}
