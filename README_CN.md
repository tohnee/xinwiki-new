<p align="center">
  <picture>
    <img src="./docs/images/logo.png" alt="XinWiki Logo" height="120"/>
  </picture>
</p>

<p align="center">
    <a href="https://github.com/tohnee/xinwiki-new" target="_blank">
        <img alt="GitHub Repository" src="https://img.shields.io/badge/GitHub-tohnee/xinwiki--new-181717?logo=github">
    </a>
    <a href="./LICENSE">
        <img src="https://img.shields.io/badge/License-MIT-ffffff?labelColor=d4eaf7&color=2e6cc4" alt="License">
    </a>
    <a href="./CHANGELOG.md">
        <img alt="版本" src="https://img.shields.io/badge/version-1.0.0-2e6cc4?labelColor=d4eaf7">
    </a>
</p>

<p align="center">
| <a href="./README.md"><b>English</b></a> | <b>简体中文</b></a> |
</p>

<p align="center">
  <h4 align="center">

  [项目介绍](#-项目介绍) • [架构设计](#-架构设计) • [核心特性](#-核心特性) • [快速开始](#-快速开始) • [部署文档](#-部署文档) • [开发指南](#-开发指南)

  </h4>
</p>

# 💡 XinWiki — 智能体驱动的知识工作平台

## 📌 项目介绍

**XinWiki** 是一款开源的、基于大语言模型（LLM）的企业级知识管理与智能问答平台，采用Apple明亮科技风设计，参考NotebookLM交互体验，专为团队知识沉淀与高效协作打造。

平台围绕三大核心能力构建：**混合检索引擎**（BM25+向量检索+知识图谱）实现高精度问答，**ReAct Agent 智能推理**自主编排知识检索、MCP工具与网络搜索完成复杂多步任务，全新的 **Wiki 模式** 则让智能体从原始文档中自动生成相互链接的结构化知识库与可视化知识图谱。结合三栏式Workspace布局、思维链可视化、二十余家主流模型厂商集成、企业级多租户RBAC权限体系（UUM认证集成），以及完全可私有化部署的模块化架构，XinWiki帮助团队把分散文档沉淀为可查询、可推理、可持续演进的专属知识资产。

框架支持从飞书、Notion及语雀等外部平台自动同步知识，覆盖PDF、Word、图片、Excel等十余种文档格式，模型层面兼容OpenAI、Anthropic Claude、DeepSeek、通义千问、智谱、豆包、Gemini、Ollama等主流厂商。全流程模块化设计，大模型、向量数据库、存储等组件均可灵活替换，支持本地与私有云部署，数据完全自主可控。

## ✨ 核心特性

### 🤖 智能对话与推理
- **思维链可视化**：完整展示Agent思考过程、工具调用与Token消耗统计
- **混合检索引擎**：BM25稀疏召回+Dense稠密召回+GraphRAG图谱增强，支持HNSW向量加速
- **ReAct智能体**：渐进式多步推理，支持自定义Agent技能与沙盒执行
- **Wiki模式**：Agent驱动自动生成结构化、相互链接的Markdown Wiki知识页面
- **模型路由**：按成本/延迟动态选择最优模型，支持Prompt版本化管理
- **矛盾检测**：自动识别文档中的矛盾信息并标注
- **可观测性**：全链路追踪，Token用量、成本、延迟实时统计

### 📚 知识管理
- **多类型知识库**：支持FAQ/文档/Wiki类型，文件夹导入、URL导入、标签管理
- **高性能编译**：增量编译引擎，知识库更新毫秒级生效
- **多数据源同步**：飞书/Notion/语雀自动同步，支持增量与全量同步
- **多格式支持**：PDF/Word/Txt/Markdown/HTML/图片/CSV/Excel/PPT/JSON
- **语义缓存**：相似度阈值0.95，按租户严格隔离，Redis不可用时自动降级为内存存储
- **嵌入批处理**：双触发机制（时间窗口+批量大小），按模型独立队列，自动去重

### 🔐 企业级安全与权限
- **多租户RBAC**：四级角色矩阵（Owner/Admin/Contributor/Viewer），资源级权限控制
- **UUM认证集成**：支持企业统一用户管理，继承组织架构与部门权限
- **数据安全**：AES-256-GCM静态加密，gRPC TLS通信，防SSRF，Agent沙箱隔离
- **审计日志**：全租户操作审计追踪

### 🎨 现代化界面
- **三栏式Workspace**：参考NotebookLM设计，左侧导航、中间对话、右侧生成面板
- **Apple明亮科技风**：蓝色主题（#007AFF），毛玻璃效果，流畅动画
- **响应式设计**：完美适配桌面与移动端
- **暗色/亮色模式**：自动跟随系统主题

## 📱 界面展示

<table>
  <tr>
    <td colspan="2" align="center"><b>💬 三栏式Workspace工作区</b><br/><img src="./docs/images/workspace.png" alt="三栏式Workspace" width="100%"></td>
  </tr>
  <tr>
    <td width="50%" align="center"><b>🧠 思维链可视化</b><br/><img src="./docs/images/thinking-chain.png" alt="思维链可视化" width="100%"></td>
    <td width="50%" align="center"><b>🕸️ Wiki知识图谱</b><br/><img src="./docs/images/wiki-graph.png" alt="Wiki知识图谱" width="100%"></td>
  </tr>
  <tr>
    <td width="50%" align="center"><b>📊 成本与用量仪表盘</b><br/><img src="./docs/images/cost-dashboard.png" alt="成本仪表盘" width="100%"></td>
    <td width="50%" align="center"><b>⚙️ 系统设置</b><br/><img src="./docs/images/settings.png" alt="系统设置" width="100%"></td>
  </tr>
</table>

## 🏗️ 架构设计

```
┌─────────────────────────────────────────────────────────────────┐
│                        Frontend (Vue 3 + TDesign)               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │  左侧导航   │  │  对话区域    │  │  右侧生成面板(Wiki/PPT等)│  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└──────────────────────────────────┬──────────────────────────────┘
                                   │ HTTP/WebSocket
┌──────────────────────────────────▼──────────────────────────────┐
│                         Backend (Go + Gin)                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │  Router  │  │  Handler │  │ Service  │  │  Agent Engine    │ │
│  └──────────┘  └──────────┘  └──────────┘  │  ┌────────────┐  │ │
│                                              │  │ Think/Act  │  │ │
│  ┌──────────────────────────────────────┐    │  └────────────┘  │ │
│  │  RBAC + UUM Auth + Multi-Tenant      │    └──────────────────┘ │
│  └──────────────────────────────────────┘                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │ Semantic │  │ Embedding│  │  Model   │  │ Cost Tracking    │ │
│  │  Cache   │  │ Batcher  │  │ Router   │  │ + Aggregation    │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘ │
└─────────────┬──────────────────┬──────────────────┬──────────────┘
              │                  │                  │
┌─────────────▼──────┐  ┌────────▼────────┐  ┌──────▼──────────────┐
│  Vector Stores     │  │  LLM Providers  │  │  Document Parser    │
│  (pgvector/ES/     │  │  (OpenAI/Claude/│  │  (PaddleOCR-VL/     │
│   Milvus/Qdrant)   │  │   DeepSeek/...) │  │   OpenDataLoader)   │
└────────────────────┘  └─────────────────┘  └─────────────────────┘
              │                  │                  │
┌─────────────▼──────────────────▼──────────────────▼──────────────┐
│                    Infrastructure Services                       │
│  ┌────────┐  ┌─────────┐  ┌───────┐  ┌────────┐  ┌────────────┐ │
│  │  Redis │  │PostgreSQL│ │ MinIO │  │ Neo4j  │  │  Langfuse  │ │
│  └────────┘  └─────────┘  └───────┘  └────────┘  └────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## 🧩 技术栈

| 层级 | 技术选型 |
|------|----------|
| **前端** | Vue 3 + TypeScript + Vite + TDesign + Pinia |
| **后端** | Go 1.24+ + Gin + GORM + Wire (依赖注入) |
| **文档解析** | Python + PaddleOCR-VL + OpenDataLoader |
| **向量数据库** | PostgreSQL (pgvector) / Elasticsearch / OpenSearch / Milvus / Qdrant |
| **关系型数据库** | PostgreSQL 15+ |
| **缓存** | Redis 7+ (语义缓存、会话存储) |
| **对象存储** | MinIO / S3 / 阿里云OSS / 腾讯云COS / 火山引擎TOS |
| **知识图谱** | Neo4j (可选) |
| **可观测性** | Langfuse + Prometheus |
| **部署** | Docker Compose / Kubernetes (Helm) |

## 🚀 快速开始

### 🛠 环境要求

- Docker 24.0+ & Docker Compose v2+
- Git
- 至少 4CPU 8GB 内存（推荐 8CPU 16GB 以获得最佳体验）

### 📦 一键启动（Docker Compose）

```bash
# 1. 克隆仓库
git clone https://github.com/tohnee/xinwiki-new.git
cd xinwiki-new

# 2. 复制环境变量配置文件
cp .env.example .env

# 3. 编辑 .env 文件，配置必要的参数（至少配置一个LLM API Key）
# vim .env

# 4. 启动核心服务
docker compose up -d
```

启动成功后访问 **http://localhost** 即可使用，默认管理员账号：`admin@xinwiki.com` / `admin123`。

> 首次启动会自动初始化数据库并创建默认租户和管理员账号，请及时修改默认密码。

### 🔧 可选服务（Docker Compose Profiles）

按需添加 `--profile` 启动额外组件，多个 profile 可叠加使用：

| Profile | 说明 | 启动命令 |
|---------|------|----------|
| _(默认)_ | 核心服务（Web+App+PostgreSQL+Redis） | `docker compose up -d` |
| `full` | 全部功能（包含所有可选组件） | `docker compose --profile full up -d` |
| `neo4j` | 知识图谱 (Neo4j) | `docker compose --profile neo4j up -d` |
| `minio` | 对象存储 (MinIO) | `docker compose --profile minio up -d` |
| `langfuse` | 链路追踪 (Langfuse) | `docker compose --profile langfuse up -d` |

组合示例：`docker compose --profile neo4j --profile minio up -d`

停止服务：`docker compose down`

停止服务并删除数据：`docker compose down -v` ⚠️ 会删除所有数据

### 🌐 服务地址

| 服务 | 地址 | 默认账号/说明 |
|------|------|--------------|
| XinWiki Web UI | http://localhost | admin@xinwiki.com / admin123 |
| 后端API | http://localhost:8080 | API文档: /swagger/index.html |
| MinIO控制台 | http://localhost:9001 | minioadmin / minioadmin |
| Langfuse追踪 | http://localhost:3000 | 查看.env配置 |
| Neo4j Browser | http://localhost:7474 | neo4j / neo4jpassword |
| pgAdmin | http://localhost:5050 | pgadmin@xinwiki.com / pgadmin123 |

## 📖 详细部署文档

### 本地开发环境部署

请参考 [开发环境搭建指南](./docs/开发指南.md) 进行本地开发环境配置。

### 生产环境部署

详细的生产环境部署指南请参考 [XinWiki生产部署文档](./docs/DEPLOYMENT.md)，包含：
- 服务器配置建议
- 环境变量详细说明
- HTTPS配置
- 数据备份与恢复
- 性能调优
- 监控与告警配置

## 🧭 开发指南

### ⚡ 快速开发模式

如果你需要频繁修改代码，使用快速开发模式，无需每次重新构建Docker镜像：

```bash
# 1. 启动基础设施服务（PostgreSQL、Redis等）
make dev-start

# 2. 启动后端服务（新终端窗口）
make dev-app

# 3. 启动前端开发服务器（新终端窗口）
make dev-frontend
```

开发服务器地址：
- 前端：http://localhost:5174
- 后端API：http://localhost:8080

**开发优势：**
- ✅ 前端修改自动热重载（Vite HMR，无需刷新）
- ✅ 后端修改支持Air热重载（秒级重启）
- ✅ 无需重新构建Docker镜像
- ✅ 支持IDE断点调试

### 📋 Make命令说明

| 命令 | 说明 |
|------|------|
| `make build` | 编译后端二进制文件 |
| `make build-frontend` | 编译前端生产包 |
| `make dev-start` | 启动开发依赖基础设施 |
| `make dev-app` | 启动后端开发服务（Air热重载） |
| `make dev-frontend` | 启动前端开发服务器 |
| `make docker-build` | 构建Docker镜像 |
| `make test` | 运行所有单元测试 |
| `make lint` | 运行代码检查 |
| `make clean` | 清理编译产物 |

## 📁 项目结构

```
xinwiki-new/
├── cmd/                    # 入口程序
│   └── server/            # 后端服务入口
├── internal/              # 内部业务代码
│   ├── agent/            # Agent引擎（思考/执行/工具调用）
│   ├── acl/              # ACL权限控制纯函数
│   ├── application/      # 应用服务层
│   ├── auth/             # 认证与RBAC（含UUM集成）
│   ├── config/           # 配置管理
│   ├── container/        # 依赖注入容器
│   ├── handler/          # HTTP处理器
│   ├── infrastructure/   # 基础设施（文档解析、向量存储等）
│   ├── models/           # LLM模型对接（Chat/Embedding/Rerank/VLM）
│   ├── router/           # 路由配置
│   ├── types/            # 核心类型定义
│   └── wiki/             # Wiki引擎（检索/编译/QA）
├── frontend/             # 前端Vue项目
│   ├── src/
│   │   ├── components/  # 组件
│   │   ├── views/       # 页面
│   │   ├── stores/      # Pinia状态管理
│   │   └── api/         # API调用封装
├── docreader/           # Python文档解析服务
├── mcp-server/          # MCP服务器
├── deploy/              # 部署配置文件
├── docs/                # 文档
├── docker-compose.yml   # Docker Compose编排
├── Makefile            # 构建脚本
└── .env.example        # 环境变量示例
```

## 🤝 贡献指南

欢迎通过Issue反馈问题或提交Pull Request。

**流程：** Fork → 新建分支 → 提交更改 → 创建PR

**规范：**
- 后端代码使用 `gofmt` 格式化
- 前端代码使用 ESLint + Prettier 格式化
- 遵循 [Conventional Commits](https://www.conventionalcommits.org/) 提交规范（`feat:` / `fix:` / `docs:` / `test:` / `refactor:`）

## 🔒 安全声明

在生产环境部署时，我们强烈建议：

- 将XinWiki服务部署在内网/私有网络环境中，避免直接暴露在公网
- 配置防火墙规则和访问控制，仅允许信任的IP访问
- 及时修改默认管理员密码
- 配置HTTPS加密传输
- 定期备份数据库
- 定期更新到最新版本以获取安全补丁

## 📄 许可证

本项目基于 [MIT](./LICENSE) 协议发布。
你可以自由使用、修改和分发本项目代码，但需保留原始版权声明。
