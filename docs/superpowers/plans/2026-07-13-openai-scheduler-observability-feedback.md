# OpenAI Scheduler Observability and Feedback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore trustworthy real-request scheduler feedback and record end-to-end first-token timing without changing the account selection policy.

**Architecture:** Add a request-scoped timing object shared by handlers and gateway forwarders, persist additive timing fields on usage logs, classify successful probes by latency, and replace per-request goroutines with a bounded outcome recorder. Existing OpenAI auto-scheduler APIs and score tables remain compatible.

**Tech Stack:** Go, Gin, Ent, PostgreSQL, Wire, testify, sqlmock.

## Global Constraints

- Work from a `feature/*` branch created from `custom-main`.
- Keep `first_token_ms` semantics unchanged; add separate end-to-end and phase timing fields.
- `previous_response_id`, overbrush, automatic cheapest-group selection, price guards, Compact, SSE, WS, passthrough, and billing behavior must not change.
- Scheduler feedback failure must never fail or delay a user request.
- Do not edit generated Ent or Wire files manually; run the repository generators.
- Migration number `182` is reserved by this plan; verify it is still unused immediately before implementation.
- Run focused tests before the full backend test suite.

---

## File Structure

- Create `backend/internal/service/openai_request_timing.go`: request-scoped routing, queue, retry, and end-to-end timing.
- Create `backend/internal/service/openai_request_timing_test.go`: timing unit tests.
- Create `backend/internal/service/openai_auto_scheduler_outcome_recorder.go`: bounded asynchronous feedback worker.
- Create `backend/internal/service/openai_auto_scheduler_outcome_recorder_test.go`: queue, shutdown, and non-blocking tests.
- Create `backend/migrations/182_openai_scheduler_observability.sql`: additive usage timing columns.
- Modify `backend/ent/schema/usage_log.go`: Ent fields for new timing columns.
- Modify `backend/internal/service/usage_log.go`: service model timing fields.
- Modify `backend/internal/service/openai_gateway_service.go`: forwarding result timing fields.
- Modify `backend/internal/service/openai_gateway_usage.go`: transfer timing fields into usage logs.
- Modify `backend/internal/repository/usage_log_repo_insert.go`: insert arguments and SQL columns.
- Modify `backend/internal/repository/usage_log_repo_query.go`: selected columns and scanning.
- Modify `backend/internal/repository/usage_log_repo_request_type_test.go`: SQL contract coverage.
- Modify OpenAI handlers and forwarders listed in Tasks 3 and 5 only at timing/outcome hook points.
- Modify `backend/internal/service/wire.go`, `backend/cmd/server/wire.go`, and generated Wire output to own recorder lifecycle.

