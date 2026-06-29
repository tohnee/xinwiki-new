package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/XinWiki/internal/types"
)

func TestMemorySemanticCache_BasicSetGet(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)
	kbIDs := []string{"kb-001"}
	embedding := []float32{1.0, 0.0, 0.0, 0.0}

	entry := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: kbIDs,
		QueryText:        "什么是机器学习？",
		QueryEmbedding:   embedding,
		Results: []*types.SearchResult{
			{ID: "chunk-001", Score: 0.95},
			{ID: "chunk-002", Score: 0.88},
		},
		ChunkMap: map[string]*types.Chunk{
			"chunk-001": {ID: "chunk-001", Content: "机器学习是人工智能的一个分支"},
			"chunk-002": {ID: "chunk-002", Content: "机器学习可以从数据中学习规律"},
		},
	}

	err := cache.Set(ctx, entry)
	require.NoError(t, err)

	t.Run("exact_match_hit", func(t *testing.T) {
		got, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, 2, len(got.Results))
		assert.Equal(t, "chunk-001", got.Results[0].ID)
		assert.Equal(t, int64(1), got.HitCount)
	})

	t.Run("similar_query_hit", func(t *testing.T) {
		similarEmbedding := []float32{0.99, 0.01, 0.0, 0.0}
		got, err := cache.Get(ctx, tenantID, kbIDs, similarEmbedding, 0.9)
		require.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("different_tenant_miss", func(t *testing.T) {
		got, err := cache.Get(ctx, uint64(999), kbIDs, embedding, 0.9)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("different_kb_miss", func(t *testing.T) {
		got, err := cache.Get(ctx, tenantID, []string{"kb-999"}, embedding, 0.9)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("low_similarity_miss", func(t *testing.T) {
		differentEmbedding := []float32{0.0, 1.0, 0.0, 0.0}
		got, err := cache.Get(ctx, tenantID, kbIDs, differentEmbedding, 0.9)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestMemorySemanticCache_Expiration(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cfg.TTL = 100 * time.Millisecond
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)
	kbIDs := []string{"kb-001"}
	embedding := []float32{1.0, 0.0}

	entry := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: kbIDs,
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-001"}},
	}

	err := cache.Set(ctx, entry)
	require.NoError(t, err)

	got, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
	require.NoError(t, err)
	assert.NotNil(t, got)

	time.Sleep(150 * time.Millisecond)

	got, err = cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
	require.NoError(t, err)
	assert.Nil(t, got, "expired entry should not be returned")
}

func TestMemorySemanticCache_InvalidateByKB(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)
	embedding := []float32{1.0, 0.0}

	entry1 := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: []string{"kb-001"},
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-001"}},
	}
	entry2 := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: []string{"kb-002"},
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-002"}},
	}

	require.NoError(t, cache.Set(ctx, entry1))
	require.NoError(t, cache.Set(ctx, entry2))

	got1, _ := cache.Get(ctx, tenantID, []string{"kb-001"}, embedding, 0.9)
	assert.NotNil(t, got1)
	got2, _ := cache.Get(ctx, tenantID, []string{"kb-002"}, embedding, 0.9)
	assert.NotNil(t, got2)

	err := cache.InvalidateByKB(ctx, tenantID, "kb-001")
	require.NoError(t, err)

	got1, _ = cache.Get(ctx, tenantID, []string{"kb-001"}, embedding, 0.9)
	assert.Nil(t, got1, "kb-001 entries should be invalidated")
	got2, _ = cache.Get(ctx, tenantID, []string{"kb-002"}, embedding, 0.9)
	assert.NotNil(t, got2, "kb-002 entries should remain")
}

func TestMemorySemanticCache_InvalidateAll(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)
	embedding := []float32{1.0, 0.0}

	entry := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: []string{"kb-001"},
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-001"}},
	}
	require.NoError(t, cache.Set(ctx, entry))

	entry2 := &types.SemanticCacheEntry{
		TenantID:         uint64(2),
		KnowledgeBaseIDs: []string{"kb-001"},
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-002"}},
	}
	require.NoError(t, cache.Set(ctx, entry2))

	err := cache.InvalidateAll(ctx, tenantID)
	require.NoError(t, err)

	got, _ := cache.Get(ctx, tenantID, []string{"kb-001"}, embedding, 0.9)
	assert.Nil(t, got, "tenant 1 entries should be invalidated")

	got2, _ := cache.Get(ctx, uint64(2), []string{"kb-001"}, embedding, 0.9)
	assert.NotNil(t, got2, "tenant 2 entries should remain")
}

func TestMemorySemanticCache_Stats(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)
	kbIDs := []string{"kb-001"}
	embedding := []float32{1.0, 0.0}

	for i := 0; i < 3; i++ {
		_, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
		require.NoError(t, err)
	}

	entry := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: kbIDs,
		QueryEmbedding:   embedding,
		Results:          []*types.SearchResult{{ID: "chunk-001"}},
	}
	require.NoError(t, cache.Set(ctx, entry))

	for i := 0; i < 7; i++ {
		_, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
		require.NoError(t, err)
	}

	stats, err := cache.Stats(ctx)
	require.NoError(t, err)
	assert.True(t, stats.Enabled)
	assert.Equal(t, "memory", stats.Backend)
	assert.Equal(t, int64(1), stats.TotalEntries)
	assert.Equal(t, int64(7), stats.TotalHits)
	assert.Equal(t, int64(3), stats.TotalMisses)
	assert.InDelta(t, 0.7, stats.HitRate, 0.01, "hit rate should be 70%%")
}

func TestMemorySemanticCache_MaxEntriesEviction(t *testing.T) {
	cfg := types.DefaultSemanticCacheConfig()
	cfg.MaxEntries = 3
	cache := NewMemorySemanticCache(cfg)
	ctx := context.Background()

	tenantID := uint64(1)

	for i := 0; i < 5; i++ {
		emb := make([]float32, 4)
		emb[i%4] = 1.0
		entry := &types.SemanticCacheEntry{
			ID:               string(rune('a' + i)),
			TenantID:         tenantID,
			KnowledgeBaseIDs: []string{"kb-001"},
			QueryEmbedding:   emb,
			Results:          []*types.SearchResult{{ID: string(rune('a' + i))}},
		}
		require.NoError(t, cache.Set(ctx, entry))
	}

	stats, _ := cache.Stats(ctx)
	assert.LessOrEqual(t, stats.TotalEntries, int64(3), "should evict entries when exceeding max")
}
