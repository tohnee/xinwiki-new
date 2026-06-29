# XinWiki 项目全面代码审查报告

**审查日期**: 2026-06-29
**审查范围**: 全项目（后端 Go / 前端 Vue / Python docreader / Docker / Helm / CI / 文档）
**审查方法**: 5 个并行子代理 + 静态代码审查 + 安全审计 + 架构分析
**审查标准**: 生产就绪度、安全合规、性能、可维护性、架构合理性

---

## 综合评估

| 维度 | 评级 | 评分 | 关键问题数 |
|---|---|---|---|
| **生产就绪度** | ❌ 不达标 | 48/100 | 18 个 Blocker |
| **安全合规** | ❌ 不达标 | 4/10 | 7 个 CRITICAL 安全漏洞 |
| **代码质量** | ⚠️ 中等 | 6/10 | 28 个 HIGH + 50+ MEDIUM |
| **架构设计** | ✅ 良好 | 8/10 | 设计有亮点 |
| **文档准确性** | ❌ 不达标 | 3/10 | README 与代码严重脱节 |
| **测试覆盖** | ⚠️ 不足 | 5/10 | 关键新功能无测试 |
| **可观测性** | ✅ 良好 | 7/10 | SpanTracker 设计优秀 |

**核心结论**：项目不可上线。必须先修复 18 个 Blocker（CRITICAL 级别问题），主要阻塞点为：默认凭证入仓、容器以 root 运行、CORS 配置错误、UUM 伪验证、并发安全 bug、文档与代码脱节。

---

## 一、CRITICAL 严重问题（必须立即修复 - P0）

### 1.1 CORS 配置违规：通配符 + 凭证同时启用
**文件**: `internal/router/router.go:111-118`
```go
AllowOrigins:     []string{"*"},
AllowCredentials: true,
```
**影响**: 任意恶意网站可携带 JWT/Cookie/X-API-Key 跨站调用 API，读取租户列表、知识库内容等敏感数据。
**修复**: 改为精确白名单 `["https://xinwiki.example.com"]`，从 config 读取；或保留 `*` 但 `AllowCredentials: false`。

### 1.2 UUM SAML/OIDC 验证完全未实现
**文件**: `internal/auth/uum/handler.go:159-223`
**问题**: `ValidateSAMLAssertion` 和 `ValidateOIDCToken` 完全不验证签名/iss/aud/exp，直接返回伪造的 assertion 并自动 provision 用户。
**影响**: 任何能调用此端点的攻击者可伪造企业身份进入任意租户。
**修复**: 实现前直接 `return errors.New("SAML/OIDC not implemented")`；实现时使用 `gosaml2` / `go-oidc` 库。

### 1.3 TENANT_AES_KEY 缺失时 panic
**文件**: `internal/application/service/tenant.go:23-25, 252-281`
**问题**: AES 密钥从环境变量读取但无长度校验，错误时 `panic`。`aes.NewCipher` 失败直接崩溃。
**修复**: 在 `startup.go` 添加 `TENANT_AES_KEY` 长度校验（16/24/32 字节）；`generateApiKey` 用 error 返回代替 panic。

### 1.4 Embedding Batcher 结果分发错位 Bug
**文件**: `internal/application/service/embedding_batcher.go:270-280`
```go
textIdx := 0
for _, originalIdxs := range textToIdx {  // map 迭代顺序随机!
    res := results[textIdx]  // 按 0,1,2 递增
    for _, idx := range originalIdxs {
        batch[idx].ResultChan <- res  // 分发错误的 embedding 向量!
    }
    textIdx++
}
```
**影响**: 文本检索返回错误结果，知识库问答答非所问，难以复现的偶发性错误。
**修复**: 按 `texts[]` 数组顺序构建 idx 映射，按顺序分发。

### 1.5 Thinking Tracker StartStep 返回局部变量指针
**文件**: `internal/agent/thinking/tracker.go:40-65`
**问题**: `append(t.steps, step)` 复制值到 slice，但返回 `&step` 指向局部变量。后续通过指针的修改（Tokens、Duration）不会反映到 slice 中。
**影响**: 思维链追踪数据不完整，前端可视化显示错误 token 计数。
**修复**: `t.currentStep = &t.steps[len(t.steps)-1]` 返回 slice 中元素的指针。

### 1.6 Prompt 模板完全内存实现，重启丢失所有版本
**文件**: `internal/application/service/prompt_template.go:20-37`
**问题**: 使用 `sync.Once` 单例 + 内存 map 存储 Prompt 模板，**完全没有持久化层**。`internal/application/repository/` 下不存在 `prompt_template_repo.go`。
**影响**: 服务重启后所有版本丢失，多实例部署版本不一致，无法做版本审计。
**修复**: 创建 `prompt_template_repo.go` 使用 GORM 持久化到 `prompt_templates` 表，复合唯一索引 `(tenant_id, template_key, version)`。

### 1.7 .env.example 硬编码加密密钥泄露
**文件**: `.env.example`
- `TENANT_AES_KEY=weknorarag-api-key-secret-secret`
- `SYSTEM_AES_KEY=weknora-system-aes-key-32bytes!!`（实际 33 字符，违反 AES-256 严格要求 32 字节）
- `JWT_SECRET=weknora-jwt-secret`（仅 19 字符）
- `DB_PASSWORD=postgres123!@#`

**影响**: 用户直接 `cp .env.example .env` 后不修改即上线，所有加密字段可被任何人解密。
**修复**: 所有敏感字段默认值改为空字符串 + 注释 `# MUST SET: openssl rand -hex 32`。

