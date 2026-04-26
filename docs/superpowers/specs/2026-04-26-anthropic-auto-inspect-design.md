# Anthropic 自动巡检设计

## 背景

当前账号管理页支持手动“测试连接”和单账号“定时测试”，但缺少一个面向 `platform=Anthropic` 账号的统一自动巡检能力。目标是在后台每 1 分钟自动巡检一次 Anthropic 账号，逐个串行执行测试连接，并在检测到限流或错误时自动把账号置为不可调度，同时记录可查询的巡检日志。

## 目标

1. 后端固定巡检所有 `platform=Anthropic` 的账号，不依赖前端当前筛选条件。
2. 巡检按账号逐个串行执行，单次批次中不并发测试多个账号。
3. 已经“不可调度且已设置自动恢复时间”的账号跳过巡检。
4. 限流时把账号设置为临时不可调度，并将自动恢复时间写为限流结束时间。
5. 普通错误时把账号设置为临时不可调度，并将自动恢复时间写为当前时间加 30 分钟。
6. 提供自动巡检日志页，记录账号、时间、返回内容、处理结果与恢复时间，便于排障。

## 非目标

1. 不改造现有单账号 `scheduled_test_plans` 的产品语义，不把全局 Anthropic 巡检伪装成若干隐藏的单账号计划。
2. 不把巡检触发依赖到前端页面在线状态。
3. 不在本期扩展到 OpenAI、Gemini、Antigravity 等其他平台。
4. 不引入永久禁用逻辑；本期异常处理统一复用临时不可调度机制。

## 现状约束

### 现有可复用能力

1. `AccountTestService.RunTestBackground(...)` 已支持后台执行账号测试，并输出 `ScheduledTestResult` 风格的结果。
2. `ScheduledTestRunnerService` 已具备按分钟触发后台 runner 的基础模式，可复用“服务启动后周期执行”的思路。
3. 账号模型已有 `schedulable`、`temp_unschedulable_until`、`temp_unschedulable_reason` 字段，可直接表达“临时不可调度直到某时刻”。
4. 管理端账号列表已有测试连接、调度开关、临时不可调度状态展示等基础 UI。

### 现有不足

1. 现有 `scheduled_test_plans/results` 是“单账号计划”模型，不适合承载“全局 Anthropic 巡检”。
2. 现有后台测试结果只区分成功/失败，没有把“限流”“普通错误”“跳过”抽象成巡检领域结果。
3. 现有管理端没有一个全局巡检日志入口，也无法按批次或结果类型检索巡检记录。

## 方案选型

### 方案 A：复用现有 scheduled test plans

为每个 Anthropic 账号自动创建隐藏计划，并扩展 runner 聚合日志。

优点：

1. 可复用现有表结构与部分 runner 逻辑。

缺点：

1. 需要把“全局 Anthropic 巡检”拆成多个隐式单账号计划，模型别扭。
2. 很难自然表达“跳过已临时不可调度账号”“集中全局日志页”“全局开关”等规则。
3. 后续维护时会混淆“手工定时测试”和“系统自动巡检”两套语义。

### 方案 B：新增专用 Anthropic 自动巡检服务

新增专用 runner、巡检日志存储、管理接口和日志页面；测试连接能力复用现有 `AccountTestService`。

优点：

1. 领域边界清晰，直接对应“Anthropic 全局自动巡检”。
2. 规则自然：逐个串行、全局开关、跳过规则、专用日志页都易于实现。
3. 不污染现有单账号 `scheduled_test` 功能。

缺点：

1. 需要新增少量后端表结构、服务、接口与前端页面。

### 方案 C：前端定时触发巡检

在账号管理页中使用前端定时器调用批量测试接口。

优点：

1. 表面实现成本低。

缺点：

1. 依赖浏览器页面在线，不稳定。
2. 与“固定巡检所有 Anthropic 账号”的目标冲突。
3. 不适合做后端可信日志与串行 worker。

### 推荐方案

采用方案 B：新增专用 Anthropic 自动巡检服务。

