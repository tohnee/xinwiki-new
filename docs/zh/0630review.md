继续深入 Review：围绕 7 个重点的严格结论
结论非常明确：当前代码库距离“企业级、多租户、多用户、强权限、可接企业 UUM、完整 Agent Runtime、完整三栏生成工作台、Wiki v2 级知识编译与检索、微服务化、去品牌化完成”的目标还有明显差距。
不是“不能用”，而是：可以作为底座和 PoC/内测版本继续演进，但不能按当前状态对外承诺 Enterprise Ready。
我按你的 7 个关注点逐项给出严格判断。
1. 多租户、多用户管理、企业级权限管控、UUM 集成
严格结论
不满足完整企业级权限管控要求。
当前具备：

多租户基础模型。
租户成员表。
Owner/Admin/Contributor/Viewer 角色。
JWT + X-Tenant-ID 切换。
OIDC 登录能力。
邀请注册、成员管理、审计日志。
Chunk / Wiki 的 ACL 字段和搜索后 ACL 过滤。
但它还不是完整企业 UUM / IAM / 组织权限集成方案。尤其缺少：
企业组织架构同步。
部门 / 群组 / 岗位 / 职级权限映射。
UUM 用户生命周期同步。
SCIM 或企业目录同步。
外部权限源到内部 ACL 的实时同步。
企业 UUM 权限变更后的缓存、Wiki、Graph、派生知识权限重算。
统一身份源禁用本地自建租户 / 自注册的强管控模式。
数据源侧权限继承，比如飞书 / Notion / Confluence 原权限完整映射到知识库、chunk、wiki、graph。
当前已有的证据
项目有 OIDC 流程，文档明确当前 OIDC 是：前端发起跳转，后端换 token，后端拿 OIDC 用户信息后查找或自动创建本地用户和默认租户，然后签发 XinWiki 自己的 JWT。
这说明：OIDC 主要解决“身份认证”，没有解决“企业权限同步”。

OIDC 文档也说明，登录成功后会自动创建本地账号和默认租户。

这对企业 UUM 来说是明显不足：企业往往要求用户、组织、租户、部门、角色来自统一目录，而不是每个 OIDC 新用户自动创建默认租户。

认证中间件支持从 JWT 解析用户和租户，也支持 X-Tenant-ID 目标租户切换，并会调用 IsTenantAccessible 检查能否访问目标租户。

认证通过后，它会把租户、用户、角色、system admin 等写入请求上下文。

RBAC 中间件已经提供角色门禁和“资源所有者或角色”门禁。

KnowledgeBase 模型有 TenantID 与 CreatorID，说明具备租户归属与资源创建者识别能力。

Chunk / WikiPage 也有 group ACL 概念，至少类型层面支持 allowed groups。

严重问题
1.1 当前 OIDC ≠ UUM 权限集成
当前 OIDC 只覆盖身份认证和基础用户创建，不等于企业 UUM 接入。企业 UUM 通常要求：
用户所属组织。
部门。
岗位。
用户组。
角色。
数据权限。
离职 / 禁用同步。
权限变更实时生效。
当前配置中只有 username/email mapping。
环境变量也只覆盖 username/email mapping，没有看到 group、department、role、tenant、organization 的 claim mapping。

1.2 RBAC 存在 fail-open 风险
rbacEnforcementEnabled 在 cfg == nil 时会返回 false。
这意味着配置异常或依赖注入异常可能导致 RBAC 不强制执行。企业级权限系统应 fail-closed，而不是 fail-open。

1.3 外部权限没有完整打通到知识对象
即使 OIDC 登录成功，当前仍主要靠本地 tenant_members、邀请、组织共享来管理权限。没有看到企业 UUM 组 / 部门到 AllowedGroupIDs 的自动同步与更新链路。
1.4 权限变更后的派生知识风险
搜索命中语义缓存后会二次 ACL 过滤，这是正确做法。
正常检索结果返回前也会 ACL 过滤。

