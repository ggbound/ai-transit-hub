package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
	log "github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// AgentRuntime 虚拟 Agent 运行时：平台自己充当宿主机（类比 Hypit 生态中的 Claude Code），
// 装载 SKILL.md 进 system → 循环调用模型 → 执行工具（读技能附件/写资产）→ 产出工作区资产。
type AgentRuntime struct {
	skillStorageRoot     string // 技能附件存储根（cfg.Storage.SkillPath）
	workspaceRoot        string // 任务工作区根（cfg.Storage.AgentTaskPath）
	allowWorkspaceScript bool   // 是否允许 execute_script 执行任务工作区内自建脚本（配置开关，沙箱可用性另行门控）

	pyenv *PyEnvManager // 任务级 Python 环境管理器（venv 模式；nil = system 模式，维持系统 python3 行为）

	sandboxes []Sandbox // 脚本执行沙箱后端链（优先级序；execute_script 逐个探测取首个可用后端）

	selector *ModelSelector // 按需模型选型器（generate_image/generate_video 等子模型调用使用）

	profileOnce sync.Once // 宿主机环境画像基础部分缓存（OS/解释器版本进程内只探测一次）
	profileBase string
}

// NewAgentRuntime 创建运行时；pyenv 传 nil 时为 system 模式（宿主系统 python3）。
// sandboxCfg 决定沙箱后端链的构成（auto 自适应或显式指定单一后端，见 buildSandboxChain）
func NewAgentRuntime(skillStorageRoot, workspaceRoot string, allowWorkspaceScript bool, pyenv *PyEnvManager, sandboxCfg config.SandboxConfig) *AgentRuntime {
	return &AgentRuntime{
		skillStorageRoot:     skillStorageRoot,
		workspaceRoot:        workspaceRoot,
		allowWorkspaceScript: allowWorkspaceScript,
		pyenv:                pyenv,
		sandboxes:            buildSandboxChain(sandboxCfg),
		selector:             NewModelSelector(),
	}
}

// AgentRunResult 单次任务运行结果
type AgentRunResult struct {
	FinalMessage     string
	Iterations       int
	Exhausted        bool // 循环耗尽（用满 max_iterations 仍未给出最终回答）
	SkillsUsed       []AgentSkillUsage
	Assets           []AgentAssetInfo
	MissingOutputs   []string // 技能声明为必需但工作区缺失的交付产物（completed 判定依据）
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	SubModelCalls    []SubModelCall // 主执行模型之外的按需子模型调用（generate_image/generate_video），计费回填用
}

// SubModelCall 一次子模型调用的计量信息（Agent 收尾后由 handler 落 usage_logs/api_test_requests）
type SubModelCall struct {
	Kind             string `json:"kind"` // image / video
	ProviderID       uint   `json:"provider_id"`
	ModelID          uint   `json:"model_id"`
	ProviderCode     string `json:"provider_code"`
	ModelCode        string `json:"model_code"`
	ExternalTaskID   string `json:"external_task_id,omitempty"` // 视频等异步任务的上游任务 ID（cgt-...），用于落库与超时恢复
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	OK               bool   `json:"ok"`
	Error            string `json:"error,omitempty"`
	HTTPStatus       int    `json:"http_status,omitempty"` // 失败时的上游 HTTP 状态码（任务内熔断判定依据）
}

// fuseStatusCodes 触发子模型熔断的上游状态码：认证失败/余额不足/无权限属于确定性失败，
// 同任务内重试同一模型必然再失败（且可能反复烧轮次）；参数类 400（如尺寸不合规）不熔断——
// 模型改参数重试即可成功，5xx/网络错误属瞬时也不熔断
var fuseStatusCodes = map[int]bool{
	http.StatusUnauthorized:    true,
	http.StatusPaymentRequired: true,
	http.StatusForbidden:       true,
}

// fusedSubModelCodes 从本任务已记录的子调用中提取应熔断的模型 code（指定 kind、失败且上游状态码确定性）
func fusedSubModelCodes(result *AgentRunResult, kind string) []string {
	var codes []string
	seen := map[string]bool{}
	for _, sc := range result.SubModelCalls {
		if sc.Kind == kind && !sc.OK && fuseStatusCodes[sc.HTTPStatus] && !seen[sc.ModelCode] {
			seen[sc.ModelCode] = true
			codes = append(codes, sc.ModelCode)
		}
	}
	return codes
}

// AgentSkillUsage 技能使用详情（渐进披露了哪些层）
type AgentSkillUsage struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	BodyLoaded  bool     `json:"body_loaded"`  // SKILL.md 正文是否已进入模型上下文（单技能直注或经 load_skill 装载）
	LoadedFiles []string `json:"loaded_files"` // 通过工具实际读取的包内文件
}

// AgentAssetInfo 工作区资产清单项
type AgentAssetInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mime    string `json:"mime"`
	RelPath string `json:"rel_path"` // 工作区内相对路径，下载端点用
}

// maxSkillFileReadSize 单个技能附件读入上下文的大小上限（防炸上下文）
const maxSkillFileReadSize = 64 * 1024

// execute_script 受控执行约束
const (
	// scriptTimeout 单脚本执行超时：需覆盖 inspect 类脚本（起 127.0.0.1 HTTP 服务 +
	// 驱动 headless Chrome 三端截图，宿主实测约 2 分钟）
	scriptTimeout = 120 * time.Second
	// maxScriptOutput stdout/stderr 回传上限（保留前段，超出追加 TRUNCATED 标记；
	// inspect 的判定结果输出在末尾但总量远小于此上限，不会被截掉）
	maxScriptOutput = 16 * 1024
	maxScriptArgs   = 16 // 脚本命令行参数数量上限
)

// chatRetryAttempts 模型调用失败重试：共 3 次尝试（1 次初始 + 2 次重试），退避 1s/2s
const chatRetryAttempts = 3

// defaultAgentMaxTokens 任务未指定 max_tokens 时注入的默认输出上限。
// 不注入时上游默认仅 ≈10k token（实测 doubao completion 10164 后 finish_reason=length），
// 长 tool_call 参数（如 write_asset 写完整 HTML）会被截成半截 JSON，模型随即原地重试烧光迭代。
// 32768 覆盖一屏 HTML 体量的工具调用；思考模型（如 doubao seed）的推理 token 与正文共享此配额。
const defaultAgentMaxTokens = 32768

// scriptInterpreters 脚本扩展名 → 解释器白名单（技能包脚本只允许经这两类解释器执行）
var scriptInterpreters = map[string]string{
	".py":  "python3",
	".py3": "python3",
	".js":  "node",
	".mjs": "node",
	".cjs": "node",
}

// assetNameRe 资产文件名白名单：字母/数字/点/下划线/连字符，不以点开头
var assetNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// maxAssetPathDepth 资产相对路径段数上限（嵌套产物防过深路径）
const maxAssetPathDepth = 8

// assetPathValid 校验工作区资产相对路径：按 / 分段，每段须符合 assetNameRe 且不超过段数上限。
// 支持扁平名（"index.html"）与嵌套相对路径（"productions/hero/BRIEF.md"）；
// 绝对路径、.. 穿越段、空段均不匹配 assetNameRe，天然被拒绝
func assetPathValid(relPath string) bool {
	if relPath == "" {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) > maxAssetPathDepth {
		return false
	}
	for _, p := range parts {
		if !assetNameRe.MatchString(p) {
			return false
		}
	}
	return true
}

// RunTask 执行虚拟 Agent 任务（异步 goroutine 中调用）
// task: 已落库的任务记录；skills: 已校验 enabled 的技能列表；adapter: 由调用方按 model/provider 创建
func (r *AgentRuntime) RunTask(ctx context.Context, task *models.AgentTask, skills []models.Skill, adapter ai.Adapter, options map[string]interface{}) (*AgentRunResult, error) {
	// 任务结束（含失败早退）即删除任务专属执行环境（venv + node_modules）：环境只为本次执行服务，
	// 终态后保留只会堆积同构目录；服务崩溃等漏删场景由 TTL 清理循环兜底（AGENT_PY_ENV_TTL，默认 7 天）
	if r.pyenv != nil {
		defer r.pyenv.RemoveTaskEnv(task.TaskID)
	}
	// 0. 依赖预检：解析技能 frontmatter 的依赖声明，环境缺失时快速失败（不烧 token）。
	// venv 模式下顺带完成本任务执行环境构建（Python venv + 任务级 node_modules，仅首次）
	if err := r.precheckSkillDependencies(ctx, task.TaskID, skills); err != nil {
		return nil, err
	}
	// 0.1 产物契约校验：技能声明的 outputs 文件名不合法属技能包缺陷，启动期报错（不烧 token）
	if err := validateOutputsContract(skills); err != nil {
		return nil, err
	}

	// 工作区：storage/agent_tasks/<task_id>/（统一绝对路径：环境画像、system prompt 与子进程 cwd 都依赖）
	wsDir := filepath.Join(r.workspaceRoot, task.TaskID)
	if abs, err := filepath.Abs(wsDir); err == nil {
		wsDir = abs
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// 1. 构建 system prompt：固定指令 + 宿主机环境画像 + 技能产物要求 + 技能注入
	//（渐进披露：单技能直注 SKILL.md 正文；多技能仅注入 metadata 清单，正文由模型 load_skill 按需装载）
	systemPrompt := r.buildSystemPrompt(skills, wsDir, task.MaxIterations)

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task.Task},
	}

	// 2. 工具定义（OpenAI function calling；生成类工具按技能 model_requirements 声明条件注册）
	chatOptions := map[string]interface{}{
		"tools":       r.toolDefinitions(skills),
		"tool_choice": "auto",
	}
	for k, v := range options {
		if k != "tools" && k != "tool_choice" { // 调用方自定义参数不覆盖内部工具配置
			chatOptions[k] = v
		}
	}
	// 调用方未指定 max_tokens 时注入默认输出上限，防止上游默认（≈10k token）截断长工具调用参数
	if _, ok := chatOptions["max_tokens"]; !ok {
		chatOptions["max_tokens"] = defaultAgentMaxTokens
	}

	result := &AgentRunResult{
		SkillsUsed: make([]AgentSkillUsage, 0, len(skills)),
		Assets:     []AgentAssetInfo{},
	}
	// 无论后续成败（含中途报错早退），已产生的技能使用与资产清单统一落库保留
	defer r.persistTaskArtifacts(task.ID, result)
	// 单技能任务 SKILL.md 正文直注 system prompt（免 load_skill 往返），使用记录同步标记正文已装载
	fullInjectBody := len(skills) == 1
	for _, s := range skills {
		result.SkillsUsed = append(result.SkillsUsed, AgentSkillUsage{ID: s.ID, Name: s.Name, BodyLoaded: fullInjectBody, LoadedFiles: []string{}})
	}

	// 3. Agent 循环
	maxIter := task.MaxIterations
	if maxIter <= 0 {
		maxIter = 8
	}
	var finalMsg string
	exhausted := true
	stepSeq := 0 // 工具调用步序（跨轮累计，trace 用）
	for iter := 1; iter <= maxIter; iter++ {
		result.Iterations = iter

		resp, err := chatWithRetry(ctx, adapter, messages, chatOptions)
		if err != nil {
			return result, fmt.Errorf("iteration %d: model call failed: %w", iter, err)
		}

		// 累计 token 用量
		p, cpt, t := adapter.ExtractTokenUsage(resp)
		result.PromptTokens += p
		result.CompletionTokens += cpt
		result.TotalTokens += t

		choice, err := firstChoiceMessage(resp)
		if err != nil {
			return result, fmt.Errorf("iteration %d: unexpected response shape: %w", iter, err)
		}

		// 解析 tool_calls
		var toolCalls []ai.ToolCall
		if raw, ok := choice["tool_calls"]; ok && raw != nil {
			blob, _ := json.Marshal(raw)
			if err := json.Unmarshal(blob, &toolCalls); err != nil {
				return result, fmt.Errorf("iteration %d: parse tool_calls: %w", iter, err)
			}
		}

		if len(toolCalls) == 0 {
			// 无工具调用 → 最终回答，循环结束
			finalMsg = contentToString(choice["content"])
			exhausted = false
			break
		}

		// 追加 assistant 消息（原样回传 tool_calls，Content 为空时置 nil）
		content := choice["content"]
		messages = append(messages, ai.Message{Role: "assistant", Content: content, ToolCalls: toolCalls})

		// 逐个执行工具并回传结果（每步记录 trace：seq 跨轮累计、耗时、成败、输出摘要）
		for _, call := range toolCalls {
			start := time.Now()
			output := r.executeTool(ctx, call, skills, wsDir, result)
			stepSeq++
			r.recordTaskStep(task.TaskID, stepSeq, iter, call, output, time.Since(start))
			messages = append(messages, ai.Message{
				Role:       "tool",
				Content:    output,
				ToolCallID: call.ID,
			})
		}

		// 预算将尽催促：剩余 ≤2 轮时注入一条提示，命令模型立即收尾——
		// 下一轮先补齐 write_asset 登记，再下一轮不调任何工具、输出纯文本最终总结，
		// 保证循环有 completed 出口（否则只能 partial/failed 收场）
		if remaining := maxIter - iter; remaining <= 2 {
			log.Info("[AgentTask] %s iteration %d/%d: inject budget-exhaustion nudge (remaining=%d)", task.TaskID, iter, maxIter, remaining)
			warning := fmt.Sprintf("[系统提示] 工具循环预算即将耗尽（剩余 %d 轮）。请立即收尾：", remaining)
			if remaining == 2 {
				warning += "本轮内用 write_asset 补齐/登记全部交付产物（这是最后的机会，之后不再有轮次执行工具）；"
			} else {
				warning += "不要再调用任何工具。"
			}
			warning += "下一轮必须输出纯文本最终总结（不携带任何 tool_calls），概述完成的工作与产物清单。"
			messages = append(messages, ai.Message{Role: "user", Content: warning})
		}

		// 每轮落库进度（轮询端点可见执行进展）
		r.updateProgress(task.ID, iter)
	}

	// 4. 汇总结果：资产清单由 write_asset 在循环中实时维护（manifest 单轨），
	// 收尾扫描工作区把脚本直写的产物补录进清单（同样可下载，补齐可见性），最后校验必需产物
	result.FinalMessage = finalMsg
	result.Exhausted = exhausted
	reconcileWorkspaceAssets(wsDir, result)
	result.MissingOutputs = checkMissingOutputs(wsDir, skills)
	return result, nil
}

