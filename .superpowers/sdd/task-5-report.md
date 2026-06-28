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
