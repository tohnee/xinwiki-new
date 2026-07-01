package router

import (
	"bytes"
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

// stubAPIKeySvcForRouter is a minimal APIKeyService for route-wiring tests.
// It returns canned values so the handler can render a 201/200 without a DB.
type stubAPIKeySvcForRouter struct {
	interfaces.APIKeyService
}

func (stubAPIKeySvcForRouter) Create(context.Context, uint64, string, string, []string, *time.Time) (*types.APIKey, string, error) {
	return &types.APIKey{ID: "ak_1", Name: "t", Prefix: "sk_ab", Scopes: types.StringArray{"kb:read"}, Status: "active", CreatedAt: time.Now()}, "sk_secret", nil
}
func (stubAPIKeySvcForRouter) List(context.Context, uint64) ([]*types.APIKey, error) {
	return nil, nil
}
func (stubAPIKeySvcForRouter) Revoke(context.Context, uint64, string) error { return nil }

// apiKeyRouteEngine builds an engine with RegisterAPIKeyRoutes plus a stub
// "auth" middleware that seeds the auth context (method, scopes, role, tenant)
// the way middleware.Auth does in production. The role is pinned to Admin so
// the per-route g.Admin() guard always passes, isolating the assertion to the
// RequireScope group middleware under test.
func apiKeyRouteEngine(t *testing.T, method string, scopes []string) *gin.Engine {
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
	h := handler.NewAPIKeyHandler(stubAPIKeySvcForRouter{})
	RegisterAPIKeyRoutes(r.Group("/api/v1"), h, &rbacGuards{})
	return r
}

// newCreateAPIKeyReq builds a POST /api-keys request with a valid body so the
// handler can bind + render 201, proving the request cleared the scope guard.
func newCreateAPIKeyReq() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys",
		bytes.NewReader([]byte(`{"name":"t","scopes":["kb:read"]}`)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestRegisterAPIKeyRoutes_APIKeyWithoutScopeDenied: an API-key caller whose
// scopes do not cover admin:apikeys is rejected at the group guard before the
// handler runs.
func TestRegisterAPIKeyRoutes_APIKeyWithoutScopeDenied(t *testing.T) {
	r := apiKeyRouteEngine(t, types.AuthMethodAPIKey, []string{"kb:read"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newCreateAPIKeyReq())
	if w.Code != http.StatusForbidden {
		t.Fatalf("api key without admin:apikeys want 403, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRegisterAPIKeyRoutes_APIKeyWithScopeAllowed: an API-key caller holding
// admin:apikeys passes the scope guard and reaches the handler (201).
func TestRegisterAPIKeyRoutes_APIKeyWithScopeAllowed(t *testing.T) {
	r := apiKeyRouteEngine(t, types.AuthMethodAPIKey, []string{"admin:apikeys"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newCreateAPIKeyReq())
	if w.Code != http.StatusCreated {
		t.Fatalf("api key with admin:apikeys want 201, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRegisterAPIKeyRoutes_APIKeyStarScopeAllowed: the legacy "*" super scope
// (granted to the legacy Tenant.APIKey) covers admin:apikeys, preserving
// backward compatibility for existing tenant-wide keys.
func TestRegisterAPIKeyRoutes_APIKeyStarScopeAllowed(t *testing.T) {
	r := apiKeyRouteEngine(t, types.AuthMethodAPIKey, []string{types.ScopeAll})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newCreateAPIKeyReq())
	if w.Code != http.StatusCreated {
		t.Fatalf("api key with '*' want 201, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRegisterAPIKeyRoutes_JWTBypassesScope: a JWT-authenticated caller is
// authorised by the role guard, not scopes; RequireScope must let it through
// even with an empty scope set.
func TestRegisterAPIKeyRoutes_JWTBypassesScope(t *testing.T) {
	r := apiKeyRouteEngine(t, types.AuthMethodJWT, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newCreateAPIKeyReq())
	if w.Code != http.StatusCreated {
		t.Fatalf("jwt caller want 201, got %d body=%s", w.Code, w.Body.String())
	}
}
