package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Tencent/XinWiki/internal/artifact"
	"github.com/Tencent/XinWiki/internal/application/service"
	apperrors "github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// ArtifactHandler exposes the generated-artifact CRUD (review 4.2): create,
// read (ACL-filtered), list, session-scoped list, lifecycle update, and
// delete. The service layer enforces the per-user ACL + creator-or-admin
// modify guard; the handler is thin glue that builds the caller identity from
// the auth context and maps service sentinels to HTTP codes.
type ArtifactHandler struct {
	artifactService   interfaces.ArtifactService
	fileService       interfaces.FileService
	generationService *artifact.GenerationService
}

// NewArtifactHandler wires the service dependency. Generation and file
// services are optional: when nil, the /generate and /download endpoints
// return 501 (registered but not wired in this build).
func NewArtifactHandler(
	artifactService interfaces.ArtifactService,
	fileService interfaces.FileService,
	generationService *artifact.GenerationService,
) *ArtifactHandler {
	return &ArtifactHandler{
		artifactService:   artifactService,
		fileService:       fileService,
		generationService: generationService,
	}
}

// createArtifactRequest is the JSON body for POST /artifacts. Type and
// sharing_policy are validated by the service.
type createArtifactRequest struct {
	SessionID         string   `json:"session_id"`
	Type              string   `json:"type" binding:"required"`
	Title             string   `json:"title"`
	SourceKBID        string   `json:"source_kb_id"`
	SourceKnowledgeID string   `json:"source_knowledge_id"`
	SourceWikiPageID  string   `json:"source_wiki_page_id"`
	SharingPolicy     string   `json:"sharing_policy"`
	AllowedUserIDs    []string `json:"allowed_user_ids"`
}

// updateArtifactStatusRequest is the JSON body for PUT /artifacts/:id/status.
type updateArtifactStatusRequest struct {
	Status     string `json:"status" binding:"required"`
	StorageURI string `json:"storage_uri"`
	SizeBytes  int64  `json:"size_bytes"`
}

// artifactCallerFromContext resolves the caller identity + tenant, rejecting
// requests that arrive without a tenant (unauthenticated) or with the zero
// value. Shared by every handler method.
func artifactCallerFromContext(c *gin.Context) (interfaces.ArtifactCaller, uint64, bool) {
	ctx := c.Request.Context()
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		c.Error(apperrors.NewUnauthorizedError("tenant context missing"))
		return interfaces.ArtifactCaller{}, 0, false
	}
	userID, _ := types.UserIDFromContext(ctx)
	return interfaces.ArtifactCaller{
		UserID:        userID,
		Role:          types.TenantRoleFromContext(ctx),
		IsSystemAdmin: types.IsSystemAdminFromContext(ctx),
	}, tenantID, true
}

// mapArtifactError translates a service error into the apperror the
// errorCapture middleware renders. Returns false when the error was not a
// recognised sentinel (caller should render 500).
func mapArtifactError(c *gin.Context, tenantID uint64, err error) bool {
	switch {
	case errors.Is(err, service.ErrArtifactNotFound):
		c.Error(apperrors.NewNotFoundError("artifact not found"))
	case errors.Is(err, service.ErrArtifactForbidden):
		c.Error(apperrors.NewForbiddenError("artifact access forbidden"))
	case errors.Is(err, service.ErrInvalidArtifactType),
		errors.Is(err, service.ErrInvalidArtifactSharingPolicy):
		c.Error(apperrors.NewValidationError(err.Error()))
	default:
		return false
	}
	return true
}

// CreateArtifact godoc
// @Summary      创建生成物
// @Description  为当前会话/用户创建一个生成物记录（状态 pending），由生成 pipeline 后续更新为 ready/failed。
// @Tags         生成物
// @Accept       json
// @Produce      json
// @Param        request  body  createArtifactRequest  true  "创建请求"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts [post]
func (h *ArtifactHandler) CreateArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}

	var req createArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}

	params := interfaces.CreateArtifactParams{
		SessionID:         req.SessionID,
		Type:              types.ArtifactType(req.Type),
		Title:             req.Title,
		SourceKBID:        req.SourceKBID,
		SourceKnowledgeID: req.SourceKnowledgeID,
		SourceWikiPageID:  req.SourceWikiPageID,
		SharingPolicy:     types.ArtifactSharingPolicy(req.SharingPolicy),
		AllowedUserIDs:    req.AllowedUserIDs,
	}
	a, err := h.artifactService.Create(ctx, tenantID, caller, params)
	if err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "CreateArtifact failed: tenant=%d err=%v", tenantID, err)
			c.Error(apperrors.NewInternalServerError("failed to create artifact").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": a})
}

