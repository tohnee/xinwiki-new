package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	secutils "github.com/Tencent/XinWiki/internal/utils"
	"gorm.io/gorm"
)

// Sentinel errors returned by apiKeyService. Callers compare with errors.Is
// to render the appropriate HTTP response.
var (
	// ErrAPIKeyNameRequired is returned by Create when the human label is
	// blank. The handler maps this to 400.
	ErrAPIKeyNameRequired = errors.New("api key name is required")

	// ErrAPIKeyNotFound is returned by Revoke when no active key matches the
	// id within the caller's tenant. Cross-tenant existence is deliberately
	// collapsed into this same sentinel so an attacker cannot probe which
	// ids exist in another tenant.
	ErrAPIKeyNotFound = errors.New("api key not found")
)

// apiKeyService implements interfaces.APIKeyService. It owns the
// generate-secret -> hash -> persist flow on Create and the tenant boundary
// check on Revoke; the repository handles persistence only.
type apiKeyService struct {
	repo interfaces.APIKeyRepository
	// now is an injection seam so tests can pin timestamps without touching
	// wall-clock state. Production wires time.Now.
	now func() time.Time
}

// NewAPIKeyService wires the repository. The service is stateless beyond the
// repo handle, so a single instance is safe to share (registered as a dig
// singleton via the provider).
func NewAPIKeyService(repo interfaces.APIKeyRepository) interfaces.APIKeyService {
	return &apiKeyService{repo: repo, now: time.Now}
}

// Create issues a new active API key. The plaintext secret is generated,
// hashed, persisted, and returned exactly once. Empty names and unknown
// scopes are rejected before the repository is touched.
func (s *apiKeyService) Create(
	ctx context.Context,
	tenantID uint64,
	userID string,
	name string,
	scopes []string,
	expiresAt *time.Time,
) (*types.APIKey, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", ErrAPIKeyNameRequired
	}
	if err := types.ValidateScopes(scopes); err != nil {
		return nil, "", err
	}

	secret, prefix := secutils.GenerateAPIKeySecret()
	now := s.now()
	key := &types.APIKey{
		ID:        secutils.GenerateAPIKeyID(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		KeyHash:   secutils.HashAPIKey(secret),
		Prefix:    prefix,
		Scopes:    types.StringArray(scopes),
		Status:    "active",
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, "", err
	}
	return key, secret, nil
}

// List returns every non-deleted API key belonging to tenantID. The repo
// already filters by tenant_id and skips soft-deleted rows.
func (s *apiKeyService) List(ctx context.Context, tenantID uint64) ([]*types.APIKey, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

// Revoke marks the key revoked. A key that does not exist or belongs to a
// different tenant returns ErrAPIKeyNotFound so cross-tenant existence is
// not leaked.
func (s *apiKeyService) Revoke(ctx context.Context, tenantID uint64, id string) error {
	k, err := s.repo.GetByID(ctx, id)
	if err != nil {
		// gorm.ErrRecordNotFound from the real repo; the in-memory fake in
		// tests returns the same sentinel. Any other error is a real failure
		// the caller should surface as 500, not 404.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAPIKeyNotFound
		}
		return err
	}
	if k == nil || k.TenantID != tenantID {
		return ErrAPIKeyNotFound
	}
	return s.repo.Revoke(ctx, id)
}
