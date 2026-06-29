package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingBatcher_BasicBatching(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	var receivedTexts []string

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		atomic.AddInt64(&callCount, 1)
		mu.Lock()
		receivedTexts = append(receivedTexts, texts...)
		mu.Unlock()

		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{float32(i), 1.0, 0.0}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 5
	cfg.MaxWaitTime = 10 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	// Send 7 concurrent requests - should be split into 2 batches (5 + 2)
	const numRequests = 7
	var wg sync.WaitGroup
	wg.Add(numRequests)

	results := make([][]float32, numRequests)
	errors := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			defer wg.Done()
			res, err := batcher.Embed(context.Background(), fmt.Sprintf("text-%d", idx))
			results[idx] = res
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all requests succeeded
	for i := 0; i < numRequests; i++ {
		require.NoError(t, errors[i])
		require.NotNil(t, results[i])
		assert.Len(t, results[i], 3)
	}

	// Should have made 2 batch calls (5 + 2)
	finalCallCount := atomic.LoadInt64(&callCount)
	assert.LessOrEqual(t, finalCallCount, int64(2), "should make at most 2 batch calls for 7 requests with max batch size 5")
	assert.GreaterOrEqual(t, finalCallCount, int64(1), "should batch at least some requests")

	// Verify all texts were received
	mu.Lock()
	assert.Len(t, receivedTexts, numRequests)
	mu.Unlock()
}

func TestEmbeddingBatcher_DuplicateDeduplication(t *testing.T) {
	var callCount int64
	var mu sync.Mutex
	var receivedTexts []string

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		atomic.AddInt64(&callCount, 1)
		mu.Lock()
		receivedTexts = append(receivedTexts, texts...)
		mu.Unlock()

		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{float32(i), 1.0, 0.0}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 10
	cfg.MaxWaitTime = 20 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	// Send 10 identical requests - should result in only 1 unique text in batch
	const numRequests = 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			res, err := batcher.Embed(context.Background(), "same-text-for-everyone")
			assert.NoError(t, err)
			assert.NotNil(t, res)
		}()
	}

	wg.Wait()

	// All 10 requests should succeed, but batch should only contain 1 unique text
	mu.Lock()
	assert.Len(t, receivedTexts, 1, "duplicate texts should be deduplicated in batch")
	assert.Equal(t, "same-text-for-everyone", receivedTexts[0])
	mu.Unlock()

	assert.Equal(t, int64(1), atomic.LoadInt64(&callCount))

	stats := batcher.Stats()
	assert.Equal(t, int64(10), stats.BatchedRequests)
	assert.Equal(t, int64(9), stats.MergeHits, "9 duplicates should be merged")
}

func TestEmbeddingBatcher_TimeWindowFlush(t *testing.T) {
	var callCount int64

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		atomic.AddInt64(&callCount, 1)
		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{float32(i)}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 100 // Large enough to not trigger size-based flush
	cfg.MaxWaitTime = 10 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	// Send 3 requests sequentially, waiting longer than MaxWaitTime between each
	for i := 0; i < 3; i++ {
		res, err := batcher.Embed(context.Background(), fmt.Sprintf("text-%d", i))
		require.NoError(t, err)
		require.NotNil(t, res)
		time.Sleep(20 * time.Millisecond) // Wait longer than flush interval
	}

	// Should have made 3 separate batch calls due to time-based flushing
	assert.Equal(t, int64(3), atomic.LoadInt64(&callCount), "each request sent after wait interval should flush separately")
}

