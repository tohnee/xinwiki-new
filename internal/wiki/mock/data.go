package mock

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/wiki"
	"github.com/google/uuid"
)

var (
	// MockWikiPages 模拟Wiki页面数据
	MockWikiPages = []*wiki.Chunk{
		{
			ID:             uuid.New().String(),
			KnowledgeBaseID: "kb-engineering",
			WikiPageID:     "page-architecture-guide",
			Content:        "# XinWiki 架构指南\n\nXinWiki 是一个 AI 驱动的企业级知识库平台，采用混合检索架构（BM25 + 向量检索 + 知识图谱）实现高精度问答。\n\n## 核心架构组件\n\n1. **检索层**：支持 BM25 关键词检索、稠密向量检索、知识图谱关联检索三种模式，通过 RRF（Reciprocal Rank Fusion）算法融合结果。\n\n2. **编译层**：增量编译引擎支持 embedding 缓存，页面更新时仅重新计算变化部分，编译速度提升 60% 以上。\n\n3. **问答层**：高精度 QA 引擎包含引用验证、置信度评分、幻觉检测模块，确保回答的准确性和可溯源性。\n\n4. **RBAC 权限层**：完整的多租户权限体系，支持企业 UUM 认证集成、部门级权限继承。",
			Section:        "架构概述",
			Path:           "/engineering/architecture-guide",
			ChunkIndex:     0,
			TokenCount:     256,
			Metadata:       map[string]string{"author": "技术架构组", "version": "2.0"},
			Score:          0.95,
			LastUpdated:    time.Now().Add(-24 * time.Hour),
		},
		{
			ID:             uuid.New().String(),
			KnowledgeBaseID: "kb-engineering",
			WikiPageID:     "page-retrieval-optimization",
			Content:        "## 检索优化策略\n\n### 混合检索 RRF 融合\n\nRRF（Reciprocal Rank Fusion）公式：\n\n```\nscore(d) = Σ 1/(k + rank_i(d))\n```\n\n其中 k=60 是经验常数，rank_i(d) 是文档 d 在第 i 个检索结果中的排名。\n\n### 查询重写\n\n系统支持以下查询重写策略：\n- **同义词扩展**：基于领域词典扩展查询词\n- **实体链接**：识别查询中的实体并关联到知识图谱\n- **多查询生成**：将复杂问题分解为多个子查询\n\n### 缓存策略\n\n检索结果缓存采用 LRU 策略，TTL 默认 5 分钟，高频查询自动延长缓存时间。",
			Section:        "检索优化",
			Path:           "/engineering/retrieval-optimization",
			ChunkIndex:     1,
			TokenCount:     198,
			Metadata:       map[string]string{"author": "搜索团队", "version": "2.0"},
			Score:          0.89,
			LastUpdated:    time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             uuid.New().String(),
			KnowledgeBaseID: "kb-product",
			WikiPageID:     "page-product-roadmap",
			Content:        "# 产品路线图 2026\n\n## Q2 目标\n\n1. **NotebookLM 风格三栏界面**：完成 280px 左侧导航 + 自适应内容区 + 380px 右侧生成面板的重构\n2. **思维链可视化**：完整展示 AI 思考过程，支持步骤展开、状态追踪、耗时统计\n3. **RBAC 多租户系统**：集成企业 UUM 认证，支持 SCIM 2.0、SAML 2.0、OIDC 协议\n\n## Q3 规划\n\n1. 支持多模态内容理解（图片、表格、公式）\n2. 实时协作编辑功能\n3. 开放 API 平台\n\n## 性能指标\n\n- 检索响应时间 P95 < 200ms\n- 问答准确率 > 95%\n- 编译速度 > 100 页/秒",
			Section:        "路线图",
			Path:           "/product/roadmap-2026",
			ChunkIndex:     0,
			TokenCount:     175,
			Metadata:       map[string]string{"author": "产品团队", "version": "1.0"},
			Score:          0.92,
			LastUpdated:    time.Now().Add(-72 * time.Hour),
		},
		{
			ID:             uuid.New().String(),
			KnowledgeBaseID: "kb-operations",
			WikiPageID:     "page-deployment-guide",
			Content:        "# 生产环境部署指南\n\n## 环境要求\n\n- Go 1.22+\n- Node.js 20+\n- PostgreSQL 15+ with pgvector\n- Redis 7+\n- 对象存储（S3/MinIO）\n\n## 配置检查清单\n\n### 数据库配置\n- [ ] pgvector 扩展已安装\n- [ ] 连接池大小设置为 CPU * 2\n- [ ] 慢查询日志已开启（阈值 100ms）\n\n### 缓存配置\n- [ ] Redis 最大内存策略设置为 allkeys-lru\n- [ ] 开启 RDB + AOF 持久化\n\n### 监控配置\n- [ ] Prometheus metrics 端点暴露\n- [ ] 关键告警规则配置：检索延迟、错误率、Token 消耗\n- [ ] 思维链追踪日志采集",
			Section:        "部署指南",
			Path:           "/operations/deployment-guide",
			ChunkIndex:     0,
			TokenCount:     189,
			Metadata:       map[string]string{"author": "运维团队", "version": "1.2"},
			Score:          0.87,
			LastUpdated:    time.Now().Add(-12 * time.Hour),
		},
		{
			ID:             uuid.New().String(),
			KnowledgeBaseID: "kb-engineering",
			WikiPageID:     "page-rbac-permissions",
			Content:        "## RBAC 权限模型设计\n\n### 核心模型\n\n```\nUser ──< UserRoleAssignment >── Role ──< RolePermission >── Permission\n                                                         │\n                                                         ▼\n                                                      Resource\n```\n\n### 权限继承规则\n\n1. **部门继承**：用户自动继承所在部门的所有角色权限\n2. **角色优先级**：直接分配的角色 > 部门继承的角色\n3. **拒绝优先**：显式拒绝权限覆盖所有允许权限\n\n### UUM 集成协议\n\n- **SCIM 2.0**：用户/部门自动同步\n- **SAML 2.0**：Web SSO 单点登录\n- **OIDC**：OAuth 2.0 + API 访问\n\n### 资源类型\n\n- `wiki:page` - Wiki 页面\n- `wiki:kb` - 知识库\n- `agent:config` - Agent 配置\n- `admin:settings` - 系统设置",
			Section:        "RBAC设计",
			Path:           "/engineering/rbac-permissions",
			ChunkIndex:     2,
			TokenCount:     203,
			Metadata:       map[string]string{"author": "安全团队", "version": "2.0"},
			Score:          0.91,
			LastUpdated:    time.Now().Add(-6 * time.Hour),
		},
	}

	// MockKnowledgeBases 模拟知识库列表
	MockKnowledgeBases = []struct {
		ID          string
		Name        string
		Description string
		DocCount    int
		Icon        string
	}{
		{"kb-engineering", "技术文档库", "架构设计、开发规范、技术方案", 128, "📚"},
		{"kb-product", "产品文档库", "产品需求、路线图、用户手册", 56, "📋"},
		{"kb-operations", "运维文档库", "部署指南、故障排查、监控告警", 42, "🔧"},
		{"kb-hr", "人力资源库", "公司制度、培训材料、入职指南", 32, "👥"},
	}

	// MockThinkingChain 模拟思维链数据
	MockThinkingChain = &wiki.ThinkingStep{
		ID:          uuid.New().String(),
		StepType:    "retrieval",
		Title:       "分析用户问题",
		Description: "理解用户查询意图，确定检索策略",
		Status:      "completed",
		DurationMs:  45,
		StartTime:   time.Now().Add(-2 * time.Second),
		EndTime:     time.Now().Add(-1955 * time.Millisecond),
		Children: []*wiki.ThinkingStep{
			{
				ID:          uuid.New().String(),
				StepType:    "query_rewrite",
				Title:       "查询重写",
				Description: "扩展同义词：XinWiki → XinWiki平台/知识库系统",
				Status:      "completed",
				DurationMs:  32,
				StartTime:   time.Now().Add(-1955 * time.Millisecond),
				EndTime:     time.Now().Add(-1923 * time.Millisecond),
			},
			{
				ID:          uuid.New().String(),
				StepType:    "bm25_search",
				Title:       "BM25 关键词检索",
				Description: "召回 15 篇相关文档，耗时 28ms",
				Status:      "completed",
				DurationMs:  28,
				StartTime:   time.Now().Add(-1923 * time.Millisecond),
				EndTime:     time.Now().Add(-1895 * time.Millisecond),
			},
			{
				ID:          uuid.New().String(),
				StepType:    "vector_search",
				Title:       "向量语义检索",
				Description: "召回 20 篇语义相关文档，Embedding 计算 89ms，向量搜索 15ms",
				Status:      "completed",
				DurationMs:  104,
				StartTime:   time.Now().Add(-1895 * time.Millisecond),
				EndTime:     time.Now().Add(-1791 * time.Millisecond),
			},
			{
				ID:          uuid.New().String(),
				StepType:    "rrf_fusion",
				Title:       "RRF 结果融合",
				Description: "融合两路召回结果，去重后共 28 篇，重排序完成",
				Status:      "completed",
				DurationMs:  12,
				StartTime:   time.Now().Add(-1791 * time.Millisecond),
				EndTime:     time.Now().Add(-1779 * time.Millisecond),
			},
		},
	}
)

