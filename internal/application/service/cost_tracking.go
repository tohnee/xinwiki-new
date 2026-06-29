package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

// CostTrackingService handles LLM cost tracking and aggregation
type CostTrackingService struct {
	db              *gorm.DB
	modelRepo       interfaces.ModelRepository
	callLogRepo     interfaces.LLMCallLogRepository
}

// NewCostTrackingService creates a new cost tracking service
func NewCostTrackingService(
	db *gorm.DB,
	modelRepo interfaces.ModelRepository,
	callLogRepo interfaces.LLMCallLogRepository,
) interfaces.CostTrackingService {
	return &CostTrackingService{
		db:          db,
		modelRepo:   modelRepo,
		callLogRepo: callLogRepo,
	}
}

// LogCall records a single LLM call with automatic cost calculation
func (s *CostTrackingService) LogCall(
	ctx context.Context,
	log *types.LLMCallLog,
) error {
	if log == nil {
		return fmt.Errorf("log cannot be nil")
	}

	// Calculate cost if model pricing is available
	if log.ModelID != "" && log.EstimatedCost == 0 {
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, log.ModelID)
		if err == nil && model != nil {
			usage := &types.TokenUsage{
				PromptTokens:     log.PromptTokens,
				CompletionTokens: log.CompletionTokens,
				CachedTokens:     log.CachedTokens,
			}
			log.EstimatedCost = model.CalculateCost(usage)
		}
	}

	if log.TotalTokens == 0 {
		log.TotalTokens = log.PromptTokens + log.CompletionTokens
	}

	return s.callLogRepo.Create(ctx, log)
}

// LogCallWithUsage is a convenience method to log a call with TokenUsage
func (s *CostTrackingService) LogCallWithUsage(
	ctx context.Context,
	tenantID uint64,
	userID, sessionID, kbID, modelID string,
	modelType types.ModelType,
	requestType types.LLMRequestType,
	usage *types.TokenUsage,
	latencyMs int,
	err error,
	traceID string,
) error {
	log := &types.LLMCallLog{
		TenantID:        tenantID,
		UserID:          userID,
		SessionID:       sessionID,
		KBID:            kbID,
		ModelID:         modelID,
		ModelType:       modelType,
		RequestType:     requestType,
		PromptTokens:    usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		CachedTokens:    usage.CachedTokens,
		TotalTokens:     usage.TotalTokens,
		LatencyMs:       latencyMs,
		TraceID:         traceID,
		Status:          types.LLMCallStatusSuccess,
	}

	if err != nil {
		log.Status = types.LLMCallStatusError
		log.ErrorMessage = err.Error()
	}

	return s.LogCall(ctx, log)
}

// GetCostDashboard returns the complete cost dashboard data for a tenant
func (s *CostTrackingService) GetCostDashboard(
	ctx context.Context,
	tenantID uint64,
	days int,
) (*types.CostDashboardSummary, error) {
	if days <= 0 {
		days = 30
	}

	end := time.Now()
	start := end.AddDate(0, 0, -days)

	// Get summary
	totalCost, totalTokens, totalCalls, err := s.callLogRepo.GetSummary(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}

	// Get daily trend
	dailyTrend, err := s.callLogRepo.AggregateDailyCost(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}

	// Get model breakdown
	modelBreakdown, err := s.callLogRepo.AggregateByModel(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}

	// Enrich model breakdown with names and percentages
	for _, mb := range modelBreakdown {
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, mb.ModelID)
		if err == nil && model != nil {
			mb.ModelName = model.DisplayName
			if mb.ModelName == "" {
				mb.ModelName = model.Name
			}
		}
		if totalCost > 0 {
			mb.Percentage = mb.TotalCost / totalCost * 100
		}
	}

	// Get top users
	topUsers, err := s.callLogRepo.AggregateByUser(ctx, tenantID, start, end, 10)
	if err != nil {
		return nil, err
	}
	for _, ub := range topUsers {
		if totalCost > 0 {
			ub.Percentage = ub.TotalCost / totalCost * 100
		}
	}

	avgCostPerCall := 0.0
	if totalCalls > 0 {
		avgCostPerCall = totalCost / float64(totalCalls)
	}

	// Convert pointer slices to value slices
	dailyTrendValues := make([]types.CostAggregation, len(dailyTrend))
	for i, d := range dailyTrend {
		dailyTrendValues[i] = *d
	}
	modelBreakdownValues := make([]types.ModelCostBreakdown, len(modelBreakdown))
	for i, m := range modelBreakdown {
		modelBreakdownValues[i] = *m
	}
	topUsersValues := make([]types.UserCostBreakdown, len(topUsers))
	for i, u := range topUsers {
		topUsersValues[i] = *u
	}

	return &types.CostDashboardSummary{
		Period:         fmt.Sprintf("%dd", days),
		StartDate:      start,
		EndDate:        end,
		TotalCost:      totalCost,
		TotalTokens:    totalTokens,
		TotalCalls:     totalCalls,
		AvgCostPerCall: avgCostPerCall,
		DailyTrend:     dailyTrendValues,
		ModelBreakdown: modelBreakdownValues,
		TopUsers:       topUsersValues,
	}, nil
}

