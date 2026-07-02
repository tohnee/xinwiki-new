package container

import (
	"context"

	"go.uber.org/dig"

	memoryRepo "github.com/Tencent/XinWiki/internal/application/repository/memory/neo4j"
	neo4jRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/neo4j"
	"github.com/Tencent/XinWiki/internal/application/repository"
	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/logger"
)

func registerRepositories(c *dig.Container, ctx context.Context) {
	// Data repositories layer
	logger.Debugf(ctx, "[Container] Registering repositories...")
	must(c.Provide(repository.NewTenantRepository))
	must(c.Provide(repository.NewTenantMemberRepository))
	must(c.Provide(repository.NewTenantInvitationRepository))
	must(c.Provide(repository.NewAuditLogRepository))
	must(c.Provide(repository.NewKnowledgeBaseRepository))
	must(c.Provide(repository.NewKnowledgeRepository))
	must(c.Provide(repository.NewKnowledgeSpanRepository))
	must(c.Provide(repository.NewChunkRepository)) // 自动注入 EventBus 参数（dig 按类型匹配）
	must(c.Provide(repository.NewKnowledgeTagRepository))
	must(c.Provide(repository.NewAPIKeyRepository))   // scoped API keys (review 4.5)
	must(c.Provide(repository.NewArtifactRepository)) // generated artifacts (review 4.2)
	must(c.Provide(repository.NewSessionRepository))
	must(c.Provide(repository.NewMessageRepository))
	must(c.Provide(repository.NewModelRepository))
	must(c.Provide(repository.NewUserRepository))
	must(c.Provide(repository.NewAuthTokenRepository))
	must(c.Provide(repository.NewSystemSettingRepository))
	must(c.Provide(neo4jRepo.NewNeo4jRepository))
	must(c.Provide(memoryRepo.NewMemoryRepository))
	must(c.Provide(repository.NewMCPServiceRepository))
	must(c.Provide(repository.NewMCPToolApprovalRepository))
	must(c.Provide(repository.NewMCPOAuthRepository))
	must(c.Provide(repository.NewCustomAgentRepository))
	must(c.Provide(repository.NewOrganizationRepository))
	must(c.Provide(repository.NewKBShareRepository))
	must(c.Provide(repository.NewAgentShareRepository))
	must(c.Provide(repository.NewEmbedChannelRepository))
	must(c.Provide(repository.NewTenantDisabledSharedAgentRepository))
	must(c.Provide(repository.NewUserResourceFavoriteRepository))
	must(c.Provide(repository.NewUserNoteRepository))
	must(c.Provide(service.NewWebSearchStateService))
	must(c.Provide(repository.NewDataSourceRepository))
	must(c.Provide(repository.NewSyncLogRepository))
	must(c.Provide(repository.NewWikiPageRepository))
	must(c.Provide(repository.NewWikiLogEntryRepository))
	must(c.Provide(repository.NewTaskPendingOpsRepository))
	must(c.Provide(repository.NewTaskDeadLetterRepository))
	must(c.Provide(repository.NewLLMCallLogRepository))
	must(c.Provide(repository.NewConflictRepository))
	must(c.Provide(repository.NewEvaluationRepository))
}
