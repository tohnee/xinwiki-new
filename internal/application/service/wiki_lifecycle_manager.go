package service

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/Tencent/XinWiki/internal/wikiquality"
)

// WikiLifecycleManager runs periodic background maintenance on wiki pages:
//   - Crystallizer: promotes high-signal, heavily-used, expert-validated pages
//     so their scores/freshness stabilize.
//   - Superseder: detects near-duplicate / overlapping pages and marks stale
//     ones as superseded so retrieval deprioritizes them.
//   - Forgetter: archives long-stale low-criticality pages (delegates the
//     final decision to wikiquality.ShouldAutoArchive).
//
// Design notes:
//   - LLM-driven when WIKI_LIFECYCLE_MODEL is set; otherwise runs deterministic
//     heuristics only. LLM calls are best-effort and never block scoring.
//   - The runner mirrors WikiScoreRefreshRunner's pattern (start/stop/once
//     semantics, startup delay, daily interval).
//   - All status writes go through WikiPageService.UpdatePageMeta so version
//     is not bumped on machine-only transitions.
type WikiLifecycleManager struct {
	wikiSvc interfaces.WikiPageService
	kbSvc   interfaces.KnowledgeBaseService
	modelSvc interfaces.ModelService

	modelID string
	interval time.Duration

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
	started   atomic.Bool
}

const (
	wikiLifecycleInterval     = 24 * time.Hour
	wikiLifecycleStartupDelay = 30 * time.Minute
)

// LifecycleManagerParams groups dependencies for DI. It uses dig.In so callers
// only need to `Provide(NewWikiLifecycleManager)` and uber/dig wires it up.
type LifecycleManagerParams struct {
	dig.In

	WikiSvc  interfaces.WikiPageService
	KbSvc    interfaces.KnowledgeBaseService
	ModelSvc interfaces.ModelService `optional:"true"`
}

// NewWikiLifecycleManager constructs the lifecycle manager. Reads
// WIKI_LIFECYCLE_MODEL from the environment; if empty the manager still runs
// deterministic-only passes.
func NewWikiLifecycleManager(p LifecycleManagerParams) *WikiLifecycleManager {
	modelID := strings.TrimSpace(os.Getenv("WIKI_LIFECYCLE_MODEL"))
	return &WikiLifecycleManager{
		wikiSvc:  p.WikiSvc,
		kbSvc:    p.KbSvc,
		modelSvc: p.ModelSvc,
		modelID:  modelID,
		interval: wikiLifecycleInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start launches the background goroutine. Idempotent.
func (m *WikiLifecycleManager) Start(ctx context.Context) {
	if m == nil || m.wikiSvc == nil || m.kbSvc == nil {
		return
	}
	m.startOnce.Do(func() {
		m.started.Store(true)
		logger.Infof(ctx,
			"[wiki-lifecycle] starting daily lifecycle runner: model=%q interval=%s",
			m.modelID, m.interval)
		go m.loop()
	})
}

// Stop signals the loop to exit and waits for completion. Idempotent.
func (m *WikiLifecycleManager) Stop() {
	if m == nil {
		return
	}
	if !m.started.Load() {
		return
	}
	m.stopOnce.Do(func() { close(m.stopCh) })
	<-m.doneCh
}

func (m *WikiLifecycleManager) loop() {
	defer close(m.doneCh)

	startupTimer := time.NewTimer(wikiLifecycleStartupDelay)
	defer startupTimer.Stop()
	select {
	case <-startupTimer.C:
	case <-m.stopCh:
		return
	}

	m.runOnce()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.runOnce()
		case <-m.stopCh:
			return
		}
	}
}

