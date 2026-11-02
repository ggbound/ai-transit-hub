package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// ModelRequirement 一次模型调用所需的类型与能力要求。
// 来源两处：技能 frontmatter 的 model_requirements 声明、Agent 运行时生成工具的按需需求。
type ModelRequirement struct {
	Type         string   // 模型类型：text / image / video / embedding（空表示不限）
	Capabilities []string // 必须全部具备的能力标签（如 function_calling）
	Model        string   // 可选：指定模型全名（provider/code），非空时跳过自动选择
	Exclude      []string // 可选：自动选择时排除的模型 code（任务内确定性失败模型的熔断名单）
}

// ModelSelection 按需选型结果：provider + model + 随机可用的平台 API Key
type ModelSelection struct {
	Provider *models.AiProvider
	Model    *models.AiModel
	ApiKey   string
}

// ModelSelector 按需模型选型器：在模型库（ai_models × ai_providers × provider_api_keys）上
// 按「类型精确匹配 → 能力标签校验 → 提供商可用性」筛选，再按能力与价格择优。
// 平台无需预先配置映射，技能声明或运行时提出要求即可自动选到可用模型。
type ModelSelector struct{}

// NewModelSelector 创建选型器（无状态，可全局复用）
func NewModelSelector() *ModelSelector { return &ModelSelector{} }

// Select 按要求选出一个可调用模型：
//  1. req.Model 非空 → 直接解析指定模型（仍校验类型匹配与提供商可用性）
//  2. 候选过滤：m.type 精确匹配、m.status=1、p.status=1、p 存在活跃 API Key
//  3. 能力校验：req.Capabilities 中每个标签模型都必须具备
//  4. 择优排序：能力标签数多者优先 → 单价（input+output）低者优先 → id 稳定序
func (s *ModelSelector) Select(req ModelRequirement) (*ModelSelection, error) {
	reqType := strings.TrimSpace(req.Type)
	reqModel := strings.TrimSpace(req.Model)

	// 1. 显式指定模型：直接解析，仅校验可用性（尊重调用方的明确意图）
	if reqModel != "" {
		return s.selectNamedModel(reqModel, reqType)
	}

	// 2. 类型过滤 + 提供商可用性过滤
	if reqType == "" {
		return nil, fmt.Errorf("model requirement must specify type or model")
	}
	var providerIDs []uint
	if err := repository.DB.Model(&models.AiProvider{}).
		Where("status = 1 AND id IN (SELECT ai_provider_id FROM provider_api_keys WHERE status = 1)").
		Pluck("id", &providerIDs).Error; err != nil {
		return nil, fmt.Errorf("load providers: %w", err)
	}
	if len(providerIDs) == 0 {
		return nil, fmt.Errorf("no enabled provider with active api keys")
	}
	var candidates []models.AiModel
	if err := repository.DB.
		Where("type = ? AND status = 1 AND provider_id IN ?", reqType, providerIDs).
		Order("id").Find(&candidates).Error; err != nil {
		return nil, fmt.Errorf("load models: %w", err)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available model of type %q", reqType)
	}

	// 3. 能力标签校验（要求的每个标签模型都要具备）
	var qualified []models.AiModel
	for _, m := range candidates {
		if modelHasCapabilities(m, req.Capabilities) {
			qualified = append(qualified, m)
		}
	}
	if len(qualified) == 0 {
		return nil, fmt.Errorf("no model of type %q satisfies capabilities %v", reqType, req.Capabilities)
	}

	// 4. 择优：能力标签数多者优先 → 价格低者优先 → id 稳定序
	sort.SliceStable(qualified, func(i, j int) bool {
		ci, cj := modelCapabilityCount(qualified[i]), modelCapabilityCount(qualified[j])
		if ci != cj {
			return ci > cj
		}
		pi := qualified[i].InputPrice + qualified[i].OutputPrice
		pj := qualified[j].InputPrice + qualified[j].OutputPrice
		if pi != pj {
			return pi < pj
		}
		return qualified[i].ID < qualified[j].ID
	})

	// 4.5 任务内熔断：排除本次任务已确定性失败（401/402/403）的模型，改选次优；
	// 全部被排除时保留原列表——尝试一次总好过直接失败
	if len(req.Exclude) > 0 {
		excluded := make(map[string]bool, len(req.Exclude))
		for _, code := range req.Exclude {
			excluded[code] = true
		}
		var remain []models.AiModel
		for _, m := range qualified {
			if !excluded[m.Code] {
				remain = append(remain, m)
			}
		}
		if len(remain) > 0 {
			qualified = remain
		} else {
			logger.Warn("[ModelSelector] all qualified %s models fused in this task, falling back to full list: %v", reqType, req.Exclude)
		}
	}

	picked := &qualified[0]
	return s.buildSelection(picked)
}

