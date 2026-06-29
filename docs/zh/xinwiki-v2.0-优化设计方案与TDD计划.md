# XinWiki v2.0 优化设计方案与TDD开发计划

## 文档说明
本文档基于XinWiki现有代码基础，针对6项核心优化需求制定详细的架构设计、实施路径和TDD验收标准。所有开发严格遵循TDD流程，确保每个功能都有完善的测试覆盖。

---

## 一、需求总览与优先级矩阵

| ID | 需求描述 | 优先级 | 预计工期 | 依赖项 |
|----|---------|--------|---------|--------|
| D2-1 | Agent运行时升级 - Claude Message API + Agent SDK + OpenCode SDK | P0 | 2周 | 无 |
| D2-2 | Wiki系统深度优化 - 高精度问答 + 编译/检索加速 | P0 | 3周 | 无 |
| D2-3 | RBAC多租户 + 企业UUM认证集成 | P0 | 2周 | 无 |
| D2-4 | 品牌统一替换 - WeKnora → XinWiki | P0 | 已完成 | ✅ |
| D2-5 | 前端三栏式重构 - NotebookLM风格 + Apple明亮科技风 | P1 | 3周 | D2-1, D2-2 |
| D2-6 | 监控增强 - 思维链展示 + Token消耗可视化 | P1 | 1周 | D2-1 |

---

## 二、D2-1: Agent运行时升级设计方案

### 2.1 架构设计

#### 2.1.1 核心能力目标
- 完整接入Claude Message API（支持tool use、thinking、extended thinking）
- 集成Claude Agent SDK，支持原生ReAct能力增强
- 兼容OpenCode SDK，支持代码理解和生成扩展
- 统一Agent抽象层，支持多Provider切换

#### 2.1.2 模块结构
```
internal/agent/
├── engine.go              # 现有引擎，改造为兼容层
├── runtime/
│   ├── interface.go       # AgentRuntime统一接口
│   ├── claude/
│   │   ├── message_api.go     # Claude Message API封装
│   │   ├── agent_sdk.go       # Claude Agent SDK集成
│   │   └── tools.go           # Claude原生工具适配
│   ├── opencode/
│   │   ├── sdk_adapter.go     # OpenCode SDK适配器
│   │   └── code_tools.go      # 代码能力工具集
│   └── hybrid/
│       └── react_engine.go    # 增强型ReAct引擎
├── tools/                 # 扩展工具注册中心
└── thinking/
    └── chain.go           # 思维链捕获与结构化
```

#### 2.1.3 关键接口定义
```go
type AgentRuntime interface {
    // 支持流式和非流式调用
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error)
    
    // 思维链获取
    GetThinkingChain(sessionID string) ([]*ThinkingStep, error)
    
    // 工具注册
    RegisterTool(tool AgentTool) error
    
    // 能力查询
    Capabilities() RuntimeCapabilities
}

type ThinkingStep struct {
    ID        string
    Type      ThinkingType // thought/tool_call/tool_result/observation
    Content   string
    Timestamp time.Time
    Tokens    TokenUsage
    ParentID  string
}
```

### 2.2 TDD开发计划

#### 2.2.1 单元测试
| 测试用例 | 验收标准 |
|---------|---------|
| TestClaudeMessageAPI_BasicChat | 基础对话调用成功，返回结构正确 |
| TestClaudeMessageAPI_ToolUse | 工具调用流程完整，参数正确解析 |
| TestClaudeMessageAPI_Thinking | 思维链内容正确捕获和结构化 |
| TestClaudeAgentSDK_ReActLoop | ReAct循环正确执行，支持多轮工具调用 |
| TestOpenCodeSDK_CodeAnalysis | 代码理解能力正常工作 |
| TestHybridRuntime_MultiProvider | 多Provider切换正常，能力协商正确 |
| TestThinkingChain_Persistence | 思维链完整存储和查询 |

