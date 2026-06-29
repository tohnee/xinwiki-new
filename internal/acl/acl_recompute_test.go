package acl

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWikiRepo 测试用 Wiki 仓库 mock
type mockWikiRepo struct {
	pagesBySourceRef map[string][]*types.WikiPage // sourceID -> pages
	updatedPages     []*types.WikiPage
}

func (m *mockWikiRepo) ListBySourceRef(_ context.Context, _ string, sourceID string) ([]*types.WikiPage, error) {
	return m.pagesBySourceRef[sourceID], nil
}
func (m *mockWikiRepo) UpdateMeta(_ context.Context, page *types.WikiPage) error {
	m.updatedPages = append(m.updatedPages, page)
	return nil
}

func TestACLRecomputer_SourceSecurityLevelUpgrade(t *testing.T) {
	// 场景：来源 Chunk 密级 L1→L3，派生 Wiki 应自动升为 L3
	wiki := &types.WikiPage{
		ID:            "wiki-1",
		TenantID:      1,
		KnowledgeBaseID: "kb-1",
		SecurityLevel: types.SecurityLevelL1,
		SourceRefs:    types.StringArray{"chunk-1|doc-1"},
	}

	repo := &mockWikiRepo{
		pagesBySourceRef: map[string][]*types.WikiPage{
			"chunk-1": {wiki},
		},
	}

	recomputer := NewACLRecomputer(repo, nil)
	bus := event.NewEventBus()
	recomputer.RegisterSubscribers(bus)

	// 发布权限变更事件：L1→L3
	evt := event.Event{
		ID:   "evt-1",
		Type: event.EventPermissionChanged,
		Data: &event.PermissionChangedData{
			TenantID:          1,
			KBID:              "kb-1",
			ResourceType:      "chunk",
			ResourceID:        "chunk-1",
			OldSecurityLevel:  types.SecurityLevelL1,
			NewSecurityLevel:  types.SecurityLevelL3,
			NewAllowedUserIDs: []string{},
			NewAllowedGroupIDs: []string{},
		},
	}

	err := bus.Emit(context.Background(), evt)
	require.NoError(t, err)

	// 验证 Wiki ACL 已更新
	assert.Equal(t, types.SecurityLevelL3, wiki.SecurityLevel,
		"wiki security level should be upgraded to L3")
	assert.Len(t, repo.updatedPages, 1, "wiki page should be persisted")
}

func TestACLRecomputer_Idempotent(t *testing.T) {
	wiki := &types.WikiPage{
		ID:            "wiki-1",
		KnowledgeBaseID: "kb-1",
		SecurityLevel: types.SecurityLevelL1,
		SourceRefs:    types.StringArray{"chunk-1|doc-1"},
	}

	repo := &mockWikiRepo{
		pagesBySourceRef: map[string][]*types.WikiPage{
			"chunk-1": {wiki},
		},
	}

	recomputer := NewACLRecomputer(repo, nil)
	bus := event.NewEventBus()
	recomputer.RegisterSubscribers(bus)

	evt := event.Event{
		ID:   "evt-dup",
		Type: event.EventPermissionChanged,
		Data: &event.PermissionChangedData{
			TenantID:         1,
			KBID:             "kb-1",
			ResourceID:       "chunk-1",
			NewSecurityLevel: types.SecurityLevelL3,
		},
	}

	// 第一次处理
	err := bus.Emit(context.Background(), evt)
	require.NoError(t, err)
	firstUpdateCount := len(repo.updatedPages)

	// 第二次处理同一事件（重复投递）
	err = bus.Emit(context.Background(), evt)
	require.NoError(t, err)

	// 不应再次更新
	assert.Equal(t, firstUpdateCount, len(repo.updatedPages),
		"duplicate event should not trigger recompute")
}

func TestACLRecomputer_NoSourceRefs_Skip(t *testing.T) {
	wiki := &types.WikiPage{
		ID:            "wiki-1",
		SecurityLevel: types.SecurityLevelL1,
		SourceRefs:    nil,
	}

	repo := &mockWikiRepo{
		pagesBySourceRef: map[string][]*types.WikiPage{
			"chunk-1": {wiki},
		},
	}

	recomputer := NewACLRecomputer(repo, nil)
	bus := event.NewEventBus()
	recomputer.RegisterSubscribers(bus)

	evt := event.Event{
		ID:   "evt-2",
		Type: event.EventPermissionChanged,
		Data: &event.PermissionChangedData{
			NewSecurityLevel: types.SecurityLevelL3,
			ResourceID:       "chunk-1",
		},
	}

	err := bus.Emit(context.Background(), evt)
	require.NoError(t, err)
	assert.Len(t, repo.updatedPages, 0, "wiki with no source refs should not be updated")
}

func TestACLRecomputer_AllowedUsersChange(t *testing.T) {
	wiki := &types.WikiPage{
		ID:              "wiki-1",
		SecurityLevel:   types.SecurityLevelL2,
		AllowedUserIDs:  types.StringArray{"user-a", "user-b"},
		SourceRefs:      types.StringArray{"chunk-1|doc-1"},
	}

	repo := &mockWikiRepo{
		pagesBySourceRef: map[string][]*types.WikiPage{
			"chunk-1": {wiki},
		},
	}

	recomputer := NewACLRecomputer(repo, nil)
	bus := event.NewEventBus()
	recomputer.RegisterSubscribers(bus)

	evt := event.Event{
		ID:   "evt-3",
		Type: event.EventPermissionChanged,
		Data: &event.PermissionChangedData{
			ResourceID:        "chunk-1",
			NewSecurityLevel:  types.SecurityLevelL2,
			NewAllowedUserIDs: []string{"user-a"}, // 缩小到只有 user-a
		},
	}

	err := bus.Emit(context.Background(), evt)
	require.NoError(t, err)

	// Wiki 应继承来源的新 ACL
	assert.Equal(t, types.SecurityLevelL2, wiki.SecurityLevel)
}

func TestACLRecomputer_DedupTTLExpiry(t *testing.T) {
	recomputer := NewACLRecomputer(&mockWikiRepo{}, nil)
	recomputer.dedupTTL = 50 * time.Millisecond

	recomputer.markProcessed("evt-x")
	assert.True(t, recomputer.isProcessed("evt-x"))

	time.Sleep(60 * time.Millisecond)
	assert.False(t, recomputer.isProcessed("evt-x"), "event should expire after TTL")
}
