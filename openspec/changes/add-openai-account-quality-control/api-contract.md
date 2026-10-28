# 管理 API 契约（拟议）

版本：v1.3；日期：2026-10-28。**本地管理 API 已实现；本文记录设计契约，功能验收状态见 [verification.md](./verification.md#11-本地实现与验证-2026-10-28)。** 仅服务现有自动不降智模式；生产部署和启用不包含在本地实现内。

## 1. 公共约定

- 根路径：`/api/v1/admin/account-quality`。全部沿用现有后台鉴权，现有管理员可用；普通用户与受限管理 API Key 返回 403，不按可见页面代替服务端鉴权。
- JSON 使用 snake_case；ID 为正整数；所有时间为 RFC3339 UTC 字符串，展示时转换为后台时区；缺失用量为 null。
- 成功响应沿用 `response.Success/Created/Accepted`：`code=0`；失败沿用 `ErrorWithDetails`：`code=HTTP状态码`、稳定 `reason`、字符串值 `metadata`。前端依赖 reason，不依赖中文 message。
- 列表默认 `page=1&page_size=20`，最大 page_size=100；超出范围返回 400。按 id 倒序，策略列表可选 updated_at 倒序；固定排序加 ID 作为平局键。
- 所有更新使用 `expected_revision` 乐观锁，服务端生成新 revision，不允许客户端直接修改 state、计数、epoch、scope_hash、审计或预算字段。
- 批量创建/编辑最多 100 项，全部校验与落库为一个事务；运行任务最多 100 个策略，逐项返回结果。批量请求中的重复 ID 返回 400，不悄悄执行两次。
- 禁止未知请求字段，限制请求体建议 256 KiB；不接受凭据、任意上游 URL、任意评分脚本或用户自填正确答案。
- 能力查询、预览和保存不调用模型；只有提交运行任务才可能产生上游费用。
- routing_mode 只读固定为 openai_auto_cheapest；后台所有检测创建/执行均校验参与库存。公网质量门仅接受原始鉴权得出的自动不降智模式，普通固定分组不适用本需求控制。

分页样式：

```json
{"code":0,"message":"success","data":{"items":[],"total":0,"page":1,"page_size":20,"pages":1}}
```

## 2. 路由总表

以下路径均相对于根路径。

| 方法 | 路径 | 用途 | 成功状态 |
| --- | --- | --- | --- |
| GET | /settings | 全局配置、revision、运行健康摘要 | 200 |
| PUT | /settings | 更新全局配置 | 200 |
| POST | /settings/emergency-disable | 原子关闭自动/控制总开关，保留策略历史 | 200 |
| GET | /benchmarks | 内置题与评分版本、输出约束、启用能力 | 200 |
| GET | /targets | 自动不降智的可检测来源组/账号库存与资格 | 200 |
| POST | /capabilities | 批量解析账号/条件，说明可检测/可控制能力 | 200 |
| POST | /preview | 预览拟保存配置及影响、预算，不执行 | 200 |
| GET | /overview | 筛选范围的库存/策略概览、24H 统计与运行健康 | 200 |
| GET | /policies | 策略分页、筛选 | 200 |
| POST | /policies | 批量新建或原子替换策略 | 201 |
| GET | /policies/:id | 单策略、当前状态、控制与预算摘要 | 200 |
| GET | /policies/:id/monitor | 单策略当前范围的完整 24H 聚合与有限时间桶 | 200 |
| PUT | /policies/:id | 编辑规则/开关，不改检测条件 | 200 |
| POST | /policies/batch-update | 全部成功或全部失败的批量规则/开关编辑 | 200 |
| POST | /policies/:id/archive | 归档、停止控制/自动，协作取消未完成任务 | 200 |
| POST | /policies/:id/release | 人工解除质量限制，原因必填 | 200 |
| POST | /runs | 批量提交手动任务，服务端判定诊断或恢复用途 | 202 |
| GET | /runs | 按批次、策略、账号、时间、结果查询任务 | 200 |
| GET | /runs/:id | 单次详情，可按需返回回答正文 | 200 |
| POST | /runs/:id/cancel | 请求取消，不能承诺退还已发出调用的额度 | 200 |
| POST | /summaries | 账号列表批量摘要，避免 N+1 | 200 |
| GET | /events | 状态/操作事件分页 | 200 |

实现时固定路由须先于参数路由登记，避免 `batch-update` 被解析成策略 ID。归档后不提供硬删除或直接“复活”；重新创建新策略建立新证据。

## 3. 字段对象

### 3.1 condition（创建时必填，之后不可 PUT 改写）

策略创建同时要求顶层 source_group_id，指活跃且允许自动调度的 OpenAI 普通业务来源组；服务端验证账号属于该组或其有效引用号池。该字段用于复现分组模型/effort/tier 上下文，不能传入自建池 ID 或任意普通组来扩大检测范围。condition、source_group_id、account_id 变更均使用显式替换。

| 字段 | 类型/规则 |
| --- | --- |
| model_id | 非空字符串，≤255 字节；账号输入模型，服务端按真实规则映射 |
| protocol | responses / chat_completions；指有效上游协议 |
| transport | http / websocket_v2；后者首期只与 responses 配对 |
| reasoning_effort | upstream_default 或当前账号/模型能力支持的值；不能用硬编码全局枚举假定所有模型支持 |
| service_tier | upstream_default 或当前路由可发出的有效档位 |
| benchmark_id | 首期仅 candy |
| benchmark_version | 首期 1，对应需求中的 candy-v1 |
| scorer_version | 首期 strict-json-v1 |

`upstream_default` 是内部哨兵，含义为按当前链路省略字段，绝不能把该字符串发给上游。明确传入值在链路被强制映射后，以有效值建立范围；能力查询与预览必须返回 requested/effective 差异，保存时客户端须确认使用归一化后的配置。

### 3.2 rules（默认与范围见需求第 12 节）

```json
{
  "auto_check_enabled":false,
  "control_enabled":false,
  "interval_seconds":1800,
  "jitter_seconds":60,
  "failure_threshold":3,
  "recovery_threshold":2,
  "cooldown_seconds":1800,
  "recovery_interval_seconds":600,
  "evidence_window_seconds":7200,
  "timeout_seconds":180,
  "max_output_tokens":4096,
  "daily_call_limit":100
}
```

创建时缺省字段补代码默认值，日预算取当前全局 default_policy_daily_call_limit，并将结果作为策略快照保存；以后更改全局默认值不修改已有策略。更新使用完整 rules 对象，缺少字段返回 400，防止局部保存把已有控制意外重置。窗口须满足 `W >= max(I,R,(F-1)*I,(P-1)*R)`，符号定义见需求 §12；不满足返回 QUALITY_INVALID_CONFIG，metadata.field=evidence_window_seconds。`can_enforce=false` 不允许保存 `control_enabled=true`，也不能通过修改全局开关绕过此检查。

### 3.3 返回的状态对象

```json
{
  "state":"observing",
  "freshness":"fresh",
  "failure_streak":1,
  "recovery_streak":0,
  "last_run_id":901,
  "last_verdict":"incorrect",
  "last_checked_at":"2026-10-28T03:00:00Z",
  "last_valid_at":"2026-10-28T03:00:00Z",
  "evidence_expires_at":"2026-10-28T05:00:00Z",
  "cooldown_until":null,
  "control":{"effective":false,"reason":"global_control_disabled","action":"observe_only"},
  "next_run_at":null
}
```

state 是当前证据/控制代次的状态；旧代次快照投影为 untested，旧历史另外保留，不能直接把数据库旧 state 回显成当前已隔离。freshness 与 last_verdict 独立，最新 unknown 不覆盖最后有效答案时间；normal+stale 页面显示“证据过期/质量未知”。

策略及摘要另返回 routing_mode、source_group_id、compatible_source_group_ids、eligibility（eligible/out_of_scope/source_context_invalid）。out_of_scope/source_context_invalid 时暂停检测和控制，保留历史，不能显示“可立即复测”。新任务返回明确原因；历史查询、人工解除与归档仍可使用。资格变化不作为第六种质量状态。

control.action 为 observe_only / baseline / fallback / deny；effective 表示两层控制开关、当前范围、能力、代次均有效，不代表一定有流量。无策略账号在摘要中返回 configured=false。

## 4. 全局设置与紧急关闭

PUT `/settings` 请求必须包含 expected_revision 与完整可编辑设置：

```json
{
  "expected_revision":1,
  "automation_enabled":false,
  "control_enabled":false,
  "global_concurrency":3,
  "queue_limit":300,
  "queue_timeout_seconds":300,
  "global_daily_call_limit":1000,
  "default_policy_daily_call_limit":100,
  "raw_retention_days":7,
  "metadata_retention_days":30,
  "store_failure_policy":"fail_closed"
}
```

首期 store_failure_policy 仅接受评审最终选定值；当前建议 fail_closed，不提供运行中静默切换。GET 增加只读 revision/control_epoch、budget_timezone、today_consumed/reserved、queue_depth、running_count、last_sync_at、store_status。健康不可读返回明确 unavailable，不伪造零。

控制 false→true 建立新全局代次，既有观察结论不会立即隔离账号。预算/保留期变更不重写已发生费用；降低预算到已用数量以下时停止新调用，不报负余额。

更改 default_policy_daily_call_limit 只改变后续新策略的缺省值。降低并发/队列不终止既有执行/清空队列；降低预算在出站前重新校验未消费预留。自动开关关闭时取消未出站的自动/恢复任务，不取消手动诊断，不以停测解除隔离。完整变更矩阵见 [集成指南 §3](./implementation-guide.md#3-配置变更的生效规则)。

POST `/settings/emergency-disable`：`{"reason":"暂停质量控制排查"}`，reason trim 后 1–500 字；不要求客户端提供旧 revision，以免紧急动作被页面版本卡住。事务锁设置，原子关闭两个开关、递增 revision，返回关闭后的设置与目标传播期限。不清空历史、不打开原本停用的账号。数据库写失败返回 503，界面不能显示已停用；跨实例传播仍遵守依赖健康前提。

## 5. 能力查询与影响预览

GET `/targets` 使用标准分页；支持 source_group_id、account_id、search、eligibility；默认只列 eligible。返回来源业务组、实际物理号池、账号 ID/名称、参与资格/原因和 compatible_source_group_ids，不含凭据。库存只包含活跃允许自动调度的 OpenAI 业务来源及其有效号池；停止参与的已有策略可在 eligibility 筛选中显示。不是某用户最终候选清单，页面需说明实际授权/最高倍率仍按请求计算。

POST `/capabilities` 接收 `{"items":[{"account_id":101,"source_group_id":7,"condition":{...}}]}`，最多 100 项；返回每项：

- normalized_condition 与 effective_upstream_model、routing_fingerprint（非敏感标识）。
- can_probe、can_enforce、supports_output_limit、reasons。
- 不支持的参数、缺失的 WS/身份适配、账号基础限制分别给原因，不伪造支持。

POST `/preview` 的mode与载荷如下，预览不保存配置：

| mode | 载荷 | 用途 |
| --- | --- | --- |
| create | items，与POST /policies相同的创建/替换项 | 新策略和条件替换 |
| update | items，每项policy_id、expected_revision和完整rules | 单项或批量规则变更 |
| settings | settings，含expected_revision的完整PUT /settings对象 | 所有全局设置变更 |
| enable_global | settings，同上且control_enabled=true | 全局启控的专门预览入口 |

items/settings按mode互斥，不接受任意上游参数。请求结构合法时返回200 + valid、validation_errors、conflicts、warnings，以及可计算的daily_calls_estimate、global_budget、estimated_detection_coverage、可控制范围/缺失条件；valid=false不是保存成功。结构/类型错误返回400 QUALITY_INVALID_CONFIG。每项validation_errors包含code、item_index或policy_id、field、message，保存接口仍按正式HTTP错误语义拒绝相同问题，不以预览结果作为免校验凭据。

增加 capacity_preview：ordinary_calls_per_day（逐策略 86400/I 求和）、global_limit、planned_call_capacity_percent，以及per_policy数组。per_policy各项含policy_id或item_index、daily_call_limit、ordinary_calls_per_day、ideal_failure_span_seconds、ideal_recovery_span_seconds、window_slack_seconds及warnings；新增尚无ID者用item_index定位。估算按拟保存后的整体活动配置计算；全局自动开关未开时分别给configured_demand和enabled_demand，不能把0次当前执行需求当成未来无费用。estimated_detection_coverage为已保存策略当前新鲜证据覆盖，不包含尚未检测的新建项；规划预算覆盖另用planned_call_capacity_percent，不混为真实覆盖率。

warnings 每项为 code、severity、policy_ids、message，code 至少含 BUDGET_DEFICIT、WINDOW_DELAY_RISK、RECOVERY_BUDGET_SHARED、OUTPUT_LIMIT_UNSUPPORTED。理想窗口不合法属于 validation_errors、阻止保存；预算缺口与排队/抖动余量不足属于 warning、允许保存，服务端保存时仍重新校验并回显警告。不新增“确认即自动扩预算”等隐藏动作。

启控预览须区分 observed_risk（旧观察结果）和 activation_effect（新代次从 untested 开始，立即质量排除数为 0）。预览不是锁或预约，保存时重新校验；不提交模型调用、不扣预算。检测覆盖为“有新鲜证据的配置范围/已配置范围”，不能把未配置 effort/模型算作测过。

## 6. 策略创建、更新与生命周期

POST `/policies` 示例，example-model 必须替换为账号实际支持模型：

```json
{
  "items":[{
    "account_id":101,
    "source_group_id":7,
    "condition":{
      "model_id":"example-model",
      "protocol":"responses",
      "transport":"http",
      "reasoning_effort":"low",
      "service_tier":"upstream_default",
      "benchmark_id":"candy",
      "benchmark_version":"1",
      "scorer_version":"strict-json-v1"
    },
    "rules":{"auto_check_enabled":false,"control_enabled":false}
  }]
}
```

返回 201 + items（id、revision、normalized_condition、rules、state）。同一规范化请求范围只允许一个活动策略；不同题目版本也不能重复建立冲突控制。替换条件使用可选 replaces_policy_id 与 replaces_expected_revision，新建与归档旧策略同事务；替代策略从 untested 开始，不暗中继承隔离。界面明确提示更换检测范围会重新积累证据。

PUT `/policies/:id`：`{"expected_revision":3,"rules":{完整规则}}`。POST `/policies/batch-update`：`{"items":[{"policy_id":11,"expected_revision":3,"rules":{完整规则}}]}`。不接收 account_id/condition；条件变化走替换。版本冲突导致整个批量更新回滚。

GET `/policies` 支持 account_id、source_group_id、model_id、state、freshness、eligibility、control_enabled、auto_check_enabled、archived（默认 false）。返回 condition、rules、state、revision、capabilities、今日预算和只读执行域；不含完整回答或凭据。

POST `/policies/:id/archive`：expected_revision + reason。停止控制与自动、推进代次、取消未发出任务；运行中协作取消，迟到结果不能改状态。归档不把账号显示成“检测正常”。

POST `/policies/:id/release`：expected_revision + reason。仅允许 quarantined/recovering；置 untested/never、清零两类计数、推进策略代次并写事件。自动/控制开关不变，下次新的结果可再次进入观察。已经 normal 不是重复“恢复成功”，返回 409 QUALITY_RELEASE_NOT_APPLICABLE；同幂等键重放原结果不重复记事件。

## 7. 运行、查询与取消

POST `/runs`：

```json
{"policy_ids":[11,12,13]}
```

不接受 account_id、任意 Prompt、模型覆盖、bypass_quality 等字段。服务端从策略快照确定条件。手动任务可以在自动开关关闭时运行；隔离冷却后的手动任务可作为恢复证据，冷却前仅诊断。运行用途为服务端只读字段，不由客户端强制指定恢复成功。

手动任务也不能越过自动不降智库存边界。提交与出站前均检查账号/来源资格，移出范围、来源上下文失效分别返回 QUALITY_TARGET_OUT_OF_SCOPE / QUALITY_SOURCE_CONTEXT_INVALID，不因“管理员手动”就改测普通固定分组。

```json
{
  "code":0,
  "message":"accepted",
  "data":{
    "batch_id":"ab859d09-f872-4aa0-aa4b-39a198f866c0",
    "items":[
      {"policy_id":11,"submission":"queued","run_id":901},
      {"policy_id":12,"submission":"existing","run_id":899},
      {"policy_id":13,"submission":"skipped","run_id":null,"reason":"QUALITY_DAILY_BUDGET_EXCEEDED"}
    ]
  }
}
```

整体请求不合法/无权限时不创建任何任务；合法批量按每项排队、复用、跳过返回 202，即使全部跳过也不伪装成已执行。不存在的策略、归档、账号禁用、预算、队列满等返回每项原因；不分配 run_id 的跳过也包含在提交审计内。排队成功只表示受理，预算在认领时再次校验，之后仍可能 skipped。

GET `/runs` 支持 batch_id、policy_id、account_id、status、verdict、trigger_source、from、to，以及 scope_hash、global_control_epoch、policy_control_epoch。普通时间筛选按 COALESCE(started_at,queued_at)，from 含边界、to 不含边界，最长 180 天，默认最近 7 天；只能查询保留期内实际存在的记录。时间桶下钻另见 §11。response_text 不出现在列表。运行状态和作答结果分离：

| status | verdict | 语义 |
| --- | --- | --- |
| queued / running | null | 未得到终态，不评价作答 |
| completed | passed / incorrect / unknown / error | 本轮结束；HTTP 200 或 completed 本身不代表质量合格 |
| cancelled / interrupted / skipped | null | 无完整质量结论；reason_code 解释原因 |

GET `/runs/:id?include_response=true` 返回输入快照、运行/作答、final_answer、expected_answer、state_applied/state_skip_reason、用量、耗时及正文。默认 include_response=false；正文过期为 null，另有 response_available=false、response_expired=true。已保存过长正文展示截断标记，并维持 unknown 结果。

POST `/runs/:id/cancel`：`{"reason":"停止本次诊断"}`。queued 可原子取消；running 只设置取消请求，返回 cancel_requested=true、status=running、upstream_dispatched。worker 终态为 cancelled；如果完成事务已先提交，则返回已完成状态与 cancellation_effective=false，不覆盖答案。取消不能撤销上游已计费调用。

## 8. 摘要与事件

POST `/summaries`：`{"account_ids":[101,102]}`，最多 100 个；每项含 configured、active_policy_count、worst_state、fresh/stale 数量、enforced_count、conditions 摘要、last_checked_at。worst_state 推荐排序 quarantined > recovering > observing > untested > normal；正常但过期的项展示质量未知，不能被最严重状态掩盖。多个策略时显示“混合状态”，详情逐条件展开。

每项附 eligibility 和 applies_to=openai_auto_cheapest；不在库存返回“不在检测范围”，仍可保留历史摘要，不返回该账号“所有分组已隔离”的推断。该接口可批量读取任意后台账号用于列表展示，但不能藉此触发范围外检测。

GET `/events` 支持 account_id、policy_id、run_id、event_type、from、to。返回旧/新状态、reason_code、关联记录、操作人、配置 revision、控制代次与时间，不返回敏感账号详情。全局管理操作可关联现有审计 ID；事件列表不可充当完整审计日志的替代品。

## 9. 幂等与冲突

创建策略、批量更新、提交运行、归档、人工解除、紧急停用使用 `Idempotency-Key`，前端每次明确操作生成一个 UUID，网络重试复用。缺少键返回现有 400 IDEMPOTENCY_KEY_REQUIRED；建议重放窗口 24 小时，按既有管理员主体 + 动作域 + key 隔离。

同 key/同归一化请求体返回原 HTTP 状态与原数据；同 key/不同体返回现有 409 IDEMPOTENCY_KEY_CONFLICT。幂等存储不可用返回现有 503 IDEMPOTENCY_STORE_UNAVAILABLE，不采用 fail-open 新建付费任务。使用既有 PostgreSQL 幂等表与指纹规则，质量管理的业务提交与可重放响应必须在同一事务保存，方案见 design §6.4；活动任务唯一约束和周期唯一约束作为另一层兜底。全局幂等 ObserveOnly 模式或协调器未初始化时，不允许绕过本模块的必需键/存储检查。

PUT 设置和单策略更新由 expected_revision 保证重复提交最多成功一次；超时后先 GET 查 revision/当前值，不自动覆盖。取消操作天然幂等，不要求付费任务重放能力。保存并检测由“保存返回 ID → 单独提交任务”两步组成，保存成功、检测失败时前端分别提示，不能回滚为“未保存”。

## 10. 错误目录与公网表现

| HTTP | reason | 含义 |
| --- | --- | --- |
| 400 | QUALITY_INVALID_CONFIG | 参数/范围/组合不合法；metadata.field 指向字段 |
| 400 | QUALITY_UNSUPPORTED_CONDITION | 不支持目标条件；metadata.condition 为简要说明 |
| 400 | QUALITY_TARGET_OUT_OF_SCOPE / QUALITY_SOURCE_CONTEXT_INVALID | 不在自动不降智库存/来源上下文已失效；批量运行在 item.reason 返回 |
| 400 | IDEMPOTENCY_KEY_REQUIRED / IDEMPOTENCY_KEY_INVALID | 沿用现有缺少键/键格式错误 |
| 401 / 403 | 沿用现有鉴权 reason | 未登录/无权限，不能触发任务 |
| 404 | QUALITY_POLICY_NOT_FOUND / QUALITY_RUN_NOT_FOUND | 管理资源不存在 |
| 409 | QUALITY_REVISION_CONFLICT | 配置已被修改，metadata.current_revision 为字符串 |
| 409 | QUALITY_SCOPE_CONFLICT | 规范化后存在活动策略，metadata.policy_id 为字符串 |
| 409 | QUALITY_POLICY_ARCHIVED / QUALITY_RELEASE_NOT_APPLICABLE | 生命周期或动作不匹配 |
| 409 | IDEMPOTENCY_KEY_CONFLICT / IDEMPOTENCY_IN_PROGRESS | 沿用现有请求体冲突/仍在处理中 |
| 429 | QUALITY_QUEUE_FULL / QUALITY_DAILY_BUDGET_EXCEEDED | 单项运行拒绝原因；批量通过 item.reason 返回 |
| 503 | QUALITY_STATE_UNAVAILABLE | 控制范围无可信质量快照 |
| 503 | QUALITY_NO_ELIGIBLE_ACCOUNTS | 有基础合格候选，但均被质量隔离 |
| 503 | QUALITY_PERSISTENCE_UNAVAILABLE | 设置/策略/任务无法可靠保存 |
| 503 | IDEMPOTENCY_STORE_UNAVAILABLE | 无法可靠保存/读取幂等结果 |

请求同时被其他基础硬门槛拒绝时，保留既有基础错误；只有质量门导致本来合格候选全部消失，才报 QUALITY_NO_ELIGIBLE_ACCOUNTS。不能将普通额度耗尽说成质量隔离。

本节公网质量错误仅适用于原始自动不降智模式及其内部切组/复用/WS turn。普通固定分组和独立 composite 不返回本模块新增质量错误；不能在共享底层选号函数里无条件读取状态。

管理端错误样例：

```json
{"code":409,"message":"配置已被其他操作更新","reason":"QUALITY_REVISION_CONFLICT","metadata":{"current_revision":"4"}}
```

公网 OpenAI 兼容接口继续使用原有 `error` 对象，不能返回管理端 envelope：

```json
{"error":{"message":"当前匹配范围暂无可用上游，请稍后重试","type":"server_error","code":"QUALITY_NO_ELIGIBLE_ACCOUNTS"}}
```

不可迁移的 response 绑定账号被隔离时返回 409、code=QUALITY_SESSION_RESTART_REQUIRED，提示重新开始会话，不暴露账号 ID。已升级 WebSocket 使用既有协议错误事件携带相同 code；HTTP 握手尚未升级时返回相应 HTTP 错误。单个 turn 在发送前拒绝后保留连接还是按既有协议结束连接，由当前转发器允许的错误恢复方式决定，验收必须证明后续请求不会绕过隔离。不得向公网客户端公开题目答案、内部策略、账号、凭据或完整管理事件。

## 11. 监控聚合与时间线契约

### 11.1 全局概览 GET /overview

支持与 /policies 相同的 source_group_id、account_id、model_id、state、freshness、eligibility、控制/自动开关筛选；默认活动策略、eligible。服务端对筛选后的**全部策略**聚合，不能只汇总当前分页。source_group_id 匹配兼容来源集合，同一策略/账号去重。

返回以下独立对象：

| 对象 | 字段与口径 |
| --- | --- |
| snapshot | as_of、window_start、window_end、complete、applies_to=openai_auto_cheapest；时间由服务端提供 |
| inventory | eligible_account_count、configured_account_count、unconfigured_account_count；按来源/账号搜索条件统计，状态/模型筛选不把未配置账号删掉；附 inventory_filters 说明此口径 |
| policies | total、state_counts、freshness_counts、effective_control_count、out_of_scope_count、source_context_invalid_count；按全部策略筛选计算，不能和账号数直接相加 |
| runs_24h | summary 同 §11.2；筛选策略各自当前 scope/epoch 的任务合并统计；返回 evidence_view=current_epoch |
| budget | 全局 budget_date、timezone、limit、consumed、reserved、remaining；始终为全局值，不随列表筛选变化 |
| health | 全局 queue_depth、running_count、store_status、last_sync_at；不能把筛选结果当成集群状态 |

库存为 0、未配置与全部未检测是不同空态。跨多个策略的作答通过率标注“所选配置的汇总样本”，不能宣称为账号健康率。通过率、有效判分率范围为 0–100%，无对应分母时 null。

### 11.2 单策略 GET /policies/:id/monitor

参数：evidence_view=current_epoch（默认）或 current_scope；bucket_seconds 可选整数 300–86400，默认策略的普通 interval_seconds。时间窗口固定服务端当前时刻向前 24 小时，不允许客户端用未来时间扩大窗口。

- current_epoch：匹配当前 policy_id、scope_hash、全局/策略状态代次；用于当前监控。全局已换代但策略尚未写入时使用投影后的新代次，不误取旧状态快照。
- current_scope：匹配当前 policy_id/scope_hash，包含该范围全部代次，页面标“包含旧观察/控制历史”。不能作为状态机输入。
- 当前范围已失效仍可显示历史和 eligibility，但不伪装成当前控制生效。历史旧 scope 用 /runs 查询；归档策略可读，不能运行。
- 服务端在短只读一致性事务中取得 as_of、策略/代次与任务统计。summary 和所有桶来自同一次快照，按 run_id 去重，时间以 started_at 优先、否则 queued_at。
- 完整窗口为 `[as_of-24h, as_of]`，未来记录排除；桶为左闭右开，最后一桶包含 as_of。桶从窗口起点连续划分，最大 `ceil(86400/300)=288` 个，最后一桶可以不足整段。
- summary 返回 total、completed、passed、incorrect、unknown、errors、queued、running、cancelled、interrupted、skipped、valid_answers、pass_rate、answer_coverage，与已移植 TS 模块同口径。没有 run_id 的提交跳过只在提交审计/预算提示，不计任务统计。
- 桶返回 index、start_at、end_at、status、total、status_counts、sample_run_ids、has_more。样本 ID 最多 5 个，按 id 倒序；has_more 表示 total 大于样本数量。每个桶所有计数包含完整匹配记录，不因样本截断而少算。
- 桶着色优先级：running > queued > error > incorrect > unknown > interrupted > cancelled > skipped > passed > empty。时间线色块不是账号质量状态。

成功响应示例为两桶的完整 24H 视图，使用 43200 秒展示粒度；实际默认 1800 秒会有 48 桶：

```json
{
  "code":0,
  "message":"success",
  "data":{
    "policy_id":11,
    "revision":4,
    "scope_hash":"example-scope-hash",
    "global_control_epoch":3,
    "policy_control_epoch":1,
    "evidence_view":"current_epoch",
    "eligibility":"eligible",
    "as_of":"2026-10-28T04:00:00Z",
    "window_start":"2026-10-27T04:00:00Z",
    "window_end":"2026-10-28T04:00:00Z",
    "bucket_seconds":43200,
    "complete":true,
    "summary":{
      "total":3,"completed":3,"passed":1,"incorrect":1,"unknown":0,
      "errors":1,"queued":0,"running":0,"cancelled":0,"interrupted":0,"skipped":0,
      "valid_answers":2,"pass_rate":50,"answer_coverage":66.66666666666667
    },
    "buckets":[
      {
        "index":0,"start_at":"2026-10-27T04:00:00Z","end_at":"2026-10-27T16:00:00Z",
        "status":"error","total":1,"status_counts":{"error":1},
        "sample_run_ids":[898],"has_more":false
      },
      {
        "index":1,"start_at":"2026-10-27T16:00:00Z","end_at":"2026-10-28T04:00:00Z",
        "status":"incorrect","total":2,"status_counts":{"passed":1,"incorrect":1},
        "sample_run_ids":[901,900],"has_more":false
      }
    ]
  }
}
```

status_counts 未出现的状态按 0 解释。complete=true 仅指此次筛选/窗口已全量聚合；不证明所有应测任务都已执行。数据读取失败返回 503 QUALITY_PERSISTENCE_UNAVAILABLE，不返回 complete=true 的空图；前端可保留上次图并标明过期和失败。

### 11.3 下钻、刷新与已移植模块的边界

点击桶使用 GET /runs，携带 policy_id、scope_hash，以及monitor_as_of、bucket_seconds、bucket_index；current_epoch视图另带返回的global_control_epoch/policy_control_epoch，current_scope视图省略这两个代次过滤。三项时间桶参数必须一起提供，不得同时提供 from/to；服务端重建 §11.2 相同桶边界，拒绝越界 index、未来 as_of 或超过元数据保留期的窗口。正常 page/page_size 分页可遍历桶内全部记录，不依赖 sample_run_ids。

monitor_as_of 固定时间边界，**不是可复用数据库快照令牌**。下钻与刷新可能看到任务从 queued/running 变终态，或因 started_at 产生而移动到别的桶；正文/历史也可能到期清理。界面提示“记录已更新，请刷新统计”，不伪造跨请求的历史状态。需要严格一致的 summary 与桶时重新请求 /monitor，不能无限持有事务。

前端按 policy_id/scope_hash/代次/evidence_view 识别一幅图，切换条件后丢弃旧异步回包，统一使用 as_of 判断数据时间。概览、分页列表与详情是不同请求，不声称三者共用一个实时快照。活动任务按 2–5 秒退避轮询；空闲监控建议 30 秒刷新；隐藏页面暂停，手动刷新不发模型调用。

正式页面直接渲染聚合返回的 summary/buckets；不得对一页 /runs 调用 buildQualityMonitor 后标为完整 24H。已移植模块可用于受控且完整的小数据集合、离线样例和聚合口径对照，传入前另按代次过滤；它当前不校验数据完整性或代次，也没有服务端桶样本上限，不能自动等同于该 API。
