# Task 3 Report: Health Scoring and Real-Sample Priority

## Status

DONE. OpenAI real request outcomes now update the unified physical-account health snapshot while preserving the legacy group-scoped strict outcome write.

## Implementation

- Added `OpenAISchedulerHealthEvent`, `OpenAISchedulerHealthSettings`, and `ApplyOpenAISchedulerHealthEvent`.
- Added real/probe EWMA with alpha 0.2/0.1, 30-minute state TTL, fresh-real priority, error/429/5xx rates, and legacy-compatible breaker/cooldown/half-open transitions.
- Added a unified health event sink that normalizes and validates the full physical key:
  `account_id + model_family + endpoint + transport`.
- Added a per-normalized-key in-process mutex around `GetBatch -> Apply -> Upsert`, preventing the recorder's two workers from losing same-key updates.
- Extended `OpenAIAutoSchedulerRecordInput` with `ModelFamily`, `Endpoint`, and `Transport`; legacy `Model` and `GroupID` semantics remain unchanged.
- Preserved strict legacy `RecordOutcome`; the recorder attempts both legacy and unified sinks and joins errors so either persistence failure increments recorder failure metrics.
- Preserved production slow/severe classification by keeping the concrete legacy scheduler as the recorder's classifier sink.
- Missing/invalid unified dimensions skip only the unified write and emit power-of-two rate-limited warnings; legacy persistence still runs.
- Added explicit attempt metadata across Responses, passthrough, Chat fallback, raw Chat, embeddings, image generation/edit, WS v2 passthrough, and WS ingress paths.
- Success prefers final `OpenAIForwardResult.UpstreamModel/UpstreamEndpoint`; failure uses metadata frozen before the actual upstream attempt.
- Normalization lowercases/trims the actual upstream model without heuristic folding, maps only the five approved endpoint types, and converts `responses_websockets_v2_ingress` to actual WS v2.
- Regenerated `backend/cmd/server/wire_gen.go` with Wire; it now constructs the health repository/sink and injects it into the production recorder.

## Files

- `backend/internal/service/openai_scheduler_health_score.go` (new)
- `backend/internal/service/openai_scheduler_health_score_test.go` (new)
- `backend/internal/service/openai_auto_scheduler_service.go`
- `backend/internal/service/openai_auto_scheduler_outcome_recorder.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `backend/internal/service/openai_gateway_forward.go`
- `backend/internal/service/openai_gateway_passthrough.go`
- `backend/internal/service/openai_gateway_chat_completions.go`
- `backend/internal/service/openai_gateway_cc_pipeline.go`
- `backend/internal/service/openai_embeddings.go`
- `backend/internal/service/openai_images.go`
- `backend/internal/service/openai_images_responses.go`
- `backend/internal/service/openai_ws_v2_passthrough_adapter.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire_gen.go` (generated only)

`openai_gateway_chat_completions_raw.go` required no new diff because it already sets the final raw Chat upstream endpoint; the shared CC sender now freezes the final body model, `chat_completions`, and `http_sse` before sending.

## TDD Evidence

### RED 1: scoring contract absent

Command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestApplyOpenAISchedulerHealthEvent -count=1
```

Result: exit 1.

```text
internal/service/openai_scheduler_health_score_test.go:14:14: undefined: DefaultOpenAISchedulerHealthSettings
internal/service/openai_scheduler_health_score_test.go:23:10: undefined: ApplyOpenAISchedulerHealthEvent
internal/service/openai_scheduler_health_score_test.go:23:80: undefined: OpenAISchedulerHealthEvent
internal/service/openai_scheduler_health_score_test.go:24:12: undefined: HealthSourceReal
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

The failure was correct: tests compiled against the required public scoring API before any production implementation existed.

### RED 2: producer metadata was not carried

Command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAISchedulerHealth(ForwardAttempt|DirectOutcome|Composite)' -count=1
```

Result: exit 1.

```text
too many arguments in call to armOpenAIUpstreamAttempt
  have (context.Context, openAIAutoSchedulerAttemptMetadata)
  want (context.Context)
too many arguments in call to svc.recordOpenAIAutoSchedulerOutcome
  have (..., OpenAIAutoSchedulerRecordInput, openAIAutoSchedulerAttemptMetadata)
  want (..., OpenAIAutoSchedulerRecordInput)
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

The failure was correct: the actual-attempt and direct-outcome APIs could not yet carry the required dimensions.

### RED 3: error-only samples incorrectly initialized TTFT EWMA

Command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyOpenAISchedulerHealthEvent/first_TTFT' -count=1
```

Result: exit 1.

```text
expected: 500
actual  : 100
TestApplyOpenAISchedulerHealthEvent/first_TTFT_initializes_prediction_after_an_error-only_sample
FAIL github.com/Wei-Shaw/sub2api/internal/service
```

Root cause: `RealSampleCount` includes errors without TTFT, so it cannot indicate that the TTFT EWMA is initialized. The implementation now uses `PredictedTTFTMS > 0` for TTFT initialization while rates still use total sample count.

### GREEN

