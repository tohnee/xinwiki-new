package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// APIKeyService administers scoped API keys (review 4.5): create (returning
// the plaintext secret exactly once), list, and revoke. Every method is
// tenant-scoped — the caller's tenantID is passed explicitly and the service
// verifies a key belongs to that tenant before mutating it, so an Admin of
// tenant A cannot revoke tenant B's keys by guessing the key id.
type APIKeyService interface {
	// Create issues a new active API key. The plaintext secret is generated,
	// hashed (utils.HashAPIKey), persisted, and returned ONLY here; the caller
	// must surface it to the user once and never log or persist it. userID
	// stamps the creator (empty for synthetic/system callers). Rejects empty
	// names and unknown scopes (types.ValidateScopes) before touching the repo.
	Create(
		ctx context.Context,
		tenantID uint64,
		userID string,
		name string,
		scopes []string,
		expiresAt *time.Time,
	) (key *types.APIKey, plaintextSecret string, err error)

	// List returns all non-deleted API keys for the tenant, newest first.
	// The returned rows never carry the plaintext secret (it is not stored)
	// and KeyHash is rendered as "-" by the JSON tag, so listings are safe to
	// return verbatim.
	List(ctx context.Context, tenantID uint64) ([]*types.APIKey, error)

	// Revoke marks the key revoked so GetByHash no longer authenticates it.
	// Returns ErrAPIKeyNotFound if the key does not exist or belongs to a
	// different tenant — cross-tenant existence is deliberately not leaked,
	// so the response is identical to revoking a missing key.
	Revoke(ctx context.Context, tenantID uint64, id string) error
}