但企业级要求是：权限变更后，缓存、Wiki 派生页、图谱边、已生成内容、会话上下文都要同步失效或重算。 当前代码虽然有 ACL 字段和事件类型，但我没有看到足够完整的“企业权限源变更 → 所有派生资产重算”的闭环。

这一项最终判断
子项	判断
多租户基础	有
多用户管理	有
RBAC	有，但需 fail-closed
企业 UUM 身份认证	仅 OIDC 基础接入
企业 UUM 权限集成	不完整
部门 / 组织 / 用户组权限继承	不完整
权限变更实时生效	不完整
企业级权限管控	未达标
2. Agent Runtime：Claude Messages API、Claude Agent SDK、OpenCode SDK
严格结论
Claude Messages API 有原生 provider 实现；Claude Agent SDK 和 OpenCode SDK 没有完成运行时集成，只存在规划文档或周边适配痕迹。
2.1 Claude Messages API
代码中存在 internal/models/chat/anthropic.go，明确实现 Anthropic Messages 协议。
Provider 层定义了 Anthropic provider，默认 base URL 为 https://api.anthropic.com/v1，描述为 Claude models via native Anthropic Messages API。

Chat 层也明确：Anthropic 走独立 Messages 协议实现，其余 OpenAI 兼容供应商走 OpenAI-compatible 远程实现。

Anthropic 实现中有：

anthropic-version
anthropic-beta
thinking beta
tools
streaming
content block
tool result block
usage merge
相关结构在 anthropic.go 中定义了 request、message、tool、thinking config、stream event 等。
因此，Claude Messages API 基础能力是存在的。

但要注意：这不等价于“完整支持 Claude 官方所有能力”。你需要继续验证：

extended thinking 的最新 API 形态。
interleaved thinking。
tool use / tool result 的多轮严格协议。
server tool / web search / code execution 等 Claude 特性是否支持。
Claude streaming 的所有 event 类型。
prompt caching 的 cache creation / cache read 独立统计。
多模态图片 / PDF input。
citations / document block。
rate limit / retry / idempotency。
Anthropic SDK 兼容性测试。
当前代码有 anthropicThinkingBeta、thinking config 和 tool 字段，但企业承诺“完整支持 Claude Messages API”前，必须跑官方兼容测试集。
2.2 Claude Agent SDK
当前我看到的是规划文档，而不是落地实现。
docs/zh/xinwiki-v2.0-优化设计方案与TDD计划.md 明确把“Claude Message API + Agent SDK + OpenCode SDK”列为 D2-1 计划项。

该文档规划了未来目录：

internal/agent/runtime/claude/message_api.go
internal/agent/runtime/claude/agent_sdk.go
internal/agent/runtime/opencode/sdk_adapter.go
但是当前实际代码搜索没有看到这些 runtime 目录和 SDK adapter 成品。也就是说：Claude Agent SDK 不算已集成。
2.3 OpenCode SDK
同上，OpenCode SDK 当前主要存在于规划文档中的 D2-1 任务，并非生产代码完成状态。
2.4 当前 Agent Runtime 的实际能力
当前 Agent 是自研 ReAct runtime：
Agent 记录 step、thought、tool calls、thinking steps、tokens。
Agent 状态记录完整 thinking chain、total tokens、duration。
Agent 会构造 runtime context，把当前 KB scope、session 等写入上下文。
支持 tool call、final answer、content filter 等流程。
这说明系统有自己的 Agent Engine，但不是 Claude Agent SDK 原生 runtime。
这一项最终判断
能力	判断
Claude Messages API	部分完成 / 有原生 provider
Claude tool use	有实现痕迹，需协议完整性验收
Claude extended thinking	有 beta 支持痕迹，需官方兼容测试
Claude Agent SDK	未完成
OpenCode SDK	未完成
Agent Runtime 企业级插件化	还需封装 runtime adapter 层
3. UI 三栏式、右侧生成模块、PPT/PDF/图表/报告生成能力
严格结论
UI 只有三栏式 Workspace 的原型/壳，不是完整产品级三栏交互；右侧生成模块目前更像 demo/mock，不能证明具备 PPT、PDF、图表、报告的完整生成能力。
3.1 三栏式 UI 是否存在？
存在一个 XinWikiWorkspace.vue，里面有：
左侧 sidebar。
中间 main content。
右侧 right panel。
right panel 有 generate / sources / thinking tabs。
右侧面板明确有“生成 / 来源 / 思维链”三个 tab。
生成类型包括：

