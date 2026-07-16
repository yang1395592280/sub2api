# OpenAI 自动最优惠分组剩余优化方案

日期：2026-07-16  
状态：部分实施（Redis 短熔断与 half-open 已落地，跨 turn 与指标仍待补强）
适用分支：`custom-main`

关联文档：

- [OpenAI 调度与自动最优惠分组稳定性修复方案](./2026-07-16-openai-scheduler-and-auto-cheapest-reliability-fix.md)
- [OpenAI 自动选择最优惠分组设计](./2026-06-30-openai-auto-cheapest-group-design.md)

## 1. 本次调整

分组短熔断采用以下规则：

```text
失败窗口：60 秒
隔离阈值：同一分组连续累计 2 次可切换故障
冷却时间：60 秒
恢复成功阈值：2 次
```

这里的“连续失败 2 次”是服务器内部的分组健康计数，**不是客户端重连次数**，也不会主动要求用户重新连接。

用户可见行为：

| 场景 | 服务端行为 | 用户感受 |
| --- | --- | --- |
| 第一次低价组 502/503/524/网络错误，首 token 前 | 当前请求立即切换到下一账号或下一分组 | 通常无感，可能只增加少量路由耗时 |
| 第二次同组故障，发生在 60 秒窗口内 | 计数达到 2，写入 Redis cooldown | 当前请求继续切换；后续请求直接跳过故障组 |
| cooldown 期间请求 | 不尝试该分组，优先使用其他稳定分组 | 不会触发客户端重连 |
| 首 token 后流式断开 | 不拼接第二个账号，关闭当前流 | 客户端可能显示重新连接或请求失败 |
| 所有分组均失败 | 返回最终错误 | 客户端可能自行重试 |

目标是：**在首 token 前尽量让用户只看到一次最终响应；分组冷却是服务端内部动作，不应表现为“正在重新连接 1/5、2/5”。**

## 2. 当前已完成范围

前一阶段已经完成：

- Balanced 调度空候选 panic 修复。
- 自动最低价当前请求内的失败分组跳过。
- Responses、Chat Completions、Messages、Embeddings、Images、WebSocket 入口接入请求级失败状态。
- 相关 service/handler 测试通过。

当前请求级状态只存在于 request context，下一请求不会继承。因此还需要完成本方案的 Redis 分组短熔断。

当前实现已补充 Redis 分组短熔断：键按 `group_id + normalized_model + endpoint + transport` 区分，60 秒窗口内第 2 次失败进入 60 秒 cooldown；到期后只允许一个 half-open probe，连续成功 2 次才恢复。Redis 读写异常时降级放行，不会把 Redis 故障转为 502。

## 3. 剩余优化清单

### P0：Redis 分组短熔断

新增分组健康键：

```text
openai:auto-group:health:{group_id}:{model_family}:{endpoint}:{transport}
```

建议使用 Redis Hash 保存计数和状态，并为整个 key 设置 TTL：

```text
failure_count
window_started_at
last_failure_at
cooldown_until
state             # closed | open | half_open
half_open_probe   # 0 | 1
last_reason
```

要求：

1. 使用 Lua 或 Redis 事务保证“检查窗口、递增计数、达到阈值、写入 cooldown”原子完成。
2. 失败窗口超过 60 秒时，计数重置为 1，而不是继续累加历史失败。
3. 第 1 次失败只更新计数，不进入 cooldown。
4. 第 2 次失败将状态切换为 `open`，设置 `cooldown_until = now + 60s`。
5. cooldown 期间自动最低价选择直接跳过该分组。
6. Redis key 必须设置 TTL，最长不超过 `cooldown + failure_window + recovery_grace`。
7. Redis 不可用时不能阻塞请求，回退到当前请求级隔离和账号级健康调度，并记录告警。

### P0：半开恢复

cooldown 到期后，分组进入 `half_open`：

- 只允许一个请求获得 `half_open_probe` 锁。
- 探测请求成功，累计成功数加一。
- 连续成功达到 2 次，状态恢复为 `closed`，清除失败计数。
- 探测请求再次出现可切换故障，立即回到 `open`，重新冷却 60 秒。
- 没有 probe 锁的请求继续跳过该组，避免恢复瞬间被流量打满。

半开状态应在日志中明确区分“恢复探测”和正常请求，不应把 half-open 账号混入普通低价候选池。

### P1：WebSocket 按 turn 隔离

当前 WebSocket 是长连接，一个连接可以包含多个 turn。需要区分两类状态：

```text
turn_failed_groups       # 仅影响当前 turn
redis_group_cooldown     # 影响连接后续 turn 和其他请求
```

规则：

1. 某个 turn 第一次低价组失败，只跳过当前 turn 的该分组。
2. 如果当前 turn 第二次同组失败，写入 Redis cooldown。
3. 未达到 Redis cooldown 的单次失败，不应永久影响后续 turn。
4. `previous_response_id` 仍保持强粘性，只有原账号硬条件不满足时才允许迁移。
5. 已经开始向客户端发送语义内容后，不允许切换账号拼接输出。

这样可以避免一个临时网络错误让整个 WebSocket 连接后续所有 turn 都长期使用高价组。

### P1：补充跨分组集成测试

必须新增真实选择链路测试，而不是只测 Redis 辅助函数：

- 低价组账号 A 返回 502，高价组账号 B 成功，客户端最终只收到成功响应。
- 低价组账号 A 第一次失败后，同一请求不再尝试该组账号 C。
- 两个独立请求在 60 秒内让同一分组累计失败 2 次，第三个请求跳过该组。
- 失败间隔超过 60 秒，计数重置，不触发 cooldown。
- cooldown 到期后只有一个 half-open probe 请求进入该组。
- probe 成功 2 次后恢复正常价格排序。
- probe 失败后重新 cooldown。
- Redis 不可用时请求仍能完成账号级 failover。
- 固定分组 API Key 不受自动分组 cooldown 影响。
- `previous_response_id` 不因价格更低而迁移。
- 首 token 后断流不会拼接第二个账号输出。
- 客户端已经断开时不启动新的分组尝试。

