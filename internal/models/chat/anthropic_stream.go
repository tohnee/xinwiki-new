package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Tencent/XinWiki/internal/types"
)

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
