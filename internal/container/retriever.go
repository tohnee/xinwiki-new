package container

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	esv7 "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v8"
	_ "github.com/go-sql-driver/mysql" // 给 Doris (database/sql) 注册 MySQL 协议驱动
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/qdrant/go-client/qdrant"
	"go.uber.org/dig"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	dorisRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/doris"
	elasticsearchRepoV7 "github.com/Tencent/XinWiki/internal/application/repository/retriever/elasticsearch/v7"
	elasticsearchRepoV8 "github.com/Tencent/XinWiki/internal/application/repository/retriever/elasticsearch/v8"
	milvusRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/milvus"
	openSearchRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/opensearch"
	postgresRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/postgres"
	qdrantRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/qdrant"
	sqliteRetrieverRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/sqlite"
	tencentVectorDBRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/tencentvectordb"
	weaviateRepo "github.com/Tencent/XinWiki/internal/application/repository/retriever/weaviate"
	"github.com/Tencent/XinWiki/internal/application/repository"
	"github.com/Tencent/XinWiki/internal/application/service"
	"github.com/Tencent/XinWiki/internal/application/service/retriever"
	"github.com/Tencent/XinWiki/internal/config"
	infra_web_search "github.com/Tencent/XinWiki/internal/infrastructure/web_search"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/tencent/vectordatabase-sdk-go/tcvectordb"
	"github.com/weaviate/weaviate-go-client/v5/weaviate"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/auth"
	wgrpc "github.com/weaviate/weaviate-go-client/v5/weaviate/grpc"
)

func registerRetriever(c *dig.Container, ctx context.Context) {
	// Initialize retrieval engine registry for search capabilities
	logger.Debugf(ctx, "[Container] Registering retrieval engine registry...")
	must(c.Provide(initRetrieveEngineRegistry))

	// Web search service (needed by AgentService)
	logger.Debugf(ctx, "[Container] Registering web search registry and providers...")
	must(c.Provide(infra_web_search.NewRegistry))
	must(c.Invoke(registerWebSearchProviders))
	must(c.Provide(repository.NewWebSearchProviderRepository))
	must(c.Provide(repository.NewVectorStoreRepository))
	// TenantStoreOwnership adapter used by the retriever factory functions
	// to verify that a resolved VectorStore belongs to the caller's tenant.
	must(c.Provide(retriever.NewVectorStoreRepoOwnership))
	must(c.Provide(service.NewWebSearchService))
	must(c.Provide(service.NewWebSearchProviderService))
	// 注册读写分离路由器
	must(c.Provide(func(cfg *config.Config) *service.VectorStoreRouter {
		rwConfig := types.DefaultReadWriteSeparationConfig()
		// 可以从配置或环境变量加载读写分离配置
		return service.NewVectorStoreRouter(rwConfig)
	}))

	must(c.Provide(NewEngineFactory))
	// StoreRegistry: same instance as RetrieveEngineRegistry, exposed as StoreRegistry interface.
	// NewRetrieveEngineRegistry always returns *retriever.RetrieveEngineRegistry which implements both.
	must(c.Provide(func(r interfaces.RetrieveEngineRegistry) (interfaces.StoreRegistry, error) {
		sr, ok := r.(*retriever.RetrieveEngineRegistry)
		if !ok {
			return nil, fmt.Errorf("registry does not implement StoreRegistry")
		}
		return sr, nil
	}))
	must(c.Provide(service.NewVectorStoreService))
}