func (m *WikiLifecycleManager) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	kbs, err := m.kbSvc.ListKnowledgeBases(ctx)
	if err != nil {
		logger.Warnf(ctx, "[wiki-lifecycle] failed to list KBs: %v", err)
		return
	}

	var (
		totalCrystallized int
		totalSuperseded   int
		totalForgotten    int
	)
	now := time.Now()
	for _, kb := range kbs {
		if !kb.IsWikiEnabled() {
			continue
		}
		select {
		case <-m.stopCh:
			logger.Infof(ctx, "[wiki-lifecycle] stopped during run")
			return
		default:
		}

		pages, err := m.wikiSvc.ListAllPages(ctx, kb.ID)
		if err != nil {
			logger.Warnf(ctx, "[wiki-lifecycle] failed to list pages for KB %s: %v", kb.ID, err)
			continue
		}

		c, s, f := m.processKB(ctx, kb.ID, pages, now)
		totalCrystallized += c
		totalSuperseded += s
		totalForgotten += f
	}

	logger.Infof(ctx,
		"[wiki-lifecycle] run complete: kbs=%d crystallized=%d superseded=%d forgotten=%d",
		len(kbs), totalCrystallized, totalSuperseded, totalForgotten)
}

// processKB runs all three lifecycle passes on one KB's pages.
func (m *WikiLifecycleManager) processKB(
	ctx context.Context, kbID string, pages []*types.WikiPage, now time.Time,
) (crystallized, superseded, forgotten int) {
	if len(pages) == 0 {
		return 0, 0, 0
	}

	// 1. Forgetter: archive stale low-criticality pages.
	for _, p := range pages {
		if p == nil {
			continue
		}
		if wikiquality.ShouldAutoArchive(p, now) && p.Status != types.WikiPageStatusArchived {
			p.Status = types.WikiPageStatusArchived
			if err := m.wikiSvc.UpdatePageMeta(ctx, p); err != nil {
				logger.Warnf(ctx, "[wiki-lifecycle] forgetter failed for page %s/%s: %v", kbID, p.Slug, err)
				continue
			}
			forgotten++
			logger.Debugf(ctx, "[wiki-lifecycle] archived stale page %s/%s", kbID, p.Slug)
		}
	}

	// 2. Crystallizer: mark expert-validated, highly-used pages as stable by
	// boosting freshness so they resist cold/archive drift.
	for _, p := range pages {
		if p == nil || p.Status == types.WikiPageStatusArchived {
			continue
		}
		crystallized += m.crystallize(ctx, p, now)
	}

	// 3. Superseder: look for duplicate titles/aliases and mark the weaker
	// (lower final score, fewer views) pages as superseded.
	superseded += m.supersedeDuplicates(ctx, kbID, pages, now)

	return crystallized, superseded, forgotten
}

// crystallize boosts the freshness state of pages that are clearly stable
// knowledge: expert-validated, high final score, decent usage. This keeps
// "golden" pages warm even if edits are rare.
func (m *WikiLifecycleManager) crystallize(ctx context.Context, p *types.WikiPage, now time.Time) int {
	if p.ExpertValidated && p.FinalScore >= 0.75 && p.ViewCount >= 10 {
		// Treat as recently accessed so freshness stays Active.
		if p.LastAccessedAt.Before(now.Add(-7 * 24 * time.Hour)) {
			p.LastAccessedAt = now.Add(-24 * time.Hour)
			wikiquality.UpdateAllScores(p, now)
			if err := m.wikiSvc.UpdatePageMeta(ctx, p); err != nil {
				logger.Warnf(ctx, "[wiki-lifecycle] crystallize update failed for %s: %v", p.ID, err)
				return 0
			}
			return 1
		}
	}
	return 0
}

