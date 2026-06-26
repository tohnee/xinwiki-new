# XinWiki 重构执行计划（Vibe Coding 可执行版）

> 文档版本：v1.1
> 日期：2026-06-25
> 基线代码：本仓库（WeKnora 衍生，VERSION = 0.6.2）
> 目标产品名：XinWiki
> 输入文档：《XinWiki 企业级知识智能平台重构方案设计文档 v2.0》
> 本文定位：把方案 v2.0 落地为小颗粒、可自测、可验收、可独立合入的 PR 级开发计划。

## 0. 关键前提：本仓库不是白纸 WeKnora

方案 v2.0 的方向是正确的，但它更像从开源 RAG 平台起步的蓝图。当前仓库已经具备多租户、RBAC、OIDC、审计、Wiki、Agent、MCP、知识图谱、数据源连接器等能力。如果照方案从零重建，会大规模重造轮子，并引入迁移、兼容性和回归风险。

因此，本计划不再按“重建 Phase 1 能力”推进，而是按以下原则执行：

```text
先核实现状
  ↓
识别真正差距
  ↓
围绕安全缺口做增量 PR
  ↓
用 TDD 和评测集固化行为
  ↓
逐步补齐知识资产治理语义
```

### 0.1 现状核对

| 方案能力 | 方案 Phase | 当前状态 | 主要证据位置 | 处理决策 |
|---|---|---|---|---|
| OIDC/UUM 登录 | P1 | 已实现 | `internal/handler/auth.go`、`docs/OIDC认证调用流程.md` | 不重建，仅按企业 UUM 做配置与兼容验证 |
| 多租户与 tenant_id 逻辑隔离 | P1 | 已实现 | `internal/handler/tenant.go`、`internal/types/tenant.go` | 不重建，补派生知识权限传播 |
| RBAC 四级角色 | P1 | 已实现 | `docs/RBAC说明.md`、`internal/middleware/rbac_*.go` | 不重建，补 Wiki/缓存/报告继承规则 |
| 审计日志 | P1 | 已实现 | `internal/handler/audit_log.go`、`internal/types/audit_log.go` | 复用现有审计，补 Wiki/ACL/治理动作 |
| 知识库、文档、Chunk、解析 | P1 | 已实现 | `internal/handler/knowledge*.go`、`docreader/` | 不重建，补 security_level 与 ACL metadata |
| RAG 检索与引用 | P1 | 已实现 | `internal/application/repository/retriever/*` | 不重建，补应用层 ACL 二次过滤回归集 |
| 限流 | P1 | 已实现 | `internal/ratelimit/limiter.go` | 复用，后续和 LLM 成本限额联动 |
| LLM 用量计量 | P1 | 部分实现 | `internal/application/repository/model_usage.go` | 补成本看板、路由、权限安全语义缓存 |
| 混合检索 BM25 + 向量 | P2 | 部分实现 | `internal/application/repository/retriever/opensearch/*` | 确认 RRF 融合与 Wiki Boost 缺口 |
| 共享空间 | P3 | 已实现 | `docs/共享空间说明.md`、`internal/handler/organization.go` | 复用，重点验证跨租户派生知识不扩大权限 |
| Wiki 页面、版本、日志 | P2 | 部分实现 | `internal/handler/wiki_page.go`、`internal/types/wiki_page.go` | 补生命周期、审核、置信度、质量评分 |
| 会话到 Wiki 草稿结晶 | P2 | 雏形 | `internal/agent/finalize.go`、`internal/agent/tools/wiki_write_page.go` | 接入派生 ACL 计算器和审核流 |
| Agent Runtime 与工具审批 | P3/P4 | 已实现 | `internal/agent/{engine,think,act,observe,approval,tools}` | 不重建，补高风险动作与派生权限用例 |
| MCP 工具接入 | P3 | 已实现 | `internal/mcp/`、`mcp-server/` | 不重建，纳入 Agent 工具权限回归 |
| Neo4j 知识图谱 | P4 | 已接入 | `internal/application/repository/memory/neo4j`、`docs/KnowledgeGraph.md` | 不作为近期主线，仅补权限过滤用例 |
| 数据源连接器 | P3 | 已实现 | `internal/datasource/` | 不重建，补同步文档 ACL 与来源追溯 |
| IM 集成 | P3 | 已实现 | `internal/im/` | 不重建，后续纳入结晶来源 |
| 事件总线 | P2 | 已实现 | `internal/event/` | 用于 ACL 重算与治理事件 |
| RAG 评测 | P2 | 部分实现 | `internal/handler/evaluation.go`、`docs/api/evaluation.md` | 补 Permission Leakage、Citation Accuracy、P95 |

