package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/api/middleware"
	"github.com/your-org/ai-transit-hub/internal/auth"
	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/service"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	BaseHandler
	cfg *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// Register 注册（已关闭）
func (h *AuthHandler) Register(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "注册功能已关闭，请联系管理员创建账户",
	})
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Debug("Bind error: %v", err)
		friendlyMsg := "请填写完整的登录信息"
		if strings.Contains(err.Error(), "Email") || strings.Contains(err.Error(), "email") {
			friendlyMsg = "请输入有效的邮箱地址"
		} else if strings.Contains(err.Error(), "Password") || strings.Contains(err.Error(), "password") {
			friendlyMsg = "密码长度至少6位"
		}
		recordAuthLog(c, service.OperationLogEntry{
			Type: "login", Module: "auth", Action: "POST /login",
			Params: gin.H{"email": req.Email},
			Status: "failed", ErrorMsg: friendlyMsg,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": friendlyMsg,
		})
		return
	}

	logger.Debug("Login attempt for email: %s", req.Email)

	var user models.User
	if err := repository.DB.Preload("Roles").Where("email = ?", req.Email).First(&user).Error; err != nil {
		logger.Debug("User not found or DB error: %v", err)
		recordAuthLog(c, service.OperationLogEntry{
			Type: "login", Module: "auth", Action: "POST /login",
			Params: gin.H{"email": req.Email},
			Status: "failed", ErrorMsg: "邮箱或密码错误",
		})
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "邮箱或密码错误，请重新输入",
		})
		return
	}

	logger.Debug("User found: %s ID: %d", user.Email, user.ID)
	logger.Debug("Stored password hash: %s", user.Password)
	logger.Debug("Provided password length: %d", len(req.Password))

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logger.Debug("Password comparison failed: %v", err)
		recordAuthLog(c, service.OperationLogEntry{
			Type: "login", Module: "auth", Action: "POST /login",
			Params: gin.H{"email": req.Email},
			Status: "failed", ErrorMsg: "邮箱或密码错误",
		})
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "邮箱或密码错误，请重新输入",
		})
		return
	}

	token, err := auth.GenerateToken(&user, h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "登录失败，请稍后重试",
		})
		return
	}

	recordAuthLog(c, service.OperationLogEntry{
		Type: "login", Module: "auth", Action: "POST /login",
		Params: gin.H{"email": req.Email},
		Status: "success",
		UserID: &user.ID, Username: user.Name,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "登录成功",
		"data": gin.H{
			"user":  user,
			"token": token,
		},
	})
}

// recordAuthLog 补全认证类操作日志的公共字段并落库
func recordAuthLog(c *gin.Context, entry service.OperationLogEntry) {
	entry.IP = c.ClientIP()
	entry.UserAgent = c.Request.UserAgent()
	service.Operation.Record(entry)
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	if user, ok := middleware.GetUser(c); ok {
		recordAuthLog(c, service.OperationLogEntry{
			Type: "logout", Module: "auth", Action: "POST /logout",
			Status: "success",
			UserID: &user.ID, Username: user.Name,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已安全退出",
	})
}

// User 获取当前用户信息
func (h *AuthHandler) User(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录后再操作",
		})
		return
	}

	if err := repository.DB.Preload("Roles").First(user, user.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取用户信息失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// UpdateProfile 更新用户信息
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录后再操作",
		})
		return
	}

	var req struct {
		Name       string `json:"name" binding:"required,max=255"`
		Department string `json:"department"`
		EmployeeID string `json:"employee_id"`
		Avatar     string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		friendlyMsg := "请填写完整的信息"
		if strings.Contains(err.Error(), "Name") || strings.Contains(err.Error(), "name") {
			friendlyMsg = "请输入昵称"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": friendlyMsg,
		})
		return
	}

	user.Name = req.Name
	user.Avatar = req.Avatar

	// 处理可空字段
	if req.Department != "" {
		user.Department = &req.Department
	} else {
		user.Department = nil
	}
	if req.EmployeeID != "" {
		user.EmployeeID = &req.EmployeeID
	} else {
		user.EmployeeID = nil
	}

	if err := repository.DB.Save(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新资料失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "资料更新成功",
		"data":    user,
	})
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	user, exists := middleware.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "请先登录后再操作",
		})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		friendlyMsg := "请填写完整的信息"
		if strings.Contains(err.Error(), "OldPassword") || strings.Contains(err.Error(), "old_password") {
			friendlyMsg = "请输入当前密码"
		} else if strings.Contains(err.Error(), "NewPassword") || strings.Contains(err.Error(), "new_password") {
			friendlyMsg = "新密码长度至少6位"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": friendlyMsg,
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "当前密码错误，请重新输入",
		})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "密码修改失败，请稍后重试",
		})
		return
	}

	user.Password = string(hashed)
	if err := repository.DB.Save(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "密码修改失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "密码修改成功",
	})
}

// UploadAvatar 上传头像（预留接口）
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"message": "头像上传功能暂未开放",
	})
}