// MockBM25Retriever 模拟BM25检索器
type MockBM25Retriever struct{}

func (m *MockBM25Retriever) Search(ctx context.Context, query string, kbIDs []string, topK int, filters map[string]interface{}) ([]*wiki.SearchResult, error) {
	results := make([]*wiki.SearchResult, 0)
	for i, chunk := range MockWikiPages {
		if i >= topK {
			break
		}
		results = append(results, &wiki.SearchResult{
			Chunk:      chunk,
			FinalScore: 0.85 - float64(i)*0.05,
			BM25Score:  0.90 - float64(i)*0.06,
			Rank:       i + 1,
		})
	}
	return results, nil
}

func (m *MockBM25Retriever) IndexDocument(ctx context.Context, chunks []*wiki.Chunk) error {
	return nil
}

func (m *MockBM25Retriever) RemoveDocument(ctx context.Context, chunkIDs []string) error {
	return nil
}

// MockVectorRetriever 模拟向量检索器
type MockVectorRetriever struct{}

func (m *MockVectorRetriever) Search(ctx context.Context, queryEmbedding []float32, kbIDs []string, topK int, filters map[string]interface{}) ([]*wiki.SearchResult, error) {
	results := make([]*wiki.SearchResult, 0)
	for i, chunk := range MockWikiPages {
		if i >= topK {
			break
		}
		results = append(results, &wiki.SearchResult{
			Chunk:       chunk,
			FinalScore:  0.88 - float64(i)*0.04,
			VectorScore: 0.92 - float64(i)*0.05,
			Rank:        i + 1,
		})
	}
	return results, nil
}