### 0.2 真正的增量主攻方向

基于当前代码状态，Vibe Coding 主攻方向不是“登录、租户、RBAC、Agent、图谱从零做一遍”，而是以下四类知识资产治理语义。

1. **派生知识权限传播**：Wiki、缓存、报告、Agent 产物必须继承来源权限，规则为密级取最高、可见范围默认取交集、来源权限变更后重算 ACL。这是最高优先级安全缺口。
2. **Wiki 完整生命周期**：当前状态模型偏基础，需要补 `reviewing`、`deprecated`、`superseded`、审核流与替代关系。
3. **置信度、质量评分、保鲜/遗忘权重**：需要落库、计算、检索加权和治理看板。
4. **LLM Gateway 成本闭环**：在现有用量计量上补成本聚合、模型路由、Prompt 模板版本化和权限安全语义缓存。

### 0.3 对方案 v2.0 的修正决议

- Phase 0/1 不做重建，改为“差距审计 + 在现有 RBAC/审计之上补齐治理语义”。
- 改名 WeKnora 到 XinWiki 只改用户可见层，例如前端标题、文档、README、部署展示名；后端 Go 包路径、`WEKNORA_*` 配置前缀、迁移与内部标识保持不变，避免大范围 diff 与迁移风险。
- SAML/LDAP、Neo4j 图推理增强、局部微服务拆分、物理隔离继续后置。
- 每个 PR 必须小颗粒、可独立验收、可回滚，并带测试。

## 1. 总体 Vibe Coding 方法

### 1.1 工作循环

```text
差距卡片
  ↓
AI 辅助阅读现有代码
  ↓
人工确认不是重造轮子
  ↓
AI 产出最小设计
  ↓
先写测试与验收脚本
  ↓
AI 实现最小改动
  ↓
运行测试和权限回归集
  ↓
人工 Code Review
  ↓
审计 / 性能 / 兼容性检查
  ↓
合入
```

### 1.2 AI 生成代码硬性约束

- 不允许绕过现有 RBAC 中间件和权限服务。
- 不允许绕过现有 audit log。
- 不允许绕过模型用量计量与后续 LLM Gateway。
- 不允许新增跨租户默认公开逻辑。
- 不允许让派生知识比来源知识拥有更宽可见范围，除非有显式审核和脱敏策略。
- 不允许删除或弱化现有测试。
- 不允许在一个 PR 中同时做迁移、前端重构、RAG 策略重写和 Agent 行为修改。

### 1.3 PR 颗粒度

一个 PR 应满足：

```text
一次开发会话可完成
最多围绕一个领域概念
自带测试
可独立回滚
不会阻塞主链路 RAG
不会要求全量数据重建
```

## 2. 里程碑与 PR 拆解

推进顺序：

```text
A 派生知识权限传播
  ↓
B Wiki 生命周期与审核
  ↓
C 置信度 / 质量 / 保鲜
  ↓
D LLM Gateway 成本闭环
  ↓
E 治理与评测补强
  ↓
F 企业集成与品牌后置项
```

A 必须最先，因为它是安全缺口，也是 Wiki、缓存、报告、Agent 产物能否安全暴露的基础。

## 3. 里程碑 A：派生知识权限传播

目标：确保所有派生知识不会绕过来源权限。

