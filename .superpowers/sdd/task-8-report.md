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