内容总结
研究简报
FAQ
时间线
思维导图
演示文稿
数据图表
所以，三栏式 UI 原型是存在的。
3.2 但生成模块不是完整能力
核心问题在 handleGenerate：它不是调用后端生成 API，而是 setTimeout 后拼接一段示例 Markdown，并写死 citations。
右侧 sources tab 也是写死 “12 个知识源” 和 v-for n in 5 的假数据。

thinking tab 使用的是 sampleThinkingSteps，也是前端写死的 sample。

这意味着：

右侧生成不是真正接后端生成 pipeline。
不是基于当前 session / KB / selected sources。
没有 artifact 数据模型。
没有 PPT/PDF export 后端。
没有图表 schema / chart renderer / dataset binding。
没有报告模板系统。
没有生成物权限隔离。
没有生成历史、版本、引用溯源。
没有可下载文件产物管理。
3.3 Workspace 路由也偏原型
/platform/workspace 路由存在，挂载 Workspace.vue。
但 Workspace.vue 当前只是用 XinWikiWorkspace 包一个 wiki page viewer，currentPage 初始为 null，没有看到真实 API 加载、页面树交互、右侧生成结果持久化等完整逻辑。

3.4 主平台不是三栏 Workspace
当前主平台布局是 Menu + RouterView + 全局模态框，并非全局三栏式 Workspace。
也就是说，三栏式 Workspace 不是整个产品的主交互骨架，而是一个单独路由 / 组件原型。

这一项最终判断
子项	判断
三栏式布局组件	有
右侧生成面板 UI	有
右侧生成真实后端能力	未完成
PPT 完整生成	未完成
PDF 完整生成	未完成
图表完整生成	未完成
报告完整生成	未完成
生成物版本 / 权限 / 导出	未完成
产品级 NotebookLM 式 Workspace	未达标
4. 用户对话、问答、生成是否隔离，是否有数据污染和越权风险
严格结论
当前已经做了一些隔离，但仍存在数据污染和越权风险，尤其在全局前端状态、会话恢复、语义缓存、生成物、Agent 上下文、派生 Wiki、共享空间和 API Key 路径上。
4.1 已有隔离设计
会话恢复避免污染
聊天页面加载 session 时，会读取 last_request_state，并在覆盖全局 settings store 前先 snapshot 默认值，离开会话时恢复，注释明确是为了避免“本会话状态污染新建对话”。
这是一个好信号，说明开发者已经意识到“会话状态污染”问题。

嵌入式模式避免污染 settings store
代码中明确：嵌入式模式由宿主页面注入 agent/KB，所以跳过恢复逻辑，避免污染宿主的 settings store。
搜索缓存后 ACL 过滤
语义缓存命中后会再做 ACL filter。
检索结果正常返回前也会做 ACL filter。

Agent runtime context 记录当前检索 scope
Agent 构造 runtime context 时，会写入当前 session 和当前绑定 KB / docs scope，注释强调这是为了多轮中 scope 切换时避免复用上一轮答案。
4.2 仍然存在的风险
风险 1：前端全局 store 仍可能污染
虽然 chat 页做了一部分状态恢复，但整个系统有多个全局 store：
settings
menu
chatResources
editorResources
commandPalette
uploadConfirm
tenant
当前只能证明 chat 的一段逻辑做了防污染，不能证明所有问答、生成、workspace、embed、agent、right panel 都不会串状态。
风险 2：右侧生成模块没有隔离模型
当前右侧生成面板是 mock，不存在生成物的 tenant/user/session/resource ACL 模型。
如果未来接入真实生成，必须补充：