// GetModelCostBreakdown returns cost breakdown by model
func (s *CostTrackingService) GetModelCostBreakdown(
	ctx context.Context,
	tenantID uint64,
	start, end time.Time,
) ([]*types.ModelCostBreakdown, error) {
	breakdown, err := s.callLogRepo.AggregateByModel(ctx, tenantID, start, end)
	if err != nil {
		return nil, err
	}

	var totalCost float64
	for _, b := range breakdown {
		totalCost += b.TotalCost
	}

	for _, mb := range breakdown {
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, mb.ModelID)
		if err == nil && model != nil {
			mb.ModelName = model.DisplayName
			if mb.ModelName == "" {
				mb.ModelName = model.Name
			}
		}
		if totalCost > 0 {
			mb.Percentage = mb.TotalCost / totalCost * 100
		}
	}

	return breakdown, nil
}

// GetDailyCostTrend returns daily cost trend data
func (s *CostTrackingService) GetDailyCostTrend(
	ctx context.Context,
	tenantID uint64,
	start, end time.Time,
) ([]*types.CostAggregation, error) {
	return s.callLogRepo.AggregateDailyCost(ctx, tenantID, start, end)
}

// QueryCostTrend performs a multi-dimensional cost query with filtering and grouping
func (s *CostTrackingService) QueryCostTrend(
	ctx context.Context,
	query *types.CostQuery,
) ([]*types.CostTrendPoint, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if query.TenantID == 0 {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if query.StartDate.IsZero() {
		query.StartDate = time.Now().AddDate(0, 0, -30)
	}
	if query.EndDate.IsZero() {
		query.EndDate = time.Now()
	}
	if query.Granularity == "" {
		query.Granularity = "day"
	}

	logger.Infof(ctx, "[CostTracking] QueryCostTrend tenant=%d start=%v end=%v models=%v groupBy=%v",
		query.TenantID, query.StartDate, query.EndDate, query.ModelIDs, query.GroupBy)

	// Build base query
	db := s.db.WithContext(ctx).Model(&types.LLMCallLog{}).
		Where("tenant_id = ?", query.TenantID).
		Where("created_at BETWEEN ? AND ?", query.StartDate, query.EndDate).
		Where("status = ?", types.LLMCallStatusSuccess)

	// Apply filters
	if len(query.ModelIDs) > 0 {
		db = db.Where("model_id IN ?", query.ModelIDs)
	}
	if len(query.RequestTypes) > 0 {
		db = db.Where("request_type IN ?", query.RequestTypes)
	}
	if len(query.UserIDs) > 0 {
		db = db.Where("user_id IN ?", query.UserIDs)
	}

	// Determine time grouping
	var timeFormat string
	switch query.Granularity {
	case "hour":
		timeFormat = "%Y-%m-%d %H:00:00"
	case "week":
		timeFormat = "%Y-%u"
	case "month":
		timeFormat = "%Y-%m"
	default:
		timeFormat = "%Y-%m-%d"
	}

	// Build select and group by clauses
	selectFields := fmt.Sprintf(`
		DATE_FORMAT(created_at, '%s') as timestamp,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(cached_tokens) as cached_tokens,
		SUM(total_tokens) as total_tokens,
		SUM(estimated_cost) as total_cost,
		COUNT(*) as call_count,
		AVG(latency_ms) as avg_latency_ms,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
		SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as error_count
	`, timeFormat)

	groupFields := []string{"DATE_FORMAT(created_at, '" + timeFormat + "')"}

	// Add group by fields
	hasGroupBy := false
	modelGrouped := false
	for _, g := range query.GroupBy {
		switch g {
		case "model":
			selectFields += ", model_id"
			groupFields = append(groupFields, "model_id")
			modelGrouped = true
			hasGroupBy = true
		case "request_type":
			selectFields += ", request_type"
			groupFields = append(groupFields, "request_type")
			hasGroupBy = true
		case "user":
			selectFields += ", user_id"
			groupFields = append(groupFields, "user_id")
			hasGroupBy = true
		}
	}

	var results []struct {
		Timestamp        string  `gorm:"column:timestamp"`
		ModelID          string  `gorm:"column:model_id"`
		RequestType      string  `gorm:"column:request_type"`
		UserID           string  `gorm:"column:user_id"`
		PromptTokens     int     `gorm:"column:prompt_tokens"`
		CompletionTokens int     `gorm:"column:completion_tokens"`
		CachedTokens     int     `gorm:"column:cached_tokens"`
		TotalTokens      int     `gorm:"column:total_tokens"`
		TotalCost        float64 `gorm:"column:total_cost"`
		CallCount        int     `gorm:"column:call_count"`
		AvgLatencyMs     int     `gorm:"column:avg_latency_ms"`
		SuccessCount     int     `gorm:"column:success_count"`
		ErrorCount       int     `gorm:"column:error_count"`
	}

	err := db.Select(selectFields).
		Group(stringsJoin(groupFields, ", ")).
		Order("timestamp ASC").
		Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to query cost trend: %w", err)
	}

	// Convert to CostTrendPoint and enrich with model names
	points := make([]*types.CostTrendPoint, 0, len(results))
	for _, r := range results {
		ts, err := time.ParseInLocation("2006-01-02 15:04:05", r.Timestamp, time.Local)
		if err != nil {
			ts, err = time.ParseInLocation("2006-01-02", r.Timestamp[:10], time.Local)
			if err != nil {
				ts = query.StartDate
			}
		}

		point := &types.CostTrendPoint{
			Timestamp:        ts,
			ModelID:          r.ModelID,
			RequestType:      r.RequestType,
			UserID:           r.UserID,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			CachedTokens:     r.CachedTokens,
			TotalTokens:      r.TotalTokens,
			TotalCost:        r.TotalCost,
			CallCount:        r.CallCount,
			AvgLatencyMs:     r.AvgLatencyMs,
			SuccessCount:     r.SuccessCount,
			ErrorCount:       r.ErrorCount,
		}

		if modelGrouped && r.ModelID != "" {
			model, err := s.modelRepo.GetByIDAnyTenant(ctx, r.ModelID)
			if err == nil && model != nil {
				point.ModelName = model.DisplayName
				if point.ModelName == "" {
					point.ModelName = model.Name
				}
			}
		}

		points = append(points, point)
	}

	return points, nil
}