// buildSystemPrompt 拼装 system：角色说明 + 工具用法 + 宿主机环境画像 + 技能产物要求 + 技能注入
// （渐进披露：单技能直注 SKILL.md 正文；多技能仅注入 metadata 清单，正文由模型 load_skill 按需装载）
func (r *AgentRuntime) buildSystemPrompt(skills []models.Skill, wsDir string, maxIterations int) string {
	var b strings.Builder
	b.WriteString("你是一个任务执行 Agent，运行在 ai-transit-hub 平台上。根据用户任务利用可用工具完成工作并产出资产文件。\n\n")
	// 轮次预算显式告知：否则模型无法规划收尾时机，常见后果是工作做完后无限自检打磨，
	// 烧满 max_iterations 仍未输出纯文本总结，任务只能落 partial（无法达 completed）
	if maxIterations > 0 {
		fmt.Fprintf(&b, "轮次预算：本次任务最多进行 %d 轮工具循环（每轮可发多个工具调用）。请合理规划进度：优先保证全部交付产物已通过 write_asset 登记写出，然后尽快以纯文本总结收尾；不要在产物齐全后继续无限自检或打磨。", maxIterations)
		b.WriteString("\n\n")
	}
	b.WriteString("可用工具：\n")
	b.WriteString("- load_skill(skill): 装载技能的完整 SKILL.md 说明。多技能任务的技能正文不随任务预注入，确定使用某技能后必须先调用它；单技能任务的正文已直接注入下方，无需调用。\n")
	b.WriteString("- read_skill_file(skill, path): 按技能名称读取技能包内的附件文件（references/scripts/assets）。仅在需要 SKILL.md 之外的更多细节时按需读取。\n")
	b.WriteString("- list_skill_files(skill): 枚举技能包内全部附件清单（路径/大小/类型）。当 SKILL.md 未列全可用文件时，先用它发现包内还有什么。\n")
	execScriptLine := "- execute_script(skill, path, args?): 受控执行技能包内自带脚本（仅支持 .py/.py3→python3、.js/.mjs/.cjs→node）。脚本在离线沙箱中运行：禁网，文件写入仅限任务工作区与系统临时目录；30 秒超时，输出截断。工作目录为任务工作区且跨多次调用持久（环境变量 SKILL_WORKSPACE 提供其路径，TMPDIR 指向其 tmp 子目录），脚本间可通过工作区文件交换中间结果。write_asset 是唯一交付通道，脚本直接写入工作区的文件视为中间产物，不计入交付清单。\n"
	if r.workspaceScriptAllowed() {
		execScriptLine = "- execute_script(skill?, path, args?): 受控执行脚本（仅支持 .py/.py3→python3、.js/.mjs/.cjs→node）。脚本来源二选一：传 skill 执行技能包内自带脚本（path 为包内相对路径）；省略 skill 执行你通过 write_asset 之外方式写入任务工作区的自建脚本（path 为工作区内相对路径，如 check.mjs）。脚本在离线沙箱中运行：禁网，文件写入仅限任务工作区与系统临时目录；30 秒超时，输出截断。工作目录为任务工作区且跨多次调用持久（环境变量 SKILL_WORKSPACE 提供其路径，TMPDIR 指向其 tmp 子目录），脚本间可通过工作区文件交换中间结果。write_asset 是唯一交付通道，脚本直接写入工作区的文件视为中间产物，不计入交付清单。\n"
	}
	b.WriteString(execScriptLine)
	b.WriteString("- write_asset(name, content): 将产出物写入任务工作区。name 支持相对子路径组织产物目录（如 productions/hero/BRIEF.md，父目录自动创建）。所有交付物必须通过该工具落盘，文本回复仅作总结；generate_image/generate_video 的产物已自动落盘登记，无需也不应再用本工具重写。\n")
	// 按需生成工具说明：与 toolDefinitions 的条件注册保持一致，仅声明了对应需求的技能才注入；
	// 附带平台侧已知的模型硬约束（预注入避免模型靠失败报错试出约束，每次试错都是计费调用）
	if agg := DeclaredModelRequirements(skills); agg.Image != nil {
		line := "- generate_image(prompt, asset_name?, size?, n?): 调用平台图像模型生成图片并自动保存为任务资产（如海报、配图、视觉稿）。任务需要图片产物时优先使用它而非手写占位内容。\n"
		if hints := subModelConstraintHints("image"); hints != "" {
			line += "  平台图像模型已知硬约束（违反会被上游直接拒绝，浪费一次计费调用）：\n" + hints + "\n"
		}
		b.WriteString(line)
	}
	if agg := DeclaredModelRequirements(skills); agg.Video != nil {
		line := "- generate_video(prompt, asset_name?, duration?, ratio?, generate_audio?): 调用平台视频模型生成视频并自动保存为任务资产（异步生成，可能耗时数分钟）。需要旁白配音时传 generate_audio=true（是否生效取决于所选模型）。\n"
		if hints := subModelConstraintHints("video"); hints != "" {
			line += "  平台视频模型已知硬约束（违反会被上游直接拒绝，浪费一次计费调用）：\n" + hints + "\n"
		}
		b.WriteString(line)
	}
	b.WriteString("\n")

	// 宿主机环境画像：解释器版本/OS/架构/工作区路径一次说清，模型无需烧轮次试探环境
	b.WriteString(r.hostEnvironmentProfile(wsDir))

	// 技能声明的必需产物：把要求显式交给模型，缺失会导致任务降级为 partial
	if req := requiredOutputsSummary(skills); req != "" {
		b.WriteString("\n技能声明的必需交付产物（最终必须以这些文件名通过 write_asset 写出）：\n")
		b.WriteString(req)
		b.WriteString("\n")
	}

	if len(skills) > 0 {
		if len(skills) == 1 {
			// 单技能：SKILL.md 正文直注（渐进披露第 2 层），省一轮 load_skill 工具往返
			b.WriteString("已装载技能：\n")
			s := skills[0]
			fmt.Fprintf(&b, "\n<skill name=\"%s\" version=\"%s\">\n%s\n</skill>\n", s.Name, s.Version, s.Content)
		} else {
			// 多技能：仅注入名称与简介（渐进披露第 1 层），正文由模型经 load_skill 按需装载（第 2 层），
			// 避免全部正文挤占 system prompt 造成 token 膨胀与跨技能指令互扰
			b.WriteString("可用技能（仅列出名称与简介，完整说明未注入；确定使用某技能后，必须先调用 load_skill 装载其 SKILL.md 再按说明工作）：\n")
			for _, s := range skills {
				fmt.Fprintf(&b, "- %s（v%s）：%s\n", s.Name, s.Version, s.Description)
			}
		}
	}

	b.WriteString("\n执行要求：\n")
	execScriptRule := "3. 技能包自带脚本可用 execute_script 受控执行（脚本工作目录为任务工作区，可用工作区文件在脚本间交换中间结果）"
	if r.workspaceScriptAllowed() {
		execScriptRule += "；也可省略 skill 执行任务工作区内自建脚本（如需校验产物、批量处理数据时先写脚本再执行）"
	}
	fmt.Fprintf(&b, "1. 先理解任务与技能说明（多技能任务先用 load_skill 装载要用的技能）；2. 需要细节时用 read_skill_file 按需读取，不确定包内有哪些文件时用 list_skill_files 枚举；%s；4. 交付物一律通过 write_asset 写出（技能声明了产物名时必须用那些文件名），脚本直写文件视为中间产物；5. 完成后输出简短总结。\n", execScriptRule)
	return b.String()
}