generated artifact 表。
tenant_id。
user_id。
session_id。
source kb/document/page refs。
ACL。
sharing policy。
export file ACL。
audit log。
否则 PPT/PDF/报告生成极容易成为越权泄漏通道。
风险 3：语义缓存保存未过滤结果
当前设计是写入未过滤结果，读取时 ACL filter。
这个设计可行，但很脆弱。任何一个读取路径漏掉 ACL filter，就会泄漏。企业级更安全的做法是：

cache key 加 tenant + permission fingerprint。
或缓存只放 chunk IDs，不放完整敏感内容。
或按用户/组/ACL version 缓存。
ACL 变更后统一 bump permission version。
风险 4：Agent 上下文污染
Agent 多轮上下文包含历史消息、工具结果、KB scope、runtime context。如果用户切换 KB、切换租户、共享空间权限变化，历史上下文中的引用和生成内容是否继续可见，需要强约束。
当前有 runtime context 的 scope snapshot，但还不能证明历史上下文会按当前权限重新裁剪。

风险 5：API Key 路径可能绕过细粒度用户隔离
API Key 认证从租户 API key 提取 tenant ID 并校验。
但 API Key 通常没有自然 user_id、group_id、role、session 隔离。如果企业内部多系统集成使用 API Key，需要 scope 化，否则就是租户级大钥匙。

这一项最终判断
子项	判断
租户级隔离	有基础
用户级会话隔离	部分有
搜索 ACL 防泄漏	部分有
生成物隔离	未完成
Agent 上下文污染防护	部分有，但不完整
语义缓存越权防护	有过滤，但设计风险仍高
API Key 细粒度隔离	不完整
企业级数据污染防控	未达标
5. Wiki 是否完整，是否对齐 LLM Wiki v2，编译/检索是否优化，问答精度是否有增益
严格结论
Wiki 模块是当前项目中完成度相对较高的一块，但仍未完全达到你给的 LLM Wiki v2 蓝图。它已经覆盖了部分关键思想：生命周期、质量评分、置信度、supersession、hybrid retrieval、graph、event-driven ingest、crystallization 工具雏形；但距离完整“自维护、协作、多 Agent、治理、输出多格式、schema 驱动”的 LLM Wiki v2 还有明显差距。
5.1 参考蓝图要点
你给的 gist《LLM Wiki v2》强调：
原始资料 / wiki / schema 三层。
不断编译知识，而不是每次 RAG 重新推导。
memory lifecycle：confidence、supersession、forgetting、consolidation tiers。
typed knowledge graph。
hybrid search：BM25 + vector + graph traversal + RRF。
event-driven automation。
quality scoring / self-healing / contradiction resolution。
multi-agent collaboration。
privacy / audit governance。
crystallization。
输出不止 Markdown，也包括表格、timeline、dependency graph、slides、structured export、brief。该 gist 明确指出 flat index.md 在 100-200 页后不适合作为主搜索，建议 BM25、向量、图遍历结合，并用 RRF 融合。 来源：GitHub Gist
5.2 XinWiki 已对齐的部分
页面类型
Wiki page 类型支持：
summary
entity
concept
index
log
synthesis
comparison
这比普通 flat wiki 更接近结构化知识库。
生命周期状态
Wiki page 状态包括：
draft
reviewing
published
deprecated
superseded
archived
并且有状态转移表。
这对齐了 LLM Wiki v2 中的 supersession、staleness、governance 方向。