### A1：Chunk / Wiki 增加密级与 ACL 字段

#### 改动范围

- 迁移新增 `security_level`、`allowed_user_ids`、`allowed_group_ids` 到 `chunks`、`wiki_pages`。
- `internal/types/wiki_page.go` 增加对应字段。
- Chunk 相关类型和索引 metadata 增加对应字段。
- 老数据默认 `security_level = L1`，ACL 为空时按现有 RBAC 行为兼容，不解释为全局公开。

#### TDD 测试

- 迁移 up/down 可执行。
- 迁移重复执行不产生脏数据。
- 老数据默认密级为 `L1`。
- Wiki repository 读写 `security_level` 与 ACL 字段往返一致。
- Chunk 索引 metadata 包含新增权限字段。

#### 验收标准

- `make test` 或项目标准测试命令通过。
- 现有 RAG 查询不回归。
- 老 Wiki 页面可继续读取。
- 新增字段不导致空 ACL 被误判为公开。

#### 推荐 Prompt

```text
请基于当前仓库实现派生知识权限传播的第一步：为 chunks 和 wiki_pages 增加 security_level、allowed_user_ids、allowed_group_ids。
要求：
1. 先写迁移和 repository 读写测试
2. 老数据默认 L1
3. 空 ACL 只能表示沿用现有 RBAC，不得表示全局公开
4. 不改动无关 RAG 策略
5. 给出回滚迁移
```

### A2：派生 ACL 计算器

#### 改动范围

新增纯函数服务，例如：

```text
internal/application/service/acl_propagation.go
```

核心规则：

```text
Derived.security_level = max(Source.security_level)
Derived.allowed_user_ids = intersection(Source.allowed_user_ids)
Derived.allowed_group_ids = intersection(Source.allowed_group_ids)
```

边界规则：

- 单来源：直接继承来源 ACL。
- 多来源：默认取交集。
- 来源 ACL 缺失：不得扩大权限，应回退为“需要运行时 RBAC 校验”。
- 有任何高密级来源：派生结果密级提升到最高。

#### TDD 测试

- 空来源返回错误或保守默认。
- 单来源继承。
- 多来源用户交集正确。
- 多来源用户无交集时结果不可见或需审核。
- 多来源 group 交集正确。
- 密级冲突取最高。
- ACL 缺失时不扩大权限。

#### 验收标准

- 表驱动测试覆盖全部分支。
- 该服务不依赖数据库，便于 Wiki、报告、缓存、Agent 复用。

### A3：结晶时继承来源权限

#### 改动范围

- 在 `internal/agent/finalize.go` 的会话结晶链路中接入 A2。
- 在 `internal/agent/tools/wiki_write_page.go` 写 Wiki 页面前接入 A2。
- Wiki 草稿写入时落库 `security_level` 和 ACL。
- 写入审计记录，记录来源与派生 ACL 摘要。

#### TDD 测试

- 一个 L1 来源生成 Wiki 草稿，页面为 L1。
- L1 + L3 来源生成 Wiki 草稿，页面为 L3。
- 两个来源 allowed users 交集为 `[u2]`，页面只允许 `[u2]`。
- 交集为空时页面进入 `reviewing` 或不可发布状态。
- 未绑定来源的自动结晶被拒绝。

#### 验收标准

- 自动结晶不能生成无来源 Wiki。
- 派生 Wiki 不得比来源更宽。
- 审计可追踪来源与 ACL 计算结果。

### A4：检索后二次 ACL 过滤 + 权限泄露回归集

#### 改动范围

- 在检索 metadata filter 之外增加应用层 ACL 二次校验。
- 复用现有 RBAC 和 KB 访问逻辑。
- 对 Chunk、Wiki、图谱结果统一做 ACL 校验接口。
- 新增 Permission Leakage 回归集并接入 CI。

#### TDD 测试

- 无权限用户检索高密级 Chunk，结果为 0。
- 无权限用户不能看到高密级 Wiki 标题。
- 无权限用户不能通过引用 URL 反推出文档。
- metadata filter 漏掉时，应用层过滤仍能兜底。
- 缓存命中前必须再次确认 ACL hash。

