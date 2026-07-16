# Task 8 Report: End-to-End Verification and Hardening

## Summary

Task 8 verification completed with one focused hardening fix.

Fix made:

- `backend/internal/repository/openai_auto_scheduler_repo.go`: changed OpenAI auto scheduler event message truncation so the 1000-byte cap never returns invalid UTF-8 when the message contains multi-byte characters.
- `backend/internal/repository/openai_auto_scheduler_repo_test.go`: added regression coverage for truncating at a rune boundary.

Commit:

- `502dc76fce3f7ac4b98bdb11c1d36c13b538fe03` (`fix: harden openai auto scheduler`)

## Commands Run And Results

### Focused Backend Tests

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAIAutoScheduler|OpenAIGatewayService_SelectAccount|OpenAI.*Scheduler'
```

Result: PASS

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	1.131s
```

Final rerun after hardening:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	(cached)
```

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run OpenAIAutoScheduler
```

Result: PASS

```text
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin	0.551s
```

Final rerun after hardening:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin	(cached)
```

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run OpenAIAutoScheduler
```

Result: PASS

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	0.703s
```

Final rerun after hardening:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	0.543s
```

### TDD Hardening Check

RED command:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run TestOpenAIAutoSchedulerRepository_InsertScoreEventTruncatesMessageAtRuneBoundary
```

Result: FAIL as expected before the fix.

```text
--- FAIL: TestOpenAIAutoSchedulerRepository_InsertScoreEventTruncatesMessageAtRuneBoundary (0.01s)
    openai_auto_scheduler_repo_test.go:211:
        Error:       Should be true
```

GREEN command:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run TestOpenAIAutoSchedulerRepository_InsertScoreEventTruncatesMessageAtRuneBoundary
```

Result: PASS

```text
ok  	github.com/Wei-Shaw/sub2api/internal/repository	0.493s
```

### Broader Backend Tests

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin ./internal/server ./internal/repository
```

Initial result: PASS

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	47.205s
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin	0.897s
?   	github.com/Wei-Shaw/sub2api/internal/server	[no test files]
ok  	github.com/Wei-Shaw/sub2api/internal/repository	1.632s
```

Final rerun after hardening:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	(cached)
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin	(cached)
?   	github.com/Wei-Shaw/sub2api/internal/server	[no test files]
ok  	github.com/Wei-Shaw/sub2api/internal/repository	1.653s
```

No `-tags unit` was needed for these packages in this worktree.

### Frontend Tests

`frontend/node_modules` was missing in the feature worktree. Following the safe Task 7 approach, I temporarily symlinked:

```text
frontend/node_modules -> /Volumes/workspace/中转站/sub2api/frontend/node_modules
```

The temporary symlink was removed after verification.

Brief command:

```bash
cd frontend
pnpm test -- OpenAIAutoScheduler openaiAutoScheduler router AppSidebar
```

Result: tests passed, but the command entered Vitest watch mode because `pnpm test` maps to `vitest`.

```text
Test Files  139 passed (139)
Tests       820 passed (820)
PASS  Waiting for file changes...
```

Non-watch verification:

```bash
cd frontend
pnpm test:run -- OpenAIAutoScheduler openaiAutoScheduler router AppSidebar
```

Result: PASS

```text
Test Files  139 passed (139)
Tests       820 passed (820)
```

Final rerun after hardening:

```text
Test Files  139 passed (139)
Tests       820 passed (820)
```

Notes:

- The run includes existing stderr/warnings from unrelated tests, including missing localStorage file warnings, unresolved mocked components in some specs, an expected invalid JSON auth-store test log, an expected group usage comparison failure log, and vue-i18n runtime compiler warnings. Exit code was 0.

Type-check brief command:

```bash
cd frontend
pnpm type-check
```

Result: FAIL because this project has no `type-check` script.

```text
ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL Command "type-check" not found
Did you mean "pnpm typecheck"?
```

Existing project script:

```bash
cd frontend
pnpm typecheck
```

Result: PASS

```text
vue-tsc --noEmit
```

Final rerun after hardening:

```text
vue-tsc --noEmit
```

Exit code: 0.

## Manual Scenario Check Status

Manual end-to-end live scenario was not fully executed.

Reason:

- No live upstream/test environment with controllable OpenAI accounts, group traffic, slow upstream simulation, 5xx simulation, and probe execution was available in this task context.
- Prior Task 7 visual QA against `http://localhost:3000` was blocked by policy; I did not fight that policy.