// CreateAdapter 按要求选出模型并创建对应适配器（子模型调用一步到位：选型 + Key + adapter）
func (s *ModelSelector) CreateAdapter(req ModelRequirement) (ai.Adapter, *ModelSelection, error) {
	sel, err := s.Select(req)
	if err != nil {
		return nil, nil, err
	}
	adapter, err := ai.NewAdapterFactory().CreateAdapterWithApiKey(sel.Provider, sel.ApiKey, sel.Model)
	if err != nil {
		return nil, nil, fmt.Errorf("create adapter for %s: %w", sel.Model.GetFullModelId(), err)
	}
	logger.Info("[ModelSelector] selected %s (type=%s) for requirement type=%s caps=%v",
		sel.Model.GetFullModelId(), sel.Model.Type, req.Type, req.Capabilities)
	return adapter, sel, nil
}

// selectNamedModel 解析显式指定的模型全名（provider/code）并校验类型与可用性
func (s *ModelSelector) selectNamedModel(fullName, reqType string) (*ModelSelection, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("invalid model %q: expected provider/code", fullName)
	}
	var provider models.AiProvider
	if err := repository.DB.Where("code = ? AND status = 1", parts[0]).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("provider %q not found or disabled", parts[0])
	}
	var model models.AiModel
	if err := repository.DB.Where("provider_id = ? AND code = ? AND status = 1", provider.ID, parts[1]).First(&model).Error; err != nil {
		return nil, fmt.Errorf("model %q not found or disabled", fullName)
	}
	if reqType != "" && model.Type != reqType {
		return nil, fmt.Errorf("model %q has type %q, requirement expects %q", fullName, model.Type, reqType)
	}
	var keyCount int64
	if err := repository.DB.Model(&models.ProviderApiKey{}).
		Where("ai_provider_id = ? AND status = 1", provider.ID).Count(&keyCount).Error; err != nil {
		return nil, fmt.Errorf("check api keys: %w", err)
	}
	if keyCount == 0 {
		return nil, fmt.Errorf("provider %q has no active api keys", provider.Code)
	}
	return s.buildSelection(&model)
}

// buildSelection 为选中模型随机挑一个活跃 API Key（多 Key 负载分摊）
func (s *ModelSelector) buildSelection(model *models.AiModel) (*ModelSelection, error) {
	var provider models.AiProvider
	if err := repository.DB.Where("id = ? AND status = 1", model.ProviderID).First(&provider).Error; err != nil {
		return nil, fmt.Errorf("provider %d not found or disabled", model.ProviderID)
	}
	var keys []models.ProviderApiKey
	if err := repository.DB.Where("ai_provider_id = ? AND status = ?", provider.ID, true).Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("load api keys: %w", err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("provider %q has no active api keys", provider.Code)
	}
	return &ModelSelection{
		Provider: &provider,
		Model:    model,
		ApiKey:   keys[rand.Intn(len(keys))].ApiKey,
	}, nil
}

// modelHasCapabilities 校验模型 capabilities JSON 是否包含要求的全部标签（要求的标签为空视为通过）
func modelHasCapabilities(m models.AiModel, required []string) bool {
	if len(required) == 0 {
		return true
	}
	var caps []string
	if len(m.Capabilities) > 0 {
		if err := json.Unmarshal(m.Capabilities, &caps); err != nil {
			return false
		}
	}
	have := make(map[string]bool, len(caps))
	for _, c := range caps {
		have[strings.ToLower(strings.TrimSpace(c))] = true
	}
	for _, r := range required {
		if !have[strings.ToLower(strings.TrimSpace(r))] {
			return false
		}
	}
	return true
}

// modelCapabilityCount 模型 capabilities 标签数（择优排序用）
func modelCapabilityCount(m models.AiModel) int {
	var caps []string
	if len(m.Capabilities) > 0 {
		if err := json.Unmarshal(m.Capabilities, &caps); err != nil {
			return 0
		}
	}
	return len(caps)
}

// ModelRequirements 技能 frontmatter 聚合出的模型需求声明
type ModelRequirements struct {
	Exec  *ModelRequirement // 主执行模型约束（Agent 循环模型，须对话型）
	Image *ModelRequirement // 非空 → 开放 generate_image 工具并按此要求选型
	Video *ModelRequirement // 非空 → 开放 generate_video 工具并按此要求选型
}

// subModelReqType 子需求未显式给 type 时的默认类型（image/video 语义由键名决定）
func subModelReqType(raw map[string]interface{}, defaultType string) ModelRequirement {
	req := ModelRequirement{Type: defaultType}
	if t, ok := raw["type"].(string); ok && strings.TrimSpace(t) != "" {
		req.Type = strings.TrimSpace(t)
	}
	if m, ok := raw["model"].(string); ok && strings.TrimSpace(m) != "" {
		req.Model = strings.TrimSpace(m)
	}
	if caps, ok := raw["capabilities"].([]interface{}); ok {
		for _, c := range caps {
			if cs, ok := c.(string); ok && strings.TrimSpace(cs) != "" {
				req.Capabilities = append(req.Capabilities, strings.TrimSpace(cs))
			}
		}
	}
	return req
}