#### 验收标准

```text
Permission Leakage Rate = 0
无权限命中数 = 0
无权限引用数 = 0
无权限标题泄露数 = 0
```

### A5：来源权限变更触发 ACL 重算

#### 改动范围

- 监听现有 `internal/event` 中权限、KB、文档变更事件。
- 触发派生 Wiki、报告、缓存、索引 ACL 重算。
- 增加定时补偿任务，避免事件丢失造成长期不一致。

#### TDD 测试

- 来源密级从 L1 升为 L3，派生 Wiki 自动升为 L3。
- 来源 allowed users 改变后，派生 Wiki ACL 重新计算。
- 事件重复投递幂等。
- 事件丢失时补偿任务可修复。

#### 验收标准

- 权限变更后派生知识不保留旧权限。
- 缓存被失效或重新计算。
- 审计记录 ACL 重算原因。

## 4. 里程碑 B：Wiki 生命周期与审核工作流

目标：把现有 Wiki 从页面系统补齐为企业知识资产生命周期系统。

### B1：扩展 Wiki 状态机

#### 改动范围

- 扩展 `internal/types/wiki_page.go` 状态：

```text
draft
reviewing
published
deprecated
superseded
archived
```

- 定义合法状态转移表。
- 非法转移返回明确错误。

#### 状态转移建议

| From | To | 场景 |
|---|---|---|
| draft | reviewing | 提交审核 |
| reviewing | published | 审核通过 |
| reviewing | draft | 审核拒绝 |
| published | deprecated | 标记过期但保留可见 |
| published | superseded | 被新页面替代 |
| deprecated | archived | 归档 |
| superseded | archived | 归档 |

#### TDD 测试

- 合法转移成功。
- 非法转移失败。
- archived 不允许恢复为 published，除非走管理员显式恢复接口。
- 状态变更写审计。

### B2：审核流接口

#### 改动范围

新增或扩展接口：

```text
submit_review
approve
reject
deprecate
supersede
archive
```

权限建议：

- Contributor 可提交审核。
- Admin / Owner 可 approve、reject、deprecate、supersede、archive。
- Viewer 只能查看有权限且已发布的页面。

#### TDD 测试

- Contributor 提交审核成功。
- Viewer 提交审核失败。
- Admin approve 成功。
- 未授权用户 approve 返回 403。
- 审核动作写 `wiki.*` 审计。

### B3：替代关系

#### 改动范围

- 新增 `wiki_supersession` 表或复用可扩展关系表。
- 记录 old_page_id、new_page_id、reason、created_by、created_at。
- 检索时 superseded 旧页降权或默认隐藏。

#### TDD 测试

- A 被 B 替代后，A 状态为 `superseded`。
- 普通检索默认返回 B，不返回 A。
- 用户访问 A 时提示已被 B 替代。
- 替代关系可审计。

## 5. 里程碑 C：置信度、质量评分与保鲜

目标：让知识不只是能写入，还能被评估、运营和持续治理。

### C1：置信度加权模型

#### 计算模型

```text
ConfidenceScore =
  0.30 × SourceAuthority
+ 0.20 × EvidenceSupport
+ 0.20 × Recency
+ 0.15 × Consistency
+ 0.10 × ExpertValidation
+ 0.05 × UsageFeedback

FinalScore = ConfidenceScore × ContradictionPenalty × StalenessPenalty
```

#### 改动范围

- 新增 `wiki_confidence` 表。
- 新增计算服务。
- Wiki 页面列表与详情返回置信度。

#### TDD 测试

- 权重计算正确。
- penalty 生效。
- 缺失维度使用保守默认。
- 专家验证可提升分数但不能突破上限。

### C2：质量评分

#### 质量维度

| 维度 | 权重 |
|---|---:|
| 内容完整性 | 20% |
| 来源可靠性 | 20% |
| 时效性 | 15% |
| 可读性 | 15% |
| 引用充分性 | 15% |
| 使用反馈 | 15% |