Covered by automated tests:

- Global and group-level OpenAI auto scheduler gating.
- Ungrouped/fallback behavior through selector/gateway tests.
- Score ordering and open-circuit skip behavior.
- Slow/error/probe score state transitions.
- Admin API and frontend dashboard behavior.

Not covered manually:

- Real traffic through group A versus group B in a live deployment.
- Real upstream latency degradation and 5xx behavior.
- Real probe cycle from `open` to `half_open` to `running`.

## Diff Scope Summary

`git merge-base main HEAD` returned:

```text
c275422251e72750bebe53e41fcf59db7f83fe6b
```

`git diff --stat main...HEAD` and `git diff --name-only main...HEAD` were run, but this view includes a much larger branch delta unrelated to this feature line, including release/deploy/docs/workbench/image API/user/admin changes. It is not a clean scope view for Task 8.

The plan base provided in the task, `d6633d8d..HEAD`, is the more appropriate scope view for the OpenAI auto scheduler feature. It showed:

```text
83 files changed, 21493 insertions(+), 78 deletions(-)
```

Scope from `d6633d8d..HEAD` is limited to expected Task 4-7 areas:

- OpenAI auto scheduler repository/service/probe/selector/scoring.
- Gateway selection and fallback integration.
- Admin handler/routes/API contract.
- Group field plumbing and generated Ent files.
- Migration `117_openai_auto_scheduler.sql`.
- Frontend admin API, route/sidebar/i18n, dashboard page, and tests.
- Task reports for prior SDD tasks.

Task 8 working diff before commit was:

```text
2 files changed, 29 insertions(+), 1 deletion(-)
```

## Remaining Risks And Notes

- Manual live E2E remains unverified without a suitable environment.
- Frontend visual QA remains not completed due the earlier localhost browser policy block.
- Frontend brief uses `pnpm type-check`, but the actual package script is `pnpm typecheck`.
- The frontend test command in the brief uses `pnpm test`, which enters watch mode; `pnpm test:run` is the reliable CI-style equivalent.
- Frontend local-only state/search filtering remains coherent up to backend `page_size=200`; backend global state/search filtering was out of scope.
- `channel_price` was not touched by Task 8 and remains outside billing calculations.

---

# Unified Scheduler Engine Task 8 Report (2026-07-14)

## Result

- Added `GET /openai-auto-scheduler/overview` and `GET /openai-auto-scheduler/health` before mutation routes.
- Overview supports only `1h`, `6h`, `24h`, and `7d`; the default is `6h`. Windows through 24h use hourly buckets and 7d uses six-hour buckets.
- PostgreSQL aggregates E2E P50/P90 from `e2e_first_token_ms`; selection P95 uses the documented `routing_ms` proxy. Slow causes use the shared scheduler slow threshold and uniquely classify queue, retry, or residual upstream TTFT.
- Probe ratio uses windowed physical probe signatures divided by physical probes plus real timed requests. Global aggregation distincts legacy per-group fan-out.
- Health uses two bounded SQL queries (count and rows) plus one deduplicated live-load batch per page. It joins group membership, exact physical health key, snapshot, concurrency capacity, load, waiting, and channel price without N+1 reads.
- Health decisions are current classifications only: `eligible`, `latency_tail`, `circuit_rejected`, `stale`, or `health_unavailable`. `sticky_escape_reason` is null because the endpoint has no request context; scheduler mode and shadow mode are separate fields.

## TDD Evidence

- Service/handler RED: compile failed on missing overview/health contracts and handler methods.
- Repository RED: compile failed on missing overview repository and query builders.
- Route RED: registered route table lacked overview and health.
- Review RED/GREEN: the initial probe query omitted slow/severe-slow probe outcomes; a SQL contract test now requires the nil-status probe signature. A second test locks inverse timestamp ordering for `snapshot_age_ms`.

## Verification

