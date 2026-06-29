package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// WikiScoreRefreshRunner periodically refreshes confidence, quality, and freshness
// scores for all wiki pages across all wiki-enabled knowledge bases. It runs daily
// to ensure scores stay up-to-date with recency and usage patterns.
//
// Uses a simple time.Ticker pattern consistent with other background runners
// (see AuditLogRetentionRunner) — no external cron library needed for daily tasks.
type WikiScoreRefreshRunner struct {
	wikiSvc interfaces.WikiPageService
	kbSvc   interfaces.KnowledgeBaseService
	interval time.Duration

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
	started   atomic.Bool
}

const (
	// wikiScoreRefreshInterval is how often scores are recalculated.
	// Daily is sufficient since freshness changes slowly over days/weeks.
	wikiScoreRefreshInterval = 24 * time.Hour

	// wikiScoreRefreshStartupDelay waits after boot before the first run,
	// avoiding contention with startup migrations and initial request load.
	wikiScoreRefreshStartupDelay = 15 * time.Minute
)

// NewWikiScoreRefreshRunner creates the score refresh runner.
// Constructor only sets up wiring; nothing runs until Start is called.
func NewWikiScoreRefreshRunner(
	wikiSvc interfaces.WikiPageService,
	kbSvc interfaces.KnowledgeBaseService,
) *WikiScoreRefreshRunner {
	return &WikiScoreRefreshRunner{
		wikiSvc:  wikiSvc,
		kbSvc:    kbSvc,
		interval: wikiScoreRefreshInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start launches the background goroutine. Safe to call multiple times (sync.Once).
func (r *WikiScoreRefreshRunner) Start(ctx context.Context) {
	if r == nil || r.wikiSvc == nil || r.kbSvc == nil {
		return
	}
	r.startOnce.Do(func() {
		r.started.Store(true)
		logger.Infof(ctx,
			"[wiki-score-refresh] starting daily score refresh: interval=%s",
			r.interval)
		go r.loop()
	})
}

// Stop signals the loop to exit and waits for it to complete. Idempotent.
func (r *WikiScoreRefreshRunner) Stop() {
	if r == nil {
		return
	}
	if !r.started.Load() {
		return
	}
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
	<-r.doneCh
}

// loop runs the refresh on schedule.
func (r *WikiScoreRefreshRunner) loop() {
	defer close(r.doneCh)

	startupTimer := time.NewTimer(wikiScoreRefreshStartupDelay)
	defer startupTimer.Stop()
	select {
	case <-startupTimer.C:
	case <-r.stopCh:
		return
	}

	r.runOnce()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.runOnce()
		case <-r.stopCh:
			return
		}
	}
}

// runOnce performs a single score refresh across all wiki-enabled knowledge bases.
// Errors are logged per-KB but don't stop the whole run; we'll try again tomorrow.
func (r *WikiScoreRefreshRunner) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// List all knowledge bases
	kbs, err := r.kbSvc.ListKnowledgeBases(ctx)
	if err != nil {
		logger.Warnf(ctx, "[wiki-score-refresh] failed to list KBs: %v", err)
		return
	}

	totalRefreshed := 0
	for _, kb := range kbs {
		// Skip non-wiki KBs
		if !kb.IsWikiEnabled() {
			continue
		}
		select {
		case <-r.stopCh:
			logger.Infof(ctx, "[wiki-score-refresh] stopped during run, refreshed=%d", totalRefreshed)
			return
		default:
		}

		refreshed, err := r.wikiSvc.RefreshAllScores(ctx, kb.ID)
		if err != nil {
			logger.Warnf(ctx,
				"[wiki-score-refresh] failed to refresh KB %s: %v", kb.ID, err)
			continue
		}
		totalRefreshed += refreshed
		logger.Debugf(ctx,
			"[wiki-score-refresh] refreshed KB %s: pages=%d", kb.ID, refreshed)
	}

	logger.Infof(ctx,
		"[wiki-score-refresh] refresh complete: total_kbs=%d total_pages=%d",
		len(kbs), totalRefreshed)
}
