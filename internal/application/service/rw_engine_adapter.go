package service

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// rwEngineAdapter 包装现有引擎实现，自动扩展读写分离所需能力
// 零侵入：不需要修改原有任何引擎实现代码
type rwEngineAdapter struct {
	legacy  interfaces.RetrieveEngineService
	lsn     atomic.Int64
	storeID string
	lastToken atomic.Pointer[types.WriteToken]
}

// WrapEngineWithRWCapabilities 将现有引擎包装为支持读写分离扩展能力的RWCapableEngine
func WrapEngineWithRWCapabilities(storeID string, engine interfaces.RetrieveEngineService) interfaces.RWCapableEngine {
	a := &rwEngineAdapter{
		legacy:  engine,
		storeID: storeID,
	}
	a.lsn.Store(0)
	return a
}

func (a *rwEngineAdapter) EngineType() types.RetrieverEngineType {
	return a.legacy.EngineType()
}

func (a *rwEngineAdapter) Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error) {
	return a.legacy.Retrieve(ctx, params)
}

func (a *rwEngineAdapter) Support() []types.RetrieverType {
	return a.legacy.Support()
}

func (a *rwEngineAdapter) HealthCheck(ctx context.Context) (*types.NodeHealth, error) {
	// 如果底层引擎支持HealthCheck，委托给它以反映真实健康状态
	if hc, ok := a.legacy.(interface{ HealthCheck(context.Context) (*types.NodeHealth, error) }); ok {
		health, err := hc.HealthCheck(ctx)
		if err != nil {
			return nil, err
		}
		if health != nil {
			health.NodeID = a.storeID
			health.IsMaster = true
			health.LSN = a.lsn.Load()
			return health, nil
		}
	}
	return &types.NodeHealth{
		NodeID:         a.storeID,
		Endpoint:       a.storeID,
		IsMaster:       true,
		Healthy:        true,
		LatencyMs:      0,
		LSN:            a.lsn.Load(),
		ReplicationLag: 0,
		Connections:    1,
		LastChecked:    time.Now(),
	}, nil
}

func (a *rwEngineAdapter) WaitForLSN(ctx context.Context, lsn int64, timeout time.Duration) error {
	// 单节点模式永远满足LSN要求，直接返回
	return nil
}

func (a *rwEngineAdapter) Flush(ctx context.Context) error {
	// 旧引擎默认不需要Flush，直接返回
	return nil
}

func (a *rwEngineAdapter) GetCurrentLSN(ctx context.Context) (int64, error) {
	return a.lsn.Load(), nil
}

func (a *rwEngineAdapter) LastWriteToken() *types.WriteToken {
	t := a.lastToken.Load()
	return t
}

func (a *rwEngineAdapter) nextLSN() int64 {
	return a.lsn.Add(1)
}

func (a *rwEngineAdapter) makeWriteToken() *types.WriteToken {
	lsn := a.nextLSN()
	token := &types.WriteToken{
		StoreID:   a.storeID,
		LSN:       lsn,
		Timestamp: time.Now().UnixMilli(),
	}
	a.lastToken.Store(token)
	return token
}

func (a *rwEngineAdapter) Index(ctx context.Context,
	embedder embedding.Embedder,
	indexInfo *types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) error {
	if err := a.legacy.Index(ctx, embedder, indexInfo, retrieverTypes); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) BatchIndex(ctx context.Context,
	embedder embedding.Embedder,
	indexInfoList []*types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) error {
	if err := a.legacy.BatchIndex(ctx, embedder, indexInfoList, retrieverTypes); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) CopyIndices(
	ctx context.Context,
	sourceKnowledgeBaseID string,
	sourceToTargetKBIDMap map[string]string,
	sourceToTargetChunkIDMap map[string]string,
	targetKnowledgeBaseID string,
	dimension int,
	knowledgeType string,
) error {
	if err := a.legacy.CopyIndices(ctx, sourceKnowledgeBaseID, sourceToTargetKBIDMap,
		sourceToTargetChunkIDMap, targetKnowledgeBaseID, dimension, knowledgeType); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error {
	if err := a.legacy.DeleteByChunkIDList(ctx, indexIDList, dimension, knowledgeType); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error {
	if err := a.legacy.DeleteBySourceIDList(ctx, sourceIDList, dimension, knowledgeType); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error {
	if err := a.legacy.DeleteByKnowledgeIDList(ctx, knowledgeIDList, dimension, knowledgeType); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error {
	if err := a.legacy.BatchUpdateChunkEnabledStatus(ctx, chunkStatusMap); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error {
	if err := a.legacy.BatchUpdateChunkTagID(ctx, chunkTagMap); err != nil {
		return err
	}
	a.makeWriteToken()
	return nil
}

func (a *rwEngineAdapter) EstimateStorageSize(ctx context.Context,
	embedder embedding.Embedder,
	indexInfoList []*types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) int64 {
	return a.legacy.EstimateStorageSize(ctx, embedder, indexInfoList, retrieverTypes)
}

// Compile-time interface check
var _ interfaces.RWCapableEngine = (*rwEngineAdapter)(nil)
