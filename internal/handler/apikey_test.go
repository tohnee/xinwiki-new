package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// stubAPIKeyService is a function-field fake for interfaces.APIKeyService.
// Embedding the interface satisfies the compiler for any methods a test does
// not override (they panic via the nil interface, surfacing an untested path
// rather than silently passing).
type stubAPIKeyService struct {
	interfaces.APIKeyService
	create func(ctx context.Context, tenantID uint64, userID string, name string, scopes []string, expiresAt *time.Time) (*types.APIKey, string, error)
	list   func(ctx context.Context, tenantID uint64) ([]*types.APIKey, error)
	revoke func(ctx context.Context, tenantID uint64, id string) error
}

func (s *stubAPIKeyService) Create(ctx context.Context, tenantID uint64, userID string, name string, scopes []string, expiresAt *time.Time) (*types.APIKey, string, error) {
	if s.create != nil {
		return s.create(ctx, tenantID, userID, name, scopes, expiresAt)
	}
	return nil, "", nil
}

func (s *stubAPIKeyService) List(ctx context.Context, tenantID uint64) ([]*types.APIKey, error) {
	if s.list != nil {
		return s.list(ctx, tenantID)
	}
	return nil, nil
}

func (s *stubAPIKeyService) Revoke(ctx context.Context, tenantID uint64, id string) error {
	if s.revoke != nil {
		return s.revoke(ctx, tenantID, id)
	}
	return nil
}

// newAPIKeyTestRouter wires the handler with errorCapture (the same test-only
// apperror renderer the other handler tests use) and seeds the auth context
// (tenantID + userID) the way middleware.Auth would in production.
func newAPIKeyTestRouter(h *APIKeyHandler, tenantID uint64, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorCapture())
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
		if userID != "" {
			ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Set(types.UserIDContextKey.String(), userID)
		c.Next()
	})
	r.POST("/api-keys", h.CreateAPIKey)
	r.GET("/api-keys", h.ListAPIKeys)
	r.DELETE("/api-keys/:id", h.RevokeAPIKey)
	return r
}

