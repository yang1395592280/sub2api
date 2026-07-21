# OpenAI 自动最优惠分组质量优先与首字切换开发方案

日期：2026-07-21  
状态：已实现  
适用分支：`feature/openai-auto-cheapest-quality-failover`  
基线版本：`v0.1.161.5`

关联文档：

- [OpenAI 调度与自动最优惠分组稳定性修复方案](./2026-07-16-openai-scheduler-and-auto-cheapest-reliability-fix.md)
- [OpenAI 自动最优惠分组剩余优化方案](./2026-07-16-openai-auto-cheapest-remaining-optimizations.md)
- [OpenAI 自动调度运行参数与公平性优化方案](./2026-07-20-openai-scheduler-runtime-configuration-and-fairness-optimization.md)

## 1. 结论

本次改造不采用“某个低价账号失败后立即降低整个低价组优先级”。`v0.1.161.2` 已经将自动最优惠分组修正为账号级失败隔离：单个账号失败后只排除该账号，下一轮仍优先查找同一低价组内的其他账号，只有该组无可用账号才进入更高价格分组。

本次需要补充的是“可用”与“达标”的区分：

```text
价格从低到高扫描
  -> 每个分组只尝试健康新鲜、可靠且首字延迟达标的账号
  -> 低价组存在任一达标账号时立即使用低价组
  -> 单账号失败只排除该账号
  -> 当前价格组没有达标账号时继续扫描更高价格组
  -> 首次请求所有价格组均无达标账号时，允许一次可用性兜底
  -> 首字超时后禁止低质量兜底，只能使用达标账号
```

关闭 Shadow 后，Balanced 的账号级健康拒绝和质量排序才影响真实选路。关闭 Shadow 不改变分组按价格升序扫描的规则，也不会把低价组中的健康账号整体降级。

## 2. 问题背景

生产观察显示，部分低价分组上游存在大量 `403`、`502`、`503` 和探测失败，首字 P95 已达到二十秒以上。高价分组整体更稳定，但当前自动最优惠策略仍可能出现：

1. Shadow 模式下 Balanced 只计算影子顺序，真实请求继续使用 Legacy 顺序。
2. 分组之间只比较价格，不比较“是否存在高置信、低延迟账号”。
3. 全组只有低置信账号时，Balanced 为保持可用性会进入 `fallback`，自动最优惠仍可能先命中低质量低价组。
4. `/v1/responses` 已有首个语义输出超时，但默认关闭，且最小配置值为 30 秒。
5. `/v1/chat/completions` 的 Responses 转换流和 raw Chat 流未获得同等首字超时保护。
6. 首字超时后当前最多只允许一次切换，但下一次选择没有显式禁止低质量兜底。

## 3. 目标

### 3.1 功能目标

1. 低价组有达标账号时必须优先使用低价组。
2. 单个账号出现 `403/502/503/传输错误/首字超时` 时只排除该账号。
3. 低价组没有达标账号时，允许直接选择更高价格组的达标账号。
4. 首次选择在所有分组均无达标账号时保留可用性兜底，避免健康数据刚初始化时全量不可用。
5. 首字超时后关闭可用性兜底，避免连续等待多个低质量账号。
6. 首字超时发生在客户端语义输出之前时，可取消当前上游并切换账号。
7. `/v1/responses`、Responses WebSocket v2、Chat Completions 转换流和 raw Chat 流使用一致的首字超时语义。
8. Shadow 开启时不改变真实选号；Shadow 关闭后质量门槛才进入真实路径。

### 3.2 非目标

- 不把不同账号的单次失败聚合成整个分组故障。
- 不改变分组倍率、用户计费或价格上限逻辑。
- 不保证客户端已经收到语义内容后仍可无缝切换上游。
- 不对图片生成、Embeddings 和非 OpenAI 平台启用首字超时。
- 不修改生产配置、数据库数据或运行中实例。

## 4. 当前源码基线

### 4.1 分组顺序

`OpenAIAutoCheapestGroupResolver.CandidateGroups` 按以下顺序排序：

1. 用户有效倍率或分组默认倍率升序。
2. `sort_order` 升序。
3. 分组 ID 升序。

### 4.2 账号失败与跨组切换

HTTP handler 使用 `failedAccountIDs` 保存当前请求已失败账号。重新选择时：

1. 从最低价分组重新开始。
2. 已失败账号由 `ExcludedIDs` 排除。
3. 同组其他账号仍可被选中。
4. 账号选择返回无可用候选后才标记当前分组耗尽。
5. 分组耗尽后，请求内跳过该组，并触发按模型、endpoint、transport 隔离的 60 秒 Redis 短熔断。