### 1.8 docker-compose 默认密钥严重泄露
**文件**: `docker-compose.yml`, `docker-compose.dev.yml`
- `LANGFUSE_ENCRYPTION_KEY: 0000...0000`（全 0 密钥，加密形同虚设）
- `LANGFUSE_NEXTAUTH_SECRET: weknora-langfuse-dev-nextauth-secret-change-me`
- `CLICKHOUSE_USER/PASSWORD: clickhouse/clickhouse`
- `MINIO_ROOT_USER/PASSWORD: langfuseminio/langfuseminiosecret`

**影响**: Langfuse 中所有 LLM 调用追踪数据（含 prompt、completion、API key 元数据）等同明文存储。
**修复**: 移除所有默认值，改为 `${LANGFUSE_ENCRYPTION_KEY:?required}` 强制失败。

### 1.9 README 与实际代码严重不符
**文件**: `README_CN.md:67-85`
**问题**: 宣称具有「三栏式 Workspace」「思维链可视化」「XinWikiLogo 组件」等，但 `frontend/src/components/` 目录下**仅有 `menu.vue` 一个组件**。`App.vue` import 了不存在的 `manual-knowledge-editor.vue` 和 `UploadConfirmHost.vue`，**前端项目无法编译**。`docs/images/` 下缺失 `workspace.png`、`thinking-chain.png`、`cost-dashboard.png`。
**影响**: 文档与产品实际情况严重脱节，前端无法编译，D4 里程碑报告声称 100% 完成但实际交付物不完整。
**修复**: 核实组件是否存在其他路径或 git history；若未开发，必须从 README 移除相应宣称，CHANGELOG 如实标注进度。

### 1.10 Postgres 连接池无上限
**文件**: `internal/container/container.go:654-658`
```go
if sqlite {
    sqlDB.SetMaxOpenConns(1)
} else {
    sqlDB.SetMaxIdleConns(10)  // 仅设置 idle，未设 open 上限！
}
```
**影响**: GORM 默认 `MaxOpenConns=0`（无上限），高并发会瞬间打开数百连接打爆 postgres `max_connections`，导致全租户连接拒绝。
**修复**: `sqlDB.SetMaxOpenConns(50)` + `SetConnMaxIdleTime(5*time.Minute)`，通过 `DB_MAX_OPEN_CONNS` env var 暴露配置。

### 1.11 迁移失败仅 warn 不阻塞启动
**文件**: `internal/container/container.go:621-628`
```go
if err := database.RunMigrationsWithOptions(...); err != nil {
    logger.Warnf(ctx, "Database migration failed: %v", err)
    logger.Warnf(ctx, "Continuing with application startup...")
}
```
**影响**: 迁移失败后应用继续启动，可能运行在部分应用的 schema 上，新增列缺失导致 GORM 查询全表失败，写入脏数据。
**修复**: `return nil, err` 直接 fail-fast，让 k8s 重启策略接管。

### 1.12 Docker 容器以 root 运行（5 个 Dockerfile）
**文件**: 
- `docker/Dockerfile.docreader`：无 `USER` 指令
- `docker/Dockerfile.odl-hybrid`：单阶段构建 + root
- `docker/Dockerfile.app`：运行阶段 root + 安装 vim/wget
- `frontend/Dockerfile`：nginx root
- `mcp-server/Dockerfile`：单阶段 + `pip install -e .` + root

**影响**: 容器 RCE 即获得 root 权限，违反 Pod Security Standards `restricted` 级别。
**修复**: 创建非 root 用户：
```dockerfile
RUN groupadd -r app && useradd -r -g app -u 1000 app
USER app
```

### 1.13 Dockerfile.app 在镜像内执行 curl|sh 远程脚本
**文件**: `docker/Dockerfile.app`
```dockerfile
RUN curl -LsSf https://astral.sh/uv/install.sh | CARGO_HOME=... sh
```
**影响**: 供应链劫持风险。astral.sh 域名被劫持即可植入恶意二进制到所有生产镜像。
**修复**: 使用 `COPY --from=ghcr.io/astral-sh/uv:0.5.0 /uv /usr/local/bin/uv`。

### 1.14 Python 依赖未固定版本 + 废弃库
**文件**: `docreader/pyproject.toml`
**问题**: 除 `textract==1.5.0` 外所有依赖使用 `>=`，无 lock 文件。`textract==1.5.0` 最后版本 2017 年，已废弃 6+ 年，传递依赖有未修复 CVE。
**修复**: 用 `uv pip compile` 生成 `requirements.lock`（含哈希）；替换 `textract` 为 `unstructured` 或 `markitdown`。

### 1.15 A5 事件驱动 ACL 重算缺失（权限泄露 P0）
**文件**: `docs/D4-review-report.md:68-83`
**问题**: 来源 Chunk 密级变更后，派生 Wiki 不会自动重算 ACL，高密级用户可见的内容仍可被低密级用户通过派生 Wiki 访问。
**当前状态**: `internal/event/event.go` 缺少 `EventPermissionChanged` / `EventDocumentACLUpdated` 事件类型。
**修复**: 按 D4-review-report.md 路线图实施，新增权限变更事件 + ACL 重算订阅者。

### 1.16 E3 Permission Leakage CI 红线缺失
**问题**: 项目验收红线明确要求「权限泄露率 = 0」作为 CI 门禁，但完全未实现。无 Permission Leakage Rate 评测指标、无 Citation Accuracy 基线、CI 无任何红线门禁接入。
**修复**: 在 `internal/application/service/metric/` 下新增 `permission_leakage.go`，CI 中接入红线断言。

