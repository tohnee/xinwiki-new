# XinWiki 代码对齐审计报告

> 审计基线：`docs/zh/xinwiki开发计划.md` + `docs/zh/xinwiki-vibe-coding-plan.md`  
> 审计日期：2026-06-28  
> 审计范围：`internal/` 全量代码  
> 报告用途：作为后续开发完善的基线参考

---

## 一、各里程碑完成度总览

| 里程碑 | 计划子项 | 已实现 | 部分实现 | 缺失 | 完成度 |
|--------|---------|--------|---------|------|--------|
| **A 派生知识权限传播** | A1-A5 | A1 A2 A3 A4 | — | A5 | 80% |
| **B Wiki生命周期** | B1-B3 | B1 B2 B3 | — | — | 100% |
| **C 置信度/质量/保鲜** | C1-C3 | C1 C2 C3 | — | — | 100% |
| **D LLM Gateway成本闭环** | D1-D3 | D2 | D1 | D3 | 55% |
| **E 治理与评测** | E1-E3 | E1 | E3 | E2 | 50% |
| **D4 读写分离**(超计划) | 6模块 | 全部 | — | — | 100% |

---

## 二、逐项审计详情

### 里程碑 A：派生知识权限传播（80%）

#### A1 Chunk/Wiki 密级与ACL字段 — 已实现

**证据文件**：
- `internal/types/chunk.go` — 含 `SecurityLevel` `AllowedUserIDs` `AllowedGroupIDs`
- `internal/types/wiki_page.go` — 同上
- `internal/types/const.go` — 定义 `SecurityLevelL1`~`L4` 常量
- `internal/application/repository/wiki_page.go` — 读写 ACL 字段
- 老数据默认 `security_level = L1`，空 ACL 沿用 RBAC 不解释为公开

#### A2 派生ACL计算器 — 已实现

**证据文件**：
- `internal/acl/acl_propagation.go` — 完整实现 `CalculateDerivedACL()`
  - 密级取最高 `max(Source.security_level)`
  - 用户ID取交集 `intersection(Source.allowed_user_ids)`
  - 组ID取交集 `intersection(Source.allowed_group_ids)`
- `internal/acl/acl_propagation_test.go` — 表驱动测试覆盖全部分支
- 辅助函数完整：
  - `ChunkToACLSource` / `WikiPageToACLSource` — 类型转换
  - `ApplyDerivedACLToWikiPage` — 应用派生ACL
  - `UserCanAccessChunk` / `UserCanAccessWikiPage` — 权限判断
  - `FilterChunksByACL` / `FilterSearchResultChunksByACL` — 批量过滤
  - `CalculateDerivedACLFromChunks` — 从Chunk直接计算
- 纯函数无数据库依赖，可被 Wiki/缓存/报告/Agent 复用

#### A3 结晶时继承来源权限 — 已实现

**证据文件**：
- `internal/agent/tools/wiki_write_page.go` — 接入 `acl.CalculateDerivedACL`
- Wiki 草稿写入时落库 `SecurityLevel` 和 ACL

#### A4 检索后二次ACL过滤 — 已实现

**证据文件**：
- `internal/application/service/knowledgebase_search.go` 第289-290行 — 调用 `applyACLFilter()` 做检索后过滤
- 第193-194行 — 缓存命中也做ACL过滤
- `internal/application/service/semantic_cache_acl_test.go` — 跨权限缓存命中测试
  - L1/L2/L3 用户分级过滤
  - 特定用户 ACL 过滤
  - 特定组 ACL 过滤

#### A5 来源权限变更触发ACL重算 — 缺失（P0 安全缺口）

**计划要求**：
1. 监听 `internal/event` 中权限、KB、文档变更事件
2. 触发派生 Wiki、缓存、索引 ACL 重算
3. 增加定时补偿任务避免事件丢失

