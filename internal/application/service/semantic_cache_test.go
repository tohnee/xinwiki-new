package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSemanticCache_CosineSimilarity(t *testing.T) {
	t.Run("identical_vectors_returns_1", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		score := cosineSimilarity(a, a)
		assert.InDelta(t, 1.0, score, 0.0001)
	})

	t.Run("orthogonal_vectors_returns_0", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{0.0, 1.0, 0.0}
		score := cosineSimilarity(a, b)
		assert.InDelta(t, 0.0, score, 0.0001)
	})

	t.Run("opposite_vectors_returns_-1", func(t *testing.T) {
		a := []float32{1.0, 0.0}
		b := []float32{-1.0, 0.0}
		score := cosineSimilarity(a, b)
		assert.InDelta(t, -1.0, score, 0.0001)
	})

	t.Run("similar_vectors_high_score", func(t *testing.T) {
		a := []float32{1.0, 0.0, 0.0}
		b := []float32{0.99, 0.14, 0.0}
		score := cosineSimilarity(a, b)
		assert.Greater(t, score, 0.98, "highly similar vectors should have score > 0.98")
	})

	t.Run("different_length_returns_0", func(t *testing.T) {
		a := []float32{1.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}
		score := cosineSimilarity(a, b)
		assert.Equal(t, 0.0, score)
	})

	t.Run("empty_vectors_returns_0", func(t *testing.T) {
		score := cosineSimilarity([]float32{}, []float32{})
		assert.Equal(t, 0.0, score)
	})

	t.Run("zero_vector_returns_0", func(t *testing.T) {
		a := []float32{0.0, 0.0, 0.0}
		b := []float32{1.0, 0.0, 0.0}
		score := cosineSimilarity(a, b)
		assert.Equal(t, 0.0, score)
	})
}

func TestGenerateCacheKey(t *testing.T) {
	t.Run("same_kbs_different_order_same_key", func(t *testing.T) {
		key1 := generateCacheKey(1, []string{"kb-a", "kb-b", "kb-c"})
		key2 := generateCacheKey(1, []string{"kb-c", "kb-a", "kb-b"})
		assert.Equal(t, key1, key2, "order of KB IDs should not affect cache key")
	})

	t.Run("different_tenants_different_key", func(t *testing.T) {
		key1 := generateCacheKey(1, []string{"kb-001"})
		key2 := generateCacheKey(2, []string{"kb-001"})
		assert.NotEqual(t, key1, key2)
	})

	t.Run("different_kbs_different_key", func(t *testing.T) {
		key1 := generateCacheKey(1, []string{"kb-001"})
		key2 := generateCacheKey(1, []string{"kb-002"})
		assert.NotEqual(t, key1, key2)
	})

	t.Run("key_is_consistent_length", func(t *testing.T) {
		key := generateCacheKey(12345, []string{"kb-001", "kb-002"})
		assert.Len(t, key, 16, "cache key should be 16 hex characters")
	})
}

func TestGenerateEntryID(t *testing.T) {
	t.Run("generates_unique_ids", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateEntryID()
			assert.False(t, ids[id], "ID should be unique")
			ids[id] = true
			assert.Len(t, id, 24, "entry ID should be 24 hex characters")
		}
	})
}
