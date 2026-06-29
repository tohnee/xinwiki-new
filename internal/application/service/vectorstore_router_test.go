package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEngine 用于测试的模拟引擎
type mockEngine struct {
	healthLatency  time.Duration
	healthy        bool
	lsn            int64
	indexErr       error
	searchResult   []*types.RetrieveResult
	lastToken      *types.WriteToken
	healthCheckErr error
}

func newMockEngine() *mockEngine {
	return &mockEngine{
		healthLatency: 10 * time.Millisecond,
		healthy:       true,
		lsn:           100,
	}
}

func (m *mockEngine) EngineType() types.RetrieverEngineType {
	return types.OpenSearchRetrieverEngineType
}

func (m *mockEngine) Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error) {
	return m.searchResult, nil
}

func (m *mockEngine) Support() []types.RetrieverType {
	return []types.RetrieverType{types.VectorRetrieverType, types.KeywordsRetrieverType}
}

func (m *mockEngine) HealthCheck(ctx context.Context) (*types.NodeHealth, error) {
	if m.healthCheckErr != nil {
		return nil, m.healthCheckErr
	}
	select {
	case <-time.After(m.healthLatency):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &types.NodeHealth{
		NodeID:    "mock",
		Healthy:   m.healthy,
		LatencyMs: m.healthLatency.Milliseconds(),
		LSN:       m.lsn,
	}, nil
}

func (m *mockEngine) makeToken() *types.WriteToken {
	m.lsn++
	token := &types.WriteToken{
		StoreID:   "test-store",
		LSN:       m.lsn,
		Timestamp: time.Now().UnixMilli(),
	}
	m.lastToken = token
	return token
}

func (m *mockEngine) Index(ctx context.Context, embedder embedding.Embedder, indexInfo *types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	if m.indexErr != nil {
		return m.indexErr
	}
	m.makeToken()
	return nil
}

func (m *mockEngine) BatchIndex(ctx context.Context, embedder embedding.Embedder, indexInfoList []*types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	if m.indexErr != nil {
		return m.indexErr
	}
	m.makeToken()
	return nil
}

func (m *mockEngine) CopyIndices(ctx context.Context, sourceKnowledgeBaseID string, sourceToTargetKBIDMap map[string]string, sourceToTargetChunkIDMap map[string]string, targetKnowledgeBaseID string, dimension int, knowledgeType string) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error {
	m.makeToken()
	return nil
}

func (m *mockEngine) Flush(ctx context.Context) error {
	return nil
}

func (m *mockEngine) GetCurrentLSN(ctx context.Context) (int64, error) {
	return m.lsn, nil
}

func (m *mockEngine) WaitForLSN(ctx context.Context, lsn int64, timeout time.Duration) error {
	if m.lsn >= lsn {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.lsn >= lsn {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

func (m *mockEngine) EstimateStorageSize(ctx context.Context, embedder embedding.Embedder, indexInfoList []*types.IndexInfo, retrieverTypes []types.RetrieverType) int64 {
	return 1024
}

func (m *mockEngine) LastWriteToken() *types.WriteToken {
	return m.lastToken
}

func TestNewVectorStoreRouter(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	router := NewVectorStoreRouter(cfg)
	assert.NotNil(t, router)
	assert.NotNil(t, router.GetRouterStats())
}

func TestVectorStoreRouter_RegisterEngine_SingleNode(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	// 禁用读写分离时，读写都走同一个节点
	writer, err := router.GetWriter(context.Background(), "store-1")
	require.NoError(t, err)
	assert.NotNil(t, writer)

	reader, err := router.GetReader(context.Background(), "store-1", types.ConsistencyLevelEventual, nil)
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestVectorStoreRouter_RegisterEngine_ReadWriteSeparation(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	cfg.HealthCheckInterval = 100 * time.Millisecond
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	replica1 := newMockEngine()
	replica2 := newMockEngine()
	replicas := []interfaces.ReadableNode{replica1, replica2}

	err := router.RegisterEngine("store-1", master, replicas)
	require.NoError(t, err)

	// 等待健康检查完成
	time.Sleep(200 * time.Millisecond)

	writer, err := router.GetWriter(context.Background(), "store-1")
	require.NoError(t, err)
	assert.NotNil(t, writer)

	// 读请求应该路由到副本
	reader, err := router.GetReader(context.Background(), "store-1", types.ConsistencyLevelEventual, nil)
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestVectorStoreRouter_StrongConsistency_ReadMaster(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	replica := newMockEngine()
	replica.lsn = 50 // 副本落后于主节点

	err := router.RegisterEngine("store-1", master, []interfaces.ReadableNode{replica})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// 强一致性读应该直接走主节点
	reader, err := router.GetReader(context.Background(), "store-1", types.ConsistencyLevelStrong, nil)
	require.NoError(t, err)
	assert.NotNil(t, reader)
}

func TestVectorStoreRouter_UnregisterEngine(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	router.UnregisterEngine("store-1")

	_, err = router.GetWriter(context.Background(), "store-1")
	assert.Error(t, err)
}

func TestVectorStoreRouter_UpdateReplicas(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	replica1 := newMockEngine()

	err := router.RegisterEngine("store-1", master, []interfaces.ReadableNode{replica1})
	require.NoError(t, err)

	replica2 := newMockEngine()
	err = router.UpdateReplicas("store-1", []interfaces.ReadableNode{replica1, replica2})
	require.NoError(t, err)

	stats := router.GetRouterStats()
	assert.Equal(t, 2, stats.TotalReplicas)
}

func TestVectorStoreRouter_Shutdown(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	cfg.HealthCheckInterval = 50 * time.Millisecond
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = router.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVectorStoreRouter_MasterCircuitBreaker_TriggersOnConsecutiveFails(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	cfg.HealthCheckInterval = 1 * time.Hour 
	cfg.CircuitBreakerThreshold = 3
	cfg.HealthCheckTimeout = 1 * time.Second
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	replica := newMockEngine()

	err := router.RegisterEngine("test-circuit", master, []interfaces.ReadableNode{replica})
	require.NoError(t, err)

	router.mu.RLock()
	entry := router.stores["test-circuit"]
	router.mu.RUnlock()
	require.NotNil(t, entry)

	entry.mu.RLock()
	masterState := entry.healthStates["master"]
	entry.mu.RUnlock()
	assert.Equal(t, types.CircuitBreakerClosed, masterState.circuitState)
	assert.Equal(t, 0, masterState.consecutiveFails)

	master.healthCheckErr = errors.New("connection refused")
	master.healthy = false

	for i := 0; i < cfg.CircuitBreakerThreshold; i++ {
		router.checkAllNodesHealth(entry)
	}

	entry.mu.RLock()
	masterState = entry.healthStates["master"]
	replicaState := entry.healthStates["replica-0"]
	entry.mu.RUnlock()

	assert.Equal(t, types.CircuitBreakerOpen, masterState.circuitState, "master circuit breaker should be OPEN after %d consecutive failures", cfg.CircuitBreakerThreshold)
	assert.Equal(t, cfg.CircuitBreakerThreshold, masterState.consecutiveFails)
	assert.Equal(t, types.CircuitBreakerClosed, replicaState.circuitState, "replica should remain healthy")
}

func TestVectorStoreRouter_MasterCircuitBreaker_ResetsOnRecovery(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	cfg.HealthCheckInterval = 1 * time.Hour
	cfg.CircuitBreakerThreshold = 3
	cfg.HealthCheckTimeout = 1 * time.Second
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("test-recovery", master, nil)
	require.NoError(t, err)

	router.mu.RLock()
	entry := router.stores["test-recovery"]
	router.mu.RUnlock()
	require.NotNil(t, entry)

	master.healthCheckErr = errors.New("connection refused")
	master.healthy = false

	for i := 0; i < cfg.CircuitBreakerThreshold; i++ {
		router.checkAllNodesHealth(entry)
	}

	entry.mu.RLock()
	assert.Equal(t, types.CircuitBreakerOpen, entry.healthStates["master"].circuitState)
	entry.mu.RUnlock()

	master.healthCheckErr = nil
	master.healthy = true

	router.checkAllNodesHealth(entry)

	entry.mu.RLock()
	masterState := entry.healthStates["master"]
	entry.mu.RUnlock()

	assert.Equal(t, types.CircuitBreakerClosed, masterState.circuitState, "circuit breaker should CLOSE after master recovers")
	assert.Equal(t, 0, masterState.consecutiveFails, "consecutive failures should reset to 0")
}

func TestVectorStoreRouter_ReplicaCircuitBreaker_FallbackToMaster(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = true
	cfg.HealthCheckInterval = 1 * time.Hour
	cfg.CircuitBreakerThreshold = 2
	cfg.HealthCheckTimeout = 1 * time.Second
	cfg.MaxReplicationLag = 1000
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	replica := newMockEngine()

	err := router.RegisterEngine("test-replica-fail", master, []interfaces.ReadableNode{replica})
	require.NoError(t, err)

	router.mu.RLock()
	entry := router.stores["test-replica-fail"]
	router.mu.RUnlock()
	require.NotNil(t, entry)

	router.checkAllNodesHealth(entry)

	entry.mu.RLock()
	initMasterState := entry.healthStates["master"]
	initReplicaState := entry.healthStates["replica-0"]
	entry.mu.RUnlock()
	assert.Equal(t, types.CircuitBreakerClosed, initMasterState.circuitState)
	assert.Equal(t, types.CircuitBreakerClosed, initReplicaState.circuitState)

	replica.healthCheckErr = errors.New("replica timeout")
	replica.healthy = false

	for i := 0; i < cfg.CircuitBreakerThreshold; i++ {
		router.checkAllNodesHealth(entry)
	}

	entry.mu.RLock()
	replicaState := entry.healthStates["replica-0"]
	entry.mu.RUnlock()
	assert.Equal(t, types.CircuitBreakerOpen, replicaState.circuitState, "replica circuit breaker should be OPEN")

	reader, err := router.GetReader(context.Background(), "test-replica-fail", types.ConsistencyLevelEventual, nil)
	require.NoError(t, err)
	assert.NotNil(t, reader)
}
