package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// RAGEvaluationService handles RAG quality evaluation including citation accuracy
type RAGEvaluationService interface {
	// EvaluateCitationAccuracy evaluates the accuracy of citations in a RAG response
	EvaluateCitationAccuracy(ctx context.Context, req *types.CitationEvaluationRequest) (*types.CitationAccuracyReport, error)

	// EvaluateBatch evaluates multiple queries in batch
	EvaluateBatch(ctx context.Context, req *types.BatchEvaluationRequest) (*types.BatchEvaluationResult, error)

	// GetReport retrieves a citation accuracy report by ID
	GetReport(ctx context.Context, tenantID uint64, reportID string) (*types.CitationAccuracyReport, error)

	// ListReports lists citation accuracy reports with filtering
	ListReports(ctx context.Context, tenantID uint64, kbID string, from, to time.Time, page, pageSize int) ([]*types.CitationAccuracyReport, int, error)

	// GetEvaluationSummary returns aggregate evaluation metrics
	GetEvaluationSummary(ctx context.Context, tenantID uint64, kbID string, from, to time.Time) (*types.EvaluationSummary, error)

	// ExtractCitations extracts citations from a response text
	ExtractCitations(ctx context.Context, response string, chunks []*types.Chunk) ([]types.Citation, error)

	// VerifyCitation verifies if a single citation is supported by the source chunk
	VerifyCitation(ctx context.Context, claim string, sourceContent string) (*types.CitationEvaluation, error)
}

// EvaluationRepository defines the data access layer for evaluation reports
type EvaluationRepository interface {
	CreateReport(ctx context.Context, report *types.CitationAccuracyReport) error
	GetReport(ctx context.Context, tenantID uint64, id string) (*types.CitationAccuracyReport, error)
	ListReports(ctx context.Context, tenantID uint64, kbID string, from, to time.Time, page, pageSize int) ([]*types.CitationAccuracyReport, int, error)
	UpdateReport(ctx context.Context, report *types.CitationAccuracyReport) error
	DeleteReport(ctx context.Context, tenantID uint64, id string) error
	GetAggregateMetrics(ctx context.Context, tenantID uint64, kbID string, from, to time.Time) (precision, recall, f1, groundedness, hallucination float64, count int, err error)
}
