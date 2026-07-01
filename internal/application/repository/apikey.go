package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

// apiKeyRepository implements interfaces.APIKeyRepository on top of GORM.
type apiKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository creates a GORM-backed APIKeyRepository.
func NewAPIKeyRepository(db *gorm.DB) interfaces.APIKeyRepository {
	return &apiKeyRepository{db: db}
}

// Create persists a new API key. The caller is responsible for hashing the
// plaintext secret into KeyHash before calling.
func (r *apiKeyRepository) Create(ctx context.Context, key *types.APIKey) error {
	if key == nil {
		return errors.New("apikey: nil key")
	}
	return r.db.WithContext(ctx).Create(key).Error
}

// GetByHash returns the ACTIVE API key matching keyHash, or an error if none.
// Revoked keys are excluded so a revoked credential cannot authenticate.
func (r *apiKeyRepository) GetByHash(ctx context.Context, keyHash string) (*types.APIKey, error) {
	if keyHash == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var k types.APIKey
	err := r.db.WithContext(ctx).
		Where("key_hash = ? AND status = ? AND deleted_at IS NULL", keyHash, "active").
		First(&k).Error
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetByID returns an API key by ID regardless of status (management path).
func (r *apiKeyRepository) GetByID(ctx context.Context, id string) (*types.APIKey, error) {
	var k types.APIKey
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&k).Error
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListByTenant returns all non-deleted API keys for a tenant.
func (r *apiKeyRepository) ListByTenant(ctx context.Context, tenantID uint64) ([]*types.APIKey, error) {
	var keys []*types.APIKey
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("created_at DESC").
		Find(&keys).Error
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// Revoke marks an API key as revoked. GetByHash will no longer return it.
func (r *apiKeyRepository) Revoke(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&types.APIKey{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("status", "revoked").Error
}

// TouchLastUsed records the last-used timestamp. Best-effort.
func (r *apiKeyRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&types.APIKey{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("last_used_at", at).Error
}
