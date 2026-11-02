package ai

import (
	"github.com/your-org/ai-transit-hub/internal/models"
)

// ZhipuAdapter 智谱 AI 适配器
// 公共能力（请求构建、认证、透传、token 统计等）全部继承自嵌入的 *GenericAdapter；
// 出现厂商差异时在此覆写对应方法即可（参考 doubao/qwen 适配器）。
type ZhipuAdapter struct {
	*GenericAdapter
}

// NewZhipuAdapter 创建智谱 AI 适配器
func NewZhipuAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *ZhipuAdapter {
	providerKey := &models.ProviderApiKey{
		AiProviderID: provider.ID,
		ApiKey:       apiKey,
		Status:       1,
	}

	return &ZhipuAdapter{
		GenericAdapter: &GenericAdapter{
			client:      upstreamHTTPClient,
			provider:    provider,
			apiKey:      providerKey,
			modelConfig: modelConfig,
		},
	}
}