### 1.17 AI5：UUM 连接测试桩伪实现
**文件**: `internal/auth/uum/handler.go:495-518`
**问题**: SCIM/SAML/OIDC/LDAP 四个连接测试方法仅打日志后直接将 provider 状态置为 `StatusActive`，未执行任何真实连接探测。
**影响**: 错误配置的 IdP 被标记为 Active，结合 1.2 的伪验证，进一步掩盖问题。
**修复**: 实现真实的连接探测（SCIM `/Users?count=1`、SAML metadata、OIDC discovery、LDAP bind）。

### 1.18 AUTO_RECOVER_DIRTY 默认 true 会丢数据
**文件**: `internal/container/container.go:613` + `internal/database/migration.go:262-299`
**问题**: 默认 `true` 时迁移失败自动 `m.Force(forceVersion)` 强制回退版本，会掩盖 schema 不一致。若 dirty migration 部分应用了 DDL，Force 后 schema 处于半应用状态，下次启动又"成功"通过迁移检查。
**修复**: 生产默认 `false`，仅开发默认 `true`；Force 前先 `pg_dump` 备份。

---

## 二、HIGH 高优先级问题（P1，强烈建议修复）

### 安全类

#### 2.1 邀请 Token 明文存储在数据库
**文件**: `internal/application/service/tenant_invitation.go:480-489`
**问题**: 邀请 token 明文持久化，DB 泄露后所有未消费的邀请 token 立即可用。
**修复**: 仅存储 SHA-256 哈希；缩短 TTL 到 24-48 小时；token 拆为 selector + verifier。

#### 2.2 RBAC EnableRBAC=false 时 fail-open
**文件**: `internal/middleware/rbac.go`
**问题**: `EnableRBAC=false` 时所有 guard 退化为日志-only，**所有人可访问所有租户的所有资源**。
**修复**: 生产环境强制 `EnableRBAC=true`，fail-open 仅在 dev 通过专门环境标记控制。

#### 2.3 Signer 使用 MD5 + math/rand
**文件**: `internal/models/utils/signer.go:65-77`
**问题**: MD5 已被破解；`math/rand` 生成 nonce 可预测；API Key 既作签名密钥又作身份标识。
**修复**: 改用 `HMAC-SHA256` + `crypto/rand`；分离身份标识（AppID）与签名密钥（API Secret）。

#### 2.4 nginx 缺少 Content-Security-Policy
**文件**: `frontend/nginx.conf:32-35`
**修复**: 添加 CSP 头：
```nginx
add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self';" always;
```

#### 2.5 JWT Token 明文存储于 localStorage
**文件**: `frontend/src/stores/auth.ts:208-216`
**影响**: 任何 XSS 漏洞可读取 token 实现长期账户接管。
**修复**: refresh token 改为 httpOnly cookie；access token 改为内存存储。

#### 2.6 trustedProxies 默认信任全部内网
**文件**: `internal/router/router.go:1296-1305`
**影响**: k8s 集群内任意 Pod 可伪造 `X-Forwarded-For` 绕过限流、伪造 ClientIP。
**修复**: 默认仅信任 `127.0.0.1/32`，强制部署时显式配置。

#### 2.7 ReDoS 风险（用户控制 regex）
**文件**: `internal/agent/tools/wiki_tools.go:764`, `wiki_read_source_doc.go:154`
**问题**: `regexp.Compile("(?i)" + query)` 将用户输入直接编译为正则。
**修复**: 使用 `regexp.QuoteMeta(query)` 或字符串子串匹配。

### 并发与正确性

#### 2.8 Redis 锁续期 goroutine 竞态条件
**文件**: `internal/application/service/wiki_ingest_batch.go:164-175`
**问题**: 续期 goroutine 使用 `context.Background()` 而非 `lockCtx`，defer 中的 `Del` 可能在续期 goroutine 仍在运行时执行，导致锁丢失。
**修复**: 使用 `lockCtx` + `sync.WaitGroup` 确保 defer `Del` 在续期 goroutine 退出后执行。

#### 2.9 AgentEngine 关键字段无锁保护
**文件**: `internal/agent/engine.go:35-52, 152-158, 224`
**问题**: `lastUsage`、`lastSentMsgCount`、`thinkingTracker` 字段在并发 Execute 调用时存在数据竞争。
**修复**: 通过 context 传递局部变量，或使用 `sync.Mutex` 保护。

#### 2.10 语义缓存 HitCount 竞态条件
**文件**: `internal/application/service/semantic_cache_memory.go:86-88`
**问题**: `bestMatch.HitCount++` 在 RLock 下执行但 increment 无写锁保护。
**修复**: 使用 `atomic.AddInt64` 或升级为写锁。

#### 2.11 30+ 处无 ok 检查的 type assertion panic 风险
**文件**: `knowledge_create.go`, `knowledge_delete.go`, `knowledge_faq.go`, `knowledge_process.go`, `knowledge.go`
**问题**: `ctx.Value(types.TenantIDContextKey).(uint64)` 等类型断言未检查 ok，context 缺失会 panic。
**修复**: 统一使用 `val, ok := ctx.Value(key).(type); if !ok { return error }`，封装 `types.MustTenantIDFromContext(ctx)`。

#### 2.12 Embedding Batcher 忽略调用方 context
**文件**: `internal/application/service/embedding_batcher.go:223-225`
**问题**: 使用 `context.Background()` + 30s timeout，不绑定到请求 context，用户取消后仍产生 API 调用成本。
**修复**: `ctx, cancel := context.WithTimeout(batch[0].Ctx, 30*time.Second)`。

