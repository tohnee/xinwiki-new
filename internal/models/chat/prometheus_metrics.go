package chat

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

var (
	llmRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xinwiki_llm_requests_total",
			Help: "Total number of LLM requests by model, provider, endpoint type, and result.",
		},
		[]string{"model", "provider", "type", "result"},
	)

	llmRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "xinwiki_llm_request_duration_seconds",
			Help:    "LLM request latency in seconds by model and type.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"model", "provider", "type", "result"},
	)

	llmTokensTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xinwiki_llm_tokens_total",
			Help: "Total tokens consumed by LLM requests, labeled by model, provider, token type (prompt/completion/cache_read/cache_write).",
		},
		[]string{"model", "provider", "token_type"},
	)

	llmStreamingChunksTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "xinwiki_llm_streaming_chunks_total",
			Help: "Total streaming chunks received for stream requests.",
		},
		[]string{"model", "provider", "result"},
	)

	llmActiveRequests = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "xinwiki_llm_active_requests",
			Help: "Current number of in-flight LLM requests by model.",
		},
		[]string{"model", "type"},
	)
)

// prometheusChat wraps a Chat implementation and records Prometheus metrics
// for every Chat/ChatStream call. It follows the same decorator pattern as
// langfuseChat and llmDebugChat.
type prometheusChat struct {
	inner    Chat
	provider string
}

// WithPrometheusMetrics wraps a Chat model to record Prometheus metrics
// for each request. Provider is the provider identifier (e.g. "anthropic","openai","ollama").
func WithPrometheusMetrics(inner Chat, provider string) Chat {
	return &prometheusChat{inner: inner, provider: provider}
}

func (p *prometheusChat) GetModelName() string { return p.inner.GetModelName() }
func (p *prometheusChat) GetModelID() string   { return p.inner.GetModelID() }

func (p *prometheusChat) Chat(ctx context.Context, messages []Message, opts *ChatOptions) (*types.ChatResponse, error) {
	model := p.inner.GetModelName()
	llmActiveRequests.WithLabelValues(model, "chat").Inc()
	start := time.Now()

	resp, err := p.inner.Chat(ctx, messages, opts)

	duration := time.Since(start).Seconds()
	result := "success"
	if err != nil {
		result = "error"
	}
	llmRequestsTotal.WithLabelValues(model, p.provider, "chat", result).Inc()
	llmRequestDuration.WithLabelValues(model, p.provider, "chat", result).Observe(duration)
	llmActiveRequests.WithLabelValues(model, "chat").Dec()

	if resp != nil {
		recordTokenUsage(model, p.provider, resp.Usage)
	}
	return resp, err
}

func (p *prometheusChat) ChatStream(ctx context.Context, messages []Message, opts *ChatOptions) (<-chan types.StreamResponse, error) {
	model := p.inner.GetModelName()
	llmActiveRequests.WithLabelValues(model, "stream").Inc()
	start := time.Now()

	innerCh, err := p.inner.ChatStream(ctx, messages, opts)
	if err != nil {
		duration := time.Since(start).Seconds()
		llmRequestsTotal.WithLabelValues(model, p.provider, "stream", "error").Inc()
		llmRequestDuration.WithLabelValues(model, p.provider, "stream", "error").Observe(duration)
		llmActiveRequests.WithLabelValues(model, "stream").Dec()
		return nil, err
	}

	outCh := make(chan types.StreamResponse, 32)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorWithFields(ctx, fmt.Errorf("prometheus_metrics stream copier panicked: %v", r),
					logger.Fields{"model": model, "provider": p.provider, "stacktrace": string(debug.Stack())})
				llmActiveRequests.WithLabelValues(model, "stream").Dec()
			}
			close(outCh)
		}()
		result := "success"
		chunks := 0
		var finalUsage *types.TokenUsage
		for resp := range innerCh {
			chunks++
			if resp.Done && resp.FinishReason == "error" {
				result = "error"
			}
			if resp.Usage != nil {
				finalUsage = resp.Usage
			}
			outCh <- resp
		}
		duration := time.Since(start).Seconds()
		llmRequestsTotal.WithLabelValues(model, p.provider, "stream", result).Inc()
		llmRequestDuration.WithLabelValues(model, p.provider, "stream", result).Observe(duration)
		llmStreamingChunksTotal.WithLabelValues(model, p.provider, result).Add(float64(chunks))
		llmActiveRequests.WithLabelValues(model, "stream").Dec()
		if finalUsage != nil {
			recordTokenUsage(model, p.provider, *finalUsage)
		}
	}()
	return outCh, nil
}

func recordTokenUsage(model, provider string, usage types.TokenUsage) {
	if usage.PromptTokens > 0 {
		llmTokensTotal.WithLabelValues(model, provider, "prompt").Add(float64(usage.PromptTokens))
	}
	if usage.CompletionTokens > 0 {
		llmTokensTotal.WithLabelValues(model, provider, "completion").Add(float64(usage.CompletionTokens))
	}
	if usage.TotalTokens > 0 {
		llmTokensTotal.WithLabelValues(model, provider, "total").Add(float64(usage.TotalTokens))
	}
	if usage.CacheReadTokens > 0 {
		llmTokensTotal.WithLabelValues(model, provider, "cache_read").Add(float64(usage.CacheReadTokens))
	}
	if usage.CacheCreationTokens > 0 {
		llmTokensTotal.WithLabelValues(model, provider, "cache_write").Add(float64(usage.CacheCreationTokens))
	}
}
