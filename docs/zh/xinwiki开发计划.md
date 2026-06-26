XinWiki 企业级知识智能平台可执行落地计划
以下计划以你给出的设计文档为蓝本，将 XinWiki 从 WeKnora v0.6.2 渐进式重构为企业级知识智能平台，核心目标保持为：
企业知识资产可安全沉淀、可准确检索、可持续治理、可逐步智能化。
整体执行原则：
不做大爆炸重构：先增强现有系统，再模块化，再局部服务化。
权限安全先行：所有 RAG、Wiki、Agent、缓存、报告能力都必须先过权限模型。
TDD 驱动开发：每个核心能力先定义验收用例、权限用例、回归用例，再编码。
闭环优先：优先交付企业可用闭环，而不是堆叠高级智能能力。
评测内建：RAG、Wiki、Agent、治理能力必须从第一阶段开始建设评测集。
1. 总体实施路线
1.1 阶段划分
Phase 0：现状评估与技术预研，2-4 周
Phase 1：企业可用版，2-3 个月
Phase 2：知识资产版，3-4 个月
Phase 3：企业增强版，4-5 个月
Phase 4：智能平台版，4-6 个月
Phase 5：智能自治版，持续演进
1.2 总体目标拆解
阶段	核心目标	关键闭环
Phase 0	摸清 WeKnora 现状	代码、架构、数据、RAG、安全基线
Phase 1	企业安全可用	登录 → 权限 → 知识库 → RAG → 引用 → 审计
Phase 2	知识资产沉淀	问答 → Wiki 草稿 → 审核 → 发布 → RAG 增强
Phase 3	企业系统集成	组织同步 → 权限同步 → 数据源接入 → 审计报表
Phase 4	平台化智能增强	Agent Runtime → 工具权限 → 图谱 → 可观测
Phase 5	自治治理	自动巡检 → 自愈 → 合并 → 归档 → 持续优化
2. TDD 总体方法
2.1 TDD 工作流
每个模块统一采用以下流程：
1. 明确业务验收标准
2. 编写测试用例
3. 编写最小实现
4. 运行测试
5. 重构代码
6. 增加边界测试
7. 增加权限 / 审计 / 回归测试
8. 合入主干
2.2 测试分层
测试类型	目标
单元测试	校验领域逻辑、权限判断、状态流转
集成测试	校验数据库、缓存、向量检索、对象存储交互
API 测试	校验接口权限、输入输出、错误码
安全测试	校验越权访问、租户隔离、ACL 过滤
RAG 评测	校验召回率、引用准确率、幻觉率
回归测试	防止现有 WeKnora 能力退化
审计测试	确保关键操作全部落审计日志
2.3 每个功能的 Definition of Done
每个需求完成必须满足：
有测试用例
有权限校验
有审计日志
有错误处理
有指标埋点
有必要文档
不破坏原有 RAG 主链路
3. Phase 0：现状评估与技术预研
3.1 阶段目标
摸清 WeKnora v0.6.2 的真实现状，确定 XinWiki MVP 范围，避免盲目改造。
3.2 工作包拆解
WP0.1 代码结构审计
目标：
识别当前后端、前端、RAG、文档解析、存储、认证、权限、部署结构。
输出：
代码结构图
核心模块说明
技术债清单
可复用模块清单
需重构模块清单
检查重点：
API 路由结构；
用户模型；
知识库模型；
文档上传流程；
Chunk 切分流程；
Embedding 流程；
检索流程；
LLM 调用流程；
前端页面结构；
数据库 schema；
部署脚本。
WP0.2 数据库结构分析
目标：
确认现有表结构是否支持 tenant_id、权限、审计、Wiki、LLM 调用记录扩展。
输出：
数据库 ER 图
核心表字段分析
迁移风险清单
新增表建议
索引优化建议
WP0.3 RAG 链路分析
目标：
明确当前 RAG 从提问到回答的完整链路。
分析对象：
query
retrieval
rerank
context build
prompt
LLM call
answer
citation
history
输出：
RAG 链路图
召回策略说明
上下文拼装说明
引用生成说明
权限风险点
性能瓶颈
WP0.4 权限与安全差距分析
目标：
确认现有系统是否存在租户隔离、知识库权限、文档权限、检索越权风险。
重点问题：
是否有统一用户身份？
是否支持租户？
是否支持知识库角色？
检索是否带权限过滤？
引用是否可能泄露无权限文档？
缓存是否跨用户复用？
文档下载是否校验权限？
输出：
权限差距分析报告
高危越权路径清单
Phase 1 权限改造范围
WP0.5 性能与容量基线
目标：
建立 Phase 1 前的基线，便于后续验证改造是否退化。
指标：
文档上传耗时
解析耗时
Embedding 耗时
检索 P50 / P95
问答 P50 / P95
并发能力
数据库慢查询
向量检索耗时
WP0.6 MVP 范围冻结
Phase 1 MVP 必须严格限制为：
UUM OIDC 登录
用户同步
租户管理
Tenant / KB RBAC
文档上传解析
权限安全 RAG
引用溯源
基础审计
LLM Gateway 基础计量
管理员控制台
明确不进入 Phase 1：
多 Agent
Neo4j
复杂 Wiki 生命周期
高级治理
企业数据源全量接入
完整微服务
复杂多模态
4. Phase 1：企业可用版
4.1 阶段目标
让 XinWiki 可以在企业内部试点安全使用。
核心闭环：