// supersedeDuplicates is a pragmatic near-duplicate detector keyed on
// normalized title/alias. When two live pages share a title (or alias set),
// the weaker one (lower score/view count, older update) is marked superseded
// pointing at the stronger. It does NOT delete content; it just changes
// status so retrieval deprioritizes the page.
func (m *WikiLifecycleManager) supersedeDuplicates(
	ctx context.Context, kbID string, pages []*types.WikiPage, now time.Time,
) int {
	byKey := make(map[string]*types.WikiPage)
	changed := 0
	for _, p := range pages {
		if p == nil {
			continue
		}
		if p.Status == types.WikiPageStatusArchived || p.Status == types.WikiPageStatusSuperseded {
			continue
		}
		keys := candidateKeys(p)
		for _, k := range keys {
			existing, ok := byKey[k]
			if !ok {
				byKey[k] = p
				continue
			}
			// Pick the stronger page as the canonical target.
			stronger, weaker := chooseStronger(existing, p)
			if weaker == nil || weaker.ID == stronger.ID {
				continue
			}
			// Skip if the weaker page is P0/P1 critical.
			crit := types.NormalizeCriticalityLevel(weaker.CriticalityLevel)
			if crit == types.CriticalityP0 || crit == types.CriticalityP1 {
				continue
			}
			// Don't supersede very recent edits (< 48h) — they might be intentional.
			if now.Sub(weaker.UpdatedAt) < 48*time.Hour {
				continue
			}
			weaker.Status = types.WikiPageStatusSuperseded
			wikiquality.UpdateAllScores(weaker, now)
			if err := m.wikiSvc.UpdatePageMeta(ctx, weaker); err != nil {
				logger.Warnf(ctx, "[wiki-lifecycle] supersede update failed for %s: %v", weaker.ID, err)
				continue
			}
			_, sErr := m.wikiSvc.Supersede(ctx, kbID, &types.WikiSupersedeRequest{
				OldPageSlug: weaker.Slug,
				NewPageSlug: stronger.Slug,
				Reason:      "lifecycle-manager: duplicate title/alias detected",
			})
			if sErr != nil {
				logger.Debugf(ctx, "[wiki-lifecycle] supersede link creation failed (non-fatal) for %s->%s: %v",
					weaker.Slug, stronger.Slug, sErr)
			}
			changed++
			byKey[k] = stronger
		}
	}
	return changed
}

// Chat returns a chat model if one is configured; nil otherwise.
func (m *WikiLifecycleManager) chat(ctx context.Context) chat.Chat {
	if m == nil || m.modelID == "" || m.modelSvc == nil {
		return nil
	}
	c, err := m.modelSvc.GetChatModel(ctx, m.modelID)
	if err != nil {
		logger.Warnf(ctx, "[wiki-lifecycle] failed to resolve lifecycle model %q: %v", m.modelID, err)
		return nil
	}
	return c
}

// candidateKeys returns normalized duplicate-detection keys for a page.
func candidateKeys(p *types.WikiPage) []string {
	keys := make([]string, 0, 1+len(p.Aliases))
	keys = append(keys, normalizeTitle(p.Title))
	for _, a := range p.Aliases {
		if n := normalizeTitle(a); n != "" {
			keys = append(keys, n)
		}
	}
	return keys
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "the ")
	s = strings.NewReplacer("(", ")", ")", "（", "（", "）", "）", "-", " ", "_", " ", "/", " ").Replace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// chooseStronger returns (stronger, weaker) based on FinalScore, then view
// count, then recency. It never returns nil for stronger; weaker is nil if
// the pages are effectively equal.
func chooseStronger(a, b *types.WikiPage) (*types.WikiPage, *types.WikiPage) {
	if a == nil {
		return b, nil
	}
	if b == nil {
		return a, nil
	}
	score := func(p *types.WikiPage) float64 {
		s := p.FinalScore
		if s <= 0 {
			s = 0.5
		}
		return s
	}
	sa, sb := score(a), score(b)
	if sa != sb {
		if sa > sb {
			return a, b
		}
		return b, a
	}
	if a.ViewCount != b.ViewCount {
		if a.ViewCount > b.ViewCount {
			return a, b
		}
		return b, a
	}
	if a.UpdatedAt.After(b.UpdatedAt) {
		return a, b
	}
	if b.UpdatedAt.After(a.UpdatedAt) {
		return b, a
	}
	return a, nil
}

// LifecycleModel returns the configured lifecycle model ID (used by tests/diag).
func (m *WikiLifecycleManager) LifecycleModel() string {
	if m == nil {
		return ""
	}
	return m.modelID
}
