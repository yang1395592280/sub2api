# Business Analytics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an admin business analytics feature that tracks revenue, channel cost, gross profit, profit margin, group pricing impact, channel account performance, and per-record profit details.

**Architecture:** Add immutable usage-level channel price snapshots first, then build an independent business analytics aggregation pipeline and admin API on top of `usage_logs`. Keep automatic channel price refresh and business aggregation out of the request hot path, reusing existing account upstream balance refresh and dashboard aggregation patterns.

**Tech Stack:** Go, Gin, Ent, PostgreSQL migrations, raw SQL repositories for aggregation, Wire dependency injection, Vue 3, TypeScript, Vite, existing admin API/client/component patterns.

## Global Constraints

- 默认使用简体中文沟通和后台文案。
- Do not change existing user billing or account scheduling semantics.
- Gross profit is `SUM(usage_logs.actual_cost) - SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1))`.
- Historical profit must use usage-time snapshots; current group/account prices must not rewrite historical meaning.
- Automatic channel price refresh must never run in the request hot path.
- Automatic channel price refresh failures must not clear or zero the last valid `accounts.channel_price`.
- Business analytics aggregation tables must be independent from existing `usage_dashboard_*` tables.
- Unknown `group_id`, `account_id`, or `channel_id` in business aggregates must be written as `0`, not `NULL`.
- First implementation may mark legacy rows without channel price snapshots as approximate, using existing account cost fields for cost.
- Prefer focused tests around touched logic; do not do unrelated refactors.

---

## File Structure

### Database and Ent

- Modify `backend/ent/schema/usage_log.go`
  - Add channel price snapshot fields.
- Create `backend/migrations/158_business_analytics.sql`
  - Add usage snapshot columns.
  - Add business aggregate tables and indexes.
- Generated Ent files under `backend/ent/**`
  - Regenerate after schema changes if the project's code generation is available; otherwise update only handwritten service/repository structs and migration in the first pass, then regenerate in the implementation checkpoint.

### Service Layer

- Modify `backend/internal/service/usage_log.go`
  - Add `ChannelPriceSnapshot`, `ChannelPriceSource`, `ChannelPriceRefreshedAt` fields to service usage log model.
- Modify usage recording paths in `backend/internal/service/gateway_service.go` and OpenAI usage paths that build `service.UsageLog`
  - Populate channel price snapshot from selected account.
- Create `backend/internal/service/business_analytics.go`
  - Request/response models and service methods.
- Create `backend/internal/service/business_analytics_aggregation.go`
  - Aggregation service, scheduling, recompute entry points.
- Create `backend/internal/service/channel_price_refresh_job.go`
  - Periodic automatic price refresh service using `OpenAIUpstreamBalanceService`.
- Modify `backend/internal/service/wire.go`
  - Add providers for business analytics service, aggregation service, and channel price refresh job.

### Repository Layer

- Modify `backend/internal/repository/usage_log_repo.go`
  - Insert/select usage snapshot fields.
- Create `backend/internal/repository/business_analytics_repo.go`
  - Raw SQL read API for overview, groups, channels, details.
- Create `backend/internal/repository/business_analytics_aggregation_repo.go`
  - Raw SQL aggregate/recompute API.
- Modify `backend/internal/repository/wire.go`
  - Add repository providers.

### Handler and Routes

- Create `backend/internal/handler/admin/business_analytics_handler.go`
  - Gin handlers for overview, groups, channels, price impact, records, export.
- Modify `backend/internal/handler/wire.go`
  - Add admin handler provider and `AdminHandlers.BusinessAnalytics`.
- Modify `backend/internal/handler/endpoint.go` or the existing admin handler struct file that defines `AdminHandlers`
  - Add `BusinessAnalytics *admin.BusinessAnalyticsHandler`.
- Modify `backend/internal/server/routes/admin.go`
  - Register `/admin/business-analytics` routes.

### Config and Startup

- Modify `backend/internal/config/config.go`
  - Add business analytics aggregation and channel refresh config with conservative defaults.
- Modify `backend/cmd/server/wire.go` and generated `backend/cmd/server/wire_gen.go`
  - Wire new services and cleanup hooks.

### Frontend