UUM 登录
  ↓
用户进入租户
  ↓
查看有权限知识库
  ↓
上传文档
  ↓
文档解析与索引
  ↓
用户提问
  ↓
RAG 按权限检索
  ↓
生成带引用答案
  ↓
记录审计与 LLM 成本
4.2 Phase 1 里程碑
里程碑	周期	目标
M1	第 1-2 周	基础数据模型与迁移
M2	第 3-4 周	UUM OIDC 登录与用户同步
M3	第 5-6 周	租户与 RBAC
M4	第 7-8 周	权限安全知识库与文档
M5	第 9-10 周	权限安全 RAG
M6	第 11-12 周	审计、LLM Gateway、试点验收
5. Phase 1 详细执行计划
5.1 M1：基础数据模型与迁移
目标
为企业级能力打基础，补齐：
tenant_id
user mapping
role binding
audit log
llm call log
security_level
主要任务
5.1.1 设计核心表
新增或调整：
users
external_user_mapping
tenants
tenant_members
role_bindings
knowledge_bases
documents
chunks
audit_logs
llm_call_logs
5.1.2 核心字段统一
核心业务表增加：
tenant_id
created_at
updated_at
created_by
updated_by
status
security_level
5.1.3 数据迁移策略
迁移原则：
现有数据默认迁入 default tenant
现有管理员成为 default tenant owner
现有知识库归属 default tenant
现有文档 security_level 默认为 L1
TDD 用例
单元测试
创建租户时必须生成唯一 tenant_id
创建知识库时必须绑定 tenant_id
创建文档时必须绑定 tenant_id 和 kb_id
security_level 缺省值必须正确
迁移测试
旧知识库迁移后必须归属 default tenant
旧文档迁移后必须保留原始内容和索引关联
旧用户迁移后必须可登录或被映射
重复执行迁移脚本必须幂等
数据约束测试
knowledge_bases.tenant_id 不允许为空
documents.tenant_id 不允许为空
chunks.tenant_id 不允许为空
role_bindings 必须有 resource_type 和 resource_id
验收标准
核心表支持 tenant_id
旧数据可迁移
迁移可回滚
迁移后原 RAG 链路不破坏
5.2 M2：UUM OIDC 登录与用户同步
目标
接入企业统一身份认证，避免 XinWiki 自建孤立账号体系。
主要任务
5.2.1 OIDC 登录
实现：
OIDC discovery
Authorization Code Flow
ID Token 校验
Access Token 校验
Refresh Token 处理
Logout 回调
5.2.2 用户自动创建
登录后根据 UUM 身份创建或更新本地用户：
external_system = UUM
external_user_id = UUM user id
display_name
email
mobile
status
5.2.3 用户禁用同步
当 UUM 用户禁用：
禁止登录
撤销会话
保留审计记录
5.2.4 登录时单用户校准
每次登录时同步：
用户基础信息
用户所属组织
用户所属组
用户状态
TDD 用例
OIDC 测试
有效 ID Token 可以登录
过期 ID Token 拒绝登录
issuer 不匹配拒绝登录
audience 不匹配拒绝登录
签名非法拒绝登录
缺少 external_user_id 拒绝登录
用户映射测试
首次登录自动创建用户
再次登录更新用户资料
同一个 external_user_id 不重复创建用户
邮箱变化不影响用户主身份
手机号变化不影响用户主身份
禁用用户不能登录
安全测试
伪造 token 不能登录
跨 issuer token 不能登录
未绑定租户用户只能进入受限状态
验收标准
用户可以通过 UUM 登录
用户唯一身份来自 external_user_id
本地用户与 UUM 用户可稳定映射
禁用用户不能继续访问
登录事件有审计
5.3 M3：租户与 RBAC
目标
支持企业级租户隔离和角色权限。
主要对象
Tenant
TenantMember
KnowledgeBase
RoleBinding
PermissionPolicy
角色体系
Owner
Admin
Contributor
Viewer
权限粒度
Phase 1 实现：
Tenant Role
KnowledgeBase Role
后续预留：
Document ACL
Wiki Page ACL
Agent Tool Policy
权限矩阵
操作	Owner	Admin	Contributor	Viewer
管理租户	是	部分	否	否
管理成员	是	是	否	否
创建知识库	是	是	可配置	否
上传文档	是	是	是	否
删除文档	是	是	自己上传可配置	否
提问	是	是	是	是
查看引用	是	是	是	是
查看审计	是	是	否	否
TDD 用例
角色判断测试
Owner 拥有租户所有权限
Admin 可以管理知识库
Contributor 可以上传文档
Viewer 只能查询和阅读
未授权用户不能访问租户
知识库权限测试
租户 Viewer 不自动拥有所有知识库权限
KB Viewer 可以查询该知识库
KB Contributor 可以上传该知识库文档
无 KB 权限不能检索该知识库
越权测试
用户 A 不能访问用户 B 所属租户
租户 A 用户不能读取租户 B 知识库
用户无 KB 权限时不能通过 API 获取知识库详情
用户无 KB 权限时不能通过 RAG 问出内容
验收标准
用户只能看到有权限租户
用户只能看到有权限知识库
API 层和 Service 层均有权限校验
权限变更立即生效或在可控延迟内生效
权限事件有审计
5.4 M4：权限安全知识库与文档
目标
文档从上传、解析、切分、索引开始即绑定权限元数据。
文档处理链路
上传文档
  ↓