质量与置信度
internal/wikiquality/scores.go 中有 confidence score、quality score、retrieval boost、freshness 等逻辑。搜索结果中也显示了测试覆盖。
这对齐了 gist 中的 confidence scoring、forgetting、retention decay 思路。

Wiki ingest pipeline 有批处理与可靠性设计
Wiki ingest 使用 asynq task payload，pending ops 表，dead letters，Redis active lock，delete tombstone，批处理上限，LLM retry。
这说明它不是一次性脚本，而是有生产 pipeline 设计。

Wiki 编译有缓存和 hash
IncrementalCompiler.CompileWiki 会 hash content，cache hit 时直接返回；变更后重新 chunk、embedding、保存 chunks。
但注意：IncrementalUpdate 当前注释明确“for now, fall back to full compilation”。

这说明真正的局部 diff 增量编译还没完成。

Wiki 检索支持 BM25 + Vector + Graph + RRF
HybridRetriever 支持 BM25、vector、graph、query rewrite、cache。
检索时会：

query rewrite。
决定 BM25 / vector / graph。
并行执行。
RRF 或 score fusion。
min score filter。
排序与 topK。
这与 LLM Wiki v2 的核心检索建议高度一致。
Wiki QA 有引用和置信度
QAEngine.Answer 会 hybrid retrieval、构造 context、调用 LLM、提取 citations、verify citations、计算 confidence、记录 thinking chain。
这对 Wiki 问答精度有潜在增益。

5.3 未完全对齐的部分
5.3.1 Wiki 编译不是真正增量
IncrementalUpdate 当前只是全量编译。
要达到 LLM Wiki v2，应实现：

section-level diff。
claim-level diff。
entity-level update。
changed chunks only embedding。
graph edge partial update。
ACL partial recompute。
old claim supersession linking。
5.3.2 Graph 是否真正参与主 RAG 还需验证
internal/wiki/retrieval.go 的 Wiki retriever 支持 graph，但主知识库搜索在 knowledgebase_search.go 中主要是 vector/keyword store group，wiki/graph-only KB 会被视作不可检索或降级路径。
Agent 工具中也有跳过 non-searchable KB 的逻辑，注释说 wiki/graph-only 可能被跳过。

这意味着：Wiki 专用检索很强，但是否已经成为主问答默认路径，需要继续核实集成点。

5.3.3 LLM Wiki v2 的 schema 驱动不完整
Gist 强调 schema / CLAUDE.md / AGENTS.md 是核心产品。当前项目有 prompt templates、wiki prompts、agent skills，但没有看到针对每个知识库的 schema document 作为一等对象：
entity schema。
relationship schema。
ingest rules。
update rules。
contradiction policy。
private/shared policy。
consolidation schedule。
lint policy。
5.3.4 自修复和治理不完整
有 lint、quality score、conflict detection、review workflow，但要达到“wiki self-healing”，还需要：
orphan page 自动修复。
broken links 自动修复。
stale claim 自动标注。
low-quality page 自动重写。
contradiction 自动提出 resolution。
human approval workflow。
reversible bulk operations。
5.3.5 多 Agent 协作不完整
Gist 强调 mesh sync、private/shared scoping、work coordination。当前 XinWiki 有多用户共享空间和 Agent，但没有看到多 Agent 对同一 Wiki 的工作锁、任务协调、冲突合并、agent identity、contribution ledger。
5.3.6 输出格式不完整
Gist 明确输出不应局限于 Markdown，而应支持 slide deck、timeline、dependency graph、structured export 等。当前右侧 UI 有 presentation/chart 类型，但只是 mock。
5.4 Wiki 问答精度是否有增益？
理论上有增益，代码上也有增益设计，但缺少评测证据。
有增益设计：

