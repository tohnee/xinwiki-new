package handler

import (
	"net/http"

	"github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type ModelRouterHandler struct {
	modelRouterService interfaces.ModelRouterService
	promptService      interfaces.PromptTemplateService
}

func NewModelRouterHandler(
	modelRouterService interfaces.ModelRouterService,
	promptService interfaces.PromptTemplateService,
) *ModelRouterHandler {
	return &ModelRouterHandler{
		modelRouterService: modelRouterService,
		promptService:      promptService,
	}
}

type modelRouterResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func (h *ModelRouterHandler) SelectModel(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var req types.ModelSelectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	req.TenantID = tenantID

	result, err := h.modelRouterService.SelectModel(ctx, &req)
	if err != nil {
		c.Error(errors.Internal("failed to select model", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    result,
	})
}

func (h *ModelRouterHandler) GetRoutingPolicy(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	policy, err := h.modelRouterService.GetRoutingPolicy(ctx, tenantID)
	if err != nil {
		c.Error(errors.Internal("failed to get routing policy", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    policy,
	})
}

func (h *ModelRouterHandler) UpdateRoutingPolicy(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var policy types.ModelRoutingPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	policy.TenantID = tenantID

	err := h.modelRouterService.UpdateRoutingPolicy(ctx, &policy)
	if err != nil {
		c.Error(errors.Internal("failed to update routing policy", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    gin.H{"message": "policy updated"},
	})
}

func (h *ModelRouterHandler) CreatePromptTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var tmpl types.PromptTemplate
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	tmpl.TenantID = tenantID

	err := h.promptService.CreateTemplate(ctx, &tmpl)
	if err != nil {
		c.Error(errors.Internal("failed to create prompt template", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    tmpl,
	})
}

func (h *ModelRouterHandler) GetPromptTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	tmplKey := c.Param("template_id")
	if tmplKey == "" {
		c.Error(errors.BadRequest("template id is required", nil))
		return
	}

	version := c.Query("version")
	if version == "" {
		tmpl, err := h.promptService.GetActiveTemplate(ctx, tenantID, tmplKey)
		if err != nil {
			c.Error(errors.Internal("failed to get prompt template", err))
			return
		}
		c.JSON(http.StatusOK, modelRouterResponse{
			Success: true,
			Data:    tmpl,
		})
		return
	}

	tmpl, err := h.promptService.GetTemplate(ctx, tenantID, tmplKey, version)
	if err != nil {
		c.Error(errors.Internal("failed to get prompt template", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    tmpl,
	})
}

func (h *ModelRouterHandler) ListPromptTemplates(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	tmplKey := c.Query("template_key")
	if tmplKey == "" {
		c.Error(errors.BadRequest("template_key query parameter is required", nil))
		return
	}

	templates, err := h.promptService.ListTemplateVersions(ctx, tenantID, tmplKey)
	if err != nil {
		c.Error(errors.Internal("failed to list prompt templates", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data: gin.H{
			"items": templates,
			"total": len(templates),
		},
	})
}

func (h *ModelRouterHandler) ActivatePromptVersion(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	tmplKey := c.Param("template_id")
	if tmplKey == "" {
		c.Error(errors.BadRequest("template id is required", nil))
		return
	}

	var req struct {
		Version string `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	err := h.promptService.ActivateVersion(ctx, tenantID, tmplKey, req.Version)
	if err != nil {
		c.Error(errors.Internal("failed to activate version", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data:    gin.H{"message": "version activated"},
	})
}

func (h *ModelRouterHandler) RenderPrompt(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	tmplKey := c.Param("template_id")
	if tmplKey == "" {
		c.Error(errors.BadRequest("template id is required", nil))
		return
	}

	version := c.Query("version")
	var vars map[string]string
	if err := c.ShouldBindJSON(&vars); err != nil {
		vars = make(map[string]string)
	}

	rendered, renderedVersion, err := h.promptService.RenderTemplate(ctx, tenantID, tmplKey, version, vars)
	if err != nil {
		c.Error(errors.Internal("failed to render prompt", err))
		return
	}

	c.JSON(http.StatusOK, modelRouterResponse{
		Success: true,
		Data: gin.H{
			"rendered": rendered,
			"version":  renderedVersion,
		},
	})
}