// toolDefinitions 返回虚拟 Agent 的工具 JSON Schema（OpenAI function calling 格式）。
// generate_image/generate_video 按需注册：仅当装载技能声明了对应 model_requirements 时开放，
// 防止无关任务误触发昂贵的生成类子模型调用
func (r *AgentRuntime) toolDefinitions(skills []models.Skill) []map[string]interface{} {
	// 工作区脚本能力开启时，execute_script 的描述与参数约束同步放宽（path 支持工作区相对路径，skill 变为可选）
	execScriptDesc := "受控执行技能包内自带脚本：按扩展名选择解释器（.py/.py3→python3，.js/.mjs/.cjs→node）。离线沙箱运行（禁网，写入仅限任务工作区与系统临时目录），30 秒超时，stdout/stderr 截断。工作目录为任务工作区且跨调用持久（SKILL_WORKSPACE 环境变量），脚本间可通过工作区文件交换中间结果"
	execScriptPathDesc := "脚本在技能包内的相对路径，如 scripts/build_index.py"
	execScriptRequired := []string{"skill", "path"}
	if r.workspaceScriptAllowed() {
		execScriptDesc = "受控执行脚本：按扩展名选择解释器（.py/.py3→python3，.js/.mjs/.cjs→node）。脚本来源二选一：传 skill 执行技能包内自带脚本（path 为包内相对路径）；省略 skill 执行任务工作区内自建脚本（path 为工作区内相对路径）。离线沙箱运行（禁网，写入仅限任务工作区与系统临时目录），30 秒超时，stdout/stderr 截断。工作目录为任务工作区且跨调用持久（SKILL_WORKSPACE 环境变量），脚本间可通过工作区文件交换中间结果"
		execScriptPathDesc = "脚本路径：传 skill 时为技能包内相对路径（如 scripts/build_index.py）；省略 skill 时为任务工作区内相对路径（如 check.mjs）"
		execScriptRequired = []string{"path"}
	}
	tools := []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "load_skill",
				"description": "装载指定技能的完整说明文档（SKILL.md 正文）。多技能任务中 system 仅列出技能名称与简介，确定使用某技能后必须先调用本工具装载其正文再开始工作；单技能任务的技能正文已在 system 中给出，无需调用",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"skill": map[string]interface{}{"type": "string", "description": "技能名称（system 中技能清单列出的名称）"},
					},
					"required": []string{"skill"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "read_skill_file",
				"description": "读取技能包内的附件文件内容（references/scripts/assets 等；多技能任务先用 load_skill 装载技能主文档，再用本工具按需读取其引用的附件）",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"skill": map[string]interface{}{"type": "string", "description": "技能名称（system 中技能清单列出的名称）"},
						"path":  map[string]interface{}{"type": "string", "description": "包内相对路径，如 references/api.md"},
					},
					"required": []string{"skill", "path"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "list_skill_files",
				"description": "枚举指定技能包内全部附件文件清单（路径/大小/MIME），用于发现 SKILL.md 未提及的可用文件",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"skill": map[string]interface{}{"type": "string", "description": "技能名称（system 中技能清单列出的名称）"},
					},
					"required": []string{"skill"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "execute_script",
				"description": execScriptDesc,
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"skill": map[string]interface{}{"type": "string", "description": "技能名称；执行任务工作区内自建脚本时省略"},
						"path":  map[string]interface{}{"type": "string", "description": execScriptPathDesc},
						"args":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "脚本命令行参数（可选）"},
					},
					"required": execScriptRequired,
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "write_asset",
				"description": "将产出物写入任务工作区（最终交付物必须落盘）。generate_image/generate_video 生成的图片/视频已自动落盘并登记为资产，不要再用本工具重写这些文件",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":    map[string]interface{}{"type": "string", "description": "资产相对路径，支持子目录组织（如 summary.md 或 productions/hero/BRIEF.md）"},
						"content": map[string]interface{}{"type": "string", "description": "文件文本内容"},
					},
					"required": []string{"name", "content"},
				},
			},
		},
	}

	// 按需生成工具：技能声明了 image/video model_requirements 才注册；
	// description 附带平台侧已知模型硬约束（工具 schema 与 system prompt 双通道注入）
	agg := DeclaredModelRequirements(skills)
	if agg.Image != nil {
		imgDesc := "调用平台图像模型生成图片并保存为任务资产。用于任务需要产出图片类产物时（如海报、配图、视觉稿）"
		if hints := subModelConstraintHints("image"); hints != "" {
			imgDesc += "。已知硬约束（违反会被上游直接拒绝）：\n" + hints
		}
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "generate_image",
				"description": imgDesc,
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt":     map[string]interface{}{"type": "string", "description": "图像生成提示词（描述画面主体、风格、构图、光影等）"},
						"asset_name": map[string]interface{}{"type": "string", "description": "保存的资产文件名（如 cover.png），可省略自动命名"},
						"size":       map[string]interface{}{"type": "string", "description": "图片尺寸，如 1024x1024、1280x720（可选，是否生效取决于所选模型）"},
						"n":          map[string]interface{}{"type": "integer", "description": "生成张数（可选，默认 1）"},
					},
					"required": []string{"prompt"},
				},
			},
		})
	}
	if agg.Video != nil {
		vidDesc := "调用平台视频模型生成视频并保存为任务资产。异步生成，可能耗时数分钟，用于任务需要产出视频类产物时"
		if hints := subModelConstraintHints("video"); hints != "" {
			vidDesc += "。已知硬约束（违反会被上游直接拒绝）：\n" + hints
		}
		tools = append(tools, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "generate_video",
				"description": vidDesc,
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"prompt":         map[string]interface{}{"type": "string", "description": "视频生成提示词（描述镜头运动、主体动作、场景氛围等）"},
						"asset_name":     map[string]interface{}{"type": "string", "description": "保存的资产文件名（如 clip.mp4），可省略自动命名"},
						"duration":       map[string]interface{}{"type": "number", "description": "视频时长秒数（可选，默认由模型决定）"},
						"ratio":          map[string]interface{}{"type": "string", "description": "画面比例（可选，如 16:9、9:16、1:1）"},
						"generate_audio": map[string]interface{}{"type": "boolean", "description": "是否生成音轨/配音（可选，不传由模型默认决定）。任务需要旁白配音时传 true，明确要求无声画面时传 false；配音内容与口型以模型实际能力为准"},
					},
					"required": []string{"prompt"},
				},
			},
		})
	}
	return tools
}

// executeTool 执行单个工具调用，返回回传给模型的文本结果（工具失败不中断循环，以错误文本回传）
func (r *AgentRuntime) executeTool(ctx context.Context, call ai.ToolCall, skills []models.Skill, wsDir string, result *AgentRunResult) string {
	var args struct {
		Skill     string   `json:"skill"`
		Path      string   `json:"path"`
		Name      string   `json:"name"`
		Content   string   `json:"content"`
		Args      []string `json:"args"`
		Prompt    string   `json:"prompt"`
		AssetName string   `json:"asset_name"`
		Size      string   `json:"size"`
		N         int      `json:"n"`
		Duration  float64  `json:"duration"`
		Ratio     string   `json:"ratio"`
		// GenerateAudio 指针形式区分"未传"（nil，保持上游模型默认行为）与显式 true/false
		GenerateAudio *bool `json:"generate_audio"`
	}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		// "unexpected end of JSON input"（Go json 包对半截输入的固定文本）大概率意味着本轮输出
		// 被 max_tokens 截断。裸错误会让模型原地重试同样的大输出，必须给出可行动的归因与出路。
		if strings.Contains(err.Error(), "unexpected end of JSON input") {
			return "TOOL ERROR: invalid arguments: 参数 JSON 不完整（在结束前被截断）——大概率是本轮输出超出 max_tokens 上限，原样重试会再次截断。请改为：① 将内容拆分成多次小段写入（每次 content 控制在 3000 字符以内，先写首段、后续追加到同一文件的不同分片名，最后用 execute_script 合并）；或 ② 直接用 execute_script 程序化生成大文件"
		}
		return fmt.Sprintf("TOOL ERROR: invalid arguments: %v", err)
	}

	switch call.Function.Name {
	case "load_skill":
		// 渐进披露第 2 层：按名称装载技能 SKILL.md 正文（不触 DB/文件系统，内容已随技能载入内存）
		skill, err := findLoadedSkill(skills, args.Skill)
		if err != nil {
			return fmt.Sprintf("TOOL ERROR: %v", err)
		}
		markSkillBodyLoaded(result, skill.ID)
		if len(skill.Content) > maxSkillFileReadSize {
			return fmt.Sprintf("已装载技能 %q（v%s）的 SKILL.md：\n\n%s\n\n[TRUNCATED: content size %d bytes, showing first %d bytes]",
				skill.Name, skill.Version, skill.Content[:maxSkillFileReadSize], len(skill.Content), maxSkillFileReadSize)
		}
		return fmt.Sprintf("已装载技能 %q（v%s）的 SKILL.md：\n\n%s", skill.Name, skill.Version, skill.Content)

	case "read_skill_file":
		// 在已装载技能中按名称查找（SKILL.md frontmatter name）
		skill, err := findLoadedSkill(skills, args.Skill)
		if err != nil {
			return fmt.Sprintf("TOOL ERROR: %v", err)
		}
		// 防路径穿越读取（复用技能包安全读取）
		target, err := SkillFilePath(r.skillStorageRoot, skill.StoragePath, args.Path)
		if err != nil {
			return fmt.Sprintf("TOOL ERROR: %v", err)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return fmt.Sprintf("TOOL ERROR: read failed: %v", err)
		}
		// 记录渐进披露行为（内存态，最终统一落库）
		markSkillFileLoaded(result, skill.ID, args.Path)
		if len(data) > maxSkillFileReadSize {
			return fmt.Sprintf("%s\n\n[TRUNCATED: file size %d bytes, showing first %d bytes]",
				string(data[:maxSkillFileReadSize]), len(data), maxSkillFileReadSize)
		}
		return string(data)

	case "list_skill_files":
		skill, err := findLoadedSkill(skills, args.Skill)
		if err != nil {
			return fmt.Sprintf("TOOL ERROR: %v", err)
		}
		var files []models.SkillFile
		if err := repository.DB.Where("skill_id = ?", skill.ID).Order("path").Find(&files).Error; err != nil {
			return fmt.Sprintf("TOOL ERROR: list files failed: %v", err)
		}
		if len(files) == 0 {
			return fmt.Sprintf("skill %q has no attached files (SKILL.md only)", skill.Name)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "files in skill %q (path | size | mime):\n", skill.Name)
		for _, f := range files {
			mm := f.MimeType
			if mm == "" {
				mm = "-"
			}
			fmt.Fprintf(&b, "%s | %d bytes | %s\n", f.Path, f.Size, mm)
		}
		return b.String()

	case "execute_script":
		if len(args.Args) > maxScriptArgs {
			return fmt.Sprintf("TOOL ERROR: too many script args (max %d)", maxScriptArgs)
		}
		// 脚本来源二选一：传 skill → 技能包内脚本（防路径穿越）；不传 skill → 任务工作区自建脚本（开关+沙箱双门控）
		var loadedSkill *models.Skill
		var scriptPath string
		var skillRoot string // 技能包根绝对路径（SKILL_ROOT 注入；工作区自建脚本为空）
		if args.Skill != "" {
			skill, err := findLoadedSkill(skills, args.Skill)
			if err != nil {
				return fmt.Sprintf("TOOL ERROR: %v", err)
			}
			// 脚本必须位于技能包内（防路径穿越/防执行工作区任意文件）
			p, err := SkillFilePath(r.skillStorageRoot, skill.StoragePath, args.Path)
			if err != nil {
				return fmt.Sprintf("TOOL ERROR: %v", err)
			}
			loadedSkill = skill
			scriptPath = p
			skillRoot = skillPackageRoot(r.skillStorageRoot, skill.StoragePath)
		} else {
			if !r.workspaceScriptAllowed() {
				return "TOOL ERROR: workspace script execution is not enabled on this host (requires AGENT_ALLOW_WORKSPACE_SCRIPT=true and an available sandbox)"
			}
			// 脚本必须位于任务工作区内（防路径穿越）
			p, err := workspaceFilePath(wsDir, args.Path)
			if err != nil {
				// 裸错误曾让模型误以为可以内联执行（path=".")，随即转入 4 轮文件系统考古找 skill 真实路径。
				// 必须给出可行动出路：先 write_asset 落盘脚本，再按文件名执行；技能自带脚本走 skill+path 通道。
				return "TOOL ERROR: invalid workspace script path: execute_script 不支持内联代码（path 不能为 \".\"、空或目录）。请先用 write_asset 把脚本写入工作区，再以 {\"path\": \"<文件名>\"} 执行；技能自带脚本则传 {\"skill\": \"<名称>\", \"path\": \"scripts/<文件>\"}，无需知道其在宿主机上的真实路径"
			}
			scriptPath = p
		}
		if info, err := os.Stat(scriptPath); err != nil || info.IsDir() {
			return fmt.Sprintf("TOOL ERROR: script not found: %s", args.Path)
		}
		interpreter, ok := scriptInterpreters[strings.ToLower(filepath.Ext(scriptPath))]
		if !ok {
			return "TOOL ERROR: unsupported script type (allowed: .py/.py3/.js/.mjs/.cjs)"
		}
		venvDir := ""    // venv 模式下 python 脚本绑定的任务虚拟环境（沙箱 env/只读挂载用）
		nodeEnvDir := "" // 任务 Node 环境目录（node_modules 父目录；npm: 依赖的 NODE_PATH/PATH 注入用，无则为空）
		taskID := filepath.Base(wsDir)
		if interpreter == "python3" && r.pyenv != nil {
			// venv 模式：解释器切换为任务专属 venv（precheck 已构建，此处幂等兜底秒级返回）。
			// 构建失败（precheck 成功后环境突变等罕见场景）降级系统 python3 继续执行，不中断任务
			if dir, err := r.pyenv.EnsureTaskVenv(ctx, taskID, nil); err != nil {
				logger.Warn("execute_script: task venv unavailable (%v), falling back to system python3", err)
			} else {
				venvDir = dir
				interpreter = r.pyenv.TaskPythonPath(taskID)
			}
		}
		if r.pyenv != nil {
			// 任务 Node 环境：precheck 已按 npm: 依赖声明构建，此处仅探测不重建——
			// node_modules 存在才注入，避免为无 npm 依赖的任务创建空目录
			if _, err := os.Stat(r.pyenv.TaskNodeModulesDir(taskID)); err == nil {
				nodeEnvDir = r.pyenv.TaskNodeEnvDir(taskID)
			}
		}
		if loadedSkill != nil {
			// 执行技能包脚本也视为读取了该文件（渐进披露行为记录）
			markSkillFileLoaded(result, loadedSkill.ID, args.Path)
		}
		// 沙箱后端自适应选择（与宿主机环境画像共用逻辑，保证画像与实况一致）：
		// venv 模式过滤掉不兼容宿主侧任务环境的后端，全部被过滤时降级宿主环境重选；
		// plain 兜底直跑时记录日志提醒运维环境缺沙箱。
		// 仅 AGENT_SANDBOX_ALLOW_PLAIN=false 或显式锁定后端不可用时无后端可选 → fail-closed
		sb, envDropped := r.selectSandboxAdaptive(venvDir, nodeEnvDir)
		if sb == nil {
			return "TOOL ERROR: no script execution backend available on this host (check AGENT_SANDBOX_BACKEND / AGENT_SANDBOX_ALLOW_PLAIN)"
		}
		if envDropped {
			logger.Warn("execute_script: task environment incompatible with available sandbox backends, falling back to host environment")
			if venvDir != "" {
				interpreter = "python3"
				venvDir = ""
			}
			nodeEnvDir = ""
		} else if sb.Name() == "plain" {
			logger.Warn("execute_script: no sandbox backend available, script runs WITHOUT network/write isolation on host")
		}
		return sb.Exec(ctx, ExecSpec{
			Interpreter: interpreter,
			Script:      scriptPath,
			Args:        args.Args,
			WorkDir:     wsDir,
			SkillRoot:   skillRoot,
			VenvDir:     venvDir,
			NodeEnvDir:  nodeEnvDir,
		})

	case "write_asset":
		if !assetPathValid(args.Name) {
			return "TOOL ERROR: invalid asset path (allowed: letters, digits, dot, underscore, hyphen; up to 8 path segments separated by /; must not start with a dot)"
		}
		target := filepath.Join(wsDir, filepath.FromSlash(args.Name))
		// 防误清空/误覆盖：目标已是非空文件时，空 content 或图片/视频类产物一律拒绝。
		// 典型误操作：generate_image 已落盘 poster.png，模型为满足"交付物必须 write_asset"
		// 对同名资产补写空 content，把成品清成 0 字节。拒绝并说明现状，模型可自我纠正
		if info, err := os.Stat(target); err == nil && info.Size() > 0 {
			extMime := mime.TypeByExtension(filepath.Ext(args.Name))
			generated := strings.HasPrefix(extMime, "image/") || strings.HasPrefix(extMime, "video/")
			if args.Content == "" || generated {
				kind := "file"
				if generated {
					kind = "generated asset (from generate_image/generate_video, already saved and registered; do not rewrite it)"
				}
				return fmt.Sprintf("TOOL ERROR: refusing to overwrite existing %s %q (%d bytes)%s",
					kind, args.Name, info.Size(),
					map[bool]string{true: "", false: " with empty content (would wipe the file)"}[generated])
			}
		}
		// 嵌套路径自动创建父目录（如 productions/hero/）
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Sprintf("TOOL ERROR: write failed: %v", err)
		}
		if err := os.WriteFile(target, []byte(args.Content), 0o644); err != nil {
			return fmt.Sprintf("TOOL ERROR: write failed: %v", err)
		}
		// manifest 单轨：write_asset 是唯一交付通道，写成功即登记/更新资产清单（同名重写视为更新）
		size := int64(len(args.Content))
		if info, err := os.Stat(target); err == nil {
			size = info.Size()
		}
		upsertAsset(result, AgentAssetInfo{
			Name:    args.Name,
			Size:    size,
			Mime:    mime.TypeByExtension(filepath.Ext(args.Name)),
			RelPath: args.Name,
		})
		return fmt.Sprintf("OK: asset %s saved (%d bytes)", args.Name, size)

	case "generate_image":
		agg := DeclaredModelRequirements(skills)
		if agg.Image == nil {
			return "TOOL ERROR: generate_image is not enabled (no loaded skill declares model_requirements.image)"
		}
		return r.runGenerateImage(ctx, *agg.Image, args.Prompt, args.AssetName, args.Size, args.N, wsDir, result)

	case "generate_video":
		agg := DeclaredModelRequirements(skills)
		if agg.Video == nil {
			return "TOOL ERROR: generate_video is not enabled (no loaded skill declares model_requirements.video)"
		}
		return r.runGenerateVideo(ctx, *agg.Video, args.Prompt, args.AssetName, args.Duration, args.Ratio, args.GenerateAudio, wsDir, result)

	default:
		return fmt.Sprintf("TOOL ERROR: unknown tool %q", call.Function.Name)
	}
}

