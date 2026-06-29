package service

import (
	"context"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// RouterWrapper 透明包装VectorStoreRouter为RetrieveEngineService接口
// 零侵入：现有代码无需任何修改即可享受读写分离能力
type RouterWrapper struct {
	router  *VectorStoreRouter
	storeID string
}

// WrapRouterAsEngineService 将路由器包装为RetrieveEngineService，保证完全向后兼容
func WrapRouterAsEngineService(router *VectorStoreRouter, storeID string) interfaces.RetrieveEngineService {
	return &RouterWrapper{
		router:  router,
		storeID: storeID,
	}
}

// NewRouterWrapper 创建路由器包装器，WrapRouterAsEngineService的别名（用于DI容器）
func NewRouterWrapper(router *VectorStoreRouter, storeID string) *RouterWrapper {
	return &RouterWrapper{
		router:  router,
		storeID: storeID,
	}
}

func (w *RouterWrapper) EngineType() types.RetrieverEngineType {
	writer, err := w.router.GetWriter(context.Background(), w.storeID)
	if err != nil {
		return types.RetrieverEngineType("unknown")
	}
	return writer.EngineType()
}

func (w *RouterWrapper) Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error) {
	// 使用params中的StoreID优先，否则使用wrapper的默认storeID
	storeID := params.StoreID
	if storeID == "" {
		storeID = w.storeID
	}

	reader, err := w.router.GetReader(ctx, storeID, params.ConsistencyLevel, params.WriteToken)
	if err != nil {
		return nil, err
	}
	return reader.Retrieve(ctx, params)
}

func (w *RouterWrapper) Support() []types.RetrieverType {
	writer, err := w.router.GetWriter(context.Background(), w.storeID)
	if err != nil {
		return nil
	}
	return writer.Support()
}

func (w *RouterWrapper) Index(ctx context.Context,
	embedder embedding.Embedder,
	indexInfo *types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.Index(ctx, embedder, indexInfo, retrieverTypes)
}

func (w *RouterWrapper) BatchIndex(ctx context.Context,
	embedder embedding.Embedder,
	indexInfoList []*types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchIndex(ctx, embedder, indexInfoList, retrieverTypes)
}

func (w *RouterWrapper) CopyIndices(
	ctx context.Context,
	sourceKnowledgeBaseID string,
	sourceToTargetKBIDMap map[string]string,
	sourceToTargetChunkIDMap map[string]string,
	targetKnowledgeBaseID string,
	dimension int,
	knowledgeType string,
) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.CopyIndices(ctx, sourceKnowledgeBaseID, sourceToTargetKBIDMap,
		sourceToTargetChunkIDMap, targetKnowledgeBaseID, dimension, knowledgeType)
}

func (w *RouterWrapper) DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteByChunkIDList(ctx, indexIDList, dimension, knowledgeType)
}

func (w *RouterWrapper) DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteBySourceIDList(ctx, sourceIDList, dimension, knowledgeType)
}

func (w *RouterWrapper) DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteByKnowledgeIDList(ctx, knowledgeIDList, dimension, knowledgeType)
}

func (w *RouterWrapper) BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchUpdateChunkEnabledStatus(ctx, chunkStatusMap)
}

func (w *RouterWrapper) BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchUpdateChunkTagID(ctx, chunkTagMap)
}

func (w *RouterWrapper) EstimateStorageSize(ctx context.Context,
	embedder embedding.Embedder,
	indexInfoList []*types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) int64 {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return 0
	}
	return writer.EstimateStorageSize(ctx, embedder, indexInfoList, retrieverTypes)
}

// Compile-time interface check
var _ interfaces.RetrieveEngineService = (*RouterWrapper)(nil)
