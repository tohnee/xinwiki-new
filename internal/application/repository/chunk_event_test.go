package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/types"
)

// setupTestDB 初始化测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移 Chunk 表
	err = db.AutoMigrate(&types.Chunk{})
	require.NoError(t, err)

	return db
}

// TestChunkRepository_UpdateChunk_PermissionEvent 验证权限变更事件正确发布
func TestChunkRepository_UpdateChunk_PermissionEvent(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	bus := event.NewEventBus()

	repo := NewChunkRepository(db, bus)

	// 1. 创建测试 Chunk
	chunk := &types.Chunk{
		ID:              "test-chunk-001",
		TenantID:        1001,
		KnowledgeBaseID: "kb-test-001",
		SecurityLevel:   "L1",
		AllowedUserIDs:  types.StringArray{"user-a", "user-b"},
		AllowedGroupIDs: types.StringArray{"group-1"},
		Content:         "初始测试内容",
	}
	err := db.WithContext(ctx).Create(chunk).Error
	require.NoError(t, err)

	// 2. 注册事件监听收集结果
	var receivedEvent *event.Event
	var receivedData *event.PermissionChangedData
	bus.On(event.EventPermissionChanged, func(ctx context.Context, evt event.Event) error {
		receivedEvent = &evt
		data, ok := evt.Data.(*event.PermissionChangedData)
		if ok {
			receivedData = data
		}
		return nil
	})

	// 3. 仅 SecurityLevel 变更 (触发事件)
	chunk.SecurityLevel = "L3"
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)

	// 4. 验证事件发布
	require.NotNil(t, receivedEvent, "SecurityLevel 变更应触发 EventPermissionChanged 事件")
	assert.Equal(t, event.EventPermissionChanged, receivedEvent.Type)
	assert.Equal(t, uint64(1001), receivedData.TenantID, "事件应包含正确的 TenantID")
	assert.Equal(t, "kb-test-001", receivedData.KBID, "事件应包含正确的 KBID")
	assert.Equal(t, "chunk", receivedData.ResourceType, "事件应包含正确的 ResourceType")
	assert.Equal(t, "test-chunk-001", receivedData.ResourceID, "事件应包含正确的 ResourceID")
	assert.Equal(t, "L1", receivedData.OldSecurityLevel, "事件应包含旧的 SecurityLevel")
	assert.Equal(t, "L3", receivedData.NewSecurityLevel, "事件应包含新的 SecurityLevel")

	// 重置状态继续测试
	receivedEvent = nil
	receivedData = nil

	// 5. 仅 AllowedUserIDs 变更 (触发事件)
	chunk.AllowedUserIDs = types.StringArray{"user-c"}
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)
	assert.NotNil(t, receivedEvent, "AllowedUserIDs 变更应触发事件")
	assert.ElementsMatch(t, []string{"user-a", "user-b"}, receivedData.OldAllowedUserIDs, "事件应包含旧的 AllowedUserIDs")
	assert.ElementsMatch(t, []string{"user-c"}, receivedData.NewAllowedUserIDs, "事件应包含新的 AllowedUserIDs")

	receivedEvent = nil
	receivedData = nil

	// 6. 仅 AllowedGroupIDs 变更 (触发事件)
	chunk.AllowedGroupIDs = types.StringArray{"group-2", "group-3"}
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)
	assert.NotNil(t, receivedEvent, "AllowedGroupIDs 变更应触发事件")
	assert.ElementsMatch(t, []string{"group-1"}, receivedData.OldAllowedGroupIDs, "事件应包含旧的 AllowedGroupIDs")
	assert.ElementsMatch(t, []string{"group-2", "group-3"}, receivedData.NewAllowedGroupIDs, "事件应包含新的 AllowedGroupIDs")

	receivedEvent = nil
	receivedData = nil

	// 7. 仅 Content 变更 (不应触发事件)
	chunk.Content = "更新后的内容"
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)
	assert.Nil(t, receivedEvent, "Content 变更不应触发权限事件")

	// 8. 组合 ACL 字段同时变更 (事件数据正确)
	chunk.SecurityLevel = "L2"
	chunk.AllowedUserIDs = types.StringArray{"user-d"}
	chunk.AllowedGroupIDs = types.StringArray{"group-4"}
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)
	assert.NotNil(t, receivedEvent, "ACL 多字段同时变更应触发事件")
	assert.Equal(t, "L3", receivedData.OldSecurityLevel)
	assert.Equal(t, "L2", receivedData.NewSecurityLevel)
	assert.ElementsMatch(t, []string{"user-c"}, receivedData.OldAllowedUserIDs)
	assert.ElementsMatch(t, []string{"user-d"}, receivedData.NewAllowedUserIDs)
}

