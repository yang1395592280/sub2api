# Task 2 Report: OpenAI Runtime Overbrush 429 Gate

## Implementation Summary

- Added `Account.IsOpenAIOverbrushEnabled()` for enabled OpenAI API Key accounts without an `upstream_admin_type`.
- Added per-account OpenAI overbrush 429 runtime counters and `ResetOpenAIOverbrush429Count`.
- Added settings-backed threshold lookup with the Task 1 default fallback.
- Gated `handleOpenAIAccountUpstreamError` immediately after its state context is created so eligible 429 responses below the configured threshold bypass both OAuth 429 marking and the existing rate-limit service.

## TDD Evidence

### RED

Created `backend/internal/service/openai_account_overbrush_test.go` before production changes, then ran:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIOverbrush429|TestHandleOpenAIAccountUpstreamError_Overbrush' -count=1
```

Result: failed as expected because `shouldSkipOpenAI429LimitForOverbrush` and `ResetOpenAIOverbrush429Count` were undefined.

### GREEN

Implemented the minimal eligibility helper, counter, reset helper, threshold lookup, and upstream-error gate. Then ran:

```bash
cd backend
gofmt -w internal/service/account.go internal/service/openai_gateway_service.go internal/service/openai_account_runtime_block_fastpath.go internal/service/openai_account_overbrush_test.go
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIOverbrush429|TestHandleOpenAIAccountUpstreamError_Overbrush' -count=1
```

Result: passed (`ok github.com/Wei-Shaw/sub2api/internal/service`).

## Tests

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

Result: passed with exit code 0.

`git diff --check` also completed without output.

## Files Changed

- `backend/internal/service/account.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_account_runtime_block_fastpath.go`
- `backend/internal/service/openai_account_overbrush_test.go`

## Self-Review

- Threshold behavior matches the brief: 429 responses before the threshold are deferred; the threshold response returns to existing rate-limit handling and clears the counter.
- Eligibility rejects OAuth, setup-token, OpenAI-compatible upstream-admin API Keys, missing opt-in flags, and non-OpenAI platforms.
- The gate is placed before context-window/image-limit and 429 handling, preventing both `markOpenAIOAuth429RateLimited` and `RateLimitService.HandleUpstreamError` while deferred.
- Success paths intentionally do not call the reset method in this task, as deferred to later tasks by the task brief.

## Concerns

- None.

## Review Fix: Atomic Concurrent 429 Counter (2026-07-10)

### Root Cause

`sync.Map` made individual map operations safe, but the `Load -> increment -> threshold check -> Delete/Store` sequence was not atomic. Concurrent 429 requests for the same account could observe stale counts and either bypass or reach the configured threshold incorrectly.

### Fix

- Added `openaiOverbrush429CountsMu` to `OpenAIGatewayService`.
- Locked the entire counter read, increment, threshold, and delete/store sequence in `shouldSkipOpenAI429LimitForOverbrush`.
- Used the same mutex in `ResetOpenAIOverbrush429Count` so a reset cannot race with an in-flight increment.
- Added `TestOpenAIOverbrush429ConcurrentThresholdUpdateIsAtomic`: 256 concurrent requests at threshold 2 must produce exactly 128 deferred results. Before the fix it failed with 119 deferred results; after the fix it passes.

### Verification

Executed from `backend`:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run '^TestOpenAIOverbrush429ConcurrentThresholdUpdateIsAtomic$' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIOverbrush429|TestHandleOpenAIAccountUpstreamError_Overbrush' -count=1
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run '^TestOpenAIOverbrush429ConcurrentThresholdUpdateIsAtomic$' -count=1
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

All commands completed with exit code 0.
