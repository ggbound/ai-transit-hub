package ai

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/models"
	"github.com/your-org/ai-transit-hub/internal/repository"
)

// AdapterFactory 适配器工厂，根据 adapter_class 创建对应的适配器
type AdapterFactory struct{}

// NewAdapterFactory 创建适配器工厂实例
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{}
}

// AdapterClass 定义适配器类型常量
const (
	AdapterClassGeneric  = "generic"
	AdapterClassOpenAI   = "openai"
	AdapterClassDoubao   = "doubao"
	AdapterClassVolc     = "volcengine"
	AdapterClassQwen     = "qwen"
	AdapterClassQianwen  = "qianwen"
	AdapterClassZhipu    = "zhipu"
	AdapterClassDeepSeek = "deepseek"
	AdapterClassClaude   = "claude"
	AdapterClassHunyuan  = "hunyuan"
)

// CreateAdapter 根据 provider 的 AdapterClass 创建对应的适配器
// provider: AI提供商信息（包含 adapter_class 字段）
// modelConfig: 模型配置信息
func (f *AdapterFactory) CreateAdapter(provider *models.AiProvider, modelConfig *models.AiModel) (Adapter, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	// 获取 API Key
	apiKey, err := f.getRandomApiKey(provider.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	// 根据 AdapterClass 创建对应的适配器
	adapterClass := strings.ToLower(strings.TrimSpace(provider.AdapterClass))

	logger.Info("Creating adapter: provider=%s, adapter_class=%s, model=%s", provider.Code, adapterClass, modelConfig.Code)

	// 兼容处理 PHP 的命名空间格式
	adapterClass = f.normalizeAdapterClass(adapterClass)

	if adapter := createByAdapterClass(adapterClass, provider, apiKey, modelConfig); adapter != nil {
		return adapter, nil
	}

	// 未知适配器类型，尝试根据 provider code 匹配
	logger.Info("Unknown adapter class, trying to match by provider code: adapter_class=%s, provider_code=%s", adapterClass, provider.Code)
	return f.matchAdapterByProviderCode(provider, apiKey, modelConfig)
}

// createByAdapterClass 按归一化后的 adapter_class 创建对应适配器。
// 未识别的适配器类型返回 nil，由调用方决定回退策略
// （CreateAdapter 按 provider code 匹配，CreateAdapterWithApiKey 直接回退 generic）。
func createByAdapterClass(adapterClass string, provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) Adapter {
	switch adapterClass {
	case AdapterClassDoubao, AdapterClassVolc, "doubao/volcengine":
		return NewDoubaoAdapter(provider, apiKey, modelConfig)
	case AdapterClassQwen, AdapterClassQianwen:
		return NewQwenAdapter(provider, apiKey, modelConfig)
	case AdapterClassOpenAI:
		return NewOpenAIAdapter(provider, apiKey, modelConfig)
	case AdapterClassDeepSeek:
		return NewDeepSeekAdapter(provider, apiKey, modelConfig)
	case AdapterClassClaude:
		return NewClaudeAdapter(provider, apiKey, modelConfig)
	case AdapterClassHunyuan:
		return NewHunyuanAdapter(provider, apiKey, modelConfig)
	case AdapterClassZhipu:
		return NewZhipuAdapter(provider, apiKey, modelConfig)
	case AdapterClassGeneric, "":
		// 默认使用通用适配器
		return NewGenericAdapter(provider, apiKey, modelConfig)
	default:
		return nil
	}
}

// NormalizedAdapterClass 返回归一化后的适配器类名（兼容 PHP 命名空间格式，
// 如 App\Services\AI\Adapters\DoubaoAdapter -> doubao），并将 volcengine
// 视为 doubao 同一适配器家族（与 CreateAdapterWithApiKey 的分支一致）。
// 供能力元数据等需要按适配器区分行为的消费方使用。
func (f *AdapterFactory) NormalizedAdapterClass(adapterClass string) string {
	normalized := f.normalizeAdapterClass(strings.ToLower(strings.TrimSpace(adapterClass)))
	switch normalized {
	case AdapterClassVolc, "doubao/volcengine":
		return AdapterClassDoubao
	}
	return normalized
}

// normalizeAdapterClass 标准化适配器类名，支持 PHP 命名空间格式
func (f *AdapterFactory) normalizeAdapterClass(adapterClass string) string {
	// 处理 PHP 命名空间格式，如 App\Services\AI\Adapters\GenericAdapter -> generic
	if strings.HasPrefix(adapterClass, "app\\services\\ai\\adapters\\") {
		name := strings.TrimPrefix(adapterClass, "app\\services\\ai\\adapters\\")
		name = strings.TrimSuffix(name, "adapter")
		return strings.ToLower(name)
	}
	// 处理 App\Services\AI\Adapters\GenericAdapter -> generic (不区分大小写)
	if strings.HasSuffix(strings.ToLower(adapterClass), "adapter") {
		parts := strings.Split(adapterClass, "\\")
		if len(parts) > 0 {
			name := parts[len(parts)-1]
			name = strings.TrimSuffix(strings.ToLower(name), "adapter")
			return name
		}
	}
	return adapterClass
}

// matchAdapterByProviderCode 根据提供商代码匹配适配器
func (f *AdapterFactory) matchAdapterByProviderCode(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) (Adapter, error) {
	providerCode := strings.ToLower(provider.Code)

	switch {
	case strings.Contains(providerCode, "doubao") || strings.Contains(providerCode, "volc"):
		return NewDoubaoAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "qwen") || strings.Contains(providerCode, "qianwen"):
		return NewQwenAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "openai"):
		return NewOpenAIAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "deepseek"):
		return NewDeepSeekAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "claude") || strings.Contains(providerCode, "anthropic"):
		return NewClaudeAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "hunyuan"):
		return NewHunyuanAdapter(provider, apiKey, modelConfig), nil
	case strings.Contains(providerCode, "zhipu"):
		return NewZhipuAdapter(provider, apiKey, modelConfig), nil
	default:
		// 默认使用通用适配器
		logger.Info("Using generic adapter as fallback: provider_code=%s", providerCode)
		return NewGenericAdapter(provider, apiKey, modelConfig), nil
	}
}

