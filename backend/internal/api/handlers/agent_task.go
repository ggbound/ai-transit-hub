package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/config"
	log "github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/service"
	"github.com/your-org/ai-transit-hub/internal/utils"
)

// agentTaskMaxIterationsLimit 单任务最大轮次上限（防失控扣费）
const agentTaskMaxIterationsLimit = 100

// agentTaskDefaultMaxIterations 未显式指定轮次时的默认值（8 轮对页面级生成任务不够用）
const agentTaskDefaultMaxIterations = 100

// wrapAgentBindError 请求体绑定/校验失败的统一包装：
// 将 Gin validator 错误转成机器可读的字段级错误码（model_required/skills_required/input_required），
// 避免把 Go 结构体内部名（如 createAgentTaskRequest.Model）透给调用方
func wrapAgentBindError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) || len(ve) == 0 {
		// JSON 语法错误或字段类型不匹配：无字段级信息，统一为 invalid_json
		respondV1Error(c, http.StatusBadRequest, "request body is not valid JSON", "invalid_request_error", "body", "invalid_json")
		return
	}
	fe := ve[0]
	field := strings.ToLower(fe.Field()) // 与 JSON 字段名一致（model/skills/input）
	code := "invalid_request"
	msg := fmt.Sprintf("field %q failed on %q validation", field, fe.Tag())
	switch {
	case fe.Field() == "Model" && fe.Tag() == "required":
		code, msg = "model_required", "model is required (provider/model format)"
	case fe.Field() == "Skills":
		code, msg = "skills_required", "skills is required (non-empty array of skill names)"
	case fe.Field() == "Input" && fe.Tag() == "required":
		code, msg = "input_required", "input is required (task description or free text)"
	}
	respondV1Error(c, http.StatusBadRequest, msg, "invalid_request_error", field, code)
}

// AgentTaskHandler 虚拟 Agent 任务端点（v1 组，API Key 鉴权）：
// POST /v1/agent/tasks                      创建异步任务，返回 task_id
// GET  /v1/agent/tasks/:taskId              轮询任务状态（含 result/skills_used/assets/usage）
// GET  /v1/agent/tasks/:taskId/assets/:name 下载任务工作区产出的资产文件
type AgentTaskHandler struct {
	cfg     *config.Config
	runtime *service.AgentRuntime
}

// NewAgentTaskHandler 创建 handler（技能附件根 + 任务工作区根来自存储配置）。
// venv 模式（AGENT_PY_ENV_MODE=venv）时顺带启动过期任务 Python 环境的 TTL 清理循环
func NewAgentTaskHandler(cfg *config.Config) *AgentTaskHandler {
	pyenv := service.NewPyEnvManager(cfg.Agent.PyEnv, cfg.Storage.AgentTaskPath)
	pyenv.StartCleanupLoop()
	return &AgentTaskHandler{
		cfg:     cfg,
		runtime: service.NewAgentRuntime(cfg.Storage.SkillPath, cfg.Storage.AgentTaskPath, cfg.Agent.AllowWorkspaceScript, pyenv, cfg.Agent.Sandbox),
	}
}

// createAgentTaskRequest 创建任务的输入契约（通用 SKILL 虚拟 Agent）：
// model 与 skills 是仅有的两个必填结构化参数，其余为用户输入数据
type createAgentTaskRequest struct {
	Model         string                 `json:"model" binding:"required"`        // 平台模型标识 provider/model（与 /v1/chat/completions 同约定）
	Skills        []string               `json:"skills" binding:"required,min=1"` // 要装载的 SKILL 名称（skills.name，SKILL.md frontmatter）
	Input         string                 `json:"input" binding:"required"`        // 用户输入数据（任务描述/自由文本）
	MaxIterations int                    `json:"max_iterations"`                  // 可选：Agent 循环轮次上限，默认 100
	Options       map[string]interface{} `json:"options"`                         // 可选：透传给模型的额外参数（temperature 等）
}

// agentRequestMeta goroutine 内记账所需的请求快照（handler 返回后 gin.Context 会被回收）
type agentRequestMeta struct {
	Method   string
	Endpoint string
	Headers  map[string][]string
	Body     json.RawMessage // 原始请求体 JSON（api_test_details.request_body 落库用）
	ApiKeyID uint            // 用户侧 API Key ID（子模型调用计费落库用）
}

// captureRawBody 读取原始请求体并恢复 Body 供后续绑定使用（goroutine 内记账落库用）
func captureRawBody(c *gin.Context) json.RawMessage {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	return json.RawMessage(raw)
}