#### 2.2.2 集成测试
| 测试用例 | 验收标准 |
|---------|---------|
| TestAgent_WikiQA_Flow | Agent可以正确调用Wiki检索工具回答问题 |
| TestAgent_Codebase_QA | Agent可以正确调用代码分析工具回答代码问题 |
| TestAgent_MultiTool_Chain | 多工具链式调用正确执行，结果正确聚合 |

#### 2.2.3 验收标准
- Claude Message API所有核心特性（tool use, thinking, streaming）正常工作
- Claude Agent SDK原生能力可配置启用/禁用
- OpenCode SDK代码能力可作为工具被Agent调用
- 所有思维链步骤完整记录，可追溯
- Token使用精确统计到每个思维步骤

---

## 三、D2-2: Wiki系统深度优化设计方案

### 3.1 架构设计

#### 3.1.1 核心能力目标
参考Wiki原理：https://gist.github.com/rohitg00/2067ab416f7bbe447c1977edaaa681e2

- **高精度问答**：混合检索（BM25+向量+图谱）+ RRF融合 + 引用溯源
- **编译优化加速**：增量编译 + 预计算Embedding缓存 + 编译产物持久化
- **检索优化加速**：分层检索 + 结果缓存 + 查询改写 + 硬件加速

#### 3.1.2 模块结构
```
internal/wiki/
├── compiler/
│   ├── interface.go       # WikiCompiler接口
│   ├── incremental.go     # 增量编译器
│   ├── cache.go           # 编译产物缓存
│   └── optimizer.go       # 编译优化器
├── retrieval/
│   ├── hybrid.go          # 混合检索引擎
│   ├── bm25.go            # BM25关键词检索
│   ├── vector.go          # 向量检索（优化版）
│   ├── graph.go           # 图谱遍历检索
│   ├── rrf.go             # RRF结果融合
│   ├── query_rewriter.go  # 查询改写
│   └── cache.go           # 检索结果缓存
├── qa/
│   ├── engine.go          # 高精度问答引擎
│   ├── citation.go        # 引用溯源与验证
│   ├── grounding.go       # 事实扎根检查
│   └── confidence.go      # 答案置信度计算
├── knowledge_graph/
│   ├── builder.go         # 图谱构建器
│   ├── entities.go        # 实体提取
│   ├── relations.go       # 关系提取
│   └── store.go           # 图谱存储（Neo4j适配）
└── lifecycle/
    ├── crystallizer.go    # 知识结晶器（对话转Wiki）
    ├── superseder.go      # 知识更替检测
    └── forgetter.go       # 知识遗忘机制
```

#### 3.1.3 关键技术点
1. **混合检索RRF融合公式**：
   ```
   score(d) = Σ (1 / (k + rank_i(d)))  for each retriever i
   其中k=60为常数，平衡不同检索器的权重
   ```

2. **增量编译触发条件**：
   - 单页面更新：只重新编译该页面及其反向链接
   - 批量导入：按批次并行编译
   - 定时重编译：低峰期全量校验

3. **分层检索流程**：
   ```
   Query → 查询改写 → 粗排（BM25+向量ANN）→ 精排（Cross-Encoder）→ 图谱扩展 → RRF融合 → 结果
   ```

### 3.2 TDD开发计划

#### 3.2.1 单元测试
| 测试用例 | 验收标准 |
|---------|---------|
| TestIncrementalCompiler_SinglePageUpdate | 单页更新时仅编译受影响页面，速度提升>80% |
| TestHybridRetrieval_RRFusion | RRF融合结果NDCG@10比单一检索提升>15% |
| TestQueryRewriter_Expansion | 查询改写后召回率提升>10% |
| TestCitationTracker_Accuracy | 引用溯源准确率>99% |
| TestKnowledgeGraph_Builder | 实体/关系抽取F1>85% |
| TestRetrievalCache_HitRate | 热查询缓存命中率>70% |
| TestAnswerConfidence_Calibration | 置信度校准误差<5% |

