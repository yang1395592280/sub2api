# Task 4 Report: Business Analytics Read API Backend

## 状态

DONE_WITH_CONCERNS

## 摘要

- 实现 Business Analytics 后端读 API：repository read methods、BusinessAnalyticsService、admin handler、`/admin/business-analytics` 路由与 Wire wiring。
- 查询历史汇总默认读取 `business_usage_daily`，当查询范围包含今天时允许直接读取 `usage_logs`。
- 毛利润口径保持为 `SUM(actual_cost) - SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1))`。
- 未实现 aggregation scheduler service、frontend、worker。

## 文件

- `backend/internal/repository/business_analytics_repo.go`
- `backend/internal/repository/business_analytics_repo_test.go`
- `backend/internal/service/business_analytics.go`
- `backend/internal/service/business_analytics_test.go`
- `backend/internal/handler/admin/business_analytics_handler.go`
- `backend/internal/handler/admin/business_analytics_handler_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/wire.go`
- `backend/internal/service/wire.go`
- `backend/internal/server/routes/admin.go`
- `backend/cmd/server/wire_gen.go`

## TDD RED/GREEN

- RED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'` initially failed because `service.BusinessOverviewResponse`, `service.PriceChangeImpactResponse`, filters, rows, and handler types did not exist.
- GREEN: added service contracts/calculations, repository read methods, handler validation/routes, and focused tests.

## 测试命令结果

- PASS: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ProfitMargin'`
- PASS: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics'`
- PASS: `GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server ./internal/server/routes -run 'BusinessAnalytics|RegisterAdminRoutes|Wire|Initialize|^$'`
- BLOCKED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'`
  - 阻塞原因：同包既有测试 `internal/handler/admin/account_handler_available_models_test.go:67` 调用 `service.NewAccountTestService` 参数数量不匹配。该文件不属于 Task 4，按任务约束未修改。

## 提交 hash

4272fb9b4697507eb10c0f510ab7014fc464a5c4

## 自检 / concerns

- 未提交 `.superpowers` scratch/report 文件。
- `wire` 二进制不可用，已按现有生成文件风格手动同步 `backend/cmd/server/wire_gen.go`，并通过 `cmd/server` 编译测试。
- Handler 测试文件已写入，但因为 handler/admin 包既有非 Task 4 测试编译失败，无法单独通过 `go test ./internal/handler/admin -run 'BusinessAnalytics'` 验证。

## Task 4 review fix - 2026-07-01

- Fixed repository today-boundary detection so internal exclusive EndDate equal to today start uses business_usage_daily instead of usage_logs.
- Fixed overview response EndDate to return user-facing inclusive date by subtracting one day from internal exclusive end.
- Added focused repository, service, and handler tests for the boundary/contract.
- RED: repository focused test failed by matching usage_logs instead of business_usage_daily; service focused test failed expected 2026-06-02 actual 2026-06-03.
- GREEN: go test ./internal/repository, go test ./internal/service, go test ./internal/handler/admin passed with GOCACHE=/tmp/sub2api-go-cache.
- Risk: handler test uses time.Local to match existing ParseInUserLocation default behavior.

## Task 4 second review fix - 2026-07-01

- Clarified `/admin/business-analytics/channels/:id/groups` handler semantics: route `:id` is the channel account/account id and intentionally writes `filter.AccountID`.
- Added handler regression coverage proving `/channels/123/groups` passes `BusinessAnalyticsFilter.AccountID = 123` to `GetGroups`.
- Fixed historical multi-day read queries so overview, groups, and channels/account active user/API key counts use range-distinct semantics instead of summing daily distinct counts.
- RED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin ./internal/repository -run 'TestBusinessAnalyticsHandler_ChannelGroupsPathIDIsAccountID|TestBusinessAnalyticsRepository_GetOverviewHistoricalCountsDistinctUsersAndAPIKeysAcrossRange|TestBusinessAnalyticsRepository_GetGroupsHistoricalCountsDistinctUsersAcrossRange'` failed in repository because generated SQL still used `SUM(active_users)` / `SUM(active_api_keys)` from `business_usage_daily`.
- GREEN: same focused command passed; broader `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin ./internal/repository -run 'TestBusinessAnalytics'` also passed.
- Risk: `active_api_keys` for historical paths reads `usage_logs` for distinct `api_key_id` because no key-level business aggregate table exists.

## Task 4 third review fix - 2026-07-01

- Fixed historical business analytics daily WHERE builder to qualify dimension filters with the same alias prefix as the date column, so `b.bucket_date` also yields `b.group_id`, `b.account_id`, and `b.platform`.
- Qualified the groups previous-period historical aggregate with alias `p` to avoid bare daily filter columns there too.
- Added sqlmock intent coverage for overview/groups/channels historical active-count CTE filters and direct SQL assertions that generated historical queries do not contain bare ` group_id =`, ` account_id =`, or ` platform =`.
- RED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalytics'` failed because overview/groups/channels SQL still emitted bare `group_id`, `account_id`, and `platform` filters in `business_usage_daily b` joins.
- GREEN: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalytics'` passed.
- Risk: none known; usage_logs fallback still uses its existing `ul.` qualified builder.

## Task 4 fourth review fix - 2026-07-01

- Fixed GetGroups includes-today SQL so previous_period aliases business_usage_daily as p when previousWhere references p.bucket_date / p.group_id / p.account_id / p.platform.
- Added sqlmock intent coverage for GetGroups includes-today query with EndDate tomorrow, asserting previous_period uses FROM business_usage_daily p and remains executable/matchable.
- RED: GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalytics' -count=1 failed in TestBusinessAnalyticsRepository_GetGroupsIncludingTodayAliasesPreviousPeriod because actual SQL had FROM business_usage_daily WHERE p.bucket_date ... and did not match FROM business_usage_daily p.
- GREEN: GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalytics' -count=1 passed.
- Risk: none known; change is scoped to the previous_period source alias in the includes-today GetGroups branch.

## Task 4 fifth review fix - 2026-07-01

- Fixed admin business analytics BadRequest messages so user-visible validation errors are Simplified Chinese.
- Added handler tests asserting response.message for missing/invalid date range, group_id/account_id, change_date, days, and invalid path ids.
- RED: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics' -count=1` failed because messages were still English, including `start_date and end_date are required`, `group_id is required`, and `Invalid group id`.
- GREEN: `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics' -count=1` passed.
- Risk: CSV headers remain machine field names by review guidance; no API shape changes beyond localized BadRequest message text.
