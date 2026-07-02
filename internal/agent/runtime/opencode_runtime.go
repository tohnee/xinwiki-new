package runtime

// This file reserves a build target for an OpenCode SDK runtime adapter.
// OpenCode (github.com/opencode-ai/opencode or similar) is not a fixed API
// today; the placeholder below documents the intended integration point.
//
// When the OpenCode SDK is available, implement Factory and register via
// RegisterFactory("opencode", NewOpenCodeRuntime).

// TODO: implement OpenCodeRuntime once the SDK stabilizes.
//
// Sketch of the future surface:
//
// type OpenCodeRuntime struct {
//     client opencode.Client
//     model  string
//     modelID string
// }
//
// func NewOpenCodeRuntime(cfg *chat.ChatConfig) (Runtime, error) {
//     c := opencode.NewClient(cfg.APIKey, cfg.BaseURL)
//     return &OpenCodeRuntime{client: c, model: cfg.ModelName, modelID: cfg.ModelID}, nil
// }
//
// OpenCode typically exposes an agentic session API with tools built-in;
// Chat()/ChatStream() will translate chat.Messages + RuntimeOptions into
// OpenCode's session/run API and map the events back to StreamResponse
// chunks (answer/thinking/tool_call/tool_args/finished).
//
// Once shipped, update init() to register:
//
// func init() {
//     RegisterFactory("opencode", NewOpenCodeRuntime)
// }
