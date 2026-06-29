package repository

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

type evaluationRepository struct {
	db *gorm.DB
}

func NewEvaluationRepository(db *gorm.DB) interfaces.EvaluationRepository {
	return &evaluationRepository{db: db}
}

func (r *evaluationRepository) CreateReport(ctx context.Context, report *types.CitationAccuracyReport) error {
	return r.db.WithContext(ctx).Create(report).Error
}

func (r *evaluationRepository) GetReport(ctx context.Context, tenantID uint64, id string) (*types.CitationAccuracyReport, error) {
	var report types.CitationAccuracyReport
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *evaluationRepository) ListReports(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	from, to time.Time,
	page, pageSize int,
) ([]*types.CitationAccuracyReport, int, error) {
	query := r.db.WithContext(ctx).Model(&types.CitationAccuracyReport{}).Where("tenant_id = ?", tenantID)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}
	if !from.IsZero() {
		query = query.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("created_at <= ?", to)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var reports []*types.CitationAccuracyReport
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&reports).Error
	if err != nil {
		return nil, 0, err
	}

	return reports, int(total), nil
}

func (r *evaluationRepository) UpdateReport(ctx context.Context, report *types.CitationAccuracyReport) error {
	return r.db.WithContext(ctx).Save(report).Error
}

func (r *evaluationRepository) DeleteReport(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.CitationAccuracyReport{}).Error
}

func (r *evaluationRepository) GetAggregateMetrics(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	from, to time.Time,
) (precision, recall, f1, groundedness, hallucination float64, count int, err error) {
	type metricsResult struct {
		AvgPrecision     float64
		AvgRecall        float64
		AvgF1            float64
		AvgGroundedness  float64
		AvgHallucination float64
		TotalCount       int
	}

	var result metricsResult
	query := r.db.WithContext(ctx).
		Model(&types.CitationAccuracyReport{}).
		Select(`
			COALESCE(AVG(precision_score), 0) as avg_precision,
			COALESCE(AVG(recall_score), 0) as avg_recall,
			COALESCE(AVG(f1_score), 0) as avg_f1,
			COALESCE(AVG(groundedness_score), 0) as avg_groundedness,
			COALESCE(AVG(hallucination_rate), 0) as avg_hallucination,
			COUNT(*) as total_count
		`).
		Where("tenant_id = ? AND status = ?", tenantID, types.EvaluationStatusCompleted)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}
	if !from.IsZero() {
		query = query.Where("created_at >= ?", from)
	}
	if !to.IsZero() {
		query = query.Where("created_at <= ?", to)
	}

	err = query.Scan(&result).Error
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	return result.AvgPrecision, result.AvgRecall, result.AvgF1, result.AvgGroundedness, result.AvgHallucination, result.TotalCount, nil
}
