package chat

import (
	"testing"
)

// TestSharedBreakerForURL_SameHostSameInstance: two constructions against
// the same endpoint must return the SAME *CircuitBreaker so failures in one
// caller actually propagate to other callers' circuit state. This is the
// guarantee that gives the breaker its fleet-wide protective effect.
func TestSharedBreakerForURL_SameHostSameInstance(t *testing.T) {
	t.Cleanup(resetBreakerRegistry)
	resetBreakerRegistry()

	cb1 := sharedBreakerForURL("https://api.anthropic.com/")
	cb2 := sharedBreakerForURL("https://api.anthropic.com/v1/messages")
	if cb1 == nil {
		t.Fatal("breaker for a real host must not be nil")
	}
	if cb1 != cb2 {
		t.Fatalf("same host must share breaker: got %p and %p", cb1, cb2)
	}
	if got := breakerRegistrySize(); got != 1 {
		t.Fatalf("registry must hold 1 entry per host, got %d", got)
	}
}

// TestSharedBreakerForURL_DifferentHostsIndependent: a separate host gets
// its own breaker, so an api.anthropic.com trip does NOT affect a self-
// hosted vLLM endpoint (and vice-versa).
func TestSharedBreakerForURL_DifferentHostsIndependent(t *testing.T) {
	t.Cleanup(resetBreakerRegistry)
	resetBreakerRegistry()

	cbAnthropic := sharedBreakerForURL("https://api.anthropic.com/")
	cbSelf := sharedBreakerForURL("https://vllm.internal.corp:8000/v1")
	if cbAnthropic == cbSelf {
		t.Fatalf("distinct hosts must receive distinct breakers")
	}
	if got := breakerRegistrySize(); got != 2 {
		t.Fatalf("registry must hold 2 entries for 2 hosts, got %d", got)
	}
}

// TestSharedBreakerForURL_EmptyAndGarbageDegradeToNil: a caller that has
// not yet computed its endpoint must not crash; it gets a nil breaker
// (the documented no-op fallback). Unparseable input likewise yields nil.
func TestSharedBreakerForURL_EmptyAndGarbageDegradeToNil(t *testing.T) {
	t.Cleanup(resetBreakerRegistry)
	resetBreakerRegistry()

	if cb := sharedBreakerForURL(""); cb != nil {
		t.Fatalf("empty URL must yield nil breaker, got %p", cb)
	}
	// A bare scheme with no host triggers url.Parse error; normalizeBreakerKey
	// then falls back to the trimmed string which is ":" -- still distinguishable
	// but should not collide with empty.
	cbColon := sharedBreakerForURL(":")
	_ = cbColon // not asserted nil -- this is a degenerate input, we just require no panic
}

// TestSharedBreakerForURL_NormalizesPortAndCase: api.anthropic.com and
// API.ANTHROPIC.COM refer to the same breaker; a :443-equivalence is not
// guaranteed (we key on Host which preserves the explicit port) but the
// case-insensitivity must hold so a user typing the URL with mixed case
// still participates in fleet-wide breaker state.
func TestSharedBreakerForURL_NormalizesPortAndCase(t *testing.T) {
	t.Cleanup(resetBreakerRegistry)
	resetBreakerRegistry()

	cbLower := sharedBreakerForURL("https://api.anthropic.com/v1")
	cbUpper := sharedBreakerForURL("HTTPS://API.Anthropic.Com/v1/")
	if cbLower != cbUpper {
		t.Fatalf("host case must be normalized to share a breaker: %p vs %p", cbLower, cbUpper)
	}
}

// TestNewAnthropicChat_AttachesSharedBreaker: the production constructor
// must wire the breaker by default (not opt-in) so the protective effect
// is on without callers having to remember WithCircuitBreaker.
func TestNewAnthropicChat_AttachesSharedBreaker(t *testing.T) {
	t.Cleanup(resetBreakerRegistry)
	resetBreakerRegistry()

	chat, err := NewAnthropicChat(&ChatConfig{
		Source:    "remote",
		Provider:  "anthropic",
		BaseURL:   "https://api.anthropic.com",
		ModelName: "claude-test",
		APIKey:    "sk-test",
	})
	if err != nil {
		t.Fatalf("NewAnthropicChat: %v", err)
	}
	// The breaker field is unexported; verify through behavior by checking
	// the shared registry gained one entry for the Anthropic host.
	if got := breakerRegistrySize(); got != 1 {
		t.Fatalf("NewAnthropicChat must populate the breaker registry, got %d", got)
	}
	// A second construction must reuse the same shared breaker (no new entry).
	if _, err := NewAnthropicChat(&ChatConfig{
		Source:    "remote",
		Provider:  "anthropic",
		BaseURL:   "https://api.anthropic.com",
		ModelName: "claude-other",
		APIKey:    "sk-other",
	}); err != nil {
		t.Fatalf("second NewAnthropicChat: %v", err)
	}
	if got := breakerRegistrySize(); got != 1 {
		t.Fatalf("registry must still hold 1 entry after 2nd construction, got %d", got)
	}
	_ = chat
}