### Task 1: Correct Probe Slow Classification

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_probe_runner.go:199`
- Modify: `backend/internal/handler/admin/openai_auto_scheduler_handler.go:227`
- Test: `backend/internal/service/openai_auto_scheduler_probe_runner_test.go`
- Test: `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`

**Interfaces:**
- Produces: `classifyOpenAIAutoSchedulerProbeEvent(OpenAIAutoSchedulerProbeResult, OpenAIAutoSchedulerSettings) string`
- Consumes: `SlowThresholdMS`, `SevereSlowThresholdMS`, and existing event constants.

- [ ] **Step 1: Write failing service tests for successful slow probes**

```go
func TestClassifyOpenAIAutoSchedulerProbeEvent(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.SlowThresholdMS = 6000
	settings.SevereSlowThresholdMS = 15000

	tests := []struct {
		name string
		in   OpenAIAutoSchedulerProbeResult
		want string
	}{
		{"fast success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: intPtr(1200)}, OpenAIAutoSchedulerEventProbeSuccess},
		{"slow success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: intPtr(7000)}, OpenAIAutoSchedulerEventSlow},
		{"severe success", OpenAIAutoSchedulerProbeResult{Success: true, TtfbMS: intPtr(16000)}, OpenAIAutoSchedulerEventSevereSlow},
		{"probe error", OpenAIAutoSchedulerProbeResult{Success: false, Err: errors.New("timeout")}, OpenAIAutoSchedulerEventProbeError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, classifyOpenAIAutoSchedulerProbeEvent(tt.in, settings))
		})
	}
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestClassifyOpenAIAutoSchedulerProbeEvent -count=1`

Expected: FAIL because `classifyOpenAIAutoSchedulerProbeEvent` does not exist.

- [ ] **Step 3: Implement one shared classifier and use it in automatic and manual probes**

```go
func classifyOpenAIAutoSchedulerProbeEvent(result OpenAIAutoSchedulerProbeResult, settings OpenAIAutoSchedulerSettings) string {
	if !result.Success || result.Err != nil {
		return OpenAIAutoSchedulerEventProbeError
	}
	normalized := normalizeOpenAIAutoSchedulerSettings(settings)
	observedMS := result.LatencyMS
	if result.TtfbMS != nil && *result.TtfbMS > 0 {
		observedMS = *result.TtfbMS
	}
	switch {
	case observedMS >= normalized.SevereSlowThresholdMS:
		return OpenAIAutoSchedulerEventSevereSlow
	case observedMS >= normalized.SlowThresholdMS:
		return OpenAIAutoSchedulerEventSlow
	default:
		return OpenAIAutoSchedulerEventProbeSuccess
	}
}
```

Automatic and manual probe paths must both call this helper with the effective settings.

- [ ] **Step 4: Run automatic/manual probe tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'TestClassifyOpenAIAutoSchedulerProbeEvent|TestOpenAIAutoSchedulerProbe' -count=1`

Expected: PASS; successful 7s and 16s probes produce `slow` and `severe_slow`.

- [ ] **Step 5: Commit the probe correctness fix**

```bash
git add backend/internal/service/openai_auto_scheduler_probe_runner.go \
  backend/internal/service/openai_auto_scheduler_probe_runner_test.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler_test.go
git commit -m "fix: classify slow OpenAI scheduler probes"
```

### Task 2: Add Request-Scoped End-to-End Timing

**Files:**
- Create: `backend/internal/service/openai_request_timing.go`
- Test: `backend/internal/service/openai_request_timing_test.go`

**Interfaces:**
- Produces: `BeginOpenAIRequestTiming(*gin.Context) *OpenAIRequestTiming`
- Produces: `OpenAIRequestTimingFromContext(*gin.Context) *OpenAIRequestTiming`
- Produces methods: `BeginRouting`, `EndRouting`, `AddQueue`, `AddRetry`, `E2EFirstTokenMS` and `Snapshot`.

- [ ] **Step 1: Write failing timing tests**

```go
func TestOpenAIRequestTimingSnapshot(t *testing.T) {
	now := time.Unix(100, 0)
	timing := newOpenAIRequestTiming(func() time.Time { return now })
	timing.BeginRouting()
	now = now.Add(12 * time.Millisecond)
	timing.EndRouting()
	timing.AddQueue(40 * time.Millisecond)
	timing.AddRetry(25 * time.Millisecond)
	now = now.Add(900 * time.Millisecond)

	snapshot := timing.Snapshot()
	require.Equal(t, 12, snapshot.RoutingMS)
	require.Equal(t, 40, snapshot.QueueMS)
	require.Equal(t, 25, snapshot.RetryMS)
	require.Equal(t, 912, timing.E2EFirstTokenMS())
}
```

- [ ] **Step 2: Run the test and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIRequestTimingSnapshot -count=1`

Expected: FAIL because the timing type does not exist.

- [ ] **Step 3: Implement the concurrency-safe timing object**

```go
type OpenAIRequestTimingSnapshot struct {
	RoutingMS int
	QueueMS   int
	RetryMS   int
}

type OpenAIRequestTiming struct {
	mu             sync.Mutex
	now            func() time.Time
	startedAt      time.Time
	routingStarted time.Time
	routing        time.Duration
	queue          time.Duration
	retry          time.Duration
}

func (t *OpenAIRequestTiming) Snapshot() OpenAIRequestTimingSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return OpenAIRequestTimingSnapshot{
		RoutingMS: int(t.routing.Milliseconds()),
		QueueMS:   int(t.queue.Milliseconds()),
		RetryMS:   int(t.retry.Milliseconds()),
	}
}
```

Use a private Gin context key and make `BeginOpenAIRequestTiming` idempotent so retries share the first request start.

- [ ] **Step 4: Run timing tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIRequestTiming -count=1`

