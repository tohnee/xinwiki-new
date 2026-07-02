package repository

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"gorm.io/gorm"
)

type userNoteRepository struct {
	db *gorm.DB
}

// NewUserNoteRepository constructs the GORM-backed implementation.
func NewUserNoteRepository(db *gorm.DB) interfaces.UserNoteRepository {
	return &userNoteRepository{db: db}
}

func (r *userNoteRepository) List(
	ctx context.Context, userID string, tenantID uint64,
) ([]*types.UserNote, error) {
	var list []*types.UserNote
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ?", userID, tenantID).
		Order("created_at DESC").
		Find(&list).Error
	return list, err
}

func (r *userNoteRepository) ListBySession(
	ctx context.Context, userID string, tenantID uint64, sessionID string,
) ([]*types.UserNote, error) {
	var list []*types.UserNote
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND session_id = ?", userID, tenantID, sessionID).
		Order("created_at DESC").
		Find(&list).Error
	return list, err
}

func (r *userNoteRepository) Get(
	ctx context.Context, userID string, tenantID uint64, id string,
) (*types.UserNote, error) {
	var note types.UserNote
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND tenant_id = ?", id, userID, tenantID).
		First(&note).Error
	if err != nil {
		return nil, err
	}
	return &note, nil
}

func (r *userNoteRepository) Create(
	ctx context.Context, note *types.UserNote,
) (*types.UserNote, error) {
	if err := r.db.WithContext(ctx).Create(note).Error; err != nil {
		return nil, err
	}
	return note, nil
}

func (r *userNoteRepository) Update(
	ctx context.Context, userID string, tenantID uint64, id string, title, content string,
) (*types.UserNote, error) {
	res := r.db.WithContext(ctx).
		Model(&types.UserNote{}).
		Where("id = ? AND user_id = ? AND tenant_id = ?", id, userID, tenantID).
		Updates(map[string]any{
			"title":      title,
			"content":    content,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.Get(ctx, userID, tenantID, id)
}

func (r *userNoteRepository) Delete(
	ctx context.Context, userID string, tenantID uint64, id string,
) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND tenant_id = ?", id, userID, tenantID).
		Delete(&types.UserNote{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