**当前状态**：
- `internal/event/event.go` 事件总线已存在，支持 `On()` / `Emit()` / `EmitAndWait()`
- 事件类型仅覆盖 RAG 链路（query/retrieval/rerank/merge/chat/agent）
- **缺少** `EventPermissionChanged` / `EventDocumentACLUpdated` / `EventKBMemberChanged` 事件类型
- `internal/event/event_data.go` 无权限变更相关 Data 结构
- **无 ACL 重算订阅者**
- **无定时补偿任务**

**风险**：来源 Chunk 密级从 L1 升为 L3 后，派生 Wiki 仍保持 L1，导致权限泄露。

---

### 里程碑 B：Wiki生命周期与审核工作流（100%）

#### B1 状态机扩展 — 已实现

**证据文件**：`internal/types/wiki_page.go` 第133-182行

6种状态全部定义：`draft` / `reviewing` / `published` / `deprecated` / `superseded` / `archived`

状态转移表完全对齐计划：
- draft → reviewing
- reviewing → draft / published
- published → deprecated / superseded
- deprecated → archived
- superseded → archived
- archived → 仅自循环（不可恢复，需管理员显式接口）

`ValidateWikiPageStatusTransition()` 校验非法转移。

#### B2 审核流接口 — 已实现

**证据文件**：
- `internal/types/wiki_page.go` 第886-896行 — 全部审核动作定义
- `internal/handler/wiki_page.go` — API 路由
- `internal/application/service/wiki_page.go` — 业务逻辑
- `internal/application/service/wiki_page_test.go` — 测试

审核动作：Submit / Approve / Reject / Deprecate / Archive / Supersede

#### B3 替代关系 — 已实现

**证据文件**：
- `internal/types/wiki_page.go` 第920-955行 — `WikiSupersession` 结构体
  - `OldPageID` `NewPageID` `Reason` `CreatedBy` `CreatedAt`
- `internal/wikiquality/scores.go` — `RetrievalBoostSuperseded` 检索降权

---

### 里程碑 C：置信度、质量评分与保鲜（100%）

#### C1 置信度加权模型 — 已实现

**证据文件**：`internal/wikiquality/scores.go`

公式严格对齐计划：
```
ConfidenceScore = 0.30*SourceAuthority + 0.20*EvidenceSupport + 0.20*Recency
                + 0.15*Consistency + 0.10*ExpertValidation + 0.05*UsageFeedback
FinalScore = ConfidenceScore * ContradictionPenalty * StalenessPenalty
```

函数列表：
- `CalculateConfidenceScore()` — 六维加权
- `CalculateContradictionPenalty()` — 矛盾惩罚 `max(0.5, 1.0 - count*0.1)`
- `CalculateStalenessPenalty()` — 过期惩罚 `1.0 - days*0.003`，下限0.3
- `CalculateFinalScore()` — 最终评分
- `CalculateSourceAuthority()` — 来源权威性
- `CalculateEvidenceSupport()` — 证据支持度

#### C2 质量评分 — 已实现

**证据文件**：`internal/wikiquality/scores.go`

六维评分权重对齐计划：
| 维度 | 计划权重 | 实现 | 函数 |
|------|---------|------|------|
| 内容完整性 | 20% | 20% | `CalculateContentCompleteness()` |
| 来源可靠性 | 20% | 20% | `CalculateSourceAuthority()` |
| 时效性 | 15% | 15% | `CalculateRecencyScore()` |
| 可读性 | 15% | 15% | `CalculateReadabilityScore()` |
| 引用充分性 | 15% | 15% | `CalculateCitationSufficiency()` |
| 使用反馈 | 15% | 15% | `calculateUsageFeedbackScore()` |

#### C3 保鲜/遗忘权重 — 已实现

**证据文件**：`internal/wikiquality/scores.go`

保鲜状态对齐计划：
| 状态 | 计划条件 | 实现条件 | 检索权重 |
|------|---------|---------|---------|
| 活跃 | 近期访问或更新 | <=30天 | 1.0 |
| 温存 | 30-90天 | 30-90天 | 0.8 |
| 冷存 | 90-180天 | 90-180天 | 0.5 |
| 归档 | 长期未更新+低置信 | >180天 | 默认不检索 |