- Create `frontend/src/api/admin/businessAnalytics.ts`
  - Typed admin API client.
- Modify `frontend/src/api/admin/index.ts`
  - Export `businessAnalytics`.
- Modify `frontend/src/router/index.ts`
  - Add `/admin/business-analytics` route.
- Create `frontend/src/views/admin/BusinessAnalyticsView.vue`
  - Page shell and tabs.
- Create focused components under `frontend/src/components/admin/businessAnalytics/`
  - `BusinessAnalyticsFilters.vue`
  - `BusinessOverviewTab.vue`
  - `BusinessGroupsTab.vue`
  - `BusinessChannelsTab.vue`
  - `BusinessPriceImpactTab.vue`
  - `BusinessRecordsTab.vue`
- Modify i18n files, especially `frontend/src/i18n/locales/zh.ts`
  - Add Chinese labels.

---

### Task 1: Usage Channel Price Snapshot

**Files:**
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Modify: usage recording code paths that instantiate `service.UsageLog`, starting with `backend/internal/service/gateway_service.go`
- Create or modify tests in `backend/internal/repository/usage_log_repo_request_type_test.go` and `backend/internal/service/gateway_record_usage_test.go`

**Interfaces:**
- Produces service fields:
  - `UsageLog.ChannelPriceSnapshot *float64`
  - `UsageLog.ChannelPriceSource *string`
  - `UsageLog.ChannelPriceRefreshedAt *time.Time`
- Produces DB columns:
  - `usage_logs.channel_price_snapshot`
  - `usage_logs.channel_price_source`
  - `usage_logs.channel_price_refreshed_at`

- [ ] **Step 1: Write repository insert/select failing test**

Add a test near existing usage log create/get tests that creates a usage row with channel price snapshot fields and reads it back.

Expected assertion shape:

```go
price := 0.123456
source := "upstream_balance"
refreshedAt := time.Now().UTC().Truncate(time.Second)
created, duplicate, err := repo.Create(ctx, &service.UsageLog{
    UserID: user.ID,
    APIKeyID: apiKey.ID,
    AccountID: account.ID,
    RequestID: "snapshot-test",
    Model: "gpt-5",
    TotalCost: 1,
    ActualCost: 1.5,
    RateMultiplier: 1.5,
    ChannelPriceSnapshot: &price,
    ChannelPriceSource: &source,
    ChannelPriceRefreshedAt: &refreshedAt,
    CreatedAt: time.Now().UTC(),
})
require.NoError(t, err)
require.True(t, created)
require.False(t, duplicate)
got, err := repo.GetByID(ctx, createdID)
require.NoError(t, err)
require.NotNil(t, got.ChannelPriceSnapshot)
require.InDelta(t, price, *got.ChannelPriceSnapshot, 0.000001)
require.Equal(t, source, *got.ChannelPriceSource)
require.WithinDuration(t, refreshedAt, *got.ChannelPriceRefreshedAt, time.Second)
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run from `backend`:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestUsageLog.*ChannelPriceSnapshot|TestUsageLogRepository.*ChannelPriceSnapshot'
```

Expected: FAIL because fields/columns are not implemented.

- [ ] **Step 3: Add Ent schema fields and migration**

In `backend/ent/schema/usage_log.go`, add:

```go
field.Float("channel_price_snapshot").
    Optional().
    Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(12,6)"}).
    Comment("渠道价格快照：本次请求发生时账号的渠道进价"),
field.String("channel_price_source").
    MaxLen(32).
    Optional().
    Nillable().
    Comment("渠道价格来源：manual/upstream_balance/fallback"),
field.Time("channel_price_refreshed_at").
    Optional().
    Nillable().
    SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).
    Comment("渠道价格最近刷新时间快照"),
```

In `backend/migrations/158_business_analytics.sql`, start with:

```sql
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS channel_price_snapshot DECIMAL(12,6),
    ADD COLUMN IF NOT EXISTS channel_price_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS channel_price_refreshed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_usage_logs_channel_price_refreshed_at
    ON usage_logs (channel_price_refreshed_at)
    WHERE channel_price_refreshed_at IS NOT NULL;
```

- [ ] **Step 4: Add service model fields**