#### 2.13 Anthropic Provider 合并 Cache Token，丢失区分信息
**文件**: `internal/models/chat/anthropic.go:499-548`
**问题**: `CachedTokens: cachedTokens + cacheCreationTokens` 合并了 cache creation 和 cache read，丢失区分信息。而 `types.TokenUsage` 已有独立字段未填充。
**影响**: 成本计算错误（creation 通常更贵），无法统计缓存命中率。
**修复**: 分别填充 `CacheReadTokens` 和 `CacheCreationTokens` 字段。

#### 2.14 模型路由跨租户 fallback 可能导致数据泄漏
**文件**: `internal/application/service/model_router.go:264-281`
**问题**: 租户未配置模型时 fallback 到 `tenantID=0` 系统级模型，可能包含其他租户私有模型。
**修复**: 系统模型添加 `is_public` 字段，Fallback 时只查 `tenant_id=0 AND is_public=true`。

### 性能问题

#### 2.15 N+1 查询问题（系统性，27 处）
**主要文件**:
- `wiki_page.go:901-933, 738-775, 440-596, 668-736`（updateInLinks、RebuildLinks、GetGraph ListAll、GetStats）
- `knowledgebase_search_shared.go:59-76, 122-136`
- `session_knowledge_qa.go:264-277, 910-957`
- `knowledge.go:614-628`
- `knowledge_faq.go:1363-1384`
- `chat_pipeline/search.go:547`
- `chat_pipeline/wiki_boost.go:87-92`

**修复**: 统一使用批量 `WHERE id IN (...)` 查询；GetStats 改用数据库聚合查询。

#### 2.16 语义缓存 Redis O(n) 全扫描
**文件**: `internal/application/service/semantic_cache_redis.go:48-101`
**问题**: 每次查询加载 tenant 下所有 entry 的 JSON 并计算 cosine 相似度。
**修复**: 使用 Redis Vector Set（Redis 8.0+）或 RediSearch 模块建立向量索引。

#### 2.17 大表回填迁移无批处理会锁表
**文件**: 
- `migrations/versioned/000043_tenant_rbac.up.sql:55-100`
- `migrations/versioned/000051_custom_agents_creator_backfill.up.sql:26-46`

**问题**: 全表 UPDATE/INSERT 在 10w+ 行时长时间持锁。
**修复**: 分批处理（每批 1000 行）+ `SET LOCAL statement_timeout = '5min'`。

#### 2.18 迁移 000059 HNSW 索引非并发
**文件**: `migrations/versioned/000059_embeddings_hnsw.up.sql:30-44`
**问题**: 在 DO 块内 `CREATE INDEX` 持 `AccessExclusiveLock`，百万行表可能锁几十分钟。
**修复**: 拆出 DO 块，单独 `CREATE INDEX CONCURRENTLY IF NOT EXISTS`。

### 部署与运维

#### 2.19 容器服务直接暴露到宿主机
**文件**: `docker-compose.yml:375-527`
**问题**: Neo4j、Milvus、Weaviate、MinIO、Doris 等数据服务端口默认 `0.0.0.0`，云主机等于公网暴露。
**修复**: 所有数据服务改 `127.0.0.1:<port>:<container_port>`，仅 frontend + app 暴露。

#### 2.20 Weaviate 匿名访问 + Milvus 关闭 seccomp
**文件**: `docker-compose.yml:467, 434-435`
**修复**: Weaviate 启用 APIKey；Milvus 改用官方 seccomp profile。

#### 2.21 Helm chart 与 docker-compose 版本漂移
**文件**: `helm/values.yaml` vs `docker-compose.yml`
| 组件 | helm | docker-compose |
|---|---|---|
| app/frontend/docreader | `wechatopenai/xinwiki-*` | `wechatopenai/weknora-*` |
| postgresql | `paradedb/paradedb:v0.18.9-pg17` | `v0.22.2-pg17` |
| redis | `redis:7-alpine` | `redis:7.0-alpine` |

**修复**: 统一镜像命名 + 版本 tag，共用一份版本清单。

#### 2.22 Helm chart 所有组件默认以 root 运行
**文件**: `helm/values.yaml:26-34, 84, 170, 217, 263, 308`
**修复**: 所有组件 `runAsNonRoot: true` + `runAsUser: <non-zero>` + `readOnlyRootFilesystem: true`。

#### 2.23 GitHub Actions 未固定到 SHA
**文件**: `.github/workflows/cli.yml`, `cli-e2e.yml`
**问题**: `actions/checkout@v6` 使用 tag 而非 commit SHA，tag 可被劫持（已有真实事件 tj-actions/changed-files）。
**修复**: 固定到完整 40 位 SHA + 注释版本号。

#### 2.24 CI 缺少关键质量门禁
**问题**: 仅 `go test -race`，缺少 `golangci-lint`、`govulncheck`、`go mod tidy`、覆盖率上传。
**修复**: 增加 jobs 覆盖 lint、漏洞扫描、依赖漂移、覆盖率。

#### 2.25 /health 端点不检查依赖可达性
**文件**: `internal/router/router.go:128-130`
**问题**: 始终返回 200，DB 宕机时 Pod 仍被标记 ready 接收流量。
**修复**: 拆分为 `/healthz`（liveness）+ `/readyz`（readiness 检查 DB/Redis/docreader）。

#### 2.26 SYSTEM_AES_KEY 长度错误仅 warn 不阻塞
**文件**: `internal/runtime/startup.go:135-139`
**问题**: 错误长度时加密被静默禁用，已加密字段以明文存库，但应用照常启动。
**修复**: 长度 != 32 时 `return fmt.Errorf("SYSTEM_AES_KEY must be 32 bytes")` 阻断启动。

#### 2.27 JWT_SECRET 仅检查存在性不检查长度
**文件**: `internal/runtime/startup.go:88`
**修复**: `len(JWT_SECRET) < 32` 时阻塞启动。