// initRetrieveEngineRegistry initializes the retrieval engine registry
// Sets up and configures various search engine backends based on configuration
// Supports multiple retrieval engines (PostgreSQL, ElasticsearchV7, ElasticsearchV8)
// Parameters:
//   - db: Database connection
//   - cfg: Application configuration
//
// Returns:
//   - Configured retrieval engine registry
//   - Error if initialization fails
func initRetrieveEngineRegistry(
	db *gorm.DB, cfg *config.Config, auditSvc interfaces.AuditLogService, router *service.VectorStoreRouter,
) (interfaces.RetrieveEngineRegistry, error) {
	// 注册工厂函数，打破 service ↔ retriever 导入循环
	retriever.SetRouterFactory(
		func(cfg types.ReadWriteSeparationConfig) retriever.Router { return service.NewVectorStoreRouter(cfg) },
		service.WrapEngineWithRWCapabilities,
	)
	registry := retriever.NewRetrieveEngineRegistry()
	retrieveDriver := strings.Split(os.Getenv("RETRIEVE_DRIVER"), ",")
	log := logger.GetLogger(context.Background())
	// Audit sink for OpenSearch driver events (index created / reindex). Driver
	// events fire under a tenant-scoped ctx at indexing time; the env-path
	// registration ctx below has no tenant, so those emits self-skip.
	auditSink := newAuditSinkAdapter(auditSvc)

	// wrapAndRegisterEnvEngine wraps an env-based engine with RW capabilities
	// and registers it with both the router and the legacy registry.
	wrapAndRegisterEnvEngine := func(envID string, engine interfaces.RetrieveEngineService, engineType string) error {
		rwConfig := types.DefaultReadWriteSeparationConfig()
		// RegisterEngineWithConfig内部自动包装，第一版不配置副本
		if err := router.RegisterEngineWithConfig(envID, engine, rwConfig, nil); err != nil {
			return err
		}
		wrapper := service.NewRouterWrapper(router, envID)
		return registry.Register(wrapper)
	}

	if slices.Contains(retrieveDriver, "postgres") {
		postgresRepo := postgresRepo.NewPostgresRetrieveEngineRepository(db)
		engine := retriever.NewKVHybridRetrieveEngine(postgresRepo, types.PostgresRetrieverEngineType)
		envID := "env:" + string(types.PostgresRetrieverEngineType)
		if err := wrapAndRegisterEnvEngine(envID, engine, string(types.PostgresRetrieverEngineType)); err != nil {
			log.Errorf("Register postgres retrieve engine failed: %v", err)
		} else {
			log.Infof("Register postgres retrieve engine success")
		}
	}
	if slices.Contains(retrieveDriver, "sqlite") {
		sqliteRepo := sqliteRetrieverRepo.NewSQLiteRetrieveEngineRepository(db)
		engine := retriever.NewKVHybridRetrieveEngine(sqliteRepo, types.SQLiteRetrieverEngineType)
		envID := "env:" + string(types.SQLiteRetrieverEngineType)
		if err := wrapAndRegisterEnvEngine(envID, engine, string(types.SQLiteRetrieverEngineType)); err != nil {
			log.Errorf("Register sqlite retrieve engine failed: %v", err)
		} else {
			log.Infof("Register sqlite retrieve engine success")
		}
	}
	if slices.Contains(retrieveDriver, "elasticsearch_v8") {
		client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
			Addresses: []string{os.Getenv("ELASTICSEARCH_ADDR")},
			Username:  os.Getenv("ELASTICSEARCH_USERNAME"),
			Password:  os.Getenv("ELASTICSEARCH_PASSWORD"),
		})
		if err != nil {
			log.Errorf("Create elasticsearch_v8 client failed: %v", err)
		} else {
			elasticsearchRepo := elasticsearchRepoV8.NewElasticsearchEngineRepository(client, cfg, nil)
			engine := retriever.NewKVHybridRetrieveEngine(elasticsearchRepo, types.ElasticsearchRetrieverEngineType)
			envID := "env:elasticsearch_v8"
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.ElasticsearchRetrieverEngineType)); err != nil {
				log.Errorf("Register elasticsearch_v8 retrieve engine failed: %v", err)
			} else {
				log.Infof("Register elasticsearch_v8 retrieve engine success")
			}
		}
	}

	if slices.Contains(retrieveDriver, "elasticsearch_v7") {
		client, err := esv7.NewClient(esv7.Config{
			Addresses: []string{os.Getenv("ELASTICSEARCH_ADDR")},
			Username:  os.Getenv("ELASTICSEARCH_USERNAME"),
			Password:  os.Getenv("ELASTICSEARCH_PASSWORD"),
		})
		if err != nil {
			log.Errorf("Create elasticsearch_v7 client failed: %v", err)
		} else {
			elasticsearchRepo := elasticsearchRepoV7.NewElasticsearchEngineRepository(client, cfg, nil)
			engine := retriever.NewKVHybridRetrieveEngine(elasticsearchRepo, types.ElasticsearchRetrieverEngineType)
			envID := "env:elasticsearch_v7"
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.ElasticsearchRetrieverEngineType)); err != nil {
				log.Errorf("Register elasticsearch_v7 retrieve engine failed: %v", err)
			} else {
				log.Infof("Register elasticsearch_v7 retrieve engine success")
			}
		}
	}

	if slices.Contains(retrieveDriver, "opensearch") {
		cc := &types.ConnectionConfig{
			Addr:               os.Getenv("OPENSEARCH_ADDR"),
			Username:           os.Getenv("OPENSEARCH_USERNAME"),
			Password:           os.Getenv("OPENSEARCH_PASSWORD"),
			InsecureSkipVerify: strings.EqualFold(os.Getenv("OPENSEARCH_INSECURE_SKIP_VERIFY"), "true"),
		}
		client, err := openSearchRepo.NewOpenSearchClient(cc)
		if err != nil {
			log.Errorf("Create opensearch client failed: %v", err)
		} else if repo, err := openSearchRepo.NewRepository(
			context.Background(), client, "", nil, openSearchRepo.WithAuditSink(auditSink),
		); err != nil {
			log.Errorf("Create opensearch repository failed: %v", err)
		} else {
			engine := retriever.NewKVHybridRetrieveEngine(repo, types.OpenSearchRetrieverEngineType)
			envID := "env:" + string(types.OpenSearchRetrieverEngineType)
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.OpenSearchRetrieverEngineType)); err != nil {
				log.Errorf("Register opensearch retrieve engine failed: %v", err)
			} else {
				log.Infof("Register opensearch retrieve engine success")
			}
		}
	}

	if slices.Contains(retrieveDriver, "qdrant") {
		qdrantHost := os.Getenv("QDRANT_HOST")
		if qdrantHost == "" {
			qdrantHost = "localhost"
		}

		qdrantPort := 6334 // Default port
		if portStr := os.Getenv("QDRANT_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				qdrantPort = port
			}
		}

		// API key for authentication (optional)
		qdrantAPIKey := os.Getenv("QDRANT_API_KEY")

		// TLS configuration (optional, defaults to false)
		// Enable TLS unless explicitly set to "false" or "0" (case insensitive)
		qdrantUseTLS := false
		if useTLSStr := os.Getenv("QDRANT_USE_TLS"); useTLSStr != "" {
			useTLSLower := strings.ToLower(strings.TrimSpace(useTLSStr))
			qdrantUseTLS = useTLSLower != "false" && useTLSLower != "0"
		}

		log.Infof("Connecting to Qdrant at %s:%d (TLS: %v)", qdrantHost, qdrantPort, qdrantUseTLS)

		client, err := qdrant.NewClient(&qdrant.Config{
			Host:   qdrantHost,
			Port:   qdrantPort,
			APIKey: qdrantAPIKey,
			UseTLS: qdrantUseTLS,
		})
		if err != nil {
			log.Errorf("Create qdrant client failed: %v", err)
		} else {
			qdrantRepository := qdrantRepo.NewQdrantRetrieveEngineRepository(client, nil)
			engine := retriever.NewKVHybridRetrieveEngine(qdrantRepository, types.QdrantRetrieverEngineType)
			envID := "env:" + string(types.QdrantRetrieverEngineType)
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.QdrantRetrieverEngineType)); err != nil {
				log.Errorf("Register qdrant retrieve engine failed: %v", err)
			} else {
				log.Infof("Register qdrant retrieve engine success")
			}
		}
	}
	if slices.Contains(retrieveDriver, "weaviate") {
		weaviateHost := os.Getenv("WEAVIATE_HOST")
		if weaviateHost == "" {
			// Docker compose default (service name inside network)
			weaviateHost = "weaviate:8080"
		}
		weaviateGrpcAddress := os.Getenv("WEAVIATE_GRPC_ADDRESS")
		if weaviateGrpcAddress == "" {
			weaviateGrpcAddress = "weaviate:50051"
		}
		weaviateScheme := os.Getenv("WEAVIATE_SCHEME")
		if weaviateScheme == "" {
			weaviateScheme = "http"
		}
		var authConfig auth.Config
		if strings.EqualFold(strings.TrimSpace(os.Getenv("WEAVIATE_AUTH_ENABLED")), "true") {
			if apiKey := strings.TrimSpace(os.Getenv("WEAVIATE_API_KEY")); apiKey != "" {
				authConfig = auth.ApiKey{Value: apiKey}
			}
		}
		weaviateClient, err := weaviate.NewClient(weaviate.Config{
			Host: weaviateHost,
			GrpcConfig: &wgrpc.Config{
				Host: weaviateGrpcAddress,
			},
			Scheme:     weaviateScheme,
			AuthConfig: authConfig,
		})
		if err != nil {
			log.Errorf("Create weaviate client failed: %v", err)
		} else {
			weaviateRepository := weaviateRepo.NewWeaviateRetrieveEngineRepository(weaviateClient, nil)
			engine := retriever.NewKVHybridRetrieveEngine(weaviateRepository, types.WeaviateRetrieverEngineType)
			envID := "env:" + string(types.WeaviateRetrieverEngineType)
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.WeaviateRetrieverEngineType)); err != nil {
				log.Errorf("Register weaviate retrieve engine failed: %v", err)
			} else {
				log.Infof("Register weaviate retrieve engine success")
			}
		}
	}
	if slices.Contains(retrieveDriver, "milvus") {
		milvusCfg := milvusclient.ClientConfig{
			DialOptions: []grpc.DialOption{grpc.WithTimeout(5 * time.Second)},
		}
		milvusAddress := os.Getenv("MILVUS_ADDRESS")
		if milvusAddress == "" {
			milvusAddress = "localhost:19530"
		}
		milvusCfg.Address = milvusAddress
		milvusUsername := os.Getenv("MILVUS_USERNAME")
		if milvusUsername != "" {
			milvusCfg.Username = milvusUsername
		}
		milvusPassword := os.Getenv("MILVUS_PASSWORD")
		if milvusPassword != "" {
			milvusCfg.Password = milvusPassword
		}
		milvusDBName := os.Getenv("MILVUS_DB_NAME")
		if milvusDBName != "" {
			milvusCfg.DBName = milvusDBName
		}
		milvusCli, err := milvusclient.New(context.Background(), &milvusCfg)
		if err != nil {
			log.Errorf("Create milvus client failed: %v", err)
		} else {
			milvusRepository := milvusRepo.NewMilvusRetrieveEngineRepository(milvusCli, nil)
			engine := retriever.NewKVHybridRetrieveEngine(milvusRepository, types.MilvusRetrieverEngineType)
			envID := "env:" + string(types.MilvusRetrieverEngineType)
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.MilvusRetrieverEngineType)); err != nil {
				log.Errorf("Register milvus retrieve engine failed: %v", err)
			} else {
				log.Infof("Register milvus retrieve engine success")
			}
		}
	}
	if slices.Contains(retrieveDriver, "doris") {
		dorisAddr := os.Getenv("DORIS_ADDR")
		if dorisAddr == "" {
			// docker-compose 默认服务名 + Doris FE MySQL 端口
			dorisAddr = "doris-fe:9030"
		}
		dorisDatabase := os.Getenv("DORIS_DATABASE")
		if dorisDatabase == "" {
			dorisDatabase = "xinwiki"
		}
		dorisUsername := os.Getenv("DORIS_USERNAME")
		if dorisUsername == "" {
			dorisUsername = "root"
		}
		dorisPassword := os.Getenv("DORIS_PASSWORD")
		dorisHTTPPort := 8030
		if portStr := os.Getenv("DORIS_HTTP_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				dorisHTTPPort = port
			}
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=Local&interpolateParams=true",
			dorisUsername, dorisPassword, dorisAddr, dorisDatabase)
		dorisDB, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Errorf("Create doris client failed: %v", err)
		} else {
			dorisDB.SetMaxOpenConns(20)
			dorisDB.SetMaxIdleConns(5)
			dorisDB.SetConnMaxLifetime(time.Hour)

			httpBase := "http://" + hostFromAddr(dorisAddr) + ":" + strconv.Itoa(dorisHTTPPort)
			dorisRepository := dorisRepo.NewDorisRetrieveEngineRepository(
				dorisDB, httpBase, dorisUsername, dorisPassword, dorisDatabase, nil,
			)
			engine := retriever.NewKVHybridRetrieveEngine(dorisRepository, types.DorisRetrieverEngineType)
			envID := "env:" + string(types.DorisRetrieverEngineType)
			if err := wrapAndRegisterEnvEngine(envID, engine, string(types.DorisRetrieverEngineType)); err != nil {
				log.Errorf("Register doris retrieve engine failed: %v", err)
			} else {
				log.Infof("Register doris retrieve engine success: %s db=%s", dorisAddr, dorisDatabase)
			}
		}
	}
	if slices.Contains(retrieveDriver, "tencent_vectordb") {
		addr := os.Getenv("TENCENT_VECTORDB_ADDR")
		username := os.Getenv("TENCENT_VECTORDB_USERNAME")
		apiKey := os.Getenv("TENCENT_VECTORDB_API_KEY")
		if addr == "" || username == "" || apiKey == "" {
			log.Errorf("Missing Tencent VectorDB configuration")
		} else {
			client, err := tcvectordb.NewRpcClient(addr, username, apiKey, &tcvectordb.ClientOption{
				ReadConsistency: tcvectordb.EventualConsistency,
				Timeout:         10 * time.Second,
			})
			if err != nil {
				log.Errorf("Create tencent_vectordb client failed: %v", err)
			} else {
				tencentRepository := tencentVectorDBRepo.NewTencentVectorDBRetrieveEngineRepository(
					client,
					os.Getenv("TENCENT_VECTORDB_DATABASE"),
					nil,
				)
				engine := retriever.NewKVHybridRetrieveEngine(tencentRepository, types.TencentVectorDBRetrieverEngineType)
				envID := "env:" + string(types.TencentVectorDBRetrieverEngineType)
				if err := wrapAndRegisterEnvEngine(envID, engine, string(types.TencentVectorDBRetrieverEngineType)); err != nil {
					log.Errorf("Register tencent_vectordb retrieve engine failed: %v", err)
				} else {
					log.Infof("Register tencent_vectordb retrieve engine success")
				}
			}
		}
	}
	// ─── DB store registration (byStoreID) ───
	if storeReg, ok := registry.(*retriever.RetrieveEngineRegistry); ok {
		loadDBStoresIntoRegistry(storeReg, db, cfg, auditSink, router)
	}

	return registry, nil
}

