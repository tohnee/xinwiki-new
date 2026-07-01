package types

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalidAPIKeyScope is returned by ValidateScopes when a scope string is
// not grantable. Wrapped with the offending value via fmt.Errorf("%w: %q")
// so callers can errors.Is it while still rendering the bad value in a 400.
var ErrInvalidAPIKeyScope = errors.New("invalid api key scope")

// API key scopes. A scope is a string of the form "<resource>:<action>" (e.g.
// "kb:read", "doc:write") or a bare resource ("chat", "agent:run"). The
// special scope "*" grants every scope (the owner/admin master scope); a
// trailing "<resource>:*" grants every action on that resource.
//
// Scopes are enforced on API-key-authenticated requests: a route declares the
// scope it requires, and RequireScope denies the request unless the key's
// granted scopes cover it. This replaces the legacy single-per-tenant API key
// that implicitly granted full Owner access to the whole tenant.
const (
	// ScopeAll is the super scope: authorizes everything. Reserve for the
	// tenant owner's personal key; do not hand to integrations.
	ScopeAll = "*"
	// Knowledge-base / document scopes.
	ScopeKBRead   = "kb:read"
	ScopeKBWrite  = "kb:write"
	ScopeDocRead  = "doc:read"
	ScopeDocWrite = "doc:write"
	// Conversation / agent scopes.
	ScopeChat     = "chat"
	ScopeAgentRun = "agent:run"
	// Administration scopes.
	ScopeAdminUsers   = "admin:users"
	ScopeAdminDelete  = "admin:delete"
	ScopeAdminAPIKeys = "admin:apikeys"
)

// knownScopes is the closed set of exact scopes a key may be granted. It is
// the single source of truth for IsValidScope: adding a scope constant above
// requires adding it here so Create refuses typos and unknown strings.
var knownScopes = map[string]struct{}{
	ScopeAll:          {},
	ScopeKBRead:       {},
	ScopeKBWrite:      {},
	ScopeDocRead:      {},
	ScopeDocWrite:     {},
	ScopeChat:         {},
	ScopeAgentRun:     {},
	ScopeAdminUsers:   {},
	ScopeAdminDelete:  {},
	ScopeAdminAPIKeys: {},
}

// knownScopeResources lists the resource families that admit a "<resource>:*"
// blanket wildcard (every action under that family). Bare scopes like "chat"
// and "agent:run" have no sub-actions and therefore no wildcard form.
var knownScopeResources = map[string]struct{}{
	"kb":    {},
	"doc":   {},
	"admin": {},
}

// IsValidScope reports whether s is a grantable scope: the super scope "*",
// a known exact scope, or a "<resource>:*" wildcard for a known resource
// family. Unknown strings (typos, invented resources, malformed segments)
// are rejected so ValidateScopes can refuse them at key-creation time.
func IsValidScope(s string) bool {
	if _, ok := knownScopes[s]; ok {
		return true
	}
	if strings.HasSuffix(s, ":*") {
		_, ok := knownScopeResources[strings.TrimSuffix(s, ":*")]
		return ok
	}
	return false
}

// ValidateScopes reports whether every entry in scopes is a grantable scope.
// An empty set is valid (the key authenticates but RequireScope fail-closes
// on every scoped route); non-emptiness is enforced at the authorization
// layer, not at validation time. Returns a wrapped ErrInvalidAPIKeyScope so
// callers can render a 400 with the offending value.
func ValidateScopes(scopes []string) error {
	for _, s := range scopes {
		if !IsValidScope(s) {
			return fmt.Errorf("%w: %q", ErrInvalidAPIKeyScope, s)
		}
	}
	return nil
}

// APIKey is a scoped, revocable API credential belonging to a tenant. Unlike
// the legacy single Tenant.APIKey (a tenant-wide master key), each APIKey
// carries an explicit Scopes list and optional UserID so integrations get
// least-privilege access and individual keys can be rotated/revoked without
// affecting others.
//
// KeyHash holds a hashed form of the secret; the plaintext is returned to the
// caller only once at creation time and never persisted. Comparison uses
// ConstantTimeCompare against KeyHash.
type APIKey struct {
	ID         string      `json:"id"          gorm:"primaryKey"`
	TenantID   uint64      `json:"tenant_id"   gorm:"index;not null"`
	UserID     string      `json:"user_id,omitempty"`
	Name       string      `json:"name"`                                               // human label, e.g. "CI ingest"
	KeyHash    string      `json:"-"                  gorm:"column:key_hash;not null"` // hashed secret, never plaintext
	Prefix     string      `json:"prefix"`                                             // first chars of plaintext for display/lookup
	Scopes     StringArray `json:"scopes"      gorm:"type:jsonb"`
	Status     string      `json:"status"      gorm:"default:'active'"` // active | revoked
	ExpiresAt  *time.Time  `json:"expires_at,omitempty"`
	LastUsedAt *time.Time  `json:"last_used_at,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	DeletedAt  *time.Time  `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName overrides the default table name for APIKey.
func (APIKey) TableName() string { return "api_keys" }

// ScopesAllow reports whether the granted scopes authorize the single required
// scope. Authorization is fail-closed: an empty granted set never authorizes.
//
// Matching rules:
//   - "*" grants everything.
//   - an exact match grants.
//   - "<resource>:*" grants any "<resource>:<action>" (segment-bound, so
//     "kb:*" does NOT grant "kbx:read").
func ScopesAllow(granted []string, required string) bool {
	if len(granted) == 0 {
		return false
	}
	for _, s := range granted {
		if s == ScopeAll || s == required {
			return true
		}
		// "<resource>:*" wildcard, segment-bound.
		if len(s) > 2 && s[len(s)-2:] == ":*" {
			prefix := s[:len(s)-1] // "kb:" (keep the colon)
			if len(required) > len(prefix) && required[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// ScopesAllowAll reports whether the granted scopes authorize EVERY required
// scope. Empty required is trivially allowed. Like ScopesAllow it is
// fail-closed on empty granted when required is non-empty.
func ScopesAllowAll(granted []string, required []string) bool {
	for _, r := range required {
		if !ScopesAllow(granted, r) {
			return false
		}
	}
	return true
}
