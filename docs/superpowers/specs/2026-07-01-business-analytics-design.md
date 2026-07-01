# Business Analytics Design

## 背景

sub2api 当前有大量用户、多个分组和多个渠道账号。分组价格不同，用户使用量会随价格变化波动；同一个渠道账号也可能同时挂在多个分组下。渠道账号的进价相同，但不同分组的售卖倍率不同，因此经营统计必须同时回答：

- 每天或每周整体收入、成本、利润和活跃用户变化。
- 每个分组的倍率、用户数、收入、成本、利润和利润率。
- 每个渠道账号在不同分组下的使用人数、成本、收入贡献和利润贡献。
- 调价前后用户数和利润是否变好。
- 渠道价格变化后，历史使用记录是否仍能按当时价格追溯。

现有项目已经有 `usage_logs.actual_cost`、`usage_logs.rate_multiplier`、`usage_logs.account_rate_multiplier`、`usage_logs.account_stats_cost` 和 `accounts.channel_price` 等基础字段。新功能应复用这些口径，并补齐渠道价格快照和经营分析聚合能力。

## 目标

1. 提供后台经营分析页面，直观看到收入、渠道成本、毛利润、利润率、活跃用户、请求量等指标。
2. 支持按日、按周查看总览、分组、渠道账号和明细报表。
3. 支持调价前后对比，辅助判断涨价、降价或维持价格。
4. 保证历史利润统计不受当前分组倍率或渠道价格变化影响。
5. 在数据量增长后仍保持页面查询速度可控。

## 非目标

1. 第一版不改变用户扣费逻辑。
2. 第一版不改变账号调度逻辑。
3. 第一版不自动给出价格调整决策，只提供对比指标和提示型结论。
4. 第一版不要求对所有历史 usage 精确回填当时渠道价格；老数据允许按现有成本口径兼容展示。

## 核心口径

- 用户收入：`SUM(usage_logs.actual_cost)`。
- 渠道成本：优先使用现有账号成本口径：
  `SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1))`。
- 毛利润：`用户收入 - 渠道成本`。
- 利润率：`毛利润 / 用户收入`；收入为 0 时展示为 `-`。
- 分组当前倍率：来自 `groups.rate_multiplier`。
- 分组区间倍率：来自 `usage_logs.rate_multiplier` 的加权平均值，避免调价前后混合误导。
- 渠道当前价格：来自 `accounts.channel_price`。
- 渠道区间价格：来自 usage 级渠道价格快照的加权平均值。
- 同一渠道账号挂多个分组时，收入按 usage 所属分组归集，成本按 usage 所属账号归集，利润在分组和账号维度都可拆分。

## 使用记录快照

为保证历史可追溯，新增 usage 级渠道价格快照字段：

- `usage_logs.channel_price_snapshot NUMERIC(12,6) NULL`
  - 本次请求发生时账号的渠道价格。
- `usage_logs.channel_price_source VARCHAR(32) NULL`
  - 渠道价格来源，例如 `manual`、`upstream_balance`、`fallback`。
- `usage_logs.channel_price_refreshed_at TIMESTAMPTZ NULL`
  - 该价格最近一次刷新时间。

请求落 usage 时只做轻量快照：

1. 从选中的账号读取 `channel_price`。
2. 从账号 `extra.upstream_balance_updated_at` 或后续规范字段读取价格刷新时间。
3. 写入 usage 快照字段。
4. 不在请求链路触发余额刷新。
5. 不在请求链路执行经营统计聚合。

老 usage 数据没有渠道价格快照时：

- 成本仍按现有 `account_stats_cost/account_rate_multiplier` 口径计算。
- 页面标记“缺少渠道价格快照”或“历史近似成本”。

## 汇总表设计

经营分析使用独立汇总表，不直接扩展现有 `usage_dashboard_hourly` / `usage_dashboard_daily`。原因是经营分析需要分组、渠道、用户去重和利润维度，直接塞入现有 dashboard 表会增加原有页面复杂度和聚合负担。

### business_usage_daily

粒度：每天 + 分组 + 渠道账号。

建议字段：

