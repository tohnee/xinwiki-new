package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/models/rerank"
	"github.com/Tencent/XinWiki/internal/models/asr"
	"github.com/Tencent/XinWiki/internal/models/vlm"
	"github.com/Tencent/XinWiki/internal/types"
)

// ModelService defines the model service interface
type ModelService interface {
	// CreateModel creates a model
	CreateModel(ctx context.Context, model *types.Model) error
	// GetModelByID gets a model by ID
	GetModelByID(ctx context.Context, id string) (*types.Model, error)
	// ListModels lists all models
	ListModels(ctx context.Context) ([]*types.Model, error)
	// UpdateModel updates a model
	UpdateModel(ctx context.Context, model *types.Model) error
	// DeleteModel deletes a model
	DeleteModel(ctx context.Context, id string) error

	// UpdateModelCredentials writes one or more credential fields on the
	// model's Parameters. Nil pointer means "do not touch this field";
	// empty string is treated as no-op (use ClearModelCredential to remove).
	// Returns the updated model.
	UpdateModelCredentials(ctx context.Context, id string, apiKey, appSecret *string) (*types.Model, error)
	// ClearModelCredential removes a single credential field. field must be
	// "api_key" or "app_secret". Clearing an already-empty field is a no-op.
	ClearModelCredential(ctx context.Context, id, field string) error
	// GetEmbeddingModel gets an embedding model
	GetEmbeddingModel(ctx context.Context, modelId string) (embedding.Embedder, error)
	// GetEmbeddingModelForTenant gets an embedding model for a specific tenant (for cross-tenant sharing)
	GetEmbeddingModelForTenant(ctx context.Context, modelId string, tenantID uint64) (embedding.Embedder, error)
	// GetRerankModel gets a rerank model
	GetRerankModel(ctx context.Context, modelId string) (rerank.Reranker, error)
	// GetChatModel gets a chat model
	GetChatModel(ctx context.Context, modelId string) (chat.Chat, error)
	// GetVLMModel gets a vision language model
	GetVLMModel(ctx context.Context, modelId string) (vlm.VLM, error)
	// GetASRModel gets an automatic speech recognition model
	GetASRModel(ctx context.Context, modelId string) (asr.ASR, error)
}

// ModelRepository defines the model repository interface
type ModelRepository interface {
	// Create creates a model
	Create(ctx context.Context, model *types.Model) error
	// GetByID gets a model by ID
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.Model, error)
	// List lists all models
	List(
		ctx context.Context,
		tenantID uint64,
		modelType types.ModelType,
		source types.ModelSource,
	) ([]*types.Model, error)
	// Update updates a model
	Update(ctx context.Context, model *types.Model) error
	// Delete deletes a model
	Delete(ctx context.Context, tenantID uint64, id string) error
	// ClearDefaultByType clears the default flag for all models of a specific type
	// optionally excluding a specific model ID.
	ClearDefaultByType(ctx context.Context, tenantID uint, modelType types.ModelType, excludeID string) error
	// GetByIDAnyTenant gets a model by ID regardless of tenant (for cost calculation)
	GetByIDAnyTenant(ctx context.Context, id string) (*types.Model, error)
}

// LLMCallLogRepository defines the LLM call log repository interface for cost tracking
type LLMCallLogRepository interface {
	// Create creates a new call log entry
	Create(ctx context.Context, log *types.LLMCallLog) error
	// BatchCreate creates multiple call log entries in batch
	BatchCreate(ctx context.Context, logs []*types.LLMCallLog) error
	// AggregateDailyCost aggregates cost by day for a tenant within a time range
	AggregateDailyCost(ctx context.Context, tenantID uint64, start, end time.Time) ([]*types.CostAggregation, error)
	// AggregateByModel aggregates cost by model for a tenant within a time range
	AggregateByModel(ctx context.Context, tenantID uint64, start, end time.Time) ([]*types.ModelCostBreakdown, error)
	// AggregateByUser aggregates cost by user for a tenant within a time range
	AggregateByUser(ctx context.Context, tenantID uint64, start, end time.Time, limit int) ([]*types.UserCostBreakdown, error)
	// GetSummary gets total cost and token summary for a time range
	GetSummary(ctx context.Context, tenantID uint64, start, end time.Time) (totalCost float64, totalTokens int, totalCalls int, err error)
}

// CostTrackingService defines the cost tracking service interface
type CostTrackingService interface {
	// LogCall records a single LLM call with automatic cost calculation
	LogCall(ctx context.Context, log *types.LLMCallLog) error
	// LogCallWithUsage is a convenience method to log a call with TokenUsage
	LogCallWithUsage(
		ctx context.Context,
		tenantID uint64,
		userID, sessionID, kbID, modelID string,
		modelType types.ModelType,
		requestType types.LLMRequestType,
		usage *types.TokenUsage,
		latencyMs int,
		err error,
		traceID string,
	) error
	// GetCostDashboard returns the complete cost dashboard data for a tenant
	GetCostDashboard(ctx context.Context, tenantID uint64, days int) (*types.CostDashboardSummary, error)
	// GetModelCostBreakdown returns cost breakdown by model
	GetModelCostBreakdown(ctx context.Context, tenantID uint64, start, end time.Time) ([]*types.ModelCostBreakdown, error)
	// GetDailyCostTrend returns daily cost trend data
	GetDailyCostTrend(ctx context.Context, tenantID uint64, start, end time.Time) ([]*types.CostAggregation, error)
}