// runGenerateImage 按需图像生成：ModelSelector 选型 → 适配器 ImagesGenerations →
// 提取图片（data[].b64_json 解码或 data[].url 下载）→ 落盘任务资产 → 记录子模型调用计量
func (r *AgentRuntime) runGenerateImage(ctx context.Context, req ModelRequirement, prompt, assetName, size string, n int, wsDir string, result *AgentRunResult) string {
	if strings.TrimSpace(prompt) == "" {
		return "TOOL ERROR: prompt is required"
	}
	// 任务内熔断：排除本次任务已确定性失败（401/402/403）的图像模型，选型改走次优
	req.Exclude = fusedSubModelCodes(result, "image")
	adapter, sel, err := r.selector.CreateAdapter(req)
	if err != nil {
		return fmt.Sprintf("TOOL ERROR: %v", err)
	}
	sub := SubModelCall{
		Kind:         "image",
		ProviderID:   sel.Provider.ID,
		ModelID:      sel.Model.ID,
		ProviderCode: sel.Provider.Code,
		ModelCode:    sel.Model.Code,
	}
	// 全路径统一记账（成败都记录，handler 据此回填计费）
	defer func() { result.SubModelCalls = append(result.SubModelCalls, sub) }()

	options := map[string]interface{}{}
	if size != "" {
		options["size"] = size
	}
	if n > 1 {
		options["n"] = n
	}

	cctx, cancel := context.WithTimeout(ctx, imageCallTimeout)
	defer cancel()
	resp, err := adapter.ImagesGenerations(cctx, prompt, options)
	if err != nil {
		sub.Error = err.Error()
		if ue, ok := ai.AsUpstreamError(err); ok {
			sub.HTTPStatus = ue.StatusCode
		}
		return fmt.Sprintf("TOOL ERROR: image generation failed: %v", err)
	}
	sub.PromptTokens, sub.CompletionTokens, sub.TotalTokens = adapter.ExtractTokenUsage(resp)

	// OpenAI 兼容格式：data[].b64_json（直接解码）或 data[].url（平台侧下载）
	items, _ := resp["data"].([]interface{})
	if len(items) == 0 {
		sub.Error = "no image in response"
		return "TOOL ERROR: image generation returned no data"
	}
	if assetName == "" {
		assetName = fmt.Sprintf("image-%d.png", time.Now().UnixNano())
	}
	var saved []string
	for i, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		var data []byte
		if b64, _ := m["b64_json"].(string); b64 != "" {
			if data, err = base64.StdEncoding.DecodeString(b64); err != nil {
				sub.Error = fmt.Sprintf("decode image %d: %v", i+1, err)
				return fmt.Sprintf("TOOL ERROR: %v", err)
			}
		} else if u, _ := m["url"].(string); u != "" {
			if data, err = downloadGeneratedFile(u, maxGeneratedImageSz); err != nil {
				sub.Error = fmt.Sprintf("download image %d: %v", i+1, err)
				return fmt.Sprintf("TOOL ERROR: %v", err)
			}
		} else {
			continue
		}
		name := multiAssetName(assetName, i, len(items), ".png")
		if _, err := saveWorkspaceAsset(wsDir, result, name, data); err != nil {
			sub.Error = err.Error()
			return fmt.Sprintf("TOOL ERROR: save asset %s failed: %v", name, err)
		}
		saved = append(saved, name)
	}
	if len(saved) == 0 {
		sub.Error = "no usable image in response"
		return "TOOL ERROR: image generation response contains no image (expected data[].b64_json or data[].url)"
	}
	sub.OK = true
	return fmt.Sprintf("OK: %d image(s) generated and saved: %s", len(saved), strings.Join(saved, ", "))
}

// runGenerateVideo 按需视频生成：ModelSelector 选型 → 适配器 VideosGenerations 提交 →
// 同步响应直接取 URL 或按异步任务轮询 GetVideoTask → 下载 mp4 → 落盘任务资产 → 记录子模型调用计量
func (r *AgentRuntime) runGenerateVideo(ctx context.Context, req ModelRequirement, prompt, assetName string, duration float64, ratio string, generateAudio *bool, wsDir string, result *AgentRunResult) string {
	if strings.TrimSpace(prompt) == "" {
		return "TOOL ERROR: prompt is required"
	}
	// 任务内熔断：排除本次任务已确定性失败（401/402/403）的视频模型，选型改走次优
	req.Exclude = fusedSubModelCodes(result, "video")
	adapter, sel, err := r.selector.CreateAdapter(req)
	if err != nil {
		return fmt.Sprintf("TOOL ERROR: %v", err)
	}
	sub := SubModelCall{
		Kind:         "video",
		ProviderID:   sel.Provider.ID,
		ModelID:      sel.Model.ID,
		ProviderCode: sel.Provider.Code,
		ModelCode:    sel.Model.Code,
	}
	defer func() { result.SubModelCalls = append(result.SubModelCalls, sub) }()

	options := map[string]interface{}{}
	if duration > 0 {
		options["duration"] = duration
	}
	if ratio != "" {
		options["ratio"] = ratio
	}
	// 音轨生成开关：Agent 显式传入时透传给适配器（doubao 顶层 generate_audio），未传保持上游默认
	if generateAudio != nil {
		options["generate_audio"] = *generateAudio
	}

	// 提交生成任务（doubao 等异步模型此处返回任务 id）
	sctx, scancel := context.WithTimeout(ctx, videoSubmitTimeout)
	defer scancel()
	resp, err := adapter.VideosGenerations(sctx, prompt, options)
	if err != nil {
		sub.Error = err.Error()
		// 仅提交失败记录状态码供熔断；轮询查询失败不代表提交模型坏，不熔断
		if ue, ok := ai.AsUpstreamError(err); ok {
			sub.HTTPStatus = ue.StatusCode
		}
		return fmt.Sprintf("TOOL ERROR: video generation submit failed: %v", err)
	}
	sub.PromptTokens, sub.CompletionTokens, sub.TotalTokens = adapter.ExtractTokenUsage(resp)

	if assetName == "" {
		assetName = fmt.Sprintf("video-%d.mp4", time.Now().UnixNano())
	}

	// 同步返回（部分模型直接给出视频 URL）与异步任务（先提交后轮询）双形态兼容
	if url := extractVideoURL(resp); url != "" {
		return r.fetchAndSaveGeneratedVideo(url, assetName, wsDir, result, &sub)
	}
	taskID := extractVideoTaskID(resp)
	if taskID == "" {
		sub.Error = "no video url or task id in response"
		return "TOOL ERROR: video generation returned neither video url nor task id"
	}
	// 记录上游任务 ID：落账时写入子调用记录，超时/失败后可从 DB 恢复任务（勿重复提交）
	sub.ExternalTaskID = taskID

	// 轮询任务状态：5s 间隔，10min 上限；轮询响应带 usage 时补充计量。
	// 单次查询失败（网络抖动/上游闪断）不放弃——云端任务已提交且在计费，
	// 连续失败达到 videoPollMaxQueryFailures 次才判失败；超时放弃时提示云端任务可能仍在跑，勿重复提交
	deadline := time.Now().Add(videoPollTimeout)
	queryFailures := 0
	for {
		if time.Now().After(deadline) {
			sub.Error = fmt.Sprintf("video task %s polling timeout after %s; cloud task %s may still be running", taskID, videoPollTimeout, taskID)
			return fmt.Sprintf("TOOL ERROR: video task %s not finished within %s; cloud task %s may still be running, do NOT resubmit, recover via task id", taskID, videoPollTimeout, taskID)
		}
		time.Sleep(videoPollInterval)
		pctx, pcancel := context.WithTimeout(ctx, 30*time.Second)
		taskResp, err := adapter.GetVideoTask(pctx, taskID)
		pcancel()
		if err != nil {
			queryFailures++
			logger.Error("agent video task %s query failure %d/%d: %v", taskID, queryFailures, videoPollMaxQueryFailures, err)
			if queryFailures >= videoPollMaxQueryFailures {
				sub.Error = fmt.Sprintf("video task %s query failed %d consecutive times: %v; cloud task %s may still be running", taskID, queryFailures, err, taskID)
				return fmt.Sprintf("TOOL ERROR: video task %s query failed %d consecutive times, giving up polling: %v; cloud task %s may still be running, do NOT resubmit, recover via task id", taskID, queryFailures, err, taskID)
			}
			continue
		}
		queryFailures = 0
		if p, c, t := adapter.ExtractTokenUsage(taskResp); t > sub.TotalTokens {
			sub.PromptTokens, sub.CompletionTokens, sub.TotalTokens = p, c, t
		}
		if url := extractVideoURL(taskResp); url != "" {
			return r.fetchAndSaveGeneratedVideo(url, assetName, wsDir, result, &sub)
		}
		if status := videoTaskStatus(taskResp); strings.Contains(strings.ToLower(status), "fail") {
			sub.Error = "video task failed: " + status
			return fmt.Sprintf("TOOL ERROR: video task %s failed: %s", taskID, status)
		}
	}
}

