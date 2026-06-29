package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type RAGEvaluationHandler struct {
	ragEvalService interfaces.RAGEvaluationService
}

func NewRAGEvaluationHandler(ragEvalService interfaces.RAGEvaluationService) *RAGEvaluationHandler {
	return &RAGEvaluationHandler{ragEvalService: ragEvalService}
}

type ragEvalResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func (h *RAGEvaluationHandler) EvaluateCitations(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var req types.CitationEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	req.TenantID = tenantID

	result, err := h.ragEvalService.EvaluateCitationAccuracy(ctx, &req)
	if err != nil {
		c.Error(errors.Internal("failed to evaluate citations", err))
		return
	}

	c.JSON(http.StatusOK, ragEvalResponse{
		Success: true,
		Data:    result,
	})
}

func (h *RAGEvaluationHandler) BatchEvaluateCitations(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var req types.BatchEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequest("invalid request", err))
		return
	}

	req.TenantID = tenantID
	for i := range req.Queries {
		req.Queries[i].TenantID = tenantID
	}

	results, err := h.ragEvalService.EvaluateBatch(ctx, &req)
	if err != nil {
		c.Error(errors.Internal("failed to batch evaluate citations", err))
		return
	}

	c.JSON(http.StatusOK, ragEvalResponse{
		Success: true,
		Data:    results,
	})
}

func (h *RAGEvaluationHandler) GetCitationReport(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	reportID := c.Param("report_id")
	if reportID == "" {
		c.Error(errors.BadRequest("report id is required", nil))
		return
	}

	report, err := h.ragEvalService.GetReport(ctx, tenantID, reportID)
	if err != nil {
		c.Error(errors.Internal("failed to get citation report", err))
		return
	}

	c.JSON(http.StatusOK, ragEvalResponse{
		Success: true,
		Data:    report,
	})
}

func (h *RAGEvaluationHandler) GetCitationMetrics(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	kbID := c.Query("kb_id")
	days := 30
	if d := c.Query("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	to := time.Now()
	from := to.AddDate(0, 0, -days)

	metrics, err := h.ragEvalService.GetEvaluationSummary(ctx, tenantID, kbID, from, to)
	if err != nil {
		c.Error(errors.Internal("failed to get citation metrics", err))
		return
	}

	c.JSON(http.StatusOK, ragEvalResponse{
		Success: true,
		Data:    metrics,
	})
}
