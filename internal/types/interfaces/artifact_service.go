package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

// ArtifactCaller bundles the caller identity the artifact service needs to
// evaluate the per-user ACL (types.CanAccessArtifact) and the modify guard
// (creator-or-admin). Threading it as one value keeps the service method
// signatures readable.
type ArtifactCaller struct {
	UserID        string
	Role          types.TenantRole
	IsSystemAdmin bool
}

// CreateArtifactParams is the input to ArtifactService.Create. The service
// mints the ID, stamps the creator + tenant, and defaults status to pending;
// the generation pipeline later calls UpdateStatus to flip it to ready/failed.
type CreateArtifactParams struct {
	SessionID         string
	Type              types.ArtifactType
	Title             string
	SourceKBID        string
	SourceKnowledgeID string
	SourceWikiPageID  string
	SharingPolicy     types.ArtifactSharingPolicy
	AllowedUserIDs    []string
}

// ArtifactService administers generated artifacts (review 4.2): create, read
// (ACL-filtered), session-scoped list, lifecycle update, and delete. Every
// method is tenant-scoped; reads additionally apply the per-user ACL so a
// non-admin caller cannot see another user's private artifacts, and
// mutations are restricted to the creator (or Admin+).
type ArtifactService interface {
	// Create mints a pending artifact owned by the caller. Validates the
	// type + sharing policy. Returns the persisted row (status pending).
	Create(ctx context.Context, tenantID uint64, caller ArtifactCaller, params CreateArtifactParams) (*types.Artifact, error)

	// Get returns the artifact if the caller may read it; otherwise
	// ErrArtifactForbidden. A missing or cross-tenant id returns
	// ErrArtifactNotFound (existence is not leaked across tenants).
	Get(ctx context.Context, tenantID uint64, caller ArtifactCaller, id string) (*types.Artifact, error)

	// List returns the caller-visible artifacts for the tenant, paginated
	// (newest first), plus the total count of visible artifacts. ACL
	// filtering is applied in-memory over the tenant's rows.
	List(ctx context.Context, tenantID uint64, caller ArtifactCaller, page, pageSize int) ([]*types.Artifact, int64, error)

	// ListBySession returns the caller-visible artifacts produced by a
	// session within the tenant. Used by the chat/agent panel.
	ListBySession(ctx context.Context, tenantID uint64, caller ArtifactCaller, sessionID string) ([]*types.Artifact, error)

	// UpdateStatus transitions the artifact's lifecycle (pending -> ready /
	// failed), recording the storage URI + size. Creator-or-admin only.
	UpdateStatus(ctx context.Context, tenantID uint64, caller ArtifactCaller, id string, status types.ArtifactStatus, storageURI string, sizeBytes int64) error

	// Delete soft-deletes the artifact. Creator-or-admin only. Cross-tenant
	// ids return ErrArtifactNotFound (no leak).
	Delete(ctx context.Context, tenantID uint64, caller ArtifactCaller, id string) error
}
