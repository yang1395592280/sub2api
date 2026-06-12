# OpenAI 账号健康调度设计

修改时间：2026-06-12 19:21 CST

## 1. 结论

第一版新增 **OpenAI 账号健康调度** 功能，采用“扩展现有 OpenAI 高级调度器”的方案。

设计重点：

- 调度粒度为 `Account`，不是 `Channel`。
- 第一版默认只覆盖 OpenAI 平台。
- 按用户分组、OpenAI 平台、模型能力分别生成主力/备用池。
- 保留人工 `accounts.priority`，不做自动改写。
- 新增运行时健康分、分层状态和管理页面。
- 失败、限流、超时、慢速会降低动态优先级。
- 恢复后先进入观察层，再逐步回到备用/主力。
- 提供独立管理页解释调度决策和支持有限人工干预。

## 2. 背景与现状

当前仓库已经具备以下基础能力：

- `accounts` 表已有 `priority`、`schedulable`、`rate_limit_reset_at`、`overload_until`、`temp_unschedulable_until` 等调度相关字段。
- OpenAI 请求链路已经通过 `OpenAIGatewayService.SelectAccountWithSchedulerForCapability` 选择账号。
- `OpenAIAccountScheduler` 已经支持 sticky session、负载感知、failover、并发槽位和调度结果上报。
- 现有调度器已经记录运行时 EWMA 指标，包括错误率和 TTFT。
- 前端已经有管理后台、账号页、渠道监控页和运维监控页，可以复用现有页面结构与组件风格。

因此第一版不新建独立调度链路，而是在现有 OpenAI 高级调度器上增加健康评分、分层选择和管理可视化。

## 3. 目标

业务目标：

- 速度快且稳定的 OpenAI 账号自动成为主力。
- 速度慢但可用的账号作为备用。
- 异常账号自动降级或冷却。
- 恢复后的账号自动回到观察、备用、主力。
- 管理员能看到每个账号为什么被选中、跳过或降级。

工程目标：

- 不污染人工配置的 `accounts.priority`。
- 不破坏现有 sticky session、failover、并发控制和 scheduler snapshot。
- 支持一键关闭新策略并回退到现有高级调度行为。
- 热路径尽量只读内存或 Redis 快照，避免高频写库。

## 4. 非目标

第一版不做以下内容：

- 不覆盖 Gemini、Anthropic、Antigravity 等其他平台。
- 不新增完整独立调度服务替换现有 OpenAI 调度器。
- 不把每次运行时指标高频写入数据库。
- 不做复杂机器学习预测。
- 不默认支持跨平台混合调度。
- 不自动修改 `accounts.priority`。

## 5. 方案选择

### 5.1 方案 A：动态改写账号 priority

优点：

- 实现路径短。
- 能直接影响现有调度排序。

缺点：

- `accounts.priority` 是人工配置字段，自动改写会污染管理员意图。
- 高频修改数据库，热路径风险高。
- 页面难解释当前排序到底来自人工还是自动。

结论：不采用。

### 5.2 方案 B：扩展现有 OpenAI 高级调度器

优点：

- 复用现有 `OpenAIAccountScheduler`、failover、sticky session、并发控制和快照机制。
- 人工优先级仍作为基础权重，动态健康只影响运行时选择。
- 容易增加管理页解释调度状态。
- 可通过独立开关快速回退。

缺点：

- 需要扩展现有调度器模型和管理接口。
- 需要设计健康分和分层规则，避免误判。

结论：采用。

### 5.3 方案 C：新建独立调度服务完全接管选择

优点：

- 边界清晰，后续可做复杂策略。

缺点：

- 会绕开现有成熟链路，改动面大。
- 容易破坏现有 sticky、failover、并发槽和快照逻辑。
- 第一版上线风险高。

结论：不采用。

## 6. 核心模型

建议新增运行时调度快照模型，不直接改写账号主表中的人工优先级。

核心字段：

- `account_id`：账号 ID。
- `group_id`：所属分组 ID。未分组账号可用 `0` 或空值表达。
- `platform`：第一版固定为 `openai`。
- `model_scope`：模型或能力范围，例如请求模型、映射后模型、endpoint capability。
- `manual_priority`：来自 `accounts.priority`。
- `dynamic_health_score`：运行时健康分，0 到 100。
- `scheduler_tier`：运行时分层。
- `degrade_reason`：降级原因。
- `cooldown_until`：冷却结束时间。
- `success_rate_ewma`：成功率 EWMA。
- `error_rate_ewma`：错误率 EWMA。
- `ttft_ewma_ms`：首包延迟 EWMA。
- `last_selected_at`：最近被选中时间。
- `last_error_at`：最近错误时间。
- `last_error_code`：最近错误分类。
- `decision_reason`：当前调度原因说明。