Expected: PASS, including concurrent `AddQueue`/`AddRetry` coverage under `go test -race`.

- [ ] **Step 5: Commit timing primitives**

```bash
git add backend/internal/service/openai_request_timing.go backend/internal/service/openai_request_timing_test.go
git commit -m "feat: track OpenAI request scheduling phases"
```

### Task 3: Instrument Selection and Queue Waiting

**Files:**
- Modify: `backend/internal/handler/openai_gateway_handler.go:345,877,1248`
- Modify: `backend/internal/handler/openai_chat_completions.go:145`
- Modify: `backend/internal/handler/openai_embeddings.go:117`
- Modify: `backend/internal/handler/openai_images.go:155`
- Test: `backend/internal/handler/openai_gateway_handler_test.go`

**Interfaces:**
- Consumes: timing API from Task 2.
- Produces: every OpenAI handler begins timing before its first account selection; routing and queue durations are recorded exactly once per attempt.

- [ ] **Step 1: Add failing handler tests**

```go
func TestOpenAIHandlerTimingIncludesSelectionAndQueue(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	timing := BeginOpenAIRequestTiming(c)
	timing.BeginRouting()
	time.Sleep(time.Millisecond)
	timing.EndRouting()
	timing.AddQueue(25 * time.Millisecond)

	snapshot := timing.Snapshot()
	require.Greater(t, snapshot.RoutingMS, 0)
	require.Equal(t, 25, snapshot.QueueMS)
}
```

Also assert a retry reuses the same timing object instead of resetting `startedAt`.

- [ ] **Step 2: Run handler timing tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run TestOpenAIHandlerTiming -count=1`

Expected: FAIL until handler hook calls exist.

- [ ] **Step 3: Wrap selection and wait operations**

```go
timing := service.BeginOpenAIRequestTiming(c)
timing.BeginRouting()
effectiveAPIKey, selection, decision, err := h.gatewayService.SelectEffectiveOpenAIAccountWithSchedulerForCapability(
	c.Request.Context(), apiKey, previousResponseID, sessionHash, requestedModel,
	excludedIDs, transport, requiredCapability, requireCompact,
	previousResponseCanMove, service.PlatformOpenAI,
)
timing.EndRouting()
```

Around `AcquireAccountSlotWithWaitTimeout`, measure the actual elapsed wait and call `timing.AddQueue(elapsed)`. Around failover delay/attempt preparation, call `timing.AddRetry(elapsed)`. Do not include upstream generation time in queue or retry.

- [ ] **Step 4: Run OpenAI handler scheduling tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestOpenAI.*(Scheduler|Wait|Retry|Timing)' -count=1`

Expected: PASS with existing account selection decisions unchanged.

- [ ] **Step 5: Commit handler instrumentation**

```bash
git add backend/internal/handler/openai_gateway_handler.go \
  backend/internal/handler/openai_chat_completions.go \
  backend/internal/handler/openai_embeddings.go \
  backend/internal/handler/openai_images.go \
  backend/internal/handler/openai_gateway_handler_test.go
git commit -m "feat: measure OpenAI routing and queue time"
```

### Task 4: Persist Additive Usage Timing Fields

**Files:**
- Create: `backend/migrations/182_openai_scheduler_observability.sql`
- Modify: `backend/ent/schema/usage_log.go`
- Modify: `backend/internal/service/usage_log.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_gateway_usage.go`
- Modify: `backend/internal/repository/usage_log_repo_insert.go`
- Modify: `backend/internal/repository/usage_log_repo_query.go`
- Test: `backend/internal/repository/usage_log_repo_request_type_test.go`
- Test: `backend/internal/service/openai_gateway_record_usage_test.go`
- Generated: `backend/ent/*`

**Interfaces:**
- Produces fields: `E2EFirstTokenMs`, `RoutingMs`, `QueueMs`, `RetryMs` as nullable integers.
- Consumes: timing snapshot from Task 2 and existing `OpenAIForwardResult.FirstTokenMs`.

- [ ] **Step 1: Write failing repository and usage mapping tests**