// TestChunkRepository_UpdateChunk_NoEventBus 验证 EventBus 为 nil 时安全跳过事件发布
func TestChunkRepository_UpdateChunk_NoEventBus(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)

	// EventBus = nil，测试不会 panic
	repo := NewChunkRepository(db, nil)

	chunk := &types.Chunk{
		ID:              "test-chunk-002",
		TenantID:        1002,
		KnowledgeBaseID: "kb-test-002",
		SecurityLevel:   "L1",
		Content:         "测试内容",
	}
	err := db.WithContext(ctx).Create(chunk).Error
	require.NoError(t, err)

	// 即使 ACL 变更，EventBus=nil 时也应安全完成更新
	chunk.SecurityLevel = "L2"
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err, "EventBus 为 nil 时不应 panic，应正常完成更新")

	// 验证数据库确实更新了
	var updated types.Chunk
	err = db.WithContext(ctx).Where("id = ?", "test-chunk-002").First(&updated).Error
	assert.NoError(t, err)
	assert.Equal(t, "L2", updated.SecurityLevel, "数据库应正确更新 SecurityLevel")
}

// TestChunkRepository_UpdateChunk_DataSerialization 验证事件数据可正确 JSON 序列化
func TestChunkRepository_UpdateChunk_DataSerialization(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	bus := event.NewEventBus()
	repo := NewChunkRepository(db, bus)

	chunk := &types.Chunk{
		ID:              "test-chunk-003",
		TenantID:        1003,
		KnowledgeBaseID: "kb-test-003",
		SecurityLevel:   "L1",
		AllowedUserIDs:  types.StringArray{"u1", "u2"},
		AllowedGroupIDs: types.StringArray{"g1"},
	}
	err := db.WithContext(ctx).Create(chunk).Error
	require.NoError(t, err)

	var jsonOutput []byte
	bus.On(event.EventPermissionChanged, func(ctx context.Context, evt event.Event) error {
		jsonOutput, err = json.MarshalIndent(evt.Data, "", "  ")
		return err
	})

	chunk.SecurityLevel = "L3"
	err = repo.UpdateChunk(ctx, chunk)
	assert.NoError(t, err)

	// 验证 JSON 序列化正常
	assert.NoError(t, err, "PermissionChangedData 应可 JSON 序列化")
	t.Logf("事件数据 JSON:\n%s", string(jsonOutput))

	// 反序列化验证完整性
	var parsed event.PermissionChangedData
	err = json.Unmarshal(jsonOutput, &parsed)
	assert.NoError(t, err, "反序列化应成功")
	assert.Equal(t, uint64(1003), parsed.TenantID)
	assert.Equal(t, "kb-test-003", parsed.KBID)
	assert.Equal(t, "L1", parsed.OldSecurityLevel)
	assert.Equal(t, "L3", parsed.NewSecurityLevel)
}

// TestChunkRepository_UpdateChunks_Batch_NoEvent 验证批量更新方法不触发事件（按规范只更新非ACL字段）
func TestChunkRepository_UpdateChunks_Batch_NoEvent(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	bus := event.NewEventBus()
	repo := NewChunkRepository(db, bus)

	// 创建多个 Chunk
	for i := 0; i < 3; i++ {
		chunk := &types.Chunk{
			ID:              fmt.Sprintf("test-chunk-batch-%d", i),
			TenantID:        1000,
			KnowledgeBaseID: "kb-batch",
			SecurityLevel:   "L1",
			Content:         fmt.Sprintf("内容 %d", i),
			IsEnabled:       true,
		}
		err := db.WithContext(ctx).Create(chunk).Error
		require.NoError(t, err)
	}

	// 收集所有事件
	var eventCount int
	bus.On(event.EventPermissionChanged, func(ctx context.Context, evt event.Event) error {
		eventCount++
		return nil
	})

	// 加载要更新的 Chunk
	chunks := make([]*types.Chunk, 3)
	for i := 0; i < 3; i++ {
		chunk := &types.Chunk{}
		err := db.WithContext(ctx).Where("id = ?", fmt.Sprintf("test-chunk-batch-%d", i)).First(chunk).Error
		require.NoError(t, err)
		chunks[i] = chunk
	}

	// 对批量更新的字段（只更新 content/is_enabled/tag_id/flags/status）做修改
	for _, c := range chunks {
		c.IsEnabled = false
		c.Status = 1
	}

	// 执行批量更新
	err := repo.UpdateChunks(ctx, chunks)
	assert.NoError(t, err)

	// 批量更新方法不触发权限事件（因为不更新ACL相关字段）
	assert.Equal(t, 0, eventCount, "UpdateChunks 批量更新只修改固定字段，不应触发权限事件")

	// 验证数据库更新生效
	for i := 0; i < 3; i++ {
		var c types.Chunk
		err := db.WithContext(ctx).Where("id = ?", fmt.Sprintf("test-chunk-batch-%d", i)).First(&c).Error
		assert.NoError(t, err)
		assert.False(t, c.IsEnabled, "批量更新 IsEnabled 应生效")
		assert.Equal(t, 1, c.Status, "批量更新 Status 应生效")
	}
}
