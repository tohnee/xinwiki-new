package types

import "time"

// SecurityLevel represents the security level of a task
type SecurityLevel string

const (
	SecurityLevelLow      SecurityLevel = "low"
	SecurityLevelMedium   SecurityLevel = "medium"
	SecurityLevelHigh     SecurityLevel = "high"
	SecurityLevelCritical SecurityLevel = "critical"
)

// TaskType represents the type of LLM task
type TaskType string

const (
	TaskTypeChat          TaskType = "chat"
	TaskTypeQA            TaskType = "qa"
	TaskTypeSummary       TaskType = "summary"
	TaskTypeRewrite       TaskType = "rewrite"
	TaskTypeExtraction    TaskType = "extraction"
	TaskTypeClassification TaskType = "classification"
	TaskTypeEmbedding     TaskType = "embedding"
	TaskTypeRerank        TaskType = "rerank"
)

// ModelTier represents the tier of a model
type ModelTier string

const (
	ModelTierBasic    ModelTier = "basic"
	ModelTierStandard ModelTier = "standard"
	ModelTierAdvanced ModelTier = "advanced"
	ModelTierPremium  ModelTier = "premium"
)

// ModelVendor represents model vendor type for security enforcement
type ModelVendor string

const (
	ModelVendorInternal ModelVendor = "internal"
	ModelVendorExternal ModelVendor = "external"
	ModelVendorHybrid   ModelVendor = "hybrid"
)

// ModelRoutingPolicy represents tenant-level model routing policy
type ModelRoutingPolicy struct {
	ID                  string          `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID            uint64          `json:"tenant_id" gorm:"index;not null"`
	DefaultModelID      string          `json:"default_model_id" gorm:"type:varchar(64);not null"`
	Strategy            ModelRoutingStrategy `json:"strategy" gorm:"type:varchar(32);default:'cost_optimized'"`
	DailyBudgetUSD      float64         `json:"daily_budget_usd" gorm:"type:decimal(20,10);default:0"`
	MonthlyBudgetUSD    float64         `json:"monthly_budget_usd" gorm:"type:decimal(20,10);default:0"`
	MaxLatencyMs        int             `json:"max_latency_ms" gorm:"default:0"`
	AllowExternalModels bool            `json:"allow_external_models" gorm:"default:true"`
	SecurityForcedModels map[SecurityLevel]string `json:"security_forced_models" gorm:"serializer:json"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// ModelSelectRequest represents a model selection request
type ModelSelectRequest struct {
	TenantID       uint64        `json:"tenant_id"`
	UserID         string        `json:"user_id"`
	TaskType       TaskType      `json:"task_type"`
	SecurityLevel  SecurityLevel `json:"security_level"`
	EstimatedInputTokens int    `json:"estimated_input_tokens"`
	MaxCostPerCall float64       `json:"max_cost_per_call"`
	MaxLatencyMs   int           `json:"max_latency_ms"`
	PreferModelID  string        `json:"prefer_model_id"`
}

// ModelSelectResult represents the result of model selection
type ModelSelectResult struct {
	ModelID    string     `json:"model_id"`
	ModelName  string     `json:"model_name"`
	RouteDecision RouteDecision `json:"route_decision"`
	EstimatedCost float64  `json:"estimated_cost"`
	EstimatedLatencyMs int `json:"estimated_latency_ms"`
}

// ModelPerformanceStats records model performance metrics for routing decisions
type ModelPerformanceStats struct {
	ModelID         string  `json:"model_id"`
	AvgLatencyMs    int     `json:"avg_latency_ms"`
	P95LatencyMs    int     `json:"p95_latency_ms"`
	AvgCostPer1KTok float64 `json:"avg_cost_per_1k_tok"`
	SuccessRate     float64 `json:"success_rate"`
	LastUpdated     time.Time `json:"last_updated"`
}

// PromptTemplate represents a versioned prompt template
type PromptTemplate struct {
	ID          string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TemplateKey string    `json:"template_key" gorm:"type:varchar(64);not null;uniqueIndex:idx_tenant_key_version,priority:1"`
	TenantID    uint64    `json:"tenant_id" gorm:"uniqueIndex:idx_tenant_key_version,priority:2;default:0"`
	Version     string    `json:"version" gorm:"type:varchar(32);not null;uniqueIndex:idx_tenant_key_version,priority:3"`
	Content     string    `json:"content" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"type:varchar(256)"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedBy   string    `json:"created_by" gorm:"type:varchar(36)"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetActiveVersion returns the active version string for a template
func (p *PromptTemplate) GetVersionString() string {
	return p.Version
}
