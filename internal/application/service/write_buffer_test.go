package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteBuffer_EnqueueAndProcess(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 10,
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  2,
		Master:       master,
	})
	defer wb.Close()

	req := &WriteRequest{
		OpType: WriteOpIndex,
	}
	ch, err := wb.Enqueue(req)
	require.NoError(t, err)

	select {
	case result := <-ch:
		assert.NoError(t, result.Err)
		assert.NotNil(t, result.Token)
	case <-time.After(2 * time.Second):
		t.Fatal("write buffer did not process request within timeout")
	}
}

func TestWriteBuffer_BatchFlush_OnMaxBatchSize(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 3,
		MaxWaitTime:  10 * time.Second, // 长超时，确保靠batch size触发
		Concurrency:  1,
		Master:       master,
	})
	defer wb.Close()

	results := make([]<-chan WriteResult, 5)
	for i := 0; i < 5; i++ {
		req := &WriteRequest{
			OpType: WriteOpIndex,
		}
		ch, err := wb.Enqueue(req)
		require.NoError(t, err)
		results[i] = ch
	}

	// 前3个请求应因batch size达到而立即处理
	for i := 0; i < 3; i++ {
		select {
		case result := <-results[i]:
			assert.NoError(t, result.Err)
		case <-time.After(2 * time.Second):
			t.Fatalf("request %d not processed within timeout", i)
		}
	}
}

func TestWriteBuffer_BatchFlush_OnTimeout(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 100, // 大batch size，确保靠timeout触发
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  1,
		Master:       master,
	})
	defer wb.Close()

	req := &WriteRequest{
		OpType: WriteOpIndex,
	}
	ch, err := wb.Enqueue(req)
	require.NoError(t, err)

	// 等待超时触发flush
	select {
	case result := <-ch:
		assert.NoError(t, result.Err)
		assert.NotNil(t, result.Token)
	case <-time.After(2 * time.Second):
		t.Fatal("write buffer did not flush on timeout")
	}
}

func TestWriteBuffer_Close_PreventsNewEnqueue(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 10,
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  1,
		Master:       master,
	})

	require.NoError(t, wb.Close())

	// 关闭后Enqueue应返回错误
	req := &WriteRequest{OpType: WriteOpIndex}
	_, err := wb.Enqueue(req)
	assert.Error(t, err)
}

func TestWriteBuffer_Close_Idempotent(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 10,
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  1,
		Master:       master,
	})

	require.NoError(t, wb.Close())
	// 二次关闭不应panic
	require.NoError(t, wb.Close())
}

func TestWriteBuffer_FlushAll(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 100,
		MaxWaitTime:  10 * time.Second,
		Concurrency:  2,
		Master:       master,
	})
	defer wb.Close()

	// 入队几个请求
	for i := 0; i < 3; i++ {
		req := &WriteRequest{OpType: WriteOpIndex}
		_, err := wb.Enqueue(req)
		require.NoError(t, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := wb.FlushAll(ctx)
	assert.NoError(t, err)
}

func TestWriteBuffer_DefaultConfig(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		Master: master,
		// MaxBatchSize=0, MaxWaitTime=0, Concurrency=0 测试默认值
	})
	defer wb.Close()

	req := &WriteRequest{OpType: WriteOpIndex}
	ch, err := wb.Enqueue(req)
	require.NoError(t, err)

	select {
	case result := <-ch:
		assert.NoError(t, result.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("write buffer with default config did not process request")
	}
}

func TestWriteBuffer_DeleteOperations(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 10,
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  2,
		Master:       master,
	})
	defer wb.Close()

	tests := []struct {
		name   string
		opType WriteOpType
		req    *WriteRequest
	}{
		{
			name:   "DeleteByChunkID",
			opType: WriteOpDeleteByChunkID,
			req: &WriteRequest{
				OpType:        WriteOpDeleteByChunkID,
				IDList:        []string{"chunk-1"},
				Dimension:     128,
				KnowledgeType: "test",
			},
		},
		{
			name:   "DeleteBySourceID",
			opType: WriteOpDeleteBySourceID,
			req: &WriteRequest{
				OpType:        WriteOpDeleteBySourceID,
				IDList:        []string{"src-1"},
				Dimension:     128,
				KnowledgeType: "test",
			},
		},
		{
			name:   "DeleteByKnowledgeID",
			opType: WriteOpDeleteByKnowledgeID,
			req: &WriteRequest{
				OpType:        WriteOpDeleteByKnowledgeID,
				IDList:        []string{"kb-1"},
				Dimension:     128,
				KnowledgeType: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := wb.Enqueue(tt.req)
			require.NoError(t, err)
			select {
			case result := <-ch:
				assert.NoError(t, result.Err)
			case <-time.After(2 * time.Second):
				t.Fatalf("%s not processed within timeout", tt.name)
			}
		})
	}
}

func TestWriteBuffer_UpdateOperations(t *testing.T) {
	master := WrapEngineWithRWCapabilities("test-store", newMockEngine())
	wb := NewWriteBuffer(WriteBufferConfig{
		MaxBatchSize: 10,
		MaxWaitTime:  50 * time.Millisecond,
		Concurrency:  2,
		Master:       master,
	})
	defer wb.Close()

	// BatchUpdateChunkEnabledStatus
	req1 := &WriteRequest{
		OpType:         WriteOpUpdateChunkEnabled,
		ChunkStatusMap: map[string]bool{"chunk-1": true},
	}
	ch1, err := wb.Enqueue(req1)
	require.NoError(t, err)
	select {
	case result := <-ch1:
		assert.NoError(t, result.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("BatchUpdateChunkEnabledStatus not processed")
	}

	// BatchUpdateChunkTagID
	req2 := &WriteRequest{
		OpType:      WriteOpUpdateChunkTag,
		ChunkTagMap: map[string]string{"chunk-1": "tag-1"},
	}
	ch2, err := wb.Enqueue(req2)
	require.NoError(t, err)
	select {
	case result := <-ch2:
		assert.NoError(t, result.Err)
	case <-time.After(2 * time.Second):
		t.Fatal("BatchUpdateChunkTagID not processed")
	}
}
