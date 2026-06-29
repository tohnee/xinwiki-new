package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CitationStatus represents the validity status of a citation
type CitationStatus string

const (
	CitationStatusSupported   CitationStatus = "supported"
	CitationStatusPartial     CitationStatus = "partial"
	CitationStatusContradicted CitationStatus = "contradicted"
	CitationStatusUnverified  CitationStatus = "unverified"
	CitationStatusMissing     CitationStatus = "missing"
)

// EvaluationStatus represents the status of an evaluation run
type EvaluationStatus string

const (
	EvaluationStatusPending   EvaluationStatus = "pending"
	EvaluationStatusRunning   EvaluationStatus = "running"
	EvaluationStatusCompleted EvaluationStatus = "completed"
	EvaluationStatusFailed    EvaluationStatus = "failed"
)

// Citation represents a single citation in a RAG response
type Citation struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	DocID     string `json:"doc_id,omitempty"`
	ChunkID   string `json:"chunk_id,omitempty"`
	StartPos  int    `json:"start_pos,omitempty"`
	EndPos    int    `json:"end_pos,omitempty"`
	Content   string `json:"content,omitempty"`
}

// CitationEvaluation represents the evaluation result for a single citation
type CitationEvaluation struct {
	Citation        Citation       `json:"citation"`
	Status          CitationStatus `json:"status"`
	Confidence      float64        `json:"confidence"`
	RelevantSnippets []string      `json:"relevant_snippets,omitempty"`
	Reasoning       string         `json:"reasoning,omitempty"`
	IsRelevant      bool           `json:"is_relevant"`
	SupportsClaim   bool           `json:"supports_claim"`
}

