package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// AnthropicRuntime is a native Runtime implementation using the official
// anthropic-sdk-go library. It supports extended thinking with explicit
// budgets, prompt caching (cache_control), and the modern Messages API
// (including interleaved thinking + tool_use blocks as shipped in 2025).
type AnthropicRuntime struct {
	client  anthropic.Client
	model   anthropic.Model
	name    string
	modelID string
}

// NewAnthropicRuntime builds a Runtime backed by anthropic-sdk-go.
func NewAnthropicRuntime(cfg *chat.ChatConfig) (Runtime, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("runtime/anthropic: API key is required")
	}
	if cfg.ModelName == "" {
		return nil, errors.New("runtime/anthropic: model name is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.ExtraConfig != nil {
		if v := cfg.ExtraConfig["anthropic_version"]; v != "" {
			opts = append(opts, option.WithHeader("anthropic-version", v))
		}
		if betas := cfg.ExtraConfig["anthropic_beta"]; betas != "" {
			opts = append(opts, option.WithHeader("anthropic-beta", betas))
		}
	}
	for k, v := range cfg.CustomHeaders {
		opts = append(opts, option.WithHeader(k, v))
	}
	client := anthropic.NewClient(opts...)

	return &AnthropicRuntime{
		client:  client,
		model:   anthropic.Model(cfg.ModelName),
		name:    "anthropic-sdk",
		modelID: cfg.ModelID,
	}, nil
}

func (r *AnthropicRuntime) Name() string      { return r.name }
func (r *AnthropicRuntime) ModelName() string { return string(r.model) }
func (r *AnthropicRuntime) ModelID() string   { return r.modelID }

func (r *AnthropicRuntime) Chat(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (*types.ChatResponse, error) {
	req, err := r.buildParams(messages, opts)
	if err != nil {
		return nil, err
	}
	msg, err := r.client.Messages.New(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("runtime/anthropic: %w", err)
	}
	return responseToChatResponse(msg), nil
}

func (r *AnthropicRuntime) ChatStream(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (<-chan types.StreamResponse, error) {
	req, err := r.buildParams(messages, opts)
	if err != nil {
		return nil, err
	}
	ch := make(chan types.StreamResponse, 64)
	go func() {
		defer close(ch)
		stream := r.client.Messages.NewStreaming(ctx, req)
		var (
			acc     anthropic.Message
			curTool *types.LLMToolCall
		)
		for stream.Next() {
			evt := stream.Current()
			if err := acc.Accumulate(evt); err != nil {
				ch <- types.StreamResponse{ResponseType: types.ResponseTypeError, Content: "accumulate: " + err.Error(), Done: true}
				return
			}
			switch event := evt.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				if cb, ok := event.ContentBlock.AsAny().(anthropic.ToolUseBlock); ok {
					tc := types.LLMToolCall{
						ID:       cb.ID,
						Type:     "function",
						Function: types.FunctionCall{Name: cb.Name},
					}
					curTool = &tc
					ch <- types.StreamResponse{
						ResponseType: types.ResponseTypeToolCall,
						ToolCalls:    []types.LLMToolCall{tc},
					}
				}
			case anthropic.ContentBlockDeltaEvent:
				switch d := event.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					if d.Text != "" {
						ch <- types.StreamResponse{ResponseType: types.ResponseTypeAnswer, Content: d.Text}
					}
				case anthropic.ThinkingDelta:
					if d.Thinking != "" {
						ch <- types.StreamResponse{ResponseType: types.ResponseTypeThinking, Content: d.Thinking}
					}
				case anthropic.InputJSONDelta:
					if curTool != nil && d.PartialJSON != "" {
						curTool.Function.Arguments += d.PartialJSON
					}
				}
			}
		}
		if err := stream.Err(); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			ch <- types.StreamResponse{ResponseType: types.ResponseTypeError, Content: err.Error(), Done: true}
			return
		}

		var (
			text        strings.Builder
			toolCalls   []types.LLMToolCall
			finishReason string
		)
		for _, b := range acc.Content {
			switch cb := b.AsAny().(type) {
			case anthropic.TextBlock:
				text.WriteString(cb.Text)
			case anthropic.ToolUseBlock:
				toolCalls = append(toolCalls, types.LLMToolCall{
					ID:       cb.ID,
					Type:     "function",
					Function: types.FunctionCall{Name: cb.Name, Arguments: string(cb.Input)},
				})
			}
		}
		if acc.StopReason != "" {
			finishReason = string(acc.StopReason)
		}
		ch <- types.StreamResponse{
			ResponseType: types.ResponseTypeAnswer,
			Content:      text.String(),
			Done:         true,
			FinishReason: finishReason,
			Usage: &types.TokenUsage{
				PromptTokens:        int(acc.Usage.InputTokens),
				CompletionTokens:    int(acc.Usage.OutputTokens),
				CachedTokens:        int(acc.Usage.CacheReadInputTokens + acc.Usage.CacheCreationInputTokens),
				CacheReadTokens:     int(acc.Usage.CacheReadInputTokens),
				CacheCreationTokens: int(acc.Usage.CacheCreationInputTokens),
				TotalTokens:         int(acc.Usage.InputTokens + acc.Usage.OutputTokens),
			},
			ToolCalls: toolCalls,
		}
	}()
	return ch, nil
}