// Create POST /v1/agent/tasks — 校验输入、落库 pending 任务、起异步 goroutine、立即返回 task_id
func (h *AgentTaskHandler) Create(c *gin.Context) {
	rawBody := captureRawBody(c)
	var req createAgentTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		wrapAgentBindError(c, err)
		return
	}

	user, ok := GetAuthUser(c)
	if !ok || user == nil {
		respondV1Error(c, http.StatusUnauthorized, "authentication required", "invalid_request_error", "", nil)
		return
	}
	userApiKeyID := uint(0)
	if id, exists := c.Get("api_key_id"); exists {
		if idUint, ok := id.(uint); ok {
			userApiKeyID = idUint
		}
	}

	// 模型解析：provider/model 字符串（带 60s TTL 缓存，provider/model 的 status=1 校验内置）
	provider, model, ok := parseV1Model(c, req.Model)
	if !ok {
		return
	}
	// Agent 任务需要对话型模型（工具循环依赖 function calling）
	if model.Type != "text" {
		respondV1Error(c, http.StatusBadRequest,
			fmt.Sprintf("model %q (type=%s) is not a chat model; agent tasks require type=text", req.Model, model.Type),
			"invalid_request_error", "model", "model_not_supported")
		return
	}

	// 轮次上限
	if req.MaxIterations <= 0 {
		req.MaxIterations = agentTaskDefaultMaxIterations
	}
	if req.MaxIterations > agentTaskMaxIterationsLimit {
		req.MaxIterations = agentTaskMaxIterationsLimit
	}

	// 技能装载：按 SKILL 名称解析（对齐 SKILL.md 开放标准的 name 引用），仅 enabled
	skills := loadSkillsByNames(c, uniqueNonEmpty(req.Skills))
	if skills == nil {
		return
	}

	h.launchAgentTask(c, user, userApiKeyID, provider, model, skills, req.Input, req.MaxIterations, req.Options, nil, "agent/tasks", rawBody)
}

// dispatchAgentTaskRequest 语义路由任务契约（/v1/agent/dispatch）：
// 只需描述需求 input；skills 与 model 均可选——缺省时由路由模型在 enabled 技能目录上做 LLM 语义路由
type dispatchAgentTaskRequest struct {
	Model         string                 `json:"model"`                    // 可选：优先用作路由模型与执行模型（provider/model）
	Skills        []string               `json:"skills"`                   // 可选：显式指定 SKILL 名称（提供时跳过语义路由）
	Input         string                 `json:"input" binding:"required"` // 用户需求描述（自由文本）
	MaxIterations int                    `json:"max_iterations"`           // 可选：Agent 循环轮次上限，默认 100
	Options       map[string]interface{} `json:"options"`                  // 可选：透传给模型的额外参数
}

