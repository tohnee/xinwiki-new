package acl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// aclRecomputeWikiRepo is the minimal subset of WikiPageRepository that
// ACLRecomputer actually depends on. Using a narrow interface keeps tests
// from needing to implement dozens of unrelated methods.
type aclRecomputeWikiRepo interface {
	ListBySourceRef(ctx context.Context, kbID string, sourceKnowledgeID string) ([]*types.WikiPage, error)
	UpdateMeta(ctx context.Context, page *types.WikiPage) error
}

// ACLRecomputer 监听权限变更事件，自动重算派生 Wiki 的 ACL
type ACLRecomputer struct {
	wikiRepo    aclRecomputeWikiRepo
	cacheSvc    interfaces.SemanticCacheService
	processedMu sync.RWMutex
	processed   map[string]time.Time // 事件ID -> 处理时间，用于幂等去重
	dedupTTL    time.Duration
}

// NewACLRecomputer 创建 ACL 重算订阅者。Accepts any value that provides the
// two repository methods ACLRecomputer needs (ListBySourceRef + UpdateMeta),
// which includes the full WikiPageRepository as well as narrower test doubles.
func NewACLRecomputer(wikiRepo aclRecomputeWikiRepo, cacheSvc interfaces.SemanticCacheService) *ACLRecomputer {
	return &ACLRecomputer{
		wikiRepo:  wikiRepo,
		cacheSvc:  cacheSvc,
		processed: make(map[string]time.Time),
		dedupTTL:  10 * time.Minute,
	}
}

// RegisterSubscribers 注册事件订阅者到 EventBus
func (r *ACLRecomputer) RegisterSubscribers(bus *event.EventBus) {
	bus.On(event.EventPermissionChanged, r.handlePermissionChanged)
	bus.On(event.EventDocumentACLUpdated, r.handlePermissionChanged)
	bus.On(event.EventKBMemberChanged, r.handlePermissionChanged)
}

// handlePermissionChanged 处理权限变更事件
func (r *ACLRecomputer) handlePermissionChanged(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(*event.PermissionChangedData)
	if !ok {
		logger.Errorf(ctx, "[ACLRecomputer] invalid event data type: %T", evt.Data)
		return nil
	}

	// 幂等检查：同一事件不重复处理
	if r.isProcessed(evt.ID) {
		logger.Infof(ctx, "[ACLRecomputer] skipping duplicate event: id=%s, resourceID=%s", evt.ID, data.ResourceID)
		return nil
	}
	r.markProcessed(evt.ID)

	logger.Infof(ctx, "[ACLRecomputer] processing permission change: resourceType=%s, resourceID=%s, oldSL=%s, newSL=%s",
		data.ResourceType, data.ResourceID, data.OldSecurityLevel, data.NewSecurityLevel)

	// 查找引用该来源的所有 Wiki 页面
	wikiPages, err := r.wikiRepo.ListBySourceRef(ctx, data.KBID, data.ResourceID)
	if err != nil {
		logger.Errorf(ctx, "[ACLRecomputer] failed to find wiki pages by source ref: %v", err)
		return err
	}

	if len(wikiPages) == 0 {
		logger.Infof(ctx, "[ACLRecomputer] no wiki pages reference resourceID=%s, skip", data.ResourceID)
		return nil
	}

	// 逐个重算 Wiki ACL
	recomputed := 0
	for _, page := range wikiPages {
		if err := r.recomputeWikiACL(ctx, page, data); err != nil {
			logger.Errorf(ctx, "[ACLRecomputer] failed to recompute ACL for wiki page %s: %v", page.ID, err)
			continue
		}
		recomputed++
	}

	// 失效关联缓存
	if r.cacheSvc != nil {
		if err := r.cacheSvc.InvalidateByKB(ctx, data.TenantID, data.KBID); err != nil {
			logger.Warnf(ctx, "[ACLRecomputer] failed to invalidate cache for kbID=%s: %v", data.KBID, err)
		}
	}

	logger.Infof(ctx, "[ACLRecomputer] ACL recompute complete: resourceID=%s, wikiPagesFound=%d, recomputed=%d",
		data.ResourceID, len(wikiPages), recomputed)

	return nil
}

