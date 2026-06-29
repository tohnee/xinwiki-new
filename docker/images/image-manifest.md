# XinWiki 离线部署镜像清单

生成时间: 2026-06-30
镜像包大小: 731MB (压缩后)

## 包含镜像

| 镜像 | 说明 | 必需 |
|------|------|------|
| xinwiki/xinwiki-ui:latest | 前端 Web UI (Vue 3 + Nginx) | ✅ |
| xinwiki/xinwiki-app:latest | 后端 API 服务 (Go + Gin) | ✅ |
| paradedb/paradedb:v0.22.2-pg17 | PostgreSQL 17 + pgvector + ParadeDB | ✅ |
| redis:7.0-alpine | Redis 缓存服务 | ✅ |
| nginx:stable-alpine | Nginx 基础镜像（前端依赖） | ✅ |

## 未包含的可选镜像

以下镜像为可选功能，离线部署时按需自行拉取：

| 镜像 | 用途 | Profile |
|------|------|---------|
| minio/minio:RELEASE.2025-09-07T16-13-09Z | 对象存储 | --profile minio |
| neo4j:2025.10.1 | 知识图谱 | --profile neo4j |
| qdrant/qdrant:v1.16.2 | 向量数据库 | - |
| searxng/searxng:latest | 网络搜索 | --profile searxng |
| clickhouse/clickhouse-server:24.8 | Langfuse 分析 | --profile langfuse |
| langfuse/langfuse:3 | LLM 可观测性 | --profile langfuse |
| langfuse/langfuse-worker:3 | Langfuse Worker | --profile langfuse |

## 使用方法

```bash
# 1. 加载镜像
cd docker/images
chmod +x load-images.sh
./load-images.sh

# 2. 回到项目根目录，配置环境
cd ../..
cp .env.example .env
vim .env  # 配置数据库密码、JWT密钥、LLM API Key 等

# 3. 启动服务
docker compose up -d

# 4. 访问
# Web UI: http://localhost
# 默认账号: admin@xinwiki.com / admin123
```
