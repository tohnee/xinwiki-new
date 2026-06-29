package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LLMRequestType represents the type of LLM request
type LLMRequestType string

const (
	LLMRequestTypeChatCompletion LLMRequestType = "chat_completion"
	LLMRequestTypeEmbedding      LLMRequestType = "embedding"
	LLMRequestTypeRerank         LLMRequestType = "rerank"
	LLMRequestTypeVLM            LLMRequestType = "vlm"
	LLMRequestTypeASR            LLMRequestType = "asr"
	LLMRequestTypeSummary        LLMRequestType = "summary"
	LLMRequestTypeQueryRewrite   LLMRequestType = "query_rewrite"
)

// LLMCallStatus represents the status of an LLM call
type LLMCallStatus string

const (
	LLMCallStatusSuccess LLMCallStatus = "success"
	LLMCallStatusError   LLMCallStatus = "error"
	LLMCallStatusTimeout LLMCallStatus = "timeout"
)

// ModelRoutingStrategy represents the routing strategy used for model selection
type ModelRoutingStrategy string

const (
	RoutingStrategyDefault        ModelRoutingStrategy = "default"
	RoutingStrategyCostOptimized  ModelRoutingStrategy = "cost_optimized"
	RoutingStrategyLatencyOptimized ModelRoutingStrategy = "latency_optimized"
	RoutingStrategyQualityFirst   ModelRoutingStrategy = "quality_first"
	RoutingStrategyFallback       ModelRoutingStrategy = "fallback"
	RoutingStrategySecurityForced ModelRoutingStrategy = "security_forced"
)

// RouteDecision records the model routing decision details
type RouteDecision struct {
	Strategy       ModelRoutingStrategy `json:"strategy"`
	SelectedModel  string               `json:"selected_model"`
	Reason         string               `json:"reason"`
	FallbackFrom   string               `json:"fallback_from,omitempty"`
	BudgetExceeded bool                 `json:"budget_exceeded"`
}

// LLMCallLog represents a single LLM API call record for cost tracking
type LLMCallLog struct {
	ID                   string               `json:"id"                   gorm:"type:varchar(36);primaryKey"`
	TenantID             uint64               `json:"tenant_id"             gorm:"index"`
	UserID               string               `json:"user_id"               gorm:"type:varchar(36);index"`
	SessionID            string               `json:"session_id"            gorm:"type:varchar(36);index"`
	KBID                 string               `json:"kb_id"                 gorm:"type:varchar(36)"`
	ModelID              string               `json:"model_id"              gorm:"type:varchar(64);index;not null"`
	ModelType            ModelType            `json:"model_type"            gorm:"type:varchar(50);default:'KnowledgeQA'"`
	RequestType          LLMRequestType       `json:"request_type"          gorm:"type:varchar(50);default:'chat_completion';not null"`
	PromptTokens         int                  `json:"prompt_tokens"         gorm:"default:0;not null"`
	CompletionTokens     int                  `json:"completion_tokens"     gorm:"default:0;not null"`
	CachedTokens         int                  `json:"cached_tokens"         gorm:"default:0;not null"`
	TotalTokens          int                  `json:"total_tokens"          gorm:"default:0;not null"`
	EstimatedCost        float64              `json:"estimated_cost"        gorm:"type:decimal(20,10);default:0;not null"`
	LatencyMs            int                  `json:"latency_ms"`
	Status               LLMCallStatus        `json:"status"                gorm:"type:varchar(20);default:'success';not null"`
	ErrorMessage         string               `json:"error_message"         gorm:"type:text"`
	TraceID              string               `json:"trace_id"              gorm:"type:varchar(64)"`
	RouteStrategy        ModelRoutingStrategy `json:"route_strategy"        gorm:"type:varchar(32);default:'default'"`
	RouteReason          string               `json:"route_reason"          gorm:"type:varchar(256)"`
	PromptTemplateID     string               `json:"prompt_template_id"    gorm:"type:varchar(64)"`
	PromptTemplateVersion string              `json:"prompt_template_version" gorm:"type:varchar(32)"`
	CreatedAt            time.Time            `json:"created_at"`
	DeletedAt            gorm.DeletedAt       `json:"-"                     gorm:"index"`
}

// BeforeCreate generates UUID for new records
func (l *LLMCallLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now()
	}
	if l.TotalTokens == 0 {
		l.TotalTokens = l.PromptTokens + l.CompletionTokens
	}
	return nil
}

