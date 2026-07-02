package container

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/acl"
	"github.com/Tencent/XinWiki/internal/application/repository"
	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/artifact"
	"github.com/Tencent/XinWiki/internal/artifact/generator"
	memoryService "github.com/Tencent/XinWiki/internal/application/service/memory"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/embedding"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

func registerServices(c *dig.Container, ctx context.Context) {
	// Business service layer
	logger.Debugf(ctx, "[Container] Registering business services...")
	must(c.Provide(service.NewTenantService))
	must(c.Provide(service.NewTenantMemberService))
	must(c.Provide(service.NewTenantInvitationService))
	must(c.Provide(service.NewAPIKeyService))   // scoped API key CRUD (review 4.5)
	must(c.Provide(service.NewArtifactService)) // generated-artifact CRUD + ACL (review 4.2)
	must(c.Provide(service.NewAuditLogService))
	must(c.Provide(service.NewAuditLogRetentionRunner))
	must(c.Provide(service.NewWikiScoreRefreshRunner))
	must(c.Provide(service.NewWikiLifecycleManager))
	must(c.Provide(service.NewKnowledgeBaseService))
	must(c.Provide(service.NewOrganizationService))
	must(c.Provide(service.NewKBShareService)) // KBShareService must be registered before KnowledgeService and KnowledgeTagService
	must(c.Provide(service.NewAgentShareService))
	must(c.Provide(service.NewKnowledgeService))
	must(c.Provide(service.NewSpanTracker))
	must(c.Provide(service.NewChunkService))
	must(c.Provide(service.NewKnowledgeTagService))
	must(c.Provide(embedding.NewBatchEmbedder))
	must(c.Provide(service.NewModelService))
	must(c.Provide(service.NewCostTrackingService, dig.As(new(interfaces.CostTrackingService))))
	must(c.Provide(service.NewModelRouterService, dig.As(new(interfaces.ModelRouterService))))
	must(c.Provide(service.NewPromptTemplateService, dig.As(new(interfaces.PromptTemplateService))))
	// DB-backed prompt-template persistence. Replaces the in-memory
	// process singleton that previously held templates in RAM only (so any
	// edit via the UI/API was lost on restart and was invisible to other
	// replicas). When NewPromptTemplateService receives a non-nil repository
	// every CRUD method flows through the DB; the in-memory fallback stays
	// available only for unit tests / Lite no-DB boot (see service.go).
	must(c.Provide(repository.NewPromptTemplateRepository, dig.As(new(interfaces.PromptTemplateRepository))))
	must(c.Provide(service.NewConflictDetectionService, dig.As(new(interfaces.ConflictDetectionService))))
	must(c.Provide(service.NewRAGEvaluationService, dig.As(new(interfaces.RAGEvaluationService))))
	must(c.Provide(initSemanticCacheService, dig.As(new(interfaces.SemanticCacheService))))
	must(c.Provide(acl.NewACLRecomputer))
	must(c.Provide(acl.NewACLReconciler))
	must(c.Provide(initEmbeddingBatcher))
	must(c.Provide(service.NewDatasetService))
	must(c.Provide(service.NewEvaluationService))
	must(c.Provide(service.NewUserService))
	must(c.Provide(service.NewSystemSettingService))
	must(c.Provide(service.NewXinWikiCloudService))

	// Extract services - register individual extracters with names
	must(c.Provide(service.NewChunkExtractService, dig.Name("chunkExtractor")))
	must(c.Provide(service.NewDataTableSummaryService, dig.Name("dataTableSummary")))
	must(c.Provide(service.NewImageMultimodalService, dig.Name("imageMultimodal")))
	must(c.Provide(service.NewKnowledgePostProcessService, dig.Name("knowledgePostProcess")))

	must(c.Provide(service.NewMessageService))
	must(c.Provide(service.NewMCPServiceService))
	must(c.Provide(service.NewMCPToolApprovalService))
	must(c.Provide(service.NewCustomAgentService))
	must(c.Provide(service.NewUserResourceFavoriteService))
	must(c.Provide(memoryService.NewMemoryService))
	must(c.Provide(service.NewWikiPageService))
	must(c.Provide(service.NewWikiLogEntryService))
	must(c.Provide(service.NewWikiIngestService, dig.Name("wikiIngest")))
	must(c.Provide(service.NewWikiLintService))
	must(c.Provide(service.NewEmbedChannelService))

	// Artifact generation wiring: the generator registry is populated with
	// the built-in (markdown/report/chart/ppt) generators, and the
	// GenerationService orchestrates LLM-driven artifact production.
	must(c.Provide(func() *generator.Registry {
		reg := generator.NewRegistry()
		artifact.RegisterDefaultGenerators(reg)
		return reg
	}))
	must(c.Provide(artifact.NewGenerationService))
}

func initSemanticCacheService(redisClient *redis.Client) (interfaces.SemanticCacheService, error) {
	cfg := types.DefaultSemanticCacheConfig()
	if os.Getenv("SEMANTIC_CACHE_ENABLED") == "false" {
		cfg.Enabled = false
		return nil, nil
	}
	if redisClient != nil {
		logger.Infof(context.Background(), "[SemanticCache] Using Redis backend")
		return service.NewRedisSemanticCache(redisClient, cfg), nil
	}
	logger.Infof(context.Background(), "[SemanticCache] Redis unavailable, using in-memory backend (Lite mode)")
	return service.NewMemorySemanticCache(cfg), nil
}

func initEmbeddingBatcher() *service.EmbeddingBatcherManager {
	cfg := service.DefaultEmbeddingBatcherConfig()

	// Allow configuration via environment variables
	if v := os.Getenv("EMBEDDING_BATCH_MAX_SIZE"); v != "" {
		if size, err := strconv.Atoi(v); err == nil && size > 0 {
			cfg.MaxBatchSize = size
		}
	}
	if v := os.Getenv("EMBEDDING_BATCH_MAX_WAIT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			cfg.MaxWaitTime = time.Duration(ms) * time.Millisecond
		}
	}
	if v := os.Getenv("EMBEDDING_BATCH_MAX_PENDING"); v != "" {
		if pending, err := strconv.Atoi(v); err == nil && pending > 0 {
			cfg.MaxPendingRequests = pending
		}
	}

	logger.Infof(context.Background(), "[EmbeddingBatcher] Initialized with MaxBatchSize=%d, MaxWaitTime=%v, MaxPendingRequests=%d",
		cfg.MaxBatchSize, cfg.MaxWaitTime, cfg.MaxPendingRequests)
	return service.NewEmbeddingBatcherManager(cfg)
}