// Dispatch POST /v1/agent/dispatch — 需求→SKILL 语义路由→异步执行：
//   - 显式 skills：直接装载，跳过路由
//   - 否则：用路由模型（req.model > AGENT_ROUTER_MODEL）在 enabled 技能目录上 LLM 语义路由
//   - 执行模型四级回退：req.model > 技能声明 model > 路由建议 model > AGENT_ROUTER_MODEL；全空则 400
func (h *AgentTaskHandler) Dispatch(c *gin.Context) {
	rawBody := captureRawBody(c)
	var req dispatchAgentTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		wrapAgentBindError(c, err)
		return
	}

	user, ok := GetAuthUser(c)
	if !ok || user == nil {
		respondV1Error(c, http.StatusUnauthorized, "authentication required", "invalid_request_error", "", nil)
		return
	}
	userApiKeyID := uint(0)
	if id, exists := c.Get("api_key_id"); exists {
		if idUint, ok := id.(uint); ok {
			userApiKeyID = idUint
		}
	}

	// 轮次上限
	if req.MaxIterations <= 0 {
		req.MaxIterations = agentTaskDefaultMaxIterations
	}
	if req.MaxIterations > agentTaskMaxIterationsLimit {
		req.MaxIterations = agentTaskMaxIterationsLimit
	}

	// 技能目录：语义路由与路由结果装载的公共基础（仅 enabled）
	var catalog []models.Skill
	if err := repository.DB.Where("enabled = ?", true).Order("name").Find(&catalog).Error; err != nil {
		respondV1Error(c, http.StatusInternalServerError, "failed to load skill catalog", "api_error", "", nil)
		return
	}
	if len(catalog) == 0 {
		respondV1Error(c, http.StatusBadRequest, "no enabled skills available", "invalid_request_error", "skills", "no_skills_available")
		return
	}

	// 路由：显式 skills 直接命中；否则 LLM 语义路由
	var routed *service.SkillRouteDecision
	var skills []models.Skill
	if explicit := uniqueNonEmpty(req.Skills); len(explicit) > 0 {
		skills = loadSkillsByNames(c, explicit)
		if skills == nil {
			return
		}
		routed = &service.SkillRouteDecision{Skills: explicit, Reason: "explicit skills provided"}
	} else {
		decision, ok := h.routeSkills(c, req.Input, req.Model, catalog)
		if !ok {
			return
		}
		routed = decision
		// 按路由结果从目录装载（RouteSkills 已保证全部命中目录）
		byName := make(map[string]models.Skill, len(catalog))
		for _, s := range catalog {
			byName[s.Name] = s
		}
		skills = make([]models.Skill, 0, len(routed.Skills))
		for _, n := range routed.Skills {
			if s, hit := byName[n]; hit {
				skills = append(skills, s)
			}
		}
	}

	// 执行模型四级回退：req.model > 技能声明 > 路由建议 > 配置兜底
	// （技能 frontmatter 声明了 model_requirements.exec.model 时优先生效）
	execModel := req.Model
	if execModel == "" {
		execModel = service.DeclaredExecModel(skills)
	}
	if execModel == "" {
		execModel = routed.Model
	}
	if execModel == "" {
		execModel = h.cfg.Agent.RouterModel
	}
	if execModel == "" {
		respondV1Error(c, http.StatusBadRequest,
			"no execution model: specify model, let the router suggest one, or configure AGENT_ROUTER_MODEL",
			"invalid_request_error", "model", "model_required")
		return
	}
	routed.Model = execModel

	// 执行模型解析与校验（Agent 循环依赖 function calling，须对话型模型）
	provider, model, ok := parseV1Model(c, execModel)
	if !ok {
		return
	}
	if model.Type != "text" {
		respondV1Error(c, http.StatusBadRequest,
			fmt.Sprintf("model %q (type=%s) is not a chat model; agent tasks require type=text", execModel, model.Type),
			"invalid_request_error", "model", "model_not_supported")
		return
	}

	h.launchAgentTask(c, user, userApiKeyID, provider, model, skills, req.Input, req.MaxIterations, req.Options, routed, "agent/dispatch", rawBody)
}

// agentTaskDetailResponse 任务详情（AgentTask 基础字段 + 工具调用步骤 trace）
type agentTaskDetailResponse struct {
	models.AgentTask
	Steps []models.AgentTaskStep `json:"steps"`
}

// Show GET /v1/agent/tasks/:taskId — 轮询任务状态（仅本人可见；含按序工具调用步骤 trace）
func (h *AgentTaskHandler) Show(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok || user == nil {
		respondV1Error(c, http.StatusUnauthorized, "authentication required", "invalid_request_error", "", nil)
		return
	}
	var task models.AgentTask
	if err := repository.DB.Where("task_id = ? AND user_id = ?", c.Param("taskId"), user.ID).First(&task).Error; err != nil {
		respondV1Error(c, http.StatusNotFound, "task not found", "invalid_request_error", "task_id", "task_not_found")
		return
	}
	var steps []models.AgentTaskStep
	if err := repository.DB.Where("task_id = ?", task.TaskID).Order("seq").Find(&steps).Error; err != nil {
		log.Error("[AgentTask] %s load steps: %v", task.TaskID, err)
		steps = []models.AgentTaskStep{}
	}
	c.JSON(http.StatusOK, agentTaskDetailResponse{AgentTask: task, Steps: steps})
}

// DownloadAsset GET /v1/agent/tasks/:taskId/assets/*assetPath — 下载任务产出的资产文件（支持嵌套相对路径）
func (h *AgentTaskHandler) DownloadAsset(c *gin.Context) {
	user, ok := GetAuthUser(c)
	if !ok || user == nil {
		respondV1Error(c, http.StatusUnauthorized, "authentication required", "invalid_request_error", "", nil)
		return
	}
	var task models.AgentTask
	if err := repository.DB.Where("task_id = ? AND user_id = ?", c.Param("taskId"), user.ID).First(&task).Error; err != nil {
		respondV1Error(c, http.StatusNotFound, "task not found", "invalid_request_error", "task_id", "task_not_found")
		return
	}
	// gin 通配参数值带前导 /，还原为资产相对路径（如 /productions/hero/BRIEF.md）
	name := strings.TrimPrefix(c.Param("assetPath"), "/")
	path, err := h.runtime.AssetFilePath(task.TaskID, name)
	if err != nil {
		respondV1Error(c, http.StatusBadRequest, err.Error(), "invalid_request_error", "name", "invalid_asset_name")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		respondV1Error(c, http.StatusNotFound, "asset not found", "invalid_request_error", "name", "asset_not_found")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		respondV1Error(c, http.StatusNotFound, "asset not found", "invalid_request_error", "name", "asset_not_found")
		return
	}
	// 用 ServeContent 而非 c.File：http.ServeFile 对以 index.html 结尾的路径会做 canonical 301 重定向（Location: ./）
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}

