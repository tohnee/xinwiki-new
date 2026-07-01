package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// ModelRouterService defines the model routing service interface
type ModelRouterService interface {
	// SelectModel selects the most appropriate model based on request parameters and routing policy
	SelectModel(ctx context.Context, req *types.ModelSelectRequest) (*types.ModelSelectResult, error)
	// GetRoutingPolicy retrieves the routing policy for a tenant
	GetRoutingPolicy(ctx context.Context, tenantID uint64) (*types.ModelRoutingPolicy, error)
	// UpdateRoutingPolicy updates the routing policy for a tenant
	UpdateRoutingPolicy(ctx context.Context, policy *types.ModelRoutingPolicy) error
	// RecordModelPerformance records model performance statistics for future routing decisions
	RecordModelPerformance(ctx context.Context, stats *types.ModelPerformanceStats) error
	// CheckBudgetExceeded checks if the tenant has exceeded their budget
	CheckBudgetExceeded(ctx context.Context, tenantID uint64) (bool, float64, float64, error)
}

// PromptTemplateService defines the prompt template version management interface
type PromptTemplateService interface {
	// CreateTemplate creates a new prompt template with version
	CreateTemplate(ctx context.Context, template *types.PromptTemplate) error
	// GetTemplate retrieves a specific version of a prompt template
	GetTemplate(ctx context.Context, tenantID uint64, templateKey, version string) (*types.PromptTemplate, error)
	// GetActiveTemplate retrieves the active version of a prompt template
	GetActiveTemplate(ctx context.Context, tenantID uint64, templateKey string) (*types.PromptTemplate, error)
	// ListTemplateVersions lists all versions of a prompt template
	ListTemplateVersions(ctx context.Context, tenantID uint64, templateKey string) ([]*types.PromptTemplate, error)
	// ActivateVersion activates a specific version of a prompt template
	ActivateVersion(ctx context.Context, tenantID uint64, templateKey, version string) error
	// RenderTemplate renders a prompt template with the given variables
	RenderTemplate(ctx context.Context, tenantID uint64, templateKey, version string, vars map[string]string) (string, string, error)
}

// ModelRouterRepository defines the data access interface for model routing
type ModelRouterRepository interface {
	// GetRoutingPolicy retrieves tenant routing policy
	GetRoutingPolicy(ctx context.Context, tenantID uint64) (*types.ModelRoutingPolicy, error)
	// SaveRoutingPolicy saves/updates tenant routing policy
	SaveRoutingPolicy(ctx context.Context, policy *types.ModelRoutingPolicy) error
	// GetModelPerformance retrieves performance stats for a model
	GetModelPerformance(ctx context.Context, modelID string) (*types.ModelPerformanceStats, error)
	// SaveModelPerformance saves model performance stats
	SaveModelPerformance(ctx context.Context, stats *types.ModelPerformanceStats) error
	// GetTenantSpend retrieves current spend for a tenant within a period
	GetTenantSpend(ctx context.Context, tenantID uint64, since time.Time) (float64, error)
}

// PromptTemplateRepository defines data access for prompt templates
type PromptTemplateRepository interface {
	// Create creates a new template version
	Create(ctx context.Context, template *types.PromptTemplate) error
	// Get retrieves a specific version
	Get(ctx context.Context, tenantID uint64, templateKey, version string) (*types.PromptTemplate, error)
	// GetActive retrieves the active version
	GetActive(ctx context.Context, tenantID uint64, templateKey string) (*types.PromptTemplate, error)
	// ListVersions lists all versions for a template key
	ListVersions(ctx context.Context, tenantID uint64, templateKey string) ([]*types.PromptTemplate, error)
	// SetActive sets a specific version as active
	SetActive(ctx context.Context, tenantID uint64, templateKey, version string) error
	// InitDefaults idempotently seeds system-level default templates. Safe to
	// call on every startup; existing rows are left untouched.
	InitDefaults(ctx context.Context) error
}