// fetchAndSaveGeneratedVideo 下载生成的视频并落盘任务资产
func (r *AgentRuntime) fetchAndSaveGeneratedVideo(url, assetName, wsDir string, result *AgentRunResult, sub *SubModelCall) string {
	data, err := downloadGeneratedFile(url, maxGeneratedVideoSz)
	if err != nil {
		sub.Error = err.Error()
		return fmt.Sprintf("TOOL ERROR: download video failed: %v", err)
	}
	name := strings.TrimSpace(assetName)
	if _, err := saveWorkspaceAsset(wsDir, result, name, data); err != nil {
		sub.Error = err.Error()
		return fmt.Sprintf("TOOL ERROR: save asset %s failed: %v", name, err)
	}
	sub.OK = true
	return fmt.Sprintf("OK: video %s saved (%d bytes)", name, len(data))
}

// extractVideoURL 从视频生成响应中提取可下载 URL（同步响应或轮询完成响应，多形态兼容）：
// output.video_url / video_url / content.video_url / data[0].url
func extractVideoURL(resp map[string]interface{}) string {
	if out, ok := resp["output"].(map[string]interface{}); ok {
		if u, _ := out["video_url"].(string); u != "" {
			return u
		}
	}
	if u, _ := resp["video_url"].(string); u != "" {
		return u
	}
	if content, ok := resp["content"].(map[string]interface{}); ok {
		if u, _ := content["video_url"].(string); u != "" {
			return u
		}
	}
	if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
		if m, ok := data[0].(map[string]interface{}); ok {
			if u, _ := m["url"].(string); u != "" {
				return u
			}
		}
	}
	return ""
}

// extractVideoTaskID 从视频生成提交响应中提取异步任务 id（顶层 id/task_id → output.* → data[0].*）
func extractVideoTaskID(resp map[string]interface{}) string {
	candidates := []map[string]interface{}{resp}
	if out, ok := resp["output"].(map[string]interface{}); ok {
		candidates = append(candidates, out)
	}
	if data, ok := resp["data"].([]interface{}); ok && len(data) > 0 {
		if m, ok := data[0].(map[string]interface{}); ok {
			candidates = append(candidates, m)
		}
	}
	for _, m := range candidates {
		for _, key := range []string{"id", "task_id"} {
			if id, _ := m[key].(string); id != "" {
				return id
			}
		}
	}
	return ""
}

// videoTaskStatus 提取视频任务状态（顶层 status 或 output.status）
func videoTaskStatus(resp map[string]interface{}) string {
	if s, _ := resp["status"].(string); s != "" {
		return s
	}
	if out, ok := resp["output"].(map[string]interface{}); ok {
		if s, _ := out["status"].(string); s != "" {
			return s
		}
	}
	return ""
}

// multiAssetName 多张生成结果的资产命名：单张用原名；多张拆为 name-1.ext 序号（无扩展名补 defaultExt）
func multiAssetName(assetName string, idx, total int, defaultExt string) string {
	ext := filepath.Ext(assetName)
	stem := strings.TrimSuffix(assetName, ext)
	if ext == "" {
		ext = defaultExt
	}
	if total > 1 {
		return fmt.Sprintf("%s-%d%s", stem, idx+1, ext)
	}
	return stem + ext
}

// saveWorkspaceAsset 生成类工具的落盘通道：写入任务工作区并登记资产清单（与 write_asset 同一口径）
func saveWorkspaceAsset(wsDir string, result *AgentRunResult, name string, data []byte) (string, error) {
	if !assetPathValid(name) {
		return "", fmt.Errorf("invalid asset name %q", name)
	}
	target := filepath.Join(wsDir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", err
	}
	size := int64(len(data))
	if info, err := os.Stat(target); err == nil {
		size = info.Size()
	}
	upsertAsset(result, AgentAssetInfo{
		Name:    name,
		Size:    size,
		Mime:    mime.TypeByExtension(filepath.Ext(name)),
		RelPath: name,
	})
	return target, nil
}

// downloadGeneratedFile 下载生成的图片/视频资源（限大小上限，超限报错）。
// 下载本身免费，失败后让模型重新生成反而要重新花钱，故网络抖动/5xx 自动重试
// downloadRetryAttempts-1 次（退避 1s/2s）；4xx（URL 过期/无权限）为确定性失败不重试
func downloadGeneratedFile(url string, maxSz int64) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= downloadRetryAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * time.Second)
		}
		data, err := downloadGeneratedFileOnce(url, maxSz)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if ue, ok := ai.AsUpstreamError(err); ok && ue.StatusCode >= 400 && ue.StatusCode < 500 {
			return nil, err
		}
	}
	return nil, lastErr
}

// downloadGeneratedFileOnce 单次下载（2 分钟超时；非 2xx 包成 UpstreamError 供重试分类）
func downloadGeneratedFileOnce(url string, maxSz int64) ([]byte, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, &ai.UpstreamError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSz+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSz {
		return nil, fmt.Errorf("file exceeds size limit (%d bytes)", maxSz)
	}
	return data, nil
}

// findLoadedSkill 在已装载技能中按名称查找
func findLoadedSkill(skills []models.Skill, name string) (*models.Skill, error) {
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i], nil
		}
	}
	return nil, fmt.Errorf("skill %q not loaded in this task", name)
}

// markSkillFileLoaded 记录渐进披露行为（内存态去重，最终统一落库）
func markSkillFileLoaded(result *AgentRunResult, skillID uint, path string) {
	for i := range result.SkillsUsed {
		if result.SkillsUsed[i].ID == skillID {
			dup := false
			for _, f := range result.SkillsUsed[i].LoadedFiles {
				if f == path {
					dup = true
					break
				}
			}
			if !dup {
				result.SkillsUsed[i].LoadedFiles = append(result.SkillsUsed[i].LoadedFiles, path)
			}
			return
		}
	}
}

// markSkillBodyLoaded 标记技能 SKILL.md 正文已进入模型上下文（单技能直注或经 load_skill 装载）
func markSkillBodyLoaded(result *AgentRunResult, skillID uint) {
	for i := range result.SkillsUsed {
		if result.SkillsUsed[i].ID == skillID {
			result.SkillsUsed[i].BodyLoaded = true
			return
		}
	}
}

// selectSandbox 从后端链中选出首个实际可用的沙箱后端；链空或全部不可用时返回 nil。
// taskEnvDir 非空时（任务级 venv 或 node_modules）跳过不支持宿主侧任务环境的后端
// （如 macOS 宿主 + 容器后端：托管 CPython 是 darwin 二进制，容器内无法运行）。
// 逐次探测（而非构造时缓存），保证运行环境变化后行为一致
func (r *AgentRuntime) selectSandbox(taskEnvDir string) Sandbox {
	for _, sb := range r.sandboxes {
		if taskEnvDir != "" && !sb.SupportsVenv() {
			continue
		}
		if sb.Available() {
			return sb
		}
	}
	return nil
}

// selectSandboxAdaptive 自适应后端选择（执行与环境画像共用）：
// 先按任务环境兼容性筛选后端链；无可用后端且存在任务环境（venv 或 node_modules）时
// 降级宿主环境再选一次（如 macOS 宿主锁定 docker 后端：托管 CPython 是 darwin 二进制，
// 容器内无法运行）。envDropped=true 表示已放弃任务环境，调用方需同步回退解释器为
// 系统 python3 并清空 nodeEnvDir
func (r *AgentRuntime) selectSandboxAdaptive(venvDir, nodeEnvDir string) (sb Sandbox, envDropped bool) {
	taskEnvDir := venvDir
	if taskEnvDir == "" {
		taskEnvDir = nodeEnvDir
	}
	if sb := r.selectSandbox(taskEnvDir); sb != nil {
		return sb, false
	}
	if taskEnvDir != "" {
		if sb := r.selectSandbox(""); sb != nil {
			return sb, true
		}
	}
	return nil, false
}

// workspaceScriptAllowed 工作区脚本执行是否可用：配置开关 + 沙箱后端链存在可用后端。
// 不满足时一律拒绝执行（fail-closed），避免自建脚本裸跑在宿主机上
func (r *AgentRuntime) workspaceScriptAllowed() bool {
	return r.allowWorkspaceScript && r.selectSandbox("") != nil
}

// workspaceFilePath 计算任务工作区内脚本的绝对路径（防路径穿越，与 SkillFilePath 同一规整模式）
func workspaceFilePath(wsDir, relPath string) (string, error) {
	clean := filepath.Clean(filepath.Join("/", relPath))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", fmt.Errorf("invalid workspace script path")
	}
	return filepath.Join(wsDir, clean), nil
}

// updateProgress 每轮更新执行进度
func (r *AgentRuntime) updateProgress(taskID uint, iter int) {
	if err := repository.DB.Model(&models.AgentTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"iterations": iter, "status": models.AgentTaskStatusRunning}).Error; err != nil {
		logger.Error("Agent task %d: update progress failed: %v", taskID, err)
	}
}

// persistTaskArtifacts 将技能使用与资产清单写入任务记录
func (r *AgentRuntime) persistTaskArtifacts(taskID uint, result *AgentRunResult) {
	usedBlob, _ := json.Marshal(result.SkillsUsed)
	assetBlob, _ := json.Marshal(result.Assets)
	if err := repository.DB.Model(&models.AgentTask{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"skills_used": models.JSON(usedBlob),
			"assets":      models.JSON(assetBlob),
		}).Error; err != nil {
		logger.Error("Agent task %d: persist artifacts failed: %v", taskID, err)
	}
}

// 步骤 trace 摘要长度约束
const (
	stepToolNameLen      = 64
	stepArgsExcerptLen   = 512
	stepOutputExcerptLen = 1024
)

// recordTaskStep 落库单个工具调用步骤（trace：可观测性与排障用；失败不影响任务执行）
func (r *AgentRuntime) recordTaskStep(taskID string, seq, iter int, call ai.ToolCall, output string, duration time.Duration) {
	ok := !strings.HasPrefix(output, "TOOL ERROR")
	outExcerpt := excerptString(output, stepOutputExcerptLen)
	step := models.AgentTaskStep{
		TaskID:        taskID,
		Seq:           seq,
		Iteration:     iter,
		Tool:          excerptString(call.Function.Name, stepToolNameLen),
		ArgsExcerpt:   excerptString(call.Function.Arguments, stepArgsExcerptLen),
		OK:            ok,
		OutputExcerpt: outExcerpt,
		DurationMs:    duration.Milliseconds(),
	}
	if !ok {
		step.Error = outExcerpt
	}
	if err := repository.DB.Create(&step).Error; err != nil {
		logger.Error("Agent task %s: record step %d failed: %v", taskID, seq, err)
	}
}