此行为必须保留。

### 4.3 Balanced 与 Shadow

Balanced 已能输出以下账号资格：

- `eligible`
- `low_confidence`
- `latency_tail`
- `hard_rejected`

Shadow 开启时，Balanced 会保留审计结果，但清空真实拒绝集合并恢复 Legacy 顺序。Shadow 关闭后，`open/half_open` 等硬拒绝才会从真实选择顺序移除。

### 4.4 首字超时

首字超时已统一接入原生 Responses HTTP、Responses WebSocket v2、Chat Completions 转换流和 raw Chat 流。运行时值由 OpenAI 自动调度控制台的系统设置保存：

- `first_output_timeout_seconds` 默认 `0`，表示关闭；启用范围 `5-600` 秒。
- `high_effort_first_output_timeout_seconds` 默认 `0`，表示关闭；启用范围 `30-1800` 秒。
- 服务端 YAML 同名字段仅保留为 `SettingService` 不可用时的兼容兜底，正常生产请求以系统设置为准。

## 5. 设计不变量

无论价格和 sticky 如何配置，都必须满足：

1. `open/half_open`、运行时封禁、模型不兼容、能力不兼容和价格保护账号不能重新进入候选池。
2. sticky 只能改变达标账号之间的顺序，不能覆盖有效健康拒绝。
3. 单账号失败不设置请求级 `failedGroups`，也不写分组短熔断。
4. 只有完成组内账号选择且没有可用候选时，才允许标记分组耗尽。
5. 首字切换只允许发生在未向客户端输出语义内容时。
6. 任何质量状态读取异常必须按可用性优先回退，不得将健康存储故障转换成用户 `502`。

## 6. 达标账号定义

自动最优惠分组的“达标账号”复用现有统一健康数据，不新增第二套健康状态。

账号同时满足以下条件时为达标：

1. 通过账号状态、模型、endpoint、transport、价格保护和运行时封禁硬过滤。
2. Balanced 当前为非 Shadow、非 Legacy 的真实模式。
3. 统一健康快照读取成功。
4. 策略资格为 `eligible`，即不是低置信、延迟尾部或硬拒绝。
5. 预测 TTFT 大于 0，且不超过现有 `slow_threshold_ms`。
6. 当前能够立即取得账号并发槽位。

`observing`、仅探测样本、真实样本过期和健康快照缺失均不进入达标池，但仍保留在首次请求的可用性兜底池中，并继续受现有探索预算约束。

## 7. 两阶段分组扫描

### 7.1 第一阶段：质量优先扫描

对自动最优惠 API Key 的候选分组按价格升序逐个执行“仅达标账号”选择：

```text
cheap group A: 有达标账号 -> 返回 A
cheap group A: 无达标账号 -> 检查 group B
group B: 有达标账号       -> 返回 B
```

质量扫描失败不视为分组硬耗尽，不写 Redis 分组熔断。原因是组内可能仍有低置信账号，后续探测或真实样本可使其恢复。

### 7.2 第二阶段：可用性兜底

仅在首次请求所有分组都没有达标账号时，按原有价格顺序执行一次宽松选择：

- 允许现有低置信探索和全低置信 `fallback`。
- 仍拒绝有效 `open/half_open` 和运行时封禁。
- 仍保持最低价优先。
- 真正无账号时按现有逻辑标记组耗尽。

如果统一健康读取失败、自动调度未启用、模式为 Legacy 或 Shadow 开启，则质量过滤不生效，保持现有真实选择行为。

### 7.3 首字超时后的严格重选

当前请求发生 `first_output_timeout` 后，在请求上下文设置“禁止质量兜底”标记：

1. 当前超时账号加入 `failedAccountIDs`。
2. 下一轮仍从最低价分组开始。
3. 同一低价组还有达标账号时继续使用该组。
4. 同一低价组只剩低置信或慢账号时进入更高价格组。
5. 所有分组均无达标账号时返回最后一次首字超时错误，不再继续消耗低质量账号。

## 8. 首字超时统一

### 8.1 配置

继续复用：

```yaml
gateway:
  openai_first_output_timeout_seconds: 0
  openai_high_effort_first_output_timeout_seconds: 0
```

调整校验范围：

| 参数 | 范围 | 推荐生产初值 |
| --- | --- | ---: |
| 普通首字超时 | `0` 或 `5-600` 秒 | `10` |
| high/xhigh/max 首字超时 | `0` 或 `30-1800` 秒 | `60` |

默认继续为 `0`，发布代码本身不自动改变生产行为。

### 8.2 超时起点