#### 3.2.2 性能测试
| 测试场景 | 性能指标 |
|---------|---------|
| 单知识库检索（10万页面） | P95 < 200ms |
| 问答端到端（含LLM） | P95 < 3s |
| 增量编译（单页面） | P95 < 500ms |
| 并发检索（100 QPS） | 无错误，P95 < 500ms |

#### 3.2.3 验收标准
- 问答准确率（人工评估）：>90%
- 引用准确率：>99%（所有引用均可溯源到原文）
- 检索速度对比现有版本：提升>3倍
- 编译速度对比现有版本：提升>5倍（增量场景）
- 知识图谱自动构建，支持实体链接和关系查询

---

## 四、D2-3: RBAC多租户与企业UUM认证设计方案

### 4.1 架构设计

#### 4.1.1 核心能力目标
- 完整的RBAC权限模型：用户-角色-权限-资源
- 多租户数据严格隔离（Schema级+行级）
- 企业UUM（统一用户管理）认证对接：
  - SCIM 2.0协议用户/部门同步
  - SAML 2.0/OIDC SSO登录
  - 部门级权限继承
  - 权限自动同步

#### 4.1.2 模块结构
```
internal/auth/
├── rbac/
│   ├── interface.go       # RBAC服务接口
│   ├── role.go            # 角色管理
│   ├── permission.go      # 权限管理
│   ├── assignment.go      # 权限分配
│   └── evaluator.go       # 权限评估引擎
├── tenant/
│   ├── manager.go         # 租户管理
│   ├── isolation.go       # 租户隔离中间件
│   └── resolver.go        # 租户上下文解析
├── uum/
│   ├── interface.go       # UUM同步接口
│   ├── scim.go            # SCIM 2.0客户端
│   ├── saml.go            # SAML 2.0 SSO
│   ├── oidc.go            # OIDC SSO增强
│   ├── syncer.go          # 用户/部门增量同步
│   └── dept_resolver.go   # 部门权限继承
└── audit/
    └── logger.go          # 权限审计日志
```

#### 4.1.3 权限模型
```
Tenant (租户)
  └── Department (部门，树形结构)
       └── User (用户)
            └── Role (角色，可继承部门角色)
                 └── Permission (权限)
                      └── Resource (资源：知识库/Wiki/文档等)
```

#### 4.1.4 UUM同步流程
1. **全量同步**：首次接入时拉取全量用户/部门
2. **增量同步**：
   - Webhook实时推送变更
   - 定时轮询兜底（5分钟间隔）
3. **权限映射**：企业部门/角色自动映射到XinWiki角色

### 4.2 TDD开发计划

#### 4.2.1 单元测试
| 测试用例 | 验收标准 |
|---------|---------|
| TestRBAC_PermissionEvaluation | 权限评估正确，支持继承和覆盖 |
| TestTenant_Isolation | 跨租户访问100%被拦截，无越权 |
| TestUUM_SCIM_Sync | SCIM用户/部门同步正确，字段映射准确 |
| TestUUM_DeptPermissionInheritance | 部门权限正确继承到子部门和用户 |
| TestSSO_SAML_Login | SAML登录流程正确，用户自动创建 |
| TestSSO_OIDC_Login | OIDC登录流程正确，组信息正确映射 |
| TestPermissionAuditLog | 所有权限校验操作都有审计日志 |

#### 4.2.2 安全测试
| 测试场景 | 验收标准 |
|---------|---------|
| 跨租户数据访问 | 100%被拦截，返回403 |
| 垂直越权（普通用户访问管理员接口） | 100%被拦截 |
| 水平越权（访问其他用户私有资源） | 100%被拦截 |
| 部门越权（访问其他部门私有知识库） | 100%被拦截 |
| Token篡改/伪造 | 100%被识别并拒绝 |

