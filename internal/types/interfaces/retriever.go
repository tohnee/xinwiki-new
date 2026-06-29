package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
)

// RetrieveEngine defines the retrieve engine interface (read-only operations)
type RetrieveEngine interface {
	// EngineType gets the retrieve engine type
	EngineType() types.RetrieverEngineType

	// Retrieve executes the retrieve
	Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error)

	// Support gets the supported retrieve types
	Support() []types.RetrieverType
}

// ReadableNode 可读节点扩展接口，读副本需要实现
type ReadableNode interface {
	RetrieveEngine
	// HealthCheck returns node health status including latency and current LSN
	HealthCheck(ctx context.Context) (*types.NodeHealth, error)
	// WaitForLSN blocks until the node has applied all writes up to the specified LSN
	WaitForLSN(ctx context.Context, lsn int64, timeout time.Duration) error
}

// WritableNode 可写节点扩展接口，主节点需要实现
type WritableNode interface {
	// Flush waits for all pending writes to be persisted to storage
	Flush(ctx context.Context) error
	// GetCurrentLSN returns the current log sequence number for consistency tracking
	GetCurrentLSN(ctx context.Context) (int64, error)
}

// RetrieveEngineRepository defines the retrieve engine repository interface
type RetrieveEngineRepository interface {
	// Save saves the index info
	Save(ctx context.Context, indexInfo *types.IndexInfo, params map[string]any) error

	// BatchSave saves the index info list
	BatchSave(ctx context.Context, indexInfoList []*types.IndexInfo, params map[string]any) error

	// EstimateStorageSize estimates the storage size
	EstimateStorageSize(ctx context.Context, indexInfoList []*types.IndexInfo, params map[string]any) int64

	// DeleteByChunkIDList deletes the index info by chunk id list
	DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error
	// DeleteBySourceIDList deletes the index info by source id list
	DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error
	// 复制索引数据
	// sourceKnowledgeBaseID: 源知识库ID
	// sourceToTargetChunkIDMap: 源分块ID到目标分块ID的映射关系
	// targetKnowledgeBaseID: 目标知识库ID
	// params: 额外参数，如向量表示等
	CopyIndices(
		ctx context.Context,
		sourceKnowledgeBaseID string,
		sourceToTargetKBIDMap map[string]string,
		sourceToTargetChunkIDMap map[string]string,
		targetKnowledgeBaseID string,
		dimension int,
		knowledgeType string,
	) error

	// DeleteByKnowledgeIDList deletes the index info by knowledge id list
	DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error

	// BatchUpdateChunkEnabledStatus updates the enabled status of chunks in batch
	// chunkStatusMap: map of chunk ID to enabled status (true = enabled, false = disabled)
	BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error

	// BatchUpdateChunkTagID updates the tag ID of chunks in batch
	// chunkTagMap: map of chunk ID to tag ID (empty string means no tag)
	BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error

	// RetrieveEngine retrieves the engine
	RetrieveEngine
}

// RetrieveEngineRegistry defines the retrieve engine registry interface
type RetrieveEngineRegistry interface {
	// Register registers the retrieve engine service
	Register(indexService RetrieveEngineService) error
	// GetRetrieveEngineService gets the retrieve engine service
	GetRetrieveEngineService(engineType types.RetrieverEngineType) (RetrieveEngineService, error)
	// GetAllRetrieveEngineServices gets all retrieve engine services
	GetAllRetrieveEngineServices() []RetrieveEngineService

	// GetByStoreID returns the engine service registered for a specific DB store ID.
	//
	// IMPORTANT: This method does NOT verify tenant ownership of the returned
	// store. Callers MUST use the CreateRetrieveEngineForKB /
	// CreateRetrieveEngineFromPayload factory functions in the retriever package
	// rather than calling this directly. The factories wrap GetByStoreID with
	// tenant ownership verification (defense-in-depth against cross-tenant IDOR).
	GetByStoreID(storeID string) (RetrieveEngineService, error)
}

// RetrieveEngineService defines the full retrieve engine service interface
// KEEP BACKWARD COMPATIBLE: 原有接口签名100%不变，所有写方法仍然返回error
type RetrieveEngineService interface {
	RetrieveEngine

	// Index indexes the index info
	Index(ctx context.Context,
		embedder embedding.Embedder,
		indexInfo *types.IndexInfo,
		retrieverTypes []types.RetrieverType,
	) error

	// BatchIndex indexes the index info list
	BatchIndex(ctx context.Context,
		embedder embedding.Embedder,
		indexInfoList []*types.IndexInfo,
		retrieverTypes []types.RetrieverType,
	) error

	// CopyIndices 从源知识库复制索引到目标知识库，免去重新计算嵌入向量的开销
	// sourceKnowledgeBaseID: 源知识库ID
	// sourceToTargetChunkIDMap: 源分块ID到目标分块ID的映射关系，key为源分块ID，value为目标分块ID
	// targetKnowledgeBaseID: 目标知识库ID
	CopyIndices(
		ctx context.Context,
		sourceKnowledgeBaseID string,
		sourceToTargetKBIDMap map[string]string,
		sourceToTargetChunkIDMap map[string]string,
		targetKnowledgeBaseID string,
		dimension int,
		knowledgeType string,
	) error

	// DeleteByChunkIDList deletes the index info by chunk id list
	DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error

	// DeleteBySourceIDList deletes the index info by source id list
	DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error

	// DeleteByKnowledgeIDList deletes the index info by knowledge id list
	DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error

	// BatchUpdateChunkEnabledStatus updates the enabled status of chunks in batch
	// chunkStatusMap: map of chunk ID to enabled status (true = enabled, false = disabled)
	BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error

	// BatchUpdateChunkTagID updates the tag ID of chunks in batch
	// chunkTagMap: map of chunk ID to tag ID (empty string means no tag)
	BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error

	// EstimateStorageSize estimates the storage size
	EstimateStorageSize(ctx context.Context,
		embedder embedding.Embedder,
		indexInfoList []*types.IndexInfo,
		retrieverTypes []types.RetrieverType,
	) int64
}

// RWCapableEngine 支持读写分离的引擎接口，在原有接口基础上扩展读写分离能力
type RWCapableEngine interface {
	RetrieveEngineService
	ReadableNode
	WritableNode
	// 获取最近一次写入的token
	LastWriteToken() *types.WriteToken
}
