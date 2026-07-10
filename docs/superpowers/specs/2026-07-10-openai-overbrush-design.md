# OpenAI API Key 超刷调度设计

日期：2026-07-10

## 背景

账号管理中存在 OpenAI API Key 导入账号。当前这类账号在请求上游返回 429 时，会进入既有限流处理：写入限流状态或运行时调度屏蔽，并在冷却窗口内不再被调度。

用户希望对“OpenAI API Key 导入账号，且未对接上游管理渠道”的账号增加“是否超刷”开关。开启后，账号遇到 429 不立即停止调度，而是在连续多次 429 后才回归原有“限流中、不再调度”的逻辑。

## 目标

- 在账号编辑弹窗中新增“是否超刷”开关。
- 开关只对 `platform=openai`、`type=apikey`、`credentials.upstream_admin_type` 为空的账号显示并生效。
- 超刷开启后，账号连续 429 次数未达到阈值前继续参与调度。
- 连续 429 达到全局阈值后，走原有 429 限流逻辑。
- 任意一次成功请求会清零该账号的连续 429 计数。
- 阈值在后台设置页面中全局配置，默认 10。

## 非目标

- 不支持 OpenAI OAuth / setup-token 账号。
- 不支持配置了 `upstream_admin_type=sub2api` 或 `upstream_admin_type=new-api` 的上游管理账号。
- 不把连续 429 计数持久化到数据库。
- 不改变现有 429 冷却时间、限流状态展示和清理逻辑。

## 数据与配置

账号级开关写入 `accounts.extra`：

```json
{
  "openai_overbrush_enabled": true
}
```

全局阈值写入 settings 表，建议新增 key：

```text
openai_overbrush_settings
```

对应 JSON：

```json
{
  "consecutive_429_threshold": 10
}
```

阈值默认 10。后台保存时建议限制为 1 到 100；读取旧配置或非法配置时回退到默认值或边界值。

## 前端设计

账号编辑弹窗 `EditAccountModal.vue`：

- 增加 `openaiOverbrushEnabled` 状态。
- 读取账号 `extra.openai_overbrush_enabled` 初始化开关。
- 保存时将开关写回 `updatePayload.extra`。
- 仅当账号满足以下条件时显示：
  - `account.platform === 'openai'`
  - `account.type === 'apikey'`
  - `credentials.upstream_admin_type` 为空

后台设置页 `SettingsView.vue`：

- 在现有“429 默认回避配置”附近增加“OpenAI 超刷设置”。
- 展示一个数字输入项：连续 429 阈值。
- 通过 admin settings API 读取和保存。

前端 API `frontend/src/api/admin/settings.ts`：

- 新增 `OpenAIOverbrushSettings` 类型。
- 新增 `getOpenAIOverbrushSettings` 和 `updateOpenAIOverbrushSettings`。

## 后端设计

新增 settings DTO、service 方法和 handler 路由：

- `OpenAIOverbrushSettings`
- `DefaultOpenAIOverbrushSettings()`
- `GetOpenAIOverbrushSettings`
- `SetOpenAIOverbrushSettings`
- `GET /api/v1/admin/settings/openai-overbrush`
- `PUT /api/v1/admin/settings/openai-overbrush`

运行时计数放在 `OpenAIGatewayService` 内存状态中，例如：

```go
openaiOverbrush429Counts sync.Map // accountID -> int
```

需要的辅助行为：

- 判断账号是否适用超刷：OpenAI + API Key + 未配置 upstream_admin_type + extra 开关开启。
- 429 时递增连续失败计数。
- 小于阈值时跳过原有账号限流和调度屏蔽逻辑。
- 达到阈值时清零或保留计数均可，但必须继续执行原有 429 限流逻辑；推荐清零，避免解除限流后再次立即被旧计数影响。
- 成功请求完成后清零该账号计数。

429 接入点应放在 `handleOpenAIAccountUpstreamError` 调用原有限流处理之前。成功清零应接入 OpenAI API Key 请求成功路径，覆盖 chat completions、responses、messages、images、embeddings 等已调用同一 gateway service 的成功响应路径。

## 行为示例

全局阈值为 10，账号 A 开启超刷：

1. A 第 1 到第 9 次连续返回 429：继续可调度，不进入限流中。
2. A 第 10 次连续返回 429：执行原有 429 限流逻辑，账号进入限流中。
3. A 在第 5 次 429 后有一次成功响应：计数清零，下一次 429 从 1 开始。

未开启超刷、非 OpenAI API Key、已配置上游管理类型的账号均保持现有行为。

## 风险与处理

- 风险：连续 429 计数放内存，服务重启会丢失。
  - 处理：这是运行时保护状态，不是业务状态；重启后重新计数可以接受。
- 风险：成功清零漏掉某个 OpenAI API Key 成功路径，会导致计数偏高。
  - 处理：优先抽出统一清零方法，并在主要 OpenAI gateway 成功路径补测试。
- 风险：超刷可能增加上游请求压力。
  - 处理：只对管理员显式开启的账号生效，并由全局阈值控制上限。

## 测试计划

后端：

- 超刷未开启时，OpenAI API Key 429 保持原有限流逻辑。
- 超刷开启且未达到阈值时，不调用 `SetRateLimited`，不写运行时 block。
- 超刷开启且达到阈值时，调用原有 429 限流逻辑。
- 成功响应后清零连续 429 计数。
- OAuth、setup-token、配置了 `upstream_admin_type` 的 API Key 不走超刷逻辑。
- settings 默认值、边界值和非法值归一化测试。

前端：

- 编辑弹窗仅在目标账号显示“是否超刷”。
- 开关读取和保存 `extra.openai_overbrush_enabled`。
- 设置页能读取、修改并保存全局阈值。
