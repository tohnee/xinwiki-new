package service

import (
	"context"
	"sync"

	"github.com/Tencent/XinWiki/internal/models/embedding"
)

// EmbeddingBatcherManager manages multiple EmbeddingBatcher instances, one per unique model identity key
type EmbeddingBatcherManager struct {
	mu       sync.RWMutex
	config   EmbeddingBatcherConfig
	batchers map[string]*EmbeddingBatcher
}

// NewEmbeddingBatcherManager creates a new batcher manager
func NewEmbeddingBatcherManager(config EmbeddingBatcherConfig) *EmbeddingBatcherManager {
	return &EmbeddingBatcherManager{
		config:   config,
		batchers: make(map[string]*EmbeddingBatcher),
	}
}

// GetOrCreateBatcher returns an existing batcher for the model key, or creates a new one
func (m *EmbeddingBatcherManager) GetOrCreateBatcher(modelKey string, embedder embedding.Embedder) *EmbeddingBatcher {
	m.mu.RLock()
	b, ok := m.batchers[modelKey]
	m.mu.RUnlock()
	if ok {
		return b
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if b, ok = m.batchers[modelKey]; ok {
		return b
	}

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		return embedder.BatchEmbed(ctx, texts)
	}

	b = NewEmbeddingBatcher(modelKey, m.config, batchFn)
	m.batchers[modelKey] = b
	return b
}

// Embed is a convenience method that routes to the correct model-specific batcher
func (m *EmbeddingBatcherManager) Embed(ctx context.Context, modelKey string, embedder embedding.Embedder, text string) ([]float32, error) {
	b := m.GetOrCreateBatcher(modelKey, embedder)
	return b.Embed(ctx, text)
}

// Shutdown shuts down all batchers gracefully
func (m *EmbeddingBatcherManager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, b := range m.batchers {
		b.Shutdown()
	}
	m.batchers = make(map[string]*EmbeddingBatcher)
}

// Stats returns aggregated stats across all batchers
func (m *EmbeddingBatcherManager) Stats() map[string]BatcherStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]BatcherStats, len(m.batchers))
	for modelKey, b := range m.batchers {
		stats[modelKey] = b.Stats()
	}
	return stats
}
