package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// stubArtifactService is a function-field fake for interfaces.ArtifactService.
type stubArtifactService struct {
	interfaces.ArtifactService
	create        func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, params interfaces.CreateArtifactParams) (*types.Artifact, error)
	get           func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, id string) (*types.Artifact, error)
	list          func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, page, pageSize int) ([]*types.Artifact, int64, error)
	listBySession func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, sessionID string) ([]*types.Artifact, error)
	updateStatus  func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, id string, status types.ArtifactStatus, storageURI string, sizeBytes int64) error
	deleteFn      func(ctx context.Context, tenantID uint64, caller interfaces.ArtifactCaller, id string) error
}

func (s *stubArtifactService) Create(ctx context.Context, t uint64, c interfaces.ArtifactCaller, p interfaces.CreateArtifactParams) (*types.Artifact, error) {
	if s.create != nil {
		return s.create(ctx, t, c, p)
	}
	return nil, nil
}
func (s *stubArtifactService) Get(ctx context.Context, t uint64, c interfaces.ArtifactCaller, id string) (*types.Artifact, error) {
	if s.get != nil {
		return s.get(ctx, t, c, id)
	}
	return nil, nil
}
func (s *stubArtifactService) List(ctx context.Context, t uint64, c interfaces.ArtifactCaller, page, ps int) ([]*types.Artifact, int64, error) {
	if s.list != nil {
		return s.list(ctx, t, c, page, ps)
	}
	return nil, 0, nil
}
func (s *stubArtifactService) ListBySession(ctx context.Context, t uint64, c interfaces.ArtifactCaller, sid string) ([]*types.Artifact, error) {
	if s.listBySession != nil {
		return s.listBySession(ctx, t, c, sid)
	}
	return nil, nil
}
func (s *stubArtifactService) UpdateStatus(ctx context.Context, t uint64, c interfaces.ArtifactCaller, id string, st types.ArtifactStatus, uri string, sz int64) error {
	if s.updateStatus != nil {
		return s.updateStatus(ctx, t, c, id, st, uri, sz)
	}
	return nil
}
func (s *stubArtifactService) Delete(ctx context.Context, t uint64, c interfaces.ArtifactCaller, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, t, c, id)
	}
	return nil
}

// newArtifactTestRouter wires the handler with errorCapture and seeds the
// auth context (tenant + user + role) the way middleware.Auth does. role
// defaults to Contributor (the create floor) unless overridden.
func newArtifactTestRouter(h *ArtifactHandler, userID string, role types.TenantRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/artifacts", h.CreateArtifact)
	r.GET("/artifacts", h.ListArtifacts)
	r.GET("/artifacts/:id", h.GetArtifact)
	r.DELETE("/artifacts/:id", h.DeleteArtifact)
	return r
}

