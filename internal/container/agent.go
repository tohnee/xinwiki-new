package container

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/acl"
	"github.com/Tencent/XinWiki/internal/agent/approval"
	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/config"
	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

func registerAgent(c *dig.Container, ctx context.Context) {
	// Agent service layer (requires event bus, web search service)
	// SessionService is passed as parameter to CreateAgentEngine method when creating AgentService
	logger.Debugf(ctx, "[Container] Registering event bus and agent service...")
	// ⚠️ 重要: EventBus 必须在 ChunkRepository 之前注册
	// ChunkRepository 依赖 EventBus 发布权限变更事件
	must(c.Provide(event.NewEventBus))

	must(c.Provide(func(cfg *config.Config, s interfaces.MCPToolApprovalService, rdb *redis.Client) *approval.Gate {
		return approval.NewGate(cfg, &approval.Adapter{Svc: s}, rdb)
	}))
	// Expose Gate as MCPApproval interface so AgentService and others can depend on the abstraction.
	must(c.Provide(func(g *approval.Gate) approval.MCPApproval { return g }))
	must(c.Provide(service.NewAgentService))

	// Session service (depends on agent service)
	// SessionService is created after AgentService and passes itself to AgentService.CreateAgentEngine when needed
	logger.Debugf(ctx, "[Container] Registering session service...")
	must(c.Provide(service.NewSessionService))

	// 注册 ACL 重算订阅者到 EventBus 并启动补偿任务
	// 必须在 EventBus 初始化后执行
	must(c.Invoke(func(bus *event.EventBus, recomputer *acl.ACLRecomputer) {
		logger.Debugf(ctx, "[Container] Registering ACL recomputer subscribers...")
		recomputer.RegisterSubscribers(bus)
		logger.Debugf(ctx, "[Container] ACL recomputer subscribers registered")
	}))

	// 启动 ACL 补偿定时任务
	must(c.Invoke(func(reconciler *acl.ACLReconciler) {
		logger.Debugf(ctx, "[Container] Starting ACL reconciler...")
		reconciler.Start(ctx)
		logger.Debugf(ctx, "[Container] ACL reconciler started")
	}))
}