#### 2.28 后台 goroutine 用 context.Background() 无法被取消
**文件**: `internal/container/container.go:324-331`
**问题**: scheduler 内部用 `context.Background()`，主进程 shutdown 信号无法传播，goroutine 泄漏。
**修复**: 从 `main.go` 注入 shutdown ctx 到 container。

#### 2.29 公开邀请注册端点未限流
**文件**: `frontend/src/utils/request.ts:79-84`
**修复**: 后端在 `internal/middleware/` 添加 IP 级别速率限制（已有 `internal/ratelimit/limiter.go`，需在路由配置应用）。

#### 2.30 D3 模型路由与 Prompt 版本化宣称但未交付
**文件**: `README_CN.md:48-49` vs `docs/D4-review-report.md:204-241`
**问题**: README 宣称「模型路由 + Prompt 版本化管理」，实际 D4-review-report 标记为 P1 缺失。
**修复**: 从 README 移除未实现的功能宣称。

---

## 三、MEDIUM 中等问题（P2，应排期修复）

### 代码质量

- **3.1** `wiki_page.go` 1872 行单文件，混合 CRUD/link/graph/stats/ACL 多个关注点，应拆分
- **3.2** `wiki_ingest_batch.go` ProcessWikiIngest 700+ 行单函数，应拆分为 map/reduce/finalize
- **3.3** `knowledge_process.go` processChunks 250+ 行
- **3.4** `menu.vue` 1900+ 行单组件，职责过重（导航/权限/搜索/智能体切换混合）
- **3.5** `streame.ts` 死代码（`renderTimer` 未使用）+ 类型错误（`onChunk` 签名不匹配）
- **3.6** `package.json` name 拼写错误 `"knowledage-base"`
- **3.7** `Formula/xinwiki-lite.rb` 类名 `WeknoraLite` 拼写错误 + sed 跨平台问题

### 性能

- **3.8** `vectorstore_router.go:517-583` lsnTracker waiter 清理不完整，goroutine 泄漏风险
- **3.9** `load_balancer.go:37` RoundRobinBalancer map 迭代顺序不确定，轮询分布不均匀
- **3.10** `load_balancer.go:61-86` LeastConnectionsBalancer connectionMap 无界增长，内存泄漏
- **3.11** `semantic_cache_memory.go:120-121` FIFO 驱逐而非 LRU，缓存命中率低
- **3.12** `milvus/repository.go:730-788` KeywordsRetrieve 跨所有 collection 串行搜索
- **3.13** `doris/repository.go:445-507` CopyIndices 使用 OFFSET 分页，深分页性能差
- **3.14** `conflict_detection.go:144-183` listAllChunksForKB 一次性加载所有 chunks，大型 KB OOM 风险
- **3.15** `knowledge_util.go:339` downloadFileFromURL 每次创建新 http.Client
- **3.16** `chat_pipeline/query_expansion.go:198` 正则每次调用编译，应包级预编译
- **3.17** `llm_call_log.go` 缺少复合索引 `(tenant_id, created_at)`，成本聚合查询大表全扫

### 配置与兼容性

- **3.18** `cost_tracking.go:232-387` QueryCostTrend 使用 MySQL 专用 `DATE_FORMAT`，SQLite/PostgreSQL 报错
- **3.19** `settings.ts:108` JSON.parse localStorage 未做异常处理，损坏 JSON 导致白屏
- **3.20** `request.ts:19` X-Request-ID 实例默认值冗余，所有请求复用同一 ID
- **3.21** `App.vue:228-242` 使用 `// @ts-ignore` 绕过类型检查，Wails API 无类型声明
- **3.22** `App.vue:6-7` import 了不存在的组件（`manual-knowledge-editor.vue`、`UploadConfirmHost.vue`），编译失败

### 数据库迁移

- **3.23** `000043_tenant_rbac.up.sql:77` `u.tenant_id <> 0` 隐式假设哨兵值
- **3.24** sqlite 仅 1 个 init 迁移，Lite 模式 schema 永远停在 v0
- **3.25** `000065_llm_cost_tracking.up.sql:41-47` 索引膨胀（7 个独立索引，部分与前缀组合索引重复）
- **3.26** 无 down 迁移测试与回滚演练

### 其他

- **3.27** `cost_tracking.go:673-675` Fallback 百分位算法数学错误，p95 可能 > p99
- **3.28** `model_router.go:467-507` selectBalanced 评分公式未归一化，tierScore 权重实际是其他 4 倍
- **3.29** `think.go:347-488` 使用线性退避而非指数退避 + 抖动
- **3.30** `act.go:459, 492` thinkingTracker 重复 EndStep
- **3.31** `prompt_template.go:272-314` RenderTemplate 未启用模板沙箱
- **3.32** `ollama.go:249-263` ChatStream token 统计错误（EvalCount 是累计总 token）
- **3.33** `model_router.go` selectCheapestModel 等修改输入 slice（副作用）
- **3.34** `approval/gate.go:290-400` 超时/取消分支存在潜在阻塞，goroutine 泄漏风险

---

## 四、LOW 低优先级问题（P3，机会修复）

