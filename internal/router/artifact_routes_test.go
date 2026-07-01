package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/handler"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// stubArtifactSvcForRouter is a minimal ArtifactService for route-wiring tests.
type stubArtifactSvcForRouter struct {
	interfaces.ArtifactService
}

func (stubArtifactSvcForRouter) Create(context.Context, uint64, interfaces.ArtifactCaller, interfaces.CreateArtifactParams) (*types.Artifact, error) {
	return &types.Artifact{ID: "art_1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusPending, CreatedAt: time.Now()}, nil
}
func (stubArtifactSvcForRouter) Get(context.Context, uint64, interfaces.ArtifactCaller, string) (*types.Artifact, error) {
	return &types.Artifact{ID: "art_1"}, nil
}
func (stubArtifactSvcForRouter) List(context.Context, uint64, interfaces.ArtifactCaller, int, int) ([]*types.Artifact, int64, error) {
	return nil, 0, nil
}
func (stubArtifactSvcForRouter) ListBySession(context.Context, uint64, interfaces.ArtifactCaller, string) ([]*types.Artifact, error) {
	return nil, nil
}
func (stubArtifactSvcForRouter) UpdateStatus(context.Context, uint64, interfaces.ArtifactCaller, string, types.ArtifactStatus, string, int64) error {
	return nil
}
func (stubArtifactSvcForRouter) Delete(context.Context, uint64, interfaces.ArtifactCaller, string) error {
	return nil
}

// artifactRouteEngine builds an engine with RegisterArtifactRoutes plus a
// stub auth middleware that seeds method + scopes + Admin role (so the role
// guard passes and the assertion isolates the RequireScope group guard).
func artifactRouteEngine(t *testing.T, method string, scopes []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		ctx = context.WithValue(ctx, types.AuthMethodContextKey, method)
		ctx = context.WithValue(ctx, types.APIKeyScopesContextKey, scopes)
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.AuthMethodContextKey.String(), method)
		c.Set(types.APIKeyScopesContextKey.String(), scopes)
		c.Next()
	})
	h := handler.NewArtifactHandler(stubArtifactSvcForRouter{})
	RegisterArtifactRoutes(r.Group("/api/v1"), h, &rbacGuards{})
	return r
}

// TestRegisterArtifactRoutes_APIKeyWithoutChatScopeDenied: an API-key caller
// whose scopes do not cover chat is rejected before the handler runs.
func TestRegisterArtifactRoutes_APIKeyWithoutChatScopeDenied(t *testing.T) {
	r := artifactRouteEngine(t, types.AuthMethodAPIKey, []string{"kb:read"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/art_1", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("api key without chat scope want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRegisterArtifactRoutes_APIKeyWithChatScopeAllowed: an API-key caller
// holding chat reaches the handler.
func TestRegisterArtifactRoutes_APIKeyWithChatScopeAllowed(t *testing.T) {
	r := artifactRouteEngine(t, types.AuthMethodAPIKey, []string{types.ScopeChat})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/art_1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("api key with chat want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRegisterArtifactRoutes_JWTBypassesScope: JWT callers bypass the scope
// check (role guard authorises them).
func TestRegisterArtifactRoutes_JWTBypassesScope(t *testing.T) {
	r := artifactRouteEngine(t, types.AuthMethodJWT, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/artifacts/art_1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("jwt caller want 200, got %d body=%s", w.Code, w.Body.String())
	}
}