func TestEmbeddingBatcher_QueueFullDegradation(t *testing.T) {
	// Block the batch function so queue fills up
	blockBatch := make(chan struct{})
	var singleCallCount int64

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		<-blockBatch // Block until test releases
		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{1.0}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 10
	cfg.MaxPendingRequests = 2 // Very small queue for testing
	cfg.MaxWaitTime = 1 * time.Hour // Effectively disable time flush
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer func() {
		close(blockBatch)
		batcher.Shutdown()
	}()

	// Fill up the queue first (capacity 2)
	go func() {
		_, _ = batcher.Embed(context.Background(), "text-1")
	}()
	go func() {
		_, _ = batcher.Embed(context.Background(), "text-2")
	}()

	time.Sleep(50 * time.Millisecond) // Wait for queue to fill

	// Now send another request - should degrade to single call immediately
	// But since our batchFn blocks, we need to handle this - actually the batcher
	// falls back to direct single call, not queued, so we need to count those
	// For this test we just verify that Embed doesn't block indefinitely
	done := make(chan struct{})
	go func() {
		defer close(done)
		// After queue is full, this should either:
		// 1. Fail with error (if queue full and batchFn blocks)
		// 2. Succeed (if we implement non-blocking correctly)
		_, _ = batcher.Embed(context.Background(), "text-3-degraded")
		atomic.AddInt64(&singleCallCount, 1)
	}()

	select {
	case <-done:
		// Good - didn't block indefinitely
		t.Log("Degradation path works without blocking")
	case <-time.After(1 * time.Second):
		t.Fatal("Embed blocked when queue was full - degradation path not working")
	}
}

func TestEmbeddingBatcher_ContextCancellation(t *testing.T) {
	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		// Slow batch function
		time.Sleep(100 * time.Millisecond)
		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{1.0}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 10
	cfg.MaxWaitTime = 50 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := batcher.Embed(ctx, "test-text")
	assert.ErrorIs(t, err, context.Canceled, "should return context.Canceled error")
}

func TestEmbeddingBatcher_ErrorPropagation(t *testing.T) {
	expectedErr := fmt.Errorf("batch API error")

	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		return nil, expectedErr
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 2
	cfg.MaxWaitTime = 5 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	_, err := batcher.Embed(context.Background(), "test-text")
	assert.ErrorIs(t, err, expectedErr, "batch function error should propagate to caller")
}

func TestEmbeddingBatcher_Stats(t *testing.T) {
	var callCount int64
	batchFn := func(ctx context.Context, texts []string) ([][]float32, error) {
		atomic.AddInt64(&callCount, 1)
		results := make([][]float32, len(texts))
		for i := range texts {
			results[i] = []float32{1.0}
		}
		return results, nil
	}

	cfg := DefaultEmbeddingBatcherConfig()
	cfg.MaxBatchSize = 3
	cfg.MaxWaitTime = 5 * time.Millisecond
	batcher := NewEmbeddingBatcher("test-model", cfg, batchFn)
	defer batcher.Shutdown()

	// Send 5 requests
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(i int) {
			defer wg.Done()
			_, err := batcher.Embed(context.Background(), fmt.Sprintf("text-%d", i))
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	stats := batcher.Stats()
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.Equal(t, int64(5), stats.BatchedRequests)
	assert.GreaterOrEqual(t, stats.BatchCount, int64(2)) // 3 + 2
	assert.Greater(t, stats.AvgBatchSize, 0.0)
	assert.Greater(t, stats.BatchRate, 0.0)

	t.Logf("Batcher Stats: %+v", stats)
}

func TestEmbeddingBatcherManager_GetOrCreate(t *testing.T) {
	mgr := NewEmbeddingBatcherManager(DefaultEmbeddingBatcherConfig())
	defer mgr.Shutdown()

	// We'll just test the manager's batcher caching logic here
	b1 := mgr.GetOrCreateBatcher("model-1", &mockEmbedder{})
	b2 := mgr.GetOrCreateBatcher("model-1", &mockEmbedder{}) // Should return same as b1
	b3 := mgr.GetOrCreateBatcher("model-2", &mockEmbedder{})

	assert.Same(t, b1, b2, "same model key should return same batcher instance")
	assert.NotSame(t, b1, b3, "different model keys should return different batcher instances")

	stats := mgr.Stats()
	assert.Len(t, stats, 2)
	_, hasModel1 := stats["model-1"]
	_, hasModel2 := stats["model-2"]
	assert.True(t, hasModel1)
	assert.True(t, hasModel2)
}

// mockEmbedder is a simple mock for testing the manager
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0}, nil
}

func (m *mockEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = []float32{1.0}
	}
	return results, nil
}

func (m *mockEmbedder) GetModelName() string  { return "mock-model" }
func (m *mockEmbedder) GetDimensions() int   { return 1 }
func (m *mockEmbedder) GetModelID() string   { return "mock-model-id" }
func (m *mockEmbedder) BatchEmbedWithPool(ctx context.Context, model embedding.Embedder, texts []string) ([][]float32, error) {
	return m.BatchEmbed(ctx, texts)
}