// GetCostSummary returns a comprehensive cost summary with all breakdown dimensions
func (s *CostTrackingService) GetCostSummary(
	ctx context.Context,
	query *types.CostQuery,
) (*types.CostSummary, error) {
	if query == nil {
		return nil, fmt.Errorf("query cannot be nil")
	}
	if query.StartDate.IsZero() {
		query.StartDate = time.Now().AddDate(0, 0, -30)
	}
	if query.EndDate.IsZero() {
		query.EndDate = time.Now()
	}

	logger.Infof(ctx, "[CostTracking] GetCostSummary tenant=%d start=%v end=%v",
		query.TenantID, query.StartDate, query.EndDate)

	// Get summary totals
	totalCost, totalTokens, totalCalls, err := s.callLogRepo.GetSummary(ctx, query.TenantID, query.StartDate, query.EndDate)
	if err != nil {
		return nil, err
	}

	// Get success rate and average latency
	var stats struct {
		AvgLatency float64 `gorm:"column:avg_latency"`
		SuccessCnt int64   `gorm:"column:success_cnt"`
	}
	err = s.db.WithContext(ctx).Model(&types.LLMCallLog{}).
		Where("tenant_id = ?", query.TenantID).
		Where("created_at BETWEEN ? AND ?", query.StartDate, query.EndDate).
		Select(`
			AVG(latency_ms) as avg_latency,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_cnt
		`).Scan(&stats).Error
	if err != nil {
		logger.Warnf(ctx, "[CostTracking] Failed to get latency stats: %v", err)
	}

	successRate := 0.0
	avgLatency := 0
	if totalCalls > 0 {
		successRate = float64(stats.SuccessCnt) / float64(totalCalls) * 100
		avgLatency = int(stats.AvgLatency)
	}

	avgCostPerCall := 0.0
	if totalCalls > 0 {
		avgCostPerCall = totalCost / float64(totalCalls)
	}

	// Get daily trend
	dailyTrend, err := s.callLogRepo.AggregateDailyCost(ctx, query.TenantID, query.StartDate, query.EndDate)
	if err != nil {
		logger.Warnf(ctx, "[CostTracking] Failed to get daily trend: %v", err)
	}

	// Get model breakdown
	modelBreakdown, err := s.callLogRepo.AggregateByModel(ctx, query.TenantID, query.StartDate, query.EndDate)
	if err != nil {
		logger.Warnf(ctx, "[CostTracking] Failed to get model breakdown: %v", err)
	}
	for _, mb := range modelBreakdown {
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, mb.ModelID)
		if err == nil && model != nil {
			mb.ModelName = model.DisplayName
			if mb.ModelName == "" {
				mb.ModelName = model.Name
			}
		}
		if totalCost > 0 {
			mb.Percentage = mb.TotalCost / totalCost * 100
		}
	}

	// Get top users
	topUsers, err := s.callLogRepo.AggregateByUser(ctx, query.TenantID, query.StartDate, query.EndDate, 10)
	if err != nil {
		logger.Warnf(ctx, "[CostTracking] Failed to get top users: %v", err)
	}
	for _, ub := range topUsers {
		if totalCost > 0 {
			ub.Percentage = ub.TotalCost / totalCost * 100
		}
	}

	// Convert pointer slices to value slices
	dailyTrendValues := make([]types.CostAggregation, len(dailyTrend))
	for i, d := range dailyTrend {
		dailyTrendValues[i] = *d
	}
	modelBreakdownValues := make([]types.ModelCostBreakdown, len(modelBreakdown))
	for i, m := range modelBreakdown {
		modelBreakdownValues[i] = *m
	}
	topUsersValues := make([]types.UserCostBreakdown, len(topUsers))
	for i, u := range topUsers {
		topUsersValues[i] = *u
	}

	// Get request type breakdown
	reqTypeQuery := &types.CostQuery{
		TenantID:  query.TenantID,
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
		ModelIDs:  query.ModelIDs,
		GroupBy:   []string{"request_type"},
	}
	reqTypePoints, err := s.QueryCostTrend(ctx, reqTypeQuery)
	if err != nil {
		logger.Warnf(ctx, "[CostTracking] Failed to get request type breakdown: %v", err)
	}

	return &types.CostSummary{
		StartDate:        query.StartDate,
		EndDate:          query.EndDate,
		TotalCost:        totalCost,
		TotalTokens:      totalTokens,
		TotalCalls:       totalCalls,
		SuccessRate:      successRate,
		AvgLatencyMs:     avgLatency,
		AvgCostPerCall:   avgCostPerCall,
		ByModel:          modelBreakdownValues,
		ByRequestType:    reqTypePoints,
		ByDay:            dailyTrendValues,
		TopUsers:         topUsersValues,
	}, nil
}