// CitationAccuracyReport represents a complete citation accuracy evaluation
type CitationAccuracyReport struct {
	ID                string                 `json:"id"                 gorm:"type:varchar(36);primaryKey"`
	TenantID          uint64                 `json:"tenant_id"           gorm:"index;not null"`
	KBID              string                 `json:"kb_id"               gorm:"type:varchar(36);index;not null"`
	Query             string                 `json:"query"               gorm:"type:text;not null"`
	Response          string                 `json:"response"            gorm:"type:text;not null"`
	Citations         []Citation             `json:"citations"           gorm:"serializer:json"`
	Evaluations       []CitationEvaluation   `json:"evaluations"         gorm:"serializer:json"`
	TotalCitations    int                    `json:"total_citations"`
	SupportedCount    int                    `json:"supported_count"`
	PartialCount      int                    `json:"partial_count"`
	ContradictedCount int                    `json:"contradicted_count"`
	MissingCount      int                    `json:"missing_count"`
	UnverifiedCount   int                    `json:"unverified_count"`
	Precision         float64                `json:"precision"`
	Recall            float64                `json:"recall"`
	F1Score           float64                `json:"f1_score"`
	GroundednessScore float64                `json:"groundedness_score"`
	HallucinationRate float64                `json:"hallucination_rate"`
	OverallScore      float64                `json:"overall_score"`
	Summary           string                 `json:"summary"             gorm:"type:text"`
	Metadata          map[string]interface{} `json:"metadata"            gorm:"serializer:json"`
	Status            EvaluationStatus       `json:"status"              gorm:"type:varchar(16);default:'pending'"`
	ErrorMsg          string                 `json:"error_msg,omitempty" gorm:"type:text"`
	DurationMs        int64                  `json:"duration_ms"`
	EvaluatedBy       string                 `json:"evaluated_by"        gorm:"type:varchar(32);default:'automated'"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	DeletedAt         gorm.DeletedAt         `json:"-"                   gorm:"index"`
}

// BeforeCreate generates UUID for new reports
func (r *CitationAccuracyReport) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	if r.UpdatedAt.IsZero() {
		r.UpdatedAt = r.CreatedAt
	}
	if r.Status == "" {
		r.Status = EvaluationStatusPending
	}
	return nil
}

// BeforeUpdate sets updated_at timestamp
func (r *CitationAccuracyReport) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = time.Now()
	if r.Status == EvaluationStatusCompleted && r.CompletedAt == nil {
		now := time.Now()
		r.CompletedAt = &now
	}
	return nil
}

// CalculateScores computes the accuracy metrics from evaluations
func (r *CitationAccuracyReport) CalculateScores() {
	r.TotalCitations = len(r.Evaluations)
	r.SupportedCount = 0
	r.PartialCount = 0
	r.ContradictedCount = 0
	r.MissingCount = 0
	r.UnverifiedCount = 0

	for _, eval := range r.Evaluations {
		switch eval.Status {
		case CitationStatusSupported:
			r.SupportedCount++
		case CitationStatusPartial:
			r.PartialCount++
		case CitationStatusContradicted:
			r.ContradictedCount++
		case CitationStatusMissing:
			r.MissingCount++
		default:
			r.UnverifiedCount++
		}
	}

	if r.TotalCitations > 0 {
		r.Precision = float64(r.SupportedCount) / float64(r.TotalCitations)
		r.GroundednessScore = float64(r.SupportedCount+r.PartialCount) / float64(r.TotalCitations)
		r.HallucinationRate = float64(r.ContradictedCount+r.MissingCount) / float64(r.TotalCitations)
		r.Recall = r.GroundednessScore
		if r.Precision+r.Recall > 0 {
			r.F1Score = 2 * r.Precision * r.Recall / (r.Precision + r.Recall)
		}
		r.OverallScore = (r.Precision + r.GroundednessScore - r.HallucinationRate) / 2
	}

	r.OverallScore = max(0, min(1, r.OverallScore))
}

// CitationEvaluationRequest represents a request to evaluate citation accuracy
type CitationEvaluationRequest struct {
	TenantID    uint64     `json:"tenant_id" validate:"required"`
	KBID        string     `json:"kb_id" validate:"required"`
	Query       string     `json:"query" validate:"required"`
	Response    string     `json:"response" validate:"required"`
	Citations   []Citation `json:"citations,omitempty"`
	ContextChunks []Chunk  `json:"context_chunks,omitempty"`
	ModelID     string     `json:"model_id,omitempty"`
	StrictMode  bool       `json:"strict_mode,omitempty"`
}

// BatchEvaluationRequest represents a batch evaluation request
type BatchEvaluationRequest struct {
	TenantID uint64                        `json:"tenant_id" validate:"required"`
	KBID     string                        `json:"kb_id" validate:"required"`
	Queries  []CitationEvaluationRequest   `json:"queries" validate:"required,min=1,max=100"`
}

// BatchEvaluationResult represents batch evaluation results with aggregate metrics
type BatchEvaluationResult struct {
	TotalQueries       int                    `json:"total_queries"`
	CompletedQueries   int                    `json:"completed_queries"`
	FailedQueries      int                    `json:"failed_queries"`
	AvgPrecision       float64                `json:"avg_precision"`
	AvgRecall          float64                `json:"avg_recall"`
	AvgF1              float64                `json:"avg_f1"`
	AvgGroundedness    float64                `json:"avg_groundedness"`
	AvgHallucination   float64                `json:"avg_hallucination"`
	Reports            []*CitationAccuracyReport `json:"reports"`
	DurationMs         int64                  `json:"duration_ms"`
	CreatedAt          time.Time              `json:"created_at"`
}

// EvaluationSummary provides summary statistics across evaluations
type EvaluationSummary struct {
	TotalEvaluations   int                    `json:"total_evaluations"`
	AvgPrecision       float64                `json:"avg_precision"`
	AvgRecall          float64                `json:"avg_recall"`
	AvgF1              float64                `json:"avg_f1"`
	AvgGroundedness    float64                `json:"avg_groundedness"`
	AvgHallucination   float64                `json:"avg_hallucination"`
	BestPerformingModel string               `json:"best_performing_model,omitempty"`
	RecentEvaluations  []*CitationAccuracyReport `json:"recent_evaluations"`
}
