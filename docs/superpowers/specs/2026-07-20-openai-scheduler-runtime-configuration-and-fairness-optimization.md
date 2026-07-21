# OpenAI 自动调度运行参数与公平性优化方案

日期：2026-07-20

## 1. 文档目标

本文针对当前生产环境的 OpenAI 自动调度配置和运行数据，回答两个问题：

1. 当前控制台参数应该如何调整。
2. 如何完善 sticky session 熔断边界、低置信度账号探索、Top-K 集中度和最小探索量。

本文区分“立即可通过控制台调整的参数”和“必须修改代码才能实现的能力”。参数调整只能缓解当前问题，不能替代状态源统一、错误归因和恢复状态机改造。

## 2. 当前生产基线

当前线上版本为 `0.1.161.2`，自动调度和高级调度均已开启，生效模式为 `balanced`，非影子模式。

当前主要配置：

| 参数 | 当前值 |
| --- | ---: |
| `top_k` | 2 |
| `exploration_rate` | 0.03 |
| `session_escape_min_gap_ms` | 1000 |
| `session_escape_ratio` | 0.25 |
| `cost_weight` | 0.20 |
| `temperature` | 0.18 |
| `max_account_share` | 0.71 |
| `low_confidence_max_share` | 0.06 |
| `latency_budget_ms` | 1000 |
| `slow_threshold_ms` | 10000 |
| `severe_slow_threshold_ms` | 20000 |
| `consecutive_slow_breaker_threshold` | 5 |
| `consecutive_error_breaker_threshold` | 5 |
| `cooldown_seconds` | 30 |
| `half_open_success_threshold` | 3 |
| `recovery_step` | 800 |
| `probe_model` | `gpt-5.5` |
| `probe_interval_seconds` | 120 |
| `probe_jitter_seconds` | 0 |
| `health_ttl_seconds` | 1800 |
| `real_sample_fresh_seconds` | 300 |

生产数据暴露出的主要现象：

- 当前分组约有 10 个候选账号，但 Top-K 只有 2，流量容易长期集中在少数账号。
- `group_id=82` 最近一小时有 238 次 HTTP 403 被计入普通错误，429 只有 9 次。
- `group_id=82 / gpt-5.5` 的旧状态表中，5 条 `open` 的冷却时间均已过期，但账号页仍显示熔断。
- 统一健康表中 83 条 `open` 有 79 条已经超过健康 TTL。
- 当前冷却时间 30 秒，但探测间隔为 120 秒、真实样本新鲜期为 300 秒、恢复要求 3 个成功样本，实际恢复时间远长于 30 秒。

## 3. 建议立即调整的参数

### 3.1 推荐配置

当前设置接口没有 `group_id` 参数，整套策略通过全局 `openai_auto_scheduler_settings` 保存。截图中虽然选择了 `plus` 临时分组，但保存以下参数会影响所有已经开启“参与均衡调度”的分组，不是只影响当前选中的分组。

建议在变更前先确认所有已启用分组都能接受同一参数。若只希望对 `plus` 临时分组灰度，应先实现分组级参数覆盖；或者在维护窗口内暂时让其他分组退出自动调度。后一种做法会改变其他分组的真实调度路径，必须单独评估和确认，不能仅凭本方案直接执行。

推荐参数如下：

| 参数 | 当前值 | 建议值 | 目的 |
| --- | ---: | ---: | --- |
| `top_k` | 2 | **4** | 让约 10 个候选中的更多账号获得正常流量，降低双账号集中风险 |
| `exploration_rate` | 0.03 | **0.05** | 提高 Top-K 内探索强度，同时保持总探索成本可控 |
| `temperature` | 0.18 | **0.22** | 适度拉平 Top-K 内份额，降低高分账号长期独占 |
| `max_account_share` | 0.71 | **0.55** | 多候选正常时，限制单账号过度集中 |
| `low_confidence_max_share` | 0.06 | **0.10** | 恢复设计文档原建议值，避免低置信账号长期饥饿 |
| `latency_budget_ms` | 1000 | **3000** | 当前跨渠道 TTFT 波动较大，1 秒预算容易将大量账号压入延迟尾部 |
| `consecutive_error_breaker_threshold` | 5 | **8（临时）** | 在 403 错误尚未正确归因前，降低误熔断频率；完成错误分类后恢复为 5 |
| `cooldown_seconds` | 30 | **120** | 与探测周期处于同一量级，避免配置显示 30 秒但实际无法恢复 |
| `half_open_success_threshold` | 3 | **2** | 缩短恢复验证时间，同时保留连续成功保护 |
| `probe_interval_seconds` | 120 | **90** | 提高冷账号和 half-open 状态恢复速度 |
| `probe_jitter_seconds` | 0 | **15** | 避免所有账号和实例在固定时刻集中探测 |
| `real_sample_fresh_seconds` | 300 | **180** | 避免刚熔断的账号因“真实样本仍新鲜”而长时间跳过恢复探测 |

