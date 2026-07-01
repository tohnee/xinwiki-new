package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type anthropicContentBlock struct {
	Type      string                  `json:"type"`
	Text      string                  `json:"text,omitempty"`
	ID        string                  `json:"id,omitempty"`
	Name      string                  `json:"name,omitempty"`
	Input     any                     `json:"input,omitempty"`
	Thinking  string                  `json:"thinking,omitempty"`
	Signature string                  `json:"signature,omitempty"`
	Data      string                  `json:"data,omitempty"`
	ToolUseID string                  `json:"tool_use_id,omitempty"`
	Content   []anthropicContentBlock `json:"content,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []anthropicContentBlock
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type anthropicRequest struct {
	Model       string                   `json:"model"`
	MaxTokens   int                      `json:"max_tokens"`
	Stream      bool                     `json:"stream,omitempty"`
	System      any                      `json:"system,omitempty"` // string or []anthropicContentBlock
	Messages    []anthropicMessage       `json:"messages"`
	Temperature *float64                 `json:"temperature,omitempty"`
	TopP        *float64                 `json:"top_p,omitempty"`
	Tools       []anthropicTool          `json:"tools,omitempty"`
	ToolChoice  any                      `json:"tool_choice,omitempty"`
	Thinking    *anthropicThinkingConfig `json:"thinking,omitempty"`
}

type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type anthropicStreamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index,omitempty"`
	Message *struct {
		ID         string                  `json:"id"`
		Type       string                  `json:"type"`
		Role       string                  `json:"role"`
		Content    []anthropicContentBlock `json:"content,omitempty"`
		StopReason string                  `json:"stop_reason,omitempty"`
		Usage      struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Delta *struct {
		Type         string `json:"type"`
		Text         string `json:"text,omitempty"`
		Thinking     string `json:"thinking,omitempty"`
		Signature    string `json:"signature,omitempty"`
		PartialJSON  string `json:"partial_json,omitempty"`
		StopReason   string `json:"stop_reason,omitempty"`
		StopSequence string `json:"stop_sequence,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Usage        *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
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

func (c *AnthropicChat) buildRequest(messages []Message, opts *ChatOptions) anthropicRequest {
	req := anthropicRequest{
		Model:     c.modelName,
		MaxTokens: 4096,
		Messages:  make([]anthropicMessage, 0, len(messages)),
	}
	if opts != nil {
		if opts.MaxTokens > 0 {
			req.MaxTokens = opts.MaxTokens
		} else if opts.MaxCompletionTokens > 0 {
			req.MaxTokens = opts.MaxCompletionTokens
		}
		if opts.Temperature > 0 {
			temperature := opts.Temperature
			req.Temperature = &temperature
		}
		if opts.TopP > 0 {
			topP := opts.TopP
			req.TopP = &topP
		}
		// 配置Thinking
		if opts.Thinking != nil && *opts.Thinking {
			req.Thinking = &anthropicThinkingConfig{
				Type:         "enabled",
				BudgetTokens: defaultThinkingBudget,
			}
			// Thinking模式下需要更高的MaxTokens
			if req.MaxTokens < defaultThinkingBudget+1024 {
				req.MaxTokens = defaultThinkingBudget + 4096
			}
		}
		// 配置Tools
		if len(opts.Tools) > 0 {
			req.Tools = make([]anthropicTool, 0, len(opts.Tools))
			for _, tool := range opts.Tools {
				req.Tools = append(req.Tools, anthropicTool{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					InputSchema: tool.Function.Parameters,
				})
			}
			// 配置tool_choice
			switch opts.ToolChoice {
			case "auto":
				req.ToolChoice = map[string]string{"type": "auto"}
			case "required":
				req.ToolChoice = map[string]string{"type": "any"}
			case "none":
				req.ToolChoice = map[string]string{"type": "none"}
			default:
				if opts.ToolChoice != "" {
					req.ToolChoice = map[string]any{
						"type": "tool",
						"name": opts.ToolChoice,
					}
				}
			}
		}
	}

	var systemContent []anthropicContentBlock
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				content = textFromMultiContent(msg.MultiContent)
			}
			if content != "" {
				systemContent = append(systemContent, anthropicContentBlock{
					Type: "text",
					Text: content,
				})
			}
		default:
			blocks := c.messageToAnthropicBlocks(msg)
			if len(blocks) > 0 {
				req.Messages = append(req.Messages, anthropicMessage{
					Role:    c.mapRole(msg.Role),
					Content: blocks,
				})
			}
		}
	}

	// 设置system prompt
	if len(systemContent) == 1 {
		req.System = systemContent[0].Text
	} else if len(systemContent) > 1 {
		req.System = systemContent
	}

	return req
}

// mapRole maps internal role names to Anthropic role names
func (c *AnthropicChat) mapRole(role string) string {
	switch role {
	case "tool":
		return "user"
	default:
		return role
	}
}

// messageToAnthropicBlocks converts an internal Message to Anthropic content blocks
func (c *AnthropicChat) messageToAnthropicBlocks(msg Message) []anthropicContentBlock {
	var blocks []anthropicContentBlock

	// 处理assistant的thinking内容
	if msg.Role == "assistant" && msg.ReasoningContent != "" {
		blocks = append(blocks, anthropicContentBlock{
			Type:     "thinking",
			Thinking: msg.ReasoningContent,
		})
	}

	// 处理文本内容
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		content = textFromMultiContent(msg.MultiContent)
	}
	if content != "" {
		blocks = append(blocks, anthropicContentBlock{
			Type: "text",
			Text: content,
		})
	}

	// 处理图片内容
	for _, part := range msg.MultiContent {
		if part.Type == "image_url" && part.ImageURL != nil {
			blocks = append(blocks, anthropicContentBlock{
				Type: "image",
				Data: part.ImageURL.URL,
			})
		}
	}

	// 处理assistant的tool calls
	if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var input any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			blocks = append(blocks, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	// 处理tool role的tool results
	if msg.Role == "tool" {
		resultContent := []anthropicContentBlock{{
			Type: "text",
			Text: msg.Content,
		}}
		blocks = append(blocks, anthropicContentBlock{
			Type:      "tool_result",
			ToolUseID: msg.ToolCallID,
			Content:   resultContent,
		})
	}

	return blocks
}

func textFromMultiContent(parts []MessageContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	textParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Type == "text" && strings.TrimSpace(part.Text) != "" {
			textParts = append(textParts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(textParts, "\n")
}

func (c *AnthropicChat) parseResponse(resp *anthropicResponse) *types.ChatResponse {
	var (
		textParts     []string
		thinkingParts []string
		toolCalls     []types.LLMToolCall
	)

	for _, part := range resp.Content {
		switch part.Type {
		case "text":
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		case "thinking":
			if part.Thinking != "" {
				thinkingParts = append(thinkingParts, part.Thinking)
			}
		case "tool_use":
			argsJSON, _ := json.Marshal(part.Input)
			toolCalls = append(toolCalls, types.LLMToolCall{
				ID:   part.ID,
				Type: "function",
				Function: types.FunctionCall{
					Name:      part.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	inputTokens := resp.Usage.InputTokens
	outputTokens := resp.Usage.OutputTokens
	cachedTokens := resp.Usage.CacheReadInputTokens
	cacheCreationTokens := resp.Usage.CacheCreationInputTokens

	return &types.ChatResponse{
		Content:          strings.Join(textParts, ""),
		ReasoningContent: strings.Join(thinkingParts, "\n"),
		ToolCalls:        toolCalls,
		FinishReason:     resp.StopReason,
		Usage: types.TokenUsage{
			PromptTokens:        inputTokens,
			CompletionTokens:    outputTokens,
			TotalTokens:         inputTokens + outputTokens,
			CacheCreationTokens: cacheCreationTokens,
			CacheReadTokens:     cachedTokens,
			CachedTokens:        cachedTokens + cacheCreationTokens,
		},
	}
}

func parseAnthropicSSE(reader io.Reader) (*types.ChatResponse, error) {
	sseReader := NewSSEReader(reader)
	var (
		contentParts  []string
		thinkingParts []string
		toolCalls     []types.LLMToolCall
		finishReason  string
		inputTokens   int
		outputTokens  int
		// cacheRead / cacheCreation are tracked separately (not collapsed into
		// a single cachedTokens) because Anthropic bills them at DIFFERENT
		// per-million rates (cache_read ≈ 0.1×, cache_creation ≈ 1.25×
		// input price). The combined `CachedTokens` is still emitted as
		// cache_read+cache_creation for compatibility with code paths that
		// only inspect that field (e.g. logs).
		cacheRead     int
		cacheCreation int
	)

	for {
		event, err := sseReader.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read SSE response: %w", err)
		}
		if event.Done {
			break
		}
		if len(event.Data) == 0 {
			continue
		}

		var streamEvent anthropicStreamEvent
		if err := json.Unmarshal(event.Data, &streamEvent); err != nil {
			return nil, fmt.Errorf("decode SSE response: %w", err)
		}
		if streamEvent.Error != nil && streamEvent.Error.Message != "" {
			return nil, fmt.Errorf("API stream error: %s", streamEvent.Error.Message)
		}
		if streamEvent.Message != nil {
			inputTokens = max(inputTokens, streamEvent.Message.Usage.InputTokens)
			outputTokens = max(outputTokens, streamEvent.Message.Usage.OutputTokens)
			cacheRead = max(cacheRead, streamEvent.Message.Usage.CacheReadInputTokens)
			cacheCreation = max(cacheCreation, streamEvent.Message.Usage.CacheCreationInputTokens)
			// 解析message_start中的tool_use块
			for _, block := range streamEvent.Message.Content {
				if block.Type == "tool_use" {
					argsJSON, _ := json.Marshal(block.Input)
					toolCalls = append(toolCalls, types.LLMToolCall{
						ID:   block.ID,
						Type: "function",
						Function: types.FunctionCall{
							Name:      block.Name,
							Arguments: string(argsJSON),
						},
					})
				}
			}
		}
		if streamEvent.ContentBlock != nil {
			if streamEvent.ContentBlock.Type == "tool_use" {
				argsJSON, _ := json.Marshal(streamEvent.ContentBlock.Input)
				toolCalls = append(toolCalls, types.LLMToolCall{
					ID:   streamEvent.ContentBlock.ID,
					Type: "function",
					Function: types.FunctionCall{
						Name:      streamEvent.ContentBlock.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
		if streamEvent.Delta != nil {
			switch streamEvent.Delta.Type {
			case "text_delta":
				if streamEvent.Delta.Text != "" {
					contentParts = append(contentParts, streamEvent.Delta.Text)
				}
			case "thinking_delta":
				if streamEvent.Delta.Thinking != "" {
					thinkingParts = append(thinkingParts, streamEvent.Delta.Thinking)
				}
			case "input_json_delta":
				// 追加到最后一个tool call的参数
				if len(toolCalls) > 0 && streamEvent.Delta.PartialJSON != "" {
					toolCalls[len(toolCalls)-1].Function.Arguments += streamEvent.Delta.PartialJSON
				}
			}
			if streamEvent.Delta.StopReason != "" {
				finishReason = streamEvent.Delta.StopReason
			}
		}
		if streamEvent.Usage != nil {
			inputTokens = max(inputTokens, streamEvent.Usage.InputTokens)
			outputTokens = max(outputTokens, streamEvent.Usage.OutputTokens)
			cacheRead = max(cacheRead, streamEvent.Usage.CacheReadInputTokens)
			cacheCreation = max(cacheCreation, streamEvent.Usage.CacheCreationInputTokens)
		}
	}

	return &types.ChatResponse{
		Content:          strings.Join(contentParts, ""),
		ReasoningContent: strings.Join(thinkingParts, "\n"),
		ToolCalls:        toolCalls,
		FinishReason:     finishReason,
		Usage: types.TokenUsage{
			PromptTokens:        inputTokens,
			CompletionTokens:    outputTokens,
			TotalTokens:         inputTokens + outputTokens,
			CacheReadTokens:     cacheRead,
			CacheCreationTokens: cacheCreation,
			CachedTokens:        cacheRead + cacheCreation,
		},
	}, nil
}

// processAnthropicStream reads SSE events from body and pushes StreamResponse
// values onto streamChan. body is the (idle-wrapped) HTTP response body; the
// caller owns closing the underlying http.Response.Body. An idle timeout
// surfaces as ErrIdleTimeout on the error channel; a cancelled context
// surfaces as the context error.
func processAnthropicStream(ctx context.Context, model string, resp *http.Response, body io.Reader, streamChan chan types.StreamResponse) {
	defer close(streamChan)
	defer resp.Body.Close()

	sseReader := NewSSEReader(body)
	var (
		usage        *types.TokenUsage
		finishReason string
		toolCalls    []types.LLMToolCall
	)

	for {
		event, err := sseReader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				logUsage(ctx, model, usage)
				streamChan <- types.StreamResponse{
					ResponseType: types.ResponseTypeAnswer,
					Content:      "",
					Done:         true,
					Usage:        usage,
					FinishReason: finishReason,
					ToolCalls:    toolCalls,
				}
			} else {
				// A client cancellation or idle-timeout is a clean stop, not a
				// provider error: surface a recognisable message but do not
				// emit a misleading "API stream error".
				msg := err.Error()
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					msg = "stream cancelled: " + err.Error()
				} else if errors.Is(err, ErrIdleTimeout) {
					msg = "stream stalled (idle timeout)"
				}
				streamChan <- types.StreamResponse{
					ResponseType: types.ResponseTypeError,
					Content:      msg,
					Done:         true,
				}
			}
			return
		}
		if event.Done {
			logUsage(ctx, model, usage)
			streamChan <- types.StreamResponse{
				ResponseType: types.ResponseTypeAnswer,
				Content:      "",
				Done:         true,
				Usage:        usage,
				FinishReason: finishReason,
				ToolCalls:    toolCalls,
			}
			return
		}
		if len(event.Data) == 0 {
			continue
		}

		var streamEvent anthropicStreamEvent
		if err := json.Unmarshal(event.Data, &streamEvent); err != nil {
			streamChan <- types.StreamResponse{
				ResponseType: types.ResponseTypeError,
				Content:      fmt.Sprintf("decode SSE response: %v", err),
				Done:         true,
			}
			return
		}
		if streamEvent.Error != nil && streamEvent.Error.Message != "" {
			streamChan <- types.StreamResponse{
				ResponseType: types.ResponseTypeError,
				Content:      streamEvent.Error.Message,
				Done:         true,
			}
			return
		}
		if streamEvent.Message != nil {
			usage = mergeAnthropicUsage(usage,
				streamEvent.Message.Usage.InputTokens,
				streamEvent.Message.Usage.OutputTokens,
				streamEvent.Message.Usage.CacheReadInputTokens,
				streamEvent.Message.Usage.CacheCreationInputTokens,
			)
		}
		if streamEvent.ContentBlock != nil {
			if streamEvent.ContentBlock.Type == "tool_use" {
				toolCalls = append(toolCalls, types.LLMToolCall{
					ID:   streamEvent.ContentBlock.ID,
					Type: "function",
					Function: types.FunctionCall{
						Name: streamEvent.ContentBlock.Name,
					},
				})
			}
		}
		if streamEvent.Delta != nil {
			if streamEvent.Delta.StopReason != "" {
				finishReason = streamEvent.Delta.StopReason
			}
			switch streamEvent.Delta.Type {
			case "text_delta":
				if streamEvent.Delta.Text != "" {
					streamChan <- types.StreamResponse{
						ResponseType: types.ResponseTypeAnswer,
						Content:      streamEvent.Delta.Text,
						Done:         false,
					}
				}
			case "thinking_delta":
				if streamEvent.Delta.Thinking != "" {
					streamChan <- types.StreamResponse{
						ResponseType: types.ResponseTypeThinking,
						Content:      streamEvent.Delta.Thinking,
						Done:         false,
					}
				}
			case "input_json_delta":
				if len(toolCalls) > 0 && streamEvent.Delta.PartialJSON != "" {
					toolCalls[len(toolCalls)-1].Function.Arguments += streamEvent.Delta.PartialJSON
				}
			}
		}
		if streamEvent.Usage != nil {
			usage = mergeAnthropicUsage(usage,
				streamEvent.Usage.InputTokens,
				streamEvent.Usage.OutputTokens,
				streamEvent.Usage.CacheReadInputTokens,
				streamEvent.Usage.CacheCreationInputTokens,
			)
		}
	}
}

func mergeAnthropicUsage(current *types.TokenUsage, inputTokens, outputTokens, cacheReadTokens, cacheCreateTokens int) *types.TokenUsage {
	if current == nil {
		current = &types.TokenUsage{}
	}
	current.PromptTokens = max(current.PromptTokens, inputTokens)
	current.CompletionTokens = max(current.CompletionTokens, outputTokens)
	current.CacheReadTokens = max(current.CacheReadTokens, cacheReadTokens)
	current.CacheCreationTokens = max(current.CacheCreationTokens, cacheCreateTokens)
	current.CachedTokens = max(current.CachedTokens, cacheReadTokens+cacheCreateTokens)
	current.TotalTokens = current.PromptTokens + current.CompletionTokens
	return current
}
