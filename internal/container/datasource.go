package container

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/datasource"
	feishuConnector "github.com/Tencent/XinWiki/internal/datasource/connector/feishu"
	notionConnector "github.com/Tencent/XinWiki/internal/datasource/connector/notion"
	rssConnector "github.com/Tencent/XinWiki/internal/datasource/connector/rss"
	yuqueConnector "github.com/Tencent/XinWiki/internal/datasource/connector/yuque"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

func registerDatasource(c *dig.Container, ctx context.Context) {
	// Data source sync framework
	logger.Debugf(ctx, "[Container] Registering data source sync framework...")
	must(c.Provide(initConnectorRegistry))
	must(c.Provide(datasource.NewScheduler))
	must(c.Provide(service.NewDataSourceService))
	must(c.Invoke(startDataSourceScheduler))
	logger.Debugf(ctx, "[Container] Data source sync framework registered")
	must(c.Invoke(startAuditLogRetention))
	logger.Debugf(ctx, "[Container] Audit log retention runner registered")
	must(c.Invoke(startWikiScoreRefresh))
	logger.Debugf(ctx, "[Container] Wiki score refresh runner registered")
	must(c.Invoke(startWikiLifecycle))
	logger.Debugf(ctx, "[Container] Wiki lifecycle runner registered")
	must(c.Provide(service.NewHousekeepingService))
	must(c.Invoke(startHousekeepingService))
	logger.Debugf(ctx, "[Container] Knowledge housekeeping runner registered")
}

// initConnectorRegistry creates and populates the connector registry with all available connectors.
// Aggregates registration errors via errors.Join so a misconfigured or duplicated connector fails
// container initialization loudly instead of silently disabling the feature at runtime.
func initConnectorRegistry() (*datasource.ConnectorRegistry, error) {
	registry := datasource.NewConnectorRegistry()

	var errs error
	if err := registry.Register(feishuConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register feishu connector: %w", err))
	}
	if err := registry.Register(notionConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register notion connector: %w", err))
	}
	if err := registry.Register(yuqueConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register yuque connector: %w", err))
	}
	if err := registry.Register(rssConnector.NewConnector()); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register rss connector: %w", err))
	}

	// Future connectors will be registered here:
	// if err := registry.Register(confluenceConnector.NewConnector()); err != nil { ... }
	// if err := registry.Register(githubConnector.NewConnector()); err != nil { ... }

	if errs != nil {
		return nil, errs
	}
	return registry, nil
}

// startDataSourceScheduler starts the data source cron scheduler and registers cleanup.
func startDataSourceScheduler(scheduler *datasource.Scheduler, cleaner interfaces.ResourceCleaner) {
	if err := scheduler.Start(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Container] data source scheduler start failed: %v", err)
	}

	cleaner.RegisterWithName("DataSourceScheduler", func() error {
		scheduler.Stop()
		return nil
	})
}

// startHousekeepingService starts the knowledge housekeeping cron and registers
// cleanup. This is the safety net that recovers any knowledge stuck in
// "processing" past a configurable threshold (see HousekeepingService for
// rationale). Best-effort: a startup error is logged but does NOT abort the
// container — the rest of the system stays usable.
func startHousekeepingService(svc *service.HousekeepingService, cleaner interfaces.ResourceCleaner) {
	if svc == nil {
		return
	}
	if err := svc.Start(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Container] housekeeping start failed: %v", err)
	}
	cleaner.RegisterWithName("KnowledgeHousekeeping", func() error {
		svc.Stop()
		return nil
	})
}

// startAuditLogRetention spins up the daily audit_logs purge sweep
// and registers shutdown cleanup. Mirrors the data-source-scheduler
// pattern: container init kicks the goroutine, ResourceCleaner stops
// it during graceful shutdown so a SIGTERM during a sweep doesn't
// orphan the goroutine.
//
// retention_days <= 0 is the configured way to disable retention;
// the runner short-circuits Start() on that path so we don't need
// to gate the wiring here.
func startAuditLogRetention(
	runner *service.AuditLogRetentionRunner, cleaner interfaces.ResourceCleaner,
) {
	runner.Start(context.Background())
	cleaner.RegisterWithName("AuditLogRetentionRunner", func() error {
		runner.Stop()
		return nil
	})
}

// startWikiScoreRefresh spins up the daily wiki page score refresh runner
// and registers shutdown cleanup. Runs 15 minutes after startup then daily
// to recalculate confidence, quality, and freshness scores.
func startWikiScoreRefresh(
	runner *service.WikiScoreRefreshRunner, cleaner interfaces.ResourceCleaner,
) {
	runner.Start(context.Background())
	cleaner.RegisterWithName("WikiScoreRefreshRunner", func() error {
		runner.Stop()
		return nil
	})
}

// startWikiLifecycle starts the wiki knowledge lifecycle manager
// (crystallizer/superseder/forgetter) and registers shutdown cleanup.
// Best-effort: a nil manager (e.g. when dependencies aren't wired) is a no-op.
func startWikiLifecycle(
	runner *service.WikiLifecycleManager, cleaner interfaces.ResourceCleaner,
) {
	if runner == nil {
		return
	}
	runner.Start(context.Background())
	cleaner.RegisterWithName("WikiLifecycleManager", func() error {
		runner.Stop()
		return nil
	})
}