校验租户权限
  ↓
校验知识库权限
  ↓
保存原文
  ↓
创建 Document
  ↓
解析
  ↓
切分 Chunk
  ↓
绑定 ACL 元数据
  ↓
Embedding
  ↓
索引
  ↓
审计
Chunk 元数据
每个 Chunk 至少包含：
{
  "tenant_id": "tenant_001",
  "kb_id": "kb_001",
  "document_id": "doc_001",
  "chunk_id": "chunk_001",
  "security_level": "L1",
  "allowed_user_ids": [],
  "allowed_group_ids": [],
  "version": 1
}
TDD 用例
上传测试
Contributor 可以上传文档
Viewer 不能上传文档
无 KB 权限不能上传文档
跨租户上传被拒绝
解析测试
上传成功后创建 ParserJob
解析失败时 Document 状态为 failed
解析成功后生成 Chunk
Chunk 必须继承 document 的 tenant_id 和 kb_id
权限元数据测试
Chunk 必须包含 tenant_id
Chunk 必须包含 kb_id
Chunk 必须包含 document_id
Chunk 必须包含 security_level
Chunk ACL 不得为空对象造成默认公开
删除与状态测试
删除文档后不再参与检索
删除文档后相关 Chunk 标记为 deleted
删除文档事件必须写审计
验收标准
文档入库全链路带 tenant_id
Chunk 索引带权限元数据
无权限用户无法访问文档详情
无权限用户无法下载原文
文档操作有审计
5.5 M5：权限安全 RAG
目标
让 RAG 检索、上下文拼装、回答引用全链路不越权。
Phase 1 RAG 链路
用户提问
  ↓
