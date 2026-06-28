# Task 6 Report: Admin API for OpenAI Auto Scheduler

## What I Implemented

- Added `OpenAIAutoSchedulerHandler` with admin endpoints for:
  - `GET /api/v1/admin/openai-auto-scheduler/settings`
  - `PUT /api/v1/admin/openai-auto-scheduler/settings`
  - `GET /api/v1/admin/openai-auto-scheduler/groups`
  - `PUT /api/v1/admin/openai-auto-scheduler/groups/:id`
  - `GET /api/v1/admin/openai-auto-scheduler/scores`
  - `GET /api/v1/admin/openai-auto-scheduler/events`
  - `POST /api/v1/admin/openai-auto-scheduler/scores/:id/reset`
  - `POST /api/v1/admin/openai-auto-scheduler/scores/:id/probe`
- Added handler-side settings validation so invalid probe intervals, thresholds, breaker thresholds, cooldowns, cost weight, and recovery step are rejected before persistence.
- Enforced OpenAI-only group toggles by checking the group platform before calling `UpdateGroup`.
- Exposed score/event list responses with internal basis-point scores plus percent display fields.
- Added `ListEvents` facade to `OpenAIAutoSchedulerService` and `ListScoreEvents` to the repository.
- Implemented manual probe by loading the account, requiring OpenAI account type, calling the existing probe checker, and recording `probe_success` or `probe_error`.
- Registered routes in the real admin route structure and wired the handler through `handler.go`, `handler/wire.go`, and generated `cmd/server/wire_gen.go`.
- Updated API contract tests with representative scheduler settings/groups route coverage.

## What I Tested and Results

- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/handler/admin -run TestOpenAIAutoScheduler`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/handler/admin 1.229s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 -tags unit ./internal/server -run TestAPIContracts`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/server 0.942s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service -run TestOpenAIAutoScheduler`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/service 1.173s`
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/repository -run TestOpenAIAutoScheduler`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/repository 0.547s`
- `GOCACHE=/tmp/sub2api-go-cache go generate ./cmd/server`
  - PASS: Wire regenerated `backend/cmd/server/wire_gen.go`.
- `git diff --check`
  - PASS: no whitespace errors.

## TDD Evidence

### RED

Command:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run TestOpenAIAutoScheduler
```

Relevant failing output:

```text
internal/handler/admin/openai_auto_scheduler_handler_test.go:96:7: undefined: NewOpenAIAutoSchedulerHandler
FAIL github.com/Wei-Shaw/sub2api/internal/handler/admin [build failed]
```

Why expected:

- The tests were added first and referenced the new handler constructor and methods before production code existed.
- The failure proved the tests exercised the missing Task 6 API surface.

### GREEN

Command:

```bash
GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/handler/admin -run TestOpenAIAutoScheduler
```

Relevant passing output:

```text
ok github.com/Wei-Shaw/sub2api/internal/handler/admin 1.229s
```

Additional green verification:

```text
ok github.com/Wei-Shaw/sub2api/internal/server 0.942s
ok github.com/Wei-Shaw/sub2api/internal/service 1.173s
ok github.com/Wei-Shaw/sub2api/internal/repository 0.547s
```

## Files Changed

- `backend/internal/handler/admin/openai_auto_scheduler_handler.go`
- `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/server/api_contract_test.go`
- `backend/internal/service/openai_auto_scheduler_service.go`
- `backend/internal/service/openai_auto_scheduler_service_test.go`
- `backend/internal/repository/openai_auto_scheduler_repo.go`
- `.superpowers/sdd/task-6-report.md`

## Self-Review Findings

- Settings update rejects broken values instead of relying on service normalization.
- Group toggle checks the existing group platform before persistence; non-OpenAI groups return a normal 400 response.
- Handler methods return normal response errors for missing services, invalid IDs, missing models, repository errors, and nil group/account cases.
- Manual probe reuses the existing checker and scheduler record path, keeping upstream probe details out of the handler.
- Wire generation reuses one OpenAI auto scheduler service/checker for both admin handler and probe runner.
- Contract test stub methods were updated only to satisfy expanded service interfaces and keep existing contract tests compiling.

## Issues or Concerns

- Manual probe requires `group_id` and `model` query parameters because the score route only carries account ID. This matches the score identity used by existing reset/service APIs.
- `go generate ./cmd/server` without `GOCACHE` failed in the sandbox due to lack of permission for `/Users/jaydenyang/Library/Caches/go-build`; rerunning with `GOCACHE=/tmp/sub2api-go-cache` succeeded.

---

## Review Fix Report - 2026-06-28 14:16:31 CST

### What Changed

- Added `OpenAIAutoSchedulerService.RecordManualProbe` for admin/manual probe writes. It returns normal application errors when the scheduler is disabled, group identity is missing/disabled/not OpenAI, model is missing, or repository persistence fails.
- Kept hot-path `Record` best-effort by sharing the same internal record flow with `bestEffort=true`, so gateway/probe-runner degradation behavior remains unchanged.
- Changed ambiguous mutation routes from `/scores/:id/reset` and `/scores/:id/probe` to explicit account identity routes:
  - `POST /api/v1/admin/openai-auto-scheduler/scores/accounts/:account_id/reset`
  - `POST /api/v1/admin/openai-auto-scheduler/scores/accounts/:account_id/probe`
- Updated handler tests and API contract tests to document the explicit account route contract and required `group_id` + `model` query params.
- Changed `ResetScore` to require an existing score state for `(account_id, group_id, model)` and return `OPENAI_AUTO_SCHEDULER_SCORE_NOT_FOUND` instead of creating a misleading new score row.
- Added handler coverage for GET settings, GET groups, GET events, manual probe success/error recording, missing mutation query params, and non-OpenAI account rejection.

### TDD / Regression Evidence

RED checks before implementation:

```text
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoSchedulerService_(RecordManualProbe|ResetScore)'
FAIL: RecordManualProbe undefined

GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAIAutoSchedulerHandler_(Probe|Reset|GetSettings|ListGroups|ListEvents)'
FAIL: explicit account_id route tests returned 400 with old :id param contract; manual probe did not call strict record path.
```

GREEN verification after implementation:

```text
GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/handler/admin -run TestOpenAIAutoScheduler
ok github.com/Wei-Shaw/sub2api/internal/handler/admin 0.459s

GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service -run TestOpenAIAutoScheduler
ok github.com/Wei-Shaw/sub2api/internal/service 0.970s

GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/repository -run TestOpenAIAutoScheduler
ok github.com/Wei-Shaw/sub2api/internal/repository 0.569s

GOCACHE=/tmp/sub2api-go-cache go test -count=1 -tags unit ./internal/server -run TestAPIContracts
ok github.com/Wei-Shaw/sub2api/internal/server 0.222s

git diff --check
PASS: no output
```

### Wire

- Wire was not regenerated because provider signatures did not change.

### Risks / Notes

- The old ambiguous `/scores/:id/reset` and `/scores/:id/probe` routes are intentionally not preserved. Task 7 should use the explicit `/scores/accounts/:account_id/...` routes with `group_id` and `model`.