// loadDBStoresIntoRegistry loads VectorStore records from DB and registers them
// in the registry's byStoreID map. Failures are logged and skipped (non-fatal).
func loadDBStoresIntoRegistry(
	storeRegistry interfaces.StoreRegistry, db *gorm.DB, cfg *config.Config, auditSink openSearchRepo.AuditSink, router *service.VectorStoreRouter,
) {
	ctx := context.Background()
	log := logger.GetLogger(ctx)

	var stores []types.VectorStore
	// GORM soft delete automatically adds "deleted_at IS NULL" condition
	if err := db.Find(&stores).Error; err != nil {
		log.Warnf("Failed to load vector stores from DB: %v", err)
		return
	}

	if len(stores) == 0 {
		return
	}

	log.Infof("Loading %d vector store(s) from database", len(stores))
	for _, store := range stores {
		svc, err := createEngineServiceFromStore(ctx, store, db, cfg, auditSink, router)
		if err != nil {
			log.Errorf("Failed to create engine for store %s (%s): %v", store.ID, store.Name, err)
			continue
		}
		storeRegistry.RegisterWithStoreID(store.ID, svc)
		log.Infof("Registered DB vector store: id=%s, name=%s, engine=%s", store.ID, store.Name, store.EngineType)
	}
}

// registerWebSearchProviders registers all web search provider types to the registry.
// Each provider type is registered with its factory function that accepts parameters.
// Provider instances are created on-demand when tenants configure them.
func registerWebSearchProviders(registry *infra_web_search.Registry) {
	registry.Register("duckduckgo", infra_web_search.NewDuckDuckGoProvider)
	registry.Register("google", infra_web_search.NewGoogleProvider)
	registry.Register("bing", infra_web_search.NewBingProvider)
	registry.Register("tavily", infra_web_search.NewTavilyProvider)
	registry.Register("ollama", infra_web_search.NewOllamaProvider)
	registry.Register("baidu", infra_web_search.NewBaiduProvider)
	registry.Register("searxng", infra_web_search.NewSearxngProvider)
}