解析用户身份
  ↓
解析 tenant_id
  ↓
解析可访问 kb_ids
  ↓
向量检索 metadata filter
  ↓
应用层 ACL 二次过滤
  ↓
上下文拼装
  ↓
LLM 回答
  ↓
引用过滤
  ↓
审计与计量
权限过滤要求
必须实现两层过滤：
检索前过滤：
tenant_id + kb_id + security_level + ACL metadata

检索后过滤：
应用层重新检查 chunk/document 权限
TDD 用例
检索前过滤测试
检索请求必须带 tenant_id filter
检索请求必须带 kb_ids filter
无权限 kb_id 不得进入 filter
跨租户 chunk 不得被召回
检索后过滤测试
召回结果必须逐条检查 ACL
无权限 chunk 必须被剔除
剔除后上下文为空时应返回无可用资料
引用安全测试
答案引用只能包含有权限文档
无权限文档标题不能出现在引用中
无权限文档片段不能出现在上下文中
无权限文档 URL 不能返回
缓存安全测试
不同用户 ACL 不得复用同一 RAG 答案缓存
不同 tenant 不得复用缓存
不同 kb 权限不得复用缓存
RAG 效果测试
标准问题 Recall@5 达标
答案必须包含引用
引用文档必须真实存在
引用内容必须支持答案
验收标准
权限泄露率为 0
回答必须有引用
无权限文档不能被问出来
RAG 引用准确率 ≥ 80%
P95 检索延迟 ≤ 500ms
5.6 M6：审计与 LLM Gateway 基础版
目标
建立企业可用的审计和成本控制基础。
5.6.1 审计日志
必审事件
登录
登出
Token 交换
创建租户
修改成员
授权
撤权
创建知识库
上传文档
删除文档
提问
检索
回答
查看引用
LLM 调用
审计字段
actor_user_id
tenant_id
action
resource_type
resource_id
request_id
ip
user_agent
result
reason
created_at
TDD 用例
登录成功写审计
登录失败写审计
授权写审计
撤权写审计
文档上传写审计
RAG 提问写审计
越权访问写审计
审计日志不可被普通用户读取
5.6.2 LLM Gateway 基础版
Phase 1 能力
统一模型调用入口
Token 计量
调用日志
错误记录
基础限流
Prompt 模板版本
成本估算
调用日志字段
tenant_id
user_id
model_id
provider
prompt_tokens
completion_tokens
total_tokens
latency_ms
cost_estimated
prompt_template_version
request_id
status
created_at
TDD 用例
所有 LLM 调用必须经过 Gateway
直接绕过 Gateway 的调用应在代码扫描中禁止
成功调用记录 token
失败调用记录错误
超限用户被限流
超限租户被限流
验收标准
核心操作审计覆盖 100%
LLM 调用有成本记录
管理员可查看租户级调用统计
RAG 请求可关联审计与 LLM 调用日志
6. Phase 2：知识资产版
6.1 阶段目标
从“能问答”升级为“能沉淀知识资产”。
核心闭环：

高质量问答
  ↓
生成 Wiki 草稿
  ↓
继承来源权限
  ↓
专家审核
  ↓
发布 Wiki
  ↓
进入检索
  ↓
增强 RAG
  ↓