建议暂时保持不变：

| 参数 | 保持值 | 原因 |
| --- | ---: | --- |
| `session_escape_min_gap_ms` | 1000 | 当前值符合既有设计；sticky 是否越过熔断不是由该参数控制 |
| `session_escape_ratio` | 0.25 | 当前值符合既有设计，暂未发现需要放宽的直接证据 |
| `cost_weight` | 0.20 | 当前问题主要是健康与分配边界，不是成本权重失衡 |
| 综合效用权重 | 保持当前 | 先修正候选范围和份额边界，避免同时改动过多变量 |
| `slow_threshold_ms` | 10000 | 当前慢响应较多，但应先统一 TTFT/E2E 口径，直接抬高阈值会掩盖性能问题 |
| `severe_slow_threshold_ms` | 20000 | 同上 |
| `consecutive_slow_breaker_threshold` | 5 | 当前值可保留，后续改成滑动窗口后再校准 |
| `recovery_step` | 800 | 主要影响旧分数表，当前不是核心矛盾 |
| `probe_model` | `gpt-5.5` | 现阶段保持兼容；后续必须改为按模型族和端点探测 |
| `health_ttl_seconds` | 1800 | 最小探索尚未实现前不宜缩短，否则会制造更多低置信账号 |

### 3.2 推荐配置 JSON

以下只列出建议调整项：

```json
{
  "top_k": 4,
  "exploration_rate": 0.05,
  "temperature": 0.22,
  "max_account_share": 0.55,
  "low_confidence_max_share": 0.10,
  "latency_budget_ms": 3000,
  "consecutive_error_breaker_threshold": 8,
  "cooldown_seconds": 120,
  "half_open_success_threshold": 2,
  "probe_interval_seconds": 90,
  "probe_jitter_seconds": 15,
  "real_sample_fresh_seconds": 180
}
```

### 3.3 参数调整的边界

上述配置无法实现以下能力：

- 不能保证 Top-K 之外的低置信账号获得任何真实流量。
- 不能保证每个低置信账号每小时获得固定数量的探索机会。
- 不能修复 400/403 被错误归因的问题。
- 不能消除旧状态表和统一健康表之间的不一致。
- 不能保证健康快照缺失或过期时 sticky 一定逃逸。

这些能力必须通过代码实现。

## 4. 问题一：Sticky Session 不得覆盖有效熔断

### 4.1 当前实现情况

统一 Balanced 链路在健康快照完整且新鲜时，已经先检查 circuit，再处理 session sticky。`open` 和 `half_open` 会被拒绝，普通 session 不会覆盖有效熔断。

当前仍存在以下边界：

- 健康快照缺失或过期时，候选会按 `running + low_confidence` 继续参与。
- 健康存储整体失败时会回退 Legacy。
- 旧 Selector 和统一 Balanced 对过期 `open`、`half_open` 的处理语义不同。
- 账号页展示旧状态，管理员无法判断 sticky 实际依据的是哪一个状态源。
- `previous_response_id`、普通 session 和重试 failover 的亲和强度不同，需要分别声明边界。

### 4.2 必须保持的调度不变量

无论请求是否携带 sticky 信息，都必须按以下优先级执行：

```text
账号和分组硬条件
  -> 模型/能力/transport 兼容
  -> 价格和配额保护
  -> 有效 circuit / runtime block
  -> sticky affinity
  -> 综合评分和流量分配
```

必须满足：

1. `open` 状态在有效冷却期内永远不能被 sticky 选中。
2. `half_open` 只能接受恢复探针或明确受控的单次恢复流量。
3. 429、5xx、并发满、排队超限和价格保护触发时，普通 session 必须立即逃逸。
4. sticky 只能改变合格候选之间的顺序，不能把硬拒绝候选重新加入候选池。
5. 状态缺失和状态过期不能等价处理：缺失可低置信探索，过期的历史熔断应先触发恢复探测。