- Focused service/handler/repository/routes: PASS.
- Race, service `2.160s`, repository `3.577s`: PASS.
- Fresh full packages: service `58.273s`, handler `23.869s`, handler/admin `0.717s`, repository `3.107s`, routes `2.668s`, cmd/server `2.654s`: PASS.
- Task 8 unit API contract, overview and health: PASS (`0.587s`).
- Full legacy API contract has four unrelated stale expectations in API key/group/settings cases; both Task 8 cases pass.
- Wire generator ran twice successfully. Generated injection is stable. `backend/go.mod` and `backend/go.sum` have no delivered diff.
- `git diff --check`: PASS before staging.

## Concerns

- Legacy score events have no probe attempt ID or endpoint/transport source fields. Global probe deduplication therefore uses account, model, event type, and second-level timestamp; two genuine probes for the same account/model/event within one second could be merged. Slow probe outcomes are distinguished from real outcomes by their nil status code, matching current producers.
- Selection P95 remains a routing-stage proxy until a narrower persisted selection timer exists.

## Reviewer Fix Wave (2026-07-14)

### RED/GREEN

- Probe identity RED lacked an exact attempt timestamp across legacy group fan-out. `OccurredAt` is now optional on event/record input, one physical probe mints one nanosecond timestamp before the checker, and every group event reuses it. Zero values keep existing wall-clock/schema-default behavior.
- Health classification RED showed request-independent rows claiming `eligible` and `latency_tail`. Those classifications and group-best SQL were removed. Rows now report only `health_unavailable`, `stale`, `circuit_rejected`, `hard_filtered`, or `context_required`; sticky escape remains null.
- Capacity RED used account concurrency instead of `EffectiveLoadFactor`. SQL now implements load-factor > concurrency > 1 and the same value feeds the one page-level load batch.
- Deleted-group and timezone RED showed inconsistent filters and session-timezone-dependent 7d buckets. Every overview query excludes soft-deleted groups and six-hour buckets are explicitly aligned in UTC.

### Verification

- Focused service/repository/handler/routes: PASS.
- Race: service `2.220s`, repository `3.381s`, PASS.
- Fresh full packages: service `58.194s`, handler `22.270s`, handler/admin `0.448s`, repository `2.586s`, routes `3.358s`, cmd/server `2.228s`, PASS.
- Task 8 API contract: PASS (`0.474s`).
- Wire regenerated successfully with no generated source change. `go.mod` and delivered `go.sum` remain unchanged.

### Residual Notes

- Exact probe identity is available for new events. Historical rows written before this fix retain their old per-group timestamps and cannot be perfectly deduplicated without a migration/backfill, which this fix intentionally avoids.
- `context_required` is intentionally conservative: model capability, quota, queue, candidate-relative latency, and per-request hard gates are evaluated only by the real scheduler.

## Final Fix Wave (2026-07-14)

### RED/GREEN

- Separated audit identity from state time. `AuditCreatedAt` is optional on legacy record input and is used only for score-event audit `CreatedAt`; score state transitions always use processing wall-clock time. Probe fan-out retains one exact timestamp across groups without moving `LastCheckedAt` backward after a newer real outcome.
- Added known hard-filter inputs to the single health row query: group status/scheduler switch, account status/schedulable/temp block, auto-pause expiry, overload, and rate-limit reset. Known gates classify as `hard_filtered`; unknown request-dependent gates remain `context_required`.
- Added table-driven coverage for every known gate and nullable timestamp scanning, plus EffectiveLoadFactor capacity cases.

### Verification

- Focused service/repository/handler/routes: PASS (`1.754s`, `1.015s`, `0.532s`, `2.180s`).
- Race: service `3.054s`, repository `2.565s`, PASS.
- Fresh full packages: service `58.107s`, handler `22.645s`, handler/admin `0.518s`, repository `2.069s`, routes `3.422s`, cmd/server `2.313s`, PASS.
- Task 8 API contract: PASS (`0.428s`).
- Wire regenerated successfully; generated source stable; no go dependency diff; `git diff --check` PASS.

### Concerns

- Historical score events before `AuditCreatedAt` cannot be perfectly deduplicated because their fan-out timestamps were independently generated; no migration/backfill was added.
- `context_required` intentionally does not claim request-level model/capability/quota/queue/parent-health decisions.