#### TDD 测试

- 六维评分计算正确。
- 引用不足时扣分。
- 过期页面扣分。
- 高负反馈页面进入治理队列。

### C3：保鲜/遗忘权重

#### 规则

| 状态 | 条件 | 检索权重 |
|---|---|---:|
| 活跃 | 近期访问或更新 | 1.0 |
| 温存 | 30-90 天未更新 | 0.8 |
| 冷存 | 90-180 天未更新 | 0.5 |
| 归档 | 长期未更新且低置信 | 默认不检索 |

`criticality_level = P0 / P1` 的页面不自动归档。

#### TDD 测试

- 时间快进后权重变化正确。
- P0 / P1 页面不自动归档。
- archived 页面默认不进入检索。
- deprecated 页面降权。

## 6. 里程碑 D：LLM Gateway 成本闭环

目标：在现有模型用量计量基础上形成可运营的成本闭环。

### D1：成本看板 API

#### 改动范围

- 在现有 `model_usage` 数据基础上按 tenant、model、user、时间窗口聚合。
- 增加成本估算字段。
- 增加租户级成本趋势 API。
- 前端增加基础看板。

#### TDD 测试

- token 聚合正确。
- 成本估算正确。
- Tenant Admin 只能看本租户。
- System Admin 可看全局。

### D2：权限安全语义缓存

#### 缓存 key 必须包含

```text
tenant_id
user_acl_hash
kb_ids
kb_version_hash
model_id
prompt_template_version
normalized_query
retrieval_policy
security_level
```

#### TDD 测试

- 同 query、不同 tenant 必 miss。
- 同 query、不同 user_acl_hash 必 miss。
- KB 内容变更后必 miss。
- prompt template 版本变化后必 miss。
- security_level 变化后必 miss。

#### 验收红线

```text
跨权限缓存命中 = 0
```

### D3：模型路由与 Prompt 模板版本化

#### 改动范围

- 模型路由策略：按任务类型、租户策略、成本预算、延迟目标选择模型。
- Prompt 模板版本管理。
- LLM 调用日志记录 route decision 与 template version。

#### TDD 测试

- 指定租户命中指定模型。
- 超预算降级模型。
- 高安全任务禁用外部模型。
- Prompt 版本变化进入日志。

## 7. 里程碑 E：治理与评测补强

### E1：文档去重

#### 改动范围

- P1：hash 去重与标题相似去重。
- P2：向量语义去重和 Chunk 级重复检测。

#### TDD 测试

- 完全相同文件识别为重复。
- 标题高度相似识别为候选重复。
- 不同租户之间默认不跨租户去重暴露内容。
- 去重建议写审计。

### E2：低风险矛盾检测

#### 安全集合

- 同一实体同一属性不同值。
- 同一接口不同参数定义。
- 同一系统不同负责人。
- 同一流程不同版本。
- 明确数值冲突。

#### TDD 测试

- 同实体属性冲突可识别。
- 不同实体不误报。
- 低置信来源不自动覆盖高置信来源。
- 冲突只生成治理建议，不自动裁决。

### E3：RAG 评测集固化

#### 指标

- Recall@K。
- MRR。
- NDCG。
- Answer Correctness。
- Faithfulness。
- Citation Accuracy。
- Permission Leakage Rate。
- Latency P95。

#### CI 红线

```text
Permission Leakage Rate = 0
Citation Accuracy 不低于基线
Latency P95 不高于基线阈值
```

## 8. 里程碑 F：企业集成与品牌后置项

### F1：SAML / LDAP

当前不是最高优先级。只有在目标企业确实要求非 OIDC 登录时启动。

### F2：Neo4j 图推理增强

当前已有 Neo4j 接入，近期不做大规模图谱重构。优先补图谱查询权限过滤与来源追溯测试。

### F3：局部微服务拆分