### 4.3 建议实现

引入统一的候选资格结果：

```go
type SchedulerEligibility struct {
    AccountID      int64
    HealthKey      OpenAISchedulerHealthKey
    Eligible       bool
    RecoveryOnly   bool
    Confidence     string
    RejectionCode  string
    SnapshotAge    time.Duration
}
```

所有 sticky、排名和负载均衡逻辑只能消费该结果，禁止再次自行解释原始 `state`。

建议增加统一判断函数：

```text
EvaluateCandidateEligibility(request, account, healthSnapshot, now)
```

返回的拒绝原因至少包含：

- `circuit_open`
- `half_open_recovery_only`
- `rate_limited`
- `runtime_blocked`
- `health_stale_recovery_required`
- `model_unsupported`
- `price_guard_rejected`
- `capacity_exhausted`

### 4.4 状态缺失和过期时的 sticky 策略

| 状态 | 普通新会话 | 普通 sticky session | `previous_response_id` |
| --- | --- | --- | --- |
| 新鲜 `running` | 正常参与 | 可保留亲和 | 满足协议约束时保留 |
| 新鲜 `observing` | 降权参与 | 达到逃逸阈值时迁移 | 除硬错误外保持 |
| 新鲜 `open` | 拒绝 | 强制逃逸 | 返回明确不可迁移错误或按现有协议处理 |
| 新鲜 `half_open` | 仅恢复流量 | 强制逃逸 | 不使用普通业务流量恢复 |
| 快照缺失 | 低置信探索 | 不复用旧 sticky，重新选择 | 按协议最小兼容处理 |
| 快照过期且历史为 `open` | 先恢复探测 | 清除 sticky 并重选 | 不直接视为健康 |
| 快照过期且历史为 `running` | 低置信参与 | 可重选，不保证原亲和 | 按协议处理 |

### 4.5 测试要求

- sticky 指向有效 `open` 时必须逃逸。
- sticky 指向 `half_open` 时不得获得普通业务流量。
- sticky 指向已过期历史 `open` 时必须触发恢复路径，不得直接当作普通 `running`。
- 健康存储失败回退 Legacy 时，已有 runtime block 和账号级限流仍必须生效。
- sticky bonus 无论多高都不能改变硬拒绝结果。
- 价格、配额、模型和 transport 硬过滤应先于 sticky。

## 5. 问题二：低置信度账号不得永久饥饿

### 5.1 当前实现限制

当前实现先截取 `eligible[:top_k]`，再在 Top-K 内计算 Softmax 和探索率。因此：

- `exploration_rate=0.03` 或 `0.05` 只会拉平 Top-K 内部份额。
- Top-K 之外的低置信账号目标份额仍然为 0。
- `low_confidence_max_share` 是上限，不是最小保障。
- 当 Top-K 全部都是低置信候选时，现有算法会因上限总和不足 100% 而重新归一化，实际单账号份额可能超过配置上限。
- 账号如果长期进不了 Top-K，就无法通过真实流量提升置信度。
- 当前后台 probe 能补充数据，但固定探测模型和有限 worker 不能替代按请求维度的最小探索。

因此，仅把 `low_confidence_max_share` 从 0.06 调到 0.10，不能完整解决饥饿问题。

### 5.2 推荐的双池分配模型

将候选拆成两个池：

```text
Exploitation Pool
  已有可靠健康样本，按综合效用选 Top-K

Exploration Pool
  健康缺失、过期或样本不足，但未触发硬拒绝
```

总体流量分成：

```text
normal_budget = 1 - exploration_budget
exploration_budget = 3% ~ 8%
```

建议第一版：

- 全局探索预算：5%。
- 单个低置信账号份额上限：10%。
- 单个低置信账号正常情况下目标份额：不超过 1%。
- 每账号每健康维度至少 1 次/10 分钟探索机会。
- 每账号每小时最多 6 次真实探索，剩余置信度通过 probe 补充。
- `open`、`half_open`、`unsupported`、`auth_failed`、价格保护失败的账号不进入探索池。

### 5.3 探索债务

建议为每个健康维度维护探索债务：

```text
exploration_debt = expected_min_samples - recent_valid_samples
```

Redis 示例键：

```text
openai:scheduler:exploration:{group_id}:{account_id}:{model_family}:{endpoint}:{transport}:{hour}
```

调度时：