- **4.1** `wiki_ingest_dedup.go` Jaccard bigram 可用 pg_trgm 替代
- **4.2** `wiki_ingest_taxonomy.go:242-256` 手动 cosine 相似度，未使用 SIMD
- **4.3** `keywords_vector_hybrid_indexer.go:104` hardcoded batchSize=40
- **4.4** `prompt_template.go:235-241` ListTemplateVersions 使用 O(n²) 冒泡排序
- **4.5** `thinking.go:110-118` chatTemplateKwargs 修改输入 request
- **4.6** `conflict_detection.go` confidence 硬编码
- **4.7** `provider.go:83` 类型名仍为 `weKnoraCloudProvider`，未跟随品牌迁移
- **4.8** `cost_tracking.go` GetCostDashboard 匿名用户不计入 top users，看板数据矛盾
- **4.9** `README_CN.md:20` HTML 标签语法错误（多余 `</a>` 闭合标签）
- **4.10** `README_CN.md:271` 声称不存在的 `internal/infrastructure/` 目录
- **4.11** `SECURITY.md` 内容不完整（缺少加密策略、RBAC 角色矩阵链接）
- **4.12** `cli/AGENTS.md` 标题与正文不一致（XinWiki vs weknora）
- **4.13** `index.html:14-15` favicon 路径错误 + type 不匹配
- **4.14** `streame.ts:180-182` SSE 重连缺退避策略
- **4.15** 品牌名混用：XinWiki / WeKnora / weknora 跨多文件残留（Makefile、CHANGELOG、docker-compose、cli/AGENTS.md）
- **4.16** `chromedp/cdproto` 使用伪版本，上游 breaking change 会破坏构建
- **4.17** `go.mod` 同时存在废弃 JWT 库 `form3tech-oss/jwt-go` 与新版本 `golang-jwt/jwt/v5`
- **4.18** Dependabot 配置遗漏 docker / gitsubmodule ecosystem
- **4.19** 仅 `/cli` 路径有 CI，server/frontend/docreader 无 CI 守护
- **4.20** Helm 缺少 PodDisruptionBudget、NetworkPolicy
- **4.21** Helm Ingress TLS 默认关闭
- **4.22** `docker-compose.yml:40` config.yaml 默认 rw 挂载，应 `:ro`
- **4.23** `desktopPreferredLANIPv4()` 依赖 `8.8.8.8:80`，离线场景失败
- **4.24** `serverStartedAt` 全局变量无 mutex
- **4.25** `bootstrapSystemAdmin` 缺少审计日志

---

## 五、架构亮点（值得肯定的设计）

### 5.1 Chat Pipeline 插件架构
`chat_pipeline/chat_pipeline.go` 的 EventManager + Plugin 中间件链模式设计优雅，支持灵活的插件注册和链式调用，闭包正确捕获循环变量，`PluginError` 的 clone/WithError 模式保证错误对象不可变性。

### 5.2 Retriever 三层架构
`retriever/composite.go` + `factory.go` + `registry.go` 构成清晰的三层架构：
- Registry 管理引擎生命周期（双 map byEngineType + byStoreID）
- Factory 封装 ownership 验证和 sentinel error
- Composite 使用 errgroup 实现并发检索，正确创建局部变量副本避免闭包捕获问题

### 5.3 ScoreNormalizer 跨引擎归一化
`retriever/normalizer.go` 的 `EngineAwareNormalizer` 对每个向量存储引擎的 score 范围进行了详尽文档化（引用上游源码版本），`clamp01` 正确处理 NaN/Inf 边界情况。

### 5.4 SpanTracker 可观测性体系
`knowledge_span_tracker.go` 的 span 树设计（root/stage/subspan/generation）镜像 Langfuse 词汇表，支持跨进程 span 桥接，`touchKnowledgeHeartbeat` 优化为仅 root/stage 级别触发避免写放大。

### 5.5 HousekeepingService 多层防御
`knowledge_housekeeping.go` 的三阶段检查（updated_at + span heartbeat + queue probe）有效区分"真正卡住"和"背压排队"，fail-safe 方向一致。

### 5.6 Doris 兼容模式设计
`doris/repository.go` 的 `dorisCompatMode`（auto/legacy/inner_product_duplicate）优雅处理 Doris 不同版本对 ANN 函数和 UNIQUE KEY 表的支持差异。

### 5.7 OAuth state / SSRF 防护
- OAuth state 单次使用（`GetDel` 原子操作）+ 10 分钟 TTL + Redis 命名空间隔离
- SSRF 防护覆盖私网 IP、云元数据端点、端口黑名单、白名单旁路、Fail-closed 默认分支

### 5.8 凭证子资源模式
`/models/:id/credentials`、`/mcp-services/:id/credentials` 凭证与主资源分离，仅返回"是否已配置"元数据，`secutils.SanitizeForLog` 用于日志脱敏。

### 5.9 OS Keyring 凭证存储
CLI 优先使用 OS Keychain（macOS Keychain / libsecret / KWallet），Fallback 到 0600 权限的 FileStore，命名空间隔离 `xinwiki:<profile>:<key>`。

### 5.10 RAG 最佳实践
- **Parent-Child Chunking**：父块 4096 + 子块 384（20% overlap）分层切片
- **MMR 去冗余**：lambda=0.7 平衡相关性与多样性，pre-computes token sets 缓解 O(n²)
- **Threshold Degradation**：rerank 无结果时自动降低阈值（×0.7，下限 0.3）重试
- **双重去重**：按 chunkID + content signature
- **RAG 引用追溯**：Pass 0 候选 slug 抽取 + Pass 1..N chunk 引用分类
- **语义缓存隔离**：缓存 key 包含 tenantID + ACL 过滤 + 主动失效 + Redis 降级 + 可禁用

---

## 六、下一步优化建议（按优先级）

### P0 - 立即修复（上线前必须完成，预计 3-5 个工作日）