// DeclaredModelRequirements 汇总多技能的 model_requirements 声明（顶层或 metadata.model_requirements）：
//   - exec：取第一个非空声明（多技能并存时不叠加，主模型只有一个）
//   - image/video：任一技能声明即开放（union 各自 capabilities）
func DeclaredModelRequirements(skills []models.Skill) *ModelRequirements {
	agg := &ModelRequirements{}
	for i := range skills {
		raw := frontmatterSection(skills[i].Frontmatter, "model_requirements")
		if raw == nil {
			continue
		}
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if agg.Exec == nil {
			if e, ok := m["exec"].(map[string]interface{}); ok {
				req := subModelReqType(e, "text")
				// 主执行模型跑 Agent 工具循环，function calling 是硬性要求：
				// 未声明时默认带上，声明了则尊重（能力匹配失败时由选型器报错暴露问题）
				if len(req.Capabilities) == 0 {
					req.Capabilities = []string{"function_calling"}
				}
				agg.Exec = &req
			}
		}
		if agg.Image == nil {
			if im, ok := m["image"].(map[string]interface{}); ok {
				req := subModelReqType(im, "image")
				agg.Image = &req
			}
		}
		if agg.Video == nil {
			if v, ok := m["video"].(map[string]interface{}); ok {
				req := subModelReqType(v, "video")
				agg.Video = &req
			}
		}
	}
	return agg
}

// DeclaredExecModel 技能 frontmatter 声明的主执行模型（exec.model，provider/code 全名）；无声明返回空。
// Dispatch 回退链的第二级：req.model > 技能声明 > 路由建议 > AGENT_ROUTER_MODEL
func DeclaredExecModel(skills []models.Skill) string {
	if agg := DeclaredModelRequirements(skills); agg.Exec != nil {
		return agg.Exec.Model
	}
	return ""
}

// frontmatterSection 从 frontmatter JSON 提取指定键（先顶层，后 metadata.*，与 outputs/dependencies 同约定）
func frontmatterSection(fm models.JSON, key string) interface{} {
	if len(fm) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(fm, &m); err != nil {
		return nil
	}
	if v, ok := m[key]; ok {
		return v
	}
	if meta, ok := m["metadata"].(map[string]interface{}); ok {
		if v, ok := meta[key]; ok {
			return v
		}
	}
	return nil
}

// 生成类子模型调用的超时与轮询约束
const (
	imageCallTimeout    = 3 * time.Minute  // 单次图片生成超时
	videoSubmitTimeout  = 1 * time.Minute  // 视频任务提交超时
	videoPollTimeout    = 10 * time.Minute // 视频任务轮询总上限
	videoPollInterval   = 5 * time.Second  // 视频任务轮询间隔
	maxGeneratedImageSz = 32 << 20         // 生成图片下载大小上限（32MB）
	maxGeneratedVideoSz = 512 << 20        // 生成视频下载大小上限（512MB）
	// 视频轮询中单次状态查询失败的容忍次数：云端任务已提交且在计费，
	// 网络抖动不应导致放弃轮询（等于丢资产白花钱），连续失败达到该值才判失败
	videoPollMaxQueryFailures = 5
	// 生成结果 URL 下载重试次数（含首次）：下载免费，失败让模型重新生成=重新花钱
	downloadRetryAttempts = 3
)

// subModelConstraints 平台侧已知的子模型硬约束（静态知识表，按模型 code 前缀匹配）。
// AutoMigrate 已跳过（复用 PHP 遗留库表结构），无法给 ai_models 加列，故以代码常量维护；
// 上游新增硬约束时在此追加一条即可。这些约束若不预注入，模型只能靠失败报错试出来，
// 每次试错都是一次真实的计费调用
var subModelConstraints = []struct {
	codePrefix string
	hint       string
}{
	{"doubao-seedream", "上游对 size 有最小像素限制：宽×高 ≥ 3686400 像素（如 1440x2560、2048x2048），过小（如 768x1024）会被上游直接拒绝"},
}

// subModelConstraintHints 汇总当前可用的指定类型模型的已知硬约束提示（逐条按模型 code 前缀标注）。
// 仅当该前缀存在活跃模型（type 匹配、status=1、provider 活跃且有 Key，与 Select 候选过滤口径一致）时才提示，
// 无匹配或查询失败返回空串（不注入内容，也不报错——约束提示是尽力而为的增强信息）
func subModelConstraintHints(reqType string) string {
	if reqType == "" {
		return ""
	}
	var providerIDs []uint
	if err := repository.DB.Model(&models.AiProvider{}).
		Where("status = 1 AND id IN (SELECT ai_provider_id FROM provider_api_keys WHERE status = 1)").
		Pluck("id", &providerIDs).Error; err != nil {
		return ""
	}
	var codes []string
	if err := repository.DB.Model(&models.AiModel{}).
		Where("type = ? AND status = 1 AND provider_id IN ?", reqType, providerIDs).
		Order("id").Pluck("code", &codes).Error; err != nil {
		return ""
	}
	var lines []string
	for _, c := range subModelConstraints {
		for _, code := range codes {
			if strings.HasPrefix(code, c.codePrefix) {
				lines = append(lines, fmt.Sprintf("  - 模型 %s*：%s", c.codePrefix, c.hint))
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}