### P1：结构化日志与指标

自动最低价请求需要增加以下字段：

```text
api_key_id
requested_model
effective_group_id
selected_group_id
failed_group_ids
group_failover_count
group_failover_reason
group_circuit_state
group_cooldown_until
account_switch_count
stream_started
client_disconnected
```

建议事件：

- `openai.auto_group.failover`
- `openai.auto_group.circuit_counted_failure`
- `openai.auto_group.circuit_opened`
- `openai.auto_group.cooldown_skipped`
- `openai.auto_group.half_open_probe`
- `openai.auto_group.recovered`
- `openai.auto_group.redis_degraded`

必须增加的指标：

- 自动最低价请求总数。
- 首选最低价分组占比。
- 当前请求分组升档次数和成功率。
- 分组 cooldown 次数。
- half-open probe 成功率。
- 自动最低价最终 `502` 率。
- 首 token 前 failover 成功率。
- 首 token 后流式中断率。
- `failover_aborted_client_disconnected` 次数。
- Redis 分组状态读写失败次数。
- 升档造成的实际倍率增幅。

### P2：usage 记录与管理端查看

如果 `usage_logs` 中不存在等价字段，新增：

```text
effective_group_id bigint
group_failover_count integer
group_failover_reason varchar(64)
```

管理端至少提供只读信息：

- 当前分组状态：`closed/open/half_open`。
- cooldown 剩余时间。
- 60 秒窗口失败次数。
- 最近失败原因。
- 最近恢复时间。
- 最近一次从低价组升档到的目标分组。

管理端不能直接编辑 Redis 计数；如需人工操作，应提供明确的“清除 cooldown”动作并记录审计日志。

## 4. 故障分类

进入分组失败计数的故障：

- `429`。
- `500`、`502`、`503`、`504`、`524`、`529`。
- TCP/TLS/代理连接错误。
- 首 token 前上游超时。
- 首 token 前连接被上游关闭。

不进入分组失败计数：

- 请求参数错误。
- 模型不支持。
- 上下文超限。
- 内容安全策略拒绝。
- 价格保护主动拒绝。
- 已经向客户端发送语义内容后的流式中断。

账号级健康统计仍应记录所有适用的真实失败，但分组级计数只记录能够证明“该分组短期不适合继续承接新请求”的故障。

## 5. 客户端无感边界

### 可以做到无感的情况

- 首 token 前上游返回 502/503/524。
- 首 token 前网络连接失败。
- 低价组失败但存在可用备用账号或备用分组。
- 服务端在客户端超时前完成下一次选择。

客户端只会看到服务端最终成功响应，可能增加一段内部路由耗时。

### 无法保证无感的情况

- 已经向客户端写出部分语义内容后上游断开。
- 所有候选分组都不可用。
- 客户端自身超时或主动断开。
- 上游响应耗时超过客户端重连/超时策略。

这类情况不能通过重复转发同一请求来强行恢复，否则可能导致重复生成、重复扣费或上下文不一致。

## 6. 配置与默认值

```json
{
  "auto_group_failure_window_seconds": 60,
  "auto_group_failure_threshold": 2,
  "auto_group_cooldown_seconds": 60,
  "auto_group_recovery_success_threshold": 2,
  "auto_group_half_open_probe_ttl_seconds": 15,
  "auto_group_failover_enabled": true,
  "auto_group_circuit_enabled": true
}
```

范围限制：

- `failure_threshold` 固定最小值为 2，避免误配置成单次故障即全组隔离。
- `failure_window` 允许 30 至 300 秒。
- `cooldown` 允许 30 至 600 秒。
- half-open probe TTL 允许 5 至 60 秒。
- Redis 状态不可用时自动降级，不等待 Redis 恢复。

## 7. 灰度与回滚

### 灰度顺序

1. 先只开启日志和计数，不实际跳过 cooldown 分组。
2. 对一个低风险自动最低价 API Key 开启实际跳过。
3. 观察至少一个完整高峰周期。
4. 按分组扩大范围。

### 观察指标

- 自动最低价最终 `502` 率必须下降。
- 首 token 前 failover 成功率必须上升。
- 客户端重连次数不能上升。
- 升档后的实际倍率增幅不能异常扩大。
- Redis degraded 次数不能持续增长。

### 回滚顺序

1. 关闭 `auto_group_circuit_enabled`，保留当前请求级分组跳过。
2. 关闭 `auto_group_failover_enabled`，回到原自动最低价顺序。
3. Balanced 回退到高级 Legacy。
4. 保留 panic 修复和日志，不回滚安全边界修复。

## 8. 验收标准

1. 同一分组 60 秒内第一次故障不会触发 Redis cooldown。
2. 同一分组 60 秒内第二次可切换故障后，第三个请求跳过该分组。
3. cooldown 期间客户端不会因为服务端隔离动作被要求重新连接。
4. cooldown 到期只放行一个 half-open probe。
5. probe 成功 2 次后分组恢复正常参与最低价排序。
6. 首 token 前存在备用分组时，最终 `502` 率明显下降。
7. WebSocket 单个 turn 的临时失败不会永久污染后续 turn。
8. 固定分组 API Key、计费、价格保护和 `previous_response_id` 语义无回归。
9. Redis 故障不会阻塞请求，也不会导致进程 panic。