- `ShouldAutoArchive()` — P0/P1 不自动归档
- `CalculateRetrievalBoost()` — 按状态和保鲜度返回权重
- `UpdateAllScores()` — 统一更新所有评分
- `RecordPageAccess()` / `RecordFeedback()` — 访问和反馈记录

---

### 里程碑 D：LLM Gateway成本闭环（55%）

#### D1 成本看板API — 部分实现

**已实现**：
- `internal/application/service/cost_tracking.go` — `LogCall()` 记录 LLM 调用
  - 含 `EstimatedCost` 自动计算
  - 含 `TotalTokens` 自动汇总
- `internal/types/llm_call_log.go` — 日志数据结构
- `internal/handler/cost_tracking.go` — Handler 路由

**缺失**：
- 缺少租户级成本趋势聚合 API
- 缺少 System Admin vs Tenant Admin 权限隔离测试

#### D2 权限安全语义缓存 — 已实现

**证据文件**：
- `internal/application/service/semantic_cache.go` — 语义缓存
- `internal/application/service/semantic_cache_redis.go` — Redis 实现
- `internal/application/service/semantic_cache_memory.go` — 内存实现
- `internal/application/service/semantic_cache_acl_test.go` — ACL 测试
  - L1/L2/L3 用户分级过滤
  - 特定用户/组 ACL 过滤
  - 缓存命中后仍做 ACL 过滤

#### D3 模型路由与Prompt模板版本化 — 缺失

**计划要求**：
1. 模型路由策略（按任务类型、租户策略、成本预算、延迟目标）
2. Prompt 模板版本管理
3. LLM 调用日志记录 route_decision 与 template_version

**当前状态**：
- 无 ModelRouter 服务
- 无 PromptTemplateVersion 管理
- `llm_call_log.go` 无 `route_decision` 和 `prompt_template_version` 字段

---

### 里程碑 E：治理与评测（50%）

#### E1 文档去重 — 已实现

**证据文件**：
- `internal/application/service/wiki_ingest_dedup.go` — 预过滤+LLM去重
  - Jaccard 相似度预过滤（`dedupCandidateScoreFloor = 0.08`）
  - 小语料 bypass（`dedupSmallCorpusBypass = 25`）
  - Top-K 候选（`dedupCandidateTopK = 20`）
- `internal/application/service/wiki_ingest_dedup_test.go` — 测试

#### E2 低风险矛盾检测 — 缺失

**计划要求**：
- 同一实体同一属性不同值
- 同一接口不同参数定义
- 同一系统不同负责人
- 明确数值冲突

**当前状态**：
- WikiPage 有 `ContradictionCount` 字段，但依赖人工设置
- `CalculateContradictionPenalty()` 使用该字段计算惩罚
- 无自动检测服务

#### E3 RAG评测集固化 — 部分缺失

**已实现指标**（`internal/application/service/metric/`）：
- Recall / Precision / MRR / NDCG / MAP / BLEU / ROUGE

**缺失指标**：
- Permission Leakage Rate — CI 红线 = 0，未实现
- Citation Accuracy — 未实现
- Faithfulness — 未实现
- Latency P95 — 未实现
- CI 红线门禁 — 未接入

---

### D4 读写分离架构 — 已实现（超计划）

6个模块全部实现，54个测试通过：
- VectorStoreRouter（路由器+熔断器+健康检查）
- LoadBalancer（RoundRobin + LeastConnections）
- RWCapableEngine Adapter（引擎适配器）
- WriteBuffer（写入缓冲）
- RouterWrapper（透明包装器）
- Prometheus 指标埋点

---

## 三、缺漏优先级排序

