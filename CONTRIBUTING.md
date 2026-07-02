# Contributing to XinWiki

Thank you for your interest in contributing to XinWiki! This document covers the essentials for getting your changes merged.

## Prerequisites

- **Go** 1.26+
- **Node.js** 18+ and **npm**
- **Python** 3.10+ with **uv** (for docreader service)
- **Docker** and **Docker Compose** (for integration testing)
- **PostgreSQL** 14+ (or use `make dev-start` for containerized infra)

## Getting Started

### 1. Fork and clone

```bash
git clone https://github.com/<your-username>/XinWiki.git
cd XinWiki
git remote add upstream https://github.com/Tencent/XinWiki.git
```

### 2. Start development infrastructure

```bash
make dev-start          # Start PostgreSQL, Redis, MinIO, Neo4j, DocReader containers
make dev-app            # Run backend with hot-reload (Air)
make dev-frontend       # Run Vite dev server with HMR (separate terminal)
```

Backend API: `http://localhost:8080` (Swagger: `/swagger/index.html`)
Frontend dev: `http://localhost:5173` (proxies API to :8080)
Default admin: `admin@xinwiki.com` / `admin123`

### 3. Create a branch

```bash
git checkout -b feat/your-feature-name
```

## Development Workflow

### Repository structure

XinWiki has **three independent Go modules** plus a Vue frontend and two Python services:

| Module | Path | go.mod | Description |
|--------|------|--------|-------------|
| Root | `.` | `go.mod` | Backend server (`cmd/server/`) |
| CLI | `cli/` | `cli/go.mod` | AI-agent-first CLI (`xinwiki`) |
| SDK | `client/` | `client/go.mod` | Generated Go SDK |
| Frontend | `frontend/` | `frontend/package.json` | Vue 3 + TypeScript + Vite |
| DocReader | `docreader/` | `docreader/pyproject.toml` | Python gRPC document parsing |
| MCP Server | `mcp-server/` | `mcp-server/pyproject.toml` | Standalone Python MCP server |

Read [CLAUDE.md](./CLAUDE.md) for detailed architecture, DI patterns, and code conventions.

### Building and testing

#### Backend (root module)

```bash
make build              # go build -o XinWiki ./cmd/server
make run                # build + run
make test               # go test -v ./...
make fmt                # go fmt ./...
make lint               # golangci-lint run (lll 120, govet, revive, gofmt, gofumpt)

# Single test
go test -run TestName ./internal/application/service/

# Race detection
go test -race -count=1 ./...
```

#### CLI module (run from `cli/`)

```bash
cd cli/
go build -o xinwiki .
go test -count=1 ./...
go vet ./...
```

Read [cli/AGENTS.md](./cli/AGENTS.md) before touching the CLI - it defines the wire contract that AI agents depend on.

#### Frontend

```bash
cd frontend/
npm install
npm run dev             # Vite dev server (HMR)
npm run build           # Production build -> dist/ -> ../web/
npm run type-check      # vue-tsc --build
npm run test            # node --test
```

#### Lite edition (single binary, zero external deps)

```bash
make build-lite         # Builds frontend -> web/, then Go with -tags sqlite_fts5
make run-lite           # build-lite + run against .env.lite
```

### Database migrations

Migrations live in `migrations/versioned/` (numbered `NNNNNN_name.up.sql` / `.down.sql`).

```bash
make migrate-up                          # Apply
make migrate-down                        # Roll back one
make migrate-create name=add_foo         # Create new versioned pair
```

Requires: `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

### Swagger docs

```bash
make install-swagger    # One-time: install swag CLI
make docs               # swag init -> ./docs (Swagger at /swagger/index.html)
```

## Code Style

### Go

- `gofmt` / `gofumpt` enforced via `make lint`
- **No em-dashes in Go source** - use ASCII `-` or rewrite
- Godoc on every exported symbol - explain *why*, not *what*
- Don't restate code in comments; don't reference task numbers / commit SHAs in inline comments
- Don't add a helper for a single caller - inline it
- **Commits**: follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:` / `fix:` / `docs:` / `test:` / `refactor:`)

### Architecture rules

- **DI uses `go.uber.org/dig`** (not Wire). Register providers in `internal/container/container.go`.
- **DI contract**: interfaces in `internal/types/interfaces/` are the single source of truth. Services depend on those interfaces, never on concrete structs.
- **Layering**: `router -> middleware -> handler -> application/service -> application/repository -> DB`
- **Adding a service**: define interface in `internal/types/interfaces/`, implement under `internal/application/service/` or `internal/application/repository/`, register in `container.go`.
- **Three MCP surfaces - don't conflate them**: `cli/internal/mcp/` (CLI stdio server), `mcp-server/` (standalone Python), `internal/mcp/` (Go backend integration).

### Frontend

- Vue 3 Composition API with `<script setup lang="ts">`
- Pinia for state management
- TDesign component library
- Apple-inspired blue theme (#007AFF), glassmorphism, dark/light mode

### Testing patterns

- **testify** - `require` for error checks (halts on failure), `assert` for value comparisons
- **Narrow interface stubs, no mocking library** - embed the interface from `internal/types/interfaces/` and override only methods under test
- **go-sqlmock** for DB-layer tests when a real DB isn't needed
- Table-driven tests for flag/validation/parser edge cases

## Pull Request Process

### Before submitting

1. **Format and lint**: `make fmt && make lint`
2. **Test**: `make test` (and `go test -race -count=1 ./...` for critical changes)
3. **Frontend**: `cd frontend && npm run type-check && npm run build`
4. **Self-review** your diff
5. **Update documentation** if your change affects public API, configuration, or behavior

### Commit messages

Follow Conventional Commits:

```
feat: add cross-encoder reranker to wiki retrieval
fix: resolve nil pointer in QA engine when TokenUsage is zero-value
docs: update OIDC SSO configuration guide
test: add table-driven tests for ACL permission propagation
refactor: extract wiki page compilation into incremental compiler
```

For breaking changes, add `!` after the type: `feat!: rename RBAC env vars (BREAKING)`

### PR checklist

- [ ] `make fmt && make lint && make test` pass locally
- [ ] Self-reviewed the code
- [ ] Added/updated tests covering the change
- [ ] Updated related documentation (README, `docs/`, Swagger annotations)
- [ ] Breaking changes are clearly called out in the PR description

### Review criteria

- **Correctness**: does the change do what it claims?
- **Tests**: are edge cases covered? Do tests pass?
- **Architecture**: does it respect the DI boundary and layering rules?
- **Security**: no secrets in code, input validation at boundaries, no SQL injection vectors
- **Performance**: no N+1 queries, no unbounded goroutines, context cancellation respected

## Security

If you discover a security vulnerability, please follow the [Security Policy](./SECURITY.md) - do **not** open a public issue.

Key security rules:
- Never commit `.env` files, API keys, or credentials
- Use `TENANT_AES_KEY` for encrypting cloud storage credentials (16/24/32 bytes)
- Production mode (`GIN_MODE=release`) runs `runtime.ValidateStartupEnv()` - all security-critical env vars must be set
- CORS must use `AllowOriginFunc` for dynamic validation, never `"*"` with `AllowCredentials`

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](./LICENSE).