## 总体设计

### 核心组件

1. `AnthropicAutoInspectService`
   - 负责每分钟触发一次 Anthropic 巡检批次。
   - 保证同一时刻最多只有一个批次在运行。
   - 按顺序查询并巡检目标账号。

2. `AnthropicAutoInspectClassifier`
   - 将测试连接返回结果分类为：
     - `success`
     - `rate_limited`
     - `error`
     - `skipped`
   - 从返回内容中提取限流结束时间与摘要信息。

3. `AnthropicAutoInspectLogRepository`
   - 持久化巡检日志与批次信息。
   - 支持分页、时间范围、账号名、结果类型筛选。

4. 管理端巡检设置与日志页
   - 账号管理页顶部增加全局自动巡检入口。
   - 新增“自动巡检日志”页面查看巡检结果。

### 执行时序

1. 每 1 分钟 tick 一次。
2. 若上一个巡检批次尚未结束，则当前 tick 直接放弃，并记录内部跳过日志。
3. 查询所有 `platform=Anthropic` 账号。
4. 过滤掉 `schedulable=false` 且 `temp_unschedulable_until != nil` 的账号。
5. 按账号 `id ASC` 的稳定顺序逐个执行后台测试连接。
6. 对每个账号生成一条日志。
7. 对限流或错误账号更新临时不可调度状态。
8. 批次结束后写入批次汇总。

## 数据模型

### 新增巡检日志表

新增 `anthropic_auto_inspect_logs`，字段包含：

1. `id`
2. `batch_id`
3. `account_id`
4. `account_name_snapshot`
5. `platform`
6. `account_type`
7. `result`
   - `success`
   - `rate_limited`
   - `error`
   - `skipped`
8. `skip_reason`
9. `response_text`
10. `error_message`
11. `rate_limit_reset_at`
12. `temp_unschedulable_until`
13. `schedulable_changed`
14. `started_at`
15. `finished_at`
16. `latency_ms`
17. `created_at`

### 新增巡检批次表

新增 `anthropic_auto_inspect_batches`，字段包含：

1. `id`
2. `trigger_source`
   - `scheduler`
   - `manual`
3. `status`
   - `running`
   - `completed`
   - `failed`
4. `total_accounts`
5. `processed_accounts`
6. `success_count`
7. `rate_limited_count`
8. `error_count`
9. `skipped_count`
10. `started_at`
11. `finished_at`
12. `created_at`

### 全局设置

在系统设置中新增：

1. `anthropic_auto_inspect_enabled`
2. `anthropic_auto_inspect_interval_minutes`
   - 当前固定为 `1`，仍持久化，便于未来扩展
3. `anthropic_auto_inspect_error_cooldown_minutes`
   - 当前固定为 `30`

## 账号筛选与跳过规则

### 候选账号范围

只巡检：

1. `platform=Anthropic`

### 跳过规则

账号满足以下条件时，跳过本轮巡检并写日志：

1. `schedulable=false`
2. `temp_unschedulable_until` 不为空

跳过原因统一记为 `already_temp_unschedulable`。

### 不跳过的情况

以下情况仍参与巡检：

1. `status=error` 但没有自动恢复时间
2. `schedulable=true` 但 `status=error`
3. 普通正常账号

## 测试执行规则

### 执行方式

1. 复用 `AccountTestService.RunTestBackground(ctx, accountID, modelID)`。
2. runner 内部对账号逐个调用，不使用 goroutine 并发同批次账号测试。
3. 每个账号测试有独立超时，避免单个账号阻塞整个批次。

### 模型选择

1. 优先使用账号可测模型中的 Sonnet 模型。
2. 若账号存在模型映射，则优先使用映射中可测的 Sonnet。
3. 若未找到 Sonnet，则回退当前默认测试模型。

## 结果分类规则

### 成功

满足以下任一条件视为成功：

1. 测试完成且 `status=success`
2. 返回 SSE 内容正常完成，且无 error event

处理：

