package ai

import (
	"github.com/your-org/ai-transit-hub/internal/models"
)

// OpenAIAdapter OpenAI 兼容适配器
// 公共能力（请求构建、认证、透传、token 统计等）全部继承自嵌入的 *GenericAdapter；
// 出现厂商差异时在此覆写对应方法即可（参考 doubao/qwen 适配器）。
type OpenAIAdapter struct {
	*GenericAdapter
}

// NewOpenAIAdapter 创建 OpenAI 适配器
func NewOpenAIAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *OpenAIAdapter {
	providerKey := &models.ProviderApiKey{
		AiProviderID: provider.ID,
		ApiKey:       apiKey,
		Status:       1,
	}

	return &OpenAIAdapter{
		GenericAdapter: &GenericAdapter{
			client:      upstreamHTTPClient,
			provider:    provider,
			apiKey:      providerKey,
			modelConfig: modelConfig,
		},
	}
}