```go
func TestOpenAIGatewayServiceRecordUsageCopiesSchedulerTiming(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(
		usageRepo,
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:         "scheduler_timing",
			Model:             "gpt-5.5",
			Usage:             OpenAIUsage{InputTokens: 10, OutputTokens: 5},
			Duration:          2 * time.Second,
		FirstTokenMs:      intPtr(900),
		E2EFirstTokenMs:   intPtr(1180),
		RoutingMs:         intPtr(20),
		QueueMs:           intPtr(200),
		RetryMs:           intPtr(60),
		},
		APIKey:  &APIKey{ID: 501, Quota: 100},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
	})
	require.NoError(t, err)
	require.Equal(t, 1180, *usageRepo.lastLog.E2EFirstTokenMs)
	require.Equal(t, 20, *usageRepo.lastLog.RoutingMs)
	require.Equal(t, 200, *usageRepo.lastLog.QueueMs)
	require.Equal(t, 60, *usageRepo.lastLog.RetryMs)
}
```

Extend SQL mock expectations in the same positional order as `usageLogInsertArgTypes`.

- [ ] **Step 2: Run tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'SchedulerTiming|E2EFirstToken' -count=1`

Expected: compile FAIL because fields are absent.

- [ ] **Step 3: Add the additive migration**

```sql
ALTER TABLE usage_logs
  ADD COLUMN IF NOT EXISTS e2e_first_token_ms INTEGER,
  ADD COLUMN IF NOT EXISTS routing_ms INTEGER,
  ADD COLUMN IF NOT EXISTS queue_ms INTEGER,
  ADD COLUMN IF NOT EXISTS retry_ms INTEGER;

CREATE INDEX IF NOT EXISTS idx_usage_logs_e2e_ttft_created_at
  ON usage_logs (created_at DESC)
  WHERE e2e_first_token_ms IS NOT NULL;
```

- [ ] **Step 4: Add fields and regenerate Ent**

Add four optional/nillable integer fields next to `first_token_ms`, then run:

`cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./ent`

Expected: Ent generated files include getters, builders, predicates, and migration schema columns.

- [ ] **Step 5: Extend insert/query contracts and usage mapping**

Place the four new arguments immediately after `first_token_ms` in every insert column list, argument builder, query select list, and scanner. In `openai_gateway_usage.go`, copy the four result fields into `UsageLog` without deriving missing values from `first_token_ms`.

- [ ] **Step 6: Run repository/service tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository ./internal/service -run 'UsageLog|SchedulerTiming|E2EFirstToken' -count=1`

Expected: PASS; legacy rows with NULL timing fields still scan successfully.

- [ ] **Step 7: Commit migration and persistence**

```bash
git add backend/migrations/182_openai_scheduler_observability.sql backend/ent \
  backend/internal/service/usage_log.go backend/internal/service/openai_gateway_service.go \
  backend/internal/service/openai_gateway_usage.go backend/internal/repository/usage_log_repo_insert.go \
  backend/internal/repository/usage_log_repo_query.go backend/internal/repository/usage_log_repo_request_type_test.go \
  backend/internal/service/openai_gateway_record_usage_test.go
git commit -m "feat: persist end-to-end OpenAI first-token timing"
```

### Task 5: Add a Bounded Outcome Recorder and Restore Production Feedback

**Files:**
- Create: `backend/internal/service/openai_auto_scheduler_outcome_recorder.go`
- Test: `backend/internal/service/openai_auto_scheduler_outcome_recorder_test.go`
- Modify: `backend/internal/service/openai_gateway_scheduling.go:205`
- Modify: `backend/internal/service/openai_gateway_forward.go`
- Modify: `backend/internal/service/openai_gateway_passthrough.go`
- Modify: `backend/internal/service/openai_ws_forwarder_support.go`
- Modify: `backend/internal/service/openai_ws_v2_passthrough_adapter.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Produces: `OpenAIAutoSchedulerOutcomeRecorder.TryRecord(OpenAIAutoSchedulerRecordInput) bool`
- Produces: `Stop(context.Context) error`
- Consumes: existing `OpenAIAutoSchedulerService.Record` and timing fields from Task 4.

- [ ] **Step 1: Write failing recorder tests**

```go
func TestOpenAIAutoSchedulerOutcomeRecorderDoesNotBlockWhenFull(t *testing.T) {
	sink := newBlockingOutcomeSink()
	recorder := NewOpenAIAutoSchedulerOutcomeRecorder(sink, 1, 1)
	t.Cleanup(func() { _ = recorder.Stop(context.Background()) })
	require.True(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 1}))
	start := time.Now()
	require.False(t, recorder.TryRecord(OpenAIAutoSchedulerRecordInput{AccountID: 2}))
	require.Less(t, time.Since(start), 50*time.Millisecond)
}
```

Also cover draining accepted records during shutdown and rejecting records after stop.

- [ ] **Step 2: Run recorder tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIAutoSchedulerOutcomeRecorder -count=1`

