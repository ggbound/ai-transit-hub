package handlers

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/service"
)

// SkillHandler 管理后台 SKILL.md 技能包管理（独立模块）
type SkillHandler struct {
	BaseHandler
	storageRoot string
}

// skillMaxUploadSize 上传 zip 大小上限
const skillMaxUploadSize = 20 << 20

func NewSkillHandler(cfg *config.Config) *SkillHandler {
	return &SkillHandler{storageRoot: cfg.Storage.SkillPath}
}

// skillCreateRequest 在线创建/编辑 SKILL 时的请求体
type skillUpsertRequest struct {
	Name         *string      `json:"name"`
	Description  *string      `json:"description"`
	Version      *string      `json:"version"`
	License      *string      `json:"license"`
	AllowedTools *string      `json:"allowed_tools"`
	Content      *string      `json:"content"`
	Tags         *models.JSON `json:"tags"`
	Enabled      *bool        `json:"enabled"`
}

// Index 分页获取技能列表（支持关键字搜索与启用状态筛选）
func (h *SkillHandler) Index(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	search := strings.TrimSpace(c.Query("search"))
	enabled := c.Query("enabled")

	var skills []models.Skill
	var total int64

	query := repository.DB.Model(&models.Skill{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if enabled != "" {
		query = query.Where("enabled = ?", enabled == "true")
	}

	if err := query.Count(&total).Error; err != nil {
		h.ErrorResponse(c, "Failed to query skills", http.StatusInternalServerError)
		return
	}
	if err := query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&skills).Error; err != nil {
		h.ErrorResponse(c, "Failed to query skills", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{
		"data": skills,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	})
}

// UserIndex 用户端分页获取可用技能列表（只读，仅返回已启用技能，不含正文内容）
func (h *SkillHandler) UserIndex(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	search := strings.TrimSpace(c.Query("search"))

	var skills []models.Skill
	var total int64

	query := repository.DB.Model(&models.Skill{}).
		Where("enabled = ?", true).
		Select("id, name, description, version, license, allowed_tools, tags, file_count, enabled, created_at, updated_at")
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		h.ErrorResponse(c, "Failed to query skills", http.StatusInternalServerError)
		return
	}
	if err := query.Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&skills).Error; err != nil {
		h.ErrorResponse(c, "Failed to query skills", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, gin.H{
		"data": skills,
		"meta": gin.H{
			"current_page": page,
			"per_page":     perPage,
			"total":        total,
		},
	})
}

// UserShow 用户端查看单个已启用技能详情（只读，含附件列表与 SKILL.md 正文）
func (h *SkillHandler) UserShow(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id < 1 {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	var skill models.Skill
	if err := repository.DB.Where("enabled = ?", true).Preload("Files").First(&skill, id).Error; err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, skill)
}

// Show 获取单个技能详情（含附件列表）
func (h *SkillHandler) Show(c *gin.Context) {
	skill, err := h.findSkill(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	if err := repository.DB.Preload("Files").First(skill, skill.ID).Error; err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	h.SuccessResponse(c, skill)
}

// Store 在线创建技能（粘贴 SKILL.md 内容）
func (h *SkillHandler) Store(c *gin.Context) {
	var req skillUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if req.Content == nil || strings.TrimSpace(*req.Content) == "" {
		h.ValidationErrorResponse(c, "content 不能为空")
		return
	}

	meta, _, err := service.ParseSkillMarkdown(*req.Content)
	if err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	skill := models.Skill{
		Content:     *req.Content,
		Enabled:     true,
		Frontmatter: meta.Frontmatter,
	}
	h.applyUpsertRequest(&skill, &req, &meta)

	if err := service.ValidateSkillName(skill.Name); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if err := h.validateSkillColumnLimits(&skill); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if skill.Description == "" {
		h.ValidationErrorResponse(c, "description 不能为空（可在表单或 frontmatter 中提供）")
		return
	}
	if err := h.checkNameUnique(skill.Name, 0); err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusConflict)
		return
	}

	if user, ok := GetAuthUser(c); ok {
		skill.CreatedBy = user.ID
	}
	if err := repository.DB.Create(&skill).Error; err != nil {
		h.ErrorResponse(c, "Failed to create skill", http.StatusInternalServerError)
		return
	}

	h.SuccessResponseWithCode(c, skill, "Skill created successfully", http.StatusCreated)
}