治理评分
6.2 工作包拆解
WP2.1 Wiki 基础模型
核心表：
wiki_pages
wiki_page_versions
wiki_sources
wiki_confidence
wiki_supersession
Wiki 页面字段：
tenant_id
title
content
page_type
status
security_level
owner_id
confidence_score
quality_score
created_by
updated_by
published_at
TDD 用例：
创建 Wiki 草稿必须绑定 tenant_id
Wiki 页面必须有状态
Wiki 发布前必须有来源
Wiki 页面版本必须可追溯
无权限用户不能查看 Wiki
WP2.2 Wiki 来源追溯
目标：
每个 Wiki 页面必须知道来源于哪些文档、Chunk、问答或会话。
来源类型：
document
chunk
qa_session
manual
external_source
TDD 用例：
Wiki 页面可绑定多个来源
来源删除后 Wiki 标记需复核
来源权限变化后 Wiki ACL 重新计算
引用覆盖率可统计
WP2.3 派生知识权限传播
核心规则：
security_level = max(source.security_level)
allowed_users = intersection(source.allowed_users)
allowed_groups = intersection(source.allowed_groups)
TDD 用例：
单来源 Wiki 继承来源权限
多来源 Wiki 默认取权限交集
高密级来源导致 Wiki 高密级
来源权限变更后 Wiki 权限重新计算
无权限用户不能通过 Wiki 获取派生内容
WP2.4 问答结晶机制 MVP
流程：
会话结束
  ↓
质量评估
  ↓
满足条件生成 Wiki 草稿
  ↓
绑定来源
  ↓
继承权限
  ↓
进入审核队列
质量评估条件：
答案有引用
用户点赞
多次相似问题
专家标记
低幻觉风险
TDD 用例：
无引用答案不能自动结晶
低质量答案不能结晶
结晶草稿必须绑定来源
结晶草稿不能直接发布
结晶草稿必须继承权限
WP2.5 Wiki 与 RAG 双引擎融合
检索流程升级：
query
  ↓
vector search + BM25
  ↓
RRF fusion
  ↓
Wiki Boost
  ↓
Rerank
  ↓
ACL filter
  ↓