#### 安全阻断项（1-2 天）
1. 修复 CORS 配置（1.1）— 改为可配置 origin 列表
2. UUM SAML/OIDC 伪验证改为返回 error（1.2）
3. TENANT_AES_KEY 启动期校验（1.3）
4. 移除 `.env.example` 和 `docker-compose.yml` 中所有默认 secret（1.7, 1.8）
5. SYSTEM_AES_KEY 长度错误时 fail-fast（2.26）
6. JWT_SECRET 长度校验（2.27）
7. RBAC fail-open 仅 dev 启用（2.2）
8. Signer 迁移到 HMAC-SHA256 + crypto/rand（2.3）

#### Docker 安全修复（1 天）
9. 5 个 Dockerfile 创建非 root 用户（1.12）
10. Dockerfile.app 移除 curl|sh 远程脚本（1.13）
11. 所有容器服务不暴露到宿主机公网（2.19）
12. Weaviate 启用 APIKey + Milvus 启用 seccomp（2.20）

#### 关键 Bug 修复（1-2 天）
13. 修复 Embedding Batcher 结果分发错位（1.4）— **影响检索准确性**
14. 修复 Thinking Tracker 指针语义（1.5）
15. 修复 Redis 锁续期竞态（2.8）
16. 修复语义缓存 HitCount 竞态（2.10）
17. 修复 30+ 处 type assertion panic 风险（2.11）— 统一封装辅助函数

#### 启动与运维（半天）
18. 修复 Postgres 连接池无上限（1.10）
19. 迁移失败阻断启动（1.11）
20. /health 拆分 liveness/readiness（2.25）
21. AUTO_RECOVER_DIRTY 默认 false（1.18）

#### 文档修复（半天）
22. 核实前端组件是否存在，修复 App.vue 失效 import（1.9）
23. README 移除未实现的功能宣称（2.30）

### P1 - 强烈建议（1-2 周内）

#### 持久化补全
24. 创建 `prompt_template_repo.go` 和 `model_router_repo.go`（1.6）
25. 实现 A5 事件驱动 ACL 重算（1.15）
26. 实现 E3 Permission Leakage CI 红线（1.16）

#### 性能优化
27. 修复 27 处 N+1 查询（2.15）— 统一批量查询重构
28. 语义缓存 Redis 改用向量索引（2.16）
29. 大表回填迁移分批处理（2.17）
30. HNSW 索引改 CONCURRENTLY（2.18）

#### 安全加固
31. 邀请 Token 哈希存储（2.1）
32. JWT Token 改 httpOnly cookie（2.5）
33. trustedProxies 默认仅 127.0.0.1（2.6）
34. ReDoS 防护（2.7）
35. 公开端点限流（2.29）

#### CI/CD 补全
36. GitHub Actions 固定到 SHA（2.23）
37. CI 增加 golangci-lint + govulncheck + go mod tidy（2.24）
38. 新增 server/frontend/docker CI 守护（3.19）

#### 部署统一
39. Helm chart 所有组件 runAsNonRoot（2.22）
40. Helm 与 docker-compose 镜像版本对齐（2.21）
41. Helm secrets 增加 CRYPTO_MASTER_KEY / CRYPTO_SALT

#### 数据库迁移
42. 迁移 dirty force 前先 pg_dump 备份
43. 增加 down 迁移 CI 测试
44. 索引膨胀优化（删除单列索引，保留复合索引）

### P2 - 中期改进（1 个月内）

#### 代码质量
45. 拆分 `wiki_page.go`、`menu.vue`、`wiki_ingest_batch.go` 等超大文件
46. 清理死代码（streame.ts、menu.vue）
47. 修复品牌名混用（统一为 XinWiki/xinwiki）
48. 添加类型声明，移除 @ts-ignore

#### 性能优化
49. vectorstore_router waiter 清理完善（3.8）
50. LeastConnectionsBalancer 内存泄漏修复（3.10）
51. 语义缓存改 LRU 驱逐（3.11）
52. Milvus 跨 collection 并发搜索（3.12）
53. llm_call_log 复合索引（3.17）

#### 跨数据库兼容
54. cost_tracking.go 方言适配（3.18）
55. sqlite 走 versioned 迁移（3.24）

#### 可观测性
56. 后台 goroutine 注入 shutdown ctx（2.28）
57. startupEnvVars 补全敏感变量
58. bootstrapSystemAdmin 审计日志

### P3 - 长期演进（季度规划）

59. 实现真实 UUM SAML/OIDC/LDAP/SCIM 验证
60. 实现真实连接测试（1.17）
61. 前端组件完整重构（三栏布局、思维链可视化）
62. 文档与代码同步（README、架构图、ROADMAP）
63. 测试覆盖率提升（关键新功能 model_router、prompt_template、conflict_detection、cost_tracking、embedding_batcher 单元测试）
64. 部署 PodDisruptionBudget、NetworkPolicy
65. Helm Ingress 默认开启 TLS
66. 替换 textract 为现代库
67. 桌面应用 IP 探测 fallback

---

## 七、修复路线图

```
Week 1 (P0 安全 + Docker + 关键 Bug)
├── Day 1-2: 安全阻断项 (CORS/UUM/AES/Secret/Signer/RBAC)
├── Day 3: Docker 安全 (5 个 Dockerfile + 容器端口)
├── Day 4: 关键 Bug (Embedding Batcher/Thinking Tracker/Redis 锁/缓存竞态)
└── Day 5: 启动运维 (连接池/迁移/健康检查) + 文档修复

Week 2-3 (P1 持久化 + 性能 + CI/CD)
├── 持久化补全 (prompt_template_repo/model_router_repo/A5/E3)
├── N+1 查询批量重构 (27 处)
├── 语义缓存向量索引
├── 迁移分批 + CONCURRENTLY
├── CI/CD 补全 (SHA 固定/lint/vulncheck)
└── Helm 与 docker-compose 对齐

Week 4-6 (P2 代码质量 + 性能 + 兼容性)
├── 超大文件拆分
├── 死代码清理 + 品牌统一
├── vectorstore_router/load_balancer 内存泄漏修复
├── 跨数据库方言适配
└── 可观测性完善

Quarter 2 (P3 长期演进)
├── UUM 真实实现
├── 前端组件重构
├── 测试覆盖率提升
└── 文档全面同步
```

