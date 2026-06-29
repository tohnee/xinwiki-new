# XinWiki 部署指南

本文档详细说明如何在本地或服务器上部署和启动 XinWiki 智能体驱动的知识工作平台。

## 目录

- [系统要求](#系统要求)
- [快速开始（Docker Compose）](#快速开始docker-compose)
- [本地开发环境部署](#本地开发环境部署)
- [生产环境部署](#生产环境部署)
- [环境变量配置详解](#环境变量配置详解)
- [可选服务配置](#可选服务配置)
- [HTTPS 配置](#https-配置)
- [数据备份与恢复](#数据备份与恢复)
- [性能调优](#性能调优)
- [监控与告警](#监控与告警)
- [常见问题排查](#常见问题排查)

---

## 系统要求

### 最低配置
- **CPU**: 4 核心
- **内存**: 8 GB RAM
- **磁盘**: 50 GB 可用空间
- **操作系统**: Linux (推荐 Ubuntu 22.04+/CentOS 8+), macOS 12+, Windows WSL2
- **软件**: Docker 24.0+, Docker Compose v2+, Git

### 推荐配置
- **CPU**: 8 核心及以上
- **内存**: 16 GB RAM 及以上
- **磁盘**: 100 GB SSD 及以上
- **网络**: 稳定的互联网连接（用于拉取 Docker 镜像和调用 LLM API）

---

## 快速开始（Docker Compose）

这是最简单的部署方式，适合快速体验和测试。

### 步骤 1: 克隆代码仓库

```bash
git clone https://github.com/tohnee/xinwiki-new.git
cd xinwiki-new
```

### 步骤 2: 配置环境变量

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑环境变量文件
vim .env
```

**必须配置的参数**（至少配置一个 LLM 提供商）：

```ini
# ========== LLM 模型配置 ==========
# OpenAI 配置（示例）
LLM_PROVIDER=openai
LLM_BASE_URL=https://api.openai.com/v1
LLM_API_KEY=sk-your-openai-api-key
LLM_MODEL_NAME=gpt-4o

# Embedding 模型配置
EMBEDDING_PROVIDER=openai
EMBEDDING_BASE_URL=https://api.openai.com/v1
EMBEDDING_API_KEY=sk-your-openai-api-key
EMBEDDING_MODEL_NAME=text-embedding-3-small
```

> 💡 支持的 LLM 提供商包括：OpenAI、Anthropic Claude、DeepSeek、通义千问、智谱 AI、豆包、Gemini、Ollama（本地模型）等。详见 [内置模型管理](./wiki/核心功能/内置模型管理.md)。

### 步骤 3: 启动核心服务

```bash
# 启动核心服务（前端 + 后端 + PostgreSQL + Redis）
docker compose up -d
```

等待所有服务启动完成（约 2-5 分钟），然后访问：

- **XinWiki Web UI**: http://localhost
- **默认管理员账号**: `a****@***********` / `admin123`

> ⚠️ **重要**: 首次登录后请立即修改默认密码！

### 步骤 4: 验证安装

1. 访问 http://localhost 应该能看到登录页面
2. 使用默认账号登录
3. 进入「设置」→「模型管理」，测试 LLM 连接是否正常
4. 创建一个知识库并上传测试文档，验证问答功能

---

## 本地开发环境部署

如果你需要修改代码或进行二次开发，请使用开发模式部署。

### 前置依赖

- Go 1.24+
- Node.js 20+
- pnpm 9+
- Python 3.10+
- Make

### 方式一：使用 Make 一键启动（推荐）

```bash
# 1. 启动基础设施服务（PostgreSQL、Redis、MinIO 等）
make dev-start

# 2. 在新终端窗口启动后端服务（支持 Air 热重载）
make dev-app

# 3. 在新终端窗口启动前端开发服务器（支持 Vite HMR）
make dev-frontend
```

服务访问地址：
- **前端开发服务器**: http://localhost:5174
- **后端 API**: http://localhost:8080
- **API 文档**: http://localhost:8080/swagger/index.html

### 方式二：手动分步启动

#### 1. 启动基础设施

```bash
# 使用 Docker Compose 启动依赖服务
docker compose -f docker-compose.dev.yml up -d postgres redis minio
```

#### 2. 配置并启动后端

```bash
# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，配置数据库连接为 localhost
# DB_HOST=localhost
# REDIS_ADDR=localhost:6379

# 安装依赖
go mod download

# 启动后端（使用 Air 热重载）
go install github.com/cosmtrek/air@latest
air
```

#### 3. 启动前端

```bash
cd frontend

# 安装依赖
pnpm install

# 启动开发服务器
pnpm dev
```

### 开发模式优势

- ✅ 前端修改自动热更新（Vite HMR），无需刷新页面
- ✅ 后端修改支持 Air 秒级热重载
- ✅ 无需每次重新构建 Docker 镜像
- ✅ 支持 IDE 断点调试
- ✅ 完整的开发日志输出

---

## 生产环境部署

### 服务器准备

```bash
# Ubuntu/Debian 系统更新
sudo apt update && sudo apt upgrade -y

# 安装必要工具
sudo apt install -y git curl wget vim ufw

# 安装 Docker
curl -fsSL https://get.docker.com | bash
sudo usermod -aG docker $USER
newgrp docker

# 验证 Docker 安装
docker --version
docker compose version
```

### 防火墙配置

```bash
# 允许 SSH
sudo ufw allow 22/tcp

# 允许 HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# 启用防火墙
sudo ufw enable
```

### 部署步骤

#### 1. 克隆代码并配置

```bash
# 创建部署目录
sudo mkdir -p /opt/xinwiki
sudo chown $USER:$USER /opt/xinwiki
cd /opt/xinwiki

# 克隆代码
git clone https://github.com/tohnee/xinwiki-new.git .
# 或者使用指定版本标签
# git checkout v1.0.0

# 配置环境变量
cp .env.example .env
vim .env
```

#### 2. 生产环境关键配置

生产环境必须修改以下配置：

```ini
# ========== 安全配置 ==========
# 生产环境使用 release 模式
GIN_MODE=release

# 禁止新用户注册（根据需要开启）
DISABLE_REGISTRATION=true

# 强密码配置（使用 openssl 生成）
# JWT_SECRET=$(openssl rand -base64 32)
# SYSTEM_AES_KEY=$(openssl rand -hex 32)
JWT_SECRET=your-strong-jwt-secret-at-least-32-chars
SYSTEM_AES_KEY=your-32-byte-aes-key-for-encryption!!
TENANT_AES_KEY=your-tenant-encryption-key-32bytes

# 数据库密码（强密码）
DB_PASSWORD=your-strong-db-password-here

# Redis 密码
REDIS_PASSWORD=your-strong-redis-password-here

# ========== 应用配置 ==========
# 外部访问 URL（配置为你的域名）
APP_EXTERNAL_URL=https://xinwiki.yourdomain.com

# 文件存储（生产环境建议使用 MinIO 或云对象存储）
STORAGE_TYPE=minio
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY_ID=your-minio-access-key
MINIO_SECRET_ACCESS_KEY=your-minio-secret-key
MINIO_BUCKET_NAME=xinwiki-files

# ========== RBAC 配置 ==========
# 启用租户 RBAC
XINWIKI_TENANT_ENABLE_RBAC=true

# 配置启动时自动提升为系统管理员的邮箱
XINWIKI_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=y**@************
```

#### 3. 启动服务

```bash
# 拉取最新镜像
docker compose pull

# 启动核心服务
docker compose up -d

# 如果需要启用可选服务
# docker compose --profile minio --profile neo4j --profile langfuse up -d
```

#### 4. 配置 Nginx 反向代理（可选，推荐）

如果需要配置域名和 HTTPS，建议使用 Nginx 反向代理：

```nginx
server {
    listen 80;
    server_name xinwiki.yourdomain.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name xinwiki.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/xinwiki.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/xinwiki.yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    client_max_body_size 100M;

    location / {
        proxy_pass http://localhost:80;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
    }
}
```

使用 Certbot 申请 SSL 证书：

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d xinwiki.yourdomain.com
```

#### 5. 配置系统服务自启动

```bash
# 创建 systemd 服务
sudo tee /etc/systemd/system/xinwiki.service > /dev/null <<EOF
[Unit]
Description=XinWiki Docker Compose Service
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/xinwiki
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

# 启用自启动
sudo systemctl daemon-reload
sudo systemctl enable xinwiki

# 启动服务
sudo systemctl start xinwiki
```

---

## 环境变量配置详解

### 基础配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `GIN_MODE` | `release` | Gin 运行模式：`debug`（开发）/ `release`（生产） |
| `LOG_LEVEL` | `info` | 日志级别：`debug`/`info`/`warn`/`error`/`fatal` |
| `TZ` | `Asia/Shanghai` | 时区设置 |
| `XINWIKI_LANGUAGE` | `zh-CN` | 默认语言 |
| `DISABLE_REGISTRATION` | `false` | 是否禁止新用户注册 |

### 服务端口配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `FRONTEND_PORT` | `80` | 前端服务端口 |
| `APP_PORT` | `8080` | 后端服务端口 |
| `APP_HOST` | `app` | 后端服务主机名（Docker 内部） |
| `DOCREADER_PORT` | `50051` | 文档解析服务 gRPC 端口 |

### 数据库配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `DB_DRIVER` | `postgres` | 数据库类型（目前仅支持 postgres） |
| `DB_HOST` | `localhost` | 数据库主机地址 |
| `DB_PORT` | `5432` | 数据库端口 |
| `DB_USER` | `postgres` | 数据库用户名 |
| `DB_PASSWORD` | - | 数据库密码 |
| `DB_NAME` | `XinWiki` | 数据库名称 |

### Redis 配置

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `REDIS_PASSWORD` | - | Redis 密码 |
| `REDIS_DB` | `0` | Redis 数据库索引 |
| `REDIS_PREFIX` | `stream:` | Redis Key 前缀 |

### 向量数据库配置

XinWiki 支持多种向量数据库，默认使用 PostgreSQL 的 pgvector 扩展。

#### PostgreSQL + pgvector（默认）

```ini
RETRIEVE_DRIVER=postgres
```

#### Qdrant

```ini
RETRIEVE_DRIVER=qdrant
QDRANT_HOST=qdrant
QDRANT_PORT=6334
QDRANT_COLLECTION=xinwiki_embeddings
# QDRANT_API_KEY=your-api-key
```

#### Milvus

```ini
RETRIEVE_DRIVER=milvus
MILVUS_ADDRESS=milvus:19530
MILVUS_COLLECTION=xinwiki_embeddings
```

#### Elasticsearch/OpenSearch

```ini
RETRIEVE_DRIVER=elasticsearch_v8
ELASTICSEARCH_ADDR=http://elasticsearch:9200
# ELASTICSEARCH_USERNAME=elastic
# ELASTICSEARCH_PASSWORD=your-password
```

### 文件存储配置

#### 本地存储（默认，开发用）

```ini
STORAGE_TYPE=local
LOCAL_STORAGE_BASE_DIR=/data/files
```

#### MinIO（推荐生产使用）

```ini
STORAGE_TYPE=minio
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY_ID=your-access-key
MINIO_SECRET_ACCESS_KEY=your-secret-key
MINIO_BUCKET_NAME=xinwiki-files
```

#### 云对象存储

支持腾讯云 COS、火山引擎 TOS、阿里云 OSS、AWS S3、华为云 OBS 等，详见各云厂商配置说明。

### LLM 模型配置

详见 [内置模型管理文档](./wiki/核心功能/内置模型管理.md)。

---

## 可选服务配置

使用 Docker Compose Profile 启用可选服务：

### 启用 MinIO 对象存储

```bash
docker compose --profile minio up -d
```

访问地址：
- MinIO API: http://localhost:9000
- MinIO Console: http://localhost:9001
- 默认账号: `minioadmin` / `minioadmin`

### 启用知识图谱（Neo4j）

```bash
docker compose --profile neo4j up -d
```

```ini
# .env 中启用
ENABLE_GRAPH_RAG=true
NEO4J_ENABLE=true
NEO4J_URI=neo4j://neo4j:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=your-neo4j-password
```

访问地址：
- Neo4j Browser: http://localhost:7474

### 启用 Langfuse 可观测性

```bash
docker compose --profile langfuse up -d
```

首次启动需要等待 ClickHouse 迁移完成（约 1-2 分钟），然后：

1. 访问 http://localhost:3000 注册管理员账号
2. 在 Langfuse 中创建项目并生成 API Key
3. 在 `.env` 中配置：

```ini
LANGFUSE_PUBLIC_KEY=pk-lf-your-public-key
LANGFUSE_SECRET_KEY=sk-lf-your-secret-key
LANGFUSE_HOST=http://langfuse-web:3000
```

重启后端服务生效。

### 启用 SearXNG 网络搜索

```bash
docker compose --profile searxng up -d
```

在 XinWiki 后台「设置」→「网络搜索」中添加 SearXNG 搜索引擎，地址填 `http://searxng:8080`。

### 启用所有服务

```bash
docker compose --profile full up -d
```

---

## HTTPS 配置

### 使用 Let's Encrypt（推荐）

使用 Certbot 自动申请和续期 SSL 证书，参考 [生产环境部署](#生产环境部署) 中的 Nginx 配置部分。

### 使用自签名证书（内网测试）

```bash
# 生成自签名证书
openssl req -x509 -newkey rsa:4096 -keyout xinwiki.key -out xinwiki.crt -days 365 -nodes

# 配置 Nginx 指向证书路径
```

---

## 数据备份与恢复

### 数据备份

创建备份脚本 `/opt/xinwiki/backup.sh`：

```bash
#!/bin/bash
set -e

BACKUP_DIR=/opt/xinwiki/backups
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

# 备份 PostgreSQL 数据库
docker compose exec -T postgres pg_dump -U postgres XinWiki | gzip > $BACKUP_DIR/xinwiki_db_$DATE.sql.gz

# 备份上传文件
docker compose exec -T app tar czf - /data/files > $BACKUP_DIR/xinwiki_files_$DATE.tar.gz

# 备份环境变量配置
cp .env $BACKUP_DIR/.env_$DATE.bak

# 保留最近 30 天的备份
find $BACKUP_DIR -type f -mtime +30 -delete

echo "Backup completed: $BACKUP_DIR/xinwiki_db_$DATE.sql.gz"
```

添加执行权限并配置定时任务：

```bash
chmod +x backup.sh

# 添加到 crontab，每天凌晨 2 点执行备份
(crontab -l 2>/dev/null; echo "0 2 * * * /opt/xinwiki/backup.sh >> /opt/xinwiki/backups/backup.log 2>&1") | crontab -
```

### 数据恢复

```bash
# 1. 停止服务
docker compose down

# 2. 恢复数据库
gunzip -c backups/xinwiki_db_YYYYMMDD_HHMMSS.sql.gz | docker compose exec -T postgres psql -U postgres XinWiki

# 3. 恢复文件
cat backups/xinwiki_files_YYYYMMDD_HHMMSS.tar.gz | docker compose exec -T app tar xzf - -C /

# 4. 启动服务
docker compose up -d
```

---

## 性能调优

### 系统参数调优

```bash
# 编辑 /etc/sysctl.conf
sudo tee -a /etc/sysctl.conf <<EOF
# 网络优化
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30

# 文件描述符
fs.file-max = 1000000
EOF

# 应用配置
sudo sysctl -p

# 编辑 /etc/security/limits.conf
sudo tee -a /etc/security/limits.conf <<EOF
* soft nofile 65536
* hard nofile 65536
root soft nofile 65536
root hard nofile 65536
EOF
```

### PostgreSQL 调优

根据服务器内存调整 PostgreSQL 配置，通常设置为系统内存的 25%-40%：

```ini
# 在 docker-compose.yml 中为 postgres 服务添加命令参数
command:
  - "postgres"
  - "-c"
  - "max_connections=200"
  - "-c"
  - "shared_buffers=4GB"
  - "-c"
  - "effective_cache_size=12GB"
  - "-c"
  - "maintenance_work_mem=1GB"
  - "-c"
  - "work_mem=64MB"
```

### Redis 调优

```ini
# 在 docker-compose.yml 中为 redis 服务添加命令参数
command:
  - "redis-server"
  - "--appendonly"
  - "yes"
  - "--maxmemory"
  - "2gb"
  - "--maxmemory-policy"
  - "allkeys-lru"
```

### Embedding 批处理配置

```ini
# 嵌入批处理大小（根据 QPS 调整）
EMBEDDING_BATCH_MAX_SIZE=64
# 批处理最大等待时间（毫秒）
EMBEDDING_BATCH_MAX_WAIT_MS=10
# 最大排队请求数
EMBEDDING_BATCH_MAX_PENDING=512
```

---

## 监控与告警

### 日志查看

```bash
# 查看所有服务日志
docker compose logs -f

# 查看特定服务日志
docker compose logs -f app
docker compose logs -f frontend

# 查看最近 100 行日志
docker compose logs --tail=100 -f app
```

### 服务状态检查

```bash
# 查看服务运行状态
docker compose ps

# 查看资源使用情况
docker stats
```

### Prometheus 指标

XinWiki 后端暴露 Prometheus 指标端点：`http://localhost:8080/metrics`，包含：
- HTTP 请求计数和延迟
- 向量数据库请求统计
- LLM 调用 Token 用量
- 缓存命中率
- 系统运行状态

可以配合 Grafana 搭建可视化监控面板。

---

## 常见问题排查

### 服务启动失败

1. 检查端口是否被占用：
   ```bash
   sudo lsof -i :80 -i :8080 -i :5432 -i :6379
   ```

2. 检查 Docker 日志：
   ```bash
   docker compose logs app
   docker compose logs postgres
   ```

3. 确认 `.env` 配置正确，特别是数据库密码和连接信息。

### 无法连接 LLM API

1. 检查服务器网络是否能访问 LLM API 端点
2. 检查 API Key 是否正确
3. 检查是否需要配置代理：
   ```ini
   WEB_PROXY=http://your-proxy:port
   ```

### 文档上传失败

1. 检查文件大小限制，默认 50MB：
   ```ini
   MAX_FILE_SIZE_MB=100
   ```

2. 检查存储目录权限：
   ```bash
   docker compose exec app ls -la /data/files
   ```

### 向量检索慢

1. 考虑使用专用向量数据库（Qdrant/Milvus）替代 pgvector
2. 调整 embedding 批处理参数
3. 启用语义缓存：
   ```ini
   SEMANTIC_CACHE_ENABLED=true
   ```

### 内存占用过高

1. 减少并发数：
   ```ini
   CONCURRENCY_POOL_SIZE=2
   ```

2. 限制文档解析并发：
   ```ini
   DOCREADER_MARKITDOWN_MAX_WORKERS=1
   DOCREADER_PDF_RENDER_MAX_WORKERS=1
   ```

---

## 更新升级

```bash
cd /opt/xinwiki

# 拉取最新代码
git pull

# 拉取最新镜像
docker compose pull

# 重启服务（会自动执行数据库迁移）
docker compose up -d
```

> 💡 升级前建议先执行 [数据备份](#数据备份)。

---

## 获取帮助

- **项目地址**: https://github.com/tohnee/xinwiki-new
- **Issue 反馈**: https://github.com/tohnee/xinwiki-new/issues
- **常见问题**: 参考 [FAQ](./wiki/运维排障/常见问题.md)