func (m *MockVectorRetriever) IndexDocument(ctx context.Context, chunks []*wiki.Chunk) error {
	return nil
}

func (m *MockVectorRetriever) RemoveDocument(ctx context.Context, chunkIDs []string) error {
	return nil
}

// MockGraphRetriever 模拟知识图谱检索器
type MockGraphRetriever struct{}

func (m *MockGraphRetriever) Search(ctx context.Context, entities []string, kbIDs []string, topK int, depth int) ([]*wiki.SearchResult, error) {
	return []*wiki.SearchResult{}, nil
}

func (m *MockGraphRetriever) Expand(ctx context.Context, chunkIDs []string, depth int) ([]*wiki.SearchResult, error) {
	return []*wiki.SearchResult{}, nil
}

// MockQueryRewriter 模拟查询重写器
type MockQueryRewriter struct{}

func (m *MockQueryRewriter) Rewrite(ctx context.Context, query string) (*wiki.QueryRewrite, error) {
	return &wiki.QueryRewrite{
		OriginalQuery:  query,
		ExpandedQueries: []string{query},
		Entities:       []string{"XinWiki", "混合检索"},
	}, nil
}

func (m *MockQueryRewriter) ExtractEntities(ctx context.Context, query string) ([]string, error) {
	return []string{"XinWiki", "混合检索"}, nil
}

// NewMockHybridRetriever 创建使用模拟数据的混合检索器
func NewMockHybridRetriever() *wiki.HybridRetriever {
	return wiki.NewHybridRetriever(
		&MockBM25Retriever{},
		&MockVectorRetriever{},
		&MockGraphRetriever{},
		&MockQueryRewriter{},
		5*time.Minute,
		1000,
	)
}
