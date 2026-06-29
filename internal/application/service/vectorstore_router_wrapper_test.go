package service

import (
	"context"
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouterWrapper_WrapAsEngineService(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	assert.NotNil(t, wrapper)
}

func TestRouterWrapper_EngineType_DelegatesToMaster(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	assert.Equal(t, master.EngineType(), wrapper.EngineType())
}

func TestRouterWrapper_Support_DelegatesToMaster(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	assert.Equal(t, master.Support(), wrapper.Support())
}

func TestRouterWrapper_Index_RoutesToWriter(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.Index(context.Background(), nil, nil, nil)
	require.NoError(t, err)

	// 写操作应生成WriteToken
	assert.NotNil(t, master.lastToken)
}

func TestRouterWrapper_Retrieve_RoutesToReader(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	master.searchResult = []*types.RetrieveResult{
		{Results: []*types.IndexWithScore{{KnowledgeID: "test-kb", ChunkID: "chunk-1", Score: 0.95}}},
	}

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	results, err := wrapper.Retrieve(context.Background(), types.RetrieveParams{
		Query:     "test query",
		TopK:      5,
		Threshold: 0.5,
		StoreID:   "store-1",
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Len(t, results[0].Results, 1)
	assert.Equal(t, "test-kb", results[0].Results[0].KnowledgeID)
}

func TestRouterWrapper_Retrieve_UsesDefaultStoreID(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()
	master.searchResult = []*types.RetrieveResult{
		{Results: []*types.IndexWithScore{{KnowledgeID: "test-kb", ChunkID: "chunk-1", Score: 0.95}}},
	}

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	// 不在params中指定StoreID，应使用wrapper的默认storeID
	wrapper := WrapRouterAsEngineService(router, "store-1")
	results, err := wrapper.Retrieve(context.Background(), types.RetrieveParams{
		Query:     "test query",
		TopK:      5,
		Threshold: 0.5,
		// StoreID 为空，应使用 wrapper.storeID
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestRouterWrapper_DeleteByChunkIDList(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.DeleteByChunkIDList(context.Background(), []string{"chunk-1"}, 128, "test")
	require.NoError(t, err)
}

func TestRouterWrapper_DeleteBySourceIDList(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.DeleteBySourceIDList(context.Background(), []string{"src-1"}, 128, "test")
	require.NoError(t, err)
}

func TestRouterWrapper_DeleteByKnowledgeIDList(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.DeleteByKnowledgeIDList(context.Background(), []string{"kb-1"}, 128, "test")
	require.NoError(t, err)
}

func TestRouterWrapper_BatchUpdateChunkEnabledStatus(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.BatchUpdateChunkEnabledStatus(context.Background(), map[string]bool{"chunk-1": true})
	require.NoError(t, err)
}

func TestRouterWrapper_BatchUpdateChunkTagID(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.BatchUpdateChunkTagID(context.Background(), map[string]string{"chunk-1": "tag-1"})
	require.NoError(t, err)
}

func TestRouterWrapper_EstimateStorageSize(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	size := wrapper.EstimateStorageSize(context.Background(), nil, nil, nil)
	assert.Equal(t, int64(1024), size)
}

func TestRouterWrapper_StoreNotFound(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	router := NewVectorStoreRouter(cfg)

	wrapper := WrapRouterAsEngineService(router, "nonexistent")
	_, err := wrapper.Retrieve(context.Background(), types.RetrieveParams{})
	assert.Error(t, err)
}

func TestRouterWrapper_BatchIndex(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.BatchIndex(context.Background(), nil, nil, nil)
	require.NoError(t, err)
}

func TestRouterWrapper_CopyIndices(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	cfg.Enabled = false
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := WrapRouterAsEngineService(router, "store-1")
	err = wrapper.CopyIndices(context.Background(), "src-kb",
		map[string]string{"a": "b"},
		map[string]string{"c": "d"},
		"tgt-kb", 128, "test")
	require.NoError(t, err)
}

func TestNewRouterWrapper(t *testing.T) {
	cfg := types.DefaultReadWriteSeparationConfig()
	router := NewVectorStoreRouter(cfg)
	master := newMockEngine()

	err := router.RegisterEngine("store-1", master, nil)
	require.NoError(t, err)

	wrapper := NewRouterWrapper(router, "store-1")
	assert.NotNil(t, wrapper)
	assert.Equal(t, "store-1", wrapper.storeID)
	assert.NotNil(t, wrapper.router)
}