#### 4.2.3 验收标准
- 支持标准RBAC模型：用户、角色、权限、资源、操作
- 租户数据严格隔离，所有查询自动带上tenant_id过滤
- 支持SCIM 2.0自动同步用户和部门
- 支持SAML 2.0和OIDC SSO登录
- 部门权限支持多级继承
- 所有权限相关操作都有完整审计日志
- 无越权漏洞（通过OWASP ZAP扫描）

---

## 五、D2-5: 前端三栏式重构设计方案

### 5.1 架构设计

#### 5.1.1 核心能力目标
- 从两栏式布局重构为NotebookLM风格三栏式布局
- Apple明亮科技风设计语言：毛玻璃效果、微妙阴影、圆角、流畅动画
- 右侧新增"生成"面板，支持多种内容生成：
  - 内容总结（摘要、要点、思维导图）
  - 简报生成
  - PPT大纲/内容生成
  - 图表生成（流程图、架构图、数据图）
  - 学习卡片、FAQ生成

#### 5.1.2 布局结构
```
┌─────────────────────────────────────────────────────────────┐
│ 顶部导航栏：Logo、知识库切换、搜索、用户头像                  │
├──────────┬──────────────────────────┬───────────────────────┤
│          │                          │                       │
│ 左侧边栏  │     中间主内容区          │    右侧生成面板        │
│ - 知识库  │  - 文档/Wiki浏览         │  - 生成类型选择        │
│ - 目录树  │  - 对话界面              │  - 生成进度展示        │
│ - 收藏夹  │  - 引用来源展示          │  - 生成结果预览        │
│ - 历史   │  - 思维链展示            │  - 结果操作（导出/复制）│
│          │                          │                       │
│ (280px)  │       (自适应-380px)      │      (380px，可折叠)   │
└──────────┴──────────────────────────┴───────────────────────┘
```

#### 5.1.3 设计系统（Apple明亮科技风）
- **颜色系统**：
  - 主色：#007AFF（Apple蓝）
  - 背景：#F5F5F7 → #FFFFFF 渐变
  - 卡片：#FFFFFF，毛玻璃效果 `backdrop-filter: blur(20px)`
  - 文字：#1D1D1F（标题）、#86868B（正文）
- **阴影**：`0 4px 20px rgba(0,0,0,0.08)` 柔和阴影
- **圆角**：12px（卡片）、8px（按钮）
- **动画**：所有过渡使用 `cubic-bezier(0.25, 0.1, 0.25, 1)` 缓动，时长200-300ms

#### 5.1.4 右侧生成面板功能模块
```
frontend/src/components/GenerationPanel/
├── GenerationPanel.vue      # 面板主容器
├── GenerationTypes.ts       # 生成类型定义
├── generators/
│   ├── SummaryGenerator.vue    # 总结生成
│   ├── BriefingGenerator.vue   # 简报生成
│   ├── PPTGenerator.vue        # PPT生成
│   ├── ChartGenerator.vue      # 图表生成（Mermaid/Excalidraw）
│   ├── FAQGenerator.vue        # FAQ生成
│   └── FlashcardGenerator.vue  # 学习卡片生成
└── GenerationResult.vue     # 结果展示与导出
```

### 5.2 TDD开发计划

#### 5.2.1 组件单元测试（Vitest + Vue Test Utils）
| 测试用例 | 验收标准 |
|---------|---------|
| TestThreeColumnLayout_Responsive | 三栏布局在不同屏幕尺寸下正确自适应，小屏幕自动折叠右侧面板 |
| TestGenerationPanel_TypeSwitch | 切换生成类型时表单正确切换，无内存泄漏 |
| TestSummaryGenerator_Output | 总结生成结果正确渲染，支持长文本折叠/展开 |
| TestChartGenerator_Mermaid | Mermaid图表正确渲染，支持导出为SVG/PNG |
| TestPPTGenerator_Outline | PPT大纲正确展示，支持层级调整和导出 |
| TestAppleStyle_VisualConsistency | 所有组件视觉风格一致，符合设计系统规范 |
| TestAnimations_Smooth | 面板展开/折叠、侧边栏切换动画流畅，无卡顿 |

