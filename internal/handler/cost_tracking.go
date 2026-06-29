package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// CostTrackingHandler exposes LLM cost dashboard APIs
type CostTrackingHandler struct {
	costService interfaces.CostTrackingService
}

// NewCostTrackingHandler constructs the handler
func NewCostTrackingHandler(costService interfaces.CostTrackingService) *CostTrackingHandler {
	return &CostTrackingHandler{costService: costService}
}

// costDashboardResponse is the response envelope for GetCostDashboard
type costDashboardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// GetCostDashboard godoc
// @Summary      获取LLM成本看板数据
// @Description  返回指定时间范围内的LLM成本汇总、日趋势、模型分布和Top用户
// @Tags         成本管理
// @Produce      json
// @Param        id     path   string  true   "租户ID"
// @Param        days   query  int     false  "统计天数，默认30天"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/dashboard [get]
func (h *CostTrackingHandler) GetCostDashboard(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	days := 30
	if raw := c.Query("days"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	dashboard, err := h.costService.GetCostDashboard(ctx, tenantID, days)
	if err != nil {
		c.Error(errors.Internal("failed to get cost dashboard", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    dashboard,
	})
}

// GetModelCostBreakdown godoc
// @Summary      获取按模型分布的成本明细
// @Description  返回指定时间范围内按模型分组的token用量和成本统计
// @Tags         成本管理
// @Produce      json
// @Param        id        path   string  true   "租户ID"
// @Param        start     query  string  false  "开始日期 (YYYY-MM-DD)"
// @Param        end       query  string  false  "结束日期 (YYYY-MM-DD)"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/by-model [get]
func (h *CostTrackingHandler) GetModelCostBreakdown(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	end := time.Now()
	start := end.AddDate(0, 0, -30)

	if raw := c.Query("start"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			start = t
		}
	}
	if raw := c.Query("end"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			end = t.Add(24 * time.Hour)
		}
	}

	breakdown, err := h.costService.GetModelCostBreakdown(ctx, tenantID, start, end)
	if err != nil {
		c.Error(errors.Internal("failed to get model cost breakdown", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    breakdown,
	})
}

// GetDailyCostTrend godoc
// @Summary      获取每日成本趋势
// @Description  返回指定时间范围内按日聚合的token用量和成本趋势
// @Tags         成本管理
// @Produce      json
// @Param        id        path   string  true   "租户ID"
// @Param        start     query  string  false  "开始日期 (YYYY-MM-DD)"
// @Param        end       query  string  false  "结束日期 (YYYY-MM-DD)"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/daily-trend [get]
func (h *CostTrackingHandler) GetDailyCostTrend(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	end := time.Now()
	start := end.AddDate(0, 0, -30)

	if raw := c.Query("start"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			start = t
		}
	}
	if raw := c.Query("end"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			end = t.Add(24 * time.Hour)
		}
	}

	trend, err := h.costService.GetDailyCostTrend(ctx, tenantID, start, end)
	if err != nil {
		c.Error(errors.Internal("failed to get daily cost trend", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    trend,
	})
}

