package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type UserController struct {
	BaseHandler
}

func NewUserController() *UserController {
	return &UserController{}
}

// Index gets all users (admin only)
func (h *UserController) Index(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	search := c.Query("search")

	var users []models.User
	var total int64

	query := repository.DB.Preload("Roles")

	if search != "" {
		query = query.Where("name LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Model(&models.User{}).Count(&total)
	query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&users)

	result := gin.H{
		"data": users,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	}

	h.SuccessResponse(c, result)
}

// Show gets a single user (admin only)
func (h *UserController) Show(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var user models.User
	if err := repository.DB.Preload("Roles").First(&user, id).Error; err != nil {
		h.ErrorResponse(c, "User not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, user)
}

// Store creates a new user (admin only)
func (h *UserController) Store(c *gin.Context) {
	// 先获取原始 JSON 数据来处理类型兼容性
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 手动解析请求，处理 roles 类型兼容性
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Roles    []uint `json:"roles"`
		Status   uint   `json:"status"`
	}

	// 解析基本字段
	if name, ok := raw["name"].(string); ok {
		req.Name = name
	}
	if email, ok := raw["email"].(string); ok {
		req.Email = email
	}
	if password, ok := raw["password"].(string); ok {
		req.Password = password
	}
	if status, ok := raw["status"].(float64); ok {
		req.Status = uint(status)
	} else if status, ok := raw["status"].(int); ok {
		req.Status = uint(status)
	}

	// 解析 roles 字段，处理类型兼容性（支持 number、string 和数组）
	if rolesRaw := raw["roles"]; rolesRaw != nil {
		if roles, ok := rolesRaw.([]interface{}); ok {
			// 处理数组类型
			req.Roles = make([]uint, 0, len(roles))
			for _, r := range roles {
				switch v := r.(type) {
				case float64:
					req.Roles = append(req.Roles, uint(v))
				case int:
					req.Roles = append(req.Roles, uint(v))
				case uint:
					req.Roles = append(req.Roles, v)
				case string:
					if id, err := strconv.Atoi(v); err == nil {
						req.Roles = append(req.Roles, uint(id))
					}
				}
			}
		} else {
			// 处理单个值类型
			req.Roles = make([]uint, 0, 1)
			switch v := rolesRaw.(type) {
			case float64:
				req.Roles = append(req.Roles, uint(v))
			case int:
				req.Roles = append(req.Roles, uint(v))
			case uint:
				req.Roles = append(req.Roles, v)
			case string:
				if id, err := strconv.Atoi(v); err == nil {
					req.Roles = append(req.Roles, uint(id))
				}
			}
		}
	}

	logger.Info("Creating user with roles: %v (count: %d)", req.Roles, len(req.Roles))

	// 验证必填字段
	if req.Name == "" || req.Email == "" || req.Password == "" {
		h.ValidationErrorResponse(c, "Name, email and password are required")
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
		Status:   1, // 默认正常状态
	}

	if req.Status > 0 {
		user.Status = req.Status
	}

	if err := repository.DB.Create(&user).Error; err != nil {
		h.ErrorResponse(c, "Failed to create user", http.StatusInternalServerError)
		return
	}

	logger.Info("User created with ID: %d", user.ID)

	if len(req.Roles) > 0 {
		logger.Info("Processing roles for user %d: %v", user.ID, req.Roles)
		var roles []models.Role
		if err := repository.DB.Where("id IN ?", req.Roles).Find(&roles).Error; err != nil {
			logger.Error("Failed to find roles: %v", err)
		} else {
			logger.Info("Found %d roles: %v", len(roles), roles)
			if len(roles) > 0 {
				// 方法1: 尝试使用 Association
				if err := repository.DB.Model(&user).Association("Roles").Append(&roles); err != nil {
					logger.Error("Failed to append roles via Association: %v", err)
				} else {
					logger.Info("Successfully appended %d roles to user %d via Association", len(roles), user.ID)
				}

				// 方法2: 使用 Preload 检查关联是否成功
				var userWithRoles models.User
				repository.DB.Preload("Roles").First(&userWithRoles, user.ID)
				logger.Info("User %d now has %d roles", user.ID, len(userWithRoles.Roles))

				// 方法3: 直接使用 SQL 插入（如果 Association 失败）
				if len(userWithRoles.Roles) == 0 {
					logger.Info("Trying direct SQL insert for roles")
					insertUserRoles(user.ID, req.Roles)
				}
			} else {
				logger.Error("No roles found for IDs: %v", req.Roles)
			}
		}
	} else {
		logger.Info("No roles to associate for user %d", user.ID)
	}

	repository.DB.Preload("Roles").First(&user, user.ID)

	h.SuccessResponseWithCode(c, user, "User created successfully", http.StatusCreated)
}

// Update updates a user (admin only)
func (h *UserController) Update(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var user models.User
	if err := repository.DB.First(&user, id).Error; err != nil {
		h.ErrorResponse(c, "User not found", http.StatusNotFound)
		return
	}

	// 先获取原始 JSON 数据来处理类型兼容性
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 手动解析请求
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Roles    []uint `json:"roles"`
		Status   *uint  `json:"status"`
	}

	// 解析基本字段
	if name, ok := raw["name"].(string); ok {
		req.Name = name
	}
	if email, ok := raw["email"].(string); ok {
		req.Email = email
	}
	if password, ok := raw["password"].(string); ok {
		req.Password = password
	}

	// 解析 status 字段
	if status, ok := raw["status"].(float64); ok {
		s := uint(status)
		req.Status = &s
	} else if status, ok := raw["status"].(int); ok {
		s := uint(status)
		req.Status = &s
	}

	// 解析 roles 字段，处理类型兼容性（支持 number、string 和数组）
	if rolesRaw := raw["roles"]; rolesRaw != nil {
		if roles, ok := rolesRaw.([]interface{}); ok {
			// 处理数组类型
			req.Roles = make([]uint, 0, len(roles))
			for _, r := range roles {
				switch v := r.(type) {
				case float64:
					req.Roles = append(req.Roles, uint(v))
				case int:
					req.Roles = append(req.Roles, uint(v))
				case uint:
					req.Roles = append(req.Roles, v)
				case string:
					// 处理字符串类型的角色ID（前端可能返回字符串）
					if id, err := strconv.Atoi(v); err == nil {
						req.Roles = append(req.Roles, uint(id))
					}
				}
			}
		} else {
			// 处理单个值类型
			req.Roles = make([]uint, 0, 1)
			switch v := rolesRaw.(type) {
			case float64:
				req.Roles = append(req.Roles, uint(v))
			case int:
				req.Roles = append(req.Roles, uint(v))
			case uint:
				req.Roles = append(req.Roles, v)
			case string:
				if id, err := strconv.Atoi(v); err == nil {
					req.Roles = append(req.Roles, uint(id))
				}
			}
		}
	}

	logger.Info("Updating user with roles: %v (count: %d)", req.Roles, len(req.Roles))

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		user.Password = string(hashedPassword)
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := repository.DB.Save(&user).Error; err != nil {
		h.ErrorResponse(c, "Failed to update user", http.StatusInternalServerError)
		return
	}

	if len(req.Roles) > 0 {
		logger.Info("Processing roles for user %d: %v", user.ID, req.Roles)
		var roles []models.Role
		if err := repository.DB.Where("id IN ?", req.Roles).Find(&roles).Error; err != nil {
			logger.Error("Failed to find roles: %v", err)
		} else if len(roles) > 0 {
			// 先清空现有角色
			if err := repository.DB.Model(&user).Association("Roles").Clear(); err != nil {
				logger.Error("Failed to clear roles: %v", err)
			}
			// 方法1: 尝试使用 Association
			if err := repository.DB.Model(&user).Association("Roles").Append(&roles); err != nil {
				logger.Error("Failed to append roles via Association: %v", err)
			} else {
				logger.Info("Successfully appended %d roles to user %d via Association", len(roles), user.ID)
			}

			// 方法2: 使用 Preload 检查关联是否成功
			var userWithRoles models.User
			repository.DB.Preload("Roles").First(&userWithRoles, user.ID)
			logger.Info("User %d now has %d roles", user.ID, len(userWithRoles.Roles))

			// 方法3: 直接使用 SQL 插入（如果 Association 失败）
			if len(userWithRoles.Roles) == 0 {
				logger.Info("Trying direct SQL insert for roles")
				insertUserRoles(user.ID, req.Roles)
			}
		} else {
			logger.Error("No roles found for IDs: %v", req.Roles)
		}
	}

	repository.DB.Preload("Roles").First(&user, user.ID)

	h.SuccessResponse(c, user, "User updated successfully")
}

// ToggleStatus toggles user status (admin only)
func (h *UserController) ToggleStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var user models.User
	if err := repository.DB.First(&user, id).Error; err != nil {
		h.ErrorResponse(c, "User not found", http.StatusNotFound)
		return
	}

	// 切换状态：1->0，0->1
	if user.Status == 1 {
		user.Status = 0
	} else {
		user.Status = 1
	}

	if err := repository.DB.Save(&user).Error; err != nil {
		h.ErrorResponse(c, "Failed to update user status", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, user, "User status updated successfully")
}

// insertUserRoles 批量写入 role_user 关联（替代逐条 INSERT 的 N 次往返）
func insertUserRoles(userID uint, roleIDs []uint) {
	if len(roleIDs) == 0 {
		return
	}
	values := make([]string, 0, len(roleIDs))
	args := make([]interface{}, 0, len(roleIDs)*2)
	for _, roleID := range roleIDs {
		values = append(values, "(?, ?)")
		args = append(args, userID, roleID)
	}
	query := "INSERT INTO role_user (user_id, role_id) VALUES " + strings.Join(values, ", ")
	if err := repository.DB.Exec(query, args...).Error; err != nil {
		logger.Error("Failed to batch insert role_user for user %d: %v", userID, err)
	} else {
		logger.Info("Inserted %d role_user rows for user %d", len(roleIDs), userID)
	}
}
