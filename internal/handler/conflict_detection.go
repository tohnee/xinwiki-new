package handler

import (
	"net/http"
	"strconv"

	"github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type ConflictDetectionHandler struct {
	conflictService interfaces.ConflictDetectionService
}

func NewConflictDetectionHandler(conflictService interfaces.ConflictDetectionService) *ConflictDetectionHandler {
	return &ConflictDetectionHandler{conflictService: conflictService}
}

type conflictResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func (h *ConflictDetectionHandler) DetectConflicts(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	kbID := c.Param("kb_id")
	if kbID == "" {
		c.Error(errors.BadRequest("knowledge base id is required", nil))
		return
	}

	var req types.ConflictDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	req.TenantID = tenantID
	req.KBID = kbID

	result, err := h.conflictService.DetectConflicts(ctx, &req)
	if err != nil {
		c.Error(errors.Internal("failed to detect conflicts", err))
		return
	}

	c.JSON(http.StatusOK, conflictResponse{
		Success: true,
		Data:    result,
	})
}

func (h *ConflictDetectionHandler) GetConflict(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	conflictID := c.Param("conflict_id")
	if conflictID == "" {
		c.Error(errors.BadRequest("conflict id is required", nil))
		return
	}

	conflict, err := h.conflictService.GetConflict(ctx, tenantID, conflictID)
	if err != nil {
		c.Error(errors.Internal("failed to get conflict", err))
		return
	}

	c.JSON(http.StatusOK, conflictResponse{
		Success: true,
		Data:    conflict,
	})
}

func (h *ConflictDetectionHandler) ListConflicts(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	kbID := c.Query("kb_id")
	status := types.ConflictStatus(c.Query("status"))
	severity := types.ConflictSeverity(c.Query("severity"))
	conflictType := types.ConflictType(c.Query("type"))

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if v, err := strconv.Atoi(ps); err == nil && v > 0 && v <= 100 {
			pageSize = v
		}
	}

	conflicts, total, err := h.conflictService.ListConflicts(ctx, tenantID, kbID, status, severity, conflictType, page, pageSize)
	if err != nil {
		c.Error(errors.Internal("failed to list conflicts", err))
		return
	}

	c.JSON(http.StatusOK, conflictResponse{
		Success: true,
		Data: gin.H{
			"items":     conflicts,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func (h *ConflictDetectionHandler) ResolveConflict(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	conflictID := c.Param("conflict_id")
	if conflictID == "" {
		c.Error(errors.BadRequest("conflict id is required", nil))
		return
	}

	var req types.ConflictResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	req.ConflictID = conflictID

	_, err := h.conflictService.ResolveConflict(ctx, tenantID, &req)
	if err != nil {
		c.Error(errors.Internal("failed to resolve conflict", err))
		return
	}

	c.JSON(http.StatusOK, conflictResponse{
		Success: true,
		Data:    gin.H{"message": "conflict resolved"},
	})
}

func (h *ConflictDetectionHandler) GetConflictSummary(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	kbID := c.Query("kb_id")

	summary, err := h.conflictService.GetConflictSummary(ctx, tenantID, kbID)
	if err != nil {
		c.Error(errors.Internal("failed to get conflict summary", err))
		return
	}

	c.JSON(http.StatusOK, conflictResponse{
		Success: true,
		Data:    summary,
	})
}