answer
TDD 用例：
Published Wiki 可参与检索
Draft Wiki 不参与普通检索
Deprecated Wiki 降权
Archived Wiki 默认不检索
Wiki 引用必须返回来源
Wiki 权限过滤必须生效
WP2.6 知识治理基础版
能力：
质量评分
文档 hash 去重
标题相似去重
过期提醒
基础矛盾检测
知识健康度看板
TDD 用例：
重复文档上传时可识别
过期文档产生提醒
低质量 Wiki 进入治理队列
同一实体属性冲突可识别
治理操作写审计
6.3 Phase 2 验收标准
Wiki 结晶审核通过率 ≥ 60%
Wiki 页面引用覆盖率 ≥ 90%
重复文档识别准确率 ≥ 85%
混合检索 Recall@10 ≥ 85%
派生知识权限泄露为 0
知识质量看板可用
7. Phase 3：企业增强版
7.1 阶段目标
支持企业大规模接入、权限同步、数据源集成和协作审核。
7.2 工作包拆解
WP3.1 SAML / LDAP / AD
能力：
SAML 登录
LDAP 用户同步
AD 组同步
多认证源路由
TDD 用例：
SAML assertion 合法可登录
签名非法拒绝
LDAP 用户禁用后不能登录
AD 组变更后权限更新
WP3.2 Workspace 深度权限同步
能力：
Workspace Space → Tenant
Workspace Role → XinWiki Role
Workspace Group → XinWiki Group
TDD 用例：
Workspace Owner 映射为 Tenant Owner
Workspace 普通成员映射为 Viewer
Workspace 移除成员后 XinWiki 权限撤销
权限同步失败可重试
WP3.3 企业数据源连接器
优先级：
企业网盘
Confluence / Wiki
Git 仓库文档
工单系统
IM 知识沉淀
API 文档系统
TDD 用例：
同步任务可增量执行
同步失败可重试
删除源文档后本地文档标记失效
同步文档必须绑定来源权限
WP3.4 Wiki 协作审核
能力：
审核流配置
多人审核
评论
退回修改
发布审批
TDD 用例：
Reviewer 可以审核
Viewer 不能审核
审核拒绝后不能发布
发布后生成版本
审核操作写审计
WP3.5 报告生成基础版
能力：
基于有权限知识生成报告
报告引用来源
报告权限继承
报告导出审计
TDD 用例：
报告不能使用无权限文档
报告必须有引用
报告导出写审计
报告分享不得扩大权限
7.3 Phase 3 验收标准
用户规模支持 1000+
并发支持 100-300
组织同步延迟 ≤ 5 分钟
数据源同步成功率 ≥ 95%
审计日志完整性 100%
Wiki 审核流程可配置
企业门户嵌入可用
8. Phase 4：智能平台版
8.1 阶段目标
平台化、智能化、云原生化。
8.2 工作包拆解
WP4.1 Agent Runtime
接口：
type AgentRuntime interface {
    Name() string
    Execute(ctx context.Context, req *AgentRequest) (*AgentResponse, error)
    StreamExecute(ctx context.Context, req *AgentRequest) (<-chan *AgentEvent, error)
    GetTools(ctx context.Context) ([]ToolInfo, error)
    HealthCheck(ctx context.Context) error
}
TDD 用例：
Agent 执行必须带用户权限上下文
Agent 工具调用前必须 Policy Check
高风险工具必须人工确认
Agent 每一步必须写审计
WP4.2 Agent 工具权限中心
高风险工具：
删除文档
发布 Wiki
修改权限
外发报告
执行代码
调用生产系统 API
TDD 用例：
无权限工具调用被拒绝
高危工具调用进入确认流程
确认超时不执行
工具执行失败写审计
WP4.3 图谱增强
Phase 4 引入 Neo4j 或图服务：
实体
关系
系统依赖
负责人
接口
流程
事件
TDD 用例：
图谱查询必须带 tenant_id
图谱结果必须经过 ACL 过滤
图谱关系来源可追溯
WP4.4 可观测体系
能力：
Metrics
Logs
Tracing
Audit
Cost Dashboard
RAG Eval Dashboard
TDD / 验收用例：
核心 API 有指标
RAG 请求有 trace_id
LLM 调用可关联 trace_id
错误率可统计
成本可按 tenant 聚合
WP4.5 局部微服务拆分
优先拆分：
llm-gateway-service
document-parser-service
search-service
wiki-service
agent-runtime-service
拆分原则：
先抽接口
再拆进程
最后拆数据库
8.3 Phase 4 验收标准
用户规模支持 3000-5000
并发支持 300-500
Agent 工具越权为 0
服务可用性达到 99.9%
LLM 成本可视化 100%
核心链路追踪覆盖 90%
9. Phase 5：智能自治版
9.1 阶段目标
让知识平台具备自我治理、自我优化、自我演进能力。
9.2 核心能力
自动矛盾裁决
自动知识自愈
自动合并与归档
多 Agent 知识维护
智能知识巡检
知识健康度自动优化
行业 Schema 模板市场
Agent 市场
完整微服务架构
多租户 Schema / 物理隔离
9.3 自治治理闭环
知识巡检
  ↓
发现问题
  ↓
生成治理建议
  ↓
低风险自动修复
  ↓
高风险人工审批
  ↓
发布变更
  ↓
