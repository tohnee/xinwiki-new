package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

// ArtifactRepository persists generated artifacts (review 4.2). The repository
// is tenant-aware on every mutating/listing path so a cross-tenant caller
// cannot enumerate or delete another tenant's artifacts at the SQL layer; the
// service layer additionally applies the per-user ACL (types.CanAccessArtifact)
// for reads.
type ArtifactRepository interface {
	// Create persists a new artifact. ID, TenantID, UserID and Type must be
	// populated by the caller; status defaults to "pending" via the column
	// default when unset.
	Create(ctx context.Context, a *types.Artifact) error

	// GetByID returns the non-deleted artifact by id, regardless of tenant.
	// The service layer enforces the tenant boundary + ACL; returning the
	// row here lets the service render a 404 (rather than leak existence)
	// when the id belongs to another tenant.
	GetByID(ctx context.Context, id string) (*types.Artifact, error)

	// ListByTenant returns every non-deleted artifact for the tenant, newest
	// first. ACL filtering is applied by the service layer so the SQL stays
	// portable across the supported vector-store backends; for high-volume
	// tenants the ACL can later move into the query.
	ListByTenant(ctx context.Context, tenantID uint64) ([]*types.Artifact, error)

	// ListBySession returns the non-deleted artifacts produced by a session
	// within the tenant, newest first. Used by the chat/agent panel to show
	// "what this session generated".
	ListBySession(ctx context.Context, tenantID uint64, sessionID string) ([]*types.Artifact, error)

	// UpdateStatus transitions an artifact's lifecycle state, recording the
	// storage URI + size once generation completes. Used by the generation
	// pipeline to flip pending -> ready / failed.
	UpdateStatus(ctx context.Context, id string, status types.ArtifactStatus, storageURI string, sizeBytes int64) error

	// SoftDelete marks the artifact deleted (deleted_at = now). Tenant-scoped
	// so a cross-tenant caller cannot delete by guessing the id.
	SoftDelete(ctx context.Context, tenantID uint64, id string) error
}