Wiki 专用 Hybrid Retriever。
RRF。
graph traversal。
query rewrite。
citation verification。
confidence scoring。
retrieval boost。
quality score。
superseded/deprecated 降权。
但缺少：
Wiki QA vs 普通 RAG 的 benchmark。
不同 KB 规模下精度曲线。
LongMemEval / RAGAS / NDCG / MRR 指标。
Graph enabled vs disabled ablation。
Wiki freshness / confidence 对答案准确率的实证提升。
这一项最终判断
子项	判断
Wiki 类型和生命周期	较完整
Wiki ingest pipeline	较强
Wiki 编译	有缓存，但真正增量不足
Wiki hybrid retrieval	设计较好
Wiki QA	有高精度设计
对齐 LLM Wiki v2	部分对齐，未完整
Wiki 问答精度增益	有设计，无充分评测证明
生产级 Wiki 自维护	未达标
6. 模块是否微服务化、是否解耦、如何配合、是否需要封装优化
严格结论
当前不是微服务化架构，而是模块化单体 + 少量外部服务。解耦程度中等，服务层/接口层比较丰富，但内部仍有较强耦合；如果目标是企业级可扩展平台，需要进一步做边界封装，而不是立即拆成大量微服务。
6.1 当前实际形态
后端是 Go 单体应用：
Gin router。
Handler。
Service。
Repository。
internal modules。
docreader 是 Python/gRPC 独立服务。
mcp-server 是独立 Python 服务。
前端独立 SPA。
DB、Redis、MinIO、Neo4j、Langfuse 外部化。
README 架构图也是 Backend 内部 Router/Handler/Service/Agent Engine/RBAC/Cache/Router/Cost Tracking 组合，而不是多个独立业务服务。
Router 构造函数注入大量 handler/service，说明它是一个大单体依赖容器。

6.2 已有解耦点
types/interfaces 定义大量 service/repository 接口。
File service 支持 local/minio/cos/tos/s3/oss 等多后端。
Retriever registry 支持 store ID 和 engine type，并有读写分离 router 抽象。
DataSource connector 有统一接口。
Model provider 有 provider registry。
MCP service、Skill service、Agent tools 是扩展点。
Langfuse manager 独立封装且 no-op 安全。
6.3 主要耦合问题
6.3.1 Router 参数过大
RouterParams 注入几十个 handler 和 service，说明边界还不够清晰。
建议按领域聚合：

AuthModule
TenantModule
KnowledgeModule
AgentModule
WikiModule
ObservabilityModule
IntegrationModule
AdminModule
6.3.2 Agent 与知识库、Wiki、MCP 耦合较重
Agent tools 直接依赖 KB、chunk、wiki、graph、model 等多个服务。长期看建议抽象为：
ToolRegistry
ToolRuntime
ToolPermissionEvaluator
ToolContext
ToolAuditSink
ToolResultStore
6.3.3 Wiki 有两套倾向
当前有 internal/wiki 独立 package，也有 internal/application/service/wiki_*。这不一定错，但需要明确：
internal/wiki 是领域核心。
application/service/wiki_* 是应用编排。
Repository 与任务系统不应泄漏到领域核心。
QAEngine 是否实际接入主问答路径需要统一。
6.3.4 Prompt / ModelRouter 内存实现不适合微服务 / 多副本
Prompt template 和 routing policy 当前有内存 map。
这会阻碍多副本和微服务化。

6.4 是否需要拆微服务？
我的建议：先做模块边界和接口治理，再拆微服务。
优先拆分候选：

DocReader 服务：已经独立，继续强化。
Worker / Task service：导入、embedding、wiki ingest、graph extraction 从 API 进程拆出。
Agent Runtime service：若要接 Claude Agent SDK / OpenCode SDK，应独立 runtime boundary。
Generation Artifact service：PPT/PDF/报告/图表生成物管理。
Auth/IAM adapter service：接企业 UUM、SCIM、LDAP、OIDC claims、权限同步。
Observability service：成本、trace、audit、evaluation 聚合。
这一项最终判断
子项	判断
当前是否微服务	不是
是否模块化	是
解耦程度	中等
是否能支撑继续演进	可以
是否需要继续封装	需要
是否应立即大拆微服务	不建议，先领域边界治理
7. WeKnora 去品牌化是否完成，是否有残留
严格结论
去品牌化没有完成，残留很多，而且不只是 README。
这项结论非常明确。