1. 计算 Top-K 正常池。
2. 计算所有非硬拒绝低置信候选的探索债务。
3. 在探索预算内，优先选择债务最高且最久未探索的账号。
4. 成功或失败都消耗一次探索机会，避免故障账号被连续冲击。
5. 达到最小有效样本后退出探索池，进入正常综合评分。

### 5.4 自适应 Top-K

固定 Top-K 可改为基于有效候选数量动态计算：

| 有效候选数 | 建议 Top-K |
| ---: | ---: |
| 1-2 | 等于候选数 |
| 3-5 | 3 |
| 6-10 | 4 |
| 11-20 | 5 |
| 20 以上 | 6，或按容量进一步裁剪 |

当前分组约 10 个候选，推荐 Top-K 4。

Top-K 之外的账号仍可进入探索池和容量回退尾部，但不能绕过熔断或硬过滤。

### 5.5 自适应低置信度上限

静态 `low_confidence_max_share=0.10` 作为第一阶段配置即可。代码阶段建议按候选数量动态限制：

```text
1 个低置信候选：max 15%
2-3 个低置信候选：每个 max 10%
4 个以上低置信候选：每个 max 5%，总探索预算仍不超过 8%
```

注意：每账号上限与总探索预算是两个不同约束。即使单账号上限为 10%，总探索预算为 5% 时，所有低置信账号合计仍不能超过 5%。

### 5.6 Probe 与真实探索分工

优先使用 probe 补充以下数据：

- 冷账号基础连通性。
- half-open 恢复。
- 账号-模型能力确认。
- endpoint 和 transport 能力确认。

真实请求探索只用于 probe 无法代表的指标：

- 真实请求 TTFT。
- 真实并发和排队表现。
- 长上下文或复杂工具调用表现。
- 实际模型路由和映射结果。

探测模型必须逐步从单一 `gpt-5.5` 改为实际模型族或已知兼容的轻量模型，且探测状态必须保留 endpoint 和 transport 维度。

### 5.7 测试要求

- Top-K 外低置信账号在探索债务存在时能获得受限流量。
- `exploration_budget=0` 时不得产生 Top-K 外真实探索流量，但后台 probe 和 Top-K 内 `exploration_rate` 仍按各自配置运行。
- 所有低置信账号的总份额不超过全局探索预算。
- 单账号份额不超过低置信度上限。
- `open`、`half_open` 和硬拒绝账号不会因探索债务重新加入。
- 固定随机种子下结果可重复；长时间统计结果符合目标份额。
- 多实例下 Redis 计数必须原子，不能重复发放探索额度。
- 账号恢复为高置信后应及时退出探索池。

## 6. 推荐实施顺序

### 阶段 A：立即配置调整

1. 明确当前设置是全局配置，列出全部已启用自动调度的分组。
2. 确认这些分组可以共同采用推荐参数；否则先实现分组级覆盖，不直接保存全局配置。
3. 记录修改前配置、时间点和主要运行指标。
4. 观察至少 24 小时，不同时修改综合权重。
5. 达到回滚条件时恢复本文件第 2 节基线配置。

### 阶段 B：正确性修复

1. 统一账号页和实际调度的有效状态源。
2. 修复旧 score state nullable 字段无法清空的问题。
3. 区分 400、403、429、5xx、网络错误和请求错误。
4. 补齐 sticky 在 stale、missing 和 Legacy fallback 下的硬边界。

### 阶段 C：公平性优化

1. 实现探索池和全局探索预算。
2. 实现探索债务和分布式额度。
3. 实现自适应 Top-K 和低置信度份额上限。
4. 将 probe 改为按模型族、端点和 transport 执行。

### 阶段 D：灰度与推广

1. 先启用 Shadow 对比目标份额和实际份额。
2. 选择低风险分组真实灰度。
3. 达标后推广到其他自动调度分组。
4. 停止旧 score state 参与决策，仅保留历史审计。

### 6.1 建议新增配置项

代码阶段建议增加以下配置，避免继续复用含义不匹配的现有字段：

| 配置项 | 建议默认值 | 含义 |
| --- | ---: | --- |
| `adaptive_top_k_enabled` | `true` | 按有效候选数量计算 Top-K |
| `exploration_budget` | `0.05` | 所有低置信账号合计可使用的真实探索预算 |
| `exploration_min_interval_seconds` | `600` | 同一健康维度两次真实探索的最小间隔 |
| `exploration_max_real_samples_per_hour` | `6` | 单账号单健康维度每小时真实探索上限 |
| `stale_open_requires_probe` | `true` | 过期的历史熔断不得直接按普通低置信候选放行 |
| `half_open_max_inflight` | `1` | half-open 恢复流量的单飞并发限制 |

