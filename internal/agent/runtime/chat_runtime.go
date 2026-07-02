package runtime

import (
	"context"
	"fmt"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// ChatRuntime adapts any existing chat.Chat implementation to the Runtime
// interface. This is the backwards-compatible shim used for providers that
// have not yet been ported to a native Runtime implementation.
type ChatRuntime struct {
	client chat.Chat
}

// NewChatRuntime builds a legacy chat.Chat via the shared factory and wraps
// it as a Runtime. This path preserves all existing provider adapters
// (OpenAI-compatible, Ollama, Gemini, Volcengine, Qwen, DeepSeek, ...)
// without requiring any changes.
func NewChatRuntime(cfg *chat.ChatConfig) (Runtime, error) {
	c, err := chat.NewChat(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("runtime/chat: init chat client: %w", err)
	}
	return &ChatRuntime{client: c}, nil
}

func (r *ChatRuntime) Name() string      { return "legacy-chat" }
func (r *ChatRuntime) ModelName() string { return r.client.GetModelName() }
func (r *ChatRuntime) ModelID() string   { return r.client.GetModelID() }

func (r *ChatRuntime) Chat(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (*types.ChatResponse, error) {
	chatOpts := r.toChatOptions(opts)
	// Inject system prompt as a system message when the caller passed one.
	if opts != nil && opts.SystemPrompt != "" {
		messages = append([]chat.Message{{Role: "system", Content: opts.SystemPrompt}}, messages...)
	}
	return r.client.Chat(ctx, messages, chatOpts)
}

func (r *ChatRuntime) ChatStream(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (<-chan types.StreamResponse, error) {
	chatOpts := r.toChatOptions(opts)
	if opts != nil && opts.SystemPrompt != "" {
		messages = append([]chat.Message{{Role: "system", Content: opts.SystemPrompt}}, messages...)
	}
	return r.client.ChatStream(ctx, messages, chatOpts)
}

func (r *ChatRuntime) toChatOptions(opts *RuntimeOptions) *chat.ChatOptions {
	if opts == nil {
		return &chat.ChatOptions{}
	}
	o := opts.ChatOptions // copy
	// Map ThinkingBudgetTokens to the Thinking boolean flag the legacy chat
	// clients understand. Legacy clients do not support explicit budget
	// control, so any non-zero budget just enables thinking mode.
	if opts.ThinkingBudgetTokens > 0 && o.Thinking == nil {
		enabled := true
		o.Thinking = &enabled
	}
	return &o
}
