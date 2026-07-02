package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/models/provider"
	"github.com/Tencent/XinWiki/internal/types"
	secutils "github.com/Tencent/XinWiki/internal/utils"
)

const (
	anthropicVersion      = "2023-06-01"
	anthropicBetaVersion  = "2024-02-29" // 支持extended thinking和tool use
	anthropicThinkingBeta = "interleaved-thinking-2025-05-14"
	defaultThinkingBudget = 16000 // 默认思考token预算
	// defaultStreamIdleTimeout is the per-SSE-event idle watchdog: if no event
	// arrives within this window during a stream, the read is aborted with
	// ErrIdleTimeout. It catches a stalled provider stream (200 + partial body
	// + hang) that the overall stream deadline (defaultStreamTimeout) would
	// otherwise let block a goroutine for the full window. Override via
	// ChatConfig.StreamIdleTimeout.
	defaultStreamIdleTimeout = 120 * time.Second
)

// needsBetaFeatures checks if the request requires beta headers
func (c *AnthropicChat) needsBetaFeatures(opts *ChatOptions) bool {
	if opts == nil {
		return false
	}
	if opts.Thinking != nil && *opts.Thinking {
		return true
	}
	if len(opts.Tools) > 0 {
		return true
	}
	return false
}

// anthropicHeaders sets the required HTTP headers including beta headers when needed
func (c *AnthropicChat) anthropicHeaders(httpReq *http.Request, opts *ChatOptions) {
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	if c.needsBetaFeatures(opts) {
		httpReq.Header.Set("anthropic-version", anthropicBetaVersion)
		httpReq.Header.Set("anthropic-beta", anthropicThinkingBeta)
	} else {
		httpReq.Header.Set("anthropic-version", anthropicVersion)
	}
	secutils.ApplyCustomHeaders(httpReq, c.customHeaders)
}

type AnthropicChat struct {
	modelName     string
	modelID       string
	baseURL       string
	apiKey        string
	customHeaders map[string]string
	// breaker protects against cascading provider failures; nil disables it
	// (default) so existing construction paths keep working.
	breaker *CircuitBreaker
	// streamIdleTimeout bounds how long a streaming read may block waiting for
	// the next SSE event before being declared stalled. Zero falls back to
	// defaultStreamIdleTimeout.
	streamIdleTimeout time.Duration
}

func NewAnthropicChat(config *ChatConfig) (*AnthropicChat, error) {
	if config.BaseURL != "" {
		if err := secutils.ValidateURLForSSRF(config.BaseURL); err != nil {
			return nil, fmt.Errorf("baseURL SSRF check failed: %w", err)
		}
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("Anthropic provider: API key is required")
	}

	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = provider.AnthropicBaseURL
	}

	// Attach the host-shared circuit breaker so a cascade of Anthropic
	// provider failures (5xx storm, TCP resets) trips once for the whole
	// fleet rather than one-instance-at-a-time. WithCircuitBreaker remains
	// as a per-instance override (e.g. tests pinning a tighter threshold)
	// and takes effect if called after construction.
	return &AnthropicChat{
		modelName:         config.ModelName,
		modelID:           config.ModelID,
		baseURL:           baseURL,
		apiKey:            config.APIKey,
		customHeaders:     config.CustomHeaders,
		streamIdleTimeout: defaultStreamIdleTimeout,
		breaker:           sharedBreakerForURL(baseURL),
	}, nil
}

// WithCircuitBreaker attaches a circuit breaker so consecutive provider
// failures trip the circuit and fast-fail subsequent calls (ErrCircuitOpen)
// instead of cascading. Nil (the default) leaves the breaker disabled.
func (c *AnthropicChat) WithCircuitBreaker(cb *CircuitBreaker) *AnthropicChat {
	c.breaker = cb
	return c
}