In `backend/internal/service/usage_log.go`, add to `UsageLog`:

```go
// ChannelPriceSnapshot records the account channel price observed when this usage was written.
ChannelPriceSnapshot *float64
// ChannelPriceSource records where ChannelPriceSnapshot came from.
ChannelPriceSource *string
// ChannelPriceRefreshedAt records when the source channel price was last refreshed.
ChannelPriceRefreshedAt *time.Time
```

- [ ] **Step 5: Update repository insert/select**

In `backend/internal/repository/usage_log_repo.go`:

- Add the three columns to `usageLogSelectColumns`.
- Add scan targets and mapper assignment.
- Add insert placeholders/args in single and batch create paths.
- Preserve `nil` values for legacy rows.

Use existing `nullFloat64Ptr` / nullable string/time helpers, or add focused helpers beside existing mapper helpers if needed.

- [ ] **Step 6: Populate snapshot from selected account**

Add a small helper in the service layer, for example in `backend/internal/service/usage_log_helpers.go`:

```go
func applyChannelPriceSnapshot(log *UsageLog, account *Account) {
    if log == nil || account == nil || account.ChannelPrice == nil {
        return
    }
    price := *account.ChannelPrice
    log.ChannelPriceSnapshot = &price
    source := "manual"
    if status := account.GetExtraString("upstream_balance_status"); status != "" {
        source = "upstream_balance"
    }
    log.ChannelPriceSource = &source
    if updatedAt := account.GetExtraString("upstream_balance_updated_at"); updatedAt != "" {
        if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
            log.ChannelPriceRefreshedAt = &ts
        }
    }
}
```

Call this helper in usage recording paths immediately before `usageLogRepo.Create` / async enqueue. Do not call upstream balance refresh from this helper.

- [ ] **Step 7: Run focused tests**