// QueryCostTrend godoc
// @Summary      多维度成本趋势查询
// @Description  支持按租户、模型、请求类型、用户、时间维度进行成本趋势查询，支持分组聚合
// @Tags         成本管理
// @Produce      json
// @Param        id           path   string  true   "租户ID"
// @Param        start        query  string  false  "开始时间 (YYYY-MM-DD或YYYY-MM-DD HH:MM:SS)"
// @Param        end          query  string  false  "结束时间 (YYYY-MM-DD或YYYY-MM-DD HH:MM:SS)"
// @Param        models       query  string  false  "模型ID列表，逗号分隔"
// @Param        request_types query string  false  "请求类型列表，逗号分隔 (chat_completion,embedding,rerank等)"
// @Param        user_ids     query  string  false  "用户ID列表，逗号分隔"
// @Param        granularity  query  string  false  "时间粒度: hour, day, week, month，默认day"
// @Param        group_by     query  string  false  "分组维度: model,request_type,user，逗号分隔"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/trend [post]
func (h *CostTrackingHandler) QueryCostTrend(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	var req struct {
		Start        string   `json:"start" form:"start"`
		End          string   `json:"end" form:"end"`
		Models       string   `json:"models" form:"models"`
		RequestTypes string   `json:"request_types" form:"request_types"`
		UserIDs      string   `json:"user_ids" form:"user_ids"`
		Granularity  string   `json:"granularity" form:"granularity"`
		GroupBy      string   `json:"group_by" form:"group_by"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Error(errors.BadRequest("invalid request parameters", err))
			return
		}
	}

	query := &types.CostQuery{
		TenantID:    tenantID,
		Granularity: req.Granularity,
	}

	// Parse start/end time
	if req.Start != "" {
		if t, err := time.Parse("2006-01-02", req.Start); err == nil {
			query.StartDate = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.Start); err == nil {
			query.StartDate = t
		}
	}
	if req.End != "" {
		if t, err := time.Parse("2006-01-02", req.End); err == nil {
			query.EndDate = t.Add(24 * time.Hour)
		} else if t, err := time.Parse("2006-01-02 15:04:05", req.End); err == nil {
			query.EndDate = t
		}
	}

	// Parse model IDs
	if req.Models != "" {
		query.ModelIDs = splitAndTrim(req.Models)
	}

	// Parse request types
	if req.RequestTypes != "" {
		reqTypes := splitAndTrim(req.RequestTypes)
		query.RequestTypes = make([]types.LLMRequestType, len(reqTypes))
		for i, rt := range reqTypes {
			query.RequestTypes[i] = types.LLMRequestType(rt)
		}
	}

	// Parse user IDs
	if req.UserIDs != "" {
		query.UserIDs = splitAndTrim(req.UserIDs)
	}

	// Parse group by
	if req.GroupBy != "" {
		query.GroupBy = splitAndTrim(req.GroupBy)
	}

	trend, err := h.costService.QueryCostTrend(ctx, query)
	if err != nil {
		c.Error(errors.Internal("failed to query cost trend", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    trend,
	})
}

// GetCostSummary godoc
// @Summary      获取综合成本汇总
// @Description  返回指定时间范围内的综合成本汇总，包括按模型、按请求类型、按日趋势和Top用户
// @Tags         成本管理
// @Produce      json
// @Param        id           path   string  true   "租户ID"
// @Param        start        query  string  false  "开始日期 (YYYY-MM-DD)"
// @Param        end          query  string  false  "结束日期 (YYYY-MM-DD)"
// @Param        models       query  string  false  "模型ID列表，逗号分隔"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/summary [get]
func (h *CostTrackingHandler) GetCostSummary(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	query := &types.CostQuery{
		TenantID: tenantID,
	}

	if raw := c.Query("start"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			query.StartDate = t
		}
	}
	if raw := c.Query("end"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			query.EndDate = t.Add(24 * time.Hour)
		}
	}
	if models := c.Query("models"); models != "" {
		query.ModelIDs = splitAndTrim(models)
	}

	summary, err := h.costService.GetCostSummary(ctx, query)
	if err != nil {
		c.Error(errors.Internal("failed to get cost summary", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    summary,
	})
}

// GetModelLatencyStats godoc
// @Summary      获取模型延迟统计
// @Description  返回指定模型的延迟统计数据，包括平均延迟、P50/P95/P99百分位、成功率、每1K tokens平均成本
// @Tags         成本管理
// @Produce      json
// @Param        id           path   string  true   "租户ID"
// @Param        start        query  string  false  "开始日期 (YYYY-MM-DD)，默认7天前"
// @Param        end          query  string  false  "结束日期 (YYYY-MM-DD)，默认今天"
// @Param        models       query  string  false  "模型ID列表，逗号分隔"
// @Success      200  {object}  costDashboardResponse
// @Failure      400  {object}  errors.AppError
// @Failure      403  {object}  errors.AppError
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id}/cost/latency-stats [get]
func (h *CostTrackingHandler) GetModelLatencyStats(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, ok := parseTenantIDFromPath(c)
	if !ok {
		return
	}

	end := time.Now()
	start := end.AddDate(0, 0, -7)

	if raw := c.Query("start"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			start = t
		}
	}
	if raw := c.Query("end"); raw != "" {
		if t, err := time.Parse("2006-01-02", raw); err == nil {
			end = t.Add(24 * time.Hour)
		}
	}

	var modelIDs []string
	if models := c.Query("models"); models != "" {
		modelIDs = splitAndTrim(models)
	}

	stats, err := h.costService.GetModelLatencyStats(ctx, tenantID, modelIDs, start, end)
	if err != nil {
		c.Error(errors.Internal("failed to get model latency stats", err))
		return
	}

	c.JSON(http.StatusOK, costDashboardResponse{
		Success: true,
		Data:    stats,
	})
}

// splitAndTrim splits a comma-separated string and trims whitespace
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
