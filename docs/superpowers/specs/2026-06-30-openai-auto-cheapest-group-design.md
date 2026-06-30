# OpenAI 自动选择最优惠分组设计

日期：2026-06-30

## 结论

本需求为用户 API Key 增加一种 OpenAI 专用的自动分组模式。用户创建或编辑密钥时可以选择“OpenAI 自动选择最优惠分组”。选择后，系统不通过定时任务修改 API Key 的固定分组，而是在每次 OpenAI 请求进入网关时动态解析本次实际生效分组。

职责边界如下：

1. 跨分组由自动最优惠分组负责：按用户当前可用 OpenAI 分组的实际倍率从低到高尝试。
2. 组内继续复用现有 OpenAI 选号逻辑：可调度账号过滤、模型支持、渠道限制、runtime block、并发槽、sticky session、OpenAI 自动调度评分和熔断。
3. 第一个能成功选出账号的分组就是本次 `effective_group_id`。
4. 本次请求的计费、usage log、账号统计、sticky session 和前端“当前生效分组”都使用 `effective_group_id`。

## 目标

- 用户不再需要手动在多个 OpenAI 低价/高价分组之间切换。
- 低价分组账号池不可用时，系统自动递增到更高倍率分组。
- 复用现有 OpenAI 自动调度的账号健康、评分和熔断状态，不新增第二套健康状态。
- 保持固定分组 API Key 的现有行为不变。
- 自动模式的实际计费和日志归属可追踪、可解释。

## 非目标

- 不支持 Anthropic、Gemini、Antigravity、Grok 等平台的自动跨分组选择。
- 不用定时任务批量修改用户 API Key 的 `group_id`。
- 不改变现有 OpenAI 自动调度的组内排序、评分、熔断规则。
- 不把订阅购买、分组授权、管理员价格配置规则合并到本功能中重做。
- 不让低价分组绕过现有模型、渠道、能力和账号状态限制。

## 当前项目基础

用户侧创建/编辑 API Key 的分组选择在 `frontend/src/views/user/KeysView.vue`，选项由 `frontend/src/views/user/keyGroupOptions.ts` 从 `/groups/available` 返回的分组构建。当前表单要求 `group_id` 非空，创建和更新请求都只提交单个固定分组。

后端 `api_keys` 表当前只有可空 `group_id` 字段。`APIKeyService.Create` 和 `APIKeyService.Update` 会对指定分组执行用户权限校验。`APIKeyService.GetAvailableGroups` 已经能返回用户当前可绑定的活跃分组，并处理标准分组、专属分组和订阅分组权限。

OpenAI 请求当前直接使用 API Key 上的 `GroupID` 参与选号。`OpenAIGatewayService.listSchedulableAccounts` 会按 `groupID` 查询候选账号；`selectAccountWithLoadAwareness` 和 `selectAccountForModelWithExclusions` 已经包含 sticky session、可调度过滤、模型支持、runtime block、并发、渠道限制和 OpenAI 自动调度排序。

OpenAI 自动调度已提供组内健康状态：

- `running`：正常参与调度。
- `observing`：可参与，但排序靠后。
- `open`：熔断中，冷却未结束时跳过。
- `half_open`：半开恢复，可参与恢复验证。

本设计复用这些状态作为组内账号是否可选的重要依据。

## 用户体验

### 创建/编辑密钥

分组下拉新增一个特殊选项：

```text
OpenAI 自动选择最优惠分组
```

显示建议：

- 该选项放在 OpenAI 分组区域顶部，或放在所有 OpenAI 分组之前。
- 图标使用现有 OpenAI 平台图标风格。
- 选中后，下方展示说明和当前生效信息：

```text
当前生效：plus特惠临时分组（0.1x）
```

如果还没有成功调用过：

```text
当前生效：等待首次调用
```

如果最近一次尝试所有候选分组都不可用：

```text
当前生效：暂无可用 OpenAI 分组
```

### 密钥列表

分组列需要区分固定模式和自动模式：

- 固定模式：继续显示原分组徽标。
- 自动模式：显示“自动最优惠”徽标，并展示最近生效分组。

建议文案：

```text
自动最优惠
最近生效：codex-plus渠道（0.15x）
```

## 数据模型

`api_keys` 建议新增字段：

```text
group_select_mode varchar(32) not null default 'fixed'
last_effective_group_id bigint null
last_effective_group_at timestamp null
```

`group_select_mode` 取值：

- `fixed`：固定分组，沿用现有 `group_id`。
- `openai_auto_cheapest`：OpenAI 自动选择最优惠分组。

字段语义：

- `group_id`：固定模式下为用户选择的固定分组；自动模式下允许为空，不作为本次实际计费分组。
- `last_effective_group_id`：最近一次自动模式成功命中的实际分组，仅用于展示和排障。
- `last_effective_group_at`：最近一次自动模式成功命中的时间。

兼容性：

- 迁移时现有 API Key 全部设为 `fixed`。
- 固定模式的行为不变。
- 如果 `group_select_mode` 为空或未知，后端按 `fixed` 处理，避免历史数据异常影响请求。