`exploration_rate` 保留为正常 Top-K 内的概率平滑参数；`exploration_budget` 专门控制 Top-K 外低置信候选，两者不能继续共用一个字段。

### 6.2 主要代码落点

| 文件 | 建议职责 |
| --- | --- |
| `backend/internal/service/openai_scheduler_policy.go` | 独立探索池、自适应 Top-K、硬份额边界 |
| `backend/internal/service/openai_balanced_scheduler.go` | 消费统一资格结果，不重复解释健康状态 |
| `backend/internal/service/openai_account_scheduler.go` | sticky、previous response、重试与熔断不变量 |
| `backend/internal/service/openai_scheduler_health_score.go` | stale/open/half-open 的统一有效状态 |
| `backend/internal/service/openai_auto_scheduler_probe_runner.go` | half-open 优先探测和按健康维度探测 |
| `backend/internal/service/openai_auto_scheduler_types.go` | 新配置项、资格结果和探索策略类型 |
| `backend/internal/repository` 下调度缓存实现 | Redis 探索额度、单飞锁和原子计数 |
| `backend/internal/service/openai_scheduler_ranking_service.go` | 展示正常份额、探索份额和混合资格状态 |
| `frontend/src/components/admin/openai-scheduler` | 新配置项、状态来源和决策解释 |

## 7. 观测指标与验收标准

配置调整后至少观察以下指标：

| 指标 | 建议目标 |
| --- | --- |
| 单账号实际份额 | 正常多候选时不超过 60% |
| Top-2 合计份额 | 不长期超过 85% |
| 低置信账号获得样本时间 | 30 分钟内至少获得 probe 或受限真实样本 |
| sticky 熔断违规选择 | 0 |
| circuit open 后恢复时间 | P95 小于 6 分钟 |
| `health_stale` 占比 | 持续下降，不长期增长 |
| 403 导致的账号级熔断 | 完成错误分类后显著下降 |
| 目标份额与实际份额偏差 | 1 小时窗口内主要账号绝对偏差小于 10% |
| 429、5xx、TTFT P90 | 不因探索调整显著恶化 |

建议增加以下决策指标：

- `sticky_escape_total{reason}`
- `scheduler_rejected_total{reason}`
- `scheduler_exploration_total{source}`
- `scheduler_low_confidence_accounts`
- `scheduler_target_actual_share_gap`
- `scheduler_circuit_recovery_seconds`
- `scheduler_health_snapshot_age_seconds`

## 8. 回滚条件

发生以下任一情况时，应恢复原配置并停止扩大灰度：

- 5xx 或请求失败率较基线上升超过 20%。
- TTFT P90 较基线上升超过 20%。
- 429 比例明显上升且持续 15 分钟。
- 低置信探索造成某一账号连续异常。
- 单账号份额约束导致全池排队或无可用候选增加。
- sticky 会话迁移率异常升高。

回滚只恢复参数，不删除健康状态、事件和使用记录。代码阶段的状态迁移必须提供独立回滚方案。

## 9. 最终结论

当前最适合的参数方向是：将 Top-K 从 2 提高到 4，将低置信度上限恢复到 10%，把探索率提高到 5%，并通过温度和最大份额降低头部集中。同时调整冷却、探测周期和真实样本新鲜期，使恢复时间与控制台配置更一致。

修改前的版本不能仅靠旧参数实现最低探索量，因为旧探索只发生在 Top-K 内。本次源码修改已新增独立探索预算、统一资格判断、基于真实样本年龄的探索债务、跨实例 Redis 探索额度、账号页统一健康聚合、按 HTTP/SSE 健康维度的恢复 probe，以及控制台运行计数；结构化上游错误码和 WebSocket 专用探测仍属于后续增强。

## 10. 开发实施方案

### 10.1 实施范围和状态

本次源码修改交付可安全灰度的第一版，包含跨实例 Redis 原子探索额度；WebSocket 专用恢复探测和结构化上游错误码仍保留为后续增强。

