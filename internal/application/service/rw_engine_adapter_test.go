package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapEngineWithRWCapabilities_InitialState(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	assert.NotNil(t, rwEngine)

	// 初始LSN应为0
	lsn, err := rwEngine.GetCurrentLSN(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), lsn)

	// 初始无WriteToken
	assert.Nil(t, rwEngine.LastWriteToken())
}

func TestRWEngineAdapter_Index_GeneratesWriteToken(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	// 执行Index前没有token
	assert.Nil(t, rwEngine.LastWriteToken())

	// 执行Index操作
	err := rwEngine.Index(context.Background(), nil, nil, nil)
	require.NoError(t, err)

	// Index后应生成WriteToken
	token := rwEngine.LastWriteToken()
	require.NotNil(t, token)
	assert.Equal(t, "test-store", token.StoreID)
	assert.Equal(t, int64(1), token.LSN)

	// LSN应递增
	lsn, _ := rwEngine.GetCurrentLSN(context.Background())
	assert.Equal(t, int64(1), lsn)
}

func TestRWEngineAdapter_BatchIndex_IncrementsLSN(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	err := rwEngine.BatchIndex(context.Background(), nil, nil, nil)
	require.NoError(t, err)

	token1 := rwEngine.LastWriteToken()
	require.NotNil(t, token1)
	assert.Equal(t, int64(1), token1.LSN)

	// 再次写入，LSN应递增
	err = rwEngine.BatchIndex(context.Background(), nil, nil, nil)
	require.NoError(t, err)

	token2 := rwEngine.LastWriteToken()
	require.NotNil(t, token2)
	assert.Equal(t, int64(2), token2.LSN)
}

func TestRWEngineAdapter_DeleteOperations_GenerateTokens(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	// DeleteByChunkIDList
	err := rwEngine.DeleteByChunkIDList(context.Background(), []string{"chunk-1"}, 128, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rwEngine.LastWriteToken().LSN)

	// DeleteBySourceIDList
	err = rwEngine.DeleteBySourceIDList(context.Background(), []string{"src-1"}, 128, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(2), rwEngine.LastWriteToken().LSN)

	// DeleteByKnowledgeIDList
	err = rwEngine.DeleteByKnowledgeIDList(context.Background(), []string{"kb-1"}, 128, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(3), rwEngine.LastWriteToken().LSN)
}

func TestRWEngineAdapter_BatchUpdateOperations_GenerateTokens(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	// BatchUpdateChunkEnabledStatus
	err := rwEngine.BatchUpdateChunkEnabledStatus(context.Background(), map[string]bool{"chunk-1": true})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rwEngine.LastWriteToken().LSN)

	// BatchUpdateChunkTagID
	err = rwEngine.BatchUpdateChunkTagID(context.Background(), map[string]string{"chunk-1": "tag-1"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), rwEngine.LastWriteToken().LSN)
}

func TestRWEngineAdapter_CopyIndices_GeneratesToken(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	err := rwEngine.CopyIndices(context.Background(), "src-kb", map[string]string{"a": "b"}, map[string]string{"c": "d"}, "tgt-kb", 128, "test")
	require.NoError(t, err)

	token := rwEngine.LastWriteToken()
	require.NotNil(t, token)
	assert.Equal(t, int64(1), token.LSN)
}

func TestRWEngineAdapter_HealthCheck_ReturnsMasterInfo(t *testing.T) {
	master := newMockEngine()
	master.lsn = 42
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	health, err := rwEngine.HealthCheck(context.Background())
	require.NoError(t, err)
	require.NotNil(t, health)

	assert.Equal(t, "test-store", health.NodeID)
	assert.True(t, health.IsMaster)
	assert.True(t, health.Healthy)
	assert.Equal(t, int64(0), health.LSN) // adapter的LSN独立于mockEngine的lsn
}

func TestRWEngineAdapter_WaitForLSN_AlwaysSucceeds(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	// 单节点模式WaitForLSN应永远成功
	err := rwEngine.WaitForLSN(context.Background(), 999, 0)
	assert.NoError(t, err)
}

func TestRWEngineAdapter_Flush_NoOp(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	err := rwEngine.Flush(context.Background())
	assert.NoError(t, err)
}

func TestRWEngineAdapter_DelegatesEngineType(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	assert.Equal(t, master.EngineType(), rwEngine.EngineType())
}

func TestRWEngineAdapter_DelegatesSupport(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	assert.Equal(t, master.Support(), rwEngine.Support())
}

func TestRWEngineAdapter_EstimateStorageSize_Delegates(t *testing.T) {
	master := newMockEngine()
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	size := rwEngine.EstimateStorageSize(context.Background(), nil, nil, nil)
	assert.Equal(t, int64(1024), size)
}

func TestRWEngineAdapter_Index_PropagatesError(t *testing.T) {
	master := newMockEngine()
	master.indexErr = assert.AnError
	rwEngine := WrapEngineWithRWCapabilities("test-store", master)

	err := rwEngine.Index(context.Background(), nil, nil, nil)
	assert.Error(t, err)

	// 失败时不应生成WriteToken
	assert.Nil(t, rwEngine.LastWriteToken())
}