- `bucket_date DATE NOT NULL`
- `group_id BIGINT NOT NULL DEFAULT 0`
- `account_id BIGINT NOT NULL DEFAULT 0`
- `channel_id BIGINT NOT NULL DEFAULT 0`
- `platform VARCHAR(50)`
- `requests BIGINT NOT NULL DEFAULT 0`
- `active_users BIGINT NOT NULL DEFAULT 0`
- `active_api_keys BIGINT NOT NULL DEFAULT 0`
- `total_tokens BIGINT NOT NULL DEFAULT 0`
- `revenue NUMERIC(20,10) NOT NULL DEFAULT 0`
- `channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0`
- `gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0`
- `avg_group_rate_multiplier NUMERIC(10,4)`
- `avg_channel_price NUMERIC(12,6)`
- `missing_channel_price_records BIGINT NOT NULL DEFAULT 0`
- `computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`

主键建议：`(bucket_date, group_id, account_id)`。未知分组、未知账号、未知渠道统一写入 `0`，避免 PostgreSQL `NULL` 唯一约束语义导致重复聚合行。

### business_usage_weekly

粒度：自然周 + 分组 + 渠道账号。

字段与 daily 基本一致，使用 `week_start DATE NOT NULL` 作为周起始日期。

### business_usage_daily_users

粒度：每天 + 分组 + 渠道账号 + 用户。

用途：活跃用户去重、用户新增/流失、调价分析。

建议字段：

- `bucket_date DATE NOT NULL`
- `group_id BIGINT NOT NULL DEFAULT 0`
- `account_id BIGINT NOT NULL DEFAULT 0`
- `user_id BIGINT NOT NULL`
- `requests BIGINT NOT NULL DEFAULT 0`
- `revenue NUMERIC(20,10) NOT NULL DEFAULT 0`
- `channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0`
- `gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0`

## 实时与离线策略

采用“快照 + 今日实时 + 历史汇总表”的混合方案：

- 今天数据：实时从 `usage_logs` 查，或使用“汇总表 + 今日未聚合增量”。
- 昨天及更早：优先读 `business_usage_daily`。
- 周数据：优先读 `business_usage_weekly`。
- 调价对比：按管理员选择的调价日期，从 daily 汇总表读取前后 3/7/14 天。
- 补算能力：提供后台重算指定日期范围的能力，修复任务中断或迁移后的汇总缺口。

聚合任务建议：

- 每 5 分钟聚合最近 2 小时，覆盖写入。
- 每小时重算当天。
- 每天凌晨重算昨天。
- 每周一重算上周。
- 如果清理历史 `usage_logs`，经营汇总表应保留更久，作为长期经营数据来源。

## 渠道价格刷新任务

新增 `ChannelPriceRefreshJob`：

- 默认每 10 分钟执行一次。
- 支持配置开关，便于线上关闭。
- 找出支持余额刷新且启用的账号，例如 OpenAI API Key、Anthropic API Key。
- 并发限制建议 3-5 个账号。
- 每个账号独立失败，不影响其他账号。
- 刷新成功：
  - 更新 `accounts.channel_price`。
  - 更新 `accounts.extra.upstream_balance_*`。
  - 记录刷新时间和来源。
- 刷新失败：
  - 只更新失败状态和错误信息。
  - 不清空旧价格。
  - 不把价格写成 0。

手动批量刷新继续保留；自动任务和手动刷新应复用同一服务能力。

## 后端接口

新增后台接口前缀：

`/api/v1/admin/business-analytics`

建议接口：

### GET /overview

用于总览看板。

参数：

- `start_date`
- `end_date`
- `granularity=day|week`
- `group_id`
- `account_id`

返回：

- 总收入、总成本、毛利润、利润率。
- 活跃用户、请求数、人均收入、人均利润。
- 收入/成本/利润趋势。
- 活跃用户趋势。
- 分组和渠道榜单摘要。

### GET /groups

分组分析表格。

返回每个分组：

- 当前倍率。
- 区间平均倍率。
- 活跃用户。
- 请求数。
- 收入。
- 成本。
- 毛利润。
- 利润率。
- 人均收入。
- 人均利润。
- 较上周期用户变化。
- 较上周期利润变化。

### GET /groups/:id/channels

某分组下渠道拆分。

返回该分组下各渠道账号：

- 使用人数。
- 请求量。
- 收入。
- 成本。
- 毛利润。
- 利润率。
- 平均渠道价格。

### GET /channels

渠道分析表格。

返回每个渠道账号：

- 当前渠道价格。
- 区间平均渠道价格。
- 被哪些分组使用。
- 活跃用户数。
- 请求数。
- 成本。
- 贡献收入。
- 贡献利润。
- 最近余额刷新时间。
- 余额刷新状态。

### GET /channels/:id/groups

某渠道账号在不同分组下的利润拆分。