| 能力 | 本次状态 | 实现说明 |
| --- | --- | --- |
| sticky 不覆盖新鲜 `open/half_open` | 已完成 | 所有排序和 sticky 先消费统一资格结果 |
| 过期历史 `open/half_open` 不直接放行 | 已完成 | 默认返回 `health_stale_recovery_required`，只允许后续 probe 恢复 |
| 健康快照缺失和过期语义分离 | 已完成 | 候选显式携带 `fresh/missing/stale` |
| Top-K 外低置信探索 | 已完成 | 独立 `exploration_budget`，不再依赖 Top-K 内 `exploration_rate` |
| 基于最后真实样本的探索债务 | 已完成 | 无真实样本优先，其次按样本年龄排序；最小间隔可配置 |
| 自适应 Top-K | 已完成 | 以配置 Top-K 为下限，按有效正常候选数扩到 3 至 6 |
| 动态低置信单账号上限 | 已完成 | 1 个最多 15%，2-3 个最多 10%，4 个以上最多 5%，同时受配置上限约束 |
| 4xx 错误粗分类修复 | 已完成第一版 | 400/404/409/413/422 和明确内容策略 403 记为 `request_error`，不触发账号熔断 |
| 控制台配置闭环 | 已完成 | 后端 JSON、校验、运行映射、前端表单和中英文文案已同步 |
| 每账号每健康维度每小时最多 N 次真实探索 | 已完成 | Redis Lua 原子预留，默认 N=6；Redis 异常时探索失败关闭 |
| 账号管理页完全切换统一健康源 | 已完成 | 按账号聚合模型族、endpoint、transport；统一读取失败才回退旧评分 |
| probe 按模型族和 transport 执行 | 已完成第一版 | 对 HTTP/SSE Responses/Chat 维度按健康表恢复探测；WebSocket 不冒充 HTTP 探测 |
| 调度运行计数 | 已完成第一版 | 控制台展示当前实例自启动累计的探索放行、额度拒绝、Redis 错误、低置信降级和统一健康回退 |

### 10.2 新增配置契约

| 配置项 | 默认值 | 合法范围 | 运行语义 |
| --- | ---: | ---: | --- |
| `adaptive_top_k_enabled` | `true` | 布尔 | 允许候选较多时把基础 Top-K 向上扩展 |
| `exploration_budget` | `0.05` | `0-0.10` | 正常候选存在时，所有 Top-K 外低置信账号合计真实流量预算 |
| `exploration_min_interval_seconds` | `600` | `30-3600` | 同一健康维度获得真实样本后，再次进入探索池前的最短间隔 |
| `exploration_max_real_samples_per_hour` | `6` | `1-60` | 同一账号健康维度每小时真实探索硬上限，跨实例共享 |
| `stale_open_requires_probe` | `true` | 布尔 | 过期历史熔断必须通过恢复探测，不得被普通请求或 sticky 激活 |

旧配置 JSON 不包含这些字段时，反序列化自动使用上述默认值。保存配置后会写入完整 JSON；不需要数据库 schema 迁移。

### 10.3 请求决策数据流

```text
候选账号硬过滤
  -> 批量加载统一健康快照
  -> 标记 fresh / missing / stale
  -> EvaluateOpenAISchedulerCandidateEligibility
       -> hard rejected / recovery only / low confidence / eligible
  -> 正常池按综合效用排序并计算自适应 Top-K
  -> 低置信池按探索债务和最小间隔筛选
  -> 95% 正常预算 + 5% 探索预算（默认）联合加权抽样
  -> 真实探索候选在并发槽位成功后原子预留 Redis 间隔/小时额度
  -> sticky 只在 eligible 候选之间调整顺序
  -> 并发槽位获取和原有 fallback
```

资格判断是唯一解释健康状态的位置。Balanced 排名、普通 session sticky、软 sticky 逃逸和最终 DB 重检不得再次自行定义 `open/half_open/stale` 语义。

统一健康聚合规则：账号存在至少一个新鲜 `running/observing` 维度时，账号页整体不得显示熔断；存在可用和不可用维度时显示 `observing`；所有新鲜维度都不可用时显示 `open/half_open`；全部维度缺失或过期时显示 `observing`，并通过 `status_source=unified_health`、可用数、维度总数和过期数向前端解释状态。统一健康批量查询失败时才回退旧评分，并记录结构化回退告警。

