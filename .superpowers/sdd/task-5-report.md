# Task 5 Report: Periodic Probe Runner

## What I implemented
- Added `OpenAIAutoSchedulerProbeRunner` with `Start()`, `Stop()`, and `runOnce()`.
- Runner reads OpenAI auto scheduler settings each tick, skips work when disabled, loads enabled OpenAI groups, loads schedulable OpenAI accounts per group, and dedupes in-flight `(account, group, model)` checks.
- Added `OpenAIAutoSchedulerProbeChecker` and a minimal OpenAI HTTP checker implementation for non-stream probe calls.
- Added `OpenAIAutoSchedulerProbeResult` for checker return values.
- Added `OpenAIAutoSchedulerService.ListEnabledOpenAIGroups()` so the runner can use the existing authoritative group filter.
- Wired the runner and checker into service provider setup and app cleanup/startup.

## What I tested and results
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run TestOpenAIAutoSchedulerProbeRunner`
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run 'Test(OpenAIAutoSchedulerProbeRunner|OpenAIAutoSchedulerService_IsEnabledForGroupRequiresGlobalAndGroup|OpenAIAutoSchedulerScore_SlowResponsesDegradeThenRecover|OpenAIAutoSchedulerSelector_GroupGate)'`
- Both passed.

## TDD Evidence
### RED
- Command: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run TestOpenAIAutoSchedulerProbeRunner`
- Relevant failure: compile errors for missing probe runner/result types and constructor wiring before implementation.
- Why expected: the runner had not been implemented yet.

### GREEN
- Command: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run TestOpenAIAutoSchedulerProbeRunner`
- Result: `ok github.com/Wei-Shaw/sub2api/internal/service (cached)`
- Command: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run 'Test(OpenAIAutoSchedulerProbeRunner|OpenAIAutoSchedulerService_IsEnabledForGroupRequiresGlobalAndGroup|OpenAIAutoSchedulerScore_SlowResponsesDegradeThenRecover|OpenAIAutoSchedulerSelector_GroupGate)'`
- Result: `ok github.com/Wei-Shaw/sub2api/internal/service (cached)`

## Files changed
- `backend/internal/service/openai_auto_scheduler_probe_runner.go`
- `backend/internal/service/openai_auto_scheduler_probe_runner_test.go`
- `backend/internal/service/openai_auto_scheduler_service.go`
- `backend/internal/service/openai_auto_scheduler_types.go`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`

## Self-review findings
- The runner is intentionally scoped to OpenAI only.
- `runOnce()` does safe gating on settings and repo errors.
- In-flight dedupe is keyed by account/group/model.
- The HTTP checker is minimal and non-stream, with a bounded response body read.

## Issues or concerns
- `backend/cmd/server/wire_gen.go` was updated directly so the app startup wires the new runner immediately; this file is generated, so a future wire regeneration may rewrite it.
- The probe model is currently the shared OpenAI test model path used by existing code.

---

## Review Fix Report - 2026-06-28 13:42:29 CST

### What I fixed
- Threaded the runner's cancelable `parentCtx` through `loop()`, interval settings reads, `runOnce()`, group/account listing, and worker probe checks so `Stop()` cancels in-flight probe work instead of leaving checks on `context.Background()`.
- Added runner tests for `probe_error` recording and `Start()`/`Stop()` cancellation of an active probe.
- Added the missing `NewOpenAIAutoSchedulerProbeChecker` provider to `service.ProviderSet`.
- Regenerated `backend/cmd/server/wire_gen.go` from `backend/cmd/server/wire.go` with the repo's `go generate ./cmd/server` path.
- Updated `backend/cmd/server/wire_gen_test.go` for the new cleanup dependency signature.

### TDD / failure evidence
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run TestOpenAIAutoSchedulerProbeRunner`
  - RED result before implementation: `TestOpenAIAutoSchedulerProbeRunner_StopCancelsInFlightProbe` failed because the checker never observed context cancellation.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./cmd/server`
  - Initial failure before Wire provider fix: Wire reported no provider for `service.OpenAIAutoSchedulerProbeChecker`.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server`
  - Initial failure after regeneration: `wire_gen_test.go` had an outdated `provideCleanup` call signature.

### Verification commands and results
- `cd backend && gofmt -w internal/service/openai_auto_scheduler_probe_runner.go internal/service/openai_auto_scheduler_probe_runner_test.go cmd/server/wire_gen_test.go`
  - Result: completed.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go generate ./cmd/server`
  - Result: Wire wrote `backend/cmd/server/wire_gen.go` successfully.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run TestOpenAIAutoSchedulerProbeRunner`
  - Result: `ok github.com/Wei-Shaw/sub2api/internal/service`.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -tags unit ./internal/service -run 'Test(OpenAIAutoSchedulerProbeRunner|OpenAIAutoSchedulerService_IsEnabledForGroupRequiresGlobalAndGroup|OpenAIAutoSchedulerScore_SlowResponsesDegradeThenRecover|OpenAIAutoSchedulerSelector_GroupGate)'`
  - Result: `ok github.com/Wei-Shaw/sub2api/internal/service`.
- `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server`
  - Result: `ok github.com/Wei-Shaw/sub2api/cmd/server`.

### Risks or notes
- `Stop()` now cancels the runner context before waiting for the worker pool, so cooperative checker/list implementations can terminate promptly.
- Wire generation now constructs `OpenAIAutoSchedulerProbeChecker` from the existing `HTTPUpstream` provider rather than a hand-edited generated file.