#### 5.2.2 E2E测试（Playwright）
| 测试场景 | 验收标准 |
|---------|---------|
| 用户完整问答流程 | 左侧选择知识库 → 中间提问 → 查看回答/引用 → 右侧生成总结 |
| 生成简报并导出 | 选择文档 → 生成简报 → 导出为Markdown/Word |
| 生成图表并插入文档 | 选择内容 → 生成流程图 → 插入到Wiki页面 |
| 响应式布局切换 | 调整窗口大小，布局正确响应，无元素错位 |

#### 5.2.3 验收标准
- 三栏布局流畅，支持右侧面板折叠/展开
- Apple设计风格统一，视觉层次清晰
- 所有生成功能可用，结果展示美观
- 动画流畅，无布局抖动（CLS < 0.1）
- Lighthouse性能得分：>90（性能/可访问性/最佳实践）
- 响应式支持：1280px以上三栏，768-1280px两栏，<768px单栏+抽屉

---

## 六、D2-6: 监控增强 - 思维链展示与Token消耗设计方案

### 6.1 架构设计

#### 6.1.1 核心能力目标
- 完整展示模型思维链：思考过程、工具调用、观察结果
- Token消耗实时统计：输入/输出/缓存Token分别统计
- 成本实时计算：基于模型单价和Token使用量
- 调用链路追踪：完整记录请求处理全路径

#### 6.1.2 模块结构
```
internal/observability/
├── thinking/
│   ├── tracker.go         # 思维链追踪器
│   ├── store.go           # 思维链存储
│   └── formatter.go       # 思维链格式化（供前端展示）
├── token/
│   ├── counter.go         # Token精确计数器（支持tiktoken）
│   ├── cost_calculator.go # 成本计算器
│   └── reporter.go        # 用量报告生成
├── metrics/
│   ├── prometheus.go      # Prometheus指标暴露
│   └── dashboard.go       # 监控看板数据接口
└── tracing/
    └── otel.go            # OpenTelemetry链路追踪集成
```

前端模块：
```
frontend/src/components/Monitoring/
├── ThinkingChainViewer.vue # 思维链查看器（折叠/展开/时间轴）
├── TokenUsageBreakdown.vue # Token消耗明细（饼图/柱状图）
├── CostDashboard.vue       # 成本看板
└── CallTraceViewer.vue     # 调用链路查看器
```

#### 6.1.3 思维链展示结构
```
Thinking Chain (时间轴视图)
├── 🧠 Thought: 用户问题分析 (120 tokens, 0.8s)
├── 🔧 Tool Call: search_wiki (query: "XinWiki架构") (45 tokens, 0.1s)
├── 📋 Tool Result: 返回5个相关页面 (1,200 tokens, 0.3s)
├── 🧠 Thought: 找到相关信息，整理答案 (230 tokens, 1.2s)
├── ✅ Final Answer: 结构化回答 + 引用 (180 tokens)
└── 📊 Token Usage: 输入1595 / 输出575 / 成本 ¥0.0123
```

### 6.2 TDD开发计划

#### 6.2.1 单元测试
| 测试用例 | 验收标准 |
|---------|---------|
| TestTokenCounter_Accuracy | Token计数误差<1%（与官方tiktoken对比） |
| TestCostCalculator_Pricing | 成本计算准确，支持不同模型和折扣 |
| TestThinkingTracker_Complete | 思维链步骤100%捕获，无遗漏 |
| TestThinkingChainPersistence | 思维链正确存储和查询，支持分页 |
| TestPrometheusMetrics_Exposed | 所有核心指标正确暴露给Prometheus |

#### 6.2.2 集成测试
| 测试场景 | 验收标准 |
|---------|---------|
| Agent调用完整追踪 | 从请求进入到响应返回，全链路思维链和Token统计完整 |
| 流式调用Token统计 | 流式调用过程中Token实时统计，最终数字准确 |
| 多工具调用链追踪 | 多轮工具调用的思维链正确关联和展示 |