// runAgentTask 异步执行体：建适配器 → 虚拟 Agent 循环 → 更新任务与计费记录
func (h *AgentTaskHandler) runAgentTask(task *models.AgentTask, model *models.AiModel, provider *models.AiProvider,
	skills []models.Skill, recordIDs *service.RecordIDs, options map[string]interface{}, meta agentRequestMeta) {

	defer func() {
		if r := recover(); r != nil {
			log.Error("[AgentTask] %s panicked: %v", task.TaskID, r)
			// 终态后 panic 不回写：成功落账后的 panic（如子调用合并阶段）不得把 completed/partial 覆盖为 failed
			if h.taskReachedFinalState(task.ID) {
				return
			}
			h.markTaskFailed(task, nil, fmt.Sprintf("internal panic: %v", r))
			service.Accounting.UpdateErrorRecords(recordIDs, 500, fmt.Sprintf("agent task panic: %v", r),
				meta.Method, meta.Endpoint, meta.Headers, meta.Body)
		}
	}()

	// 标记 running
	now := time.Now()
	repository.DB.Model(&models.AgentTask{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"status": models.AgentTaskStatusRunning, "started_at": &now})

	// 平台侧 API Key（多 Key 随机负载分摊）
	apiKey, err := ai.GetRandomApiKey(provider.ID)
	if err != nil {
		log.Error("[AgentTask] %s no active api keys for provider_id=%d: %v", task.TaskID, provider.ID, err)
		h.markTaskFailed(task, nil, "no active provider api keys")
		service.Accounting.UpdateErrorRecords(recordIDs, 500, err.Error(), meta.Method, meta.Endpoint, meta.Headers, meta.Body)
		return
	}

	factory := ai.NewAdapterFactory()
	adapter, err := factory.CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		log.Error("[AgentTask] %s adapter creation failed: %v", task.TaskID, err)
		h.markTaskFailed(task, nil, "adapter creation failed: "+err.Error())
		service.Accounting.UpdateErrorRecords(recordIDs, 500, "Failed to create adapter: "+err.Error(),
			meta.Method, meta.Endpoint, meta.Headers, meta.Body)
		return
	}

	// 虚拟 Agent 循环（后台任务，脱离请求生命周期）
	result, runErr := h.runtime.RunTask(context.Background(), task, skills, adapter, options)

	// 组装回传负载（同时用于任务记录与计费回填）
	responsePayload := h.buildResponsePayload(task, result, runErr)

	if runErr != nil {
		log.Error("[AgentTask] %s failed at iteration %d: %v", task.TaskID, taskIterations(result), runErr)
		h.markTaskFailed(task, result, runErr.Error())
		// 带用量落失败记录：主模型已消耗的 tokens 同样写入 usage_log，保持与 agent_tasks 记账口径一致
		var errPrompt, errCompletion, errTotal int
		var errCost float64
		if result != nil {
			errPrompt = result.PromptTokens
			errCompletion = result.CompletionTokens
			errTotal = result.TotalTokens
			errCost = utils.CalculateCost(errPrompt, errCompletion, model)
		}
		service.Accounting.UpdateErrorRecordsWithUsage(recordIDs, 500, runErr.Error(),
			errPrompt, errCompletion, errTotal, errCost,
			meta.Method, meta.Endpoint, meta.Headers, meta.Body)
		// 循环中断前已发生的子模型调用（image/video）同样落计费
		h.billSubModelCalls(task, result, meta, recordIDs)
		return
	}

	// 成功返回后的状态判定：迭代耗尽/空产出不再静默 completed
	cost := utils.CalculateCost(result.PromptTokens, result.CompletionTokens, model)
	completedAt := time.Now()
	finalStatus, errMsg := classifyRunOutcome(task, result)

	// 产物契约：必需产物缺失时 completed 降级为 partial；partial/failed 的说明追加缺失明细
	if len(result.MissingOutputs) > 0 {
		missingMsg := fmt.Sprintf("missing required outputs: %v", result.MissingOutputs)
		switch finalStatus {
		case models.AgentTaskStatusCompleted:
			finalStatus = models.AgentTaskStatusPartial
			errMsg = missingMsg
		default:
			if errMsg != "" {
				errMsg += "; " + missingMsg
			} else {
				errMsg = missingMsg
			}
		}
	}

	updates := map[string]interface{}{
		"status":            finalStatus,
		"result":            result.FinalMessage,
		"prompt_tokens":     result.PromptTokens,
		"completion_tokens": result.CompletionTokens,
		"total_tokens":      result.TotalTokens,
		"cost":              cost,
		"completed_at":      &completedAt,
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	if err := repository.DB.Model(&models.AgentTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		log.Error("[AgentTask] %s persist final state: %v", task.TaskID, err)
	}

	// 计费回填：partial 视为有交付（按成功记账），failed 状态带已消耗用量
	if finalStatus == models.AgentTaskStatusFailed {
		service.Accounting.UpdateErrorRecordsWithUsage(recordIDs, 500, errMsg,
			result.PromptTokens, result.CompletionTokens, result.TotalTokens, cost,
			meta.Method, meta.Endpoint, meta.Headers, meta.Body)
		h.billSubModelCalls(task, result, meta, recordIDs)
		log.Error("[AgentTask] %s finished as failed: %s", task.TaskID, errMsg)
		return
	}
	service.Accounting.UpdateSuccessRecords(recordIDs, 200, responsePayload,
		result.PromptTokens, result.CompletionTokens, result.TotalTokens, cost,
		meta.Method, meta.Endpoint, meta.Headers, meta.Body, model)
	h.billSubModelCalls(task, result, meta, recordIDs)
	log.Info("[AgentTask] %s %s: iterations=%d, assets=%d, tokens=%d", task.TaskID, finalStatus, result.Iterations, len(result.Assets), result.TotalTokens)
}

