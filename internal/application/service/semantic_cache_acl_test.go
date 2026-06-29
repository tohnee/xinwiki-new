package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Tencent/XinWiki/internal/acl"
	"github.com/Tencent/XinWiki/internal/types"
)
func TestSemanticCache_ACLFiltering(t *testing.T) {
	chunkMap := map[string]*types.Chunk{
		"chunk-public": {
			ID:            "chunk-public",
			Content:       "公开内容 - 所有用户可见",
			SecurityLevel: types.SecurityLevelL1,
		},
		"chunk-internal": {
			ID:            "chunk-internal",
			Content:       "内部内容 - L2及以上可见",
			SecurityLevel: types.SecurityLevelL2,
		},
		"chunk-secret": {
			ID:            "chunk-secret",
			Content:       "机密内容 - L3及以上可见",
			SecurityLevel: types.SecurityLevelL3,
		},
		"chunk-user-alice": {
			ID:             "chunk-user-alice",
			Content:        "用户专属内容 - 仅Alice可见",
			SecurityLevel:  types.SecurityLevelL3,
			AllowedUserIDs: []string{"alice-001"},
		},
		"chunk-group-eng": {
			ID:              "chunk-group-eng",
			Content:         "研发组专属内容 - 仅研发组可见",
			SecurityLevel:   types.SecurityLevelL3,
			AllowedGroupIDs: []string{"group-engineering"},
		},
	}

	results := []*types.SearchResult{
		{ID: "chunk-public", Score: 0.95},
		{ID: "chunk-internal", Score: 0.90},
		{ID: "chunk-secret", Score: 0.85},
		{ID: "chunk-user-alice", Score: 0.80},
		{ID: "chunk-group-eng", Score: 0.75},
	}

	t.Run("L1_public_user_only_sees_public", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "bob-001", []string{})
		assert.Len(t, filtered, 1)
		assert.Equal(t, "chunk-public", filtered[0].ID)
	})

	t.Run("L2_internal_user_sees_public_and_internal", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL2, "bob-001", []string{})
		assert.Len(t, filtered, 2)
		ids := []string{filtered[0].ID, filtered[1].ID}
		assert.Contains(t, ids, "chunk-public")
		assert.Contains(t, ids, "chunk-internal")
	})

	t.Run("L3_user_sees_all_security_levels", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL3, "bob-001", []string{})
		assert.Len(t, filtered, 3)
	})

	t.Run("alice_sees_her_private_chunk", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "alice-001", []string{})
		assert.Len(t, filtered, 2)
		ids := []string{filtered[0].ID, filtered[1].ID}
		assert.Contains(t, ids, "chunk-public")
		assert.Contains(t, ids, "chunk-user-alice")
	})

	t.Run("engineering_group_member_sees_group_chunk", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "bob-001", []string{"group-engineering"})
		assert.Len(t, filtered, 2)
		ids := []string{filtered[0].ID, filtered[1].ID}
		assert.Contains(t, ids, "chunk-public")
		assert.Contains(t, ids, "chunk-group-eng")
	})

	t.Run("alice_in_eng_group_sees_all", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL1, "alice-001", []string{"group-engineering"})
		assert.Len(t, filtered, 3)
		ids := []string{filtered[0].ID, filtered[1].ID, filtered[2].ID}
		assert.Contains(t, ids, "chunk-public")
		assert.Contains(t, ids, "chunk-user-alice")
		assert.Contains(t, ids, "chunk-group-eng")
	})

	t.Run("L4_admin_sees_everything", func(t *testing.T) {
		filtered := acl.FilterSearchResultChunksByACL(results, chunkMap, types.SecurityLevelL4, "admin", []string{})
		assert.Len(t, filtered, 5)
	})
}

func TestSemanticCache_CacheHitACLReFilter(t *testing.T) {
	ctx := context.Background()
	cfg := types.DefaultSemanticCacheConfig()
	cache := NewMemorySemanticCache(cfg)

	tenantID := uint64(1)
	kbIDs := []string{"kb-001"}
	embedding := []float32{1.0, 0.0, 0.0}

	chunkMap := map[string]*types.Chunk{
		"chunk-public":   {ID: "chunk-public", SecurityLevel: types.SecurityLevelL1},
		"chunk-internal": {ID: "chunk-internal", SecurityLevel: types.SecurityLevelL2},
		"chunk-secret":   {ID: "chunk-secret", SecurityLevel: types.SecurityLevelL3},
	}

	allResults := []*types.SearchResult{
		{ID: "chunk-public", Score: 0.95},
		{ID: "chunk-internal", Score: 0.90},
		{ID: "chunk-secret", Score: 0.85},
	}

	entry := &types.SemanticCacheEntry{
		TenantID:         tenantID,
		KnowledgeBaseIDs: kbIDs,
		QueryText:        "测试查询",
		QueryEmbedding:   embedding,
		Results:          allResults,
		ChunkMap:         chunkMap,
	}
	err := cache.Set(ctx, entry)
	assert.NoError(t, err)

	t.Run("L1_user_gets_only_public_from_cache", func(t *testing.T) {
		cached, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
		assert.NoError(t, err)
		assert.NotNil(t, cached)

		filtered := acl.FilterSearchResultChunksByACL(cached.Results, cached.ChunkMap, types.SecurityLevelL1, "user-001", []string{})
		assert.Len(t, filtered, 1)
		assert.Equal(t, "chunk-public", filtered[0].ID)
	})

	t.Run("L3_user_gets_all_from_cache", func(t *testing.T) {
		cached, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
		assert.NoError(t, err)
		assert.NotNil(t, cached)

		filtered := acl.FilterSearchResultChunksByACL(cached.Results, cached.ChunkMap, types.SecurityLevelL3, "admin-001", []string{})
		assert.Len(t, filtered, 3)
	})

	t.Run("cache_hit_count_increments_correctly", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			_, err := cache.Get(ctx, tenantID, kbIDs, embedding, 0.9)
			assert.NoError(t, err)
		}

		stats, err := cache.Stats(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(7), stats.TotalHits)
	})
}
