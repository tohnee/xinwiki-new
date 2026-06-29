package metric

import (
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPermissionLeakage_L1User_NoLeakage(t *testing.T) {
	m := NewPermissionLeakageMetric()

	chunkMap := map[string]*types.Chunk{
		"chunk-public": {
			ID:            "chunk-public",
			SecurityLevel: types.SecurityLevelL1,
		},
	}

	results := []*types.SearchResult{
		{ID: "chunk-public", Score: 0.95},
	}

	input := &LeakageTestInput{
		SearchResults:     results,
		ChunkMap:          chunkMap,
		UserSecurityLevel: types.SecurityLevelL1,
		UserID:            "user-1",
	}

	res := m.Compute(input)
	assert.True(t, res.Pass, "L1 user accessing L1 chunk should pass")
	assert.Equal(t, 0, res.LeakedResults)
	assert.Equal(t, 0.0, res.LeakageRate)
}

func TestPermissionLeakage_L1User_HighSecurityChunk_LeakageDetected(t *testing.T) {
	m := NewPermissionLeakageMetric()

	chunkMap := map[string]*types.Chunk{
		"chunk-public": {
			ID:            "chunk-public",
			SecurityLevel: types.SecurityLevelL1,
		},
		"chunk-secret": {
			ID:            "chunk-secret",
			SecurityLevel: types.SecurityLevelL3,
		},
	}

	// 模拟 ACL 过滤失效：L1 用户收到了 L3 chunk
	results := []*types.SearchResult{
		{ID: "chunk-public", Score: 0.95},
		{ID: "chunk-secret", Score: 0.90}, // 泄露！
	}

	input := &LeakageTestInput{
		SearchResults:     results,
		ChunkMap:          chunkMap,
		UserSecurityLevel: types.SecurityLevelL1,
		UserID:            "user-1",
	}

	res := m.Compute(input)
	assert.False(t, res.Pass, "leakage should be detected")
	assert.Equal(t, 1, res.LeakedResults)
	assert.Greater(t, res.LeakageRate, 0.0)
	assert.Contains(t, res.LeakedChunkIDs, "chunk-secret")
}

func TestPermissionLeakage_ACLFilterWorking_NoLeakage(t *testing.T) {
	m := NewPermissionLeakageMetric()

	chunkMap := map[string]*types.Chunk{
		"chunk-public": {
			ID:            "chunk-public",
			SecurityLevel: types.SecurityLevelL1,
		},
		"chunk-secret": {
			ID:            "chunk-secret",
			SecurityLevel: types.SecurityLevelL3,
		},
	}

	// ACL 过滤正常工作：L1 用户只收到 L1 chunk
	results := []*types.SearchResult{
		{ID: "chunk-public", Score: 0.95},
	}

	input := &LeakageTestInput{
		SearchResults:     results,
		ChunkMap:          chunkMap,
		UserSecurityLevel: types.SecurityLevelL1,
		UserID:            "user-1",
	}

	res := m.Compute(input)
	assert.True(t, res.Pass, "properly filtered results should pass")
	assert.Equal(t, 0, res.LeakedResults)
}

func TestPermissionLeakage_UserSpecificACL(t *testing.T) {
	m := NewPermissionLeakageMetric()

	chunkMap := map[string]*types.Chunk{
		"chunk-alice": {
			ID:             "chunk-alice",
			SecurityLevel:  types.SecurityLevelL3,
			AllowedUserIDs: types.StringArray{"alice"},
		},
		"chunk-bob": {
			ID:             "chunk-bob",
			SecurityLevel:  types.SecurityLevelL3,
			AllowedUserIDs: types.StringArray{"bob"},
		},
	}

	// Bob 不应访问 Alice 专属 chunk
	results := []*types.SearchResult{
		{ID: "chunk-bob", Score: 0.95},
		{ID: "chunk-alice", Score: 0.90}, // 泄露！
	}

	input := &LeakageTestInput{
		SearchResults:     results,
		ChunkMap:          chunkMap,
		UserSecurityLevel: types.SecurityLevelL1,
		UserID:            "bob",
	}

	res := m.Compute(input)
	assert.False(t, res.Pass)
	assert.Equal(t, 1, res.LeakedResults)
	assert.Contains(t, res.LeakedChunkIDs, "chunk-alice")
}

func TestPermissionLeakage_CrossTenant(t *testing.T) {
	m := NewPermissionLeakageMetric()

	chunkMap := map[string]*types.Chunk{
		"chunk-tenant-a": {
			ID:       "chunk-tenant-a",
			TenantID: 1,
		},
		"chunk-tenant-b": {
			ID:       "chunk-tenant-b",
			TenantID: 2,
		},
	}

	results := []*types.SearchResult{
		{ID: "chunk-tenant-a", Score: 0.95},
		{ID: "chunk-tenant-b", Score: 0.90}, // 跨租户泄露
	}

	res := m.CheckCrossTenantLeakage(results, chunkMap, 1)
	assert.False(t, res.Pass)
	assert.Equal(t, 1, res.LeakedResults)
	assert.Contains(t, res.LeakedChunkIDs, "chunk-tenant-b")
}

func TestPermissionLeakage_WikiLeakage(t *testing.T) {
	m := NewPermissionLeakageMetric()

	pages := []*types.WikiPage{
		{
			ID:            "wiki-public",
			SecurityLevel: types.SecurityLevelL1,
		},
		{
			ID:            "wiki-secret",
			SecurityLevel: types.SecurityLevelL3,
		},
	}

	// L1 用户不应看到 L3 Wiki
	res := m.CheckWikiLeakage(pages, types.SecurityLevelL1, "user-1", nil)
	assert.False(t, res.Pass)
	assert.Equal(t, 1, res.LeakedResults)
	assert.Contains(t, res.LeakedChunkIDs, "wiki-secret")
}

func TestPermissionLeakage_AssertZeroLeakage_Pass(t *testing.T) {
	m := NewPermissionLeakageMetric()
	res := &LeakageTestResult{
		TotalResults:  5,
		LeakedResults: 0,
		LeakageRate:   0.0,
		Pass:          true,
	}
	err := m.AssertZeroLeakage(res)
	assert.NoError(t, err)
}

func TestPermissionLeakage_AssertZeroLeakage_Fail(t *testing.T) {
	m := NewPermissionLeakageMetric()
	res := &LeakageTestResult{
		TotalResults:   5,
		LeakedResults:  1,
		LeakageRate:    0.2,
		Pass:           false,
		LeakedChunkIDs: []string{"chunk-x"},
	}
	err := m.AssertZeroLeakage(res)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PERMISSION LEAKAGE DETECTED")
}