// billSubModelCalls 将 Agent 循环内的子模型调用（generate_image/generate_video）单独落计费记录：
// 每个子调用独立建 usage_log/api_test_request（endpoint 带冒号后缀区分类型，externalTaskID 关联任务），
// 按子调用的实际成败回填用量与费用；同时把子调用用量与费用增量累加进主任务的 usage_log 与 agent_tasks，
// 使主日志与任务详情 API 均反映总消耗（exec 模型 + 子模型）。子记录保留，用于按调用类型区分。
func (h *AgentTaskHandler) billSubModelCalls(task *models.AgentTask, result *service.AgentRunResult, meta agentRequestMeta, recordIDs *service.RecordIDs) {
	if result == nil || len(result.SubModelCalls) == 0 {
		return
	}
	var subPrompt, subCompletion, subTotal int
	var subCost float64
	for _, sub := range result.SubModelCalls {
		var subModel models.AiModel
		if err := repository.DB.First(&subModel, sub.ModelID).Error; err != nil {
			log.Error("[AgentTask] %s bill sub-%s call: model %d not found: %v", task.TaskID, sub.Kind, sub.ModelID, err)
			continue
		}
		var subProvider models.AiProvider
		if err := repository.DB.First(&subProvider, sub.ProviderID).Error; err != nil {
			log.Error("[AgentTask] %s bill sub-%s call: provider %d not found: %v", task.TaskID, sub.Kind, sub.ProviderID, err)
			continue
		}
		subRecordIDs, err := service.Accounting.CreateInitialRecords(
			task.UserID, &subModel, &subProvider, "openai",
			meta.Endpoint+":"+sub.Kind, meta.ApiKeyID, 0, task.TaskID, "api",
		)
		if err != nil {
			log.Error("[AgentTask] %s bill sub-%s call: create records failed: %v", task.TaskID, sub.Kind, err)
			continue
		}
		cost := utils.CalculateCost(sub.PromptTokens, sub.CompletionTokens, &subModel)
		subPrompt += sub.PromptTokens
		subCompletion += sub.CompletionTokens
		subTotal += sub.TotalTokens
		subCost += cost
		if !sub.OK {
			errMsg := sub.Error
			if errMsg == "" {
				errMsg = "sub-model call failed"
			}
			service.Accounting.UpdateErrorRecordsWithUsage(subRecordIDs, 500, errMsg,
				sub.PromptTokens, sub.CompletionTokens, sub.TotalTokens, cost,
				meta.Method, meta.Endpoint+":"+sub.Kind, meta.Headers, meta.Body)
			continue
		}
		responsePayload := map[string]interface{}{
			"kind":  sub.Kind,
			"model": subProvider.Code + "/" + subModel.Code,
			"usage": gin.H{
				"prompt_tokens":     sub.PromptTokens,
				"completion_tokens": sub.CompletionTokens,
				"total_tokens":      sub.TotalTokens,
			},
		}
		// 视频等异步子任务的上游任务 ID 落入 response_body，供后续查询/恢复上游任务使用
		if sub.ExternalTaskID != "" {
			responsePayload["task_id"] = sub.ExternalTaskID
		}
		service.Accounting.UpdateSuccessRecords(subRecordIDs, 200, responsePayload,
			sub.PromptTokens, sub.CompletionTokens, sub.TotalTokens, cost,
			meta.Method, meta.Endpoint+":"+sub.Kind, meta.Headers, meta.Body, &subModel)
	}
	// 子调用用量与费用合并进主任务 usage_log（在主调用回填之后执行，增量累加）
	if recordIDs != nil && recordIDs.UsageLogID > 0 && subTotal > 0 {
		if err := repository.DB.Model(&models.UsageLog{}).Where("id = ?", recordIDs.UsageLogID).Updates(map[string]interface{}{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", subPrompt),
			"completion_tokens": gorm.Expr("completion_tokens + ?", subCompletion),
			"total_tokens":      gorm.Expr("total_tokens + ?", subTotal),
			"cost":              gorm.Expr("cost + ?", subCost),
		}).Error; err != nil {
			log.Error("[AgentTask] %s merge sub-model usage into main usage_log %d: %v", task.TaskID, recordIDs.UsageLogID, err)
		}
	}
	// 同步累加进任务表：任务详情 API（GET /v1/agent/tasks/:id）返回的 usage/cost 同样反映总消耗
	//（三个调用点均处于 agent_tasks 最终状态 UPDATE 之后，gorm.Expr 增量累加安全）
	if task.ID > 0 && subTotal > 0 {
		if err := repository.DB.Model(&models.AgentTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"prompt_tokens":     gorm.Expr("prompt_tokens + ?", subPrompt),
			"completion_tokens": gorm.Expr("completion_tokens + ?", subCompletion),
			"total_tokens":      gorm.Expr("total_tokens + ?", subTotal),
			"cost":              gorm.Expr("cost + ?", subCost),
		}).Error; err != nil {
			log.Error("[AgentTask] %s merge sub-model usage into agent_tasks %d: %v", task.TaskID, task.ID, err)
		}
	}
}