func (r *AnthropicRuntime) buildParams(messages []chat.Message, opts *RuntimeOptions) (anthropic.MessageNewParams, error) {
	if opts == nil {
		opts = &RuntimeOptions{}
	}
	params := anthropic.MessageNewParams{
		Model: r.model,
	}
	if opts.MaxTokens > 0 {
		params.MaxTokens = int64(opts.MaxTokens)
	} else {
		params.MaxTokens = 4096
	}
	if opts.Temperature > 0 {
		params.Temperature = anthropic.Float(opts.Temperature)
	}
	if opts.TopP > 0 {
		params.TopP = anthropic.Float(opts.TopP)
	}
	if opts.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: opts.SystemPrompt, Type: "text"}}
	}
	if opts.ThinkingBudgetTokens > 0 {
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				Type:         "enabled",
				BudgetTokens: int64(opts.ThinkingBudgetTokens),
			},
		}
		if params.MaxTokens <= int64(opts.ThinkingBudgetTokens) {
			params.MaxTokens = int64(opts.ThinkingBudgetTokens) + 4096
		}
	}
	if len(opts.Tools) > 0 {
		var tools []anthropic.ToolUnionParam
		for _, t := range opts.Tools {
			def := t.Function
			var schema map[string]any
			if len(def.Parameters) > 0 {
				if err := json.Unmarshal(def.Parameters, &schema); err != nil {
					return params, fmt.Errorf("runtime/anthropic: invalid tool schema for %q: %w", def.Name, err)
				}
			} else {
				schema = map[string]any{"type": "object"}
			}
			inputSchema := anthropic.ToolInputSchemaParam{Type: "object"}
			if p, ok := schema["properties"]; ok {
				inputSchema.Properties = p
				delete(schema, "properties")
			}
			if req, ok := schema["required"]; ok {
				if rs, ok := req.([]any); ok {
					for _, s := range rs {
						inputSchema.Required = append(inputSchema.Required, fmt.Sprint(s))
					}
				}
				delete(schema, "required")
			}
			delete(schema, "type")
			inputSchema.ExtraFields = schema
			desc := def.Description
			tools = append(tools, anthropic.ToolUnionParam{
				OfTool: &anthropic.ToolParam{
					Name:        def.Name,
					Description: anthropic.String(desc),
					InputSchema: inputSchema,
				},
			})
		}
		params.Tools = tools
		switch opts.ToolChoice {
		case "required":
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfAny: &anthropic.ToolChoiceAnyParam{Type: "any"},
			}
		case "none":
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfNone: &anthropic.ToolChoiceNoneParam{Type: "none"},
			}
		case "":
			fallthrough
		case "auto":
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfAuto: &anthropic.ToolChoiceAutoParam{Type: "auto"},
			}
		default:
			params.ToolChoice = anthropic.ToolChoiceUnionParam{
				OfTool: &anthropic.ToolChoiceToolParam{Type: "tool", Name: opts.ToolChoice},
			}
		}
	}
	var anthroMsgs []anthropic.MessageParam
	for _, m := range messages {
		if m.Role == "system" {
			params.System = append(params.System, anthropic.TextBlockParam{Text: m.Content, Type: "text"})
			continue
		}
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		var blocks []anthropic.ContentBlockParamUnion
		if m.ReasoningContent != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfThinking: &anthropic.ThinkingBlockParam{Type: "thinking", Thinking: m.ReasoningContent},
			})
		}
		if m.Content != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfText: &anthropic.TextBlockParam{Type: "text", Text: m.Content},
			})
		}
		for _, part := range m.MultiContent {
			switch part.Type {
			case "text":
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfText: &anthropic.TextBlockParam{Type: "text", Text: part.Text},
				})
			case "image_url":
				// Callers are expected to provide data URIs. Anthropic SDK
				// Source{Data string} accepts base64 directly; if the URL is
				// a data URI, strip the prefix.
				data := part.ImageURL.URL
				mediaType := anthropic.Base64ImageSourceMediaTypeImagePNG
				if strings.HasPrefix(data, "data:") {
					if idx := strings.Index(data, ";base64,"); idx > 5 {
						mt := data[5:idx]
						switch mt {
						case "image/jpeg", "image/jpg":
							mediaType = anthropic.Base64ImageSourceMediaTypeImageJPEG
						case "image/png":
							mediaType = anthropic.Base64ImageSourceMediaTypeImagePNG
						case "image/gif":
							mediaType = anthropic.Base64ImageSourceMediaTypeImageGIF
						case "image/webp":
							mediaType = anthropic.Base64ImageSourceMediaTypeImageWebP
						}
						data = data[idx+8:]
					}
				}
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfImage: &anthropic.ImageBlockParam{
						Type: "image",
						Source: anthropic.ImageBlockParamSourceUnion{
							OfBase64: &anthropic.Base64ImageSourceParam{
								MediaType: mediaType,
								Data:      data,
							},
						},
					},
				})
			}
		}
		for _, tc := range m.ToolCalls {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: json.RawMessage(tc.Function.Arguments),
				},
			})
		}
		if m.ToolCallID != "" {
			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolResult: &anthropic.ToolResultBlockParam{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content: []anthropic.ToolResultBlockParamContentUnion{
						{OfText: &anthropic.TextBlockParam{Type: "text", Text: m.Content}},
					},
				},
			})
		}
		anthroMsgs = append(anthroMsgs, anthropic.MessageParam{
			Role:    anthroRole(role),
			Content: blocks,
		})
	}
	params.Messages = anthroMsgs
	if len(params.Messages) == 0 {
		return params, errors.New("runtime/anthropic: at least one message is required")
	}
	return params, nil
}

