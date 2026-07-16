# Final Review Fix Report: OpenAI Auto Scheduler

Date: 2026-06-28

## What Was Fixed

- Made `channel_price` affect OpenAI auto-scheduler selection only:
  - Selector now derives a deterministic price adjustment from the current eligible candidate set.
  - Cheaper candidates receive a positive weighted selection-score adjustment; more expensive candidates receive a negative adjustment.
  - Existing eligibility filters, state tiers, priority, and LRU fallback remain intact.
- Preserved signed component score diagnostics:
  - `latency_score`, `error_score`, `recovery_score`, and `cost_score` are no longer constrained to `0..10000` in Ent schema/generated validators.
  - Repository now preserves signed component scores, clamped only to `-10000..10000`.
  - `final_score` and `base_score` remain bounded to `0..10000`.
- Made outcome recording concurrency-safe:
  - `OpenAIAutoSchedulerService.Record` and `ResetScore` now serialize read-compute-upsert per `(account_id, group_id, model)`.
  - Added concurrent error regression coverage proving consecutive counts survive concurrent records and trip the breaker.
- Updated runtime statistics in the pure scoring state machine:
  - `request_count`, `ttfb_sample_count`, `slow_rate`, `error_rate`, and `stuck_rate` now update from recorded outcomes.
- Fixed manual reset while global scheduler is disabled:
  - Manual reset bypasses the global enabled guard and actually resets existing score state.
- Fixed frontend score formatting:
  - Dashboard now renders internal basis points as normalized `0.0000` to `1.0000`, e.g. `8200` becomes `0.8200`.
- Removed the unused neutral selector constant.

## Tests Run And Results

- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./ent/schema -run TestOpenAIAutoSchedulerSchemas`
  - PASS: `ok github.com/Wei-Shaw/sub2api/ent/schema 2.298s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/service 48.005s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/repository`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/repository 1.833s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/handler/admin`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/handler/admin 0.837s`
- `pnpm test:run OpenAIAutoSchedulerView.spec.ts openaiAutoScheduler.spec.ts`
  - PASS: 2 files, 9 tests.

## Schema, Migration, And Generated File Changes

- Changed Ent schema for `OpenAIAutoSchedulerScoreState`:
  - Removed non-negative range validators from `latency_score`, `error_score`, `recovery_score`, and `cost_score`.
  - Kept `final_score` and `base_score` range validators at `0..10000`.
- Updated migration `117_openai_auto_scheduler.sql`:
  - Replaces `openai_auto_scheduler_score_states_score_check` so only `final_score` and `base_score` are constrained to `0..10000`.
- Regenerated Ent files with:
  - `GOCACHE=/tmp/sub2api-go-cache go generate ./ent`

## Billing Confirmation

- Billing, usage-cost, account rate multiplier, and usage log code were not changed.
- `channel_price` is used only for scheduler selection ranking in `openai_auto_scheduler_selector.go`.
- No gateway billing or usage-cost calculation path was modified for this review fix.

## Remaining Risks

- The service-level keyed mutex protects concurrent records within a single process. A horizontally scaled deployment could still need a database row lock or compare-and-update strategy for cross-process serialization.
- Price score is intentionally relative to the current candidate set; absolute price differences across different filtered sets are not directly comparable.

---

# Final Review Fix Report: Business Analytics

Date: 2026-07-01

## RED Tests

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ProfitMargin' -count=1`
  - RED: build failed because `BusinessGroupRow.AverageRateMultiplier` and `BusinessChannelRow.AverageChannelPrice` were missing.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics' -count=1`
  - RED: build failed because `BusinessAnalyticsFilter.Granularity`, average fields, and record channel price snapshot fields were missing.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics' -count=1`
  - RED: build failed because handler test expected `BusinessAnalyticsFilter.Granularity`.
- `pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts src/views/admin/__tests__/BusinessAnalyticsView.spec.ts`
  - RED: view test failed because groups/channels still rendered `admin.businessAnalytics.notProvidedByApi` instead of real average values.

## GREEN Tests

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'BusinessAnalytics|ProfitMargin' -count=1`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/service`.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'BusinessAnalytics' -count=1`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/repository`.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'BusinessAnalytics' -count=1`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/handler/admin`.
- `pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts src/views/admin/__tests__/BusinessAnalyticsView.spec.ts`
  - PASS: 2 files, 12 tests.
- `pnpm exec vue-tsc --noEmit`
  - PASS.

## Changes

- Added backend `granularity` parsing/validation and passed it through `BusinessAnalyticsFilter`.
- Historical `granularity=week` overview/trend reads use `business_usage_weekly` when the range excludes today; today-inclusive ranges keep `usage_logs` fallback.
- Historical trend active users are computed from `business_usage_daily_users` distinct user IDs instead of summing aggregate `active_users`.
- Groups/channels now return interval average rate/price fields from aggregate rows or `usage_logs` fallback snapshots.
- Records now return `channel_price_snapshot` and row-level `channel_price_snapshot_missing` for frontend markers.
- Frontend API types and admin view render real average fields and row-level missing snapshot markers.

## Concerns

- Groups/channels weekly historical queries use weekly aggregate rows for business totals and daily users / usage logs for distinct user and API key counts, preserving existing read behavior where weekly rows do not carry enough distinct cross-row detail.
- No billing, gross profit formula, account scheduling, or `usage_dashboard_*` path was changed.