分布式探索额度先将 `account_id:model_family:endpoint:transport` 归一化后计算 SHA-256 摘要，取前 12 字节十六进制作为 Redis Cluster hash tag，再使用两个 key：`openai:scheduler:exploration:{digest}:interval` 和 `openai:scheduler:exploration:{digest}:window`。Lua 脚本在同一事务内检查最小间隔和一小时窗口，只有两个条件都满足才写入间隔键并递增小时计数；TTL 分别覆盖最小间隔和一小时窗口。Redis 错误按 fail-closed 处理，只跳过该探索候选，不阻断正常候选。

### 10.4 分配算法

正常池只包含高置信且未触发硬拒绝的候选。有效 Top-K 计算如下：

| 正常候选数 | 自适应值 |
| ---: | ---: |
| 1-2 | 候选数 |
| 3-5 | 3 |
| 6-10 | 4 |
| 11-20 | 5 |
| 20 以上 | 6 |

最终 Top-K 为 `max(配置 top_k, 自适应值)`，再裁剪到候选总数。这样当前显式设置的较大 Top-K 不会被自动策略降低。

低置信探索池只接收以下候选：

- 没有有效健康样本或快照已过期。
- 未触发 `open`、`half_open`、runtime block、能力、价格或容量硬拒绝。
- 没有真实样本，或最后真实样本年龄达到 `exploration_min_interval_seconds`。

当正常池存在时，正常目标份额乘以 `1 - exploration_budget`，探索池平分经过单账号上限校正后的预算。Top-K 外剩余候选仍保留在容量回退尾部，但目标份额为 0。

当全池都是低置信账号时，系统进入显式可用性降级模式：在低置信候选中选取有效 Top-K 并分配 100% 份额。此时“保持服务可用”和“探索总份额不超过 5%”无法同时满足，本实现选择前者；监控和排名必须将其标记为 `fallback`，灰度验收时需单独统计。

### 10.5 4xx 错误归因

新增中性事件 `request_error`：

- 400、404、409、413、422 默认属于请求参数、资源或内容错误。
- 403 只有在错误消息明确包含 `content policy`、`safety system`、`policy violation` 或 `moderation` 时属于请求错误。
- 其余 401/403 仍视为账号凭证或权限错误。
- 429 保持 `rate_limited`，5xx 和网络错误保持账号错误。

`request_error` 保留审计事件和状态码，但不增加连续账号错误、不打开 circuit，也不写入统一健康错误 EWMA。后续阶段应把结构化上游 error code 一并传入分类器，减少对文本关键字的依赖。

至少应暴露以下观测指标或等价结构化事件：探索额度允许次数、最小间隔拒绝次数、一小时上限拒绝次数、Redis 错误次数、统一健康回退旧评分次数、账号页统一健康聚合维度数，以及 `fallback` 全低置信降级请求数。控制台的运行计数是当前实例自启动后的累计值，不代表数据库时间窗口，也不代表多实例全局总和；多实例汇总应以结构化日志或后续 Prometheus/Redis 聚合为准。指标应包含分组、账号、模型族、endpoint、transport 和 traffic class 标签，但不得包含 Token 或完整凭据。

### 10.6 代码落点

| 文件 | 本次职责 |
| --- | --- |
| `backend/internal/service/openai_auto_scheduler_types.go` | 新配置和 `request_error` 契约、默认值、向后兼容反序列化 |
| `backend/internal/service/openai_scheduler_policy.go` | 统一资格、双池分配、自适应 Top-K、探索债务和动态上限 |
| `backend/internal/service/openai_balanced_scheduler.go` | 快照语义保留、sticky 前置资格判断和运行配置映射 |
| `backend/internal/service/openai_account_scheduler.go` | 普通 sticky/软 sticky 最终熔断复检 |
| `backend/internal/service/openai_gateway_scheduling.go` | 4xx 请求错误和账号错误分类 |
| `backend/internal/service/openai_scheduler_health_score.go` | 中性请求错误不污染健康状态 |
| `backend/internal/handler/admin/openai_auto_scheduler_handler.go` | 新配置范围校验 |
| `backend/internal/service/openai_scheduler_overview_service.go` | 将当前实例调度运行计数并入控制台 overview |
| `backend/internal/repository/openai_scheduler_exploration_cache.go` | Redis Lua 探索额度结果分类和结构化事件 |
| `frontend/src/components/admin/openai-scheduler` | 新配置表单和事件展示 |

### 10.7 测试矩阵

必须自动验证：

