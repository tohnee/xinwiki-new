package service

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	secutils "github.com/Tencent/XinWiki/internal/utils"
	"gorm.io/gorm"
)

// Sentinel errors returned by artifactService. Callers compare with
// errors.Is to render the appropriate HTTP response.
var (
	// ErrInvalidArtifactType is returned by Create when the artifact kind is
	// not one of the declared ArtifactType constants.
	ErrInvalidArtifactType = errors.New("invalid artifact type")

	// ErrInvalidArtifactSharingPolicy is returned by Create when the sharing
	// policy is not one of the declared constants.
	ErrInvalidArtifactSharingPolicy = errors.New("invalid artifact sharing policy")

	// ErrArtifactNotFound is returned by Get/UpdateStatus/Delete when no
	// non-deleted artifact matches the id within the caller's tenant.
	// Cross-tenant ids collapse into this same sentinel so existence is not
	// leaked across tenants.
	ErrArtifactNotFound = errors.New("artifact not found")

	// ErrArtifactForbidden is returned by Get/UpdateStatus/Delete when the
	// artifact exists in the tenant but the caller lacks read/modify
	// permission (not creator, not Admin+, and not on the allowed list).
	ErrArtifactForbidden = errors.New("artifact access forbidden")
)

// artifactService implements interfaces.ArtifactService. It owns the
// generate-id -> stamp -> persist flow on Create and the per-user ACL +
// creator-or-admin modify guard on every read/mutation; the repository
// handles persistence only.
type artifactService struct {
	repo interfaces.ArtifactRepository
	// now is an injection seam so tests can pin timestamps without touching
	// wall-clock state. Production wires time.Now.
	now func() time.Time
}

// NewArtifactService wires the repository. The service is stateless beyond
// the repo handle, so a single instance is safe to share.
func NewArtifactService(repo interfaces.ArtifactRepository) interfaces.ArtifactService {
	return &artifactService{repo: repo, now: time.Now}
}

// Create mints a pending artifact owned by the caller. Validates the type +
// sharing policy before touching the repo.
func (s *artifactService) Create(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	params interfaces.CreateArtifactParams,
) (*types.Artifact, error) {
	if !params.Type.IsValid() {
		return nil, ErrInvalidArtifactType
	}
	if !params.SharingPolicy.IsValid() {
		return nil, ErrInvalidArtifactSharingPolicy
	}
	now := s.now()
	a := &types.Artifact{
		ID:                secutils.GenerateArtifactID(),
		TenantID:          tenantID,
		UserID:            caller.UserID,
		SessionID:         params.SessionID,
		Type:              params.Type,
		Status:            types.ArtifactStatusPending,
		Title:             params.Title,
		SourceKBID:        params.SourceKBID,
		SourceKnowledgeID: params.SourceKnowledgeID,
		SourceWikiPageID:  params.SourceWikiPageID,
		SharingPolicy:     params.SharingPolicy,
		AllowedUserIDs:    types.StringArray(params.AllowedUserIDs),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// Get returns the artifact if the caller may read it; otherwise
// ErrArtifactForbidden. A missing or cross-tenant id returns
// ErrArtifactNotFound.
func (s *artifactService) Get(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	id string,
) (*types.Artifact, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, err
	}
	if a == nil || a.TenantID != tenantID {
		return nil, ErrArtifactNotFound
	}
	if !types.CanAccessArtifact(a, caller.UserID, caller.Role, caller.IsSystemAdmin) {
		return nil, ErrArtifactForbidden
	}
	return a, nil
}

// List returns the caller-visible artifacts for the tenant, paginated (newest
// first), plus the total count of visible artifacts. ACL filtering is applied
// in-memory over the tenant's rows; for high-volume tenants the ACL can later
// move into the SQL query.
func (s *artifactService) List(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	page, pageSize int,
) ([]*types.Artifact, int64, error) {
	rows, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	visible := make([]*types.Artifact, 0, len(rows))
	for _, a := range rows {
		if types.CanAccessArtifact(a, caller.UserID, caller.Role, caller.IsSystemAdmin) {
			visible = append(visible, a)
		}
	}
	total := int64(len(visible))

	// Guard against non-positive paging input the same way the rest of the
	// API does (treat page<1 as 1, pageSize<=0 as a sane default).
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	if offset >= len(visible) {
		return []*types.Artifact{}, total, nil
	}
	end := offset + pageSize
	if end > len(visible) {
		end = len(visible)
	}
	return visible[offset:end], total, nil
}

// ListBySession returns the caller-visible artifacts produced by a session
// within the tenant.
func (s *artifactService) ListBySession(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	sessionID string,
) ([]*types.Artifact, error) {
	rows, err := s.repo.ListBySession(ctx, tenantID, sessionID)
	if err != nil {
		return nil, err
	}
	visible := make([]*types.Artifact, 0, len(rows))
	for _, a := range rows {
		if types.CanAccessArtifact(a, caller.UserID, caller.Role, caller.IsSystemAdmin) {
			visible = append(visible, a)
		}
	}
	return visible, nil
}

// UpdateStatus transitions the artifact's lifecycle. Creator-or-admin only;
// the generation pipeline uses this to flip pending -> ready / failed.
func (s *artifactService) UpdateStatus(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	id string,
	status types.ArtifactStatus,
	storageURI string,
	sizeBytes int64,
) error {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrArtifactNotFound
		}
		return err
	}
	if a == nil || a.TenantID != tenantID {
		return ErrArtifactNotFound
	}
	if !canModifyArtifact(a, caller) {
		return ErrArtifactForbidden
	}
	return s.repo.UpdateStatus(ctx, id, status, storageURI, sizeBytes)
}

// Delete soft-deletes the artifact. Creator-or-admin only.
func (s *artifactService) Delete(
	ctx context.Context,
	tenantID uint64,
	caller interfaces.ArtifactCaller,
	id string,
) error {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrArtifactNotFound
		}
		return err
	}
	if a == nil || a.TenantID != tenantID {
		return ErrArtifactNotFound
	}
	if !canModifyArtifact(a, caller) {
		return ErrArtifactForbidden
	}
	return s.repo.SoftDelete(ctx, tenantID, id)
}

// canModifyArtifact reports whether the caller may mutate the artifact:
// system admin, tenant Admin+, or the creator. Unlike reads, modify is never
// granted via the explicit-sharing allowed list — being allowed to view a
// shared artifact does not confer edit/delete rights.
func canModifyArtifact(a *types.Artifact, caller interfaces.ArtifactCaller) bool {
	if caller.IsSystemAdmin || caller.Role.HasPermission(types.TenantRoleAdmin) {
		return true
	}
	return caller.UserID != "" && a.UserID == caller.UserID
}