超时从单次账号 forward 开始计时，包含：

- 等待响应头。
- 收到响应头后等待首个语义输出。
- Responses 的 `response.created`、`response.in_progress` 等前导事件。
- Chat 流中的空行、usage-only、keepalive 和被安全检测暂存的前导帧。

### 8.3 超时动作

1. 取消或关闭当前上游请求/响应体。
2. 不向客户端泄漏当前尝试的上游响应头或语义字节。
3. 生成 `504 first_output_timeout` 类型的 `UpstreamFailoverError`。
4. 记录账号级调度失败和统一健康事件。
5. 当前账号进入 `failedAccountIDs`。
6. 自动最优惠请求启用严格质量重选。
7. 保留“最多一次首字超时换号”限制，防止重复请求和潜在重复计费无限扩大。

## 9. Shadow 行为

| 状态 | 账号级质量顺序 | 自动最优惠质量跨组扫描 |
| --- | --- | --- |
| `shadow_mode=true` | 只审计，真实使用 Legacy | 不改变真实选择 |
| `shadow_mode=false` | 真实应用资格、评分和熔断 | 真实应用达标账号优先 |
| 健康存储异常 | 回退 Legacy | 回退原价格优先 |

因此关闭 Shadow 是让本次质量门槛生效的必要条件，但不是首字体验改善的全部条件。首字超时必须配置为非零，Chat 路径也必须完成代码覆盖。

## 10. 代码改动

| 文件 | 改动 |
| --- | --- |
| `openai_auto_cheapest_group_context.go` | 增加质量扫描标记和首字超时后的禁止兜底状态 |
| `openai_gateway_scheduling.go` | 自动最优惠执行质量扫描和可用性兜底两阶段选择 |
| `openai_account_scheduler.go` | 请求级仅达标账号过滤，保持固定分组行为不变 |
| `openai_balanced_scheduler.go` | 将现有慢阈值传入运行时 Balanced 设置 |
| `openai_first_output_timeout.go` | 增加首字超时原因标识和公共判定 |
| `openai_gateway_chat_completions.go` | Responses 转换流增加响应头及语义输出超时 |
| `openai_gateway_chat_completions_raw.go` | raw Chat 流增加响应头及语义输出超时 |
| `openai_gateway_cc_pipeline.go` | CC 请求发送支持可选首字响应头 deadline |
| `openai_gateway_handler.go` | Responses/WS 首字超时后启用严格质量重选 |
| `openai_chat_completions.go` | Chat 首字超时允许安全换号并启用严格质量重选 |
| `config.go` | 普通首字超时最小值调整为 5 秒 |

## 11. 兼容性与安全边界

### 11.1 固定分组

质量跨组扫描只在 `UsesOpenAIAutoCheapestGroup()` 为真时创建。固定分组 API Key 的选择、sticky、等待队列和计费行为不变。

### 11.2 非流式请求

首字超时只作用于流式请求。非流式请求继续使用现有 HTTP 超时和 failover 逻辑。

### 11.3 已输出语义内容

一旦客户端已经收到语义内容，不允许切换账号拼接第二条流。仅写出 SSE keepalive 注释时可沿用现有 `SafeToFailoverAfterWrite` 规则。

### 11.4 重复请求和计费

上游可能在取消前已开始计算。即使客户端未收到内容，切换账号仍可能产生两次上游成本，因此继续保留一次首字超时换号上限。

## 12. 日志与观测

新增或补充结构化日志：

- `openai_auto_cheapest_quality_group_skipped`
  - `group_id`
  - `requested_model`
  - `strict_after_timeout`
- `openai_auto_cheapest_quality_fallback`
  - 所有分组无达标账号后进入可用性兜底
- `first_output_timeout`
  - `account_id`
  - `group_id`
  - `endpoint`
  - `phase`
  - `elapsed_ms`
  - `timeout_ms`

继续复用现有调度审计、usage TTFT、账号切换数和统一健康快照，不新增数据库迁移。

## 13. 测试矩阵

### 13.1 自动最优惠质量扫描

1. 低价组有一个达标账号和多个低置信账号，选择达标低价账号。
2. 低价组无达标账号、高价组有达标账号，选择高价组。
3. 低价组第一个账号失败后，同组另一个达标账号仍优先于高价组。
4. 所有分组无达标账号，首次请求回到最低价低置信兜底。
5. 首字超时后所有分组无达标账号，不再进入低置信兜底。
6. Shadow 开启时质量扫描不改变 Legacy 实际顺序。
7. 健康存储读取失败时回退原价格优先。
8. 固定分组 API Key 不受影响。

