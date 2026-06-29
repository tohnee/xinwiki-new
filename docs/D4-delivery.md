# D4 里程碑：向量数据库读写分离架构 — 开发交付文档

> 交付日期：2026-06-28  
> 测试状态：全部 54 个单元测试通过  
> 默认配置：读写分离关闭（`Enabled: false`），零侵入向后兼容

---

## 一、架构设计

### 1.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        调用层（上层服务）                          │
│   knowledge_service / chunk_service / session_service            │
└──────────────────────────┬──────────────────────────────────────┘
                           │ RetrieveEngineService 接口
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                   RouterWrapper（透明包装器）                      │
│   · 零侵入：上层代码无需任何修改                                    │
│   · 读请求 → GetReader() → 路由到副本/主节点                       │
│   · 写请求 → GetWriter() → 路由到主节点                            │
└──────────────────────────┬──────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                   VectorStoreRouter（核心路由器）                   │
│                                                                   │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────────┐ │
│  │ 一致性路由   │  │ 负载均衡器    │  │ 健康检查 + 熔断器        │ │
│  │             │  │              │  │                         │ │
│  │ Strong→主   │  │ RoundRobin   │  │ 周期性 HealthCheck      │ │
│  │ Session→LSN │  │ LeastConn    │  │ 连续失败N次→熔断OPEN     │ │
│  │ Eventual→LB │  │              │  │ 恢复→熔断CLOSE          │ │
│  └─────────────┘  └──────────────┘  └─────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
┌──────────────────┐ ┌──────────┐ ┌──────────┐
│  RWCapableEngine │ │ Replica  │ │ Replica  │
│  (主节点/写)      │ │ (读副本)  │ │ (读副本)  │
│                  │ │          │ │          │
│ ┌──────────────┐ │ │ ┌──────┐ │ │ ┌──────┐ │ │
│ │WriteBuffer   │ │ │ │Health│ │ │ │Health│ │ │
│ │(批量写入缓冲) │ │ │ │Check │ │ │ │Check │ │ │
│ └──────────────┘ │ │ └──────┘ │ │ └──────┘ │ │
└──────────────────┘ └──────────┘ └──────────┘
```

### 1.2 请求路由流程

```
客户端读请求
    │
    ├─ Enabled=false ──────────────────────→ 主节点（直读）
    │
    ├─ Enabled=true, Consistency=Strong ──→ 主节点（强一致）
    │
    ├─ Enabled=true, Consistency=Session ─→ 等待副本LSN追平
    │                                         ├─ 追平→副本读
    │                                         └─ 超时→降级主节点
    │
    └─ Enabled=true, Consistency=Eventual ─→ 负载均衡选健康副本
                                                ├─ 有健康副本→副本读
                                                └─ 无健康副本→降级主节点

客户端写请求
    │
    └─→ 主节点（所有写操作都走主节点）
            ├─ 经WriteBuffer批量缓冲
            └─ 返回WriteToken（含LSN）
```

---

## 二、关键模块说明

### 2.1 VectorStoreRouter（核心路由器）

**文件**：`internal/application/service/vectorstore_router.go`

核心职责：
- 根据一致性级别（Strong/Session/Eventual）路由读请求
- 管理主节点和副本的健康状态与熔断器
- 维护 LSN 水位追踪，支持会话一致性
- 周期性健康检查，自动隔离故障节点

关键配置项：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `Enabled` | `false` | 读写分离总开关 |
| `HealthCheckInterval` | `5s` | 健康检查周期 |
| `CircuitBreakerThreshold` | `5` | 连续失败N次触发熔断 |
| `MaxReplicationLag` | `1s` | 副本最大允许延迟 |
| `WaitForLSNTimeout` | `500ms` | Session一致性等待超时 |

### 2.2 LoadBalancer（负载均衡器）

**文件**：`internal/application/service/load_balancer.go`

提供两种策略：
- **RoundRobinBalancer**：轮询选择，默认策略
- **LeastConnectionsBalancer**：最小连接数选择

### 2.3 RWCapableEngine Adapter（引擎适配器）

**文件**：`internal/application/service/rw_engine_adapter.go`

零侵入包装现有引擎，自动扩展：
- `WriteToken` 生成（每次写操作后递增 LSN）
- `HealthCheck` 委托给底层引擎
- `WaitForLSN` 单节点模式直接返回成功

### 2.4 WriteBuffer（写入缓冲）

**文件**：`internal/application/service/write_buffer.go`

批量写入缓冲，减少主节点写入压力：
- 按 `MaxBatchSize` 或 `MaxWaitTime` 触发批量刷盘
- 多 worker 并发处理
- 关闭后入队请求降级为直接写主节点

### 2.5 RouterWrapper（透明包装器）

**文件**：`internal/application/service/vectorstore_router_wrapper.go`

将路由器包装为 `RetrieveEngineService` 接口，上层代码零改动：
- `Retrieve()` → `GetReader()` → 路由到副本/主节点
- `Index()/Delete()/Update()` → `GetWriter()` → 路由到主节点

### 2.6 可观测性

**文件**：`internal/application/service/vectorstore_metrics.go`

Prometheus 指标埋点：
- `read_requests_total`：读请求计数（按 storeID/nodeType/consistency/result）
- `write_requests_total`：写请求计数
- `circuit_breaker_state`：熔断器状态（0=Closed, 1=Open）
- `node_health_check_latency`：健康检查延迟
- `replica_lsn_lag`：副本 LSN 延迟
- `healthy_nodes_gauge`：健康节点数
- `request_latency`：请求延迟

---

## 三、测试覆盖

### 3.1 测试文件清单

| 测试文件 | 测试数 | 覆盖模块 |
|---------|--------|---------|
| `vectorstore_router_test.go` | 10 | 路由、熔断、健康检查、降级 |
| `load_balancer_test.go` | 5 | 轮询、最小连接、空节点池 |
| `rw_engine_adapter_test.go` | 12 | LSN递增、Token生成、错误传播、委托 |
| `write_buffer_test.go` | 10 | 批量刷盘、超时刷盘、关闭、并发 |
| `vectorstore_router_wrapper_test.go` | 14 | 读写路由、全接口委托、StoreID解析 |
| `knowledge_create_test.go` | 1 | VectorStore 绑定验证 |
| **合计** | **54** | |

### 3.2 熔断机制测试场景

| 场景 | 验证点 |
|------|--------|
| 主节点连续故障 | 连续失败达阈值后熔断器 OPEN |
| 主节点恢复 | 健康检查通过后熔断器 CLOSED |
| 副本故障降级 | 副本熔断后读请求降级到主节点 |

---

## 四、部署方式

### 4.1 关闭读写分离（默认，零风险）

无需任何配置变更，行为与升级前完全一致。

### 4.2 开启读写分离

在 `container.go` 的 `initRetrieveEngineRegistry` 中修改默认配置：

```go
rwConfig := types.DefaultReadWriteSeparationConfig()
rwConfig.Enabled = true  // 开启读写分离
```

注册副本节点：

```go
router.RegisterEngineWithConfig(storeID, master, storeCfg, replicas)
```

---

## 五、已知限制与后续规划

1. **熔断器无半开状态**：当前仅 Closed/Open 两态，未实现 Half-Open 探测
2. **WriteBuffer 无合并优化**：当前逐条处理，未合并同类型操作
3. **副本配置为静态注册**：暂不支持从数据库动态加载副本列表
4. **GetRouter() 方法保留**：返回 Router 接口，当前无调用方，预留高级配置场景