// getRandomApiKey 获取随机可用的 API Key，在多 Key 之间实现负载分摊
func (f *AdapterFactory) getRandomApiKey(providerID uint) (string, error) {
	var keys []models.ProviderApiKey
	if err := repository.DB.Where("ai_provider_id = ? AND status = ?", providerID, true).Find(&keys).Error; err != nil {
		return "", err
	}

	if len(keys) == 0 {
		return "", fmt.Errorf("no active API keys found for provider ID %d", providerID)
	}

	// 在可用 Key 中随机选择，实现多 Key 之间的负载分摊
	return keys[rand.Intn(len(keys))].ApiKey, nil
}

// CreateAdapterWithApiKey 使用指定的 API Key 创建适配器
func (f *AdapterFactory) CreateAdapterWithApiKey(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) (Adapter, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider cannot be nil")
	}

	// 根据 AdapterClass 创建适配器
	adapterClass := strings.ToLower(strings.TrimSpace(provider.AdapterClass))

	// 兼容处理 PHP 的命名空间格式
	adapterClass = f.normalizeAdapterClass(adapterClass)

	if adapter := createByAdapterClass(adapterClass, provider, apiKey, modelConfig); adapter != nil {
		return adapter, nil
	}

	// 未识别的适配器类型，直接使用通用适配器
	return NewGenericAdapter(provider, apiKey, modelConfig), nil
}

// GetAvailableAdapterClasses 获取所有可用的适配器类型
func (f *AdapterFactory) GetAvailableAdapterClasses() []map[string]string {
	return []map[string]string{
		{"value": AdapterClassGeneric, "label": "通用适配器（推荐）"},
		{"value": AdapterClassOpenAI, "label": "OpenAI 适配器"},
		{"value": AdapterClassDoubao, "label": "豆包/火山引擎适配器"},
		{"value": AdapterClassQwen, "label": "通义千问适配器"},
		{"value": AdapterClassZhipu, "label": "智谱 AI 适配器"},
		{"value": AdapterClassDeepSeek, "label": "DeepSeek 适配器"},
		{"value": AdapterClassClaude, "label": "Claude 适配器"},
		{"value": AdapterClassHunyuan, "label": "混元适配器"},
	}
}
