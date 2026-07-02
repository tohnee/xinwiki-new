package container

import (
	"context"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/router"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

func registerTaskQueue(c *dig.Container, ctx context.Context, redisAvailable bool) {
	logger.Debugf(ctx, "[Container] Registering task enqueuer...")
	if redisAvailable {
		must(c.Provide(router.NewAsyncqClient, dig.As(new(interfaces.TaskEnqueuer))))
		must(c.Provide(router.NewAsynqServer))
		// Asynq inspector for cancel-by-knowledge-id (best-effort
		// dequeue of pending/scheduled/retry tasks + active-task cancel).
		must(c.Provide(router.NewAsynqInspector))
		must(c.Provide(router.NewAsynqTaskInspector))
	} else {
		syncExec := router.NewSyncTaskExecutor()
		must(c.Provide(func() interfaces.TaskEnqueuer { return syncExec }))
		must(c.Provide(func() *router.SyncTaskExecutor { return syncExec }))
		// Lite mode: no Redis means no asynq inspector. SyncTaskExecutor
		// dispatches inline goroutines that the checkpoint-based abort
		// already handles.
		must(c.Provide(router.NewNoopTaskInspector))
	}
}