func responseToChatResponse(m *anthropic.Message) *types.ChatResponse {
	var text strings.Builder
	var reasoning strings.Builder
	var toolCalls []types.LLMToolCall
	for _, b := range m.Content {
		switch cb := b.AsAny().(type) {
		case anthropic.TextBlock:
			text.WriteString(cb.Text)
		case anthropic.ThinkingBlock:
			reasoning.WriteString(cb.Thinking)
		case anthropic.ToolUseBlock:
			toolCalls = append(toolCalls, types.LLMToolCall{
				ID:       cb.ID,
				Type:     "function",
				Function: types.FunctionCall{Name: cb.Name, Arguments: string(cb.Input)},
			})
		}
	}
	resp := &types.ChatResponse{
		Content: text.String(),
		Usage: types.TokenUsage{
			PromptTokens:        int(m.Usage.InputTokens),
			CompletionTokens:    int(m.Usage.OutputTokens),
			CachedTokens:        int(m.Usage.CacheReadInputTokens + m.Usage.CacheCreationInputTokens),
			CacheReadTokens:     int(m.Usage.CacheReadInputTokens),
			CacheCreationTokens: int(m.Usage.CacheCreationInputTokens),
			TotalTokens:         int(m.Usage.InputTokens + m.Usage.OutputTokens),
		},
		ToolCalls: toolCalls,
	}
	if reasoning.Len() > 0 {
		resp.ReasoningContent = reasoning.String()
	}
	return resp
}

func anthroRole(r string) anthropic.MessageParamRole {
	switch r {
	case "assistant":
		return anthropic.MessageParamRoleAssistant
	case "user":
		return anthropic.MessageParamRoleUser
	default:
		return anthropic.MessageParamRoleUser
	}
}

func init() {
	RegisterFactory("anthropic", NewAnthropicRuntime)
	RegisterFactory("anthropic-sdk", NewAnthropicRuntime)
}