func doAPIKey(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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

// TestAPIKeyHandler_Create_ReturnsSecretOnce: a successful create returns 201
// and the plaintext secret exactly once; the service receives the parsed
// fields and the caller's tenant/user identity.
func TestAPIKeyHandler_Create_ReturnsSecretOnce(t *testing.T) {
	var gotTenant uint64
	var gotUser, gotName string
	var gotScopes []string
	svc := &stubAPIKeyService{
		create: func(_ context.Context, tenantID uint64, userID string, name string, scopes []string, _ *time.Time) (*types.APIKey, string, error) {
			gotTenant, gotUser, gotName, gotScopes = tenantID, userID, name, scopes
			return &types.APIKey{ID: "ak_1", TenantID: tenantID, Name: name, Prefix: "sk_abcd", Scopes: types.StringArray(scopes), Status: "active", CreatedAt: time.Now()}, "sk_plaintext_secret", nil
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodPost, "/api-keys",
		map[string]any{"name": "CI ingest", "scopes": []string{"kb:read", "doc:write"}})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if gotTenant != 7 || gotUser != "u1" || gotName != "CI ingest" {
		t.Errorf("service received wrong identity: tenant=%d user=%s name=%q", gotTenant, gotUser, gotName)
	}
	if len(gotScopes) != 2 || gotScopes[0] != "kb:read" || gotScopes[1] != "doc:write" {
		t.Errorf("scopes not forwarded: %v", gotScopes)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["secret"] != "sk_plaintext_secret" {
		t.Errorf("secret not returned once, got %v", data["secret"])
	}
	if data["secret_shown"] != true {
		t.Errorf("secret_shown flag missing, got %v", data["secret_shown"])
	}
	if data["id"] != "ak_1" || data["prefix"] != "sk_abcd" {
		t.Errorf("id/prefix not projected: %+v", data)
	}
	if data["key_hash"] != nil {
		t.Errorf("key_hash must never be returned, got %v", data["key_hash"])
	}
}

// TestAPIKeyHandler_Create_RejectsEmptyName: a blank name is rejected by
// binding validation (required) before the service is touched.
func TestAPIKeyHandler_Create_RejectsEmptyName(t *testing.T) {
	called := false
	svc := &stubAPIKeyService{
		create: func(context.Context, uint64, string, string, []string, *time.Time) (*types.APIKey, string, error) {
			called = true
			return nil, "", nil
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodPost, "/api-keys",
		map[string]any{"name": "", "scopes": []string{"kb:read"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d body=%s", w.Code, w.Body.String())
	}
	if called {
		t.Errorf("service must not be called on binding failure")
	}
}

// TestAPIKeyHandler_Create_RejectsInvalidScope: an unknown scope surfaces as
// 400 (validation). Scope validation lives in the service (single source of
// truth), so the service IS called and returns types.ErrInvalidAPIKeyScope.
func TestAPIKeyHandler_Create_RejectsInvalidScope(t *testing.T) {
	called := false
	svc := &stubAPIKeyService{
		create: func(context.Context, uint64, string, string, []string, *time.Time) (*types.APIKey, string, error) {
			called = true
			return nil, "", fmt.Errorf("%w: %q", types.ErrInvalidAPIKeyScope, "foo:bar")
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodPost, "/api-keys",
		map[string]any{"name": "CI", "scopes": []string{"foo:bar"}})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad scope, got %d body=%s", w.Code, w.Body.String())
	}
	if !called {
		t.Errorf("service must be called (it owns scope validation)")
	}
}

// TestAPIKeyHandler_Create_RepoErrorMapsTo500: a non-sentinel service error
// surfaces as 500.
func TestAPIKeyHandler_Create_RepoErrorMapsTo500(t *testing.T) {
	svc := &stubAPIKeyService{
		create: func(context.Context, uint64, string, string, []string, *time.Time) (*types.APIKey, string, error) {
			return nil, "", errors.New("db down")
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodPost, "/api-keys",
		map[string]any{"name": "CI", "scopes": []string{"kb:read"}})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAPIKeyHandler_List_OmitsSecretAndHash: the list response carries no
// plaintext secret and no key_hash on any row, regardless of repo contents.
func TestAPIKeyHandler_List_OmitsSecretAndHash(t *testing.T) {
	svc := &stubAPIKeyService{
		list: func(context.Context, uint64) ([]*types.APIKey, error) {
			return []*types.APIKey{
				{ID: "ak_1", TenantID: 7, Name: "CI", Prefix: "sk_aa", Scopes: types.StringArray{"kb:read"}, Status: "active", KeyHash: "SECRET-HASH", CreatedAt: time.Now()},
				{ID: "ak_2", TenantID: 7, Name: "Old", Prefix: "sk_bb", Scopes: types.StringArray{"*"}, Status: "revoked", KeyHash: "SECRET-HASH-2", CreatedAt: time.Now()},
			}, nil
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodGet, "/api-keys", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	keys, _ := data["api_keys"].([]any)
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if data["total"] != float64(2) {
		t.Errorf("total = %v, want 2", data["total"])
	}
	for i, k := range keys {
		row, _ := k.(map[string]any)
		if row["secret"] != nil {
			t.Errorf("row %d leaked secret", i)
		}
		if row["key_hash"] != nil {
			t.Errorf("row %d leaked key_hash", i)
		}
		if row["id"] == "" {
			t.Errorf("row %d missing id", i)
		}
	}
}

// TestAPIKeyHandler_Revoke_Success: revoking an owned key returns 200.
func TestAPIKeyHandler_Revoke_Success(t *testing.T) {
	var gotID string
	svc := &stubAPIKeyService{
		revoke: func(_ context.Context, _ uint64, id string) error {
			gotID = id
			return nil
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodDelete, "/api-keys/ak_9", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if gotID != "ak_9" {
		t.Errorf("id not forwarded: %q", gotID)
	}
}

// TestAPIKeyHandler_Revoke_NotFoundMapsTo404: service.ErrAPIKeyNotFound surfaces
// as 404.
func TestAPIKeyHandler_Revoke_NotFoundMapsTo404(t *testing.T) {
	svc := &stubAPIKeyService{
		revoke: func(context.Context, uint64, string) error {
			return service.ErrAPIKeyNotFound
		},
	}
	h := NewAPIKeyHandler(svc)

	w := doAPIKey(t, newAPIKeyTestRouter(h, 7, "u1"), http.MethodDelete, "/api-keys/ak_x", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAPIKeyHandler_Revoke_MissingTenantContext: with no tenant in the auth
// context, the handler rejects before calling the service (401).
func TestAPIKeyHandler_Revoke_MissingTenantContext(t *testing.T) {
	called := false
	svc := &stubAPIKeyService{
		revoke: func(context.Context, uint64, string) error {
			called = true
			return nil
		},
	}
	h := NewAPIKeyHandler(svc)

	// tenantID=0 + empty user simulates an unauthenticated request.
	w := doAPIKey(t, newAPIKeyTestRouter(h, 0, ""), http.MethodDelete, "/api-keys/ak_9", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	if called {
		t.Errorf("service must not be called without tenant context")
	}
}