// GetModelLatencyStats returns latency statistics for models
func (s *CostTrackingService) GetModelLatencyStats(
	ctx context.Context,
	tenantID uint64,
	modelIDs []string,
	start, end time.Time,
) ([]*types.ModelLatencyStats, error) {
	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -7)
	}
	if end.IsZero() {
		end = time.Now()
	}

	db := s.db.WithContext(ctx).Model(&types.LLMCallLog{}).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Where("status = ?", types.LLMCallStatusSuccess)

	if len(modelIDs) > 0 {
		db = db.Where("model_id IN ?", modelIDs)
	}

	type dbResult struct {
		ModelID      string  `gorm:"column:model_id"`
		AvgLatency   float64 `gorm:"column:avg_latency"`
		P50Latency   float64 `gorm:"column:p50_latency"`
		P95Latency   float64 `gorm:"column:p95_latency"`
		P99Latency   float64 `gorm:"column:p99_latency"`
		CallCount    int64   `gorm:"column:call_count"`
		SuccessCnt   int64   `gorm:"column:success_cnt"`
		TotalCost    float64 `gorm:"column:total_cost"`
		TotalTokens  int     `gorm:"column:total_tokens"`
	}

	var results []dbResult
	err := db.Select(`
		model_id,
		AVG(latency_ms) as avg_latency,
		PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY latency_ms) as p50_latency,
		PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) as p95_latency,
		PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY latency_ms) as p99_latency,
		COUNT(*) as call_count,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_cnt,
		SUM(estimated_cost) as total_cost,
		SUM(total_tokens) as total_tokens
	`).Group("model_id").Find(&results).Error
	if err != nil {
		// Fallback if PERCENTILE_CONT not supported
		logger.Warnf(ctx, "[CostTracking] PERCENTILE_CONT not supported, using approximate percentiles: %v", err)
		return s.getModelLatencyStatsFallback(ctx, tenantID, modelIDs, start, end)
	}

	stats := make([]*types.ModelLatencyStats, 0, len(results))
	for _, r := range results {
		successRate := 0.0
		avgCostPer1K := 0.0
		if r.CallCount > 0 {
			successRate = float64(r.SuccessCnt) / float64(r.CallCount) * 100
		}
		if r.TotalTokens > 0 {
			avgCostPer1K = r.TotalCost / float64(r.TotalTokens) * 1000
		}

		modelName := r.ModelID
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, r.ModelID)
		if err == nil && model != nil {
			modelName = model.DisplayName
			if modelName == "" {
				modelName = model.Name
			}
		}

		stats = append(stats, &types.ModelLatencyStats{
			ModelID:      r.ModelID,
			ModelName:    modelName,
			AvgLatencyMs: int(r.AvgLatency),
			P50LatencyMs: int(r.P50Latency),
			P95LatencyMs: int(r.P95Latency),
			P99LatencyMs: int(r.P99Latency),
			CallCount:    int(r.CallCount),
			SuccessRate:  successRate,
			AvgCostPer1K: avgCostPer1K,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalCost > stats[j].TotalCost
	})

	return stats, nil
}

