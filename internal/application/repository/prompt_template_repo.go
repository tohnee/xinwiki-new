package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PromptTemplateRepository provides database-backed persistence for prompt templates.
// This replaces the previous in-memory implementation and ensures templates survive
// process restarts and are consistent across multiple replicas.
type PromptTemplateRepository struct {
	db *gorm.DB
}

// NewPromptTemplateRepository creates a new DB-backed prompt template repository.
func NewPromptTemplateRepository(db *gorm.DB) *PromptTemplateRepository {
	return &PromptTemplateRepository{db: db}
}

// Create stores a new prompt template version in the database.
func (r *PromptTemplateRepository) Create(ctx context.Context, tpl *types.PromptTemplate) error {
	if tpl == nil {
		return fmt.Errorf("template cannot be nil")
	}
	if tpl.TemplateKey == "" {
		return fmt.Errorf("template_key is required")
	}
	if tpl.Version == "" {
		return fmt.Errorf("version is required")
	}
	if tpl.Content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	if tpl.ID == "" {
		tpl.ID = uuid.New().String()
	}
	now := time.Now()
	if tpl.CreatedAt.IsZero() {
		tpl.CreatedAt = now
	}
	tpl.UpdatedAt = now

	if err := r.db.WithContext(ctx).Create(tpl).Error; err != nil {
		return fmt.Errorf("failed to create template: %w", err)
	}

	// If this template is active, deactivate other versions
	if tpl.IsActive {
		r.db.WithContext(ctx).Model(&types.PromptTemplate{}).
			Where("template_key = ? AND tenant_id = ? AND version != ?",
				tpl.TemplateKey, tpl.TenantID, tpl.Version).
			Update("is_active", false)
	}

	logger.Infof(ctx, "[PromptTemplate] Created template key=%s version=%s tenant=%d",
		tpl.TemplateKey, tpl.Version, tpl.TenantID)
	return nil
}

// Get retrieves a specific template version. Falls back to system-level (tenant_id=0)
// if the tenant-specific version is not found.
func (r *PromptTemplateRepository) Get(ctx context.Context, tenantID uint64, templateKey, version string) (*types.PromptTemplate, error) {
	var tpl types.PromptTemplate

	// Try tenant-specific first
	err := r.db.WithContext(ctx).
		Where("template_key = ? AND tenant_id = ? AND version = ?", templateKey, tenantID, version).
		First(&tpl).Error
	if err == nil {
		return &tpl, nil
	}

	// Fall back to system-level
	if tenantID != 0 {
		err = r.db.WithContext(ctx).
			Where("template_key = ? AND tenant_id = 0 AND version = ?", templateKey, version).
			First(&tpl).Error
		if err == nil {
			return &tpl, nil
		}
	}

	return nil, fmt.Errorf("template %s version %s not found", templateKey, version)
}

// GetActive retrieves the active template for a given key. Falls back to system-level
// if no tenant-specific active template exists.
func (r *PromptTemplateRepository) GetActive(ctx context.Context, tenantID uint64, templateKey string) (*types.PromptTemplate, error) {
	var tpl types.PromptTemplate

	// Try tenant-specific active template first
	searchTenants := []uint64{tenantID}
	if tenantID != 0 {
		searchTenants = append(searchTenants, 0)
	}

	for _, tid := range searchTenants {
		err := r.db.WithContext(ctx).
			Where("template_key = ? AND tenant_id = ? AND is_active = ?", templateKey, tid, true).
			Order("updated_at DESC").
			First(&tpl).Error
		if err == nil {
			return &tpl, nil
		}
	}

	// Fallback: get the latest version regardless of active flag
	for _, tid := range searchTenants {
		err := r.db.WithContext(ctx).
			Where("template_key = ? AND tenant_id = ?", templateKey, tid).
			Order("created_at DESC").
			First(&tpl).Error
		if err == nil {
			return &tpl, nil
		}
	}

	return nil, fmt.Errorf("no active template found for %s", templateKey)
}

