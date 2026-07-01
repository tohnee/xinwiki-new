package repository

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

// artifactRepository implements interfaces.ArtifactRepository on top of GORM.
type artifactRepository struct {
	db *gorm.DB
}

// NewArtifactRepository creates a GORM-backed ArtifactRepository.
func NewArtifactRepository(db *gorm.DB) interfaces.ArtifactRepository {
	return &artifactRepository{db: db}
}

// Create persists a new artifact. The caller is responsible for populating
// ID, TenantID, UserID and Type; the column default sets status to "pending".
// The JSON columns are defaulted to empty containers when unset so the NOT
// NULL constraint (types.JSON.Value returns SQL NULL for an empty RawMessage)
// is satisfied on both postgres and sqlite.
func (r *artifactRepository) Create(ctx context.Context, a *types.Artifact) error {
	if len(a.SourceRefs) == 0 {
		a.SourceRefs = types.JSON("[]")
	}
	if len(a.Metadata) == 0 {
		a.Metadata = types.JSON("{}")
	}
	return r.db.WithContext(ctx).Create(a).Error
}

// GetByID returns the non-deleted artifact by id. The service layer enforces
// the tenant boundary + ACL; returning the row here lets the service render a
// 404 (rather than leak existence) when the id belongs to another tenant.
func (r *artifactRepository) GetByID(ctx context.Context, id string) (*types.Artifact, error) {
	var a types.Artifact
	err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// ListByTenant returns every non-deleted artifact for the tenant, newest
// first. ACL filtering is applied by the service layer.
func (r *artifactRepository) ListByTenant(ctx context.Context, tenantID uint64) ([]*types.Artifact, error) {
	var rows []*types.Artifact
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListBySession returns the non-deleted artifacts produced by a session
// within the tenant, newest first.
func (r *artifactRepository) ListBySession(ctx context.Context, tenantID uint64, sessionID string) ([]*types.Artifact, error) {
	var rows []*types.Artifact
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND deleted_at IS NULL", tenantID, sessionID).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// UpdateStatus transitions an artifact's lifecycle state, recording the
// storage URI + size once generation completes (pending -> ready / failed).
// updated_at is bumped so listeners can detect the change.
func (r *artifactRepository) UpdateStatus(ctx context.Context, id string, status types.ArtifactStatus, storageURI string, sizeBytes int64) error {
	return r.db.WithContext(ctx).Model(&types.Artifact{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"status":      string(status),
			"storage_uri": storageURI,
			"size_bytes":  sizeBytes,
			"updated_at":  time.Now(),
		}).Error
}

// SoftDelete marks the artifact deleted. Tenant-scoped so a cross-tenant
// caller cannot delete by guessing the id (the WHERE clause simply matches
// no row, a no-op).
func (r *artifactRepository) SoftDelete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Model(&types.Artifact{}).
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", id, tenantID).
		Update("deleted_at", time.Now()).Error
}