## 接口变更

### API Key 创建/更新

创建和更新请求增加：

```json
{
  "group_select_mode": "fixed"
}
```

规则：

- `fixed` 模式下，继续要求 `group_id` 非空，并校验用户能绑定该分组。
- `openai_auto_cheapest` 模式下，`group_id` 可为空；后端只允许该模式用于 OpenAI 自动分组，不接受普通平台自动模式。
- 用户切回 `fixed` 时必须重新提交合法 `group_id`。

### API Key 列表/详情 DTO

返回字段增加：

```json
{
  "group_select_mode": "openai_auto_cheapest",
  "last_effective_group_id": 82,
  "last_effective_group_at": "2026-06-30T15:45:00Z",
  "last_effective_group": {
    "id": 82,
    "name": "plus特惠临时分组",
    "platform": "openai",
    "rate_multiplier": 0.1
  }
}
```

`last_effective_group` 可通过浅层 DTO 返回，避免前端再发一次查询。

### 当前推荐分组接口

首期可以不单独新增实时推荐接口。前端先展示最近生效分组。

如果后续需要页面打开时就展示“当前推荐”，可新增只读接口：

```text
GET /api/v1/api-keys/:id/effective-group-preview
```

该接口只能作为预测展示，不承诺下一次请求一定使用该分组。

## 自动选组流程

请求进入 OpenAI 网关后，如果 API Key 是固定模式：

```text
沿用 apiKey.GroupID -> 现有 OpenAI 选号链路
```

如果 API Key 是 `openai_auto_cheapest`：

1. 读取用户当前可用 OpenAI 分组。
2. 过滤掉非 OpenAI、非 active、用户无权限、订阅无效的分组。
3. 计算每个分组的实际倍率：
   - 优先使用用户专属分组倍率。
   - 没有专属倍率时使用分组 `rate_multiplier`。
4. 按实际倍率升序排序；倍率相同时按 `sort_order`、`id` 稳定排序。
5. 逐个分组调用现有 OpenAI 选号逻辑。
6. 第一个成功返回账号选择结果的分组即为本次 `effective_group_id`。
7. 将本次请求上下文中的 API Key 分组视图切换到实际分组，后续计费、日志和统计使用实际分组。
8. 最佳努力更新 API Key 的 `last_effective_group_id` 和 `last_effective_group_at`。

伪流程：

```text
for group in sortedAvailableOpenAIGroups:
    selection = trySelectAccount(group.id, requestModel, requestCapability)
    if selection success:
        return effectiveGroup=group, selection

return no available OpenAI groups
```

这里的 `trySelectAccount` 必须复用现有 `SelectAccountWithLoadAwareness` / `SelectAccountWithSchedulerForCapability` 所覆盖的检查，而不是新写一套简化判断。

## 跳过规则

分组是否可用由“该分组内能否为当前请求选出账号”决定。以下情况会导致当前分组跳过：

- 分组内没有可调度账号。
- 所有账号都不支持当前模型。
- 所有账号都不支持当前 endpoint/capability，例如 compact、responses、chat completions 等能力要求。
- 账号处于 OpenAI 自动调度 `open` 且冷却未结束。
- 账号被 runtime block。
- 账号持久状态、手动调度开关、rate limit、overload、临时不可调度状态使其不可选。
- 渠道模型限制、渠道价格限制、privacy 要求等组内限制不满足。
- 并发槽不可获得，并且当前策略不允许等待或等待队列已满。

OpenAI 自动调度状态参与方式：

- `open`：熔断中，组内选号跳过。
- `half_open`：允许参与恢复验证。
- `running`：正常参与。
- `observing`：允许参与，但现有排序靠后。

如果一个低价分组的所有候选账号都被上述规则排除，自动选组继续尝试下一个更高倍率分组。

## 计费与日志

自动模式下，计费必须使用实际生效分组，而不是 API Key 原始 `group_id`。

需要统一替换为 `effective_group_id` 的位置：

- 用户分组倍率解析。
- 图片生成倍率解析。
- 订阅分组计费类型判断。
- usage log 的 `group_id`。
- 账号统计定价费用计算。
- OpenAI 自动调度结果记录。
- sticky session 的 group 维度 key。
- ops/routing 相关上下文字段。

如果请求最终所有分组都不可用，不更新 `last_effective_group_id`，但可以在日志中记录候选分组数量和最后失败原因，方便排障。

## Sticky Session

自动模式下 sticky session 必须按实际分组隔离：

```text
sticky_key = effective_group_id + session_hash
```

不能用“自动模式”本身作为 group key，否则可能把低价组的 sticky 账号错误复用到高价组，或把已跳过分组的账号继续命中。

当低价分组选不出账号并递增到高价分组时，高价分组拥有自己的 sticky 绑定。后续请求如果低价分组恢复，仍可重新回到低价分组。

## 轻量缓存

允许增加短 TTL 缓存，但缓存只优化候选列表，不替代实时选号。

建议缓存：

- 用户可用 OpenAI 分组列表。
- 分组实际倍率排序结果。

建议 TTL：

- 30 秒到 60 秒。

