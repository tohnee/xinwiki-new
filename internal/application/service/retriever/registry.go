package retriever

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// Router 定义读写分离路由器接口，打破 service ↔ retriever 导入循环
type Router interface {
	RegisterEngine(storeID string, master interfaces.RWCapableEngine, replicas []interfaces.ReadableNode) error
	UnregisterEngine(storeID string)
	GetReader(ctx context.Context, storeID string, consistency types.ConsistencyLevel, token *types.WriteToken) (interfaces.ReadableNode, error)
	GetWriter(ctx context.Context, storeID string) (interfaces.RetrieveEngineService, error)
}

// EngineWrapper 将旧引擎包装为 RWCapableEngine 的函数类型
type EngineWrapper func(storeID string, engine interfaces.RetrieveEngineService) interfaces.RWCapableEngine

// noopRouter 在工厂未注册时使用的简单内存实现，保证 nil 安全和测试可用
type noopRouter struct {
	mu      sync.RWMutex
	engines map[string]interfaces.RWCapableEngine
}

func newNoopRouter() *noopRouter {
	return &noopRouter{engines: make(map[string]interfaces.RWCapableEngine)}
}

func (r *noopRouter) RegisterEngine(storeID string, master interfaces.RWCapableEngine, _ []interfaces.ReadableNode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.engines[storeID] = master
	return nil
}
func (r *noopRouter) UnregisterEngine(storeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.engines, storeID)
}
func (r *noopRouter) GetReader(ctx context.Context, storeID string, _ types.ConsistencyLevel, _ *types.WriteToken) (interfaces.ReadableNode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if eng, ok := r.engines[storeID]; ok {
		return eng, nil
	}
	return nil, fmt.Errorf("store %s not found", storeID)
}
func (r *noopRouter) GetWriter(ctx context.Context, storeID string) (interfaces.RetrieveEngineService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if eng, ok := r.engines[storeID]; ok {
		return eng, nil
	}
	return nil, fmt.Errorf("store %s not found", storeID)
}

// simpleRWEngine 在工厂未注册时使用的简单包装器，将 RetrieveEngineService 适配为 RWCapableEngine
type simpleRWEngine struct {
	interfaces.RetrieveEngineService
	storeID string
}

func (e *simpleRWEngine) HealthCheck(_ context.Context) (*types.NodeHealth, error) {
	return &types.NodeHealth{NodeID: e.storeID, Healthy: true, LastChecked: time.Now()}, nil
}
func (e *simpleRWEngine) WaitForLSN(_ context.Context, _ int64, _ time.Duration) error {
	return nil
}
func (e *simpleRWEngine) Flush(_ context.Context) error {
	return nil
}
func (e *simpleRWEngine) GetCurrentLSN(_ context.Context) (int64, error) {
	return 0, nil
}
func (e *simpleRWEngine) LastWriteToken() *types.WriteToken {
	return nil
}

// noopWrapper 在工厂未注册时使用的包装器
func noopWrapper(storeID string, engine interfaces.RetrieveEngineService) interfaces.RWCapableEngine {
	return &simpleRWEngine{RetrieveEngineService: engine, storeID: storeID}
}

// 工厂变量，由 service 包在初始化时通过 SetRouterFactory 注册
var (
	routerFactory = func(cfg types.ReadWriteSeparationConfig) Router { return newNoopRouter() }
	engineWrapper EngineWrapper = noopWrapper
)

// SetRouterFactory 由 service 包调用，注册路由器和引擎包装器的工厂函数
// 必须在 NewRetrieveEngineRegistry 之前调用
func SetRouterFactory(rf func(cfg types.ReadWriteSeparationConfig) Router, ew EngineWrapper) {
	routerFactory = rf
	engineWrapper = ew
}

// RetrieveEngineRegistry implements the retrieval engine registry.
// It maintains two maps:
//   - byEngineType: env stores registered via RETRIEVE_DRIVER (backward compatible)
//   - byStoreID: DB stores registered via VectorStore table (instance-based)
//
// Implements both interfaces.RetrieveEngineRegistry and interfaces.StoreRegistry.
// D4版本集成读写分离路由器，自动实现读写分离、负载均衡、健康检查等能力
type RetrieveEngineRegistry struct {
	mu           sync.RWMutex
	byEngineType map[types.RetrieverEngineType]interfaces.RetrieveEngineService
	byStoreID    map[string]interfaces.RetrieveEngineService
	router       Router
}

// routerWrapper 包装单个store的router，对外暴露RetrieveEngineService接口，自动路由读写
type routerWrapper struct {
	storeID string
	router  Router
}

