## XinWiki 离线镜像包 (linux/amd64)

适用于 Linux amd64 架构的 XinWiki 离线部署镜像。

### 包含内容

**基础设施镜像（运行时必需）**
- `xinwiki-infra-amd64.tar` (509MB) - paradedb, redis
- `xinwiki-build-base-amd64.tar` (820MB) - golang, python, node, debian, nginx, uv

**可选组件镜像**
- `xinwiki-optional-amd64.tar` (1.3GB) - minio, neo4j, qdrant, searxng, clickhouse, langfuse, langfuse-worker

**单镜像** (见 assets 列表): clickhouse, debian, golang, langfuse, langfuse-worker, minio, neo4j, nginx, node, paradedb, python310, python311, qdrant, redis, searxng, uv

**总大小: 约 5.2 GB**

### 使用方法

```bash
# 1. 下载需要的镜像包
wget https://github.com/tohnee/xinwiki-new/releases/download/images-v0.1.0/xinwiki-infra-amd64.tar
wget https://github.com/tohnee/xinwiki-new/releases/download/images-v0.1.0/xinwiki-build-base-amd64.tar
wget https://github.com/tohnee/xinwiki-new/releases/download/images-v0.1.0/SHA256SUMS

# 2. 校验
sha256sum -c SHA256SUMS

# 3. 加载镜像
docker load -i xinwiki-infra-amd64.tar
docker load -i xinwiki-build-base-amd64.tar

# 4. 返回项目根目录启动
cp .env.example .env
vim .env
docker compose up -d
```

### 架构

- 目标平台: **linux/amd64**
- 不适用于 ARM64 / Mac Apple Silicon