// Upload 上传 zip 技能包（内含 SKILL.md）
func (h *SkillHandler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.ValidationErrorResponse(c, "请通过 file 字段上传 zip 文件")
		return
	}
	if fileHeader.Size > 20<<20 {
		h.ErrorResponse(c, "上传文件超过大小限制（20MB）", http.StatusBadRequest)
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		h.ErrorResponse(c, "读取上传文件失败", http.StatusBadRequest)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, skillMaxUploadSize+1))
	if err != nil {
		h.ErrorResponse(c, "读取上传文件失败", http.StatusBadRequest)
		return
	}
	if int64(len(data)) > skillMaxUploadSize {
		h.ErrorResponse(c, "上传文件超过大小限制（20MB）", http.StatusBadRequest)
		return
	}

	pkg, err := service.ExtractSkillPackage(data)
	if err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.checkNameUnique(pkg.Meta.Name, 0); err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusConflict)
		return
	}

	skill := models.Skill{
		Name:         pkg.Meta.Name,
		Description:  pkg.Meta.Description,
		Version:      orDefault(pkg.Meta.Version, "0.0.1"),
		License:      pkg.Meta.License,
		AllowedTools: pkg.Meta.AllowedTools,
		Content:      string(pkg.MainFile.Data),
		Frontmatter:  pkg.Meta.Frontmatter,
		Enabled:      true,
	}
	if user, ok := GetAuthUser(c); ok {
		skill.CreatedBy = user.ID
	}
	if err := h.validateSkillColumnLimits(&skill); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if err := repository.DB.Create(&skill).Error; err != nil {
		h.ErrorResponse(c, "Failed to create skill", http.StatusInternalServerError)
		return
	}

	if err := h.writeSkillAssets(&skill, pkg.Assets); err != nil {
		// 附件落盘失败时回滚记录，避免出现无文件的脏数据
		repository.DB.Delete(&models.Skill{}, skill.ID)
		h.ErrorResponse(c, "保存技能附件失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.SuccessResponseWithCode(c, skill, "Skill uploaded successfully", http.StatusCreated)
}

// Update 更新技能（支持在线编辑 SKILL.md 内容）
func (h *SkillHandler) Update(c *gin.Context) {
	skill, err := h.findSkill(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	var req skillUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}

	// 内容变更时重新解析 frontmatter
	var meta service.SkillMeta
	if req.Content != nil && *req.Content != skill.Content {
		if strings.TrimSpace(*req.Content) == "" {
			h.ValidationErrorResponse(c, "content 不能为空")
			return
		}
		parsedMeta, _, parseErr := service.ParseSkillMarkdown(*req.Content)
		if parseErr != nil {
			h.ValidationErrorResponse(c, parseErr.Error())
			return
		}
		meta = parsedMeta
		skill.Content = *req.Content
		skill.Frontmatter = meta.Frontmatter
	}
	h.applyUpsertRequest(skill, &req, &meta)

	if err := service.ValidateSkillName(skill.Name); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if err := h.validateSkillColumnLimits(skill); err != nil {
		h.ValidationErrorResponse(c, err.Error())
		return
	}
	if skill.Description == "" {
		h.ValidationErrorResponse(c, "description 不能为空")
		return
	}
	if err := h.checkNameUnique(skill.Name, skill.ID); err != nil {
		h.ErrorResponse(c, err.Error(), http.StatusConflict)
		return
	}

	if err := repository.DB.Save(skill).Error; err != nil {
		h.ErrorResponse(c, "Failed to update skill", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, skill, "Skill updated successfully")
}

// ToggleStatus 启用/停用技能
func (h *SkillHandler) ToggleStatus(c *gin.Context) {
	skill, err := h.findSkill(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	skill.Enabled = !skill.Enabled
	if err := repository.DB.Save(skill).Error; err != nil {
		h.ErrorResponse(c, "Failed to update skill", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, skill, "Skill status updated successfully")
}

// Download 将技能打包为 zip 下载
func (h *SkillHandler) Download(c *gin.Context) {
	skill, err := h.findSkill(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	var files []models.SkillFile
	repository.DB.Where("skill_id = ?", skill.ID).Find(&files)

	data, err := service.BuildSkillZip(skill, files, h.storageRoot)
	if err != nil {
		h.ErrorResponse(c, "Failed to build skill package", http.StatusInternalServerError)
		return
	}

	filename := skill.Name + ".zip"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/zip", data)
}

// Destroy 删除技能（级联删除附件记录与磁盘文件）
func (h *SkillHandler) Destroy(c *gin.Context) {
	skill, err := h.findSkill(c.Param("id"))
	if err != nil {
		h.ErrorResponse(c, "Skill not found", http.StatusNotFound)
		return
	}

	// 删除磁盘附件目录
	if skill.StoragePath != "" {
		if dir, pathErr := service.SkillFilePath(h.storageRoot, skill.StoragePath, "."); pathErr == nil {
			os.RemoveAll(dir)
		}
	}

	if err := repository.DB.Where("skill_id = ?", skill.ID).Delete(&models.SkillFile{}).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete skill files", http.StatusInternalServerError)
		return
	}
	if err := repository.DB.Delete(skill).Error; err != nil {
		h.ErrorResponse(c, "Failed to delete skill", http.StatusInternalServerError)
		return
	}

	h.SuccessResponse(c, nil, "Skill deleted successfully")
}

// ---------- internal helpers ----------

func (h *SkillHandler) findSkill(idParam string) (*models.Skill, error) {
	id, err := strconv.Atoi(idParam)
	if err != nil || id < 1 {
		return nil, fmt.Errorf("invalid id")
	}
	var skill models.Skill
	if err := repository.DB.First(&skill, id).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// applyUpsertRequest 将请求体字段应用到技能记录；显式字段优先于 frontmatter 解析结果
func (h *SkillHandler) applyUpsertRequest(skill *models.Skill, req *skillUpsertRequest, meta *service.SkillMeta) {
	if req.Name != nil {
		skill.Name = strings.TrimSpace(*req.Name)
	} else if skill.Name == "" {
		skill.Name = meta.Name
	}
	if req.Description != nil {
		skill.Description = strings.TrimSpace(*req.Description)
	} else if skill.Description == "" {
		skill.Description = meta.Description
	}
	if req.Version != nil {
		skill.Version = strings.TrimSpace(*req.Version)
	} else if skill.Version == "" && meta.Version != "" {
		skill.Version = meta.Version
	}
	if req.License != nil {
		skill.License = strings.TrimSpace(*req.License)
	} else if skill.License == "" && meta.License != "" {
		skill.License = meta.License
	}
	if req.AllowedTools != nil {
		skill.AllowedTools = strings.TrimSpace(*req.AllowedTools)
	} else if skill.AllowedTools == "" && meta.AllowedTools != "" {
		skill.AllowedTools = meta.AllowedTools
	}
	if req.Tags != nil {
		skill.Tags = *req.Tags
	}
	if req.Enabled != nil {
		skill.Enabled = *req.Enabled
	}
}

func (h *SkillHandler) checkNameUnique(name string, excludeID uint) error {
	var count int64
	query := repository.DB.Model(&models.Skill{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	query.Count(&count)
	if count > 0 {
		return fmt.Errorf("技能标识 %s 已存在", name)
	}
	return nil
}

// validateSkillColumnLimits 校验技能字段不超过数据库列长
// （skills.version varchar(32)、license varchar(64)、description varchar(1024)）
func (h *SkillHandler) validateSkillColumnLimits(skill *models.Skill) error {
	if len([]rune(skill.Version)) > 32 {
		return fmt.Errorf("version 长度不能超过 32 个字符")
	}
	if len([]rune(skill.License)) > 64 {
		return fmt.Errorf("license 长度不能超过 64 个字符")
	}
	if len([]rune(skill.Description)) > 1024 {
		return fmt.Errorf("description 长度不能超过 1024 个字符")
	}
	return nil
}

// writeSkillAssets 将附件写入磁盘并记录索引
func (h *SkillHandler) writeSkillAssets(skill *models.Skill, assets []service.PackageFile) error {
	if len(assets) == 0 {
		return nil
	}

	dir := filepath.Join(h.storageRoot, strconv.FormatUint(uint64(skill.ID), 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	skill.StoragePath = dir

	fileRows := make([]models.SkillFile, 0, len(assets))
	for _, asset := range assets {
		target, err := service.SkillFilePath(h.storageRoot, skill.StoragePath, asset.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, asset.Data, 0o644); err != nil {
			return err
		}
		fileRows = append(fileRows, models.SkillFile{
			SkillID:  skill.ID,
			Path:     asset.Path,
			Size:     int64(len(asset.Data)),
			MimeType: mime.TypeByExtension(filepath.Ext(asset.Path)),
		})
	}

	if err := repository.DB.Create(&fileRows).Error; err != nil {
		return err
	}
	skill.FileCount = len(fileRows)
	return repository.DB.Model(skill).Updates(map[string]interface{}{
		"storage_path": skill.StoragePath,
		"file_count":   skill.FileCount,
	}).Error
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