// classifyRunOutcome 依据运行结果判定最终状态与说明：
//   - 循环耗尽 + 无资产 → failed（白烧 token 无产出）
//   - 循环耗尽 + 有资产 → partial（资产可下载，缺最终总结）
//   - 正常结束但零字回答 + 无资产 → failed（空产出）
//   - 其余 → completed
func classifyRunOutcome(task *models.AgentTask, result *service.AgentRunResult) (status, errMsg string) {
	switch {
	case result.Exhausted && len(result.Assets) == 0:
		return models.AgentTaskStatusFailed,
			fmt.Sprintf("max_iterations (%d) exhausted: no final answer and no assets produced", task.MaxIterations)
	case result.Exhausted:
		return models.AgentTaskStatusPartial,
			fmt.Sprintf("max_iterations (%d) exhausted: assets produced but no final summary", task.MaxIterations)
	case result.FinalMessage == "" && len(result.Assets) == 0:
		return models.AgentTaskStatusFailed,
			"empty result: model returned no content and produced no assets"
	default:
		return models.AgentTaskStatusCompleted, ""
	}
}

// buildResponsePayload 组装计费/日志回传负载
func (h *AgentTaskHandler) buildResponsePayload(task *models.AgentTask, result *service.AgentRunResult, runErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"task_id": task.TaskID,
	}
	if result != nil {
		payload["iterations"] = result.Iterations
		payload["result"] = result.FinalMessage
		payload["skills_used"] = result.SkillsUsed
		payload["assets"] = result.Assets
		if len(result.MissingOutputs) > 0 {
			payload["missing_outputs"] = result.MissingOutputs
		}
		payload["usage"] = gin.H{
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.CompletionTokens,
			"total_tokens":      result.TotalTokens,
		}
	}
	if runErr != nil {
		payload["error"] = runErr.Error()
	}
	return payload
}