// CostAggregation represents aggregated cost data for a time period
type CostAggregation struct {
	Date             string  `json:"date"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	TotalCost        float64 `json:"total_cost"`
	CallCount        int     `json:"call_count"`
}

// ModelCostBreakdown represents cost broken down by model
type ModelCostBreakdown struct {
	ModelID          string  `json:"model_id"`
	ModelName        string  `json:"model_name"`
	TotalTokens      int     `json:"total_tokens"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	TotalCost        float64 `json:"total_cost"`
	CallCount        int     `json:"call_count"`
	Percentage       float64 `json:"percentage"`
}

// UserCostBreakdown represents cost broken down by user
type UserCostBreakdown struct {
	UserID           string  `json:"user_id"`
	UserName         string  `json:"user_name,omitempty"`
	TotalTokens      int     `json:"total_tokens"`
	TotalCost        float64 `json:"total_cost"`
	CallCount        int     `json:"call_count"`
	Percentage       float64 `json:"percentage"`
}

// CostDashboardSummary represents the cost dashboard summary data
type CostDashboardSummary struct {
	Period           string               `json:"period"`
	StartDate        time.Time            `json:"start_date"`
	EndDate          time.Time            `json:"end_date"`
	TotalCost        float64              `json:"total_cost"`
	TotalTokens      int                  `json:"total_tokens"`
	TotalCalls       int                  `json:"total_calls"`
	AvgCostPerCall   float64              `json:"avg_cost_per_call"`
	DailyTrend       []CostAggregation    `json:"daily_trend"`
	ModelBreakdown   []ModelCostBreakdown `json:"model_breakdown"`
	TopUsers         []UserCostBreakdown  `json:"top_users,omitempty"`
}

// CostQuery represents a multi-dimensional cost query
type CostQuery struct {
	TenantID     uint64     `json:"tenant_id"`
	StartDate    time.Time  `json:"start_date"`
	EndDate      time.Time  `json:"end_date"`
	ModelIDs     []string   `json:"model_ids,omitempty"`
	RequestTypes []LLMRequestType `json:"request_types,omitempty"`
	UserIDs      []string   `json:"user_ids,omitempty"`
	Granularity  string     `json:"granularity,omitempty"` // hour, day, week, month
	GroupBy      []string   `json:"group_by,omitempty"`    // model, user, request_type, day
}

// CostTrendPoint represents a single point in cost trend
type CostTrendPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	ModelID          string    `json:"model_id,omitempty"`
	ModelName        string    `json:"model_name,omitempty"`
	RequestType      string    `json:"request_type,omitempty"`
	UserID           string    `json:"user_id,omitempty"`
	TotalTokens      int       `json:"total_tokens"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CachedTokens     int       `json:"cached_tokens"`
	TotalCost        float64   `json:"total_cost"`
	CallCount        int       `json:"call_count"`
	AvgLatencyMs     int       `json:"avg_latency_ms"`
	SuccessCount     int       `json:"success_count"`
	ErrorCount       int       `json:"error_count"`
}

// CostSummary represents aggregated cost summary with breakdowns
type CostSummary struct {
	StartDate        time.Time          `json:"start_date"`
	EndDate          time.Time          `json:"end_date"`
	TotalCost        float64            `json:"total_cost"`
	TotalTokens      int                `json:"total_tokens"`
	TotalCalls       int                `json:"total_calls"`
	SuccessRate      float64            `json:"success_rate"`
	AvgLatencyMs     int                `json:"avg_latency_ms"`
	AvgCostPerCall   float64            `json:"avg_cost_per_call"`
	ByModel          []ModelCostBreakdown `json:"by_model,omitempty"`
	ByRequestType    []CostTrendPoint   `json:"by_request_type,omitempty"`
	ByDay            []CostAggregation  `json:"by_day,omitempty"`
	TopUsers         []UserCostBreakdown `json:"top_users,omitempty"`
}

// ModelLatencyStats represents latency statistics for a model
type ModelLatencyStats struct {
	ModelID       string  `json:"model_id"`
	ModelName     string  `json:"model_name"`
	AvgLatencyMs  int     `json:"avg_latency_ms"`
	P50LatencyMs  int     `json:"p50_latency_ms"`
	P95LatencyMs  int     `json:"p95_latency_ms"`
	P99LatencyMs  int     `json:"p99_latency_ms"`
	CallCount     int     `json:"call_count"`
	SuccessRate   float64 `json:"success_rate"`
	AvgCostPer1K  float64 `json:"avg_cost_per_1k"`
}
