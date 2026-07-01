package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

// fakeArtifactRepo is an in-memory stand-in for interfaces.ArtifactRepository.
// Embedding the interface satisfies the compiler for any unused methods.
type fakeArtifactRepo struct {
	interfaces.ArtifactRepository
	rows      []*types.Artifact
	createErr error
}

func (r *fakeArtifactRepo) Create(_ context.Context, a *types.Artifact) error {
	if r.createErr != nil {
		return r.createErr
	}
	cp := *a
	r.rows = append(r.rows, &cp)
	return nil
}

func (r *fakeArtifactRepo) GetByID(_ context.Context, id string) (*types.Artifact, error) {
	for _, a := range r.rows {
		if a.ID == id && a.DeletedAt == nil {
			cp := *a
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeArtifactRepo) ListByTenant(_ context.Context, tenantID uint64) ([]*types.Artifact, error) {
	var out []*types.Artifact
	for _, a := range r.rows {
		if a.TenantID == tenantID && a.DeletedAt == nil {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeArtifactRepo) ListBySession(_ context.Context, tenantID uint64, sessionID string) ([]*types.Artifact, error) {
	var out []*types.Artifact
	for _, a := range r.rows {
		if a.TenantID == tenantID && a.SessionID == sessionID && a.DeletedAt == nil {
			cp := *a
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeArtifactRepo) UpdateStatus(_ context.Context, id string, status types.ArtifactStatus, storageURI string, sizeBytes int64) error {
	for _, a := range r.rows {
		if a.ID == id {
			a.Status = status
			a.StorageURI = storageURI
			a.SizeBytes = sizeBytes
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeArtifactRepo) SoftDelete(_ context.Context, tenantID uint64, id string) error {
	for _, a := range r.rows {
		if a.ID == id && a.TenantID == tenantID {
			a.DeletedAt = timePtr(time.Now())
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func viewerCaller(userID string) interfaces.ArtifactCaller {
	return interfaces.ArtifactCaller{UserID: userID, Role: types.TenantRoleViewer}
}
func adminCaller(userID string) interfaces.ArtifactCaller {
	return interfaces.ArtifactCaller{UserID: userID, Role: types.TenantRoleAdmin}
}

// TestArtifactService_Create_MintsPendingArtifactOwnedByCaller: Create stamps
// tenant + creator, defaults status to pending, and persists.
func TestArtifactService_Create_MintsPendingArtifactOwnedByCaller(t *testing.T) {
	repo := &fakeArtifactRepo{}
	svc := NewArtifactService(repo)

	a, err := svc.Create(context.Background(), 7, viewerCaller("u1"), interfaces.CreateArtifactParams{
		Type:          types.ArtifactTypePDF,
		Title:         "Q3 report",
		SessionID:     "s1",
		SourceKBID:    "kb1",
		SharingPolicy: types.ArtifactSharingPrivate,
	})
	requireNoArtifactErr(t, err)
	if a.TenantID != 7 || a.UserID != "u1" {
		t.Errorf("tenant/creator not stamped: %+v", a)
	}
	if a.Status != types.ArtifactStatusPending {
		t.Errorf("status = %q, want pending", a.Status)
	}
	if a.ID == "" {
		t.Errorf("ID must be generated")
	}
	if len(repo.rows) != 1 || repo.rows[0].ID != a.ID {
		t.Errorf("artifact must be persisted once")
	}
}

// TestArtifactService_Create_RejectsInvalidType: an unknown type is rejected
// before the repo is touched.
func TestArtifactService_Create_RejectsInvalidType(t *testing.T) {
	repo := &fakeArtifactRepo{}
	svc := NewArtifactService(repo)

	_, err := svc.Create(context.Background(), 7, viewerCaller("u1"), interfaces.CreateArtifactParams{
		Type:          types.ArtifactType("widget"),
		SharingPolicy: types.ArtifactSharingPrivate,
	})
	if !errors.Is(err, ErrInvalidArtifactType) {
		t.Errorf("invalid type want ErrInvalidArtifactType, got %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("repo must not be touched on validation failure")
	}
}

// TestArtifactService_Create_RejectsInvalidSharingPolicy: an unknown policy
// is rejected.
func TestArtifactService_Create_RejectsInvalidSharingPolicy(t *testing.T) {
	repo := &fakeArtifactRepo{}
	svc := NewArtifactService(repo)

	_, err := svc.Create(context.Background(), 7, viewerCaller("u1"), interfaces.CreateArtifactParams{
		Type:          types.ArtifactTypePDF,
		SharingPolicy: types.ArtifactSharingPolicy("public"),
	})
	if !errors.Is(err, ErrInvalidArtifactSharingPolicy) {
		t.Errorf("invalid policy want ErrInvalidArtifactSharingPolicy, got %v", err)
	}
}

// TestArtifactService_Get_CreatorAllowed: the creator may read their own
// private artifact.
func TestArtifactService_Get_CreatorAllowed(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingPrivate},
	}}
	svc := NewArtifactService(repo)

	a, err := svc.Get(context.Background(), 7, viewerCaller("u1"), "a1")
	requireNoArtifactErr(t, err)
	if a.ID != "a1" {
		t.Errorf("wrong artifact returned")
	}
}

// TestArtifactService_Get_NonCreatorPrivateForbidden: a non-creator viewer
// cannot read a private artifact (403, not 404, since they're in the tenant).
func TestArtifactService_Get_NonCreatorPrivateForbidden(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingPrivate},
	}}
	svc := NewArtifactService(repo)

	_, err := svc.Get(context.Background(), 7, viewerCaller("u2"), "a1")
	if !errors.Is(err, ErrArtifactForbidden) {
		t.Errorf("non-creator private want ErrArtifactForbidden, got %v", err)
	}
}

// TestArtifactService_Get_CrossTenantNotFound: an id from another tenant
// returns ErrArtifactNotFound (no existence leak), not forbidden.
func TestArtifactService_Get_CrossTenantNotFound(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 2, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingTenant},
	}}
	svc := NewArtifactService(repo)

	_, err := svc.Get(context.Background(), 7, viewerCaller("u1"), "a1")
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("cross-tenant want ErrArtifactNotFound, got %v", err)
	}
}

// TestArtifactService_Get_NotFound: a missing id returns ErrArtifactNotFound.
func TestArtifactService_Get_NotFound(t *testing.T) {
	repo := &fakeArtifactRepo{}
	svc := NewArtifactService(repo)

	_, err := svc.Get(context.Background(), 7, viewerCaller("u1"), "nope")
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("missing want ErrArtifactNotFound, got %v", err)
	}
}

// TestArtifactService_List_ACLFiltersAndPaginates: a viewer sees only their
// own + tenant-shared artifacts; admin sees all; pagination respects total.
func TestArtifactService_List_ACLFiltersAndPaginates(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingPrivate},
		{ID: "a2", TenantID: 7, UserID: "u2", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingPrivate},
		{ID: "a3", TenantID: 7, UserID: "u2", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingTenant},
		// Different tenant — must not appear at all.
		{ID: "x1", TenantID: 9, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingTenant},
	}}
	svc := NewArtifactService(repo)

	// u1 (viewer) sees a1 (own private) + a3 (tenant-shared) = 2.
	rows, total, err := svc.List(context.Background(), 7, viewerCaller("u1"), 1, 10)
	requireNoArtifactErr(t, err)
	if total != 2 || len(rows) != 2 {
		t.Fatalf("u1 want 2 visible (total=%d, got=%d)", total, len(rows))
	}
	// u2 (viewer) sees a2 (own private) + a3 (tenant-shared) = 2.
	rows, total, err = svc.List(context.Background(), 7, viewerCaller("u2"), 1, 10)
	requireNoArtifactErr(t, err)
	if total != 2 {
		t.Fatalf("u2 want 2 visible, got %d", total)
	}
	// admin sees all 3 in tenant 7.
	_, total, err = svc.List(context.Background(), 7, adminCaller("admin"), 1, 10)
	requireNoArtifactErr(t, err)
	if total != 3 {
		t.Errorf("admin want 3 visible (all tenant), got %d", total)
	}
}