// GetArtifact godoc
// @Summary      获取生成物
// @Description  获取单个生成物；非创建者且无 ACL 权限返回 403，跨租户/不存在返回 404。
// @Tags         生成物
// @Produce      json
// @Param        id  path  string  true  "Artifact ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts/{id} [get]
func (h *ArtifactHandler) GetArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("artifact id is required"))
		return
	}

	a, err := h.artifactService.Get(ctx, tenantID, caller, id)
	if err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "GetArtifact failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to load artifact").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": a})
}

// ListArtifacts godoc
// @Summary      列出生成物
// @Description  列出当前租户中调用者可见的生成物（ACL 过滤），分页。
// @Tags         生成物
// @Produce      json
// @Param        page       query  int  false  "页码（从 1 起）"  default(1)
// @Param        page_size  query  int  false  "每页数量"          default(20)
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts [get]
func (h *ArtifactHandler) ListArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	page, pageSize, ok := parseListPagination(c)
	if !ok {
		return
	}

	rows, total, err := h.artifactService.List(ctx, tenantID, caller, page, pageSize)
	if err != nil {
		logger.Errorf(ctx, "ListArtifacts failed: tenant=%d err=%v", tenantID, err)
		c.Error(apperrors.NewInternalServerError("failed to list artifacts").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"artifacts": rows,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// ListSessionArtifacts godoc
// @Summary      列出会话生成物
// @Description  列出指定会话产生的、调用者可见的生成物（供聊天/Agent 面板展示）。
// @Tags         生成物
// @Produce      json
// @Param        session_id  path  string  true  "会话 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /sessions/{session_id}/artifacts [get]
func (h *ArtifactHandler) ListSessionArtifacts(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		c.Error(apperrors.NewValidationError("session id is required"))
		return
	}

	rows, err := h.artifactService.ListBySession(ctx, tenantID, caller, sessionID)
	if err != nil {
		logger.Errorf(ctx, "ListSessionArtifacts failed: tenant=%d session=%s err=%v", tenantID, sessionID, err)
		c.Error(apperrors.NewInternalServerError("failed to list session artifacts").WithDetails(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"artifacts": rows, "total": len(rows)},
	})
}

// UpdateArtifactStatus godoc
// @Summary      更新生成物状态
// @Description  生成 pipeline 完成/失败时更新状态与存储位置；仅创建者或 Admin+。
// @Tags         生成物
// @Accept       json
// @Produce      json
// @Param        id       path  string                     true  "Artifact ID"
// @Param        request  body  updateArtifactStatusRequest  true  "状态更新"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts/{id}/status [put]
func (h *ArtifactHandler) UpdateArtifactStatus(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("artifact id is required"))
		return
	}

	var req updateArtifactStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	// Sanity-check the status value before hitting the service so a typo
	// renders as 400 rather than persisting an unknown state.
	switch types.ArtifactStatus(req.Status) {
	case types.ArtifactStatusPending, types.ArtifactStatusReady, types.ArtifactStatusFailed:
	default:
		c.Error(apperrors.NewValidationError("status must be one of pending/ready/failed"))
		return
	}

	if err := h.artifactService.UpdateStatus(ctx, tenantID, caller, id, types.ArtifactStatus(req.Status), req.StorageURI, req.SizeBytes); err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "UpdateArtifactStatus failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to update artifact status").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteArtifact godoc
// @Summary      删除生成物
// @Description  软删除生成物；仅创建者或 Admin+，跨租户返回 404。
// @Tags         生成物
// @Produce      json
// @Param        id  path  string  true  "Artifact ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts/{id} [delete]
func (h *ArtifactHandler) DeleteArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("artifact id is required"))
		return
	}

	if err := h.artifactService.Delete(ctx, tenantID, caller, id); err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "DeleteArtifact failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to delete artifact").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// generateArtifactRequest is the JSON body for POST /artifacts/:id/generate.
type generateArtifactRequest struct {
	Prompt string `json:"prompt"`
}

