# Task 4 Report: Selector and OpenAI Gateway Integration

## What I Implemented

- Added `OpenAIAutoSchedulerSelector` with `Rank` behavior:
  - Disabled or ungrouped requests return original candidates with `used=false`.
  - Open circuit states are skipped only when `CooldownUntil` is nil or still in the future.
  - Missing score state falls back to neutral score/running state via `NewOpenAIAutoSchedulerScoreState`.
  - Ranking order is state tier, final score descending, account priority ascending, then oldest `LastUsedAt`.
  - Score service errors preserve original candidate order and return `used=false`.
- Added `OpenAIAutoSchedulerService.GetStateForSelection` for hot-path selector reads.
- Integrated selector ordering into OpenAI gateway selection:
  - Non-load-aware selection ranks only after existing eligibility checks.
  - Load-aware selection ranks only after existing candidate filters, while full accounts/load failures still fall through safely.
  - Integration is gated to `PlatformOpenAI`; other platforms are untouched.
- Added gateway setter `SetOpenAIAutoScheduler` to avoid constructor churn.
- Added asynchronous request outcome recording:
  - Uses nil checks and group gating through `OpenAIAutoSchedulerService.Record`.
  - Uses a background context with 2s timeout.
  - Records success/error/rate-limited outcomes from native OpenAI, passthrough, WS, and raw chat-completions fallback paths.
  - Service errors degrade silently because `Record` already returns nil on repository write failures.
- Made the existing fake OpenAI auto scheduler repo test-safe for async outcome assertions.

## What I Tested And Results

- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIAutoSchedulerSelector`
  - PASS after implementation.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoSchedulerSelector|TestOpenAIGatewayService_SelectAccountWithScheduler'`
  - PASS after gateway integration and async record helper.
- `GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoSchedulerSelector|TestOpenAIGatewayService_SelectAccountWithScheduler|TestOpenAI.*Scheduler'`
  - PASS.
- `GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service`
  - PASS: `ok github.com/Wei-Shaw/sub2api/internal/service 47.015s`.

## TDD Evidence

### RED

Command:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIAutoSchedulerSelector
```

Relevant failing output:

```text
internal/service/openai_auto_scheduler_selector_test.go:37:14: undefined: NewOpenAIAutoSchedulerSelector
internal/service/openai_auto_scheduler_selector_test.go:52:14: undefined: NewOpenAIAutoSchedulerSelector
internal/service/openai_auto_scheduler_selector_test.go:69:14: undefined: NewOpenAIAutoSchedulerSelector
internal/service/openai_auto_scheduler_selector_test.go:89:14: undefined: NewOpenAIAutoSchedulerSelector
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

Why expected:

- The selector tests were written before the selector existed, so compile failure on the missing constructor proved the tests were exercising new behavior.

Additional gateway RED:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoSchedulerSelector|TestOpenAIGatewayService_SelectAccountWithScheduler'
```

Relevant failing output:

```text
svc.SetOpenAIAutoScheduler undefined (type *OpenAIGatewayService has no field or method SetOpenAIAutoScheduler)
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

Async record RED:

```text
svc.recordOpenAIAutoSchedulerOutcome undefined (type *OpenAIGatewayService has no field or method recordOpenAIAutoSchedulerOutcome)
```

### GREEN

Command:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAutoSchedulerSelector|TestOpenAIGatewayService_SelectAccountWithScheduler|TestOpenAI.*Scheduler'
```

Relevant passing output:

```text
ok github.com/Wei-Shaw/sub2api/internal/service 0.570s
```

Final verification command:

```bash
GOCACHE=/tmp/sub2api-go-cache go test -count=1 ./internal/service
```

Relevant passing output:

```text
ok github.com/Wei-Shaw/sub2api/internal/service 47.015s
```

## Files Changed

- `backend/internal/service/openai_auto_scheduler_selector.go`
- `backend/internal/service/openai_auto_scheduler_selector_test.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_service_test.go`
- `backend/internal/service/openai_gateway_responses_chat_fallback.go`
- `backend/internal/service/openai_auto_scheduler_service_test.go`

## Self-Review Findings

- Selector ranking is applied only after existing OpenAI eligibility filters, so it does not make unschedulable, runtime-blocked, unsupported-model, rate-limited, or channel-restricted accounts eligible.
- Group gating remains authoritative because `Rank` delegates to `IsEnabledForGroup`; nil group IDs return `used=false`, so ungrouped keys do not use auto scheduling.
- Service/cache errors in selector state reads return original candidate order.
- Outcome recording is intentionally best-effort and asynchronous; it does not block the request hot path.
- `channel_price` was not used in gateway billing or usage cost paths by this task.

## Issues Or Concerns

- No known blocking concerns.
- The load-aware path still honors full-load filtering and acquisition behavior, but when auto scheduler ranking is active it intentionally uses scheduler score order among available candidates instead of re-sorting by priority/load.