// markTaskFailed 置任务失败并保留已产生的 token 用量
func (h *AgentTaskHandler) markTaskFailed(task *models.AgentTask, result *service.AgentRunResult, errMsg string) {
	updates := map[string]interface{}{
		"status":        models.AgentTaskStatusFailed,
		"error_message": errMsg,
		"completed_at":  time.Now(),
	}
	if result != nil {
		updates["iterations"] = result.Iterations
		updates["prompt_tokens"] = result.PromptTokens
		updates["completion_tokens"] = result.CompletionTokens
		updates["total_tokens"] = result.TotalTokens
	}
	if err := repository.DB.Model(&models.AgentTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		log.Error("[AgentTask] %s mark failed: %v", task.TaskID, err)
	}
}

// taskReachedFinalState 查询任务是否已达终态（completed/partial/failed）；
// 查询失败按未达终态处理（走原有失败收尾路径）
func (h *AgentTaskHandler) taskReachedFinalState(taskID uint) bool {
	var current models.AgentTask
	if err := repository.DB.Select("status").First(&current, taskID).Error; err != nil {
		log.Error("[AgentTask] query final state for task id %d: %v", taskID, err)
		return false
	}
	switch current.Status {
	case models.AgentTaskStatusCompleted, models.AgentTaskStatusPartial, models.AgentTaskStatusFailed:
		return true
	default:
		return false
	}
}

// generateAgentTaskID 生成 "agt_" 前缀的唯一任务ID
func generateAgentTaskID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("agt_%s_%s", time.Now().Format("20060102150405"), hex.EncodeToString(buf)), nil
}

