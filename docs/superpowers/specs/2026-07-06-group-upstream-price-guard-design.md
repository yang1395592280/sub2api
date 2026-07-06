# 分组级上游余额刷新与价格上限设计

日期：2026-07-06

## 背景

管理员希望单独建立一个福利分组。该分组可能只维护一个上游账号渠道，也可能后续临时加入备用账号。系统需要按分组定时刷新组内上游账号余额和上游分组价格，并在上游有效价格倍率超过福利分组配置的上限时停止调用对应账号。

当前项目已有基础能力：

1. 账号表已有 `channel_price`，用于 OpenAI 自动调度成本权重。
2. `OpenAIUpstreamBalanceService.Refresh` 已能调用上游 `/v1/usage`，读取上游余额、上游分组、上游分组倍率和有效倍率，并写入账号 `extra` 与 `channel_price`。
3. 账号已有 `temp_unschedulable_until` / `temp_unschedulable_reason`，真实调度会跳过临时不可调度账号。
4. 仓储已有 `ListUpstreamBalanceRefreshCandidates`，但当前没有分组级余额刷新 runner。

## 目标

1. 在分组上配置上游余额定时刷新策略。
2. 后台任务按分组扫描并刷新该分组下所有支持的上游账号。
3. 在分组上配置统一的上游价格倍率上限。
4. 组内账号刷新后，如果上游有效倍率超过分组上限，则停止调度该账号。
5. 后续刷新发现价格回到上限以内时，仅自动恢复由本价格策略暂停的账号，不恢复管理员手动停用账号。

## 非目标

1. 不自动创建福利分组；管理员仍通过现有分组管理创建。
2. 不强制福利分组只能有一个账号；策略按分组覆盖组内所有账号。
3. 不改变用户计费倍率、API Key 扣费或账单逻辑。
4. 不重写 OpenAI 自动调度评分模型。
5. 不支持每个账号单独设置价格上限，本期只做分组统一上限。

## 方案选择

### 方案 A：账号级配置

每个账号配置刷新开关和价格上限。优点是灵活，缺点是福利分组换账号时需要重复配置，也容易出现同组账号策略不一致。

### 方案 B：分组级配置

分组配置刷新开关、刷新间隔和价格上限。后台按分组获取组内账号并逐个刷新。优点是符合福利分组的业务语义，后续增删账号无需重复配置。

### 方案 C：复用自动调度配置

把价格上限并入 OpenAI 自动调度设置。优点是调度相关配置集中，缺点是会把“余额刷新/价格风控”绑定到 OpenAI 自动调度页面，且不适合 Anthropic API Key 上游余额刷新。

推荐方案 B。

## 数据模型

在 `groups` 增加分组级字段：

1. `upstream_balance_refresh_enabled`：是否启用分组级上游余额定时刷新，默认 `false`。
2. `upstream_balance_refresh_interval_seconds`：刷新间隔，默认 `600`，最小值建议 `60`。
3. `upstream_price_max_multiplier`：上游价格倍率上限，默认 `0` 表示不限制。

账号 `extra` 继续保存已有上游余额字段，并补充价格策略状态：

1. `upstream_price_guard_status`：`ok`、`blocked`、`unsupported`、`error`。
2. `upstream_price_guard_group_id`：触发策略的本地分组 ID。
3. `upstream_price_guard_max_multiplier`：触发时的分组上限快照。
4. `upstream_price_guard_actual_multiplier`：上游有效倍率快照。
5. `upstream_price_guard_checked_at`：最近检查时间。
6. `upstream_price_guard_error`：策略判断错误信息。

价格自动暂停使用现有 `temp_unschedulable_until` / `temp_unschedulable_reason`，原因前缀固定为 `upstream_price_guard:`。

## 后台流程

新增 `GroupUpstreamBalanceRefreshRunner`。

启动后按固定扫描间隔运行：

1. 查询启用了 `upstream_balance_refresh_enabled=true` 的 active 分组。
2. 按每个分组的 `upstream_balance_refresh_interval_seconds` 判断是否到期。
3. 查询当前分组下支持上游余额刷新的账号：OpenAI / Anthropic、API Key、active、未删除、存在 `base_url` 和 `api_key`。
4. 对每个账号调用现有 `OpenAIUpstreamBalanceService.Refresh`。
5. 刷新成功后读取账号 `channel_price` 或本次快照中的有效倍率，与分组 `upstream_price_max_multiplier` 比较。
6. 超过上限时，设置临时不可调度到远期时间，并写入 `upstream_price_guard: group_id=... actual=... max=...` 原因。
7. 未超过上限时，如果账号当前的临时不可调度原因是 `upstream_price_guard:`，则清除此临时不可调度状态。
8. 刷新失败只记录余额刷新错误和价格策略错误，不自动恢复，也不因未知价格直接暂停。

为避免多个分组重复刷新同一个账号，runner 以 `group_id + account_id` 作为本次任务粒度。若同一账号加入多个启用价格上限的分组，只要任一分组判定超限，该账号保持不可调度；恢复时只在没有任何分组超限时清除价格策略暂停。

## 停止调用与恢复语义

超过上限后的停止调用使用临时不可调度，而不是直接写 `schedulable=false`。

这样可以保留管理员手动开关语义：

1. 管理员手动设置 `schedulable=false` 后，价格恢复不会自动开启账号。
2. 只有 `temp_unschedulable_reason` 以 `upstream_price_guard:` 开头时，价格恢复才会自动清除。
3. 其他临时不可调度原因，例如 token refresh、上游错误、代理故障，不会被价格恢复逻辑清除。

临时不可调度时间建议设置为 `now + 24h`，每次刷新仍超限时续期。这样即使 runner 短时失败，也不会马上误恢复；下次刷新低于上限会主动清除。

## 管理端

分组创建和编辑表单增加：

1. 上游余额定时刷新开关。
2. 刷新间隔秒数。
3. 上游价格倍率上限。

账号列表已有上游余额展示，后续可在同一列或状态列展示价格策略状态：

1. 当前上游倍率。
2. 本地分组上限。
3. 是否因价格超限被临时暂停。

首期可以只做分组配置和账号状态可见性，不新增独立页面。

## 错误处理

1. 分组刷新配置非法时，后端保存时校验并返回错误。
2. 上游接口失败时，保留 `OpenAIUpstreamBalanceService` 已有错误记录。
3. 未获取到有效倍率时，不触发价格暂停，状态写为 `unsupported`。
4. 单个账号刷新失败不影响同组其他账号。
5. runner panic 或 context cancel 时停止当前批次，不影响下次扫描。

## 兼容性

新增分组字段默认关闭，不影响现有分组。

`upstream_price_max_multiplier=0` 表示不限制，只做余额刷新，不做价格阻断。

已有手动余额刷新接口继续可用。手动刷新可以同步执行价格检查，使管理员手动点击刷新后立即看到是否超限。

## 验证

后端测试：

1. 分组启用刷新后，runner 会刷新该分组下所有支持账号。
2. 上游有效倍率超过分组上限时，账号写入价格策略状态并进入临时不可调度。
3. 上游有效倍率降回上限以内时，仅清除 `upstream_price_guard:` 导致的临时不可调度。
4. 管理员手动 `schedulable=false` 不会被价格恢复逻辑改回 `true`。
5. 刷新失败时不误暂停，也不误恢复。

前端测试：

1. 分组创建和编辑表单能保存刷新开关、刷新间隔和价格上限。
2. 账号列表能展示价格策略状态。
3. 表单校验能拦截负数上限和过低刷新间隔。