暂不拆。只有当 LLM Gateway、Parser、Search、Wiki、Agent Runtime 出现明确扩展瓶颈时，再从接口抽象开始拆分。

### F4：品牌改名 XinWiki

只改用户可见层：

- 前端标题。
- 页面 Logo 和文案。
- README 和文档展示名。
- 镜像描述。
- Helm values 中展示名。

不改：

- Go module path。
- 数据库表前缀。
- 历史迁移。
- `WEKNORA_*` 环境变量。
- 内部 package 名。

## 9. 贯穿性验收红线

每个 PR 都必须满足：

```text
1. 权限泄露率 = 0
2. make test 或项目标准测试命令通过
3. 新功能必须带测试
4. 派生知识不得绕过来源权限
5. 核心写操作必须写审计
6. 跨权限缓存命中 = 0
7. 不回归现有 RAG 引用准确率
8. 不回归现有 P95 延迟基线
9. 不重建已实现的大件能力
10. 不引入大范围无关 diff
```

## 10. 每类 PR 的标准 DoD

### 10.1 数据迁移 PR

- 有 up/down 迁移。
- 幂等或明确说明不可重复执行边界。
- 老数据默认值安全。
- 有 repository 读写测试。
- 有回滚验证。

### 10.2 权限 PR

- 有正向权限测试。
- 有越权测试。
- 有跨租户测试。
- 有缓存与引用泄露测试。
- 有审计测试。

### 10.3 RAG PR

- 有 Recall / Citation 回归。
- 有 Permission Leakage 回归。
- 有 P95 延迟记录。
- 有无引用降级策略。
- 有检索前 filter 和检索后 ACL 校验。

### 10.4 Wiki PR

- 有状态流转测试。
- 有来源追溯测试。
- 有权限继承测试。
- 有审计测试。
- 有检索可见性测试。

### 10.5 Agent PR

- 有工具权限测试。
- 有人工确认测试。
- 有失败恢复测试。
- 有逐步审计测试。
- 有高风险动作阻断测试。

## 11. 推荐 Vibe Coding Prompt 模板

### 11.1 差距审计 Prompt

```text
你是 XinWiki 企业级重构工程师。
请先阅读当前仓库中与【能力名称】相关的代码和文档，判断该能力是：
1. 已实现
2. 部分实现
3. 未实现

要求输出：
- 证据文件
- 现有调用链
- 缺口列表
- 不建议重写的部分
- 建议最小增量 PR
不要修改代码。
```

### 11.2 TDD Prompt

```text
你是 XinWiki 的 TDD 工程师。
请先为【功能】编写测试，不要实现业务代码。
必须覆盖：
1. 正常路径
2. 异常路径
3. 权限路径
4. 跨租户路径
5. 审计路径
6. 并发或幂等路径
7. 回归风险
8. 缓存或引用泄露风险
```

### 11.3 实现 Prompt

```text
请根据已确认的测试实现【功能】。
要求：
1. 只修改必要文件
2. 不重建现有能力
3. 不绕过 RBAC
4. 不绕过 audit log
5. 不绕过模型用量计量 / LLM Gateway
6. 不扩大派生知识权限
7. 保持现有 API 兼容，除非任务明确要求破坏性变更
8. 实现后列出测试命令
```

### 11.4 修复失败 Prompt

```text
以下测试失败：
【失败日志】

请分析根因并修复。
要求：
1. 不删除测试
2. 不降低断言强度
3. 不绕过业务规则
4. 不扩大权限
5. 只修复根因
6. 说明为什么不会引入回归
```

### 11.5 Code Review Prompt

```text
请对本次变更做 Code Review。
重点检查：
1. 是否重造了仓库已有能力
2. 权限是否完整
3. 租户隔离是否完整
4. 派生知识是否继承来源权限
5. 审计是否完整
6. LLM 调用是否可计量
7. 缓存是否可能跨用户泄露
8. 迁移是否安全
9. 是否存在无关改动
10. 是否有足够测试
```

## 12. 每日开发流程

### 12.1 开始前

