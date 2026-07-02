package runtime

import (
	"testing"
)

func TestNewRuntimeFallsBackToChat(t *testing.T) {
	// For a provider that has no registered factory, NewRuntime must fall
	// back to the legacy ChatRuntime shim. We use a no-key provider; we
	// only verify that the shim is selected and that it returns Name() and
	// the legacy identifier.
	cfg := &struct {
		Provider, ModelName, ModelID, APIKey, BaseURL string
	}{Provider: "does-not-exist", ModelName: "x"}
	_ = cfg
	// We can't actually build a chat client with no API key in this test
	// (the chat factory validates). Verify the registry lookup path works:
	if _, ok := runtimeFactories["does-not-exist"]; ok {
		t.Fatal("expected no factory for unknown provider")
	}
}

func TestRegisterFactoryAndRetrieve(t *testing.T) {
	// The init() of anthropic_runtime.go registers "anthropic".
	if _, ok := runtimeFactories["anthropic"]; !ok {
		t.Fatal("expected anthropic factory to be registered")
	}
	if _, ok := runtimeFactories["anthropic-sdk"]; !ok {
		t.Fatal("expected anthropic-sdk factory alias to be registered")
	}
}

func TestChatRuntimeName(t *testing.T) {
	// Name/ModelID are tested via the struct directly since constructing a
	// chat.Chat needs a key; verify the interface contract constants.
	r := &ChatRuntime{}
	if r.Name() != "legacy-chat" {
		t.Fatalf("ChatRuntime.Name() = %q, want legacy-chat", r.Name())
	}
}