---

## 八、关键文件清单（按修复优先级）

### P0 必修
- `internal/router/router.go`（CORS / health / trustedProxies / swagger）
- `internal/auth/uum/handler.go`（SAML/OIDC 伪验证 + 连接测试桩）
- `internal/application/service/tenant.go`（AES key panic）
- `internal/application/service/embedding_batcher.go`（结果错位 + context）
- `internal/agent/thinking/tracker.go`（指针语义）
- `internal/application/service/prompt_template.go`（持久化缺失）
- `internal/runtime/startup.go`（env 校验）
- `internal/container/container.go`（连接池 + 迁移失败 + 后台 goroutine）
- `internal/middleware/rbac.go`（fail-open）
- `internal/models/utils/signer.go`（MD5 → HMAC-SHA256）
- `.env.example` + `docker-compose.yml` + `docker-compose.dev.yml`（默认 secret）
- 5 个 Dockerfile（非 root 用户）
- `frontend/src/App.vue`（失效 import）
- `README_CN.md`（移除未实现宣称）

### P1 必修
- `internal/application/service/wiki_ingest_batch.go`（Redis 锁续期）
- `internal/application/service/semantic_cache_memory.go`（HitCount 竞态）
- `internal/application/service/semantic_cache_redis.go`（向量索引）
- `internal/application/service/model_router.go`（跨租户 fallback + selectBalanced 归一化）
- `internal/models/chat/anthropic.go`（Cache Token 区分）
- `internal/agent/engine.go`（数据竞争）
- 跨文件 30+ 处 type assertion（统一封装辅助函数）
- 跨文件 27 处 N+1 查询（批量重构）
- `migrations/versioned/000043_*.sql`、`000051_*.sql`、`000059_*.sql`（分批 + CONCURRENTLY）
- `.github/workflows/*.yml`（SHA 固定 + lint + vulncheck）
- `helm/values.yaml`（runAsNonRoot + tag 固定 + secrets 补全）

### P2 应修
- `internal/application/service/wiki_page.go`（拆分 + N+1）
- `frontend/src/components/menu.vue`（拆分）
- `frontend/src/api/chat/streame.ts`（死代码 + 类型 + 重连）
- `frontend/src/stores/settings.ts`（localStorage 异常处理）
- `internal/application/service/cost_tracking.go`（方言适配 + 百分位算法）
- `internal/application/service/vectorstore_router.go`（waiter 清理）
- `internal/application/service/load_balancer.go`（内存泄漏）

---

## 九、审查涉及的文件清单

### 后端核心
- **Wiki/RAG/知识库**: 50+ 文件（wiki_ingest_*, wiki_page, wiki_score_refresh, wiki_linkify, wiki_lint, knowledgebase_search, session_knowledge_qa, chat_pipeline/*, retriever/*, semantic_cache*, vectorstore_router*, load_balancer, embedding_batcher*）
- **Agent/LLM/Cost**: 30+ 文件（agent/*, models/chat/*, models/utils/signer, cost_tracking, model_router, prompt_template, conflict_detection, llm_call_log）
- **RBAC/安全**: 30+ 文件（auth/rbac/*, auth/uum/*, acl/*, middleware/*, container/*, runtime/*, router/*）

### 前端
- App.vue, main.ts, router/, stores/, api/, utils/, components/, index.html, nginx.conf, vite.config.ts, package.json

### 部署与 CI
- 5 个 Dockerfile, docker-compose.yml, .github/workflows/*, helm/*, Makefile, .env.example

### 文档
- README.md, README_CN.md, SECURITY.md, CHANGELOG.md, docs/*, AGENTS.md

### 数据库
- 65 个 migration SQL 文件（抽查 000043/000051/000059/000063/000065）

---

## 十、总体结论

XinWiki 项目在架构设计上有诸多亮点（Chat Pipeline 插件体系、SpanTracker 可观测性、ScoreNormalizer 跨引擎归一化、RAG 最佳实践、OAuth/SSRF 防护），但**生产就绪度不达标**：

1. **安全合规严重不足**：7 个 CRITICAL 安全漏洞，包括 CORS 配置违规、UUM 伪验证、默认密钥入仓、容器以 root 运行
2. **关键功能未完成**：Prompt 模板无持久化、A5 ACL 重算缺失、E3 CI 红线缺失、UUM 真实验证未实现
3. **并发安全 bug**：Embedding Batcher 结果错位、Thinking Tracker 指针错误、Redis 锁续期竞态、缓存 HitCount 竞态
4. **性能问题系统性**：27 处 N+1 查询、语义缓存 O(n) 全扫描、迁移锁表
5. **文档与代码严重脱节**：README 宣称的功能组件实际不存在，前端无法编译
6. **CI/CD 不完整**：仅 /cli 路径有 CI，无 lint/vulncheck/覆盖率

**建议**：按 P0 → P1 → P2 → P3 优先级修复，P0 修复完成后达到 70 分以上方可考虑小范围灰度上线。所有 P1 修复完成后达到 85 分以上可全面上线。

---

**报告生成时间**: 2026-06-29
**审查工具**: 5 个并行子代理 + 静态代码审查 + 安全审计
**审查文件数**: 200+ 文件
**发现问题数**: 18 CRITICAL + 30 HIGH + 34 MEDIUM + 25 LOW = 107 个问题
