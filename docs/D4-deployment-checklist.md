# D4 里程碑：生产环境部署配置检查清单

> 部署前请逐项确认，所有标记 [必须] 的项目必须在部署前完成。

---

## 一、环境变量配置（.env）

### 1.1 [必须] 核心服务

| 配置项 | 检查内容 | 示例值 |
|--------|---------|--------|
| `GIN_MODE` | 生产环境必须为 `release` | `release` |
| `LOG_LEVEL` | 生产环境建议 `info` 或 `warn` | `info` |
| `TZ` | 时区设置 | `Asia/Shanghai` |
| `DISABLE_REGISTRATION` | 生产环境建议 `true` | `true` |

### 1.2 [必须] 数据库

| 配置项 | 检查内容 |
|--------|---------|
| `DB_DRIVER` | 确认数据库类型（postgres/mysql） |
| `DB_HOST` / `DB_PORT` | 确认数据库地址可达 |
| `DB_USER` / `DB_PASSWORD` | 确认凭据有效，密码已强加密 |
| `DB_NAME` | 确认数据库名称正确 |

### 1.3 [必须] 向量存储

| 配置项 | 检查内容 |
|--------|---------|
| `RETRIEVE_DRIVER` | 确认向量库类型（postgres/elasticsearch_v7/qdrant/milvus/weaviate/doris/tencent_vectordb/opensearch） |
| 向量库连接参数 | 确认对应向量库的地址、端口、认证信息已配置 |
| `SSRF_WHITELIST` | 如使用内网地址，确认已加入白名单 |

### 1.4 [必须] 安全配置

| 配置项 | 检查内容 |
|--------|---------|
| `JWT_SECRET` | 确认已设置为强随机值（非默认值） |
| `TENANT_AES_KEY` | 确认已设置为强随机值 |
| `SYSTEM_AES_KEY` | 确认为32字节强随机值 |
| `XINWIKI_TENANT_ENABLE_RBAC` | 生产环境建议 `true` |

### 1.5 [必须] 文件存储

| 配置项 | 检查内容 |
|--------|---------|
| `STORAGE_TYPE` | 确认存储类型（local/minio/cos/tos/s3） |
| `LOCAL_STORAGE_BASE_DIR` | 如使用 local，确认目录存在且有权限 |
| 对象存储凭据 | 如使用云存储，确认 AK/SK/Endpoint/Bucket 已配置 |

### 1.6 [建议] 可观测性

| 配置项 | 检查内容 |
|--------|---------|
| `LANGFUSE_PUBLIC_KEY` / `SECRET_KEY` | 如需链路追踪，确认已配置 |
| `LLM_DEBUG_LOG` | 生产环境建议 `false` |
| `CONCURRENCY_POOL_SIZE` | 根据负载调整，默认5 |

---

## 二、配置文件检查（config/config.yaml）

### 2.1 [必须] 服务配置

```yaml
server:
  port: 8080          # 确认端口未被占用
  host: "0.0.0.0"     # 确认绑定地址正确
```

### 2.2 [必须] 知识库配置

```yaml
knowledge_base:
  chunk_size: 512        # 确认分块大小
  chunk_overlap: 50      # 确认分块重叠
  document_process_timeout: 2h  # 确认超时时间
```

### 2.3 [必须] 对话配置

```yaml
conversation:
  max_rounds: 5          # 确认最大对话轮数
  embedding_top_k: 30    # 确认向量检索数量
  vector_threshold: 0.2  # 确认向量相似度阈值
  rerank_threshold: 0.3  # 确认重排阈值
```

---

## 三、D4 读写分离专属检查

### 3.1 [必须] 读写分离开关

确认当前部署状态：

| 检查项 | 当前状态 | 说明 |
|--------|---------|------|
| `ReadWriteSeparationConfig.Enabled` | `false`（默认） | 关闭状态，所有请求走主节点，行为与升级前一致 |
| 向后兼容性 | 已验证 | RouterWrapper 透明包装，上层代码零改动 |

> 如果要开启读写分离，需额外完成以下检查：

### 3.2 [开启时必须] 副本节点配置

| 检查项 | 说明 |
|--------|------|
| 副本节点地址列表 | 确认 `ReadEndpoints` 已配置所有读副本地址 |
| 副本可达性 | 确认所有副本节点网络可达 |
| 副本数据同步 | 确认副本已与主节点完成初始数据同步 |

### 3.3 [开启时必须] 熔断器参数

| 参数 | 默认值 | 生产建议 | 说明 |
|------|--------|---------|------|
| `CircuitBreakerThreshold` | 5 | 3-5 | 连续失败次数，过低易误判 |
| `HealthCheckInterval` | 5s | 5-10s | 检查周期，过短增加负载 |
| `HealthCheckTimeout` | 2s | 2-3s | 检查超时 |
| `MaxReplicationLag` | 1s | 1-5s | 副本最大允许延迟 |
| `WaitForLSNTimeout` | 500ms | 500ms-1s | Session一致性等待超时 |

### 3.4 [开启时必须] 写入缓冲参数

| 参数 | 默认值 | 生产建议 | 说明 |
|------|--------|---------|------|
| `WriteMaxBatchSize` | 1000 | 500-1000 | 批量写入大小 |
| `WriteMaxWaitTime` | 100ms | 50-100ms | 攒批等待时间 |
| `WriteConcurrency` | 4 | 2-8 | 并发写入协程数 |

---

## 四、Docker Compose 部署检查

### 4.1 [必须] 镜像与版本

| 检查项 | 说明 |
|--------|------|
| `XINWIKI_VERSION` | 确认镜像版本标签（latest/main/具体版本号） |
| `docker-compose.yml` | 确认使用正确的 compose 文件 |

### 4.2 [必须] 数据持久化

| 检查项 | 说明 |
|--------|------|
| 数据库数据卷 | 确认 `postgres_data` / `mysql_data` 挂载正确 |
| 文件存储卷 | 确认 `local_storage` 挂载到持久化目录 |
| Redis 数据卷 | 确认 `redis_data` 挂载正确 |

### 4.3 [必须] 网络与端口

| 检查项 | 说明 |
|--------|------|
| `APP_PORT` | 确认应用端口映射 |
| `FRONTEND_PORT` | 确认前端端口映射 |
| 向量库端口 | 确认向量库端口映射且内部可达 |
| Redis 端口 | 确认 Redis 仅内部可达（不对外暴露） |

---

## 五、部署前最终确认

### 5.1 [必须] 编译与测试

```bash
# 编译检查
go build ./...

# 运行 D4 相关测试
go test -v -count=1 -timeout 120s ./internal/application/service/... -run "VectorStore|RoundRobin|LeastConnections|WriteBuffer|RWEngineAdapter|RouterWrapper"

# 运行 retriever 包测试
go test -v ./internal/application/service/retriever/...
```

### 5.2 [必须] 数据库迁移

```bash
# 确认所有迁移已执行
go run cmd/migrate/main.go
```

### 5.3 [建议] 灰度发布

1. 先在测试环境部署，验证核心功能
2. 生产环境先以 `Enabled=false` 部署，观察1-2天
3. 确认无异常后再考虑开启读写分离

---

## 六、回滚方案

| 场景 | 回滚操作 |
|------|---------|
| D4 代码异常 | 回退到上一个稳定版本镜像 |
| 读写分离异常 | 设置 `Enabled=false`，重启服务 |
| 副本节点故障 | 移除副本配置，降级为单节点模式 |

> 读写分离开关为运行时配置，关闭后立即生效，无需回滚代码。