`scheduler_tier` 建议取值：

- `primary`：主力，正常情况下优先调度。
- `standby`：备用，主力不可用、满载或失败后接管。
- `observe`：观察，恢复中或波动中，默认少量探测或暂不承载主流量。
- `degraded`：降级，冷却期内默认不参与调度。

## 7. 健康分规则

健康分从 100 起算，按运行时信号扣分或恢复。

主要扣分因子：

- TTFT 高于阈值或同组延迟基线时扣分。
- 错误率升高时扣分。
- 连续失败时额外扣分。
- 429、5xx、timeout、network_error 按不同权重扣分。
- auth/token 类不可自愈错误直接进入不可调度或现有 error 状态。

建议错误权重：

- `rate_limited`：高权重，尊重现有 `rate_limit_reset_at`。
- `upstream_5xx`：中高权重，连续出现进入冷却。
- `timeout/network_error`：中权重，按连续次数判断。
- `high_latency`：低到中权重，先进入观察，不立即降级。
- `auth_error/token_error`：不可自愈错误，沿用现有账号错误处理。

恢复规则：

- 冷却到期后进入 `observe`。
- 连续成功次数达到阈值且 TTFT 回到阈值内，进入 `standby`。
- 健康分重新进入当前候选池前 N 或前百分比后，进入 `primary`。
- 手动解除冷却只进入 `observe`，不直接回主力。

## 8. 调度规则

请求进入后：

1. 沿用现有 OpenAI 账号候选过滤：
   - 分组过滤
   - OpenAI 平台过滤
   - 模型支持过滤
   - endpoint capability 过滤
   - `schedulable`、限流、过载、过期等状态过滤
   - 本次 failover 排除列表过滤
2. 基于运行时健康快照分层：
   - `primary`
   - `standby`
   - `observe`
   - `degraded`
3. 正常情况下优先从 `primary` 池选择。
4. 当主力池为空、满载、被排除或失败时，进入 `standby`。
5. `observe` 默认不承载主流量，可配置少量探测流量。
6. `degraded` 在冷却期内不参与调度。

同层内排序或加权选择因子：

- `dynamic_health_score`
- 当前负载与等待数
- `accounts.priority`
- 最近使用时间或随机扰动

failover 处理：

- 失败账号加入本次请求排除列表。
- 上报失败结果给调度器。
- 更新运行时健康分和降级原因。
- 若主力失败，下一轮选择备用或观察层可用账号。

## 9. 持久化策略

第一版避免高频写库。

建议：

- 运行时健康指标和分层快照放内存或 Redis。
- 页面查询运行时快照。
- 策略配置落库到 settings。
- 管理员手动操作可写入审计日志或系统日志。
- 历史趋势后续再新增分钟级 rollup 表。

可选后续表：

- `openai_scheduler_account_rollups`
- `openai_scheduler_events`

第一版不强制新增这些历史表。

## 10. 后台接口

建议路由统一挂在 admin 下。

### 10.1 获取概览

`GET /api/admin/openai-scheduler/overview`

返回：

- 主力账号数
- 备用账号数
- 观察账号数
- 降级账号数
- 平均 TTFT
- 平均健康分
- 最近切换次数
- 当前策略开关状态

### 10.2 获取账号列表

`GET /api/admin/openai-scheduler/accounts`

查询参数：

- `group_id`
- `model`
- `tier`
- `health_status`
- `search`
- `page`
- `page_size`

返回字段：

- 账号基本信息
- 所属分组
- 支持能力
- 分层状态
- 健康分
- 成功率
- TTFT
- 当前负载
- 冷却时间
- 调度原因

### 10.3 获取账号详情

`GET /api/admin/openai-scheduler/accounts/:id`

返回：

- 健康分详情
- 近期错误摘要
- 当前冷却状态
- 最近被调度原因
- 最近被跳过原因
- 支持模型和 capability 摘要

### 10.4 执行人工操作

`POST /api/admin/openai-scheduler/accounts/:id/actions`

请求体：

- `action`
- `reason`
- `duration_seconds`

第一版支持：

- `run_probe`：立即探测。
- `promote_observe`：进入观察状态。
- `cooldown`：手动冷却。
- `clear_cooldown`：解除冷却，进入观察。

第一版暂不默认支持 `pin_primary`，避免人工固定主力绕过健康保护。

### 10.5 获取与更新策略

`GET /api/admin/openai-scheduler/settings`

`PUT /api/admin/openai-scheduler/settings`

配置项：

- `enabled`
- `health_ranking_enabled`
- `primary_ratio`
- `primary_min_count`
- `ttft_degrade_ms`
- `error_rate_degrade_threshold`
- `consecutive_failure_threshold`
- `recover_success_threshold`
- `cooldown_seconds`
- `observe_probe_ratio`

## 11. 前端页面

