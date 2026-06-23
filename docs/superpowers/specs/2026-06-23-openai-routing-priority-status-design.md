# OpenAI 调度优先级状态设计

日期：2026-06-23

## 背景

账号管理页当前把账号状态、调度开关、稳定性、手动优先级、渠道价格分散在不同列里。管理员能看到单项数据，但很难直接判断某个 OpenAI 渠道在真实调度中为什么靠前、为什么靠后、为什么被跳过，尤其在冷却、观察、隔离、运行时阻断和价格权重同时存在时容易误判。

本设计目标是让账号管理页提供“调度器视角”的状态：列表给出简洁优先级摘要，详情解释排序和排除原因；同时整理后续需要修正的调度策略与状态一致性问题。

## 目标

1. 在账号管理页新增可解释的调度优先级状态。
2. 支持查看当前账号在指定分组、模型、平台下的真实候选排序。
3. 明确区分持久状态、高级调度健康态、运行时阻断和前端倒计时状态。
4. 暴露排序原因和排除原因，减少“明明能调度却显示冷却”“低价高速渠道没排前”等排障成本。
5. 分阶段落地，第一阶段先可解释，第二阶段再调整策略。

## 非目标

1. 第一阶段不直接改变线上调度选择结果。
2. 第一阶段不重写 OpenAI scheduler 架构。
3. 不把账号列表变成完整调度面板；复杂解释放到详情弹窗。
4. 不在列表接口返回大体量候选明细，避免拖慢账号管理页。

## 现状问题

### 状态来源分裂

账号状态至少来自四类来源：

- 持久状态：`status`、`schedulable`、`rate_limit_reset_at`、`overload_until`、`temp_unschedulable_until`。
- 高级调度健康态：`primary`、`standby`、`observe`、`degraded`、`cooldown_until`。
- 运行时阻断：OAuth 429、403 临时阻断、transport error 等进程内 block。
- 前端本地展示：倒计时和筛选逻辑。

这些状态当前没有统一解释入口，导致账号列表、OpenAI 调度页和真实调度行为可能看起来不一致。

### 优先级不可解释

当前高级调度会先处理 `previous_response_id` 粘性，再处理 `session_hash` 粘性，最后进入 Top-K 加权选择。候选分数受手动优先级、负载、等待数、错误率、TTFT、渠道价格、健康分影响，但账号页没有展示分数拆解。

### 部分配置语义不清

`primary_ratio`、`primary_min_count`、`observe_probe_ratio` 已有设置结构，但现有实际调度中没有完整参与排序和恢复探测。第一阶段应如实展示，第二阶段再补齐或隐藏无效配置。

## 第一阶段设计：可解释调度状态

### 后端接口

新增调度解释接口，建议路径：

- `GET /api/v1/admin/openai-scheduler/ranking`
- `GET /api/v1/admin/openai-scheduler/accounts/:id/routing-explain`

查询参数：

- `group_id`：可选；为空时使用全局 OpenAI 候选视角。
- `model`：可选；为空时只做通用可调度解释。
- `platform`：默认 `openai`。
- `account_id`：详情接口路径参数。

列表摘要返回轻量字段：

- `account_id`
- `rank`
- `tier`
- `score`
- `status_label`
- `summary_reason`
- `is_schedulable_now`
- `primary_block_reason`
- `snapshot_at`

详情返回完整解释：

- 候选排序列表。
- 当前账号分数拆解：priority、load、queue、error_rate、ttft、price、health。
- 当前账号状态来源：持久状态、健康态、运行时阻断。
- 排除原因列表。
- Top-K 和加权随机说明。
- sticky 命中说明：是否因为 `previous_response_id` 或 `session_hash` 可能绕过普通排序。

### 排除原因枚举

后端统一输出稳定 reason code，前端做本地化：

- `status_error`
- `status_inactive`
- `manual_unschedulable`
- `rate_limited`
- `overloaded`
- `temp_unschedulable`
- `runtime_blocked`
- `health_degraded`
- `model_unsupported`
- `capability_unsupported`
- `transport_unsupported`
- `group_mismatch`
- `privacy_not_set`
- `quota_auto_paused`
- `concurrency_full`
- `channel_restricted`
- `compact_unsupported`

### 账号管理页展示