1. 新鲜 `open/half_open` 账号不进入选择顺序。
2. sticky 指向过期历史 `open` 时返回 `health_stale_recovery_required`，且账号不进入 fallback。
3. 正常候选存在时，低置信探索总份额不超过 `exploration_budget`。
4. 低置信单账号份额不超过配置上限和动态上限。
5. 刚获得真实样本的低置信账号在最小间隔内目标份额为 0。
6. 十个正常候选、基础 Top-K 为 2 时，自适应结果为 4。
7. 固定随机种子产生可重复的联合加权顺序。
8. 请求侧 4xx 不增加账号连续错误，凭证/权限 403 仍增加错误。
9. 旧配置 JSON 能补齐新默认值，完整新配置能原样保存和读取。
10. 前端配置校验范围与后端完全一致。
11. 探索候选额度被拒绝后继续尝试正常候选，且 fresh load 重建不丢失 exploration traffic class 和 health key。
12. 统一健康聚合按可用、不可用、过期、缺失维度输出稳定状态；批量读取异常时才使用旧评分。
13. Redis Lua 在最小间隔和小时上限边界下原子拒绝重复预留，跨实例共享同一额度。

### 10.8 灰度和回滚

1. 部署代码但保持 `shadow_mode=true`，确认新配置已读取、资格原因和目标份额正常。
2. 检查所有启用自动调度的分组，因为配置仍是全局级。
3. 选择低风险分组退出 Shadow，观察至少 24 小时。
4. 重点对比 sticky 逃逸、无可用账号、低置信真实请求、429、5xx、TTFT P90 和单账号份额。
5. 异常时先设置 `adaptive_top_k_enabled=false`、`exploration_budget=0`、`stale_open_requires_probe=false` 回到接近旧分配语义；必要时将模式切回 `legacy`。Redis 不可用时系统会自动关闭真实探索，正常候选仍可继续服务。
6. 回滚代码不删除统一健康状态和审计事件；新增 JSON 字段会被旧版本忽略。

`stale_open_requires_probe=false` 只用于紧急兼容回滚。它会重新允许过期历史熔断作为低置信候选参与，不能作为长期配置。

## 11. 2026-07-21 置信度与决策审计实施记录

### 11.1 本阶段完成内容

- 统一实时选择、sticky 复检、熔断预检和管理端排名的置信度口径：仅 `running` 且存在新鲜真实样本时为 `high`；只有探测样本或真实样本过期时为 `medium`；`observing/open/half_open`、过期快照和无样本快照为 `low`。
- `medium/low` 候选统一进入低置信池，受探索额度和 `low_confidence_max_share` 约束，不再因探测刷新 `updated_at` 而被误认为具有新鲜真实样本。
- 管理端健康查询显式读取 `last_real_at`；排名服务使用它判断真实样本新鲜度。
- 正常账号的延迟基线只使用可参与正常流量的高置信候选，避免低置信账号的异常低预测 TTFT 把正常账号误判为延迟尾部。
- 新增 `openai_scheduler_decision_audits` 表，记录影子决策、探索放行、探索拒绝原因、探索异常和低置信兜底实际选择，并保存当时的分组、账号、健康维度、资格、流量类别、目标份额和关键参数快照。
- 审计通过 4096 容量的有界异步队列写入，单次数据库写入超时 2 秒；队列满、写入失败或服务停止期间只丢失审计，不延迟或中断用户请求。进程停止时在现有清理超时内排空队列。

### 11.2 数据库兼容性和回滚

- 迁移 `187_openai_scheduler_decision_audits.sql` 只新增审计表和查询索引，不修改已有表、数据或接口契约。
- 代码回滚后新增表可以保留，不影响旧版本运行；如需清理，应在确认不再需要历史审计后单独执行，不把删表纳入应用回滚。
- 审计不可用不会影响调度主链路，但会降低影子观察的完整性，应同时监控写入失败和队列丢弃日志。

### 11.3 验证结果

已执行并通过：

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./... -count=1
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestOpenAISchedulerDecisionAuditRecorder' -count=1
```

覆盖内容包括置信度分类、`observing`/仅探测样本进入低置信池、正常延迟基线隔离、影子决策审计、探索拒绝原因、显式 SQL 字段、迁移结构、Wire 注入和并发写入安全。

### 11.4 发布边界

本阶段只完成源码、迁移和测试，未修改生产配置、生产数据库或生产实例。发布后继续保持影子模式，至少观察 6 至 12 小时，并用审计表核对影子选择、探索放行/拒绝、低置信兜底和目标份额，再决定是否扩大真实流量。