缓存失效来源：

- 用户订阅变化。
- 用户专属分组倍率变化。
- 用户 allowed group 变化。
- 分组状态、倍率、平台、排序变化。

不建议缓存：

- 账号是否可用。
- 账号是否熔断。
- 并发槽是否可获得。
- 某模型是否当前可选。

这些状态必须在请求时复用现有 OpenAI 选号链路实时判断。

## 错误处理

当所有候选分组都无法选出账号时，对用户返回与现有 OpenAI 无可用账号一致的错误语义。

内部日志建议包含：

- `api_key_id`
- `user_id`
- `group_select_mode`
- `candidate_group_ids`
- `attempted_group_ids`
- `selected_effective_group_id`
- `requested_model`
- `last_error`

对外错误不暴露全部内部账号池细节，避免泄露渠道配置。

## 前端实现要点

- `KeyGroupOption.value` 目前是 number，需要扩展为支持特殊 sentinel，例如字符串 `openai_auto_cheapest`。
- `buildKeyGroupOptions` 可在 OpenAI 分组列表前插入自动选项。
- `KeysView.vue` 的表单校验需要从“`group_id` 必填”改为“固定模式 `group_id` 必填，自动模式不要求”。
- 创建/更新请求需要同时提交 `group_select_mode`。
- 分组列和下拉已使用 `GroupBadge` / `GroupOptionItem`，自动选项需要单独渲染，避免把 sentinel 当真实 group。
- 搜索分组时，“自动选择最优惠分组”应能被搜索命中。

## 后端实现要点

- API Key service 增加模式校验，固定模式保持现有路径。
- 鉴权缓存快照增加 `group_select_mode`、`last_effective_group_id` 等必要字段，避免热路径反复查 API Key。
- 增加自动候选分组解析服务，复用 `GetAvailableGroups` 的权限判断思路，但应避免每次请求重复执行昂贵 DB 查询。
- OpenAI gateway 增加自动分组选择入口，在调用账号选择前解析出本次实际分组。
- 选中实际分组后构造请求级 API Key/Group 视图，避免后续计费代码继续读取原始自动 API Key 的空 `GroupID`。
- 更新 `last_effective_group_id` 必须最佳努力，不应阻塞用户请求成功返回。

## 测试计划

后端单元测试：

- 固定模式创建/更新行为保持不变。
- 自动模式创建时允许 `group_id` 为空。
- 自动模式只允许 OpenAI 自动选择，不影响其他平台。
- 候选分组按用户专属倍率优先排序。
- 低价分组无可调度账号时切到下一分组。
- 低价分组账号全部 OpenAI 自动调度 `open` 时切到下一分组。
- 低价分组模型不支持时切到下一分组。
- 所有候选分组不可用时返回无可用账号。
- 自动模式下 usage log 和计费使用 `effective_group_id`。
- 自动模式下 sticky session 按 `effective_group_id` 隔离。

前端单元测试：

- 分组下拉包含自动选项。
- 自动选项渲染图标、文案、描述和搜索结果正确。
- 自动模式提交 `group_select_mode=openai_auto_cheapest` 且不要求 `group_id`。
- 固定模式仍要求 `group_id`。
- 密钥列表正确展示自动模式和最近生效分组。

集成验证：

- 创建自动 API Key，低价组可用时命中最低价组。
- 手动让最低价组账号进入熔断或不可调度后，请求命中下一价位组。
- 恢复最低价组后，后续请求可重新回到最低价组。
- 核对 usage log、用户余额扣费和账号统计中的 group_id 一致。

## 风险与边界

- 自动模式会增加一次请求中跨分组尝试次数。需要限制候选分组数量，或在循环中尽早复用现有错误，避免极端情况下放大 DB/缓存压力。
- 如果前端展示“当前生效”使用最近一次成功结果，它不是对下一次请求的强承诺。文案应避免写成“下一次必用”。
- 用户权限或订阅过期后，最近生效分组可能不再可用。展示时可标注“最近生效”，实际请求必须重新校验权限。
- 自动模式下计费和日志如果漏掉某处仍使用原始 `apiKey.GroupID`，会造成账单归属错误。实现时应优先抽出 request effective group 上下文，减少遗漏。
- OpenAI 自动调度未启用的分组仍可参与自动选组，但组内只走现有基础调度，不使用评分熔断状态。这保持分组级开关语义不变。

## 待确认项

本设计已确认：

- 自动选择在每次调用时动态执行，不用定时任务修改 API Key 分组。
- 允许增加短 TTL 轻量缓存。
- 跳过条件复用现有 OpenAI 选号逻辑。
- OpenAI 自动调度中的 `open` 熔断状态作为组内账号不可用依据之一。

实现前仍建议确认：

- 自动候选范围是否为用户可用的全部 OpenAI 分组，还是只包含管理员标记为可自动选择的 OpenAI 分组。
- 自动选项是否允许订阅型 OpenAI 分组参与；如果允许，订阅过期时请求期跳过。
- 是否需要管理端增加“允许被自动最优惠选择”的分组开关，用于排除特殊用途 OpenAI 分组。