func (w *routerWrapper) EngineType() types.RetrieverEngineType {
	writer, err := w.router.GetWriter(context.Background(), w.storeID)
	if err != nil {
		return types.OpenSearchRetrieverEngineType
	}
	if svc, ok := writer.(interfaces.RetrieveEngineService); ok {
		return svc.EngineType()
	}
	return types.OpenSearchRetrieverEngineType
}

func (w *routerWrapper) Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error) {
	start := time.Now()
	log := logger.GetLogger(ctx)
	log.Infof("[Retriever] Retrieve request: storeID=%s, type=%s, topK=%d, kbIDs=%v",
		w.storeID, params.RetrieverType, params.TopK, params.KnowledgeBaseIDs)

	reader, err := w.router.GetReader(ctx, w.storeID, params.ConsistencyLevel, nil)
	if err != nil {
		log.Errorf("[Retriever] Failed to get reader for store %s: %v", w.storeID, err)
		return nil, err
	}

	results, err := reader.Retrieve(ctx, params)
	duration := time.Since(start)
	if err != nil {
		log.Errorf("[Retriever] Retrieve failed for store %s after %dms: %v",
			w.storeID, duration.Milliseconds(), err)
		return nil, err
	}

	totalResults := 0
	for _, r := range results {
		totalResults += len(r.Results)
	}
	log.Infof("[Retriever] Retrieve completed: storeID=%s, totalResults=%d, duration=%dms",
		w.storeID, totalResults, duration.Milliseconds())
	return results, nil
}

func (w *routerWrapper) Support() []types.RetrieverType {
	writer, err := w.router.GetWriter(context.Background(), w.storeID)
	if err != nil {
		return nil
	}
	if svc, ok := writer.(interfaces.RetrieveEngineService); ok {
		return svc.Support()
	}
	return nil
}

func (w *routerWrapper) Index(ctx context.Context, embedder embedding.Embedder, indexInfo *types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.Index(ctx, embedder, indexInfo, retrieverTypes)
}

func (w *routerWrapper) BatchIndex(ctx context.Context, embedder embedding.Embedder, indexInfoList []*types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchIndex(ctx, embedder, indexInfoList, retrieverTypes)
}

func (w *routerWrapper) CopyIndices(ctx context.Context, sourceKnowledgeBaseID string, sourceToTargetKBIDMap map[string]string, sourceToTargetChunkIDMap map[string]string, targetKnowledgeBaseID string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.CopyIndices(ctx, sourceKnowledgeBaseID, sourceToTargetKBIDMap, sourceToTargetChunkIDMap, targetKnowledgeBaseID, dimension, knowledgeType)
}

func (w *routerWrapper) DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteByChunkIDList(ctx, indexIDList, dimension, knowledgeType)
}

func (w *routerWrapper) DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteBySourceIDList(ctx, sourceIDList, dimension, knowledgeType)
}

func (w *routerWrapper) DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.DeleteByKnowledgeIDList(ctx, knowledgeIDList, dimension, knowledgeType)
}

func (w *routerWrapper) BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchUpdateChunkEnabledStatus(ctx, chunkStatusMap)
}

func (w *routerWrapper) BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	return writer.BatchUpdateChunkTagID(ctx, chunkTagMap)
}

func (w *routerWrapper) Flush(ctx context.Context) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	if flusher, ok := writer.(interface{ Flush(context.Context) error }); ok {
		return flusher.Flush(ctx)
	}
	return nil
}

func (w *routerWrapper) GetCurrentLSN(ctx context.Context) (int64, error) {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return 0, err
	}
	if lsnGetter, ok := writer.(interface{ GetCurrentLSN(context.Context) (int64, error) }); ok {
		return lsnGetter.GetCurrentLSN(ctx)
	}
	return 0, nil
}

func (w *routerWrapper) WaitForLSN(ctx context.Context, lsn int64, timeout time.Duration) error {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return err
	}
	if lsnWaiter, ok := writer.(interface{ WaitForLSN(context.Context, int64, time.Duration) error }); ok {
		return lsnWaiter.WaitForLSN(ctx, lsn, timeout)
	}
	return nil
}

func (w *routerWrapper) EstimateStorageSize(ctx context.Context, embedder embedding.Embedder, indexInfoList []*types.IndexInfo, retrieverTypes []types.RetrieverType) int64 {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return 0
	}
	return writer.EstimateStorageSize(ctx, embedder, indexInfoList, retrieverTypes)
}

func (w *routerWrapper) HealthCheck(ctx context.Context) (*types.NodeHealth, error) {
	writer, err := w.router.GetWriter(ctx, w.storeID)
	if err != nil {
		return nil, err
	}
	if healthChecker, ok := writer.(interface{ HealthCheck(context.Context) (*types.NodeHealth, error) }); ok {
		return healthChecker.HealthCheck(ctx)
	}
	return &types.NodeHealth{
		NodeID:   w.storeID,
		Healthy:  true,
		LSN:      0,
		LastChecked: time.Now(),
	}, nil
}

