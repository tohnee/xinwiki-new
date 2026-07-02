package agent

import (
	"context"

	"github.com/Tencent/XinWiki/internal/agent/runtime"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// chatRuntimeAdapter adapts chat.Chat to the runtime.Runtime interface for
// backwards compatibility when no explicit runtime is configured.
type chatRuntimeAdapter struct {
	c chat.Chat
}

func (a *chatRuntimeAdapter) Name() string      { return "chat" }
func (a *chatRuntimeAdapter) ModelName() string { return a.c.GetModelName() }
func (a *chatRuntimeAdapter) ModelID() string   { return a.c.GetModelID() }

func (a *chatRuntimeAdapter) Chat(ctx context.Context, messages []chat.Message, opts *runtime.RuntimeOptions) (*types.ChatResponse, error) {
	return a.c.Chat(ctx, messages, toChatOptions(opts))
}

func (a *chatRuntimeAdapter) ChatStream(ctx context.Context, messages []chat.Message, opts *runtime.RuntimeOptions) (<-chan types.StreamResponse, error) {
	return a.c.ChatStream(ctx, messages, toChatOptions(opts))
}

// activeRuntime returns the preferred runtime for agent LLM calls. If an
// explicit runtime was set via SetRuntime (e.g. Anthropic native SDK), it
// is used; otherwise the engine falls back to wrapping the configured
// chatModel so existing providers keep working unchanged.
func (e *AgentEngine) activeRuntime() runtime.Runtime {
	if e.rt != nil {
		return e.rt
	}
	return &chatRuntimeAdapter{c: e.chatModel}
}

// systemPromptFromMessages extracts a system prompt from the head of the
// message slice, if present. It returns the (possibly empty) system prompt
// and the remaining messages. This is needed because Runtime carries system
// separately while chat.Chat accepts it as a role=system message.
func extractSystemPrompt(messages []chat.Message) (string, []chat.Message) {
	if len(messages) == 0 {
		return "", messages
	}
	if messages[0].Role != "system" {
		return "", messages
	}
	return messages[0].Content, messages[1:]
}

func toChatOptions(opts *runtime.RuntimeOptions) *chat.ChatOptions {
	if opts == nil {
		return &chat.ChatOptions{}
	}
	o := opts.ChatOptions
	if opts.ThinkingBudgetTokens > 0 && o.Thinking == nil {
		enabled := true
		o.Thinking = &enabled
	}
	return &o
}

func toRuntimeOptions(opts *chat.ChatOptions, systemPrompt string) *runtime.RuntimeOptions {
	ro := &runtime.RuntimeOptions{SystemPrompt: systemPrompt}
	if opts != nil {
		ro.ChatOptions = *opts
	}
	return ro
}