### 13.2 首字超时

1. Chat Responses 转换路径等待响应头超时，取消上游并返回可切换错误。
2. Chat Responses 转换路径只有前导事件、没有语义输出时超时。
3. raw Chat 等待响应头超时。
4. raw Chat 收到非语义前导帧但没有客户端输出时超时。
5. 首个语义输出到达后停止首字计时器。
6. 超时前客户端不收到尝试账号的响应头或语义字节。
7. 已有语义输出后不切换账号。
8. 配置为 `0` 时保持原同步路径。
9. 普通超时允许 5 秒，高推理超时仍要求至少 30 秒。

## 14. 灰度、发布与回滚

### 14.0 首字超时运行时设置

普通请求和高推理请求的首字超时已纳入 OpenAI 调度控制台的“灰度与配置”页，保存到 `openai_auto_scheduler_settings` 系统设置，不需要编辑服务器 YAML 或重启服务。界面保存后通过现有设置缓存立即传播，网关请求路径优先使用该运行时值；仅在 `SettingService` 不可用的测试或异常启动路径中保留 YAML 值作为兼容兜底。

- `first_output_timeout_seconds`：`0` 表示关闭；启用时允许 5-600 秒。
- `high_effort_first_output_timeout_seconds`：`0` 表示关闭；启用时允许 30-1800 秒。
- 推荐生产初始值：普通请求 10 秒，高推理请求 60 秒。
- 保存后应观察首字超时次数、换号成功率、重复上游成本和用户端错误率。

### 14.1 发布顺序

1. 发布代码，保持 `shadow_mode=true` 且首字超时为 `0`，确认无行为变化。
2. 将普通首字超时设为 10 秒，高推理设为 60 秒，先观察超时事件和换号结果。
3. 确认 Chat 与 Responses 的最终错误率、重复请求和上游成本没有异常。
4. 关闭 Shadow，使达标账号质量门槛进入真实流量。
5. 观察至少 24 小时后再调整阈值。

### 14.2 重点指标

- 首字 P50/P90/P95。
- 首字超时次数和超时后成功率。
- 低价组首次命中率。
- 低价组达标账号命中率。
- 跨组升档比例和倍率增幅。
- 最终 `502/503/504` 率。
- 单请求账号切换次数。
- 低置信 fallback 次数。
- `all_rejected` 次数。

### 14.3 回滚

1. 将首字超时配置恢复为 `0`，立即关闭主动首字切换。
2. 将 `shadow_mode` 恢复为 `true`，质量门槛退回只观察。
3. 必要时回滚应用版本；本次不新增数据库结构，代码回滚无迁移依赖。

## 15. 验收标准

1. 低价组存在达标账号时，真实请求不会因同组其他账号失败而整体跳过低价组。
2. 低价组仅剩低置信或慢账号、高价组存在达标账号时，非 Shadow 模式选择高价组。
3. 首字超时后不会再次选择低质量兜底账号。
4. Chat Completions 与 Responses 均能在客户端语义输出前完成一次首字超时换号。
5. Shadow 开启、首字超时关闭时与 `v0.1.161.5` 行为兼容。
6. 固定分组、计费、模型映射、Compact、价格保护和粘性会话无回归。
7. 相关 service、handler、config 测试全部通过。

## 16. 实施结果

本方案已按上述设计完成源码实现：

1. 自动最优惠分组先按价格顺序扫描达标账号，低价组内有达标账号时不会被同组其他故障账号拖累。
2. 所有分组都没有达标账号时，首次选择保留一次原有可用性兜底；质量扫描本身不写组级短熔断。
3. 首字超时后，请求进入严格质量重选，不再尝试低置信、延迟尾部或超过 `slow_threshold_ms` 的账号。
4. sticky 与 `previous_response_id` 在严格质量过滤之后执行，不能覆盖有效熔断和质量门槛。
5. 原生 Responses、Responses WebSocket v2、Chat Completions Responses 转换流和 raw Chat 流统一使用明确的 `first_output_timeout` 原因。
6. Chat 两条流式路径均覆盖响应头等待和首个客户端语义输出等待，前导事件、usage-only chunk 和 keepalive 不会提前解除计时。
7. 普通首字超时校验范围已调整为 `0` 或 `5-600` 秒；高推理范围保持 `0` 或 `30-1800` 秒，默认值仍为 `0`。

实际验证：

```text
go test ./internal/service -count=1  -> 通过
go test ./internal/handler -count=1  -> 通过
go test ./internal/config -count=1   -> 通过
go test ./... -count=1               -> 通过
git diff --check                     -> 通过
```