func doArtifact(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		reader = bytes.NewReader(buf)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestArtifactHandler_Create_Returns201: a valid create returns 201 and the
// artifact, with the caller's tenant + identity forwarded to the service.
func TestArtifactHandler_Create_Returns201(t *testing.T) {
	var gotTenant uint64
	var gotUser string
	svc := &stubArtifactService{
		create: func(_ context.Context, tenantID uint64, c interfaces.ArtifactCaller, _ interfaces.CreateArtifactParams) (*types.Artifact, error) {
			gotTenant, gotUser = tenantID, c.UserID
			return &types.Artifact{ID: "art_1", TenantID: tenantID, UserID: c.UserID, Type: types.ArtifactTypePDF, Status: types.ArtifactStatusPending}, nil
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u1", types.TenantRoleContributor), http.MethodPost, "/artifacts",
		map[string]any{"type": "pdf", "title": "Q3", "sharing_policy": "private"})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if gotTenant != 7 || gotUser != "u1" {
		t.Errorf("identity not forwarded: tenant=%d user=%s", gotTenant, gotUser)
	}
}

// TestArtifactHandler_Create_RejectsInvalidType: the service's
// ErrInvalidArtifactType surfaces as 400.
func TestArtifactHandler_Create_RejectsInvalidType(t *testing.T) {
	svc := &stubArtifactService{
		create: func(context.Context, uint64, interfaces.ArtifactCaller, interfaces.CreateArtifactParams) (*types.Artifact, error) {
			return nil, service.ErrInvalidArtifactType
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u1", types.TenantRoleContributor), http.MethodPost, "/artifacts",
		map[string]any{"type": "widget", "sharing_policy": "private"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestArtifactHandler_Get_ForbiddenMapsTo403: ErrArtifactForbidden -> 403.
func TestArtifactHandler_Get_ForbiddenMapsTo403(t *testing.T) {
	svc := &stubArtifactService{
		get: func(context.Context, uint64, interfaces.ArtifactCaller, string) (*types.Artifact, error) {
			return nil, service.ErrArtifactForbidden
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u2", types.TenantRoleViewer), http.MethodGet, "/artifacts/art_1", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestArtifactHandler_Get_NotFoundMapsTo404: ErrArtifactNotFound -> 404.
func TestArtifactHandler_Get_NotFoundMapsTo404(t *testing.T) {
	svc := &stubArtifactService{
		get: func(context.Context, uint64, interfaces.ArtifactCaller, string) (*types.Artifact, error) {
			return nil, service.ErrArtifactNotFound
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u1", types.TenantRoleViewer), http.MethodGet, "/artifacts/art_x", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestArtifactHandler_List_ReturnsPaginated: a successful list returns 200
// with rows + total.
func TestArtifactHandler_List_ReturnsPaginated(t *testing.T) {
	svc := &stubArtifactService{
		list: func(context.Context, uint64, interfaces.ArtifactCaller, int, int) ([]*types.Artifact, int64, error) {
			return []*types.Artifact{{ID: "a1"}, {ID: "a2"}}, 5, nil
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u1", types.TenantRoleViewer), http.MethodGet, "/artifacts?page=1&page_size=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["total"] != float64(5) {
		t.Errorf("total = %v, want 5", data["total"])
	}
}

// TestArtifactHandler_Delete_ForbiddenMapsTo403: non-creator delete -> 403.
func TestArtifactHandler_Delete_ForbiddenMapsTo403(t *testing.T) {
	svc := &stubArtifactService{
		deleteFn: func(context.Context, uint64, interfaces.ArtifactCaller, string) error {
			return service.ErrArtifactForbidden
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u2", types.TenantRoleViewer), http.MethodDelete, "/artifacts/art_1", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestArtifactHandler_Delete_Success: creator delete -> 200.
func TestArtifactHandler_Delete_Success(t *testing.T) {
	called := false
	svc := &stubArtifactService{
		deleteFn: func(context.Context, uint64, interfaces.ArtifactCaller, string) error {
			called = true
			return nil
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	w := doArtifact(t, newArtifactTestRouter(h, "u1", types.TenantRoleContributor), http.MethodDelete, "/artifacts/art_1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !called {
		t.Errorf("service Delete not called")
	}
}

// TestArtifactHandler_Create_MissingTenantContext: without a tenant the
// handler rejects before calling the service (401).
func TestArtifactHandler_Create_MissingTenantContext(t *testing.T) {
	called := false
	svc := &stubArtifactService{
		create: func(context.Context, uint64, interfaces.ArtifactCaller, interfaces.CreateArtifactParams) (*types.Artifact, error) {
			called = true
			return nil, nil
		},
	}
	h := NewArtifactHandler(svc, nil, nil)

	// Engine with NO tenant seeded.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/artifacts", h.CreateArtifact)

	w := doArtifact(t, r, http.MethodPost, "/artifacts", map[string]any{"type": "pdf", "sharing_policy": "private"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	if called {
		t.Errorf("service must not be called without tenant context")
	}
}

// keep fmt import used even if future edits drop a reference.
var _ = fmt.Sprintf
