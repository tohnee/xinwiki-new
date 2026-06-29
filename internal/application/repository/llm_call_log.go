package repository

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

type llmCallLogRepository struct {
	db *gorm.DB
}

// NewLLMCallLogRepository creates a new LLM call log repository
func NewLLMCallLogRepository(db *gorm.DB) interfaces.LLMCallLogRepository {
	return &llmCallLogRepository{db: db}
}

// Create creates a new call log entry
func (r *llmCallLogRepository) Create(ctx context.Context, log *types.LLMCallLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// BatchCreate creates multiple call log entries in batch
func (r *llmCallLogRepository) BatchCreate(ctx context.Context, logs []*types.LLMCallLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

// AggregateDailyCost aggregates cost by day for a tenant within a time range
func (r *llmCallLogRepository) AggregateDailyCost(
	ctx context.Context, tenantID uint64, start, end time.Time,
) ([]*types.CostAggregation, error) {
	var results []*types.CostAggregation

	dateExpr := "DATE(created_at)"
	if r.db.Dialector.Name() == "sqlite" {
		dateExpr = "date(created_at)"
	}

	err := r.db.WithContext(ctx).
		Model(&types.LLMCallLog{}).
		Select(`
			` + dateExpr + ` as date,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(cached_tokens), 0) as cached_tokens,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(estimated_cost), 0) as total_cost,
			COUNT(*) as call_count
		`).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND status = ?",
			tenantID, start, end, types.LLMCallStatusSuccess).
		Group("date").
		Order("date ASC").
		Scan(&results).Error

	return results, err
}

// AggregateByModel aggregates cost by model for a tenant within a time range
func (r *llmCallLogRepository) AggregateByModel(
	ctx context.Context, tenantID uint64, start, end time.Time,
) ([]*types.ModelCostBreakdown, error) {
	var results []*types.ModelCostBreakdown

	err := r.db.WithContext(ctx).
		Model(&types.LLMCallLog{}).
		Select(`
			model_id,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(cached_tokens), 0) as cached_tokens,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(estimated_cost), 0) as total_cost,
			COUNT(*) as call_count
		`).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND status = ?",
			tenantID, start, end, types.LLMCallStatusSuccess).
		Group("model_id").
		Order("total_cost DESC").
		Scan(&results).Error

	return results, err
}

// AggregateByUser aggregates cost by user for a tenant within a time range
func (r *llmCallLogRepository) AggregateByUser(
	ctx context.Context, tenantID uint64, start, end time.Time, limit int,
) ([]*types.UserCostBreakdown, error) {
	var results []*types.UserCostBreakdown

	if limit <= 0 {
		limit = 10
	}

	err := r.db.WithContext(ctx).
		Model(&types.LLMCallLog{}).
		Select(`
			user_id,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(estimated_cost), 0) as total_cost,
			COUNT(*) as call_count
		`).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND status = ? AND user_id != ''",
			tenantID, start, end, types.LLMCallStatusSuccess).
		Group("user_id").
		Order("total_cost DESC").
		Limit(limit).
		Scan(&results).Error

	return results, err
}

// GetSummary gets total cost and token summary for a time range
func (r *llmCallLogRepository) GetSummary(
	ctx context.Context, tenantID uint64, start, end time.Time,
) (float64, int, int, error) {
	var result struct {
		TotalCost   float64
		TotalTokens int
		CallCount   int
	}

	err := r.db.WithContext(ctx).
		Model(&types.LLMCallLog{}).
		Select(`
			COALESCE(SUM(estimated_cost), 0) as total_cost,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COUNT(*) as call_count
		`).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ? AND status = ?",
			tenantID, start, end, types.LLMCallStatusSuccess).
		Scan(&result).Error

	if err != nil {
		return 0, 0, 0, err
	}

	return result.TotalCost, result.TotalTokens, result.CallCount, nil
}