// ListVersions returns all versions of a template, including system-level fallbacks.
func (r *PromptTemplateRepository) ListVersions(ctx context.Context, tenantID uint64, templateKey string) ([]*types.PromptTemplate, error) {
	var templates []*types.PromptTemplate

	tenantIDs := []uint64{tenantID}
	if tenantID != 0 {
		tenantIDs = append(tenantIDs, 0)
	}

	err := r.db.WithContext(ctx).
		Where("template_key = ? AND tenant_id IN (?)", templateKey, tenantIDs).
		Order("created_at DESC").
		Find(&templates).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list template versions: %w", err)
	}

	return templates, nil
}

// Activate sets a specific version as active and deactivates all other versions
// for the same template key and tenant.
func (r *PromptTemplateRepository) Activate(ctx context.Context, tenantID uint64, templateKey, version string) error {
	return r.activate(ctx, tenantID, templateKey, version)
}

// SetActive implements interfaces.PromptTemplateRepository.SetActive. It is
// the DI-facing method name declared in
// internal/types/interfaces/model_router.go and merely forwards to the
// existing Activate path so the same code path satisfies both the internal
// call site (Create.InitDefaults uses Activate directly) and the DI-bound
// service that calls SetActive through the interface.
func (r *PromptTemplateRepository) SetActive(ctx context.Context, tenantID uint64, templateKey, version string) error {
	return r.activate(ctx, tenantID, templateKey, version)
}

// activate is the single shared implementation behind the two method-name
// aliases (Activate / SetActive) so future renames of either surface stay
// in lockstep.
func (r *PromptTemplateRepository) activate(ctx context.Context, tenantID uint64, templateKey, version string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Deactivate all versions for this key+tenant
		if err := tx.Model(&types.PromptTemplate{}).
			Where("template_key = ? AND tenant_id = ?", templateKey, tenantID).
			Update("is_active", false).Error; err != nil {
			return fmt.Errorf("failed to deactivate templates: %w", err)
		}

		// Activate the target version
		result := tx.Model(&types.PromptTemplate{}).
			Where("template_key = ? AND tenant_id = ? AND version = ?", templateKey, tenantID, version).
			Updates(map[string]interface{}{"is_active": true, "updated_at": time.Now()})
		if result.Error != nil {
			return fmt.Errorf("failed to activate template: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("template %s version %s not found", templateKey, version)
		}

		logger.Infof(ctx, "[PromptTemplate] Activated %s version=%s tenant=%d", templateKey, version, tenantID)
		return nil
	})
}

// InitDefaults seeds the database with default system-level prompt templates
// if they don't already exist. This is idempotent and safe to call on every startup.
func (r *PromptTemplateRepository) InitDefaults(ctx context.Context) error {
	defaults := map[string]string{
		"system.chat":           "你是一个智能助手，请根据用户的问题提供准确、有用的回答。",
		"system.knowledge_qa":   "你是一个专业的知识库问答助手。请基于以下提供的参考资料回答用户问题。\n如果参考资料中没有相关信息，请明确说明你无法从提供的资料中找到答案，不要编造信息。\n回答时请标注引用来源，使用[1]、[2]这样的标记。\n\n参考资料：\n{{.context}}\n\n用户问题：{{.question}}",
		"system.summary":        "请对以下内容进行简洁准确的总结，保留关键信息。\n\n内容：{{.content}}",
		"system.extraction":     "请从以下文本中提取关键信息，以JSON格式输出。\n\n文本：{{.content}}",
		"system.classification": "请将以下内容分类到预定义类别中。类别：{{.categories}}\n\n内容：{{.content}}",
		"system.rewrite":        "请改写以下内容，使其更加{{.style}}。\n\n原文：{{.content}}",
	}

	for key, content := range defaults {
		// Check if already exists
		var count int64
		r.db.WithContext(ctx).Model(&types.PromptTemplate{}).
			Where("template_key = ? AND tenant_id = 0 AND version = ?", key, "v1.0").
			Count(&count)
		if count > 0 {
			continue // Already exists, skip
		}

		tpl := &types.PromptTemplate{
			ID:          uuid.New().String(),
			TemplateKey: key,
			TenantID:    0,
			Version:     "v1.0",
			Content:     content,
			Description: fmt.Sprintf("Default template for %s", key),
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(tpl).Error; err != nil {
			logger.Warnf(ctx, "[PromptTemplate] Failed to init default %s: %v", key, err)
		}
	}

	return nil
}
