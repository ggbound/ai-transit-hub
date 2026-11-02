package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// V1 透传链路的核心可观测性指标（默认 Register，由 main.go 的 /metrics 暴露）

// V1RequestsTotal V1 请求计数。labels: type（请求类型 chat/embeddings/...）status（success|error）
var V1RequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "v1_requests_total",
		Help: "Total number of V1 passthrough requests by type and status",
	},
	[]string{"type", "status"},
)

// UpstreamDuration 上游 AI 提供商调用耗时（秒）。labels: provider type
var UpstreamDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "v1_upstream_duration_seconds",
		Help:    "Upstream AI provider call duration in seconds",
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600},
	},
	[]string{"provider", "type"},
)

// TokensTotal V1 请求 token 消耗。labels: model kind（prompt|completion|total）
var TokensTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "v1_tokens_total",
		Help: "Total tokens consumed by V1 requests, by model and kind",
	},
	[]string{"model", "kind"},
)

func init() {
	prometheus.MustRegister(V1RequestsTotal, UpstreamDuration, TokensTotal)
}