// nilIfZero 0 → nil（可选外键）
func nilIfZero(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// copyHeaders 复制请求头（goroutine 内使用，避免 gin.Context 回收竞争）
func copyHeaders(src http.Header) map[string][]string {
	dst := make(map[string][]string, len(src))
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}

// taskIterations 安全取轮次（result 可能为 nil）
func taskIterations(result *service.AgentRunResult) int {
	if result == nil {
		return 0
	}
	return result.Iterations
}

// launchAgentTask 落库 pending 任务 → 写路由 step（可选）→ 计费初始记录 → 异步执行 → 202 响应。
// Create（/v1/agent/tasks）与 Dispatch（/v1/agent/dispatch）共用的执行入口；
// endpoint 仅用于计费记录的 endpoint 标识；routed 非 nil 时落 skill_router step 并随响应回传
func (h *AgentTaskHandler) launchAgentTask(c *gin.Context, user *models.User, apiKeyID uint,
	provider *models.AiProvider, model *models.AiModel, skills []models.Skill,
	input string, maxIterations int, options map[string]interface{},
	routed *service.SkillRouteDecision, endpoint string, rawBody json.RawMessage) {

	// 生成任务ID并落库（pending）
	taskID, err := generateAgentTaskID()
	if err != nil {
		respondV1Error(c, http.StatusInternalServerError, "failed to generate task id", "api_error", "", nil)
		return
	}
	skillIDs := make([]uint, 0, len(skills))
	for _, s := range skills {
		skillIDs = append(skillIDs, s.ID)
	}
	task := models.AgentTask{
		TaskID:        taskID,
		UserID:        user.ID,
		ApiKeyID:      nilIfZero(apiKeyID),
		ModelID:       model.ID,
		ProviderID:    provider.ID,
		Task:          input,
		MaxIterations: maxIterations,
		Status:        models.AgentTaskStatusPending,
	}
	if blob, err := json.Marshal(skillIDs); err == nil {
		task.SkillIDs = models.JSON(blob)
	}
	if err := repository.DB.Create(&task).Error; err != nil {
		respondV1Error(c, http.StatusInternalServerError, "failed to create agent task", "api_error", "", nil)
		return
	}

	// 路由决策落 step trace（seq=0：Agent 循环首个工具 step 从 1 起，不冲突）
	if routed != nil {
		if blob, err := json.Marshal(routed); err == nil {
			step := models.AgentTaskStep{
				TaskID:        taskID,
				Seq:           0,
				Iteration:     0,
				Tool:          "skill_router",
				OK:            true,
				OutputExcerpt: string(blob),
			}
			if err := repository.DB.Create(&step).Error; err != nil {
				log.Error("[AgentTask] %s persist router step: %v", taskID, err)
			}
		}
	}

	// 计费初始记录（externalTaskID 关联 agent_tasks.task_id）
	recordIDs, err := service.Accounting.CreateInitialRecords(
		user.ID, model, provider, "openai", endpoint,
		apiKeyID, 0, taskID, "api",
	)
	if err != nil {
		log.Error("[AgentTask] Failed to create initial records for %s: %v", taskID, err)
		repository.DB.Model(&task).Update("status", models.AgentTaskStatusFailed)
		respondV1Error(c, http.StatusInternalServerError, "failed to create request records", "api_error", "server", "record_create_failed")
		return
	}

	// 请求快照（goroutine 内记账用）
	meta := agentRequestMeta{
		Method:   c.Request.Method,
		Endpoint: c.Request.URL.String(),
		Headers:  copyHeaders(c.Request.Header),
		Body:     rawBody,
		ApiKeyID: apiKeyID,
	}

	// 异步执行：goroutine 内建适配器 → RunTask → 更新任务状态与计费
	go h.runAgentTask(&task, model, provider, skills, recordIDs, options, meta)

	resp := gin.H{
		"task_id":  taskID,
		"status":   models.AgentTaskStatusPending,
		"poll_url": fmt.Sprintf("/v1/agent/tasks/%s", taskID),
	}
	if routed != nil {
		resp["routed"] = routed
	}
	c.JSON(http.StatusAccepted, resp)
}

// routeSkills LLM 语义路由：单技能目录直接命中不烧 token；否则用路由模型决策。
// 路由模型优先级：reqModel > AGENT_ROUTER_MODEL；失败时自响应错误并返回 (nil, false)
func (h *AgentTaskHandler) routeSkills(c *gin.Context, input, reqModel string, catalog []models.Skill) (*service.SkillRouteDecision, bool) {
	// 目录中只有一个技能时无需路由
	if len(catalog) == 1 {
		return &service.SkillRouteDecision{Skills: []string{catalog[0].Name}, Reason: "only enabled skill"}, true
	}

	routerModelStr := reqModel
	if routerModelStr == "" {
		routerModelStr = h.cfg.Agent.RouterModel
	}
	if routerModelStr == "" {
		respondV1Error(c, http.StatusBadRequest,
			"no routing model: specify model or configure AGENT_ROUTER_MODEL",
			"invalid_request_error", "model", "router_model_missing")
		return nil, false
	}
	provider, model, ok := parseV1Model(c, routerModelStr)
	if !ok {
		return nil, false
	}
	if model.Type != "text" {
		respondV1Error(c, http.StatusBadRequest,
			fmt.Sprintf("routing model %q (type=%s) is not a chat model", routerModelStr, model.Type),
			"invalid_request_error", "model", "model_not_supported")
		return nil, false
	}
	apiKey, err := ai.GetRandomApiKey(provider.ID)
	if err != nil {
		log.Error("[AgentTask] router: no active api keys for provider_id=%d: %v", provider.ID, err)
		respondV1Error(c, http.StatusBadGateway, "skill routing failed: no active provider api keys", "api_error", "", "router_failed")
		return nil, false
	}
	adapter, err := ai.NewAdapterFactory().CreateAdapterWithApiKey(provider, apiKey.ApiKey, model)
	if err != nil {
		log.Error("[AgentTask] router: adapter creation failed: %v", err)
		respondV1Error(c, http.StatusBadGateway, "skill routing failed: "+err.Error(), "api_error", "", "router_failed")
		return nil, false
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	decision, err := service.RouteSkills(ctx, input, catalog, adapter)
	if err != nil {
		log.Error("[AgentTask] router: routing failed: %v", err)
		respondV1Error(c, http.StatusBadGateway, "skill routing failed: "+err.Error(), "api_error", "", "router_failed")
		return nil, false
	}
	return decision, true
}

// loadSkillsByNames 按名称装载 enabled 技能；缺失/禁用/DB 错误时自响应错误并返回 nil
func loadSkillsByNames(c *gin.Context, names []string) []models.Skill {
	if len(names) == 0 {
		respondV1Error(c, http.StatusBadRequest, "skills is required (min 1)", "invalid_request_error", "skills", "skills_required")
		return nil
	}
	var skills []models.Skill
	if err := repository.DB.Where("name IN ? AND enabled = ?", names, true).Find(&skills).Error; err != nil {
		respondV1Error(c, http.StatusInternalServerError, "failed to load skills", "api_error", "", nil)
		return nil
	}
	if len(skills) < len(names) {
		found := make(map[string]bool, len(skills))
		for _, s := range skills {
			found[s.Name] = true
		}
		var missing []string
		for _, n := range names {
			if !found[n] {
				missing = append(missing, n)
			}
		}
		respondV1Error(c, http.StatusBadRequest,
			fmt.Sprintf("skills not found or disabled: %v", missing),
			"invalid_request_error", "skills", "skills_not_found")
		return nil
	}
	return skills
}

// uniqueNonEmpty 去重并去除空串，保持原顺序
func uniqueNonEmpty(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