在账号管理页新增一列：`调度优先级`。

列表展示格式：

- 可调度：`#3 主力 · 成本优 · 低负载`
- 观察态：`#8 观察 · 首包偏高`
- 不可调度：`跳过 · 429 冷却 3m20s`
- 隔离：`隔离 · 连续失败`

点击该列打开详情弹窗。弹窗包含：

- 当前账号调度摘要。
- 选定分组和模型。
- 分数拆解。
- 排除原因。
- 同组候选 Top 列表。
- 快照更新时间和状态来源说明。

### 前端状态口径

账号列表里的 `状态` 列继续展示账号持久状态和即时冷却。

新增 `调度优先级` 列展示调度器视角，不替代原状态列。

`稳定性` 列可以保留，但后续可考虑与调度优先级合并或弱化，避免重复展示 `主力/备用/观察/隔离`。

### 状态刷新

账号状态组件增加本地倒计时刷新，保证冷却到期后 UI 自动更新。

调度优先级摘要使用轻量接口刷新。账号页自动刷新关闭时，倒计时仍应本地更新；排序和分数仍以服务端快照为准。

## 第二阶段设计：策略优化

### 成本/速度策略

新增策略模式：

- `balanced`：保留现有均衡逻辑。
- `cost_first`：提高价格权重，低价且速度可接受的渠道更靠前。
- `speed_first`：提高 TTFT 和错误率权重，优先快速稳定。

策略应影响分数，不改变前置安全过滤。

### Top-K 选择模式

新增选择模式：

- `weighted_top_k`：当前 Top-K 加权随机。
- `strict_best`：最高分优先。

页面必须提示当前模式，避免管理员把“候选排名第一”误解为“每次必选”。

### Observe 探测

让 `observe_probe_ratio` 真正参与调度：观察态账号按小比例进入候选，用于恢复验证。

探测流量应受上限保护，且不允许探测 `degraded` 或持久不可调度账号。

### 健康原因细分

上报调度结果时带失败分类：

- `429`
- `403`
- `timeout`
- `transport_error`
- `upstream_5xx`
- `manual`

健康态、详情弹窗和日志使用同一套原因码。

## 数据流

1. 账号管理页加载账号列表。
2. 前端请求调度摘要接口，按当前筛选的账号 ID 批量获取轻量状态。
3. 用户点击某个账号的调度优先级。
4. 前端请求详情接口。
5. 后端从 scheduler snapshot、账号仓储、并发服务和运行时健康态组装解释。
6. 前端渲染排序、分数拆解和排除原因。

## 错误处理

- scheduler cache 不可用时，接口返回 `source=db_fallback` 或明确错误。
- 并发负载读取失败时，返回排序但标记 `load_unknown=true`。
- 健康态不存在时，按默认健康态展示，并标记 `health_sample_missing=true`。
- 查询模型为空时，不输出模型专属排除原因。

## 测试计划

后端测试：

- 排名摘要接口返回稳定字段。
- 详情接口包含分数拆解和排除原因。
- 不同状态组合下 reason code 正确。
- 低价高速账号在 `cost_first` 下排名提升。
- `observe_probe_ratio` 只影响 observe，不影响 degraded。

前端测试：

- 账号列表展示调度优先级列。
- 点击列打开详情弹窗。
- 冷却倒计时到期后状态文案自动变化。
- 排除原因本地化正确。
- 接口失败时展示降级提示。

## 分阶段交付

第一阶段：

1. 后端调度解释 DTO 和接口。
2. 账号页调度优先级列。
3. 调度详情弹窗。
4. 状态来源与排除原因展示。
5. 冷却倒计时刷新。

第二阶段：

1. 成本/速度/均衡策略。
2. strict best / weighted top-k 选择模式。
3. observe probe 真正参与调度。
4. 健康失败原因细分。
5. 设置页和 OpenAI 调度页同步展示新策略。

## 风险

- 调度解释接口如果直接重算完整候选，可能影响账号页性能；第一阶段必须采用轻量摘要加按需详情。
- 如果第一阶段同时修改真实选择策略，线上流量分布会变动；因此策略优化放到第二阶段。
- 运行时健康态是进程内状态，多实例环境下解释可能不是全局一致；详情需要展示数据来源和快照时间。