Focused score/recorder command from the brief:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAISchedulerHealth|OutcomeRecorder' -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 0.863s
```

The focused tests cover fast, slow, severe slow, 429, 5xx, probe priority, TTL expiry, open cooldown, half-open recovery, metadata normalization, missing-dimension skip, HTTP/WS separation, endpoint separation, legacy preservation, composite classification, and same-key two-worker updates.

## Generator

The brief's isolated-cache command initially failed before running Wire because the repository baseline lacks the `github.com/google/subcommands` checksum required by the Wire CLI:

```text
missing go.sum entry for module providing package github.com/google/subcommands
```

Wire was then run through the same `go generate ./cmd/server` directive with module resolution enabled:

```bash
cd backend && GOFLAGS=-mod=mod GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
```

```text
wire: github.com/Wei-Shaw/sub2api/cmd/server: wrote .../backend/cmd/server/wire_gen.go
```

The generator-added CLI-only checksum lines were removed afterward. Final `go.sum` has no diff, and `wire_gen.go` was not manually edited.

## Extended Verification

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 56.390s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestOpenAISchedulerHealthOutcomeSinkSerializesSameKeyAcrossTwoWorkers|TestOpenAISchedulerHealthOutcomeSink|TestOpenAISchedulerHealthCompositePreservesProductionSlowClassification' -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 2.219s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run OpenAISchedulerHealth -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/repository 0.812s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/cmd/server 0.438s
```

```bash
git diff --check
```

Result: exit 0, no output.

## Concurrency Semantics

- The sink normalizes the complete health key before locking.
- Both recorder workers use the same sink instance and therefore the same per-key mutex registry.
- The lock covers repository load, pure event application, and upsert as one in-process critical section.
- Different physical keys remain independent and can be processed concurrently.
- The race test adds an artificial repository read delay so an unlocked implementation deterministically reads the same old snapshot twice and loses one sample; the final assertion requires `RealSampleCount == 2`.
- Per the approved design, multiple application instances may briefly overwrite each other and converge through later samples/TTL. No distributed lock was added in Task 3.

## Self-review

- Unified keys never contain `GroupID`; tests use different legacy groups for one physical account and still create one row per actual endpoint/transport key.
- Only actual upstream attempts receive unified metadata. WS connection-pool acquisition failures remain legacy-only because no upstream request was made.
- Fallback records the first failed Responses attempt as `responses`; the next raw attempt is independently frozen as `chat_completions`.
- Ingress-only WS selector transport is normalized and never persisted.
- Missing or unknown endpoint/transport values cannot create wildcard rows.
- Queue capacity, non-blocking `TryRecord`, metrics, drain/Stop behavior, HTTP/WS timing, and legacy group switches were not changed.
- No database schema, migration, production configuration, `progress.md`, or `go.sum` change is included.

## Concerns

- Multi-instance load/apply/upsert has the approved short eventual-consistency window; only same-process workers are serialized in this task.
- The per-key lock registry follows the existing legacy scheduler pattern and retains one mutex per observed normalized health key for the process lifetime.
- The baseline Wire generator currently needs module resolution for its unlisted CLI-only indirect checksum; runtime dependencies and final `go.sum` remain unchanged.

## Reviewer Fixes

### Fix 1: preserve open and half-open recovery semantics

Added state-sequence coverage for error-open followed by slow, slow-open followed by error, half-open followed by slow, expired open followed by 429, and unexpired error-open followed by another error.

Initial RED command:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyOpenAISchedulerHealthEvent/(error-open|slow-open|half_open_slow|expired_open)' -count=1
```

Result: exit 1. All four cases expected `open` but received `observing`.

The follow-up cooldown RED used:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyOpenAISchedulerHealthEvent/unexpired_open_circuit_repeated_error' -count=1
```

```text
expected cooldown: 1970-01-01 00:17:40 UTC
actual cooldown:   1970-01-01 00:17:41 UTC
```

The final state helper applies this order: an unexpired `open` preserves its cooldown; expired/missing-cooldown `open` refreshes it; `half_open` immediately reopens; other states use the configured consecutive breaker threshold. Counters, rates, and TTFT still update before the state decision.

### Fix 2: freeze mapped WS ingress attempt models

Initial metadata API RED:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAISchedulerHealthWSIngressFailureUsesMappedAttemptModel -count=1
```

```text
undefined: openAIWSIngressAttemptMetadata
FAIL github.com/Wei-Shaw/sub2api/internal/service [build failed]
```

The production call-site RED was then verified by temporarily restoring the reviewed bug and running the real HTTP bridge and ctx-pool WS relay integration test:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_FailureOutcomeUsesMappedAttemptModel -count=1
```

Result: exit 1 in both subtests.

```text
HTTP_bridge_failure: expected "mapped-upstream-model", actual "client-alias"
WS_relay_failure:    expected "mapped-upstream-model", actual "client-alias"
```

The fix freezes `account.GetMappedModel(originalModel)` plus upstream normalization immediately before each actual HTTP bridge/WS relay attempt. Failure uses that frozen metadata; success may still override from the final result. The same integration command after restoring the fix passed in 0.667s.

### Reviewer-fix verification

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyOpenAISchedulerHealthEvent|TestOpenAISchedulerHealthWSIngressFailureUsesMappedAttemptModel|TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_FailureOutcomeUsesMappedAttemptModel' -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 0.758s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'TestOpenAISchedulerHealthOutcomeSinkSerializesSameKeyAcrossTwoWorkers|TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_FailureOutcomeUsesMappedAttemptModel' -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 2.176s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/internal/service 56.320s
```

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -count=1
```

```text
ok github.com/Wei-Shaw/sub2api/cmd/server 0.436s
```
