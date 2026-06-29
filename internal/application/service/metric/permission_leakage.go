package metric

import (
	"fmt"

	"github.com/Tencent/XinWiki/internal/acl"
	"github.com/Tencent/XinWiki/internal/types"
)

// PermissionLeakageMetric 权限泄露评测指标
// 检测无权限用户是否能访问高密级 Chunk/Wiki
// CI 红线：Permission Leakage Rate = 0
type PermissionLeakageMetric struct{}

// NewPermissionLeakageMetric 创建权限泄露评测器
func NewPermissionLeakageMetric() *PermissionLeakageMetric {
	return &PermissionLeakageMetric{}
}

// LeakageTestInput 权限泄露测试输入
type LeakageTestInput struct {
	// 检索结果（含所有密级的 Chunk）
	SearchResults []*types.SearchResult
	// Chunk 映射表
	ChunkMap map[string]*types.Chunk
	// 测试用户的密级
	UserSecurityLevel string
	// 测试用户 ID
	UserID string
	// 测试用户所属组
	UserGroupIDs []string
}

// LeakageTestResult 权限泄露测试结果
type LeakageTestResult struct {
	TotalResults      int     // 检索结果总数
	AccessibleResults int     // 用户有权访问的结果数
	LeakedResults     int     // 泄露的结果数（无权限但未被过滤）
	LeakageRate       float64 // 泄露率 = LeakedResults / TotalResults
	LeakedChunkIDs    []string // 泄露的 Chunk ID 列表
	Pass              bool    // 是否通过（LeakageRate == 0）
}

// Compute 计算权限泄露率
// 返回 0.0 表示无泄露（通过），>0.0 表示存在泄露
func (m *PermissionLeakageMetric) Compute(input *LeakageTestInput) *LeakageTestResult {
	result := &LeakageTestResult{
		TotalResults: len(input.SearchResults),
	}

	for _, sr := range input.SearchResults {
		chunk, ok := input.ChunkMap[sr.ID]
		if !ok {
			continue
		}

		if acl.UserCanAccessChunk(chunk, input.UserSecurityLevel, input.UserID, input.UserGroupIDs) {
			result.AccessibleResults++
		} else {
			// 无权限但出现在结果中 = 泄露
			result.LeakedResults++
			result.LeakedChunkIDs = append(result.LeakedChunkIDs, sr.ID)
		}
	}

	if result.TotalResults > 0 {
		result.LeakageRate = float64(result.LeakedResults) / float64(result.TotalResults)
	}

	result.Pass = result.LeakageRate == 0.0
	return result
}

// CheckWikiLeakage 检查 Wiki 页面是否存在权限泄露
// 无权限用户不应看到高密级 Wiki 的标题或内容
func (m *PermissionLeakageMetric) CheckWikiLeakage(pages []*types.WikiPage, userSecurityLevel string, userID string, userGroupIDs []string) *LeakageTestResult {
	result := &LeakageTestResult{
		TotalResults: len(pages),
	}

	for _, page := range pages {
		if acl.UserCanAccessWikiPage(page, userSecurityLevel, userID, userGroupIDs) {
			result.AccessibleResults++
		} else {
			result.LeakedResults++
			result.LeakedChunkIDs = append(result.LeakedChunkIDs, page.ID)
		}
	}

	if result.TotalResults > 0 {
		result.LeakageRate = float64(result.LeakedResults) / float64(result.TotalResults)
	}

	result.Pass = result.LeakageRate == 0.0
	return result
}

// CheckCrossTenantLeakage 检查跨租户泄露
// 用户 A 的检索结果不应包含租户 B 的 Chunk
func (m *PermissionLeakageMetric) CheckCrossTenantLeakage(results []*types.SearchResult, chunkMap map[string]*types.Chunk, expectedTenantID uint64) *LeakageTestResult {
	result := &LeakageTestResult{
		TotalResults: len(results),
	}

	for _, sr := range results {
		chunk, ok := chunkMap[sr.ID]
		if !ok {
			continue
		}
		if chunk.TenantID != expectedTenantID {
			result.LeakedResults++
			result.LeakedChunkIDs = append(result.LeakedChunkIDs, sr.ID)
		} else {
			result.AccessibleResults++
		}
	}

	if result.TotalResults > 0 {
		result.LeakageRate = float64(result.LeakedResults) / float64(result.TotalResults)
	}

	result.Pass = result.LeakageRate == 0.0
	return result
}

// AssertZeroLeakage 断言泄露率为0，否则返回错误（用于 CI 红线）
func (m *PermissionLeakageMetric) AssertZeroLeakage(result *LeakageTestResult) error {
	if !result.Pass {
		return fmt.Errorf(
			"PERMISSION LEAKAGE DETECTED: rate=%.4f, leaked=%d/%d, chunkIDs=%v",
			result.LeakageRate, result.LeakedResults, result.TotalResults, result.LeakedChunkIDs,
		)
	}
	return nil
}
