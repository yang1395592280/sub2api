### Task 5 Report: Business Analytics Aggregation Scheduler

**时间**: 2026-07-01

**修改文件**:
- `backend/internal/service/business_analytics_aggregation.go`
- `backend/internal/service/business_analytics_aggregation_test.go`
- `backend/internal/service/wire.go`
- `backend/internal/config/config.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`

**核心变更**:
- 新增 `BusinessAnalyticsAggregationService`，通过 `TimingWheelService.ScheduleRecurring` 定时触发经营分析聚合。
- 定时任务按配置回看窗口重算 daily 聚合，并同步重算当前 UTC 周的 weekly 聚合。
- 使用 atomic running flag 防止定时任务并发重叠。
- 新增 `TriggerRecomputeRange(start,end)`，拒绝空/反向/超出最大天数的非法范围，并异步重算指定范围。
- 新增 `config.BusinessAnalyticsConfig` 及默认值：
  - `enabled=true`
  - `aggregation_interval_seconds=300`
  - `lookback_seconds=7200`
  - `backfill_enabled=true`
  - `backfill_max_days=90`
- 接入 service provider set、server wire 和 cleanup Stop。

**RED 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation'`
- 结果：失败，符合预期。
- 摘要：`NewBusinessAnalyticsAggregationService`、`config.BusinessAnalyticsConfig`、`businessAnalyticsAggregationJobName` 尚不存在导致编译失败。

**GREEN 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation'`
- 结果：通过，`ok github.com/Wei-Shaw/sub2api/internal/service`。
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/config ./internal/service -run 'BusinessAnalytics|Config|Wire' -count=1`
- 结果：通过，`internal/config` 与 `internal/service` 均通过。

**风险点**:
- weekly 聚合当前每次 scheduled run 都重算当前周，满足 brief “once per scheduled run or at least once per hour”，但会增加少量重复 SQL 计算成本。
- `wire_gen.go` 本次按现有生成结果手工同步，未运行 wire 生成器。

**后续建议**:
- 若生产数据量增长明显，可后续增加 weekly 聚合的小时级节流或 leader lock。

---

### Task 5 Reviewer Fix Report: Business Analytics Aggregation Range Semantics

**时间**: 2026-07-01

**修改文件**:
- `backend/internal/service/business_analytics_aggregation.go`
- `backend/internal/service/business_analytics_aggregation_test.go`

**核心变更**:
- 定时经营分析 daily 重算范围改为完整 UTC 日期边界：`truncateToDayUTC(now-lookback)` 到 `truncateToDayUTC(now)+1 day`，避免同日小时窗口删除不到已有 daily 行导致重复插入主键冲突。
- 手动 `TriggerRecomputeRange` 在 `backfill_enabled=false` 时直接拒绝，返回 `ErrBusinessAnalyticsRecomputeDisabled`。
- 手动最大跨度限制在允许手动触发时始终生效，不再依赖 `backfill_enabled` 条件。
- weekly 重算改为覆盖 `[start,end)` 涉及的所有 UTC 周起始日，避免跨周范围只重算结束周。

**RED 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation' -count=1`
- 结果：失败，符合预期。
- 摘要：
  - `ScheduledAggregationUsesUTCDateRangeForDaily` 失败：daily start 仍为 `now-2h`，不是 UTC day boundary。
  - `TriggerRecomputeRangeRecomputesAllWeeksInRange` 失败：跨周范围未完成 3 个 week_start 调用。
  - `TriggerRecomputeRangeRejectsWhenBackfillDisabled` 失败：`backfill_enabled=false` 时返回 `<nil>`。

**GREEN 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation' -count=1`
- 结果：通过，`ok github.com/Wei-Shaw/sub2api/internal/service 3.161s`。

**风险点**:
- 跨周手动重算会按涉及周数多次调用 weekly SQL，范围越大计算成本越高；当前受 `backfill_max_days` 限制。
- 定时 daily 现在覆盖 UTC 日期，若 lookback 跨 UTC 日会重算多天，符合删除/插入 daily 表的日期粒度。

---

### Task 5 Second Fix Report: Manual Daily UTC Boundary

**时间**: 2026-07-01

**修改文件**:
- `backend/internal/service/business_analytics_aggregation.go`
- `backend/internal/service/business_analytics_aggregation_test.go`

**核心变更**:
- 手动 `TriggerRecomputeRange` 在异步重算前将 daily/recompute 范围归一为完整 UTC 日期边界：`truncateToDayUTC(start)` 到 `truncateToDayUTC(end)+1 day`。
- weekly 继续基于重算范围计算涉及周，归一化后不会遗漏原始 start/end 覆盖的 UTC 周。

**RED 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation' -count=1`
- 结果：失败，符合预期。
- 摘要：`TriggerRecomputeRangeUsesFullUTCDayRangeForDaily` 失败，daily start 实际为 `2026-07-01 02:30:00 +0000 UTC`，期望 `2026-07-01 00:00:00 +0000 UTC`。

**GREEN 验证**:
- 命令：`GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation' -count=1`
- 结果：通过，`ok github.com/Wei-Shaw/sub2api/internal/service 0.598s`。

**风险点**:
- 手动 daily 会重算 end 所在 UTC 整天；与 repository 按日期删除再插入的粒度一致，但范围较大时会多覆盖 end 当天剩余小时。