// recomputeWikiACL 重算单个 Wiki 页面的 ACL
func (r *ACLRecomputer) recomputeWikiACL(ctx context.Context, page *types.WikiPage, triggerData *event.PermissionChangedData) error {
	// 收集所有来源的 ACL
	sources := make([]types.ACLSource, 0, len(page.SourceRefs))
	for _, ref := range page.SourceRefs {
		sourceID := ParseSourceRefUUID(ref)
		if sourceID == "" {
			continue
		}
		// 如果是触发变更的来源，使用新 ACL
		if sourceID == triggerData.ResourceID {
			sources = append(sources, types.ACLSource{
				SecurityLevel:   triggerData.NewSecurityLevel,
				AllowedUserIDs:  triggerData.NewAllowedUserIDs,
				AllowedGroupIDs: triggerData.NewAllowedGroupIDs,
			})
			continue
		}
		// 其他来源使用其当前 ACL（通过 Wiki 自身已存储的来源信息）
		// 注意：完整实现需要从 Chunk 仓库获取每个来源的当前 ACL
		// 这里简化为使用 Wiki 当前 ACL 作为基线
		sources = append(sources, WikiPageToACLSource(page))
	}

	if len(sources) == 0 {
		return fmt.Errorf("no valid sources for wiki page %s", page.ID)
	}

	// 计算派生 ACL
	result, err := CalculateDerivedACL(sources)
	if err != nil {
		return fmt.Errorf("calculate derived ACL: %w", err)
	}

	// 检查是否有变化
	oldSL := page.SecurityLevel
	oldUsers := fmt.Sprintf("%v", page.AllowedUserIDs)
	oldGroups := fmt.Sprintf("%v", page.AllowedGroupIDs)

	// 应用新 ACL
	ApplyDerivedACLToWikiPage(page, result)

	newSL := page.SecurityLevel
	newUsers := fmt.Sprintf("%v", page.AllowedUserIDs)
	newGroups := fmt.Sprintf("%v", page.AllowedGroupIDs)

	// 只有 ACL 实际变化时才写入
	if oldSL == newSL && oldUsers == newUsers && oldGroups == newGroups {
		logger.Infof(ctx, "[ACLRecomputer] wiki page %s ACL unchanged (SL=%s)", page.ID, newSL)
		return nil
	}

	// 持久化更新
	if err := r.wikiRepo.UpdateMeta(ctx, page); err != nil {
		return fmt.Errorf("persist wiki ACL update: %w", err)
	}

	logger.Infof(ctx, "[ACLRecomputer] wiki page %s ACL updated: SL %s→%s, trigger=%s",
		page.ID, oldSL, newSL, triggerData.ResourceID)

	return nil
}

// isProcessed 检查事件是否已处理（幂等）
func (r *ACLRecomputer) isProcessed(eventID string) bool {
	r.processedMu.RLock()
	defer r.processedMu.RUnlock()
	_, exists := r.processed[eventID]
	if !exists {
		return false
	}
	// 检查是否过期
	if time.Since(r.processed[eventID]) > r.dedupTTL {
		return false
	}
	return true
}

// markProcessed 标记事件已处理
func (r *ACLRecomputer) markProcessed(eventID string) {
	r.processedMu.Lock()
	defer r.processedMu.Unlock()
	r.processed[eventID] = time.Now()
	// 清理过期记录
	if len(r.processed) > 1000 {
		cutoff := time.Now().Add(-r.dedupTTL)
		for id, t := range r.processed {
			if t.Before(cutoff) {
				delete(r.processed, id)
			}
		}
	}
}
