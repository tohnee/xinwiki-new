package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

type SemanticCacheService interface {
	Get(ctx context.Context, tenantID uint64, kbIDs []string, queryEmbedding []float32, threshold float64) (*types.SemanticCacheEntry, error)
	Set(ctx context.Context, entry *types.SemanticCacheEntry) error
	InvalidateByKB(ctx context.Context, tenantID uint64, kbID string) error
	InvalidateAll(ctx context.Context, tenantID uint64) error
	Stats(ctx context.Context) (*types.SemanticCacheStats, error)
}