| 优先级 | 缺漏项 | 里程碑 | 风险 | 影响 |
|--------|--------|--------|------|------|
| **P0** | A5: 事件驱动ACL重算 | A | 权限泄露 | Wiki/缓存/索引 |
| **P0** | E3: Permission Leakage CI红线 | E | 安全无门禁 | 全局 |
| **P1** | D3: 模型路由+Prompt版本化 | D | 成本无法优化 | LLM全链路 |
| **P1** | E2: 矛盾检测服务 | E | 冲突无法发现 | 知识质量 |
| **P1** | E3: Citation Accuracy评测 | E | 引用无监控 | RAG质量 |
| **P2** | D1: 成本趋势聚合API | D | 无成本视图 | 运营 |
| **P2** | A5: 定时补偿任务 | A | 事件丢失 | 一致性 |
| **P3** | E3: Faithfulness评测 | E | 幻觉无监控 | 回答质量 |

---

## 四、贯穿性验收红线检查

| 红线 | 状态 | 说明 |
|------|------|------|
| 权限泄露率 = 0 | 未验证 | A4有过滤但A5缺失导致变更后可能泄露 |
| 测试通过 | 通过 | D4相关54个测试通过 |
| 新功能带测试 | 通过 | 所有新模块都有测试 |
| 派生知识不绕过来源权限 | 未验证 | 初始创建OK，变更后无重算 |
| 核心写操作写审计 | 通过 | Wiki审核流写审计 |
| 跨权限缓存命中 = 0 | 通过 | 语义缓存有ACL过滤测试 |
| 不回归RAG引用准确率 | 未验证 | 缺少Citation Accuracy基线 |
| 不回归P95延迟 | 未验证 | 缺少P95延迟基线 |

---

## 五、P0 缺漏实施路线图

### A5: 事件驱动ACL重算

**需修改/新增的文件**：

| 文件 | 变更类型 | 关键逻辑 |
|------|---------|---------|
| `internal/event/event.go` | 修改 | 新增 `EventPermissionChanged` `EventDocumentACLUpdated` `EventKBMemberChanged` 事件类型常量 |
| `internal/event/event_data.go` | 修改 | 新增 `PermissionChangedData` 结构体（含 TenantID/ResourceType/ResourceID/OldACL/NewACL） |
| `internal/acl/acl_recompute.go` | 新增 | ACL重算订阅者：监听事件 → 查找关联Wiki → 重算ACL → 失效缓存 |
| `internal/acl/acl_recompute_test.go` | 新增 | 测试：来源密级变更后Wiki ACL自动更新、事件幂等、缓存失效 |
| `internal/acl/acl_reconcile.go` | 新增 | 定时补偿任务：扫描Wiki ACL与来源ACL一致性，修复不一致 |
| `internal/container/container.go` | 修改 | 注册ACL重算订阅者到EventBus |

**关键测试用例**：
1. 来源Chunk密级 L1→L3，派生Wiki自动升为L3
2. 来源 allowed_users 变更后Wiki ACL重新计算
3. 事件重复投递幂等
4. 事件丢失时补偿任务可修复
5. 缓存被失效或重新计算
6. 审计记录ACL重算原因

### E3: Permission Leakage CI红线

**需新增的文件**：

| 文件 | 变更类型 | 关键逻辑 |
|------|---------|---------|
| `internal/application/service/metric/permission_leakage.go` | 新增 | Permission Leakage评测指标实现 |
| `internal/application/service/metric/permission_leakage_test.go` | 新增 | 测试：无权限用户检索高密级Chunk结果为0等 |
| `internal/application/service/metric/citation_accuracy.go` | 新增 | Citation Accuracy评测指标 |
| `internal/application/service/metric/citation_accuracy_test.go` | 新增 | 测试：引用准确率计算 |
| `scripts/ci/permission_leakage_check.sh` | 新增 | CI红线脚本：运行权限泄露测试，非0则退出 |

