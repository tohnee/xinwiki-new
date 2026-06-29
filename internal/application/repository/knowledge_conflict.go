package repository

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

type conflictRepository struct {
	db *gorm.DB
}

func NewConflictRepository(db *gorm.DB) interfaces.ConflictRepository {
	return &conflictRepository{db: db}
}

func (r *conflictRepository) Create(ctx context.Context, conflict *types.Conflict) error {
	return r.db.WithContext(ctx).Create(conflict).Error
}

func (r *conflictRepository) BatchCreate(ctx context.Context, conflicts []*types.Conflict) error {
	if len(conflicts) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&conflicts).Error
}

func (r *conflictRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.Conflict, error) {
	var conflict types.Conflict
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&conflict).Error
	if err != nil {
		return nil, err
	}
	return &conflict, nil
}

func (r *conflictRepository) List(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	status types.ConflictStatus,
	severity types.ConflictSeverity,
	conflictType types.ConflictType,
	page, pageSize int,
) ([]*types.Conflict, int, error) {
	query := r.db.WithContext(ctx).Model(&types.Conflict{}).Where("tenant_id = ?", tenantID)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if conflictType != "" {
		query = query.Where("type = ?", conflictType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var conflicts []*types.Conflict
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&conflicts).Error
	if err != nil {
		return nil, 0, err
	}

	return conflicts, int(total), nil
}

func (r *conflictRepository) Update(ctx context.Context, conflict *types.Conflict) error {
	return r.db.WithContext(ctx).Save(conflict).Error
}

func (r *conflictRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&types.Conflict{}).Error
}

func (r *conflictRepository) FindExisting(ctx context.Context, tenantID uint64, kbID string, conflictType types.ConflictType, entityType, attribute string) (*types.Conflict, error) {
	var conflict types.Conflict
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND kb_id = ? AND type = ?", tenantID, kbID, conflictType)

	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}
	if attribute != "" {
		query = query.Where("attribute = ?", attribute)
	}

	err := query.First(&conflict).Error
	if err != nil {
		return nil, err
	}
	return &conflict, nil
}

func (r *conflictRepository) UpdateStatus(ctx context.Context, tenantID uint64, id string, status types.ConflictStatus, resolution string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}
	if resolution != "" {
		updates["resolution"] = resolution
	}
	return r.db.WithContext(ctx).
		Model(&types.Conflict{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Updates(updates).Error
}

func (r *conflictRepository) BatchUpdateStatus(ctx context.Context, tenantID uint64, ids []string, status types.ConflictStatus) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&types.Conflict{}).
		Where("tenant_id = ? AND id IN ?", tenantID, ids).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now().UTC(),
		}).Error
}

func (r *conflictRepository) DeleteByKBID(ctx context.Context, tenantID uint64, kbID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND kb_id = ?", tenantID, kbID).
		Delete(&types.Conflict{}).Error
}

func (r *conflictRepository) CountByStatus(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictStatus]int, error) {
	type result struct {
		Status types.ConflictStatus
		Count  int
	}
	var results []result

	query := r.db.WithContext(ctx).
		Model(&types.Conflict{}).
		Select("status, COUNT(*) as count").
		Where("tenant_id = ?", tenantID)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}

	err := query.Group("status").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[types.ConflictStatus]int)
	for _, r := range results {
		counts[r.Status] = r.Count
	}
	return counts, nil
}

func (r *conflictRepository) CountBySeverity(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictSeverity]int, error) {
	type result struct {
		Severity types.ConflictSeverity
		Count    int
	}
	var results []result

	query := r.db.WithContext(ctx).
		Model(&types.Conflict{}).
		Select("severity, COUNT(*) as count").
		Where("tenant_id = ?", tenantID)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}

	err := query.Group("severity").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[types.ConflictSeverity]int)
	for _, r := range results {
		counts[r.Severity] = r.Count
	}
	return counts, nil
}

func (r *conflictRepository) CountByType(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictType]int, error) {
	type result struct {
		Type  types.ConflictType
		Count int
	}
	var results []result

	query := r.db.WithContext(ctx).
		Model(&types.Conflict{}).
		Select("type, COUNT(*) as count").
		Where("tenant_id = ?", tenantID)

	if kbID != "" {
		query = query.Where("kb_id = ?", kbID)
	}

	err := query.Group("type").Scan(&results).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[types.ConflictType]int)
	for _, r := range results {
		counts[r.Type] = r.Count
	}
	return counts, nil
}