新增页面：

- 路由：`/admin/openai-scheduler`
- 菜单名：`OpenAI 调度`
- 页面文件：`frontend/src/views/admin/OpenAISchedulerView.vue`
- API 文件：`frontend/src/api/admin/openaiScheduler.ts`

页面布局：

- 顶部概览卡：
  - 主力账号数
  - 备用账号数
  - 观察账号数
  - 降级账号数
  - 平均 TTFT
  - 健康均值
- 左侧策略配置：
  - 启用开关
  - 最快优先模式
  - 主力比例
  - 错误降级条件
  - 延迟降级条件
  - 恢复条件
- 右侧账号表：
  - 排名
  - 账号名
  - 分组
  - 模型/能力
  - 分层
  - 健康分
  - 成功率
  - TTFT
  - 当前负载
  - 调度原因
  - 操作按钮
- 详情弹窗：
  - 健康变化
  - 最近错误原因
  - 被选中或跳过原因
  - 手动冷却、解除冷却、立即探测

视觉风格：

- 复用现有后台的 `AppLayout`、`TablePageLayout`、`DataTable`、`Pagination`、`Toggle`、`ConfirmDialog` 等组件。
- 表格信息密度接近现有管理页，不做营销式大卡片。
- 分层状态使用清晰 badge：
  - 主力：绿色
  - 备用：蓝色
  - 观察：紫色
  - 降级：红色或橙红色

## 12. 开关与回滚

沿用现有 `openai_advanced_scheduler_enabled`，并新增更细粒度开关：

- `openai_scheduler_health_ranking_enabled`

开关语义：

- `openai_advanced_scheduler_enabled=false`：走现有非高级调度路径。
- `openai_advanced_scheduler_enabled=true` 且 `openai_scheduler_health_ranking_enabled=false`：保留现有高级调度，不使用健康分层。
- `openai_scheduler_health_ranking_enabled=true`：启用健康分层调度。

回滚方式：

- 线上出现异常时关闭 `openai_scheduler_health_ranking_enabled`。
- 关闭后健康数据仍可采集和展示为观测模式。
- 调度选择恢复到现有高级调度逻辑。

## 13. 错误处理

错误分类与处理：

- `429/rate_limited`：立即降分并尊重 `rate_limit_reset_at`。
- `5xx`：累计失败，超过阈值进入冷却。
- `timeout/network_error`：累计失败，结合连续次数判断。
- `high_latency`：先进入观察，连续慢再降级。
- `auth_error/token_error`：沿用现有账号 error 或不可调度逻辑。
- `snapshot/cache unavailable`：降级走现有调度器，不能影响请求可用性。

## 14. 验证计划

后端测试：

- 健康分计算：
  - 高成功率低延迟进入 `primary`。
  - 高延迟进入 `observe` 或 `standby`。
  - 连续错误进入 `degraded`。
  - 冷却到期后进入 `observe`。
- 调度选择：
  - 有主力时优先选择主力。
  - 主力为空、满载或失败时选择备用。
  - failover 排除本次失败账号。
  - 人工 `priority` 不被自动修改。
- 开关回退：
  - 新开关关闭时走旧高级调度行为。
  - Redis 或 cache 异常时请求仍可用。
- 接口测试：
  - overview 聚合正确。
  - accounts 支持筛选分页。
  - actions 拒绝非法状态跳转。

前端测试：

- 表格正确渲染 `primary`、`standby`、`observe`、`degraded`。
- 策略保存表单校验。
- 手动操作成功和失败提示。
- 空数据、加载中、接口错误状态。

手工验证：

1. 准备 3 个 OpenAI 测试账号，模拟快、慢、错误。
2. 观察页面是否自动分出主力、备用、降级。
3. 发起请求确认主力优先。
4. 模拟主力 5xx 或 429，确认 failover 到备用。
5. 恢复主力后确认先进入观察，再回到备用或主力。

## 15. 风险与控制

风险：

- 健康分阈值不合理导致账号误降级。
- observe 探测流量过大影响用户请求。
- 运行时快照与数据库账号状态短暂不一致。
- 页面展示的决策原因不完整，影响排障。

控制：

- 第一版默认保守阈值。
- observe 探测比例默认很低或关闭。
- 保留现有调度 fallback。
- 所有自动降级都记录原因。
- 新策略有独立开关，可快速关闭。

## 16. 实施顺序建议

1. 扩展 OpenAI 调度器运行时统计模型。
2. 增加健康分和分层计算逻辑。
3. 增加新策略开关和默认配置。
4. 将分层结果接入现有候选选择流程。
5. 增加 admin 查询接口和人工操作接口。
6. 新增前端 OpenAI 调度页面。
7. 补充后端与前端测试。
8. 手工验证主力、备用、降级、恢复完整流程。