**关键测试用例**：
1. 无权限用户检索高密级Chunk，结果为0
2. 无权限用户不能看到高密级Wiki标题
3. 无权限用户不能通过引用URL反推文档
4. metadata filter漏掉时应用层过滤仍能兜底
5. 缓存命中前再次确认ACL hash
6. 来源权限变更后派生Wiki ACL自动更新（与A5联动）

---

## 六、建议执行顺序

```
第1步: A5 事件驱动ACL重算（P0安全红线）
  ├── 新增事件类型和Data结构
  ├── 实现ACL重算订阅者
  ├── 实现定时补偿任务
  └── 编写测试（含幂等、补偿、审计）

第2步: E3 Permission Leakage CI红线（P0安全红线）
  ├── 实现Permission Leakage评测指标
  ├── 实现Citation Accuracy评测指标
  ├── 编写CI红线脚本
  └── 接入CI流程

第3步: D3 模型路由+Prompt版本化（P1）
  ├── 实现ModelRouter服务
  ├── LLM调用日志增加route_decision字段
  └── Prompt模板版本管理

第4步: E2 矛盾检测服务（P1）
  ├── 实现实体属性冲突扫描器
  ├── 输出治理建议（不自动裁决）
  └── 写审计

第5步: D1 成本趋势聚合API（P2）
  ├── 实现租户/模型/时间窗口聚合查询
  └── 权限隔离测试

第6步: E3 补充 Faithfulness + P95 Latency 评测（P3）
  ├── 实现Faithfulness评测指标
  └── 实现P95延迟基线
```

---

## 七、已实现模块质量评价

### 代码质量良好的模块

| 模块 | 评价 |
|------|------|
| ACL传播计算器 | 纯函数设计，无DB依赖，表驱动测试覆盖全分支，可复用性强 |
| Wiki状态机 | 状态转移表严格对齐计划，非法转移有明确错误 |
| 置信度/质量评分 | 公式严格对齐计划，辅助函数完整，有测试 |
| 保鲜/遗忘权重 | 规则清晰，P0/P1保护逻辑正确 |
| 语义缓存ACL | 跨权限缓存命中测试覆盖L1/L2/L3+用户/组ACL |
| 文档去重 | Jaccard预过滤+LLM去重，小语料bypass优化 |
| D4读写分离 | 6模块完整，54测试通过，导入循环已修复 |

### 需要加固的模块

| 模块 | 问题 | 建议 |
|------|------|------|
| 事件系统 | 无ACL重算订阅者，事件类型仅覆盖RAG链路 | 新增权限变更事件+订阅者 |
| LLM调用日志 | 缺少route_decision和prompt_template_version字段 | 扩展结构体 |
| 评测系统 | 缺少Permission Leakage/Citation Accuracy/Faithfulness | 新增评测指标 |
| 成本看板 | 仅有单条日志，缺少聚合查询 | 新增聚合API |
| 矛盾检测 | ContradictionCount依赖人工设置 | 新增自动检测服务 |

---

## 八、结论

### 安全红线状态：⚠️ 存在缺口

当前最高优先级安全缺口是 **A5（事件驱动ACL重算）** 和 **E3（Permission Leakage CI红线）**。

- A4的应用层ACL过滤在**初始创建时**有效，但**来源权限变更后**派生知识不会自动更新ACL
- 缺少自动化的Permission Leakage评测，无法防止权限泄露随代码变更回归

### 已完成里程碑质量：✅ 良好

里程碑B（Wiki生命周期）和C（置信度/质量/保鲜）已100%实现，代码质量良好，测试覆盖完整。

### 超计划完成：✅ D4读写分离

D4读写分离架构（6个模块、54个测试）已完整交付，含熔断器、负载均衡、写入缓冲等企业级能力。

### 下一步行动建议

1. **立即启动** A5 + E3 的实现（P0安全红线）
2. **随后推进** D3 模型路由 + E2 矛盾检测（P1）
3. **最后补齐** D1 成本聚合 + E3 补充指标（P2/P3）