// NewRetrieveEngineRegistry creates a new retrieval engine registry
func NewRetrieveEngineRegistry() interfaces.RetrieveEngineRegistry {
	rwConfig := types.DefaultReadWriteSeparationConfig()
	// 第一版默认关闭读写分离，保持完全兼容，可通过配置开启
	rwConfig.Enabled = false
	return &RetrieveEngineRegistry{
		byEngineType: make(map[types.RetrieverEngineType]interfaces.RetrieveEngineService),
		byStoreID:    make(map[string]interfaces.RetrieveEngineService),
		router:       routerFactory(rwConfig),
	}
}

// --- interfaces.RetrieveEngineRegistry methods (unchanged behavior) ---

// Register registers a retrieval engine service by engine type.
// Returns an error if the engine type is already registered.
func (r *RetrieveEngineRegistry) Register(repo interfaces.RetrieveEngineService) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	log := logger.GetLogger(context.Background())
	if _, exists := r.byEngineType[repo.EngineType()]; exists {
		return fmt.Errorf("repository type %s already registered", repo.EngineType())
	}

	log.Infof("[Retriever] Registering engine by type: %s, support=%v", repo.EngineType(), repo.Support())
	r.byEngineType[repo.EngineType()] = repo
	// 环境变量引擎也注册到router（单节点模式）
	wrapped := engineWrapper(string(repo.EngineType()), repo)
	_ = r.router.RegisterEngine(string(repo.EngineType()), wrapped, nil)
	log.Infof("[Retriever] Engine %s registered successfully", repo.EngineType())
	return nil
}

// GetRetrieveEngineService retrieves a retrieval engine service by type.
// Only searches the byEngineType map (env stores).
func (r *RetrieveEngineRegistry) GetRetrieveEngineService(repoType types.RetrieverEngineType) (
	interfaces.RetrieveEngineService, error,
) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	repo, exists := r.byEngineType[repoType]
	if !exists {
		return nil, fmt.Errorf("repository of type %s not found", repoType)
	}

	return repo, nil
}

// GetAllRetrieveEngineServices retrieves all registered retrieval engine services.
// Only returns byEngineType entries (env stores) for backward compatibility.
func (r *RetrieveEngineRegistry) GetAllRetrieveEngineServices() []interfaces.RetrieveEngineService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]interfaces.RetrieveEngineService, 0, len(r.byEngineType))
	for _, v := range r.byEngineType {
		result = append(result, v)
	}

	return result
}

// --- interfaces.StoreRegistry methods (new, for VectorStore-based engines) ---

// RegisterWithStoreID registers an engine service by VectorStore ID.
// Unlike Register(), the same EngineType can be registered multiple times
// with different StoreIDs (e.g., two Elasticsearch clusters).
// Upsert semantics: existing entry is overwritten silently.
func (r *RetrieveEngineRegistry) RegisterWithStoreID(storeID string, svc interfaces.RetrieveEngineService) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 使用适配器包装旧引擎，自动适配新接口
	wrapped := engineWrapper(storeID, svc)
	_ = r.router.RegisterEngine(storeID, wrapped, nil)

	// 返回透明包装器，对上层完全兼容
	wrapper := &routerWrapper{
		storeID: storeID,
		router:  r.router,
	}
	r.byStoreID[storeID] = wrapper
}

// GetByStoreID retrieves an engine service by VectorStore ID.
// Callers must verify tenant ownership before using the returned service.
func (r *RetrieveEngineRegistry) GetByStoreID(storeID string) (interfaces.RetrieveEngineService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, exists := r.byStoreID[storeID]
	if !exists {
		return nil, fmt.Errorf("store %s not found in registry", storeID)
	}
	return svc, nil
}

// UnregisterByStoreID removes an engine service from the byStoreID map.
// Idempotent: returns silently if the storeID is not found.
//
// NOTE: gRPC-based clients (Qdrant, Milvus) hold connections that are not closed here.
// Known Phase 1 limitation — store deletion is rare, connections cleaned up on process exit.
// Phase 2 should add Close() to RetrieveEngineService interface and call it here.
func (r *RetrieveEngineRegistry) UnregisterByStoreID(storeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.router.UnregisterEngine(storeID)
	delete(r.byStoreID, storeID)
}

// GetRouter 获取读写分离路由器实例（用于高级配置）
func (r *RetrieveEngineRegistry) GetRouter() Router {
	return r.router
}

// Compile-time assertion: *RetrieveEngineRegistry satisfies the
// interfaces.RetrieveEngineRegistry contract, including GetByStoreID.
var _ interfaces.RetrieveEngineRegistry = (*RetrieveEngineRegistry)(nil)
