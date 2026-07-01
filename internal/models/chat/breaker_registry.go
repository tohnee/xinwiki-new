package chat

import (
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	secutils "github.com/Tencent/XinWiki/internal/utils"
)

// Chat provider endpoints share a small, package-level set of circuit
// breakers keyed by their base-URL host. Tripping the breaker for
// api.anthropic.com must not affect self-hosted vLLM (or vice-versa), so
// each distinct host gets its own breaker. Construction is idempotent:
// every NewAnthropicChat call with the same endpoint reuses the singleton
// for that host, which is what gives the breaker its fleet-wide protective
// effect - without it, each short-lived chat client would carry its own
// (useless) breaker state.
//
// Operators tune the policy with env vars (XINWIKI_* preferred, WEKNORA_*
// legacy alias still honored by ResolveEnvName):
//
//	XINWIKI_LLM_BREAKER_THRESHOLD       consecutive failures to trip (default 5)
//	XINWIKI_LLM_BREAKER_OPEN_SECONDS    open window before a half-open probe (default 30)
var (
	breakerRegMu sync.Mutex
	breakerReg   = make(map[string]*CircuitBreaker)

	// defaultBreakerCfg is captured at init time from env vars so the same
	// threshold/open-window applies to every provider a process ever talks to.
	defaultBreakerCfg = CircuitBreakerConfig{
		FailureThreshold: breakerEnvInt("WEKNORA_LLM_BREAKER_THRESHOLD", 5),
		OpenDuration:      time.Duration(breakerEnvInt("WEKNORA_LLM_BREAKER_OPEN_SECONDS", 30)) * time.Second,
	}
)

// breakerEnvInt resolves an env var via ResolveEnvName (so the XINWIKI_
// alias takes precedence over the WEKNORA_ legacy name) and returns the
// parsed integer or fallback. Non-positive or unparseable values fall
// back so an operator typo never silently disables the breaker entirely.
func breakerEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(secutils.ResolveEnvName(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// sharedBreakerForURL returns the singleton breaker protecting the given
// provider endpoint, creating it on first use. An empty / unparseable
// baseURL yields a nil breaker (the disabled fallback) so a misconfigured
// caller degrades to the prior "no breaker" behavior instead of crashing.
func sharedBreakerForURL(baseURL string) *CircuitBreaker {
	key := normalizeBreakerKey(baseURL)
	if key == "" {
		return nil
	}
	breakerRegMu.Lock()
	defer breakerRegMu.Unlock()
	if cb, ok := breakerReg[key]; ok {
		return cb
	}
	cb := NewCircuitBreaker(defaultBreakerCfg)
	breakerReg[key] = cb
	return cb
}

// normalizeBreakerKey collapses a base URL to its host (lowercased) so
// "https://api.anthropic.com/" and "https://api.anthropic.com:443/v1"
// share a breaker. Falls back to the trimmed URL when parsing fails so we
// still distinguish obviously different strings.
func normalizeBreakerKey(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	if u, err := url.Parse(trimmed); err == nil && u.Host != "" {
		return strings.ToLower(u.Host)
	}
	return strings.ToLower(strings.TrimRight(trimmed, "/"))
}

// resetBreakerRegistry wipes the per-URL breaker map. Test-only: lets a
// test put the package back into a known (empty) state without exposing
// the registry as public API. Not safe for concurrent use with live calls.
func resetBreakerRegistry() {
	breakerRegMu.Lock()
	defer breakerRegMu.Unlock()
	breakerReg = make(map[string]*CircuitBreaker)
}

// breakerRegistrySize is a small read-only accessor used by tests to assert
// the singleton behavior without holding the lock externally.
func breakerRegistrySize() int {
	breakerRegMu.Lock()
	defer breakerRegMu.Unlock()
	return len(breakerReg)
}