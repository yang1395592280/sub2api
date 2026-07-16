## Task 3 Report

### 状态

DONE_WITH_CONCERNS

### 摘要

已按 brief 完成 Business Analytics 聚合表与聚合 repository/service interface/provider：

- 在 migration 中追加独立于 `usage_dashboard_*` 的 `business_usage_daily`、`business_usage_weekly`、`business_usage_daily_users`。
- 新增 `BusinessAnalyticsAggregationRepository` service interface。
- 新增 raw SQL aggregation repository，提供 `RecomputeDaily` / `RecomputeWeekly`。
- 注册 `NewBusinessAnalyticsAggregationRepository` 到 repository provider set。
- 未实现 read API handlers、scheduler service、frontend worker。

### 文件

- `backend/migrations/158_business_analytics.sql`
- `backend/internal/repository/business_analytics_aggregation_repo.go`
- `backend/internal/repository/business_analytics_aggregation_repo_test.go`
- `backend/internal/service/business_analytics.go`
- `backend/internal/repository/wire.go`

### TDD RED/GREEN

RED：

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalyticsAggregation'
```

结果：失败，报 `undefined: newBusinessAnalyticsAggregationRepositoryWithSQL`，符合“测试先行，repo 尚不存在”的预期。

GREEN：

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalyticsAggregation'
```

结果：通过。

### 测试命令结果

目标测试：

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalyticsAggregation'
```

结果：

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	(cached)
```

扩展验证：

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository ./internal/service
```

结果：`internal/service` 通过；`internal/repository` 失败于既有 `usage_log_repo_request_type_test.go`，错误为 usage log create sqlmock 期望 50 个参数但实际 53 个参数。该失败不来自 Task 3 新增文件，本任务未修改该既有测试。

### 提交 hash

未提交。

原因：当前 index 已存在 `.superpowers/sdd/task-1-report.md` staged scratch。用户明确要求提交前必须确认 `git diff --cached --name-only` 中没有 `.superpowers`；我不能回滚或 unstage 其他任务 scratch，因此无法安全提交业务文件。

### 自检 / concerns

- 已确认 business aggregates 使用独立表名，不依赖 `usage_dashboard_*`。
- Unknown `group_id` / `account_id` / `channel_id` 在 business aggregates 中用 `COALESCE(..., 0)` 写 0。
- 毛利润口径使用 `SUM(actual_cost) - SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1))`。
- daily/weekly 主表按 migration 主键维度聚合；`channel_id` 与 `platform` 使用聚合值填充，避免同一 bucket/group/account 多渠道或多平台记录触发主键冲突。
- `platform` 复用 repository 既有有效平台表达式 `COALESCE(NULLIF(g.platform,''), a.platform)`。
- Concern：未提交，需先处理现有 `.superpowers` staged scratch 后，再显式 pathspec add/commit Task 3 业务文件。
