package container

import (
	"context"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/handler"
	"github.com/Tencent/XinWiki/internal/handler/session"
	imPkg "github.com/Tencent/XinWiki/internal/im"
	"github.com/Tencent/XinWiki/internal/im/dingtalk"
	"github.com/Tencent/XinWiki/internal/im/feishu"
	"github.com/Tencent/XinWiki/internal/im/mattermost"
	"github.com/Tencent/XinWiki/internal/im/slack"
	"github.com/Tencent/XinWiki/internal/im/telegram"
	"github.com/Tencent/XinWiki/internal/im/wechat"
	"github.com/Tencent/XinWiki/internal/im/wecom"
	"github.com/Tencent/XinWiki/internal/logger"
)

func registerHandlers(c *dig.Container, ctx context.Context) {
	// HTTP handlers layer
	logger.Debugf(ctx, "[Container] Registering HTTP handlers...")
	must(c.Provide(handler.NewTenantHandler))
	must(c.Provide(handler.NewTenantMemberHandler))
	must(c.Provide(handler.NewTenantInvitationHandler))
	must(c.Provide(handler.NewAPIKeyHandler))   // scoped API key CRUD (review 4.5)
	must(c.Provide(handler.NewArtifactHandler)) // generated-artifact CRUD (review 4.2)
	must(c.Provide(handler.NewAuditLogHandler))
	must(c.Provide(handler.NewKnowledgeBaseHandler))
	must(c.Provide(handler.NewKnowledgeHandler))
	must(c.Provide(handler.NewChunkHandler))
	must(c.Provide(handler.NewFAQHandler))
	must(c.Provide(handler.NewTagHandler))
	must(c.Provide(session.NewHandler))
	must(c.Provide(handler.NewMessageHandler))
	must(c.Provide(handler.NewModelHandler))
	must(c.Provide(handler.NewEvaluationHandler))
	must(c.Provide(handler.NewInitializationHandler))
	must(c.Provide(handler.NewAuthHandler))
	must(c.Provide(handler.NewSystemHandler))
	must(c.Provide(handler.NewMCPServiceHandler))
	must(c.Provide(handler.NewMCPCredentialsHandler))
	must(c.Provide(handler.NewMCPOAuthHandler))
	must(c.Provide(handler.NewModelCredentialsHandler))
	must(c.Provide(handler.NewWebSearchProviderCredentialsHandler))
	must(c.Provide(handler.NewDataSourceCredentialsHandler))
	must(c.Provide(handler.NewWebSearchHandler))
	must(c.Provide(handler.NewWebSearchProviderHandler))
	must(c.Provide(handler.NewVectorStoreHandler))
	must(c.Provide(handler.NewCustomAgentHandler))
	must(c.Provide(handler.NewUserResourceFavoriteHandler))
	must(c.Provide(handler.NewUserNoteHandler))
	must(c.Provide(service.NewSkillService))
	must(c.Provide(handler.NewSkillHandler))
	must(c.Provide(handler.NewOrganizationHandler))

	// Data source handler
	must(c.Provide(handler.NewDataSourceHandler))
	// Wiki page handler
	must(c.Provide(handler.NewWikiPageHandler))
	// IM integration
	logger.Debugf(ctx, "[Container] Registering IM integration...")
	must(c.Provide(imPkg.NewService))
	must(c.Invoke(registerIMAdapterFactories))
	must(c.Provide(handler.NewIMHandler))
	must(c.Provide(handler.NewEmbedChannelHandler))
	must(c.Provide(handler.NewXinWikiCloudHandler))
	must(c.Provide(handler.NewCostTrackingHandler))
	must(c.Provide(handler.NewConflictDetectionHandler))
	must(c.Provide(handler.NewRAGEvaluationHandler))
	must(c.Provide(handler.NewModelRouterHandler))
	logger.Debugf(ctx, "HTTP handlers registered")
}

// registerIMAdapterFactories registers adapter factories for each IM platform
// and loads enabled channels from the database. Each platform's factory lives
// in its own subpackage to keep this file focused on wiring.
func registerIMAdapterFactories(imService *imPkg.Service) {
	imService.RegisterAdapterFactory("wecom", wecom.NewFactory())
	imService.RegisterAdapterFactory("feishu", feishu.NewFactory())
	imService.RegisterAdapterFactory("slack", slack.NewFactory())
	imService.RegisterAdapterFactory("telegram", telegram.NewFactory())
	imService.RegisterAdapterFactory("dingtalk", dingtalk.NewFactory())
	imService.RegisterAdapterFactory("mattermost", mattermost.NewFactory())
	imService.RegisterAdapterFactory("wechat", wechat.NewFactory())

	// Load and start all enabled channels from database
	if err := imService.LoadAndStartChannels(); err != nil {
		logger.Warnf(context.Background(), "[IM] Failed to load channels from database: %v", err)
	}
}
