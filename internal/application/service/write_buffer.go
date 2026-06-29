package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// WriteOpType 写入操作类型
type WriteOpType int

const (
	WriteOpIndex WriteOpType = iota
	WriteOpBatchIndex
	WriteOpCopyIndices
	WriteOpDeleteByChunkID
	WriteOpDeleteBySourceID
	WriteOpDeleteByKnowledgeID
	WriteOpUpdateChunkEnabled
	WriteOpUpdateChunkTag
)

// WriteRequest 写入请求
type WriteRequest struct {
	OpType       WriteOpType
	Embedder     embedding.Embedder
	IndexInfo    *types.IndexInfo
	IndexInfoList []*types.IndexInfo
	RetrieverTypes []types.RetrieverType
	// CopyIndices参数
	SourceKBID            string
	SourceToTargetKBMap   map[string]string
	SourceToTargetChunkMap map[string]string
	TargetKBID            string
	Dimension             int
	KnowledgeType         string
	// Delete参数
	IDList []string
	// Update参数
	ChunkStatusMap map[string]bool
	ChunkTagMap    map[string]string

	DoneCh chan WriteResult
}

// WriteResult 写入结果
type WriteResult struct {
	Token *types.WriteToken
	Err   error
}

// WriteBufferConfig 写入缓冲区配置
type WriteBufferConfig struct {
	MaxBufferPerKB int
	MaxBatchSize   int
	MaxWaitTime    time.Duration
	Concurrency    int
	Master         interfaces.RWCapableEngine
}

// WriteBuffer 写入缓冲接口
type WriteBuffer interface {
	// Enqueue 写入请求入队
	Enqueue(req *WriteRequest) (<-chan WriteResult, error)
	// FlushAll 刷新所有缓冲的写入
	FlushAll(ctx context.Context) error
	// Close 关闭缓冲区
	Close() error
}

// bufferedWriter 带缓冲的写入器，包装主节点实现批量写入
type bufferedWriter struct {
	master interfaces.RWCapableEngine
	buffer WriteBuffer
}

// writeBuffer 实际实现
type writeBuffer struct {
	mu         sync.Mutex
	cfg        WriteBufferConfig
	buffer     []*WriteRequest
	bufferSize int
	timer      *time.Timer
	workerCh   chan *WriteRequest
	stopCh     chan struct{}
	wg         sync.WaitGroup
	closed     bool
}

// NewWriteBuffer 创建写入缓冲区
func NewWriteBuffer(cfg WriteBufferConfig) WriteBuffer {
	if cfg.MaxBatchSize <= 0 {
		cfg.MaxBatchSize = 100
	}
	if cfg.MaxWaitTime <= 0 {
		cfg.MaxWaitTime = 10 * time.Millisecond
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	wb := &writeBuffer{
		cfg:      cfg,
		buffer:   make([]*WriteRequest, 0, cfg.MaxBatchSize),
		workerCh: make(chan *WriteRequest, cfg.MaxBatchSize*2),
		stopCh:   make(chan struct{}),
	}

	// 启动工作协程
	for i := 0; i < cfg.Concurrency; i++ {
		wb.wg.Add(1)
		go wb.runWorker()
	}

	return wb
}

func (wb *writeBuffer) Enqueue(req *WriteRequest) (<-chan WriteResult, error) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	if wb.closed {
		return nil, errors.New("write buffer closed")
	}

	req.DoneCh = make(chan WriteResult, 1)
	wb.buffer = append(wb.buffer, req)
	wb.bufferSize++

	// 达到批次大小，立即刷新
	if wb.bufferSize >= wb.cfg.MaxBatchSize {
		wb.flushLocked()
		return req.DoneCh, nil
	}

	// 启动定时器
	if wb.timer == nil {
		wb.timer = time.AfterFunc(wb.cfg.MaxWaitTime, func() {
			wb.mu.Lock()
			defer wb.mu.Unlock()
			if !wb.closed {
				wb.flushLocked()
			}
		})
	}

	return req.DoneCh, nil
}

func (wb *writeBuffer) flushLocked() {
	if wb.timer != nil {
		wb.timer.Stop()
		wb.timer = nil
	}

	if len(wb.buffer) == 0 {
		return
	}

	// 将批次发送给worker
	// 简化实现：当前逐条发送，实际可以合并同类型操作
	for _, req := range wb.buffer {
		wb.workerCh <- req
	}

	// 清空缓冲区
	wb.buffer = wb.buffer[:0]
	wb.bufferSize = 0
}

