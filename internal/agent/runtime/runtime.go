// Package runtime defines the AgentRuntime abstraction for pluggable LLM
// backends used by the agent engine.
//
// The agent engine historically talked directly to chat.Chat. The Runtime
// interface extends that contract to cover agent-native capabilities that
// not all vendors expose identically:
//
//   - extended thinking (budget tokens)
//   - prompt caching / cache control hints
//   - structured output / JSON schema
//   - parallel tool calls
//   - image blocks (already covered by chat.Message.MultiContent)
//
// Concrete implementations live alongside this file:
//
//   - ChatRuntime      – adapts any existing chat.Chat to Runtime (legacy shim)
//   - AnthropicRuntime – official anthropic-sdk-go based native client
//
// Additional providers (OpenAI Responses API, Gemini, OpenCode SDK) can be
// added by implementing the Runtime interface and registering them via
// RegisterRuntime.
package runtime

import (
	"context"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

// Runtime is the narrow contract that the agent engine requires from an
// LLM backend. It mirrors chat.Chat but adds agent-specific knobs.
//
// Every Runtime MUST implement at least the streaming path; non-streaming
// Chat() can be emulated by draining the stream if the backend does not
// provide a native non-streaming call.
type Runtime interface {
	// Chat performs a non-streaming (single-turn) completion.
	Chat(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (*types.ChatResponse, error)

	// ChatStream returns a channel of incremental stream events.
	ChatStream(ctx context.Context, messages []chat.Message, opts *RuntimeOptions) (<-chan types.StreamResponse, error)

	// Name returns a human-readable backend identifier (e.g. "anthropic-sdk",
	// "openai-responses", "opencode", "legacy-chat"). Used in logs/traces.
	Name() string

	// ModelName returns the resolved model name passed to the backend.
	ModelName() string

	// ModelID returns the internal model ID used by XinWiki.
	ModelID() string
}

// RuntimeOptions extends chat.ChatOptions with agent-native knobs.
type RuntimeOptions struct {
	// ChatOptions carries the baseline generation parameters shared by all
	// providers (temperature, tools, tool_choice, ...).
	chat.ChatOptions

	// ThinkingBudgetTokens, when >0, requests extended thinking / chain-
	// of-thought generation with roughly this token budget.
	ThinkingBudgetTokens int `json:"thinking_budget_tokens,omitempty"`

	// CacheControl enables provider-side prompt caching where supported
	// (e.g. Anthropic cache_control blocks on system messages and tools).
	CacheControl bool `json:"cache_control,omitempty"`

	// SystemPrompt is provided separately because some runtimes (Anthropic
	// Messages API, OpenAI Responses) treat the system prompt as a top-
	// level parameter rather than a role="system" message.
	SystemPrompt string `json:"-"`

	// Metadata are opaque key/value pairs forwarded to the provider for
	// tracing / user-id tags (e.g. x-client-id, user-id for Anthropic).
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Factory builds a Runtime from a chat.ChatConfig. Implementations should
// validate the config and return an error if required fields are missing.
type Factory func(cfg *chat.ChatConfig) (Runtime, error)

var runtimeFactories = map[string]Factory{}

// RegisterFactory registers a Runtime factory under a provider name.
// Calling RegisterFactory twice with the same name panics. Thread-safe
// only at init time; register from init() funcs.
func RegisterFactory(provider string, f Factory) {
	if _, ok := runtimeFactories[provider]; ok {
		panic("runtime: factory already registered for " + provider)
	}
	runtimeFactories[provider] = f
}

// NewRuntime returns a Runtime for the given provider. If no factory is
// registered for the provider, it falls back to the legacy chat.Chat shim
// (ChatRuntime), which preserves backwards compatibility with every
// provider already supported via chat.NewRemoteChat.
func NewRuntime(cfg *chat.ChatConfig) (Runtime, error) {
	if f, ok := runtimeFactories[cfg.Provider]; ok {
		return f(cfg)
	}
	// Fallback: wrap the legacy chat client so existing providers keep
	// working without modification.
	return NewChatRuntime(cfg)
}
