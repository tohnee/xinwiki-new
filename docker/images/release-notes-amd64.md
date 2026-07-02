## XinWiki 离线部署包 (linux/amd64)

适用于企业内部无互联网环境的 Linux amd64 服务器部署 XinWiki。

### 快速部署（三步）

```bash
# 1. 下载部署包并解压
wget https://github.com/tohnee/xinwiki-new/releases/download/images-v0.1.0/xinwiki-offline-deploy-amd64.tar.gz
tar xzf xinwiki-offline-deploy-amd64.tar.gz
cd xinwiki-offline-deploy

# 2. 下载并加载 Docker 镜像
bash scripts/download-images.sh
bash scripts/load-images.sh

# 3. 配置并启动
./deploy.sh
```

### 文件列表

| 文件 | 说明 | 大小 |
|------|------|------|
| **xinwiki-offline-deploy-amd64.tar.gz** | 部署包（代码+配置+脚本） | 200KB |
| **xinwiki-app.tar** | 后端 API 服务镜像 | 560MB |
| **xinwiki-ui.tar** | 前端 Web UI 镜像 | 35MB |
| **xinwiki-docreader.tar** | 文档解析服务镜像 | 1.7GB |
| **xinwiki-sandbox.tar** | Agent 沙箱镜像 | 81MB |
| **paradedb.tar** | PostgreSQL 17 + pgvector | 496MB |
| **redis.tar** | Redis 缓存 | 13MB |
| **nginx.tar** | Nginx | 25MB |
| **golang.tar** | Go 编译环境 | 283MB |
| **python310.tar** | Python 3.10 | 359MB |
| **python311.tar** | Python 3.11 | 43MB |
| **node.tar** | Node.js 20 | 68MB |
| **debian.tar** | Debian 12 | 27MB |
| **uv.tar** | uv (Python 包管理) | 16MB |
| **clickhouse.tar** | ClickHouse (Langfuse) | 171MB |
| **langfuse.tar** | Langfuse 可观测性 | 280MB |
| **langfuse-worker.tar** | Langfuse Worker | 313MB |
| **minio.tar** | MinIO 对象存储 | 59MB |
| **neo4j.tar** | Neo4j 知识图谱 | 383MB |
| **qdrant.tar** | Qdrant 向量数据库 | 68MB |
| **searxng.tar** | SearXNG 搜索 | 93MB |
| **xinwiki-infra-amd64.tar** | 基础设施镜像合集 | 509MB |
| **xinwiki-build-base-amd64.tar** | 构建基础镜像合集 | 820MB |
| **xinwiki-optional-amd64.tar** | 可选组件镜像合集 | 1.3GB |
| **SHA256SUMS** | 校验文件 | - |

### 部署方式

**方式一：一键部署（推荐）**
```bash
./deploy.sh
```

**方式二：手动部署**
```bash
# 1. 下载镜像
bash scripts/download-images.sh

# 2. 加载镜像
bash scripts/load-images.sh

# 3. 配置环境
cp .env.example .env
vim .env
# 必须配置: DB_PASSWORD, JWT_SECRET, REDIS_PASSWORD, 至少一个 LLM API Key

# 4. 启动
docker compose up -d                    # 最小化部署
docker compose --profile full up -d     # 完整部署
```

### 访问

- Web UI: http://localhost
- API: http://localhost:8080
- Swagger: http://localhost:8080/swagger/index.html
- 默认账号: admin@xinwiki.com / admin123

### 在服务器上构建镜像（可选）

如果需要在服务器上从源码构建：
```bash
bash scripts/build-offline-package.sh
```

### 架构说明

- 目标平台: **linux/amd64**
- 不适用于 ARM64 / Mac Apple Silicon
