package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/XinWiki/internal/types"
)

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
