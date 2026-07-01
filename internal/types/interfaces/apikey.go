package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// APIKeyRepository persists scoped API keys (review 4.5). The auth middleware
// uses GetByHash to resolve a presented X-API-Key to its tenant + scopes;
// management handlers use Create/ListByTenant/GetByID/Revoke to administer
// keys. GetByHash returns only active (non-revoked) keys — a revoked key must
// not authenticate even when its hash matches.
type APIKeyRepository interface {
	// Create persists a new API key. KeyHash must already be hashed by the
	// caller (utils.HashAPIKey); plaintext is never stored.
	Create(ctx context.Context, key *types.APIKey) error
	// GetByHash returns the active API key whose KeyHash matches, or an error
	// if no active key matches. Used by the auth path.
	GetByHash(ctx context.Context, keyHash string) (*types.APIKey, error)
	// GetByID returns an API key by ID regardless of status (management path).
	GetByID(ctx context.Context, id string) (*types.APIKey, error)
	// ListByTenant returns all non-deleted API keys for a tenant.
	ListByTenant(ctx context.Context, tenantID uint64) ([]*types.APIKey, error)
	// Revoke marks an API key as revoked so GetByHash no longer returns it.
	Revoke(ctx context.Context, id string) error
	// TouchLastUsed records the last-used timestamp. Best-effort: errors are
	// logged by the caller and never block authentication.
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
}