// excerptString 按 rune 截断摘要（不破坏 UTF-8 多字节字符），超长补省略号
func excerptString(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// upsertAsset 将资产登记进结果清单（同名重写视为更新）
func upsertAsset(result *AgentRunResult, info AgentAssetInfo) {
	for i := range result.Assets {
		if result.Assets[i].Name == info.Name {
			result.Assets[i] = info
			return
		}
	}
	result.Assets = append(result.Assets, info)
}

// SkillOutput 技能 frontmatter 声明的交付产物
type SkillOutput struct {
	Name     string
	Required bool
}

// declaredOutputs 从 frontmatter JSON 提取产物声明（顶层 outputs 或 metadata.outputs）：
//   - 字符串/字符串数组: ["report.md", "data.csv"] → 全部必需
//   - 对象数组: [{"name": "report.md", "required": false}, ...]（required 缺省 true）
func declaredOutputs(fm models.JSON) []SkillOutput {
	if len(fm) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(fm, &m); err != nil {
		return nil
	}
	raw, ok := m["outputs"]
	if !ok {
		if meta, ok := m["metadata"].(map[string]interface{}); ok {
			raw, ok = meta["outputs"]
		}
	}
	if !ok {
		return nil
	}
	var outs []SkillOutput
	add := func(name string, required bool) {
		name = strings.TrimSpace(name)
		if name != "" {
			outs = append(outs, SkillOutput{Name: name, Required: required})
		}
	}
	switch t := raw.(type) {
	case string:
		add(t, true)
	case []interface{}:
		for _, v := range t {
			switch e := v.(type) {
			case string:
				add(e, true)
			case map[string]interface{}:
				name, _ := e["name"].(string)
				required := true
				if rb, ok := e["required"].(bool); ok {
					required = rb
				}
				add(name, required)
			}
		}
	}
	return outs
}

// validateOutputsContract 校验技能声明的产物路径合法（不合法属技能包缺陷，任务启动期报错不烧 token）
func validateOutputsContract(skills []models.Skill) error {
	var problems []string
	for _, s := range skills {
		for _, o := range declaredOutputs(s.Frontmatter) {
			if !assetPathValid(o.Name) {
				problems = append(problems, fmt.Sprintf("skill %q declares invalid output name %q", s.Name, o.Name))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid outputs contract: %s", strings.Join(problems, "; "))
	}
	return nil
}

// requiredOutputsSummary 生成注入 system prompt 的必需产物清单
func requiredOutputsSummary(skills []models.Skill) string {
	var lines []string
	for _, s := range skills {
		for _, o := range declaredOutputs(s.Frontmatter) {
			if o.Required {
				lines = append(lines, fmt.Sprintf("- %s（技能 %q 要求）", o.Name, s.Name))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// checkMissingOutputs 校验必需产物是否已在工作区落盘（write_asset 或脚本直写均计入，按文件名跨技能去重）
func checkMissingOutputs(wsDir string, skills []models.Skill) []string {
	seen := map[string]bool{}
	var missing []string
	for _, s := range skills {
		for _, o := range declaredOutputs(s.Frontmatter) {
			if !o.Required || seen[o.Name] {
				continue
			}
			seen[o.Name] = true
			if _, err := os.Stat(filepath.Join(wsDir, filepath.FromSlash(o.Name))); err != nil {
				missing = append(missing, o.Name)
			}
		}
	}
	return missing
}

// reconcileWorkspaceAssets 收尾扫描任务工作区，把 manifest 外的产物补录进资产清单。
// write_asset 是 manifest 单轨登记口，但脚本直写工作区的文件同样可下载——这里兜底补齐可见性
// （tmp/ 为脚本临时目录不作为资产；已登记的同名资产不覆盖）
func reconcileWorkspaceAssets(wsDir string, result *AgentRunResult) {
	known := make(map[string]bool, len(result.Assets))
	for _, a := range result.Assets {
		known[a.Name] = true
	}
	_ = filepath.WalkDir(wsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(wsDir, path)
		if rerr != nil {
			return nil
		}
		if d.IsDir() {
			if rel == "tmp" {
				return fs.SkipDir
			}
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if known[relSlash] || !assetPathValid(relSlash) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		upsertAsset(result, AgentAssetInfo{
			Name:    relSlash,
			Size:    info.Size(),
			Mime:    mime.TypeByExtension(filepath.Ext(relSlash)),
			RelPath: relSlash,
		})
		return nil
	})
}

// AssetFilePath 返回资产下载的绝对路径（防穿越：相对路径已在写入时白名单校验，这里再兜底）
func (r *AgentRuntime) AssetFilePath(taskID, name string) (string, error) {
	if !assetPathValid(name) {
		return "", fmt.Errorf("invalid asset name")
	}
	return filepath.Join(r.workspaceRoot, taskID, filepath.FromSlash(name)), nil
}

// firstChoiceMessage 提取 OpenAI 响应中 choices[0].message
func firstChoiceMessage(resp map[string]interface{}) (map[string]interface{}, error) {
	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid choice format")
	}
	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no message in choice")
	}
	return msg, nil
}

// contentToString 将响应 content（string 或其他）转为纯文本
func contentToString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		blob, _ := json.Marshal(t)
		return string(blob)
	}
}

// chatWithRetry 带重试的模型调用（流式），按错误类型分级：
//   - ctx 取消：立即放弃，不再重试
//   - 确定性上游错误（4xx，除 408 请求超时与 429 限流）：参数/认证/余额等失败重试无意义，直接返回
//   - 瞬时错误（429/408/5xx/网络抖动）：最多重试 chatRetryAttempts-1 次，退避 1s/2s；
//     429 退避加倍（2s/4s）——连续限流时短退避大概率继续撞限流
//
// 走流式（ChatStream）而非非流式（Chat）：agent 长输出（如 write_asset 写整页 HTML 的单次
// tool_call 参数）非流式期间上游持续静默，链路上的网关/代理可能因空闲超时掐断连接；流式期间
// 分片持续回包消除该静默窗口。分片在 chatStreamAssemble 内还原成与非流式完全相同的响应形状，
// 重试语义不变（上游错误在任何回调触发前返回，不存在半截数据被误当成功重试的问题）。
func chatWithRetry(ctx context.Context, adapter ai.Adapter, messages []ai.Message, chatOptions map[string]interface{}) (map[string]interface{}, error) {
	var lastErr error
	for attempt := 1; attempt <= chatRetryAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt-1) * time.Second
			if ue, ok := ai.AsUpstreamError(lastErr); ok && ue.StatusCode == http.StatusTooManyRequests {
				backoff *= 2
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		resp, err := chatStreamAssemble(ctx, adapter, messages, chatOptions)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		logger.Error("agent chat attempt %d/%d failed: %v", attempt, chatRetryAttempts, err)
		if ue, ok := ai.AsUpstreamError(err); ok {
			if sc := ue.StatusCode; sc >= 400 && sc < 500 && sc != http.StatusRequestTimeout && sc != http.StatusTooManyRequests {
				return nil, lastErr
			}
		}
	}
	return nil, lastErr
}

// chatStreamAssemble 以流式发起一次 chat 调用，把增量分片还原成与非流式 Chat 相同的
// OpenAI 响应形状（choices[0].message 的 content/tool_calls + usage），agent 工具环
// 下游（ExtractTokenUsage / firstChoiceMessage / tool_calls 解析）无需感知流式。
func chatStreamAssemble(ctx context.Context, adapter ai.Adapter, messages []ai.Message, chatOptions map[string]interface{}) (map[string]interface{}, error) {
	acc := &streamAccumulator{}
	err := adapter.ChatStream(ctx, messages, chatOptions, acc.absorb)
	if err != nil {
		return nil, err
	}
	return acc.response(), nil
}

// streamAccumulator 累积流式增量分片，还原成非流式 OpenAI 响应。
// tool_calls 分片按上游 index 合并（id/function.name 取首个非空，arguments 逐段拼接）；
// usage 取任意分片携带的最后一次值（Generic 流式已注入 stream_options.include_usage）。
type streamAccumulator struct {
	content      strings.Builder
	toolCalls    []ai.ToolCall
	toolSlots    map[int]int // 上游 tool_calls index → toolCalls 下标
	finishReason string
	usage        map[string]interface{}
	id           string
	model        string
	chunks       int
}

func (s *streamAccumulator) toolSlot(idx int) *ai.ToolCall {
	if pos, ok := s.toolSlots[idx]; ok {
		return &s.toolCalls[pos]
	}
	if s.toolSlots == nil { // 懒初始化（零值可用）
		s.toolSlots = make(map[int]int)
	}
	pos := len(s.toolCalls)
	s.toolCalls = append(s.toolCalls, ai.ToolCall{Type: "function"})
	s.toolSlots[idx] = pos
	return &s.toolCalls[pos]
}

func (s *streamAccumulator) absorb(chunk map[string]interface{}) {
	if chunk == nil {
		return
	}
	if done, _ := chunk["done"].(bool); done { // parseStreamLine 的 [DONE] 标记
		return
	}
	s.chunks++
	if v, ok := chunk["id"].(string); ok && v != "" {
		s.id = v
	}
	if v, ok := chunk["model"].(string); ok && v != "" {
		s.model = v
	}
	if u, ok := chunk["usage"].(map[string]interface{}); ok {
		s.usage = u
	}
	choices, ok := chunk["choices"].([]interface{})
	if !ok {
		return
	}
	for _, ci := range choices {
		choice, ok := ci.(map[string]interface{})
		if !ok {
			continue
		}
		if fr, ok := choice["finish_reason"].(string); ok && fr != "" {
			s.finishReason = fr
		}
		delta, _ := choice["delta"].(map[string]interface{})
		if delta == nil {
			// 兼容个别上游在末块直接给 message 形状
			delta, _ = choice["message"].(map[string]interface{})
		}
		if delta == nil {
			continue
		}
		if c, ok := delta["content"].(string); ok {
			s.content.WriteString(c)
		}
		rawCalls, ok := delta["tool_calls"].([]interface{})
		if !ok {
			continue
		}
		for _, callRaw := range rawCalls {
			call, ok := callRaw.(map[string]interface{})
			if !ok {
				continue
			}
			idx := len(s.toolCalls) // 缺 index 视为新工具调用（规范要求携带，防御性兜底）
			if v, ok := call["index"].(float64); ok {
				idx = int(v)
			}
			entry := s.toolSlot(idx)
			if v, ok := call["id"].(string); ok && v != "" && entry.ID == "" {
				entry.ID = v
			}
			if v, ok := call["type"].(string); ok && v != "" {
				entry.Type = v
			}
			if fn, ok := call["function"].(map[string]interface{}); ok {
				if v, ok := fn["name"].(string); ok && v != "" && entry.Function.Name == "" {
					entry.Function.Name = v
				}
				if v, ok := fn["arguments"].(string); ok {
					entry.Function.Arguments += v
				}
			}
		}
	}
}

// response 还原非流式 OpenAI 响应形状；finish_reason 缺失时按内容推断（信息性字段，工具环不依赖）
func (s *streamAccumulator) response() map[string]interface{} {
	finishReason := s.finishReason
	if finishReason == "" {
		if len(s.toolCalls) > 0 {
			finishReason = "tool_calls"
		} else {
			finishReason = "stop"
		}
	}
	msg := map[string]interface{}{"role": "assistant"}
	if c := s.content.String(); c != "" {
		msg["content"] = c
	} else {
		msg["content"] = nil
	}
	if len(s.toolCalls) > 0 {
		msg["tool_calls"] = s.toolCalls
	}
	resp := map[string]interface{}{
		"object": "chat.completion",
		"choices": []interface{}{
			map[string]interface{}{"message": msg, "finish_reason": finishReason},
		},
	}
	if s.id != "" {
		resp["id"] = s.id
	}
	if s.model != "" {
		resp["model"] = s.model
	}
	if s.usage != nil {
		resp["usage"] = s.usage
	}
	logger.Debug("agent chat streamed: %d chunks, finish_reason=%s, content %d chars, %d tool_calls", s.chunks, finishReason, s.content.Len(), len(s.toolCalls))
	return resp
}

// precheckSkillDependencies 解析技能 frontmatter 的依赖声明并预检执行环境（任务启动时快速失败，不烧 token）。
// venv 模式（r.pyenv != nil）：pip: 依赖改在任务专属 venv 内满足（uv 构建+安装）、npm: 依赖改在
// 任务专属 node_modules 内满足（宿主 npm 安装）——收集后一次性构建（首次任务耗时数秒~数十秒，后续任务复用秒级返回）
// 平台扩展约定（非 Agent Skills 标准强制项），frontmatter 顶层 dependencies 或 metadata.dependencies：
//
//   - 字符串数组: ["python3", "node >= 18", "node >= 20, < 24", "pip:requests", "npm:typescript"]
//
//     · 纯可执行名：校验 PATH 上是否存在（venv 模式下 python3/node/npm 例外：分别由任务 venv
//     托管 CPython 与三级引导的自托管 Node 运行时提供，宿主机未装也能执行）
//     · 可执行名 + 版本约束（>=/<=/==/!=/>/</无操作符，无操作符视为最低版本；逗号分隔多段须全部满足）：
//     运行 <bin> --version 实测比对；版本探测失败时 fail-open（仅确认二进制存在，部分二进制不支持 --version）
//     · pip:<包名>：经 python3 -m pip show 校验已安装（venv 模式下改为安装进任务 venv）
//     · npm:<包名>：经 npm ls -g --depth=0 校验全局已安装（venv 模式下改为安装进任务 node_modules）
//
//   - 对象: {"python3": ">=3.8"}（键为依赖声明，值为版本约束；等价于 "键 值" 字符串形式）
func (r *AgentRuntime) precheckSkillDependencies(ctx context.Context, taskID string, skills []models.Skill) error {
	var problems []string
	cache := map[string]string{} // 依赖声明 → 校验失败原因（"" 表示通过）；跨技能去重，避免重复探测
	pipPkgs := []string{}        // venv 模式：待装入任务 venv 的 pip 依赖（跨技能去重）
	pipSeen := map[string]bool{}
	npmPkgs := []string{} // venv 模式：待装入任务 node_modules 的 npm 依赖（跨技能去重）
	npmSeen := map[string]bool{}
	for _, s := range skills {
		for _, dep := range declaredDependencies(s.Frontmatter) {
			if pkg, ok := strings.CutPrefix(dep, "pip:"); ok && r.pyenv != nil {
				pkg = strings.TrimSpace(pkg)
				if pkg == "" {
					problems = append(problems, fmt.Sprintf("skill %q requires %q: empty pip package name", s.Name, dep))
					continue
				}
				if !pipSeen[pkg] {
					pipSeen[pkg] = true
					pipPkgs = append(pipPkgs, pkg)
				}
				continue
			}
			if pkg, ok := strings.CutPrefix(dep, "npm:"); ok && r.pyenv != nil {
				pkg = strings.TrimSpace(pkg)
				if pkg == "" {
					problems = append(problems, fmt.Sprintf("skill %q requires %q: empty npm package name", s.Name, dep))
					continue
				}
				if strings.HasPrefix(pkg, "-") {
					// 依赖声明来自技能包（不可信）：以 "-" 开头会被 npm 解析为命令行旗标（注入防护）
					problems = append(problems, fmt.Sprintf("skill %q requires %q: invalid npm package name", s.Name, dep))
					continue
				}
				if !npmSeen[pkg] {
					npmSeen[pkg] = true
					npmPkgs = append(npmPkgs, pkg)
				}
				continue
			}
			// venv 模式：python3 声明由任务 venv 的托管 CPython 提供，跳过宿主 PATH 探测
			// （宿主机未装 python3 也能执行）；带版本约束时与托管版本比对
			// venv 模式：node/npm 声明由三级引导的自托管 Node 运行时兜底提供（自托管缓存 →
			// 宿主 PATH → 官方发行包下载，宿主机未装 Node 也能执行）；引导降级宿主 PATH 时
			// 回落普通探测，带版本约束时与实际采用的二进制实测比对
			if r.pyenv != nil {
				if fields := strings.Fields(dep); len(fields) > 0 {
					switch fields[0] {
					case "python3":
						constraint := strings.TrimSpace(strings.TrimPrefix(dep, fields[0]))
						if reason := checkVersionConstraint(r.pyenv.cfg.PythonVersion, constraint); reason != "" {
							problems = append(problems, fmt.Sprintf("skill %q requires %q: %s", s.Name, dep, reason))
						}
						continue
					case "node", "npm":
						bin := fields[0]
						constraint := strings.TrimSpace(strings.TrimPrefix(dep, bin))
						binDir, err := r.pyenv.EnsureNodeRuntime(ctx)
						if err != nil {
							problems = append(problems, fmt.Sprintf("skill %q requires %q: node runtime bootstrap failed: %s", s.Name, dep, err))
							continue
						}
						reason := checkDependency(dep) // 引导降级宿主 PATH（binDir == ""）时的探测路径
						if binDir != "" {
							reason = checkManagedBinary(filepath.Join(binDir, bin), constraint)
						}
						if reason != "" {
							problems = append(problems, fmt.Sprintf("skill %q requires %q: %s", s.Name, dep, reason))
						}
						continue
					}
				}
			}
			reason, ok := cache[dep]
			if !ok {
				reason = checkDependency(dep)
				cache[dep] = reason
			}
			if reason != "" {
				problems = append(problems, fmt.Sprintf("skill %q requires %q: %s", s.Name, dep, reason))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("dependency precheck failed: %s", strings.Join(problems, "; "))
	}
	if r.pyenv != nil {
		// 构建任务专属执行环境（venv 含托管解释器与 pip 依赖、node_modules 含 npm 依赖；失败快速返回不烧 token）
		if _, err := r.pyenv.EnsureTaskVenv(ctx, taskID, pipPkgs); err != nil {
			return fmt.Errorf("task python environment build failed: %w", err)
		}
		if _, err := r.pyenv.EnsureTaskNodeEnv(ctx, taskID, npmPkgs); err != nil {
			return fmt.Errorf("task node environment build failed: %w", err)
		}
	}
	return nil
}

// checkVersionConstraint 校验已知版本号是否满足约束（逗号分隔多段须全部满足）。
// 用于无法/无需运行 --version 的已知版本比对（如托管 CPython 与声明约束）
func checkVersionConstraint(actual, constraint string) string {
	if strings.TrimSpace(constraint) == "" {
		return ""
	}
	for _, part := range strings.Split(constraint, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := depVersionRe.FindStringSubmatch(part)
		if m == nil {
			return fmt.Sprintf("unrecognized version constraint %q (expect like \">= 18\", \"!= 1.2\" or \">= 20, < 24\")", part)
		}
		op := m[1]
		if op == "" {
			op = ">=" // 无操作符视为最低版本
		}
		if !versionSatisfies(actual, op, m[2]) {
			return fmt.Sprintf("version %s does not satisfy %q", actual, part)
		}
	}
	return ""
}

// checkDependency 校验单条依赖声明：通过返回 ""，失败返回原因
func checkDependency(dep string) string {
	if pkg, ok := strings.CutPrefix(dep, "pip:"); ok {
		return checkPipPackage(strings.TrimSpace(pkg))
	}
	if pkg, ok := strings.CutPrefix(dep, "npm:"); ok {
		return checkNpmPackage(strings.TrimSpace(pkg))
	}
	fields := strings.Fields(dep)
	if len(fields) == 0 {
		return "empty dependency declaration"
	}
	bin := fields[0]
	path, err := exec.LookPath(bin)
	if err != nil {
		if bin == "node" {
			return fmt.Sprintf("executable %q is not available on host (install Node.js on the server, or set AGENT_PY_ENV_MODE=venv to auto-bootstrap a self-hosted Node runtime)", bin)
		}
		return fmt.Sprintf("executable %q is not available on host", bin)
	}
	if constraint := strings.TrimSpace(strings.TrimPrefix(dep, bin)); constraint != "" {
		return checkBinaryVersion(bin, path, constraint)
	}
	return ""
}

// checkManagedBinary 校验自托管运行时二进制：存在且可执行，带版本约束时实测比对
// （按绝对路径探测版本，PATH 寻址会误中宿主二进制；探测失败 fail-open，同 checkBinaryVersion）
func checkManagedBinary(path, constraint string) string {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || st.Mode()&0o111 == 0 {
		return fmt.Sprintf("managed runtime binary %q is missing or not executable", path)
	}
	if constraint == "" {
		return ""
	}
	return checkBinaryVersion(path, path, constraint)
}

// depVersionRe 版本约束解析：操作符(可选) + 1~3 段数值版本（如 ">= 18"、"!= 1.2"、"3.8"、"== 1.2.3"、带 v 前缀）
var depVersionRe = regexp.MustCompile(`^(>=|<=|==|=|!=|>|<)?\s*v?(\d+(?:\.\d+){0,2})$`)

// versionTokenRe 从二进制 --version 输出中提取版本号
var versionTokenRe = regexp.MustCompile(`\d+(?:\.\d+){0,2}`)

// checkBinaryVersion 实测二进制版本并与约束比对。
// 约束支持逗号分隔多段（如 ">= 20, < 24"，须全部满足）与 != ；版本探测失败时 fail-open
// （仅确认二进制存在，跳过版本比对——部分二进制不支持 --version，fail-closed 会误杀）
func checkBinaryVersion(bin, path, constraint string) string {
	actual, err := probeBinaryVersion(bin, 10*time.Second)
	if err != nil {
		return "" // fail-open：二进制在 PATH 上即视为满足，版本未知不再拦
	}
	for _, part := range strings.Split(constraint, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := depVersionRe.FindStringSubmatch(part)
		if m == nil {
			return fmt.Sprintf("unrecognized version constraint %q (expect like \">= 18\", \"!= 1.2\" or \">= 20, < 24\")", part)
		}
		op := m[1]
		if op == "" {
			op = ">=" // 无操作符视为最低版本
		}
		if !versionSatisfies(actual, op, m[2]) {
			return fmt.Sprintf("found version %s at %q, which does not satisfy %q", actual, path, part)
		}
	}
	return ""
}

// profileProbeTimeout 环境画像采集中单个二进制的探测超时（画像要快，避免拖慢任务启动）
const profileProbeTimeout = 3 * time.Second

// probeBinaryVersion 运行 `<bin> --version`（无输出再试 -V），从输出首行提取版本号
func probeBinaryVersion(bin string, timeout time.Duration) (string, error) {
	cctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var lastErr error
	for _, flag := range []string{"--version", "-V"} {
		out, err := exec.CommandContext(cctx, bin, flag).CombinedOutput()
		first := firstLine(string(out))
		if m := versionTokenRe.FindString(first); m != "" {
			return m, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("no version number in %q output", flag)
		}
	}
	return "", lastErr
}

// versionSatisfies 判断实际版本是否满足约束（op 为 >/>=/</<=/==/!=）
func versionSatisfies(actual, op, want string) bool {
	c := compareVersionParts(actual, want)
	switch op {
	case ">=":
		return c >= 0
	case "<=":
		return c <= 0
	case ">":
		return c > 0
	case "<":
		return c < 0
	case "!=":
		return c != 0
	default: // "==" / "="
		return c == 0
	}
}

// collectHostProfile 采集宿主机环境画像的基础部分（OS/arch + 常用解释器版本），进程内缓存一次
func collectHostProfile() string {
	var b strings.Builder
	fmt.Fprintf(&b, "- 操作系统/架构: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	for _, bin := range []string{"python3", "pip3", "node", "npm"} {
		if v, err := probeBinaryVersion(bin, profileProbeTimeout); err == nil && v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", bin, v)
		} else {
			fmt.Fprintf(&b, "- %s: 未检测到\n", bin)
		}
	}
	return b.String()
}

// hostEnvironmentProfile 构建注入 system prompt 的宿主机环境画像：
// 基础部分（OS/解释器版本）经 sync.Once 进程内缓存，工作区路径逐任务拼接。
// 目的：模型开跑前就知道环境事实，不必烧轮次用 --version/试探脚本探测
func (r *AgentRuntime) hostEnvironmentProfile(wsDir string) string {
	r.profileOnce.Do(func() { r.profileBase = collectHostProfile() })
	var b strings.Builder
	b.WriteString("宿主机环境画像（实测，可信）：\n")
	b.WriteString(r.profileBase)
	// 沙箱后端自适应选择（与 execute_script 共用逻辑，保证画像与实况一致）：
	// venv 模式先按兼容性筛选，全部被过滤时降级宿主环境重选（envDropped=true）
	venvHint, nodeHint := "", ""
	if r.pyenv != nil {
		taskID := filepath.Base(wsDir)
		venvHint = r.pyenv.TaskVenvDir(taskID)
		if _, err := os.Stat(r.pyenv.TaskNodeModulesDir(taskID)); err == nil {
			nodeHint = r.pyenv.TaskNodeEnvDir(taskID)
		}
	}
	sb, envDropped := r.selectSandboxAdaptive(venvHint, nodeHint)
	// venv 模式：python 脚本运行于任务专属 venv，画像如实描述，避免模型误用系统 pip/系统 Python
	if r.pyenv != nil {
		if envDropped {
			b.WriteString("- Python 执行环境: 系统 Python（任务专属 venv 与当前沙箱后端不兼容，已降级；请勿假设 venv 预装包存在）\n")
		} else {
			version, pkgs := r.pyenv.VenvInfo(venvHint)
			switch {
			case version == "":
				b.WriteString("- Python 执行环境: 任务专属 venv（构建中，首次 python 脚本执行前自动就绪）\n")
			case len(pkgs) > 0:
				fmt.Fprintf(&b, "- Python 执行环境: 任务专属 venv（CPython %s，预装包: %s）。沙箱内禁网不可临时 pip 安装，依赖须在 dependencies 中声明 pip: 包名\n", version, strings.Join(pkgs, ", "))
			default:
				b.WriteString("- Python 执行环境: 任务专属 venv（仅标准库）。沙箱内禁网不可临时 pip 安装，第三方依赖须在 dependencies 中声明 pip: 包名\n")
			}
		}
		// Node 执行环境画像（npm: 依赖经宿主 npm 预装到任务 node_modules；与 Python 画像同受 envDropped 约束）
		switch {
		case envDropped && nodeHint != "":
			b.WriteString("- Node 执行环境: 宿主机（任务级 node_modules 与当前沙箱后端不兼容，已降级；npm 依赖不可用，纯逻辑脚本可执行）\n")
		case nodeHint != "":
			if pkgs := r.pyenv.NodeEnvInfo(nodeHint); len(pkgs) > 0 {
				fmt.Fprintf(&b, "- Node 执行环境: 任务级 node_modules（预装包: %s）。沙箱内禁网不可临时 npm 安装，依赖须在 dependencies 中声明 npm: 包名；NODE_PATH 仅对 require 生效（ESM import 不适用）\n", strings.Join(pkgs, ", "))
			} else {
				b.WriteString("- Node 执行环境: 任务级 node_modules（空，依赖尚未装入）。沙箱内禁网不可临时 npm 安装，第三方依赖须在 dependencies 中声明 npm: 包名；NODE_PATH 仅对 require 生效（ESM import 不适用）\n")
			}
		}
	}
	// Node 不可用时显式给出出路（与基础画像的 "node: 未检测到" 呼应，避免模型反复试错 JS 脚本）；
	// node_modules 存在（nodeHint != ""）即隐含 node 可用，上面已给出正向画像，不再重复负面提示；
	// 自托管运行时已引导时分两支如实画像（只探测不触发下载）
	if _, err := exec.LookPath("node"); err != nil && nodeHint == "" {
		nodeBin := ""
		if r.pyenv != nil {
			nodeBin = r.pyenv.NodeRuntimeBinDir()
		}
		if nodeBin != "" {
			fmt.Fprintf(&b, "- Node 执行环境: 宿主机未安装 Node.js，已自动引导自托管运行时（%s），node 脚本与 npm: 依赖声明照常可用\n", nodeBin)
		} else {
			b.WriteString("- Node 执行环境: 宿主机未安装 Node.js（node 脚本不可执行，npm: 依赖声明也无法满足；请改用 Python 脚本，或让用户在宿主机安装 Node.js）\n")
		}
	}
	fmt.Fprintf(&b, "- 任务工作区绝对路径: %s（execute_script 的工作目录，SKILL_WORKSPACE 即指向此路径）\n", wsDir)
	// 沙箱画像按自适应选择结果生成（plain 兜底直跑场景如实呈现边界，模型据此保守行事）
	if sb != nil {
		fmt.Fprintf(&b, "- 沙箱边界（%s）：%s。\n", sb.Name(), sb.ProfileLine())
	} else {
		b.WriteString("- 沙箱边界：当前配置无任何可用脚本执行后端（无可用沙箱且未允许直跑兜底），execute_script 将拒绝执行。请避免依赖脚本执行，直接以模型自身能力完成任务并用 write_asset 交付。\n")
	}
	return b.String()
}

// compareVersionParts 按数值逐段比较版本号（缺省段按 0 补齐），返回 -1/0/1
func compareVersionParts(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// checkPipPackage 校验 python 三方包已安装（python3 -m pip show，退出码 0 即已安装）
func checkPipPackage(pkg string) string {
	if pkg == "" {
		return "empty package name"
	}
	if _, err := exec.LookPath("python3"); err != nil {
		return fmt.Sprintf("cannot verify python package %q: python3 not available on host", pkg)
	}
	cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "python3", "-m", "pip", "show", pkg).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "No module named pip") {
			return fmt.Sprintf("cannot verify python package %q: pip is not available in python3", pkg)
		}
		return fmt.Sprintf("python package %q is not installed (checked via python3 -m pip show)", pkg)
	}
	return ""
}

// checkNpmPackage 校验 npm 全局包已安装（npm ls -g --depth=0，退出码 0 即已安装）
func checkNpmPackage(pkg string) string {
	if pkg == "" {
		return "empty package name"
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Sprintf("cannot verify npm package %q: npm not available on host (install Node.js/npm on the server, or set AGENT_PY_ENV_MODE=venv to auto-install npm: deps into a per-task node_modules)", pkg)
	}
	cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := exec.CommandContext(cctx, "npm", "ls", "-g", "--depth=0", pkg).Run(); err != nil {
		return fmt.Sprintf("npm package %q is not installed globally (checked via npm ls -g)", pkg)
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// declaredDependencies 从 frontmatter JSON 提取依赖声明（顶层 dependencies 或 metadata.dependencies）
func declaredDependencies(fm models.JSON) []string {
	if len(fm) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(fm, &m); err != nil {
		return nil
	}
	raw, ok := m["dependencies"]
	if !ok {
		if meta, ok := m["metadata"].(map[string]interface{}); ok {
			raw, ok = meta["dependencies"]
		}
	}
	if !ok {
		return nil
	}
	var deps []string
	switch t := raw.(type) {
	case []interface{}:
		for _, v := range t {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				deps = append(deps, strings.TrimSpace(s))
			}
		}
	case string:
		if strings.TrimSpace(t) != "" {
			deps = append(deps, strings.TrimSpace(t))
		}
	case map[string]interface{}:
		for k, v := range t {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			if vs, ok := v.(string); ok && strings.TrimSpace(vs) != "" {
				deps = append(deps, k+" "+strings.TrimSpace(vs)) // {"python3": ">=3.8"} → "python3 >=3.8"
			} else {
				deps = append(deps, k)
			}
		}
	}
	sort.Strings(deps) // map 遍历无序，排序保证报错信息稳定
	return deps
}

// envSecretRe 环境变量名疑似敏感项（不透传给脚本进程，防密钥经 stdout/资产外泄）
var envSecretRe = regexp.MustCompile(`(?i)(secret|token|password|api_?key|credential|private)`)

// sanitizeEnv 过滤疑似敏感的环境变量
func sanitizeEnv() []string {
	raw := os.Environ()
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		k, _, _ := strings.Cut(e, "=")
		if envSecretRe.MatchString(k) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// buildScriptEnv 组装脚本进程环境：过滤敏感变量后覆盖/追加运行时所需项。
// SKILL_ROOT 由运行时完全控制：技能包脚本注入包根；工作区自建脚本显式清除，
// 防止宿主机环境残留的同名变量误导脚本。
// venvDir 非空（venv 模式）：注入 VIRTUAL_ENV 与任务 venv bin 前置的 PATH，
// 并清除宿主机残留的 PYTHONHOME/PYTHONPATH（会把解释器引向系统环境）。
// nodeEnvDir 非空：注入 NODE_PATH 与 node_modules/.bin 前置的 PATH。
// PATH 统一局部组装后一次性写入，避免 venv/node 段互相覆盖对方的前缀。
// backendName 非空（沙箱后端执行）：注入 TASTE_CHROME_EXTRA_ARGS=--no-sandbox——
// Chrome/Chromium 自身的内层沙箱（setuid sandbox / Seatbelt 辅助进程）在外层
// 沙箱下初始化必失败（Operation not permitted），需由脚本侧的浏览器驱动
// （skills/*/scripts/lib/cdp.mjs）读取该变量透传 --no-sandbox；plain 后端无此
// 冲突，不注入（保持 Chrome 内层沙箱原样）
func buildScriptEnv(tmpDir, wsDir, skillRoot, venvDir, nodeEnvDir, backendName string) []string {
	env := sanitizeEnv()
	env = setEnvVar(env, "TMPDIR", tmpDir)
	env = setEnvVar(env, "SKILL_WORKSPACE", wsDir)
	if backendName != "" {
		env = setEnvVar(env, "TASTE_CHROME_EXTRA_ARGS", "--no-sandbox")
	} else {
		env = unsetEnvVar(env, "TASTE_CHROME_EXTRA_ARGS")
	}
	if skillRoot != "" {
		env = setEnvVar(env, "SKILL_ROOT", skillRoot) // 技能包根绝对路径，供脚本定位包内资源
	} else {
		env = unsetEnvVar(env, "SKILL_ROOT")
	}
	env = setEnvVar(env, "PYTHONDONTWRITEBYTECODE", "1") // 免去 python 写字节码缓存（工作区外写会被沙箱拒绝）
	path := os.Getenv("PATH")
	if venvDir != "" {
		path = filepath.Join(venvDir, "bin") + ":" + path
		env = setEnvVar(env, "VIRTUAL_ENV", venvDir)
		// 任务环境自洽：宿主机残留的 Python 路径设置会把解释器引向系统环境，必须清除
		env = unsetEnvVar(env, "PYTHONHOME")
		env = unsetEnvVar(env, "PYTHONPATH")
	}
	if nodeEnvDir != "" {
		nmDir := filepath.Join(nodeEnvDir, "node_modules")
		// NODE_PATH 仅对 CJS require 生效（Node 限制，ESM import 不读取）；
		// ESM 依赖脚本侧以相对/绝对路径引用 node_modules 内模块
		path = filepath.Join(nmDir, ".bin") + ":" + path
		env = setEnvVar(env, "NODE_PATH", nmDir)
	}
	// 自托管 Node 运行时（宿主无 node 时的引导产物）：宿主 PATH 已有 node 则不前置
	//（与 resolveInterpreter 的宿主优先语义一致）；venvDir 单独存在（纯逻辑 node 脚本
	// 无 nodeEnvDir）时也经 envRootOf 覆盖
	if _, err := exec.LookPath("node"); err != nil {
		if envRoot := envRootOf(venvDir, nodeEnvDir); envRoot != "" {
			if bin := managedNodeBin(envRoot); bin != "" {
				path = bin + ":" + path
			}
		}
	}
	env = setEnvVar(env, "PATH", path)
	return env
}

// unsetEnvVar 从环境切片中移除指定变量
func unsetEnvVar(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}

// setEnvVar 覆盖或追加环境变量（避免重复键——execve 下 getenv 取首值会吞掉覆盖）
func setEnvVar(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

// limitedBuffer 带字节上限的缓冲（超出部分丢弃，防失控脚本占爆内存）
type limitedBuffer struct {
	buf     bytes.Buffer
	limit   int
	dropped bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < b.limit {
		n := b.limit - b.buf.Len()
		if n > len(p) {
			n = len(p)
		}
		b.buf.Write(p[:n])
		if n < len(p) {
			b.dropped = true
		}
	} else {
		b.dropped = true
	}
	return len(p), nil // 永远报告写成功，避免脚本因管道错误中断
}

func (b *limitedBuffer) String() string {
	s := b.buf.String()
	if b.dropped {
		s += fmt.Sprintf("\n[TRUNCATED: output exceeded %d bytes]", b.limit)
	}
	return s
}
