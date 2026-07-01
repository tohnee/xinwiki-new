package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/Tencent/XinWiki/internal/types"
)

type mockChatForMetrics struct {
	name       string
	chatDelay  time.Duration
	chatErr    error
	chatResp   *types.ChatResponse
	streamCh   chan types.StreamResponse
	streamErr  error
}

func (m *mockChatForMetrics) GetModelName() string { return m.name }
func (m *mockChatForMetrics) GetModelID() string   { return "test-model-id" }

func (m *mockChatForMetrics) Chat(_ context.Context, _ []Message, _ *ChatOptions) (*types.ChatResponse, error) {
	if m.chatDelay > 0 {
		time.Sleep(m.chatDelay)
	}
	return m.chatResp, m.chatErr
}

func (m *mockChatForMetrics) ChatStream(_ context.Context, _ []Message, _ *ChatOptions) (<-chan types.StreamResponse, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return m.streamCh, nil
}

func TestPrometheusChat_NonStream_Success(t *testing.T) {
	mock := &mockChatForMetrics{
		name: "test-metric-model",
		chatResp: &types.ChatResponse{
			Content: "hello",
			Usage: types.TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				CacheReadTokens:  30,
			},
		},
	}
	wrapped := WithPrometheusMetrics(mock, "test-provider")

	resp, err := wrapped.Chat(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected hello, got %s", resp.Content)
	}

	// Verify counter was incremented
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather error: %v", err)
	}
	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_llm_requests_total" {
			for _, m := range mf.GetMetric() {
				labels := labelMapForChat(m)
				if labels["model"] == "test-metric-model" && labels["result"] == "success" && labels["type"] == "chat" {
					found = true
					if m.GetCounter().GetValue() < 1 {
						t.Errorf("expected counter >= 1, got %f", m.GetCounter().GetValue())
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_llm_requests_total success counter not found")
	}

	// Verify token metrics
	foundTokens := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_llm_tokens_total" {
			for _, m := range mf.GetMetric() {
				labels := labelMapForChat(m)
				if labels["model"] == "test-metric-model" && labels["token_type"] == "prompt" {
					foundTokens = true
					if m.GetCounter().GetValue() < 100 {
						t.Errorf("expected prompt tokens >= 100, got %f", m.GetCounter().GetValue())
					}
				}
			}
		}
	}
	if !foundTokens {
		t.Fatal("xinwiki_llm_tokens_total prompt counter not found")
	}
}

func TestPrometheusChat_NonStream_Error(t *testing.T) {
	mock := &mockChatForMetrics{
		name:    "test-error-model",
		chatErr: context.DeadlineExceeded,
	}
	wrapped := WithPrometheusMetrics(mock, "test-provider")

	_, err := wrapped.Chat(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	metrics, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_llm_requests_total" {
			for _, m := range mf.GetMetric() {
				labels := labelMapForChat(m)
				if labels["model"] == "test-error-model" && labels["result"] == "error" {
					if m.GetCounter().GetValue() < 1 {
						t.Errorf("expected error counter >= 1, got %f", m.GetCounter().GetValue())
					}
					return
				}
			}
		}
	}
	t.Fatal("error counter not found")
}

func TestPrometheusChat_Stream_Success(t *testing.T) {
	ch := make(chan types.StreamResponse, 3)
	ch <- types.StreamResponse{Content: "hel"}
	ch <- types.StreamResponse{Content: "lo", Usage: &types.TokenUsage{PromptTokens: 80, CompletionTokens: 20, TotalTokens: 100}}
	ch <- types.StreamResponse{Done: true}
	close(ch)

	mock := &mockChatForMetrics{name: "test-stream-model", streamCh: ch}
	wrapped := WithPrometheusMetrics(mock, "test-provider")

	streamCh, err := wrapped.ChatStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}

	var chunks int
	for range streamCh {
		chunks++
	}
	if chunks < 3 {
		t.Fatalf("expected >=3 chunks, got %d", chunks)
	}

	// Wait a tiny bit for goroutine to finish recording
	time.Sleep(50 * time.Millisecond)

	metrics, _ := prometheus.DefaultGatherer.Gather()
	found := false
	for _, mf := range metrics {
		if mf.GetName() == "xinwiki_llm_streaming_chunks_total" {
			for _, m := range mf.GetMetric() {
				labels := labelMapForChat(m)
				if strings.HasPrefix(labels["model"], "test-stream-model") {
					found = true
					if m.GetCounter().GetValue() < 3 {
						t.Errorf("expected chunks >= 3, got %f", m.GetCounter().GetValue())
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("xinwiki_llm_streaming_chunks_total not found")
	}
}

func TestPrometheusChat_Stream_Error(t *testing.T) {
	mock := &mockChatForMetrics{
		name:      "test-stream-err",
		streamErr: context.Canceled,
	}
	wrapped := WithPrometheusMetrics(mock, "test-provider")

	_, err := wrapped.ChatStream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected stream error")
	}
}

func labelMapForChat(m *dto.Metric) map[string]string {
	lm := make(map[string]string, len(m.GetLabel()))
	for _, lp := range m.GetLabel() {
		lm[lp.GetName()] = lp.GetValue()
	}
	return lm
}