Expected: FAIL because the recorder does not exist.

- [ ] **Step 3: Implement the bounded recorder**

```go
type openAIAutoSchedulerOutcomeSink interface {
	Record(context.Context, OpenAIAutoSchedulerRecordInput) error
}

type OpenAIAutoSchedulerOutcomeRecorder struct {
	queue   chan OpenAIAutoSchedulerRecordInput
	stopCh  chan struct{}
	stopped atomic.Bool
	wg      sync.WaitGroup
	sink    openAIAutoSchedulerOutcomeSink
}

func (r *OpenAIAutoSchedulerOutcomeRecorder) TryRecord(input OpenAIAutoSchedulerRecordInput) bool {
	if r == nil || r.stopped.Load() {
		return false
	}
	select {
	case r.queue <- input:
		return true
	default:
		return false
	}
}
```

Workers use a bounded context per record. Expose accepted, failed, and dropped counters through existing logging/metrics patterns.

- [ ] **Step 4: Replace the detached goroutine helper**

`recordOpenAIAutoSchedulerOutcome` must enrich account/group/model fields and call `TryRecord`; it must not create a goroutine. Wire the recorder into `OpenAIGatewayService` through the existing scheduler injection boundary.

- [ ] **Step 5: Restore success/error calls on every OpenAI transport**

For successful responses, report `success`, `slow`, or `severe_slow` using end-to-end TTFT and configured thresholds. For 429 use `rate_limited`; for other upstream errors use `error`. Add calls at the final outcome boundary so a retry attempt is recorded once and the final response is not double-counted.

- [ ] **Step 6: Generate Wire and run focused tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./cmd/server -run 'OutcomeRecorder|AutoSchedulerOutcome|WireGen' -count=1
```

Expected: PASS; `rg 'recordOpenAIAutoSchedulerOutcome\(' backend/internal/service --glob '!**/*_test.go'` lists real forwarding call sites.

- [ ] **Step 7: Commit restored real feedback**

```bash
git add backend/internal/service/openai_auto_scheduler_outcome_recorder.go \
  backend/internal/service/openai_auto_scheduler_outcome_recorder_test.go \
  backend/internal/service/openai_gateway_scheduling.go backend/internal/service/openai_gateway_forward.go \
  backend/internal/service/openai_gateway_passthrough.go backend/internal/service/openai_ws_forwarder_support.go \
  backend/internal/service/openai_ws_v2_passthrough_adapter.go backend/internal/service/wire.go \
  backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "fix: restore real OpenAI scheduler feedback"
```

### Task 6: Phase Verification

**Files:**
- Test only; no production file changes expected.

**Interfaces:**
- Verifies all outputs from Tasks 1-5.

- [ ] **Step 1: Run focused service and repository tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin -run 'OpenAI.*(Scheduler|Timing|Usage|Probe|Outcome|Wait|Retry)' -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 2: Run full backend tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./... -count=1`

Expected: PASS with zero package failures.

- [ ] **Step 3: Run race tests for the new concurrent components**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'OpenAIRequestTiming|OpenAIAutoSchedulerOutcomeRecorder' -count=1`

Expected: PASS with no race reports.

- [ ] **Step 4: Verify migration and worktree scope**

Run:

```bash
git diff --check custom-main...HEAD
git status --short
git log --oneline custom-main..HEAD
```

Expected: no whitespace errors; only files named in this plan are changed; commits match Tasks 1-5.
