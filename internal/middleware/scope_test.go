package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// runScopeGuard builds a gin engine whose only handler is the RequireScope
// guard for `required`, followed by a 200 handler. It seeds the request
// context with the given auth method + scopes (mimicking what the auth
// middleware does) and returns the response status code.
func runScopeGuard(t *testing.T, method string, scopes []string, required string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.AuthMethodContextKey.String(), method)
		c.Set(types.APIKeyScopesContextKey.String(), scopes)
		c.Next()
	})
	r.GET("/x", RequireScope(required), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.ServeHTTP(w, req)
	return w.Code
}

func TestRequireScope_APIKeyAllowed(t *testing.T) {
	// API key with a covering wildcard scope -> 200.
	code := runScopeGuard(t, types.AuthMethodAPIKey, []string{"kb:*"}, "kb:read")
	require.Equal(t, http.StatusOK, code, "kb:* should allow kb:read")
}

func TestRequireScope_APIKeyDenied(t *testing.T) {
	// API key whose scopes do not cover the required scope -> 403.
	code := runScopeGuard(t, types.AuthMethodAPIKey, []string{"kb:read"}, "kb:write")
	require.Equal(t, http.StatusForbidden, code, "kb:read should not allow kb:write")
}

func TestRequireScope_APIKeyEmptyScopesDenied(t *testing.T) {
	// API key with no scopes -> fail-closed 403.
	code := runScopeGuard(t, types.AuthMethodAPIKey, nil, "kb:read")
	require.Equal(t, http.StatusForbidden, code, "empty scopes should fail-closed")
}

func TestRequireScope_JWTBypassesScopeCheck(t *testing.T) {
	// JWT-authenticated requests are authorized by role guards, not scopes;
	// RequireScope must let them through regardless of scopes.
	code := runScopeGuard(t, types.AuthMethodJWT, nil, "kb:read")
	require.Equal(t, http.StatusOK, code, "JWT auth should bypass scope check")
}

func TestRequireScope_NoAuthMethodBypasses(t *testing.T) {
	// Unauthenticated request (no method set) -> no scope enforcement. The
	// auth middleware runs before RequireScope on real routes, so reaching
	// RequireScope without a method means the route is public or auth failed
	// upstream; scope enforcement does not apply.
	code := runScopeGuard(t, "", nil, "kb:read")
	require.Equal(t, http.StatusOK, code, "no auth method should not trigger scope enforcement")
}

func TestRequireScope_SuperScopeAllowed(t *testing.T) {
	code := runScopeGuard(t, types.AuthMethodAPIKey, []string{"*"}, "admin:delete")
	require.Equal(t, http.StatusOK, code, "\"*\" should allow any scope")
}
