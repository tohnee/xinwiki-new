# XinWiki Developer SOP

Standard Operating Procedures for common development tasks.

## Table of Contents

- [1. Adding a New API Endpoint](#1-adding-a-new-api-endpoint)
- [2. Adding a New Service](#2-adding-a-new-service)
- [3. Adding a New Vector Store Backend](#3-adding-a-new-vector-store-backend)
- [4. Adding a New LLM Provider](#4-adding-a-new-llm-provider)
- [5. Creating a Database Migration](#5-creating-a-database-migration)
- [6. Adding a New Agent Tool](#6-adding-a-new-agent-tool)
- [7. Adding a New Frontend Page](#7-adding-a-new-frontend-page)
- [8. Debugging the ReAct Agent Loop](#8-debugging-the-react-agent-loop)
- [9. Debugging Wiki Compilation](#9-debugging-wiki-compilation)
- [10. Release Process](#10-release-process)

---

## 1. Adding a New API Endpoint

**Steps:**

1. **Define DTOs** in `internal/handler/dto/` - request/response structs with `json` tags and `binding` validation tags.

2. **Add handler method** in `internal/handler/` - create or extend a handler file:
   ```go
   func (h *FooHandler) CreateFoo(c *gin.Context) {
       ctx := c.Request.Context()
       var req dto.CreateFooRequest
       if err := c.ShouldBindJSON(&req); err != nil {
           c.Error(apperrors.NewValidationError("invalid request").WithDetails(err.Error()))
           return
       }
       result, err := h.fooSvc.Create(ctx, &req)
       if err != nil {
           c.Error(err)
           return
       }
       c.JSON(http.StatusOK, dto.ToFooResponse(result))
   }
   ```

3. **Register route** in `internal/router/router.go` - add to `RouterParams` struct if a new handler/service is needed, then wire the route with RBAC permission:
   ```go
   fooGroup := v1.Group("/foos")
   fooGroup.Use(middleware.Auth(params.Config), middleware.Tenant())
   fooGroup.POST("/", g.Editor(), params.FooHandler.CreateFoo)
   ```

4. **Add Swagger annotations** above the handler method (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Router`).

5. **Regenerate Swagger docs**: `make docs`

6. **Write tests** - test the handler with a stubbed service interface.

**Checklist:**
- [ ] DTO defined with validation tags
- [ ] Handler method with error handling (use `c.Error()` + `apperrors`)
- [ ] Route registered with correct RBAC permission (`g.Viewer()`, `g.Editor()`, `g.Owner()`, or `g.Admin()`)
- [ ] Swagger annotations added
- [ ] Tests written
- [ ] `make docs` regenerated

---

## 2. Adding a New Service

**Steps:**

1. **Define interface** in `internal/types/interfaces/` - this is the DI contract:
   ```go
   type FooService interface {
       Create(ctx context.Context, req *types.Foo) (*types.Foo, error)
       GetByID(ctx context.Context, id uint64) (*types.Foo, error)
   }
   ```

2. **Implement service** in `internal/application/service/foo_service.go`:
   ```go
   type fooService struct {
       repo interfaces.FooRepository
   }
   func NewFooService(repo interfaces.FooRepository) interfaces.FooService {
       return &fooService{repo: repo}
   }
   ```

3. **Define repository interface** in `internal/types/interfaces/` (if persistence needed).

4. **Implement repository** in `internal/application/repository/foo_repository.go` using GORM.

5. **Register in DI container** in `internal/container/container.go`:
   ```go
   must(c.Provide(service.NewFooService))
   must(c.Provide(repository.NewFooRepository))
   ```

6. **Add to RouterParams** (or handler params) if the service is consumed by a handler.

**Checklist:**
- [ ] Interface defined in `internal/types/interfaces/`
- [ ] Service implemented with interface dependency injection
- [ ] Repository implemented (if persistence needed)
- [ ] DI container registration added
- [ ] No circular imports (service depends on interfaces, not concrete types)

---

## 3. Adding a New Vector Store Backend

**Steps:**

1. **Create driver directory** under `internal/application/repository/retriever/<vendor>/`.

2. **Implement the `RetrieveEngineService` interface** - defined in `internal/types/interfaces/`. Key methods:
   - `StoreVectors(ctx, chunks)` - upsert embeddings
   - `Search(ctx, query, opts)` - vector similarity search
   - `DeleteVectors(ctx, ids)` - remove by IDs
   - `HealthCheck(ctx)` - connectivity check

3. **Register in engine factory** - `internal/container/engine_factory.go`:
   ```go
   case types.DriverMyNewStore:
       return newMyNewStoreEngine(store, db)
   ```

4. **Add driver constant** to `internal/types/` (e.g., `DriverMyNewStore = "mystore"`).

5. **Add config example** to `.env.example` with `RETRIEVE_DRIVER=mystore` documentation.

6. **Write tests** - at minimum, interface conformance test.

**Reference implementations:** `postgres/` (pgvector), `milvus/`, `qdrant/` are good templates.

---

## 4. Adding a New LLM Provider

**Steps:**

1. **Create provider adapter** in `internal/models/provider/` - implement the chat/embedding/rerank interface.

2. **Register in provider registry** - typically in `internal/models/chat/` or `internal/models/provider/registry.go`.

3. **Add model config** to `config/builtin_models.yaml.example` with provider name, context window, pricing.

4. **Add to cost tracking** - update `internal/application/service/cost_tracking.go` pricing table if applicable.

5. **Test** - verify streaming and non-streaming responses, token counting, error handling.

**Reference:** OpenAI and DeepSeek providers are good templates.

---

## 5. Creating a Database Migration

**Steps:**

1. **Generate migration files:**
   ```bash
   make migrate-create name=add_foo_table
   ```
   This creates `migrations/versioned/NNNNNN_add_foo_table.up.sql` and `.down.sql`.

2. **Write UP migration** - forward schema change:
   ```sql
   CREATE TABLE foos (
       id BIGSERIAL PRIMARY KEY,
       name VARCHAR(255) NOT NULL,
       tenant_id BIGINT NOT NULL REFERENCES tenants(id),
       created_at TIMESTAMPTZ DEFAULT NOW(),
       updated_at TIMESTAMPTZ DEFAULT NOW()
   );
   CREATE INDEX idx_foos_tenant ON foos(tenant_id);
   ```

3. **Write DOWN migration** - reversible rollback:
   ```sql
   DROP TABLE IF EXISTS foos;
   ```

4. **Test locally:**
   ```bash
   make migrate-up      # Apply
   make migrate-down    # Roll back - verify it works
   make migrate-up      # Re-apply
   ```

5. **If SQLite (Lite mode) variant needed** - create equivalent in `migrations/sqlite/`.

**Rules:**
- Never modify an existing migration that has been merged - create a new one
- Always include `down.sql` (even if it's just `DROP TABLE`)
- Use `IF EXISTS` / `IF NOT EXISTS` for idempotency
- Add indexes for foreign keys and frequently-filtered columns
- Avoid `ALTER TABLE` on large tables without batching (can lock)

---

## 6. Adding a New Agent Tool

**Steps:**

1. **Define tool** in `internal/agent/tools/` - implement the tool interface:
   ```go
   type Tool interface {
       Name() string
       Description() string
       Execute(ctx context.Context, args json.RawMessage) (ToolResult, error)
   }
   ```

2. **Register tool** in the tool registry (`internal/agent/tools/registry.go`).

3. **Add to tool list** in `internal/agent/prompts.go` so the LLM knows about it.

4. **Handle risky operations** - if the tool has side effects (writes, deletes, network calls), route through the approval gate:
   ```go
   if requiresApproval {
       if err := h.approvalGate.Request(ctx, approval.Request{...}); err != nil {
           return ToolResult{}, err
       }
   }
   ```

5. **Write tests** - test argument parsing, execution, error handling.

6. **Update sandbox config** if the tool runs in a Docker sandbox (`internal/sandbox/`).

---

## 7. Adding a New Frontend Page

**Steps:**

1. **Create view component** in `frontend/src/views/`:
   ```vue
   <script setup lang="ts">
   import { ref, onMounted } from 'vue'
   import { useRoute, useRouter } from 'vue-router'

   const route = useRoute()
   const router = useRouter()
   // ...
   </script>
   ```

2. **Register route** in `frontend/src/router/index.ts`:
   ```typescript
   {
     path: '/foo/:id?',
     name: 'Foo',
     component: () => import('@/views/foo/FooView.vue'),
     meta: { title: 'Foo', requiresAuth: true }
   }
   ```

3. **Add API wrapper** in `frontend/src/api/`:
   ```typescript
   export const fooApi = {
     list: () => request.get('/foos'),
     get: (id: string) => request.get(`/foos/${id}`),
     create: (data: FooCreateReq) => request.post('/foos', data),
   }
   ```

4. **Add Pinia store** if state sharing is needed (in `frontend/src/stores/`).

5. **Add i18n entries** in `frontend/src/i18n/` (both `zh-CN` and `en-US`).

6. **Type-check**: `cd frontend && npm run type-check`

**Conventions:**
- Use `<script setup lang="ts">` (Composition API)
- Use TDesign components (`t-button`, `t-table`, etc.)
- Follow Apple-inspired blue theme (#007AFF)
- Support dark/light mode
- Add loading states and error handling

---

## 8. Debugging the ReAct Agent Loop

The ReAct engine lives in `internal/agent/` and follows: `think -> act -> observe -> think -> ... -> finalize`.

**Debug steps:**

1. **Enable debug logging:**
   ```bash
   LOG_LEVEL=debug make dev-app
   ```

2. **Check thinking trace** - the `thinkingTracker` (`internal/agent/thinking/`) records each think/act/observe step. Logs are prefixed with `[agent]`.

3. **Common issues:**
   - **Agent loops infinitely**: check `maxIterations` in engine config; ensure the finalize tool is available
   - **Tool execution fails silently**: check tool error handling - errors should be returned, not swallowed
   - **Token budget exhausted**: check `internal/agent/token/` usage tracking
   - **Goroutine leak**: ensure all goroutines have `defer recover()` and respect context cancellation

4. **Inspect tool results** - `observe.go` ingests tool results into the conversation. Check the `ToolResult` struct for malformed output.

5. **Approval gate blocking** - if a tool requires approval, check `internal/agent/approval/gate.go` - the gate has a timeout and can block the loop.

6. **Sandbox mode** - if `XINWIKI_SANDBOX_MODE=docker`, skills run in containers. Check `internal/sandbox/` for container lifecycle issues.

---

## 9. Debugging Wiki Compilation

The Wiki compiler lives in `internal/wiki/compiler.go` and does incremental knowledge-base compilation.

**Debug steps:**

1. **Enable Wiki debug logging:**
   ```bash
   LOG_LEVEL=debug ENABLE_WIKI=true make dev-app
   ```

2. **Check compilation status:**
   - `compiler.go` processes knowledge files into Wiki pages
   - Each page gets a `confidence_score`, `quality_score`, and `freshness` status
   - Check `internal/wikiquality/` for quality scoring logic

3. **Common issues:**
   - **Pages not appearing**: check knowledge file parsing status, chunk generation, and embedding completion
   - **Stale pages**: the `WikiScoreRefreshRunner` runs daily - check if it's scheduled
   - **Retrieval quality poor**: check hybrid retrieval weights (BM25 + vector + graph), reranker config
   - **Graph RAG not working**: ensure `ENABLE_GRAPH_RAG=true` and Neo4j is reachable

4. **Lifecycle manager** - `internal/application/service/wiki_lifecycle_manager.go` has three components:
   - **Crystallizer**: keeps high-quality pages fresh
   - **Superseder**: detects and merges duplicate pages
   - **Forgetter**: archives stale/deprecated pages

5. **Query rewriter** - `internal/wiki/query_rewriter.go` transforms user queries before retrieval. Check if rewrite rules are causing unexpected behavior.

---

## 10. Release Process

### Standard Edition (Docker)

1. **Update version:**
   ```bash
   echo "0.7.0" > VERSION
   ```

2. **Update CHANGELOG.md** - add new version section with changes.

3. **Build and test:**
   ```bash
   make build-prod
   make test
   cd frontend && npm run build
   ```

4. **Build Docker images:**
   ```bash
   make docker-build-all
   ```

5. **Tag and push:**
   ```bash
   git add -A && git commit -m "release: v0.7.0"
   git tag v0.7.0
   git push origin master --tags
   ```

6. **GitHub Release** - the `docker-image.yml` workflow triggers on tag push and builds multi-arch images.

### Lite Edition

1. **Trigger the `release-lite.yml` workflow** via GitHub Actions UI (workflow_dispatch) with the tag name.

2. **Update Homebrew Formula** - the workflow auto-updates `Formula/xinwiki-lite.rb` with new SHA256 hashes.

### CLI

1. **Update `cli/CHANGELOG.md`** with changes.

2. **Run CLI tests:**
   ```bash
   cd cli/
   go test -count=1 ./...
   go vet ./...
   go test -race -count=1 ./...
   ```

3. **Update skill parity** if commands/flags changed:
   ```bash
   go test ./internal/skillparity/...
   ```

---

## Quick Reference: Common Commands

| Task | Command |
|------|---------|
| Build backend | `make build` |
| Run backend (hot-reload) | `make dev-app` |
| Run frontend (HMR) | `make dev-frontend` |
| Run all tests | `make test` |
| Format code | `make fmt` |
| Lint | `make lint` |
| Race detection | `go test -race -count=1 ./...` |
| Build Lite | `make build-lite` |
| Start infra | `make dev-start` |
| Stop infra | `make dev-stop` |
| Apply migrations | `make migrate-up` |
| Generate Swagger | `make docs` |
| Build Docker images | `make docker-build-all` |
| Frontend type-check | `cd frontend && npm run type-check` |
| Frontend build | `cd frontend && npm run build` |
| CLI build | `cd cli && go build -o xinwiki .` |
| CLI tests | `cd cli && go test -count=1 ./...` |