1. 仅记录日志。
2. 不主动清除已有临时不可调度状态，因为本期跳过了此类账号。

### 限流

满足以下任一条件视为限流：

1. 返回内容中出现明确限流语义，如 `rate limit`、`rate limited`、`too many requests`
2. 返回内容可解析出限流结束时间
3. 未来若 `AccountTestService` 暴露更结构化的限流字段，则优先使用结构化字段

处理：

1. 调用账号仓储写入临时不可调度状态。
2. `temp_unschedulable_until = 限流结束时间`
3. `temp_unschedulable_reason` 写入巡检限流摘要
4. 日志 `result = rate_limited`

### 普通错误

满足以下条件视为普通错误：

1. 测试失败
2. 未命中限流规则

处理：

1. 调用账号仓储写入临时不可调度状态。
2. `temp_unschedulable_until = now + 30 minutes`
3. `temp_unschedulable_reason` 写入巡检错误摘要
4. 日志 `result = error`

### 跳过

满足跳过规则时：

1. 不执行测试连接
2. 写日志 `result = skipped`
3. 写 `skip_reason = already_temp_unschedulable`

## 串行与并发控制

### 同批次账号执行

1. 必须严格串行。
2. 一个账号结束后再开始下一个账号。

### 批次重入保护

1. runner 需要持有进程内互斥锁。
2. 若系统已有分布式锁能力，优先增加一个轻量全局锁，防止多实例重复巡检。
3. 若无法拿到锁，则本轮不执行。

## API 与前端

### 账号管理页入口

在账号管理页顶部增加“1分钟自动巡检”入口，提供：

1. 全局开关
2. 跳转日志页按钮
3. “立即执行一次”按钮

### 新增管理接口

至少包括：

1. `GET /api/v1/admin/anthropic-auto-inspect/settings`
2. `PUT /api/v1/admin/anthropic-auto-inspect/settings`
3. `POST /api/v1/admin/anthropic-auto-inspect/run`
4. `GET /api/v1/admin/anthropic-auto-inspect/logs`
5. `GET /api/v1/admin/anthropic-auto-inspect/batches`

### 日志页

新增独立页面“自动巡检日志”，展示：

1. 巡检时间
2. 账号名称
3. 平台/类型
4. 巡检结果
5. 返回内容摘要
6. 是否修改了调度状态
7. 自动恢复时间
8. 批次 ID

支持筛选：

1. 账号名搜索
2. 结果类型
3. 时间范围

## 错误处理

1. 单个账号巡检失败不能中断整个批次。
2. 账号测试超时应归类为普通错误，并应用 30 分钟恢复时间。
3. 日志写入失败需要记录系统日志，但不应导致账号状态回滚。
4. 批次级失败要写入批次状态，便于后续排查。

## 测试策略

### 后端

1. runner 仅查询 Anthropic 账号。
2. 已临时不可调度账号会被跳过。
3. 同一批次账号串行执行。
4. 限流结果会写入限流结束时间。
5. 普通错误会写入 30 分钟自动恢复时间。
6. 单账号失败不会影响后续账号继续执行。
7. 多实例或重复 tick 下不会发生并发重入。

### 前端

1. 账号管理页能展示全局巡检开关与日志入口。
2. 日志页能正确展示结果、恢复时间、摘要与筛选项。
3. 手动触发巡检与设置更新的交互状态正确。

## 风险与权衡

1. 限流识别如果仅依赖文本匹配，可能存在误判；后续可逐步升级为结构化错误解析。
2. 若 Anthropic 账号量较大，严格串行可能导致单批次执行时间超过 1 分钟；因此必须加重入保护，允许下一分钟跳过而不是叠加并发。
3. 若部署为多实例，必须确认锁策略，否则可能重复巡检同一账号。

## 里程碑

1. 后端：补齐巡检日志表、批次表、runner、分类器、设置接口。
2. 前端：账号页入口、日志页、路由、API 封装。
3. 验证：单测、定向集成验证、手动 UI 冒烟。