### GET /price-change-impact

调价分析。

参数：

- `group_id`
- `change_date`
- `window_days=3|7|14`

返回：

- 调价前后平均倍率变化。
- 活跃用户变化。
- 请求数变化。
- 收入变化。
- 成本变化。
- 毛利润变化。
- 利润率变化。
- 新增用户数。
- 流失用户数。

### GET /records

明细报表。

支持日期、分组、渠道账号、用户、API Key、模型筛选，支持分页和排序。

字段包括：

- 时间。
- 用户。
- API Key。
- 分组。
- 渠道账号。
- 模型。
- token 或请求数。
- 分组倍率快照。
- 渠道价格快照。
- 用户收入。
- 渠道成本。
- 毛利润。
- 利润率。

### GET /export

导出报表。第一版建议先支持 CSV，后续再扩 Excel 样式。

## 前端页面

后台新增菜单：`经营分析`。

页面使用页签组织：

1. 总览看板
   - 指标卡：收入、成本、毛利润、利润率、活跃用户、请求数、人均收入、人均利润。
   - 趋势图：收入/成本/利润、活跃用户、利润率。
   - 榜单：利润最高分组、利润率最高分组、低利润分组、成本最高渠道、用户下降最快分组。
2. 分组分析
   - 表格展示各分组经营指标。
   - 支持展开查看该分组下各渠道账号贡献和用户排行。
3. 渠道分析
   - 表格展示各渠道账号成本、收入贡献、利润贡献和刷新状态。
   - 支持展开查看该渠道在不同分组下的利润拆分。
4. 调价分析
   - 管理员选择分组、调价日期、窗口天数。
   - 展示调价前后用户数、收入、成本、利润和利润率变化。
5. 明细报表
   - 支持筛选、排序、分页和导出。

页面风格应保持后台经营工具风格：信息密度高、表格强、图表辅助，不做营销式展示。

## 实施阶段

### 阶段 1：使用记录快照补齐

- 新增 usage 渠道价格快照字段。
- 落 usage 时写入当时渠道价格、来源和刷新时间。
- 不改变现有扣费和调度逻辑。

### 阶段 2：自动刷新渠道价格

- 新增自动刷新任务。
- 每 10 分钟批量刷新支持的渠道账号。
- 刷新失败不覆盖旧价格。
- 支持配置开关、并发限制和超时。

### 阶段 3：经营汇总表和接口

- 新增 daily、weekly、daily_users 汇总表。
- 新增聚合任务和重算能力。
- 新增经营分析后台接口。

### 阶段 4：前端经营分析页面

- 新增后台菜单和页面。
- 实现总览看板、分组分析、渠道分析、调价分析、明细报表。
- 先支持 CSV 导出。

## 第一版建议范围

第一版至少交付：

- usage 渠道价格快照。
- 自动刷新渠道价格。
- 日维度经营汇总表。
- 总览看板。
- 分组分析。
- 渠道分析。
- 明细报表基础筛选。

周汇总、调价分析和增强导出可以作为第一版后半段或第二版，但数据库设计应提前预留。

## 风险与处理

- 老 usage 缺少渠道价格快照：按现有账号成本口径展示，并标记为历史近似数据。
- 上游余额接口不稳定：保留上一次有效价格，页面显示最近刷新时间和失败状态。
- 首次历史汇总数据量大：必须分批重算，避免一次性全量扫描。
- 调价日期没有历史记录：第一版由管理员手动选择调价日期；后续可新增分组价格变更历史表。
- 非 PostgreSQL 环境：参考现有 dashboard 聚合策略，可禁用预聚合并降级为实时查询或仅支持有限范围查询。

## 验证计划

- 单元测试：
  - usage 写入时渠道价格快照。
  - 渠道价格刷新成功/失败行为。
  - 利润、利润率、加权平均倍率计算。
- Repository 测试：
  - daily 汇总 upsert。
  - 重算日期范围。
  - 缺少渠道价格快照的数据兼容。
- Handler 测试：
  - overview/groups/channels/records 参数校验和返回结构。
  - price-change-impact 前后窗口计算。
- 前端测试：
  - 筛选参数传递。
  - 表格字段展示。
  - 空数据、缺少价格快照、刷新失败状态展示。
- 手工验证：
  - 创建一个渠道账号同时挂两个不同倍率分组。
  - 构造同一天 usage，确认同一渠道成本一致、不同分组收入不同。
  - 修改分组倍率后新 usage 使用新倍率，历史利润不漂移。
