package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/Tencent/XinWiki/internal/application/service"
	apperrors "github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// APIKeyHandler exposes the tenant-scoped CRUD for scoped API keys
// (review 4.5): create (returning the plaintext secret exactly once), list,
// and revoke. Routes are mounted under /api-keys and gated by Admin+ at the
// route layer; RequireScope(admin:apikeys) additionally constrains callers
// authenticating with an API key (JWT callers bypass the scope check).
type APIKeyHandler struct {
	apiKeyService interfaces.APIKeyService
}

// NewAPIKeyHandler wires the service dependency.
func NewAPIKeyHandler(apiKeyService interfaces.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeyService: apiKeyService}
}

// createAPIKeyRequest is the JSON body for POST /api-keys. Name is the
// human label; Scopes is the grant list (validated by the service against
// the known scope set); ExpiresAt is optional.
type createAPIKeyRequest struct {
	Name      string     `json:"name" binding:"required"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// apiKeyListItem is the safe projection for list responses: it deliberately
// omits KeyHash and never carries the plaintext secret (which is not stored
// at all). Only the display Prefix is exposed so operators can recognise a
// key in the UI without re-deriving the secret.
type apiKeyListItem struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func projectAPIKeyListItem(k *types.APIKey) apiKeyListItem {
	return apiKeyListItem{
		ID:         k.ID,
		Name:       k.Name,
		Prefix:     k.Prefix,
		Scopes:     []string(k.Scopes),
		Status:     k.Status,
		ExpiresAt:  k.ExpiresAt,
		LastUsedAt: k.LastUsedAt,
		CreatedAt:  k.CreatedAt,
	}
}

// tenantFromContext resolves the caller's tenant id, rejecting requests that
// arrive without one (unauthenticated) or with the zero value (a bug — no
// real tenant has id 0). Centralised so all three handlers share the same
// 401 semantics.
func tenantFromContext(c *gin.Context) (uint64, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("tenant context missing"))
		return 0, false
	}
	return tenantID, true
}

// CreateAPIKey godoc
// @Summary      创建 API Key
// @Description  为当前租户创建一个带 scope 的可吊销 API Key。明文 secret 仅在本次响应中返回一次，之后只保留哈希。
// @Tags         API Key
// @Accept       json
// @Produce      json
// @Param        request  body  createAPIKeyRequest  true  "创建请求"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /api-keys [post]
func (h *APIKeyHandler) CreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := tenantFromContext(c)
	if !ok {
		return
	}
	userID, _ := types.UserIDFromContext(ctx)

	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}

	key, secret, err := h.apiKeyService.Create(ctx, tenantID, userID, req.Name, req.Scopes, req.ExpiresAt)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAPIKeyNameRequired),
			errors.Is(err, types.ErrInvalidAPIKeyScope):
			c.Error(apperrors.NewValidationError(err.Error()))
		default:
			logger.Errorf(ctx, "CreateAPIKey failed: tenant=%d err=%v", tenantID, err)
			c.Error(apperrors.NewInternalServerError("failed to create api key").WithDetails(err.Error()))
		}
		return
	}

	// secret_shown flags that the plaintext is in THIS response only; the UI
	// must capture it now because subsequent reads (List) never return it.
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"id":           key.ID,
			"name":         key.Name,
			"prefix":       key.Prefix,
			"scopes":       []string(key.Scopes),
			"status":       key.Status,
			"expires_at":   key.ExpiresAt,
			"created_at":   key.CreatedAt,
			"secret":       secret,
			"secret_shown": true,
		},
	})
}

// ListAPIKeys godoc
// @Summary      列出 API Key
// @Description  列出当前租户的所有 API Key（不含明文 secret 与 key_hash）。
// @Tags         API Key
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /api-keys [get]
func (h *APIKeyHandler) ListAPIKeys(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := tenantFromContext(c)
	if !ok {
		return
	}

	keys, err := h.apiKeyService.List(ctx, tenantID)
	if err != nil {
		logger.Errorf(ctx, "ListAPIKeys failed: tenant=%d err=%v", tenantID, err)
		c.Error(apperrors.NewInternalServerError("failed to list api keys").WithDetails(err.Error()))
		return
	}

	items := make([]apiKeyListItem, 0, len(keys))
	for _, k := range keys {
		items = append(items, projectAPIKeyListItem(k))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"api_keys": items,
			"total":    len(items),
		},
	})
}

// RevokeAPIKey godoc
// @Summary      吊销 API Key
// @Description  吊销指定的 API Key；被吊销的 key 立即无法通过 X-API-Key 鉴权。跨租户 key 返回 404（不泄露存在性）。
// @Tags         API Key
// @Produce      json
// @Param        id  path  string  true  "API Key ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /api-keys/{id} [delete]
func (h *APIKeyHandler) RevokeAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := tenantFromContext(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("api key id is required"))
		return
	}

	if err := h.apiKeyService.Revoke(ctx, tenantID, id); err != nil {
		switch {
		case errors.Is(err, service.ErrAPIKeyNotFound):
			c.Error(apperrors.NewNotFoundError("api key not found"))
		default:
			logger.Errorf(ctx, "RevokeAPIKey failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to revoke api key").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