- 拉取最新代码。
- 查看当前 PR 依赖的前置 PR 是否已合入。
- 明确本次只做一个 PR 粒度任务。
- 先让 AI 做差距审计，不直接实现。

### 12.2 开发中

- 先写测试，再写实现。
- 每完成一个小改动就运行相关测试。
- 不让 AI 在未确认情况下大范围格式化文件。
- 遇到已有能力时优先复用，不重写。

### 12.3 提交前

- 运行最小相关测试。
- 运行权限泄露回归集。
- 运行 `git diff --check`。
- 检查是否有无关文件。
- 写清楚验收证据。

## 13. 分支与提交规范

### 13.1 分支命名

```text
feature/acl-propagation-a1-fields
feature/acl-propagation-a2-calculator
feature/wiki-review-workflow-b2
feature/wiki-confidence-c1
feature/llm-cost-dashboard-d1
fix/rag-post-acl-filter
```

### 13.2 Commit 粒度

推荐：

```text
test(acl): add propagation table tests
feat(acl): add derived ACL calculator
feat(wiki): persist derived ACL on crystallized pages
feat(rag): add post-retrieval ACL filtering
feat(audit): record wiki review transitions
```

避免：

```text
feat: rewrite enterprise platform
feat: add all XinWiki features
refactor: massive cleanup
```

## 14. 上线前检查清单

### 14.1 安全检查

- 所有租户资源有 tenant_id 过滤。
- 所有 KB 资源有 RBAC 校验。
- 所有 RAG 结果有应用层 ACL 二次过滤。
- 所有 Wiki、报告、缓存有来源权限继承。
- 所有引用结果有权限过滤。
- 缓存 key 包含 `user_acl_hash`。
- 无密钥硬编码。
- 审计日志不可被普通用户读取。

### 14.2 数据检查

- 迁移脚本可回滚。
- 老数据默认值安全。
- 新增索引不阻塞大表写入。
- 权限字段不会把空 ACL 解释为公开。
- 派生 ACL 重算任务可幂等执行。

### 14.3 RAG 检查

- 标准评测集通过。
- Permission Leakage Rate 为 0。
- Citation Accuracy 不低于基线。
- P95 延迟不高于基线阈值。
- 无引用时有降级策略。

### 14.4 运维检查

- 指标可查看。
- 日志可查看。
- 审计可追踪。
- 成本可统计。
- 失败任务可重试。
- 数据可备份和恢复。

## 15. 与方案 v2.0 的映射

| 方案章节 | 本计划对应 | 调整说明 |
|---|---|---|
| §4.4.4 权限传播模型 | 里程碑 A | 提到最高优先级，作为安全闭环基础 |
| §4.7 Wiki 知识引擎 | 里程碑 B、C | 在现有 Wiki 之上增量增强 |
| §4.8 Governance 知识治理 | 里程碑 C、E | 先做安全子集与可测规则 |
| §4.10 LLM Gateway | 里程碑 D | 在现有 model_usage 上补成本闭环 |
| §7 评测体系 | 里程碑 E | 补 Permission Leakage、Citation Accuracy、P95 |
| §8 分层路线图 | 本计划整体 | 从重建式 Phase 改为增量式 PR 里程碑 |
| 身份、组织、租户、RBAC | 现状核对 | 已实现，不重建 |
| Agent、MCP、Neo4j、连接器 | 现状核对 + F | 已有基础，后续只补权限与治理语义 |

## 16. 最终结论

XinWiki 当前最重要的工作不是把企业能力从零再做一遍，而是在现有 WeKnora 能力之上补齐“知识资产治理语义”。

正确顺序是：

```text
派生知识权限传播
  ↓
Wiki 生命周期与审核
  ↓
置信度、质量与保鲜
  ↓
LLM 成本与权限安全缓存
  ↓
治理与评测固化
  ↓
企业集成和品牌后置
```

一句话原则：

> 任何智能生成、Wiki 结晶、报告、缓存、Agent 工具产物，只要不能证明继承来源权限，就不能上线。
