package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

// DatasetService provides operations for working with evaluation datasets.
type DatasetService interface {
	GetDatasetByID(ctx context.Context, datasetID string) ([]*types.QAPair, error)
}

// EvaluationService handles evaluation task orchestration for knowledge base and chat models.
type EvaluationService interface {
	Evaluation(ctx context.Context, datasetID, knowledgeBaseID, chatModelID, rerankModelID string) (*types.EvaluationDetail, error)
	EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error)
}

// LLMService provides LLM capabilities used by evaluation and other services.
// It reuses the chat.Chat interface methods needed for evaluation tasks.
type LLMService interface {
	// Chat performs a non-streaming chat completion
	Chat(ctx context.Context, system string, user string) (string, error)
}

// Metrics is the interface that all evaluation metric calculators implement.
type Metrics interface {
	Compute(input *types.MetricInput) float64
}