// DownloadArtifact godoc
// @Summary      下载生成物
// @Description  以附件形式下载 ready 状态的生成物文件；非 ready 返回 409。
// @Tags         生成物
// @Produce      application/octet-stream
// @Param        id  path  string  true  "Artifact ID"
// @Success      200  {file}  binary
// @Security     Bearer
// @Router       /artifacts/{id}/download [get]
func (h *ArtifactHandler) DownloadArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("artifact id is required"))
		return
	}
	if h.fileService == nil {
		c.Error(apperrors.NewInternalServerError("file service not configured"))
		return
	}

	a, err := h.artifactService.Get(ctx, tenantID, caller, id)
	if err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "DownloadArtifact load failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to load artifact").WithDetails(err.Error()))
		}
		return
	}
	if a.Status != types.ArtifactStatusReady {
		c.Error(apperrors.NewConflictError("artifact is not ready for download").WithDetails(
			fmt.Sprintf("current status: %s", a.Status)))
		return
	}
	if a.StorageURI == "" {
		c.Error(apperrors.NewInternalServerError("artifact has no storage uri"))
		return
	}
	rc, err := h.fileService.GetFile(ctx, a.StorageURI)
	if err != nil {
		logger.Errorf(ctx, "DownloadArtifact read failed: tenant=%d id=%s err=%v", tenantID, id, err)
		c.Error(apperrors.NewInternalServerError("failed to read artifact file").WithDetails(err.Error()))
		return
	}
	defer rc.Close()

	filename := filepath.Base(a.StorageURI)
	mt := a.MimeType
	if mt == "" {
		// Some rows store mime_type inside Metadata; fall back to extension
		// sniffing using the system type database.
		if m := mime.TypeByExtension(filepath.Ext(filename)); m != "" {
			mt = m
		} else {
			mt = "application/octet-stream"
		}
	}
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	c.Header("Content-Disposition", cd)
	c.Header("Content-Type", mt)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")
	if a.SizeBytes > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", a.SizeBytes))
	}
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, rc); err != nil {
		logger.Errorf(ctx, "DownloadArtifact stream failed: tenant=%d id=%s err=%v", tenantID, id, err)
	}
}

// GenerateArtifact godoc
// @Summary      触发生成物生成
// @Description  以异步方式重新/首次生成指定生成物。立即返回 202 Accepted，
// @Description  客户端可通过 GET /artifacts/:id 轮询状态。
// @Tags         生成物
// @Accept       json
// @Produce      json
// @Param        id       path  string                   true  "Artifact ID"
// @Param        request  body  generateArtifactRequest  true  "生成参数"
// @Success      202  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /artifacts/{id}/generate [post]
func (h *ArtifactHandler) GenerateArtifact(c *gin.Context) {
	ctx := c.Request.Context()
	caller, tenantID, ok := artifactCallerFromContext(c)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewValidationError("artifact id is required"))
		return
	}
	if h.generationService == nil {
		c.Error(apperrors.NewInternalServerError("generation service not configured"))
		return
	}

	var req generateArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)

	// Validate ownership / visibility of the artifact before kicking off work.
	a, err := h.artifactService.Get(ctx, tenantID, caller, id)
	if err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "GenerateArtifact load failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to load artifact").WithDetails(err.Error()))
		}
		return
	}

	// Reset status to pending (so the UI shows "in progress") and clear any
	// previous file. Creator-or-admin only (enforced by UpdateStatus).
	if err := h.artifactService.UpdateStatus(ctx, tenantID, caller, id,
		types.ArtifactStatusPending, "", 0); err != nil {
		if !mapArtifactError(c, tenantID, err) {
			logger.Errorf(ctx, "GenerateArtifact reset status failed: tenant=%d id=%s err=%v", tenantID, id, err)
			c.Error(apperrors.NewInternalServerError("failed to reset artifact status").WithDetails(err.Error()))
		}
		return
	}

	// Kick off generation asynchronously. Use a background context detached
	// from the request so generation continues after the HTTP response is
	// sent (chromedp-driven renderers may take 30+s).
	go func() {
		bgCtx := context.Background()
		userID := a.UserID
		if userID == "" {
			userID = caller.UserID
		}
		if err := h.generationService.Generate(bgCtx, id, tenantID, userID, req.Prompt); err != nil {
			logger.Errorf(bgCtx, "GenerateArtifact background failed: tenant=%d id=%s err=%v", tenantID, id, err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"data":    gin.H{"id": id, "status": string(types.ArtifactStatusPending)},
	})
}