// getModelLatencyStatsFallback provides fallback latency stats without percentile functions
func (s *CostTrackingService) getModelLatencyStatsFallback(
	ctx context.Context,
	tenantID uint64,
	modelIDs []string,
	start, end time.Time,
) ([]*types.ModelLatencyStats, error) {
	db := s.db.WithContext(ctx).Model(&types.LLMCallLog{}).
		Where("tenant_id = ?", tenantID).
		Where("created_at BETWEEN ? AND ?", start, end).
		Where("status = ?", types.LLMCallStatusSuccess)

	if len(modelIDs) > 0 {
		db = db.Where("model_id IN ?", modelIDs)
	}

	type dbResult struct {
		ModelID     string  `gorm:"column:model_id"`
		AvgLatency  float64 `gorm:"column:avg_latency"`
		MaxLatency  int     `gorm:"column:max_latency"`
		CallCount   int64   `gorm:"column:call_count"`
		SuccessCnt  int64   `gorm:"column:success_cnt"`
		TotalCost   float64 `gorm:"column:total_cost"`
		TotalTokens int     `gorm:"column:total_tokens"`
	}

	var results []dbResult
	err := db.Select(`
		model_id,
		AVG(latency_ms) as avg_latency,
		MAX(latency_ms) as max_latency,
		COUNT(*) as call_count,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_cnt,
		SUM(estimated_cost) as total_cost,
		SUM(total_tokens) as total_tokens
	`).Group("model_id").Find(&results).Error
	if err != nil {
		return nil, err
	}

	stats := make([]*types.ModelLatencyStats, 0, len(results))
	for _, r := range results {
		successRate := 0.0
		avgCostPer1K := 0.0
		if r.CallCount > 0 {
			successRate = float64(r.SuccessCnt) / float64(r.CallCount) * 100
		}
		if r.TotalTokens > 0 {
			avgCostPer1K = r.TotalCost / float64(r.TotalTokens) * 1000
		}

		modelName := r.ModelID
		model, err := s.modelRepo.GetByIDAnyTenant(ctx, r.ModelID)
		if err == nil && model != nil {
			modelName = model.DisplayName
			if modelName == "" {
				modelName = model.Name
			}
		}

		// Approximate percentiles from average and max
		p50 := int(r.AvgLatency)
		p95 := int(float64(r.AvgLatency)*1.5 + float64(r.MaxLatency)*0.2)
		p99 := r.MaxLatency

		stats = append(stats, &types.ModelLatencyStats{
			ModelID:      r.ModelID,
			ModelName:    modelName,
			AvgLatencyMs: int(r.AvgLatency),
			P50LatencyMs: p50,
			P95LatencyMs: p95,
			P99LatencyMs: p99,
			CallCount:    int(r.CallCount),
			SuccessRate:  successRate,
			AvgCostPer1K: avgCostPer1K,
		})
	}

	return stats, nil
}

// stringsJoin joins strings with separator
func stringsJoin(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	result := elems[0]
	for i := 1; i < len(elems); i++ {
		result += sep + elems[i]
	}
	return result
}