Run from `backend`:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'UsageLog.*ChannelPriceSnapshot|UsageLogRepository.*ChannelPriceSnapshot'
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'RecordUsage|ChannelPriceSnapshot'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/ent/schema/usage_log.go backend/migrations/158_business_analytics.sql backend/internal/service/usage_log.go backend/internal/service/usage_log_helpers.go backend/internal/repository/usage_log_repo.go backend/internal/repository/*usage*test.go backend/internal/service/*usage*test.go
git commit -m "feat: snapshot channel price on usage logs"
```

---

### Task 2: Automatic Channel Price Refresh Job

**Files:**
- Create: `backend/internal/service/channel_price_refresh_job.go`
- Test: `backend/internal/service/channel_price_refresh_job_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/cmd/server/wire.go`
- Modify generated wiring if Wire is not run automatically: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes: `OpenAIUpstreamBalanceService.Refresh(ctx, accountID)`
- Consumes account repository list capability. If no suitable list method exists, add:
  - `ListUpstreamBalanceRefreshCandidates(ctx context.Context, limit int) ([]*Account, error)`
- Produces:
  - `ChannelPriceRefreshJob.Start()`
  - `ChannelPriceRefreshJob.Stop()`
  - `ChannelPriceRefreshJob.RunOnce(ctx context.Context) ChannelPriceRefreshResult`

- [ ] **Step 1: Write job behavior tests**

Create tests covering:

```go
func TestChannelPriceRefreshJob_RunOnceRefreshesEligibleAccountsAndKeepsGoingAfterFailure(t *testing.T)
func TestChannelPriceRefreshJob_DisabledDoesNothing(t *testing.T)
func TestChannelPriceRefreshJob_UsesConcurrencyLimit(t *testing.T)
```

Use stubs:

```go
type channelPriceRefreshAccountRepoStub struct {
    accounts []*service.Account
}

type upstreamBalanceRefresherStub struct {
    mu sync.Mutex
    calls []int64
    errors map[int64]error
}
```

Assert success/failed counts and that all eligible accounts were attempted even when one fails.

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestChannelPriceRefreshJob'
```

Expected: FAIL because job does not exist.

- [ ] **Step 3: Implement config**

Add config struct in `backend/internal/config/config.go`:

```go
type ChannelPriceRefreshConfig struct {
    Enabled bool `mapstructure:"enabled" yaml:"enabled"`
    IntervalSeconds int `mapstructure:"interval_seconds" yaml:"interval_seconds"`
    Concurrency int `mapstructure:"concurrency" yaml:"concurrency"`
    TimeoutSeconds int `mapstructure:"timeout_seconds" yaml:"timeout_seconds"`
}
```

Add it under an existing admin/ops/dashboard config grouping or a new `BusinessAnalytics` config. Defaults:

- `Enabled: false` for safest rollout unless user explicitly enables in config.
- `IntervalSeconds: 600`
- `Concurrency: 3`
- `TimeoutSeconds: 30`

- [ ] **Step 4: Implement service**

Implement `ChannelPriceRefreshJob` with:

```go
type ChannelPriceRefreshResult struct {
    Attempted int
    Success int
    Failed int
}
```

Rules:

- Return immediately when disabled.
- Clamp interval to 10 minutes if invalid.
- Clamp concurrency to 1-5.
- Use per-account timeout.
- Log errors with `account_id`, but do not return early.

- [ ] **Step 5: Wire job startup and cleanup**

Add provider:

```go
func ProvideChannelPriceRefreshJob(repo AccountRepository, refresher *OpenAIUpstreamBalanceService, timingWheel *TimingWheelService, cfg *config.Config) *ChannelPriceRefreshJob {
    job := NewChannelPriceRefreshJob(repo, refresher, timingWheel, cfg)
    job.Start()
    return job
}
```

Add cleanup hook in `backend/cmd/server/wire.go` / `wire_gen.go` only if the job owns goroutines/tickers that require `Stop()`.

- [ ] **Step 6: Run tests**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestChannelPriceRefreshJob|TestOpenAIUpstreamBalanceServiceRefresh'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/channel_price_refresh_job.go backend/internal/service/channel_price_refresh_job_test.go backend/internal/service/wire.go backend/internal/config/config.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: add automatic channel price refresh job"
```

---

### Task 3: Business Analytics Aggregation Tables and Repository

**Files:**
- Modify: `backend/migrations/158_business_analytics.sql`
- Create: `backend/internal/repository/business_analytics_aggregation_repo.go`
- Test: `backend/internal/repository/business_analytics_aggregation_repo_test.go`
- Create: `backend/internal/service/business_analytics.go`
- Modify: `backend/internal/repository/wire.go`

**Interfaces:**
- Produces repository interface:

```go
type BusinessAnalyticsAggregationRepository interface {
    RecomputeDaily(ctx context.Context, startDate, endDate time.Time) error
    RecomputeWeekly(ctx context.Context, weekStart time.Time) error
}
```

- Produces aggregate tables:
  - `business_usage_daily`
  - `business_usage_weekly`
  - `business_usage_daily_users`

- [ ] **Step 1: Write aggregation repository tests**

Use sqlmock or existing repository integration style. Cover SQL intent:

- `RecomputeDaily` deletes range then inserts aggregate rows.
- `COALESCE(group_id, 0)` and `COALESCE(account_id, 0)` are used.
- `channel_cost` uses existing account cost expression.
- `missing_channel_price_records` counts rows with `channel_price_snapshot IS NULL`.

- [ ] **Step 2: Run tests to verify failure**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestBusinessAnalyticsAggregation'
```

Expected: FAIL because repo does not exist.

- [ ] **Step 3: Complete migration tables**

Append to `backend/migrations/158_business_analytics.sql`:

```sql
CREATE TABLE IF NOT EXISTS business_usage_daily (
    bucket_date DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    channel_id BIGINT NOT NULL DEFAULT 0,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    requests BIGINT NOT NULL DEFAULT 0,
    active_users BIGINT NOT NULL DEFAULT 0,
    active_api_keys BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    avg_group_rate_multiplier NUMERIC(10,4),
    avg_channel_price NUMERIC(12,6),
    missing_channel_price_records BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_date, group_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_business_usage_daily_group_date
    ON business_usage_daily (group_id, bucket_date DESC);
CREATE INDEX IF NOT EXISTS idx_business_usage_daily_account_date
    ON business_usage_daily (account_id, bucket_date DESC);

CREATE TABLE IF NOT EXISTS business_usage_weekly (
    week_start DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    channel_id BIGINT NOT NULL DEFAULT 0,
    platform VARCHAR(50) NOT NULL DEFAULT '',
    requests BIGINT NOT NULL DEFAULT 0,
    active_users BIGINT NOT NULL DEFAULT 0,
    active_api_keys BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    avg_group_rate_multiplier NUMERIC(10,4),
    avg_channel_price NUMERIC(12,6),
    missing_channel_price_records BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (week_start, group_id, account_id)
);

CREATE TABLE IF NOT EXISTS business_usage_daily_users (
    bucket_date DATE NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    account_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL,
    requests BIGINT NOT NULL DEFAULT 0,
    revenue NUMERIC(20,10) NOT NULL DEFAULT 0,
    channel_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    gross_profit NUMERIC(20,10) NOT NULL DEFAULT 0,
    PRIMARY KEY (bucket_date, group_id, account_id, user_id)
);
```

- [ ] **Step 4: Implement aggregation repository**

Implement raw SQL recompute:

- Delete target date range from daily tables.
- Insert grouped rows from `usage_logs`.
- Use:

```sql
COALESCE(SUM(ul.actual_cost), 0) AS revenue,
COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS channel_cost,
COALESCE(SUM(ul.actual_cost), 0) - COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS gross_profit
```

- Use weighted average for group rate:

```sql
CASE WHEN COUNT(*) > 0 THEN SUM(ul.rate_multiplier * GREATEST(ul.actual_cost, 0.000000001)) / SUM(GREATEST(ul.actual_cost, 0.000000001)) END
```

- Use simple average for `channel_price_snapshot` over non-null rows unless tests require a weighted variant.

- [ ] **Step 5: Register repository provider**

Add `NewBusinessAnalyticsAggregationRepository` to `backend/internal/repository/wire.go`.

- [ ] **Step 6: Run tests**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalyticsAggregation'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/migrations/158_business_analytics.sql backend/internal/repository/business_analytics_aggregation_repo.go backend/internal/repository/business_analytics_aggregation_repo_test.go backend/internal/service/business_analytics.go backend/internal/repository/wire.go
git commit -m "feat: add business analytics aggregation repository"
```

---

### Task 4: Business Analytics Read API Backend

**Files:**
- Create: `backend/internal/repository/business_analytics_repo.go`
- Test: `backend/internal/repository/business_analytics_repo_test.go`
- Create/modify: `backend/internal/service/business_analytics.go`
- Create: `backend/internal/handler/admin/business_analytics_handler.go`
- Test: `backend/internal/handler/admin/business_analytics_handler_test.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: admin handler struct file
- Modify: `backend/internal/server/routes/admin.go`

**Interfaces:**
- Produces service methods:

```go
func (s *BusinessAnalyticsService) GetOverview(ctx context.Context, filter BusinessAnalyticsFilter) (*BusinessOverviewResponse, error)
func (s *BusinessAnalyticsService) GetGroups(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessGroupRow, error)
func (s *BusinessAnalyticsService) GetChannels(ctx context.Context, filter BusinessAnalyticsFilter) ([]BusinessChannelRow, error)
func (s *BusinessAnalyticsService) GetPriceChangeImpact(ctx context.Context, input PriceChangeImpactInput) (*PriceChangeImpactResponse, error)
func (s *BusinessAnalyticsService) GetRecords(ctx context.Context, filter BusinessRecordsFilter) (*BusinessRecordsResponse, error)
```

- Produces routes under `/api/v1/admin/business-analytics`.

- [ ] **Step 1: Write handler tests for route shape and validation**

Create tests that assert:

- Missing/invalid date range returns 400.
- `/overview` returns `gross_profit` and `profit_margin`.
- `/price-change-impact` requires `group_id` and valid `change_date`.

Use a stub service interface if the handler accepts an interface; otherwise instantiate service with a stub repository.

- [ ] **Step 2: Run tests to verify failure**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'
```

Expected: FAIL because handler does not exist.

- [ ] **Step 3: Implement repository read methods**

Read from aggregate tables for date ranges ending before today. For first pass, allow today's range to query `usage_logs` directly in repository when `end_date >= today`.

Implement:

- Overview totals and trend rows.
- Group rows with joins to `groups` for current group name/rate.
- Channel rows with joins to `accounts` for account name/current channel price and `extra` balance status.
- Records from `usage_logs` with joins to user/api key/group/account.

- [ ] **Step 4: Implement service calculations**

Centralize derived metrics:

```go
func ProfitMargin(revenue, grossProfit float64) *float64 {
    if revenue == 0 {
        return nil
    }
    v := grossProfit / revenue
    return &v
}
```

Also calculate:

- `revenue_per_active_user`
- `profit_per_active_user`
- previous-period comparison for group rows.

- [ ] **Step 5: Implement handlers and route registration**

Add:

```go
func registerBusinessAnalyticsRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
    analytics := admin.Group("/business-analytics")
    analytics.GET("/overview", h.Admin.BusinessAnalytics.GetOverview)
    analytics.GET("/groups", h.Admin.BusinessAnalytics.GetGroups)
    analytics.GET("/groups/:id/channels", h.Admin.BusinessAnalytics.GetGroupChannels)
    analytics.GET("/channels", h.Admin.BusinessAnalytics.GetChannels)
    analytics.GET("/channels/:id/groups", h.Admin.BusinessAnalytics.GetChannelGroups)
    analytics.GET("/price-change-impact", h.Admin.BusinessAnalytics.GetPriceChangeImpact)
    analytics.GET("/records", h.Admin.BusinessAnalytics.GetRecords)
    analytics.GET("/export", h.Admin.BusinessAnalytics.Export)
}
```

Call it from `RegisterAdminRoutes`.

- [ ] **Step 6: Wire dependencies**

Update provider sets and generated wire files.

- [ ] **Step 7: Run focused backend tests**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ProfitMargin'
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics'
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/repository/business_analytics_repo.go backend/internal/repository/business_analytics_repo_test.go backend/internal/service/business_analytics.go backend/internal/handler/admin/business_analytics_handler.go backend/internal/handler/admin/business_analytics_handler_test.go backend/internal/handler/wire.go backend/internal/server/routes/admin.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: add business analytics admin API"
```

---

### Task 5: Business Analytics Aggregation Scheduler

**Files:**
- Create/modify: `backend/internal/service/business_analytics_aggregation.go`
- Test: `backend/internal/service/business_analytics_aggregation_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/cmd/server/wire.go`
- Modify generated wiring if needed: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes `BusinessAnalyticsAggregationRepository`.
- Produces `BusinessAnalyticsAggregationService.Start()`, `TriggerRecomputeRange(start, end time.Time) error`.

- [ ] **Step 1: Write scheduler service tests**

Cover:

- Disabled service does not schedule.
- `TriggerRecomputeRange` rejects invalid ranges.
- Scheduled aggregation calls repo for recent two hours.
- Concurrent scheduled runs do not overlap.

- [ ] **Step 2: Run tests to verify failure**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation'
```

Expected: FAIL because service does not exist.

- [ ] **Step 3: Implement config**

Add:

```go
type BusinessAnalyticsConfig struct {
    Enabled bool `mapstructure:"enabled" yaml:"enabled"`
    AggregationIntervalSeconds int `mapstructure:"aggregation_interval_seconds" yaml:"aggregation_interval_seconds"`
    LookbackSeconds int `mapstructure:"lookback_seconds" yaml:"lookback_seconds"`
    BackfillEnabled bool `mapstructure:"backfill_enabled" yaml:"backfill_enabled"`
    BackfillMaxDays int `mapstructure:"backfill_max_days" yaml:"backfill_max_days"`
}
```

Defaults:

- `Enabled: true`
- `AggregationIntervalSeconds: 300`
- `LookbackSeconds: 7200`
- `BackfillEnabled: true`
- `BackfillMaxDays: 90`

- [ ] **Step 4: Implement scheduler service**

Follow `DashboardAggregationService` pattern:

- Use `TimingWheelService.ScheduleRecurring`.
- Use atomic running flag.
- Use timeout per run.
- Recompute daily range for recent lookback.
- Recompute current week weekly aggregate once per scheduled run or at least once per hour.

- [ ] **Step 5: Wire service**

Provider should instantiate, call `Start()`, and return the service.

- [ ] **Step 6: Run tests**

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalyticsAggregation'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/business_analytics_aggregation.go backend/internal/service/business_analytics_aggregation_test.go backend/internal/service/wire.go backend/internal/config/config.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: schedule business analytics aggregation"
```

---

### Task 6: Frontend API Client and Route

**Files:**
- Create: `frontend/src/api/admin/businessAnalytics.ts`
- Test: `frontend/src/api/admin/__tests__/businessAnalytics.spec.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify router tests if route whitelist/title tests require updates.

**Interfaces:**
- Produces `adminAPI.businessAnalytics`.
- Produces route `/admin/business-analytics`.

- [ ] **Step 1: Write API tests**

Test that:

- `getOverview` calls `/admin/business-analytics/overview`.
- `getGroups` passes filters.
- `getRecords` passes pagination.

Use existing api client mock style in `frontend/src/api/__tests__`.

- [ ] **Step 2: Run tests to verify failure**

```bash
cd frontend
pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts
```

Expected: FAIL because module does not exist.

- [ ] **Step 3: Implement API client**

Create types:

```ts
export interface BusinessAnalyticsFilter {
  start_date: string
  end_date: string
  granularity?: 'day' | 'week'
  group_id?: number
  account_id?: number
}

export interface BusinessMetricSummary {
  revenue: number
  channel_cost: number
  gross_profit: number
  profit_margin: number | null
  active_users: number
  requests: number
  revenue_per_active_user: number | null
  profit_per_active_user: number | null
}
```

Implement functions: `getOverview`, `getGroups`, `getGroupChannels`, `getChannels`, `getChannelGroups`, `getPriceChangeImpact`, `getRecords`, `exportCsv`.

- [ ] **Step 4: Export from admin API barrel**

Import and add `businessAnalyticsAPI` in `frontend/src/api/admin/index.ts`.

- [ ] **Step 5: Add route**

Add route:

```ts
{
  path: '/admin/business-analytics',
  name: 'AdminBusinessAnalytics',
  component: () => import('@/views/admin/BusinessAnalyticsView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'Business Analytics',
    titleKey: 'admin.businessAnalytics.title',
    descriptionKey: 'admin.businessAnalytics.description'
  }
}
```

- [ ] **Step 6: Run frontend tests**

```bash
cd frontend
pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts src/router/__tests__/title.spec.ts src/router/__tests__/guards.spec.ts
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api/admin/businessAnalytics.ts frontend/src/api/admin/__tests__/businessAnalytics.spec.ts frontend/src/api/admin/index.ts frontend/src/router/index.ts frontend/src/router/__tests__
git commit -m "feat: add business analytics frontend API"
```

---

### Task 7: Frontend Business Analytics Page

**Files:**
- Create: `frontend/src/views/admin/BusinessAnalyticsView.vue`
- Create components under `frontend/src/components/admin/businessAnalytics/`
- Create tests under `frontend/src/views/admin/__tests__/BusinessAnalyticsView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`

**Interfaces:**
- Consumes `adminAPI.businessAnalytics` from Task 6.
- Produces admin UI with tabs:
  - overview
  - groups
  - channels
  - priceImpact
  - records

- [ ] **Step 1: Write view test**

Test:

- Page loads overview by default.
- Filters trigger API reload.
- Switching tabs calls corresponding API.
- Empty/missing snapshot state is rendered for records.

- [ ] **Step 2: Run test to verify failure**

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts
```

Expected: FAIL because page does not exist.

- [ ] **Step 3: Implement page shell**

Use existing admin page patterns:

- `AppLayout`.
- Existing `DateRangePicker`, `Select`, `Pagination`, table styles.
- Compact tabs.
- Avoid nested cards and oversized hero UI.

- [ ] **Step 4: Implement overview tab**

Show:

- revenue
- channel cost
- gross profit
- profit margin
- active users
- requests
- revenue per active user
- profit per active user

Use existing chart components where possible; if chart data shape does not match, start with tables and compact metric cards.

- [ ] **Step 5: Implement groups/channels tabs**

Tables must include:

- current price/rate
- interval average price/rate
- active users
- requests
- revenue
- channel cost
- gross profit
- profit margin
- previous period changes where API returns them

- [ ] **Step 6: Implement price impact tab**

Controls:

- group selector
- change date
- window days segmented control: 3 / 7 / 14

Display before/after comparison table and user gained/lost counts.

- [ ] **Step 7: Implement records tab**

Fields:

- time
- user
- API key
- group
- channel account
- model
- tokens/requests
- group rate snapshot
- channel price snapshot
- revenue
- channel cost
- gross profit
- profit margin

Rows with missing channel price snapshot show a subtle “历史近似” marker.

- [ ] **Step 8: Add Chinese i18n**

Add `admin.businessAnalytics` labels in `zh.ts`, including all tab titles and column labels.

- [ ] **Step 9: Run frontend tests**

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts
pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts
```

Expected: PASS.

- [ ] **Step 10: Start dev server and visually verify**

Run:

```bash
cd frontend
pnpm dev --host 127.0.0.1 --port 3000
```

Open `/admin/business-analytics` with an authenticated admin session if available. Verify:

- No overlapping text on desktop/mobile widths.
- Empty states are readable.
- Tables do not shift layout when loading.
- Tabs and filters are usable.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/views/admin/BusinessAnalyticsView.vue frontend/src/views/admin/__tests__/BusinessAnalyticsView.spec.ts frontend/src/components/admin/businessAnalytics frontend/src/i18n/locales/zh.ts
git commit -m "feat: add business analytics admin page"
```

---

### Task 8: End-to-End Verification and Documentation

**Files:**
- Modify if needed: `docs/superpowers/specs/2026-07-01-business-analytics-design.md`
- Modify if implementation behavior differs from the spec: `README_CN.md`

**Interfaces:**
- Verifies all previous task outputs together.

- [ ] **Step 1: Run backend focused test suite**

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ChannelPriceRefresh|RecordUsage|OpenAIUpstreamBalanceServiceRefresh'
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics|UsageLog.*ChannelPriceSnapshot'
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics'
```

Expected: PASS.

- [ ] **Step 2: Run frontend focused tests**

```bash
cd frontend
pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts src/views/admin/__tests__/BusinessAnalyticsView.spec.ts src/router/__tests__/guards.spec.ts
```

Expected: PASS.

- [ ] **Step 3: Run formatting**

```bash
cd backend
gofmt -w internal/service/business_analytics*.go internal/service/channel_price_refresh_job.go internal/repository/business_analytics*.go internal/handler/admin/business_analytics_handler.go
```

Expected: command exits 0.

- [ ] **Step 4: Check git diff for unintended changes**

```bash
git status --short
git diff --stat
```

Expected: only business analytics related files changed.

- [ ] **Step 5: Manual scenario verification**

Create or use test data with:

- One account in two groups.
- Group A rate lower than Group B.
- Same account channel price.
- Usage rows before and after group rate change.

Verify:

- Same account has same channel cost basis.
- Group revenue differs by rate.
- Gross profit equals revenue minus account cost.
- Historical rows do not change when current `accounts.channel_price` changes.

- [ ] **Step 6: Commit final verification/docs adjustments**

```bash
git add docs backend frontend
git commit -m "docs: update business analytics verification notes"
```

Skip this commit if no documentation or verification-note changes are needed.

---

## Self-Review

### Spec Coverage

- Usage channel price snapshots: Task 1.
- Automatic 10-minute channel price refresh: Task 2.
- Independent daily/weekly/user aggregate tables: Task 3.
- Overview, groups, channels, price impact, records APIs: Task 4.
- Aggregation scheduling and recompute: Task 5.
- Frontend API and route: Task 6.
- Admin page with overview/groups/channels/price impact/records: Task 7.
- Verification and manual business scenario: Task 8.

### Placeholder Scan

No unresolved placeholder markers are intentionally left. Any implementation-specific discovery should be resolved inside the referenced task before coding that task.

### Type Consistency

The plan consistently uses:

- `channel_price_snapshot`
- `channel_price_source`
- `channel_price_refreshed_at`
- `BusinessAnalyticsService`
- `BusinessAnalyticsAggregationRepository`
- `ChannelPriceRefreshJob`