// TestArtifactService_List_Pagination: page size limits the returned rows
// while total reflects all visible.
func TestArtifactService_List_Pagination(t *testing.T) {
	repo := &fakeArtifactRepo{}
	for i := 0; i < 5; i++ {
		repo.rows = append(repo.rows, &types.Artifact{ID: "a", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF, SharingPolicy: types.ArtifactSharingPrivate})
	}
	svc := NewArtifactService(repo)

	rows, total, err := svc.List(context.Background(), 7, viewerCaller("u1"), 1, 2)
	requireNoArtifactErr(t, err)
	if total != 5 || len(rows) != 2 {
		t.Fatalf("page 1 want total=5 got=2, got total=%d rows=%d", total, len(rows))
	}
	// Page 3 (offset 4) returns the last item.
	rows, _, err = svc.List(context.Background(), 7, viewerCaller("u1"), 3, 2)
	requireNoArtifactErr(t, err)
	if len(rows) != 1 {
		t.Errorf("page 3 want 1 row, got %d", len(rows))
	}
	// Page beyond range returns empty.
	rows, _, err = svc.List(context.Background(), 7, viewerCaller("u1"), 10, 2)
	requireNoArtifactErr(t, err)
	if len(rows) != 0 {
		t.Errorf("out-of-range page want 0 rows, got %d", len(rows))
	}
}

// TestArtifactService_UpdateStatus_CreatorOnly: the creator may flip status;
// another user may not.
func TestArtifactService_UpdateStatus_CreatorOnly(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusPending},
	}}
	svc := NewArtifactService(repo)

	requireNoArtifactErr(t, svc.UpdateStatus(context.Background(), 7, viewerCaller("u1"), "a1", types.ArtifactStatusReady, "s3://a1", 100))
	if repo.rows[0].Status != types.ArtifactStatusReady {
		t.Errorf("status not updated")
	}

	err := svc.UpdateStatus(context.Background(), 7, viewerCaller("u2"), "a1", types.ArtifactStatusFailed, "", 0)
	if !errors.Is(err, ErrArtifactForbidden) {
		t.Errorf("non-creator update want ErrArtifactForbidden, got %v", err)
	}
}

// TestArtifactService_Delete_CreatorOnly: creator may delete; another user
// may not; cross-tenant id returns NotFound.
func TestArtifactService_Delete_CreatorOnly(t *testing.T) {
	repo := &fakeArtifactRepo{rows: []*types.Artifact{
		{ID: "a1", TenantID: 7, UserID: "u1", Type: types.ArtifactTypePDF},
	}}
	svc := NewArtifactService(repo)

	err := svc.Delete(context.Background(), 7, viewerCaller("u2"), "a1")
	if !errors.Is(err, ErrArtifactForbidden) {
		t.Errorf("non-creator delete want ErrArtifactForbidden, got %v", err)
	}
	requireNoArtifactErr(t, svc.Delete(context.Background(), 7, viewerCaller("u1"), "a1"))
	if repo.rows[0].DeletedAt == nil {
		t.Errorf("artifact not soft-deleted")
	}
	// Already deleted -> GetByID returns NotFound -> Delete returns NotFound.
	err = svc.Delete(context.Background(), 7, viewerCaller("u1"), "a1")
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("re-delete want ErrArtifactNotFound, got %v", err)
	}
}

func requireNoArtifactErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