7.1 明显残留
搜索结果显示大量 WeKnora / weknora / WEKNORA / wechatopenai 残留，包括：
Homebrew Formula
Formula/xinwiki-lite.rb 仍是 class WeknoraLite，homepage 指向 Tencent/WeKnora，release 包名也是 WeKnora-lite，bin 也是 weknora-lite。
DocReader proto
docreader/proto/docreader.proto 的 go_package 仍指向 github.com/Tencent/WeKnora/internal/docreader/proto。
DocReader README / image
DocReader README 中仍使用 wechatopenai/weknora-docreader 镜像。
文档环境变量
RBAC 文档仍使用 WEKNORA_TENANT_ENABLE_RBAC、WEKNORA_AUDIT_RETENTION_DAYS 等。
Agent skills 文档仍使用 WEKNORA_SANDBOX_MODE、WEKNORA_SANDBOX_DOCKER_IMAGE，默认镜像为 wechatopenai/weknora-sandbox:latest。

Embed 文档
embed 文档仍有 /weknora-widget.js。
CLI 和 Skills
CLI 仍大量使用 weknora 命令、skills 路径、Claude skills 名称。
小程序
miniprogram 包名是 weknora-miniprogram，storage key 是 weknora_settings，页面文案也有 WeKnora。
systemd service
deploy/xinwiki-lite.service 描述仍是 WeKnora Lite，执行文件仍是 WeKnora-lite。
7.2 为什么这很严重
去品牌化残留不只是观感问题，会影响：
用户认知。
镜像名。
CLI 命令。
环境变量。
配置路径。
数据目录。
systemd service。
Homebrew Formula。
文档链接。
容器日志。
安全扫描和 SBOM。
企业采购审查。
企业客户看到 WeKnora / XinWiki 混用，会认为项目迁移不完整、工程治理不严谨。
这一项最终判断
子项	判断
README 去品牌化	部分完成
文档去品牌化	未完成
CLI 去品牌化	未完成
镜像去品牌化	未完成
环境变量去品牌化	未完成
proto/package 残留	未完成
小程序去品牌化	未完成
企业交付可接受度	不达标
最终严格结论
1. 企业权限 / UUM
不达标。
当前只有 OIDC 身份接入和本地租户 RBAC，不是完整企业 UUM 权限集成。需要补齐组织、部门、用户组、角色、外部权限同步和权限变更重算。

2. Claude / Agent Runtime
Claude Messages API 部分达标；Claude Agent SDK / OpenCode SDK 不达标。
Anthropic provider 有原生 Messages API 实现，但 Claude Agent SDK 和 OpenCode SDK 仍是规划项，不是已完成运行时能力。

3. UI 三栏与生成能力
UI 原型存在，生成能力不达标。
三栏 Workspace 和右侧生成面板存在，但右侧生成是 mock，PPT/PDF/图表/报告没有完整生成 pipeline、artifact 模型、导出和权限治理。

4. 对话 / 问答 / 生成隔离
部分达标，仍有越权和污染风险。
搜索和会话有一些隔离设计，但生成物、全局前端状态、语义缓存、Agent 上下文、API Key、派生 Wiki 仍需系统治理。

5. Wiki
Wiki 是亮点，但未完整达到 LLM Wiki v2。
Wiki 生命周期、质量评分、hybrid retrieval、RRF、QA、batch ingest 都比较强；但 schema 驱动、自修复、多 Agent 协作、真正增量编译、输出多格式、精度评测仍不足。

