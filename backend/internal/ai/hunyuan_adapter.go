package ai

import (
	"github.com/your-org/ai-transit-hub/internal/models"
)

// HunyuanAdapter 混元适配器
// 公共能力（请求构建、认证、透传、token 统计等）全部继承自嵌入的 *GenericAdapter；
// 出现厂商差异时在此覆写对应方法即可（参考 doubao/qwen 适配器）。
type HunyuanAdapter struct {
	*GenericAdapter
}

// NewHunyuanAdapter 创建混元适配器
func NewHunyuanAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *HunyuanAdapter {
	providerKey := &models.ProviderApiKey{
		AiProviderID: provider.ID,
		ApiKey:       apiKey,
		Status:       1,
	}

	return &HunyuanAdapter{
		GenericAdapter: &GenericAdapter{
			client:      upstreamHTTPClient,
			provider:    provider,
			apiKey:      providerKey,
			modelConfig: modelConfig,
		},
	}
}