func (wb *writeBuffer) runWorker() {
	defer wb.wg.Done()
	for {
		select {
		case <-wb.stopCh:
			// 处理剩余请求
			wb.mu.Lock()
			for _, req := range wb.buffer {
				wb.processRequest(req)
			}
			wb.buffer = wb.buffer[:0]
			wb.mu.Unlock()

			// 处理channel中剩余请求
			for {
				select {
				case req := <-wb.workerCh:
					wb.processRequest(req)
				default:
					return
				}
			}
		case req := <-wb.workerCh:
			wb.processRequest(req)
		}
	}
}

func (wb *writeBuffer) processRequest(req *WriteRequest) {
	var token *types.WriteToken
	var err error

	ctx := context.Background()
	master := wb.cfg.Master

	switch req.OpType {
	case WriteOpIndex:
		err = master.Index(ctx, req.Embedder, req.IndexInfo, req.RetrieverTypes)
	case WriteOpBatchIndex:
		err = master.BatchIndex(ctx, req.Embedder, req.IndexInfoList, req.RetrieverTypes)
	case WriteOpCopyIndices:
		err = master.CopyIndices(ctx, req.SourceKBID, req.SourceToTargetKBMap, req.SourceToTargetChunkMap, req.TargetKBID, req.Dimension, req.KnowledgeType)
	case WriteOpDeleteByChunkID:
		err = master.DeleteByChunkIDList(ctx, req.IDList, req.Dimension, req.KnowledgeType)
	case WriteOpDeleteBySourceID:
		err = master.DeleteBySourceIDList(ctx, req.IDList, req.Dimension, req.KnowledgeType)
	case WriteOpDeleteByKnowledgeID:
		err = master.DeleteByKnowledgeIDList(ctx, req.IDList, req.Dimension, req.KnowledgeType)
	case WriteOpUpdateChunkEnabled:
		err = master.BatchUpdateChunkEnabledStatus(ctx, req.ChunkStatusMap)
	case WriteOpUpdateChunkTag:
		err = master.BatchUpdateChunkTagID(ctx, req.ChunkTagMap)
	}

	// 写入成功后获取最新的WriteToken
	if err == nil {
		token = master.LastWriteToken()
	}

	req.DoneCh <- WriteResult{Token: token, Err: err}
	close(req.DoneCh)
}