6. 微服务化 / 解耦
不是微服务化，是模块化单体。
解耦基础不错，但 Router/Service 依赖庞大，Prompt/ModelRouter 内存实现阻碍多副本。建议先做领域边界治理，再拆 worker、agent runtime、artifact、IAM adapter 等服务。

7. 去品牌化
明确未完成。
WeKnora 残留非常多，覆盖 Formula、文档、CLI、环境变量、镜像、小程序、systemd、proto 等，不符合企业交付标准。

企业级落地准入 Gate
如果你要把它作为企业级知识库平台推进，我建议设置下面这些“必须通过”的 Gate。
P0 Gate：不通过则不能企业交付
企业 UUM adapter：
OIDC + SCIM/LDAP/企业 API。
group/department/role claim mapping。
禁用本地自注册。
用户禁用 / 离职同步。
权限变更触发 ACL / cache / wiki / graph 重算。
RBAC fail-closed：
配置缺失直接启动失败。
生产禁止 enable_rbac=false。
Claude Runtime：
完整 Claude Messages API 官方兼容测试。
Claude Agent SDK adapter。
OpenCode SDK adapter。
Runtime abstraction：AgentRuntime / ToolRuntime / ArtifactRuntime。
右侧生成真实化：
artifact table。
artifact ACL。
PPT/PDF/chart/report generation pipeline。
export service。
source citation and lineage。
数据隔离：
所有 cache key 带 tenant + user/permission fingerprint。
所有生成物带 tenant/user/session/source refs。
Agent context 每轮按当前权限裁剪。
API Key scope 化。
Wiki v2 补齐：
真正增量编译。
schema document。
claim-level confidence。
graph search 接入主 QA。
self-healing lint。
benchmark 证明精度增益。
去品牌化：
统一 XinWiki 命名。
兼容旧 WEKNORA env 但文档主推 XINWIKI。
镜像、CLI、service、proto、包名、小程序全部清理。
Testing / Checks
✅ rg -n "Claude|Anthropic|anthropic|message API|messages API|opencode|OpenCode|agent sdk|Claude Code|claude" . -g '!frontend/node_modules/**' -g '!docreader/.venv/**' -g '!**/*.sum' | head -300
✅ rg -n "weknora|Weknora|WEKNORA|wechatopenai|WeKnora|XinWiki|xinwiki" . -g '!frontend/node_modules/**' -g '!docreader/.venv/**' -g '!**/*.sum' | head -400
✅ find frontend/src -maxdepth 3 -type f | sort | sed -n '1,260p'
✅ rg -n "Workspace|workspace|three|right|panel|PPT|ppt|pdf|chart|diagram|report|生成|右侧|三栏|canvas|artifact|Markdown|download" frontend/src internal -g '*.vue' -g '*.ts' -g '*.go' | head -300
✅ nl -ba frontend/src/components/XinWikiWorkspace.vue | sed -n '1,260p'
✅ nl -ba frontend/src/components/XinWikiWorkspace.vue | sed -n '260,560p'
✅ nl -ba frontend/src/views/workspace/Workspace.vue | sed -n '1,260p'
✅ nl -ba frontend/src/views/platform/index.vue | sed -n '1,220p'
✅ nl -ba internal/wiki/compiler.go | sed -n '1,240p'
✅ nl -ba internal/wiki/retrieval.go | sed -n '1,260p'
✅ nl -ba internal/wiki/qa.go | sed -n '1,240p'
✅ nl -ba internal/application/service/wiki_ingest.go | sed -n '1,220p'
✅ nl -ba docs/OIDC认证调用流程.md | sed -n '1,130p'
✅ rg -n "OIDC|oidc|organization|department|group|UUM|mapping|UserInfo|LastActiveTenant|member" internal/handler internal/application/service internal/types internal/config -g '*.go' | head -240
✅ 浏览并核对用户提供的 LLM Wiki v2 gist：https://gist.github.com/rohitg00/2067ab416f7bbe447c1977edaaa681e2