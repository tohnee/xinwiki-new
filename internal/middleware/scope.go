package middleware

import (
	"net/http"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/gin-gonic/gin"
)

// RequireScope returns a gin guard that enforces API-key scopes. It applies
// ONLY to requests authenticated via an API key (AuthMethod == "apikey"):
//
//   - API-key request whose granted scopes do not cover `required` is denied
//     with 403 (fail-closed — empty scopes deny).
//   - JWT-authenticated requests pass through: interactive users are
//     authorized by role/ownership guards (RequireRole / RequireOwnershipOrRole
//     / RequireKBAccess), not by API-key scopes. Layering scopes on top of
//     JWT would double-gate and break existing routes.
//   - Requests with no auth method set pass through: on a real route the auth
//     middleware runs first and sets the method, so reaching RequireScope
//     without one means the route is public or auth already failed upstream.
//
// Place RequireScope AFTER the auth middleware and BEFORE the handler. Routes
// that mutate data should declare a write scope (e.g. RequireScope(types.ScopeKBWrite));
// read routes a read scope.
func RequireScope(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		method, _ := c.Get(types.AuthMethodContextKey.String())
		methodStr, _ := method.(string)
		if methodStr != types.AuthMethodAPIKey {
			// Not an API-key request: scope enforcement does not apply.
			c.Next()
			return
		}

		granted, _ := c.Get(types.APIKeyScopesContextKey.String())
		grantedScopes, _ := granted.([]string)
		if !types.ScopesAllow(grantedScopes, required) {
			logger.Warnf(c.Request.Context(),
				"[scope] api key denied: required=%s granted=%v path=%s",
				required, granted, c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: API key lacks required scope: " + required,
			})
			return
		}
		c.Next()
	}
}