#### 6.2.3 验收标准
- 思维链完整记录，每个步骤都有时间、Token数、耗时
- Token计数误差<1%，输入/输出/缓存分项统计
- 成本实时计算，支持按租户/用户/模型维度统计
- Prometheus指标完整，支持Grafana看板
- 前端思维链展示清晰，支持折叠/展开和复制
- Token消耗和成本在对话界面实时显示

---

## 七、总体实施排期

### Sprint 1（第1-2周）：Agent运行时 + 监控基础
- D2-1: Agent运行时升级（Claude Message API/Agent SDK/OpenCode SDK）
- D2-6: 监控基础（Token统计、思维链追踪后端）

### Sprint 2（第3-4周）：Wiki系统优化
- D2-2: Wiki混合检索引擎（BM25+向量+图谱RRF融合）
- D2-2: 增量编译器和缓存优化

### Sprint 3（第5-6周）：RBAC + UUM认证
- D2-3: RBAC权限模型实现
- D2-3: 企业UUM SCIM/SAML/OIDC集成

### Sprint 4（第7-8周）：Wiki高精度问答 + 监控前端
- D2-2: 高精度问答引擎 + 引用溯源
- D2-6: 思维链和Token监控前端界面

### Sprint 5（第9-11周）：前端三栏式重构
- D2-5: 三栏布局框架 + Apple设计系统
- D2-5: 右侧生成面板（总结/简报/PPT/图表）

### Sprint 6（第12周）：集成测试 + 验收
- 全链路集成测试
- 性能测试和优化
- 安全渗透测试
- 文档完善和验收

---

## 八、TDD严格执行流程

每个功能模块开发严格遵循以下流程：

1. **RED阶段**：先编写失败的测试用例
   - 单元测试覆盖核心逻辑
   - 集成测试覆盖模块交互
   - 安全测试覆盖权限边界
   - 性能测试覆盖关键路径

2. **GREEN阶段**：编写最小实现让测试通过
   - 不做过度设计
   - 先让功能跑通
   - 不急于优化

3. **REFACTOR阶段**：重构代码保持可读性和性能
   - 消除重复代码
   - 优化命名和结构
   - 确保测试仍然通过

4. **验收阶段**：
   - 代码Review
   - 测试覆盖率>80%（核心模块>90%）
   - 性能指标达标
   - 安全测试通过
   - 文档更新完成

---

## 九、风险与应对措施

| 风险 | 影响 | 概率 | 应对措施 |
|-----|------|------|---------|
| Claude Agent SDK API变动 | 高 | 中 | 封装适配层，核心逻辑不直接依赖SDK |
| UUM系统对接复杂度超预期 | 中 | 高 | 先做标准协议（SCIM/SAML/OIDC），定制化部分留扩展点 |
| 前端重构工作量大 | 中 | 中 | 组件化开发，分阶段交付，先保证核心流程 |
| Wiki检索优化效果不明显 | 中 | 低 | 建立评测集，每轮优化都用数据说话，不达标不合并 |
| Token计数精度问题 | 低 | 中 | 使用官方tiktoken库，做对比校验，预留修正系数 |

---

## 十、验收总标准

所有功能开发完成后，必须满足以下总体验收标准：

1. **功能完整性**：6项需求全部实现，符合设计要求
2. **测试覆盖率**：核心模块单元测试覆盖率>90%，整体>80%
3. **性能指标**：所有性能测试达标，关键路径P95延迟符合要求
4. **安全性**：通过OWASP Top 10安全扫描，无高危/中危漏洞
5. **兼容性**：主流浏览器（Chrome/Safari/Firefox/Edge）最新两版正常工作
6. **可观测性**：所有关键操作有日志、有指标、有追踪
7. **文档完整性**：API文档、部署文档、用户文档全部更新完成
