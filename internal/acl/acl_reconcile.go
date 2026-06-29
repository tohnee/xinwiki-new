package acl

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// ACLReconciler 定时补偿任务，扫描 Wiki ACL 与来源 ACL 一致性
// 防止事件丢失导致长期权限不一致
type ACLReconciler struct {
	wikiRepo    interfaces.WikiPageRepository
	chunkRepo   interfaces.ChunkRepository
	interval    time.Duration
	batchSize   int
	stopCh      chan struct{}
}

// NewACLReconciler 创建定时补偿任务
func NewACLReconciler(wikiRepo interfaces.WikiPageRepository, chunkRepo interfaces.ChunkRepository) *ACLReconciler {
	return &ACLReconciler{
		wikiRepo:  wikiRepo,
		chunkRepo: chunkRepo,
		interval:  6 * time.Hour, // 每6小时执行一次
		batchSize: 100,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动定时补偿任务
func (r *ACLReconciler) Start(ctx context.Context) {
	go r.run(ctx)
	logger.Infof(ctx, "[ACLReconciler] started, interval=%v", r.interval)
}

// Stop 停止定时补偿任务
func (r *ACLReconciler) Stop() {
	close(r.stopCh)
}

func (r *ACLReconciler) run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			logger.Infof(ctx, "[ACLReconciler] stopped")
			return
		case <-ctx.Done():
			logger.Infof(ctx, "[ACLReconciler] context cancelled")
			return
		case <-ticker.C:
			r.reconcileBatch(ctx)
		}
	}
}

// reconcileBatch 执行一轮补偿扫描
func (r *ACLReconciler) reconcileBatch(ctx context.Context) {
	// 遍历所有 KB 的 Wiki 页面，检查 ACL 一致性
	// 注意：完整实现需要 KB 列表，这里简化为按 cursor 扫描
	cursor := ""
	totalChecked := 0
	totalFixed := 0

	for {
		pages, nextCursor, err := r.wikiRepo.ListPagesCursor(ctx, "", cursor, r.batchSize)
		if err != nil {
			logger.Errorf(ctx, "[ACLReconciler] failed to list pages: %v", err)
			break
		}

		for _, page := range pages {
			totalChecked++
			if r.checkAndFix(ctx, page) {
				totalFixed++
			}
		}

		if nextCursor == "" || len(pages) < r.batchSize {
			break
		}
		cursor = nextCursor
	}

	if totalFixed > 0 {
		logger.Infof(ctx, "[ACLReconciler] reconcile complete: checked=%d, fixed=%d", totalChecked, totalFixed)
	} else {
		logger.Infof(ctx, "[ACLReconciler] reconcile complete: checked=%d, all consistent", totalChecked)
	}
}

// checkAndFix 检查单个 Wiki 页面的 ACL 一致性，不一致则修复
// 返回 true 表示进行了修复
func (r *ACLReconciler) checkAndFix(ctx context.Context, page *types.WikiPage) bool {
	if len(page.SourceRefs) == 0 {
		return false
	}

	// 收集所有来源的当前 ACL
	sources := make([]types.ACLSource, 0, len(page.SourceRefs))
	for _, ref := range page.SourceRefs {
		sourceID := ParseSourceRefUUID(ref)
		if sourceID == "" {
			continue
		}
		// 从 Chunk 仓库获取来源的当前 ACL
		chunk, err := r.chunkRepo.GetChunkByIDOnly(ctx, sourceID)
		if err != nil || chunk == nil {
			// 来源不存在或已删除，跳过
			continue
		}
		sources = append(sources, ChunkToACLSource(chunk))
	}

	if len(sources) == 0 {
		return false
	}

	// 计算应该的 ACL
	expected, err := CalculateDerivedACL(sources)
	if err != nil {
		logger.Errorf(ctx, "[ACLReconciler] failed to calculate expected ACL for page %s: %v", page.ID, err)
		return false
	}

	// 检查是否一致
	if page.SecurityLevel == expected.SecurityLevel &&
		stringSliceEqual(page.AllowedUserIDs, expected.AllowedUserIDs) &&
		stringSliceEqual(page.AllowedGroupIDs, expected.AllowedGroupIDs) {
		return false // 一致，无需修复
	}

	// 不一致，修复
	oldSL := page.SecurityLevel
	ApplyDerivedACLToWikiPage(page, expected)

	if err := r.wikiRepo.UpdateMeta(ctx, page); err != nil {
		logger.Errorf(ctx, "[ACLReconciler] failed to persist ACL fix for page %s: %v", page.ID, err)
		return false
	}

	logger.Warnf(ctx, "[ACLReconciler] *** ACL FIXED *** pageID=%s, SL %s→%s (reconcile)",
		page.ID, oldSL, expected.SecurityLevel)
	return true
}

// stringSliceEqual 比较两个字符串切片是否相等
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