// WithStreamIdleTimeout overrides the per-SSE-event idle watchdog. A stalled
// stream that stops emitting events for this duration is aborted with
// ErrIdleTimeout instead of blocking until the overall stream deadline.
func (c *AnthropicChat) WithStreamIdleTimeout(d time.Duration) *AnthropicChat {
	if d > 0 {
		c.streamIdleTimeout = d
	}
	return c
}

func (c *AnthropicChat) Chat(ctx context.Context, messages []Message, opts *ChatOptions) (*types.ChatResponse, error) {
	reqBody := c.buildRequest(messages, opts)
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := withLLMTimeout(ctx, defaultChatTimeout)
	defer cancel()

	endpoint := c.endpoint()
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		return nil, fmt.Errorf("endpoint SSRF check failed: %w", err)
	}

	var result *types.ChatResponse
	callErr := c.breaker.Call(ctx, func(ctx context.Context) error {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		c.anthropicHeaders(httpReq, opts)

		resp, err := rawHTTPClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			chatResp, err := parseAnthropicSSE(bytes.NewReader(body))
			if err != nil {
				return err
			}
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, chatResp.Content)
			}
			logUsage(ctx, c.modelName, &chatResp.Usage)
			result = chatResp
			return nil
		}

		var chatResp anthropicResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			if chatResp.Error != nil && chatResp.Error.Message != "" {
				return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, chatResp.Error.Message)
			}
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		result = c.parseResponse(&chatResp)
		logUsage(ctx, c.modelName, &result.Usage)
		return nil
	})
	if callErr != nil {
		return nil, callErr
	}
	return result, nil
}

func (c *AnthropicChat) ChatStream(ctx context.Context, messages []Message, opts *ChatOptions) (<-chan types.StreamResponse, error) {
	reqBody := c.buildRequest(messages, opts)
	reqBody.Stream = true
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Apply the overall stream deadline as a hard ceiling (only when the caller
	// did not set one). The per-event idle watchdog (IdleReader) additionally
	// catches a stream that connects, emits a partial body, then stalls.
	streamCtx, cancel := withLLMTimeout(ctx, defaultStreamTimeout)

	endpoint := c.endpoint()
	if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
		cancel()
		return nil, fmt.Errorf("endpoint SSRF check failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	c.anthropicHeaders(httpReq, opts)

	// The HTTP send itself is protected by the breaker so a down provider
	// fast-fails. The streaming body is consumed outside the breaker (the
	// breaker.Call contract is synchronous; the stream goroutine runs after).
	var resp *http.Response
	callErr := c.breaker.Call(streamCtx, func(ctx context.Context) error {
		r, err := rawHTTPClient.Do(httpReq)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		resp = r
		return nil
	})
	if callErr != nil {
		cancel()
		return nil, callErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Wrap the body so a stalled stream (no SSE event within the idle window)
	// aborts instead of blocking the stream goroutine for the full deadline.
	bodyReader := NewIdleReaderContext(streamCtx, resp.Body, c.streamIdleTimeout)

	streamChan := make(chan types.StreamResponse)
	go func() {
		defer cancel()
		processAnthropicStream(streamCtx, c.modelName, resp, bodyReader, streamChan)
	}()
	return streamChan, nil
}

func (c *AnthropicChat) GetModelName() string {
	return c.modelName
}

func (c *AnthropicChat) GetModelID() string {
	return c.modelID
}

func (c *AnthropicChat) endpoint() string {
	baseURL := strings.TrimRight(c.baseURL, "/")
	if isAnthropicMessagesEndpoint(baseURL) {
		return baseURL
	}
	if isAnthropicVersionedBaseURL(baseURL) {
		return baseURL + "/messages"
	}
	return baseURL + "/v1/messages"
}

func isAnthropicMessagesEndpoint(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	return strings.HasSuffix(path, "/messages")
}

func isAnthropicVersionedBaseURL(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	return strings.HasSuffix(path, "/v1") || strings.HasSuffix(path, "/v1beta")
}