评估效果
10. 核心工程任务优先级
10.1 P0：必须优先完成
UUM OIDC 登录
用户与组织同步
Tenant / KB RBAC
RAG 检索权限过滤
审计日志
PostgreSQL 稳定化
文档解析稳定化
LLM Gateway 基础计量
RAG 评测集
派生知识权限模型
10.2 P1：重点建设
Wiki 增强
结晶机制
知识质量评分
去重合并
BM25 + 向量混合检索
Workspace 权限同步
企业门户集成
报告生成基础版
Agent Runtime 抽象
10.3 P2：后置建设
多 Agent 协作
Agent Flow 可视化
Agent 市场
Neo4j 图谱
复杂多模态
电路图识别
自动自愈
完整微服务
物理租户隔离
11. 推荐开发 Backlog
Epic 1：身份认证与用户同步
User Stories：
作为企业用户，我希望通过 UUM 登录 XinWiki。
作为管理员，我希望用户首次登录时自动创建账号。
作为安全管理员，我希望禁用用户无法继续访问系统。
作为审计管理员，我希望查看用户登录记录。
Epic 2：租户与权限
User Stories：
作为租户 Owner，我希望管理租户成员。
作为管理员，我希望给用户分配知识库角色。
作为普通用户，我只能看到自己有权限的知识库。
作为安全管理员，我希望所有越权访问被记录。
Epic 3：知识库与文档
User Stories：
作为 Contributor，我希望上传文档到知识库。
作为 Viewer，我不能上传或删除文档。
作为管理员，我希望查看文档解析状态。
作为用户，我希望文档上传后可以被检索。
Epic 4：权限安全 RAG
User Stories：
作为用户，我希望只基于有权限知识提问。
作为用户，我希望答案必须带引用。
作为安全管理员，我希望无权限文档不会出现在答案和引用中。
作为平台管理员，我希望查看 RAG 请求日志。
Epic 5：审计与成本
User Stories：
作为管理员，我希望查看核心操作审计日志。
作为平台管理员，我希望查看 LLM Token 消耗。
作为租户 Owner，我希望查看本租户成本趋势。
作为安全管理员，我希望追踪一次问答使用了哪些来源。
Epic 6：Wiki 知识沉淀
User Stories：
作为用户，我希望将高质量问答转为 Wiki 草稿。
作为专家，我希望审核 Wiki 草稿。
作为用户，我希望查看 Wiki 的来源引用。
作为管理员，我希望 Wiki 页面继承来源权限。
Epic 7：知识治理
User Stories：
作为管理员，我希望识别重复文档。
作为知识负责人，我希望收到过期知识提醒。
作为专家，我希望查看冲突知识候选。
作为运营人员，我希望查看知识健康度看板。
Epic 8：企业集成
User Stories：
作为企业管理员，我希望同步 Workspace 成员和角色。
作为用户，我希望从企业门户进入 XinWiki。
作为管理员，我希望接入企业数据源。
作为审计人员，我希望导出审计报表。
Epic 9：Agent 与工具
User Stories：
作为用户，我希望 Agent 帮我完成知识检索和报告生成。
作为安全管理员，我希望 Agent 调用工具前经过权限检查。
作为管理员，我希望高风险工具需要人工确认。
作为审计人员，我希望查看 Agent 每一步执行记录。
12. 关键测试集设计
12.1 权限泄露测试集
必须覆盖：
跨租户访问
跨知识库访问
无权限文档检索
无权限引用泄露
缓存跨用户复用
Wiki 派生权限扩大
报告分享越权
Agent 工具越权
目标：
Permission Leakage Rate = 0
12.2 RAG 评测集
每个问题包含：
question
expected_answer
expected_sources
tenant_id
kb_id
allowed_user
denied_user
difficulty
指标：
Recall@K
MRR
NDCG
Answer Correctness
Faithfulness
Citation Accuracy
Hallucination Rate
Latency P95
12.3 Wiki 评测集
指标：
页面质量分
结晶转化率
审核通过率
重复率
过期率
矛盾率
引用覆盖率
使用率
12.4 Agent 评测集
指标：
Task Success Rate
Tool Success Rate
Avg Iterations
Human Intervention Rate
Cost Per Task
Policy Deny Rate
Unsafe Action Blocked
13. 架构演进建议
13.1 Phase 1：增强单体
XinWiki Monolith
├── Auth Module
├── UserOrg Module
├── Tenant/RBAC Module
├── Knowledge Module
├── Chat/RAG Module
├── Wiki Module
├── Audit Module
├── LLM Gateway Module
└── Admin Console
重点：
不拆服务
先清晰模块边界
共享数据库
强化权限与审计
13.2 Phase 2-3：模块化单体
Identity Domain
Knowledge Domain
Retrieval Domain
Wiki Domain
Governance Domain
Agent Domain
Integration Domain
LLM Domain
重点：
按领域组织代码
模块间通过接口交互
引入进程内事件
为未来微服务做准备
13.3 Phase 4：局部服务化
优先拆分：
LLM Gateway
Document Parser
Search Service
Wiki Service
Agent Runtime Service
原因：
计算密集
横向扩展收益高
业务耦合相对低
可独立部署
13.4 Phase 5：完整微服务化
auth-service
user-org-service
tenant-service
knowledge-service
retrieval-service
wiki-service
agent-service
llm-gateway-service
governance-service
audit-service
integration-service
14. 团队执行建议
14.1 Phase 1 团队配置
项目经理：1
架构师：1
后端工程师：3-4
前端工程师：2
测试工程师：1-2
DevOps：1
产品 / 解决方案：1
总计：
10-12 人
14.2 推荐小组划分
小组 A：身份与权限
负责：
UUM OIDC
用户同步
租户
RBAC
权限测试集
小组 B：知识与 RAG
负责：
知识库
文档处理
Chunk ACL
向量检索
RAG 权限过滤
引用溯源
小组 C：审计与 LLM Gateway
负责：
审计日志
LLM Gateway
Token 计量
成本统计
限流
小组 D：前端与管理台
负责：
登录页
租户管理
知识库管理
权限管理
审计页面
成本看板
小组 E：测试与质量
负责：
TDD 规范
自动化测试
权限泄露测试
RAG 评测集
回归测试
验收测试
15. 首批 12 周执行节奏
第 1-2 周
代码审计
数据库审计
RAG 链路审计
MVP 冻结
数据模型设计
迁移脚本设计
TDD 框架补齐
第 3-4 周
OIDC 登录
用户映射
用户同步
登录审计
认证 API 测试
第 5-6 周
Tenant 模型
Tenant Member
Role Binding
KB RBAC
权限中间件
权限单元测试
第 7-8 周
知识库改造
文档上传权限
Chunk ACL 元数据
文档解析状态
文档审计
第 9-10 周
RAG metadata filter
应用层 ACL 二次过滤
引用过滤
RAG 权限测试
RAG 效果评测
第 11-12 周
LLM Gateway
Token 计量
限流
成本日志
审计查询
管理台完善
试点验收
16. 关键风险控制
16.1 权限风险
控制措施：
所有 API 必须有权限中间件
所有 Service 必须有权限校验
所有检索必须有前置 metadata filter
所有召回结果必须二次 ACL ��验
所有引用必须过滤
所有缓存 key 必须包含 user_acl_hash
16.2 RAG 效果风险
控制措施：
建立标准问答集
建立来源标注集
每次检索策略变更跑评测
低于阈值不能发布
保留 fallback 检索策略
16.3 重构风险
控制措施：
先包裹旧能力，再替换旧能力
保留兼容 API
分模块开关发布
提供迁移回滚方案
所有迁移脚本幂等
16.4 LLM 成本风险
控制措施：
所有调用经过 LLM Gateway
租户级限额
用户级限额
Prompt 模板版本管理
响应缓存
成本看板
异常调用告警
17. 最小可交付 MVP
Phase 1 MVP 交付后，应至少满足：
用户可以用 UUM 登录
用户只能看到有权限租户
用户只能看到有权限知识库
Contributor 可以上传文档
Viewer 只能阅读和提问
文档上传后可以被解析和索引
RAG 只能检索有权限内容
回答必须带引用
无权限文档不能通过答案、引用、标题、URL 泄露
核心操作有审计
LLM 调用有 token 与成本记录
管理员可以查看用户、权限、知识库、审计、成本
18. 最终建议
XinWiki 的开发顺序应严格遵循：
身份认证
  ↓
租户权限
  ↓
知识库权限
  ↓
文档 ACL
  ↓
权限安全 RAG
  ↓
审计与成本
  ↓
Wiki 沉淀
  ↓
知识治理
  ↓
企业集成
  ↓
Agent 智能增强
  ↓
自治治理
不要先做：
Agent Flow
Neo4j
复杂多模态
完整微服务
自动知识自愈
Agent 市场
最关键的第一性原则是：
任何智能能力，如果不能证明不越权，就不能上线。