func (wb *writeBuffer) FlushAll(ctx context.Context) error {
	wb.mu.Lock()
	wb.flushLocked()
	wb.mu.Unlock()

	// 简单等待：确保所有入队请求都被处理
	// 生产环境应该有更精确的等待机制
	select {
	case <-time.After(500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wb *writeBuffer) Close() error {
	wb.mu.Lock()
	if wb.closed {
		wb.mu.Unlock()
		return nil
	}
	wb.closed = true
	close(wb.stopCh)
	wb.mu.Unlock()

	wb.wg.Wait()
	close(wb.workerCh)
	return nil
}

// 实现bufferedWriter的所有写入方法
func (w *bufferedWriter) Index(ctx context.Context, embedder embedding.Embedder, indexInfo *types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	req := &WriteRequest{
		OpType:        WriteOpIndex,
		Embedder:      embedder,
		IndexInfo:     indexInfo,
		RetrieverTypes: retrieverTypes,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		// 缓冲区不可用，直接写主节点
		return w.master.Index(ctx, embedder, indexInfo, retrieverTypes)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) BatchIndex(ctx context.Context, embedder embedding.Embedder, indexInfoList []*types.IndexInfo, retrieverTypes []types.RetrieverType) error {
	req := &WriteRequest{
		OpType:         WriteOpBatchIndex,
		Embedder:       embedder,
		IndexInfoList:  indexInfoList,
		RetrieverTypes: retrieverTypes,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.BatchIndex(ctx, embedder, indexInfoList, retrieverTypes)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) CopyIndices(ctx context.Context, sourceKnowledgeBaseID string, sourceToTargetKBIDMap map[string]string, sourceToTargetChunkIDMap map[string]string, targetKnowledgeBaseID string, dimension int, knowledgeType string) error {
	req := &WriteRequest{
		OpType:                WriteOpCopyIndices,
		SourceKBID:            sourceKnowledgeBaseID,
		SourceToTargetKBMap:   sourceToTargetKBIDMap,
		SourceToTargetChunkMap: sourceToTargetChunkIDMap,
		TargetKBID:            targetKnowledgeBaseID,
		Dimension:             dimension,
		KnowledgeType:         knowledgeType,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.CopyIndices(ctx, sourceKnowledgeBaseID, sourceToTargetKBIDMap, sourceToTargetChunkIDMap, targetKnowledgeBaseID, dimension, knowledgeType)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) DeleteByChunkIDList(ctx context.Context, indexIDList []string, dimension int, knowledgeType string) error {
	req := &WriteRequest{
		OpType:        WriteOpDeleteByChunkID,
		IDList:        indexIDList,
		Dimension:     dimension,
		KnowledgeType: knowledgeType,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.DeleteByChunkIDList(ctx, indexIDList, dimension, knowledgeType)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) DeleteBySourceIDList(ctx context.Context, sourceIDList []string, dimension int, knowledgeType string) error {
	req := &WriteRequest{
		OpType:        WriteOpDeleteBySourceID,
		IDList:        sourceIDList,
		Dimension:     dimension,
		KnowledgeType: knowledgeType,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.DeleteBySourceIDList(ctx, sourceIDList, dimension, knowledgeType)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) DeleteByKnowledgeIDList(ctx context.Context, knowledgeIDList []string, dimension int, knowledgeType string) error {
	req := &WriteRequest{
		OpType:        WriteOpDeleteByKnowledgeID,
		IDList:        knowledgeIDList,
		Dimension:     dimension,
		KnowledgeType: knowledgeType,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.DeleteByKnowledgeIDList(ctx, knowledgeIDList, dimension, knowledgeType)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) BatchUpdateChunkEnabledStatus(ctx context.Context, chunkStatusMap map[string]bool) error {
	req := &WriteRequest{
		OpType:         WriteOpUpdateChunkEnabled,
		ChunkStatusMap: chunkStatusMap,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.BatchUpdateChunkEnabledStatus(ctx, chunkStatusMap)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) BatchUpdateChunkTagID(ctx context.Context, chunkTagMap map[string]string) error {
	req := &WriteRequest{
		OpType:      WriteOpUpdateChunkTag,
		ChunkTagMap: chunkTagMap,
	}
	ch, err := w.buffer.Enqueue(req)
	if err != nil {
		return w.master.BatchUpdateChunkTagID(ctx, chunkTagMap)
	}

	select {
	case result := <-ch:
		return result.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *bufferedWriter) Flush(ctx context.Context) error {
	return w.buffer.FlushAll(ctx)
}

func (w *bufferedWriter) GetCurrentLSN(ctx context.Context) (int64, error) {
	return w.master.GetCurrentLSN(ctx)
}

func (w *bufferedWriter) WaitForLSN(ctx context.Context, lsn int64, timeout time.Duration) error {
	return w.master.WaitForLSN(ctx, lsn, timeout)
}

func (w *bufferedWriter) EngineType() types.RetrieverEngineType {
	return w.master.EngineType()
}

func (w *bufferedWriter) Retrieve(ctx context.Context, params types.RetrieveParams) ([]*types.RetrieveResult, error) {
	return w.master.Retrieve(ctx, params)
}

func (w *bufferedWriter) Support() []types.RetrieverType {
	return w.master.Support()
}

func (w *bufferedWriter) HealthCheck(ctx context.Context) (*types.NodeHealth, error) {
	return w.master.HealthCheck(ctx)
}

func (w *bufferedWriter) LastWriteToken() *types.WriteToken {
	return w.master.LastWriteToken()
}

func (w *bufferedWriter) EstimateStorageSize(ctx context.Context,
	embedder embedding.Embedder,
	indexInfoList []*types.IndexInfo,
	retrieverTypes []types.RetrieverType,
) int64 {
	return w.master.EstimateStorageSize(ctx, embedder, indexInfoList, retrieverTypes)
}

var _ interfaces.RWCapableEngine = (*bufferedWriter)(nil)
