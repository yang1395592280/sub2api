# OpenAI Unified Scheduler Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace split advanced/automatic ranking with one balanced, batch-loaded health policy while preserving all existing OpenAI scheduling contracts.

**Architecture:** Store physical-account health by model family, endpoint, and transport in a new additive table. A focused policy module receives already hard-filtered candidates plus batch health/load data and returns a deterministic decision/order; the existing account scheduler remains responsible for slot acquisition and failover. Probe and upstream-price runners share existing leader-lock infrastructure and deduplicate physical accounts.

**Tech Stack:** Go, Ent, PostgreSQL, Redis leader locks, Gin, Wire, testify.

## Global Constraints

- Execute only after `2026-07-13-openai-scheduler-observability-feedback.md` passes.
- Keep `/openai-auto-scheduler/*`, old score state/event tables, setting keys, and group switches backward compatible.
- Health is keyed by physical account, not business group.
- `previous_response_id` remains strong sticky; ordinary session stickiness follows balanced-mode escape rules.
- Price guard, model/endpoint/transport support, quota, rate limit, account status, parent health, Compact, overbrush, and excluded IDs remain hard gates outside soft scoring.
- Default Top-K is 3; default exploration is 3%.
- Ordinary session escape requires both a 1,000ms absolute gap and a 25% relative gap, unless a hard escape signal is present.
- Half-open accounts never enter normal Top-K.
- Migration number `183` is reserved by this plan; verify it is unused immediately before implementation.
- New implementation belongs in focused files; keep changes in `openai_account_scheduler.go` and gateway forwarding files thin.

---

## File Structure

- Create `backend/ent/schema/openai_scheduler_health_state.go`: unified physical-account health schema.
- Create `backend/migrations/183_openai_scheduler_health_states.sql`: additive table and indexes.
- Create `backend/internal/service/openai_scheduler_health.go`: health key/snapshot/repository interfaces.
- Create `backend/internal/service/openai_scheduler_health_score.go`: event application, decay, expiry, and circuit state machine.
- Create `backend/internal/service/openai_scheduler_health_score_test.go`: deterministic state tests.
- Create `backend/internal/repository/openai_scheduler_health_repo.go`: batch load and upsert.
- Create `backend/internal/repository/openai_scheduler_health_repo_test.go`: SQL/Ent repository tests.
- Create `backend/internal/service/openai_balanced_scheduler.go`: balanced policy and shadow comparison.
- Create `backend/internal/service/openai_balanced_scheduler_test.go`: ordering, sticky, SLA, and exploration tests.
- Create `backend/internal/service/openai_scheduler_overview_service.go`: control-console aggregates.
- Modify existing auto scheduler settings, handler, probe runner, account scheduler, group price runner, Wire providers, and tests as listed by task.

### Task 1: Add Unified Health State Storage

**Files:**
- Create: `backend/ent/schema/openai_scheduler_health_state.go`
- Create: `backend/migrations/183_openai_scheduler_health_states.sql`
- Create: `backend/internal/service/openai_scheduler_health.go`
- Generated: `backend/ent/*`
- Test: `backend/ent/schema/openai_auto_scheduler_schema_test.go`

**Interfaces:**
- Produces: `OpenAISchedulerHealthKey`, `OpenAISchedulerHealthSnapshot`, and `OpenAISchedulerHealthRepository`.

- [ ] **Step 1: Write a failing schema contract test**

```go
func TestOpenAISchedulerHealthStateSchema(t *testing.T) {
	schema := OpenAISchedulerHealthState{}
	requireSchemaFields(t, schema,
		"account_id", "model_family", "endpoint", "transport", "state",
		"predicted_ttft_ms", "error_rate", "rate_limited_rate", "server_error_rate",
		"real_sample_count", "probe_sample_count", "last_real_at", "last_probe_at",
		"cooldown_until", "expires_at",
	)
}
```

- [ ] **Step 2: Run the schema test and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent/schema -run TestOpenAISchedulerHealthStateSchema -count=1`

Expected: compile FAIL because the schema does not exist.

- [ ] **Step 3: Define service types and repository contract**

```go
type OpenAISchedulerHealthKey struct {
	AccountID  int64
	ModelFamily string
	Endpoint    string
	Transport   string
}

type OpenAISchedulerHealthSnapshot struct {
	Key                   OpenAISchedulerHealthKey
	State                 string
	PredictedTTFTMS       float64
	ErrorRate             float64
	RateLimitedRate       float64
	ServerErrorRate       float64
	ConsecutiveSlow       int
	ConsecutiveError      int
	ConsecutiveSuccess    int
	RealSampleCount       int64
	ProbeSampleCount      int64
	LastRealAt            *time.Time
	LastProbeAt           *time.Time
	CooldownUntil         *time.Time
	ExpiresAt             time.Time
}

type OpenAISchedulerHealthRepository interface {
	GetBatch(context.Context, []OpenAISchedulerHealthKey) (map[OpenAISchedulerHealthKey]OpenAISchedulerHealthSnapshot, error)
	Upsert(context.Context, OpenAISchedulerHealthSnapshot) error
}
```

Normalize all key strings before repository calls.

- [ ] **Step 4: Add Ent schema and migration**

The Ent schema uses a unique index on `account_id, model_family, endpoint, transport`; `predicted_ttft_ms` uses PostgreSQL `decimal(12,3)`, rates use `decimal(8,4)`, and timestamps use `timestamptz`.

```sql
CREATE TABLE IF NOT EXISTS openai_scheduler_health_states (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  model_family VARCHAR(100) NOT NULL DEFAULT '',
  endpoint VARCHAR(100) NOT NULL DEFAULT '',
  transport VARCHAR(32) NOT NULL DEFAULT '',
  state VARCHAR(20) NOT NULL DEFAULT 'running',
  predicted_ttft_ms DECIMAL(12,3) NOT NULL DEFAULT 0,
  error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  rate_limited_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  server_error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  consecutive_slow INTEGER NOT NULL DEFAULT 0,
  consecutive_error INTEGER NOT NULL DEFAULT 0,
  consecutive_success INTEGER NOT NULL DEFAULT 0,
  real_sample_count BIGINT NOT NULL DEFAULT 0,
  probe_sample_count BIGINT NOT NULL DEFAULT 0,
  last_real_at TIMESTAMPTZ,
  last_probe_at TIMESTAMPTZ,
  cooldown_until TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_openai_scheduler_health_key
  ON openai_scheduler_health_states(account_id, model_family, endpoint, transport);
CREATE INDEX IF NOT EXISTS idx_openai_scheduler_health_expiry
  ON openai_scheduler_health_states(expires_at);
```

- [ ] **Step 5: Generate Ent and run schema tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./ent
GOCACHE=/tmp/sub2api-go-cache go test ./ent/schema -run 'OpenAISchedulerHealth|OpenAIAutoScheduler' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the storage contract**

```bash
git add backend/ent backend/migrations/183_openai_scheduler_health_states.sql \
  backend/internal/service/openai_scheduler_health.go
git commit -m "feat: add unified OpenAI health state storage"
```

### Task 2: Implement Batch Health Repository

**Files:**
- Create: `backend/internal/repository/openai_scheduler_health_repo.go`
- Test: `backend/internal/repository/openai_scheduler_health_repo_test.go`
- Modify: `backend/internal/repository/wire.go`

**Interfaces:**
- Consumes: Task 1 repository interface.
- Produces: one database query for any candidate batch and an atomic upsert per health key.

- [ ] **Step 1: Write failing batch-load tests**

```go
func TestOpenAISchedulerHealthRepositoryGetBatch(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &openAISchedulerHealthRepository{client: client}
	ctx := context.Background()
	keys := []service.OpenAISchedulerHealthKey{
		{AccountID: 10, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
		{AccountID: 11, ModelFamily: "gpt-5", Endpoint: "responses", Transport: "sse"},
	}
	require.NoError(t, repo.Upsert(ctx, service.OpenAISchedulerHealthSnapshot{
		Key: keys[0], State: service.OpenAIAutoSchedulerStateRunning,
		PredictedTTFTMS: 1400, ExpiresAt: time.Now().Add(30 * time.Minute),
	}))
	got, err := repo.GetBatch(ctx, keys)
	require.NoError(t, err)
	require.Contains(t, got, keys[0])
	require.NotContains(t, got, keys[1])
}
```

- [ ] **Step 2: Run repository test and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run TestOpenAISchedulerHealthRepository -count=1`

Expected: FAIL because repository implementation is missing.

- [ ] **Step 3: Implement one OR predicate query and upsert**

Build normalized unique keys, query with an Ent OR predicate, and return a map. Upsert uses `OnConflictColumns(account_id, model_family, endpoint, transport).UpdateNewValues()`.

```go
func (r *openAISchedulerHealthRepository) GetBatch(ctx context.Context, keys []service.OpenAISchedulerHealthKey) (map[service.OpenAISchedulerHealthKey]service.OpenAISchedulerHealthSnapshot, error) {
	keys = normalizeUniqueOpenAIHealthKeys(keys)
	result := make(map[service.OpenAISchedulerHealthKey]service.OpenAISchedulerHealthSnapshot, len(keys))
	if len(keys) == 0 { return result, nil }
	predicates := make([]predicate.OpenAISchedulerHealthState, 0, len(keys))
	for _, key := range keys {
		predicates = append(predicates, openaischedulerhealthstate.And(
			openaischedulerhealthstate.AccountIDEQ(key.AccountID),
			openaischedulerhealthstate.ModelFamilyEQ(key.ModelFamily),
			openaischedulerhealthstate.EndpointEQ(key.Endpoint),
			openaischedulerhealthstate.TransportEQ(key.Transport),
		))
	}
	rows, err := r.client.OpenAISchedulerHealthState.Query().Where(openaischedulerhealthstate.Or(predicates...)).All(ctx)
	if err != nil { return nil, err }
	for _, row := range rows { snapshot := healthEntityToService(row); result[snapshot.Key] = snapshot }
	return result, nil
}
```

- [ ] **Step 4: Run repository tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run TestOpenAISchedulerHealthRepository -count=1`

Expected: PASS and SQL mock sees one SELECT for two keys.

- [ ] **Step 5: Commit repository implementation**

```bash
git add backend/internal/repository/openai_scheduler_health_repo.go \
  backend/internal/repository/openai_scheduler_health_repo_test.go backend/internal/repository/wire.go
git commit -m "feat: batch load OpenAI health snapshots"
```

### Task 3: Implement Health Scoring and Real-Sample Priority

**Files:**
- Create: `backend/internal/service/openai_scheduler_health_score.go`
- Test: `backend/internal/service/openai_scheduler_health_score_test.go`
- Modify: `backend/internal/service/openai_auto_scheduler_service.go`
- Modify: `backend/internal/service/openai_auto_scheduler_outcome_recorder.go`
- Modify: `backend/internal/service/openai_gateway_scheduling.go`
- Modify: `backend/internal/service/openai_gateway_forward.go`
- Modify: `backend/internal/service/openai_gateway_passthrough.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions.go`
- Modify: `backend/internal/service/openai_gateway_chat_completions_raw.go`
- Modify: `backend/internal/service/openai_gateway_cc_pipeline.go`
- Modify: `backend/internal/service/openai_embeddings.go`
- Modify: `backend/internal/service/openai_images.go`
- Modify: `backend/internal/service/openai_images_responses.go`
- Modify: `backend/internal/service/openai_ws_v2_passthrough_adapter.go`
- Modify: `backend/internal/service/openai_ws_forwarder_ingress.go`
- Modify: `backend/internal/service/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Produces: `OpenAISchedulerHealthEvent`, `OpenAISchedulerHealthSettings`, and `ApplyOpenAISchedulerHealthEvent(now, current, event, settings) OpenAISchedulerHealthSnapshot`.
- Consumes: outcome recorder events from the observability plan.
- Extends `OpenAIAutoSchedulerRecordInput` with `ModelFamily`, `Endpoint`, and `Transport`; `Model` remains unchanged for the legacy group-scoped score path.
- Normalizes `ModelFamily` from the actual upstream model with trim + lowercase and no heuristic family folding. Endpoint values are `responses`, `chat_completions`, `embeddings`, `images_generations`, or `images_edits`; transport values reuse the actual `OpenAIUpstreamTransport` value and never persist the ingress-only selector value.

- [ ] **Step 1: Write failing table tests**

Cover fast real success, slow real success, severe slow breaker, 429, 5xx, probe older than real data, TTL expiry, open cooldown, half-open recovery, metadata normalization, missing-dimension skip, HTTP/WS separation, endpoint separation, legacy write preservation, and concurrent same-key updates from two recorder workers.

```go
func TestApplyOpenAISchedulerHealthEvent_ProbeCannotOverwriteFreshRealSample(t *testing.T) {
	now := time.Unix(1000, 0)
	lastRealAt := now.Add(-time.Minute)
	current := OpenAISchedulerHealthSnapshot{
		PredictedTTFTMS: 1400,
		RealSampleCount:  20,
		LastRealAt:       &lastRealAt,
	}
	event := OpenAISchedulerHealthEvent{Source: HealthSourceProbe, TTFTMS: 12000, OccurredAt: now}
	got := ApplyOpenAISchedulerHealthEvent(now, current, event, DefaultOpenAISchedulerHealthSettings())
	require.Equal(t, 1400.0, got.PredictedTTFTMS)
	require.Equal(t, int64(1), got.ProbeSampleCount)
}
```

- [ ] **Step 2: Run score tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestApplyOpenAISchedulerHealthEvent -count=1`

Expected: FAIL because event application is missing.

- [ ] **Step 3: Implement source-aware EWMA and state transitions**

Use alpha 0.2 for real samples, alpha 0.1 for eligible probe samples, 30-minute state TTL, existing configured slow/severe thresholds, and existing consecutive breaker thresholds. A fresh real sample is one newer than `now - RealSampleFreshSeconds`; probes update counts but not predicted TTFT while that condition holds.

- [ ] **Step 4: Connect the outcome recorder sink**

For each accepted real event with a complete normalized key, serialize load-apply-upsert by that key inside the process, load the key, apply the event, and upsert unified health. Keep the legacy `OpenAIAutoSchedulerService.RecordOutcome` write for every compatibility event during rollout, including events skipped by the unified sink because metadata is incomplete. Preserve production slow/severe classification when the recorder is wired to the composite sink rather than directly to `OpenAIAutoSchedulerService`.

The actual attempt producer must populate all three dimensions explicitly. Success uses the final `OpenAIForwardResult.UpstreamModel` and actual endpoint/transport. Failure uses the model, endpoint, and transport fixed before that attempt began. Do not infer a fallback endpoint from the inbound route, do not write wildcard keys, and normalize `responses_websockets_v2_ingress` to the actual WS transport before enqueueing.

- [ ] **Step 5: Wire the unified health sink and regenerate Wire**

Add the health repository and health event sink to the service provider set, then run:

`cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server`

Expected: generated constructors pass the unified sink to the outcome recorder without manual edits.

- [ ] **Step 6: Run score and recorder tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAISchedulerHealth|OutcomeRecorder' -count=1`

Expected: PASS; real data wins over probe, slow successes can open the circuit, HTTP/WS and endpoint rows remain separate, two workers do not lose same-key updates, and missing metadata still reaches the legacy sink without creating a unified row.

- [ ] **Step 7: Commit unified health feedback**

```bash
git add backend/internal/service/openai_scheduler_health_score.go \
  backend/internal/service/openai_scheduler_health_score_test.go \
  backend/internal/service/openai_auto_scheduler_service.go \
  backend/internal/service/openai_auto_scheduler_outcome_recorder.go \
  backend/internal/service/openai_gateway_scheduling.go \
  backend/internal/service/openai_gateway_forward.go \
  backend/internal/service/openai_gateway_passthrough.go \
  backend/internal/service/openai_gateway_chat_completions.go \
  backend/internal/service/openai_gateway_chat_completions_raw.go \
  backend/internal/service/openai_gateway_cc_pipeline.go \
  backend/internal/service/openai_embeddings.go backend/internal/service/openai_images.go \
  backend/internal/service/openai_images_responses.go \
  backend/internal/service/openai_ws_v2_passthrough_adapter.go \
  backend/internal/service/openai_ws_forwarder_ingress.go \
  backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: learn OpenAI health from real requests"
```

### Task 4: Implement Balanced Selection Policy

**Files:**
- Create: `backend/internal/service/openai_balanced_scheduler.go`
- Test: `backend/internal/service/openai_balanced_scheduler_test.go`
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_account_scheduler_test.go`
- Modify: `backend/internal/service/openai_gateway_scheduling.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_embeddings.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/service/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Produces: `OpenAIBalancedScheduler.Order(context.Context, OpenAIBalancedSelectionInput) (OpenAIBalancedSelectionResult, error)`.
- Consumes: batch health repository and candidate load information already fetched by the account scheduler.
- Extends the internal `OpenAIAccountScheduleRequest` with an explicit canonical `RequiredEndpoint`. Handlers pass the requested operation: `responses`, `chat_completions`, `embeddings`, `images_generations`, or `images_edits`.
- Resolves each candidate's actual upstream endpoint with the same account/transport rules as forwarding: WS and Responses bridges use `responses`; raw-compatible API key accounts use `chat_completions`; OAuth image bridges use `responses`; direct image API key accounts preserve generations versus edits. Missing or unknown dimensions fall back to legacy order instead of using a wildcard key.

- [ ] **Step 1: Write failing policy tests**

```go
func TestOpenAIBalancedSchedulerEscapesSlowSession(t *testing.T) {
	input := OpenAIBalancedSelectionInput{
		SessionAccountID: 1,
		Candidates: []OpenAIBalancedCandidate{
			{AccountID: 1, PredictedTTFTMS: 2600, State: OpenAIAutoSchedulerStateRunning},
			{AccountID: 2, PredictedTTFTMS: 1200, State: OpenAIAutoSchedulerStateRunning},
		},
		Settings: OpenAIBalancedSettings{TopK: 3, SessionEscapeMinGapMS: 1000, SessionEscapeRatio: 0.25},
	}
	result, err := NewOpenAIBalancedScheduler(nil).Order(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.OrderedAccountIDs[0])
	require.Equal(t, "ttft", result.StickyEscapeReason)
}
```

Also test strong previous-response ordering, price only inside latency-eligible pool, group priority, queue escape, half-open exclusion, deterministic no-exploration tests, seeded 3% exploration, and exact candidate health keys for Responses/raw Chat, HTTP/WS, embeddings, OAuth image bridge, API key image generations, and API key image edits.

- [ ] **Step 2: Run policy tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestOpenAIBalancedScheduler -count=1`

Expected: FAIL because policy types do not exist.

- [ ] **Step 3: Implement balanced ordering**

The policy performs these pure steps: remove active-open/half-open candidates, compute best predicted TTFT, preserve strong previous account, apply ordinary sticky escape, form a latency-eligible pool, rank by predicted TTFT/error/queue/group priority/price/quota, select Top 3, then apply weighted order or seeded exploration.

```go
type OpenAIBalancedSelectionResult struct {
	OrderedAccountIDs []int64
	StickyEscapeReason string
	CandidateCount int
	TopK int
	Shadow bool
}
```

- [ ] **Step 4: Fix group priority at the adapter boundary**

Change `openAIAccountSchedulingPriority` to accept `groupID` and return `AccountGroup.Priority` when the account belongs to that group; fall back to account priority for ungrouped or legacy records.

- [ ] **Step 5: Add one thin integration call in the account scheduler**

After existing hard filtering, resolve a complete Task 3-compatible health key for every candidate and load all keys with one `GetBatch`. If any required dimension cannot be resolved or the health repository fails, preserve the legacy candidate order. Otherwise map candidates into `OpenAIBalancedSelectionInput`, call the policy, and reorder existing candidate structs by returned account IDs. Slot acquisition, DB recheck, compact retry, overbrush exclusions, and wait-plan creation remain unchanged.

Propagate `RequiredEndpoint` through the effective-group wrappers and entry handlers as an internal parameter only. Images must use the parsed request endpoint and must not infer generations versus edits from `OpenAIImagesCapability`. Endpoint propagation must not change request validation, model mapping, forwarding, billing, or failover.

- [ ] **Step 6: Wire the policy and regenerate Wire**

Provide `OpenAIBalancedScheduler` from the health repository and inject it through the existing OpenAI scheduler construction boundary. Run `cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server`.

- [ ] **Step 7: Run policy and account scheduler tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAIBalancedScheduler|OpenAIGatewayService_SelectAccountWithScheduler|OpenAIAccountSchedulingPriority' -count=1`

Expected: PASS; group priority is honored and previous response remains strong sticky.

- [ ] **Step 8: Commit the balanced policy**

```bash
git add backend/internal/service/openai_balanced_scheduler.go \
  backend/internal/service/openai_balanced_scheduler_test.go \
  backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_account_scheduler_test.go \
  backend/internal/service/openai_gateway_scheduling.go \
  backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_chat_completions.go \
  backend/internal/handler/openai_embeddings.go backend/internal/handler/openai_images.go \
  backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: unify OpenAI account selection policy"
```

### Task 5: Add Runtime Settings and Shadow Mode

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_types.go`
- Modify: `backend/internal/service/setting_features.go`
- Modify: `backend/internal/handler/admin/openai_auto_scheduler_handler.go`
- Modify: `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`
- Modify: `frontend/src/api/admin/openaiAutoScheduler.ts` only for generated-compatible type additions; UI use belongs to the frontend plan.

**Interfaces:**
- Produces settings: `mode`, `shadow_mode`, `top_k`, `exploration_rate`, `session_escape_min_gap_ms`, `session_escape_ratio`, `health_ttl_seconds`, `real_sample_fresh_seconds`, `probe_jitter_seconds`.

- [ ] **Step 1: Write failing normalization and validation tests**

```go
func TestNormalizeOpenAIAutoSchedulerSettings_BalancedDefaults(t *testing.T) {
	got := normalizeOpenAIAutoSchedulerSettings(OpenAIAutoSchedulerSettings{})
	require.Equal(t, "balanced", got.Mode)
	require.Equal(t, 3, got.TopK)
	require.InDelta(t, 0.03, got.ExplorationRate, 0.0001)
	require.Equal(t, 1000, got.SessionEscapeMinGapMS)
	require.InDelta(t, 0.25, got.SessionEscapeRatio, 0.0001)
}
```

- [ ] **Step 2: Run settings tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'OpenAIAutoSchedulerSettings|BalancedDefaults' -count=1`

Expected: FAIL because fields are absent.

- [ ] **Step 3: Add backward-compatible JSON settings and validation**

Old JSON without new fields normalizes to the values above. Validate Top-K 1-10, exploration 0-0.10, escape gap 0-30000ms, ratio 0-2, TTL 60-86400s, real freshness 30-3600s, and probe jitter 0 to half the probe interval.

- [ ] **Step 4: Implement shadow comparison**

When `shadow_mode=true`, compute and log/store the balanced decision but return the legacy order. Include legacy account ID, shadow account ID, predicted TTFT difference, and reason. When false, return the balanced order.

- [ ] **Step 5: Run settings and shadow tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'OpenAIAutoSchedulerSettings|Shadow' -count=1`

Expected: PASS and old settings payloads still decode.

- [ ] **Step 6: Commit settings and shadow mode**

```bash
git add backend/internal/service/openai_auto_scheduler_types.go backend/internal/service/setting_features.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler_test.go \
  frontend/src/api/admin/openaiAutoScheduler.ts
git commit -m "feat: add balanced scheduler shadow controls"
```

### Task 6: Deduplicate and Coordinate Probe Runs

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_probe_runner.go`
- Modify: `backend/internal/service/openai_auto_scheduler_probe_runner_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes: existing `LeaderLockCache`, DB advisory-lock fallback, settings, and unified health freshness.
- Produces: one probe per physical health key per interval across all groups and instances.

- [ ] **Step 1: Write failing dedupe/leader/jitter tests**

Create two enabled groups containing the same account and assert checker count is one. Create a leader-lock stub returning false and assert checker count is zero. Inject deterministic jitter source and assert next delay lies within interval ± jitter.

- [ ] **Step 2: Run probe runner tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'OpenAIAutoSchedulerProbeRunner.*(Deduplicate|Leader|Jitter|Fresh)' -count=1`

Expected: FAIL with duplicate checker calls and missing lock dependencies.

- [ ] **Step 3: Build a deduplicated probe plan**

Collect all enabled group memberships, normalize each physical health key, and retain the list of group IDs only for legacy event fan-out. Skip keys with fresh real samples. Use a timer and deterministic `nextOpenAIProbeDelay(interval, jitter, randInt64)` helper.

- [ ] **Step 4: Gate each cycle with the existing singleton leader lock**

Use key `openai-auto-scheduler-probe`, a process-unique owner, and TTL greater than the maximum cycle runtime. Release immediately after the cycle.

- [ ] **Step 5: Generate Wire and run probe tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./cmd/server -run 'OpenAIAutoSchedulerProbeRunner|WireGen' -count=1
```

Expected: PASS; same account in multiple groups produces one upstream probe.

- [ ] **Step 6: Commit coordinated probes**

```bash
git add backend/internal/service/openai_auto_scheduler_probe_runner.go \
  backend/internal/service/openai_auto_scheduler_probe_runner_test.go \
  backend/internal/service/wire.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "perf: coordinate OpenAI scheduler probes"
```

### Task 7: Deduplicate Group Price Refresh Runs

**Files:**
- Modify: `backend/internal/service/group_upstream_balance_refresh_runner.go`
- Modify: `backend/internal/service/group_upstream_balance_refresh_runner_test.go`
- Modify: `backend/internal/service/group_upstream_balance_refresh_runner_compat_test.go`
- Modify: `backend/internal/service/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Produces: one upstream balance refresh per physical account per cycle; group-specific price guards still run for every membership.

- [ ] **Step 1: Write failing multi-group dedupe test**

```go
func TestGroupUpstreamBalanceRefreshRunnerRefreshesSharedAccountOnce(t *testing.T) {
	groups := []Group{
		{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
		{ID: 20, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600},
	}
	account := Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	groupRepo := &groupUpstreamRefreshGroupRepoStub{groups: groups}
	accountRepo := &groupUpstreamRefreshAccountRepoStub{accounts: map[int64][]Account{
		10: {account},
		20: {account},
	}}
	refresher := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{42: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, refresher)
	runner.runOnce(context.Background(), time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))
	require.Equal(t, []int64{42}, refresher.calls)
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run TestGroupUpstreamBalanceRefreshRunnerRefreshesSharedAccountOnce -count=1`

Expected: FAIL because refresh count is two.

- [ ] **Step 3: Build account-to-groups plan, then refresh once**

For every due group, collect candidate account IDs into `map[int64][]Group`. Refresh each account once; apply `ApplyGroupUpstreamPriceGuard` for each associated group using the same refreshed account snapshot.

- [ ] **Step 4: Add leader lock and jitter**

Use singleton key `group-upstream-balance-refresh`, existing leader-lock fallback, and a scan timer with ±10% bounded jitter. Preserve each group's configured refresh interval.

- [ ] **Step 5: Run runner and compatibility tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'GroupUpstreamBalanceRefreshRunner|UpstreamPriceGuard' -count=1`

Expected: PASS; every group price guard runs and upstream refresh is deduplicated.

- [ ] **Step 6: Commit price refresh coordination**

```bash
git add backend/internal/service/group_upstream_balance_refresh_runner.go \
  backend/internal/service/group_upstream_balance_refresh_runner_test.go \
  backend/internal/service/group_upstream_balance_refresh_runner_compat_test.go \
  backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "perf: deduplicate upstream price refreshes"
```

### Task 8: Add Overview and Health Read APIs

**Files:**
- Create: `backend/internal/service/openai_scheduler_overview_service.go`
- Test: `backend/internal/service/openai_scheduler_overview_service_test.go`
- Modify: `backend/internal/handler/admin/openai_auto_scheduler_handler.go`
- Modify: `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/repository/usage_log_repo_stats.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/service/wire.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Produces endpoints: `GET /openai-auto-scheduler/overview` and `GET /openai-auto-scheduler/health`.
- Overview returns E2E P50/P90, selection P95, probe ratio, group summaries, trend, and slow-cause counts.
- Health returns paginated actual decision rows joined from group membership, health snapshot, load, and price.
- Produces repository contract: `GetOpenAISchedulerOverviewMetrics(context.Context, OpenAISchedulerOverviewParams) (OpenAISchedulerOverviewMetrics, error)`.

- [ ] **Step 1: Write failing service/handler tests**

```go
func TestOpenAISchedulerOverviewServiceBuildsControlConsoleMetrics(t *testing.T) {
	repo := &openAISchedulerOverviewRepoStub{
		metrics: OpenAISchedulerOverviewMetrics{E2EP50MS: 2970, E2EP90MS: 7210, SelectionP95MS: 18, ProbeRatio: 0.24},
	}
	svc := NewOpenAISchedulerOverviewService(repo)
	got, err := svc.GetOverview(context.Background(), OpenAISchedulerOverviewParams{Window: 6 * time.Hour})
	require.NoError(t, err)
	require.Equal(t, 2970.0, got.E2EP50MS)
	require.InDelta(t, 0.24, got.ProbeRatio, 0.0001)
}

type openAISchedulerOverviewRepoStub struct {
	metrics OpenAISchedulerOverviewMetrics
}

func (s *openAISchedulerOverviewRepoStub) GetOpenAISchedulerOverviewMetrics(context.Context, OpenAISchedulerOverviewParams) (OpenAISchedulerOverviewMetrics, error) {
	return s.metrics, nil
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'OpenAISchedulerOverview|OpenAISchedulerHealth' -count=1`

Expected: FAIL because services/routes are missing.

- [ ] **Step 3: Implement bounded aggregate queries**

Use PostgreSQL percentile functions on `e2e_first_token_ms`; bound windows to 1h, 6h, 24h, or 7d; group trend buckets by hour for 24h and shorter. Do not issue one query per group or account.

- [ ] **Step 4: Add backward-compatible routes and DTOs**

Register overview/health routes before the existing score mutation routes. Add fields without changing existing score/event DTOs.

- [ ] **Step 5: Wire the overview service and regenerate Wire**

Add constructor dependencies to the existing auto-scheduler handler provider and run `cd backend && GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server`.

- [ ] **Step 6: Run service/handler/API contract tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin ./internal/server -run 'OpenAIScheduler|APIContract' -count=1`

Expected: PASS and existing endpoints remain registered.

- [ ] **Step 7: Commit control-plane APIs**

```bash
git add backend/internal/service/openai_scheduler_overview_service.go \
  backend/internal/service/openai_scheduler_overview_service_test.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler.go \
  backend/internal/handler/admin/openai_auto_scheduler_handler_test.go \
  backend/internal/server/routes/admin.go backend/internal/repository/usage_log_repo_stats.go \
  backend/internal/handler/wire.go backend/internal/service/wire.go backend/cmd/server/wire_gen.go
git commit -m "feat: expose OpenAI scheduler control metrics"
```

### Task 9: Backend Phase Verification

**Files:**
- Test only.

**Interfaces:**
- Verifies Tasks 1-8 and all preserved二开 behavior.

- [ ] **Step 1: Run focused unified scheduler tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./ent/schema ./internal/repository ./internal/service ./internal/handler/admin ./internal/server \
  -run 'OpenAI(SchedulerHealth|BalancedScheduler|AutoScheduler|AccountScheduler|UpstreamBalance|UpstreamPriceGuard)|GroupUpstreamBalanceRefresh|APIContract' -count=1
```

Expected: PASS with zero failures.

- [ ] **Step 2: Run full backend tests and race tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./... -count=1
GOCACHE=/tmp/sub2api-go-cache go test -race ./internal/service -run 'OpenAISchedulerHealth|OpenAIBalancedScheduler|ProbeRunner|GroupUpstreamBalanceRefreshRunner' -count=1
```

Expected: PASS with no package failures or races.

- [ ] **Step 3: Confirm merge-friendly diff scope**

Run:

```bash
git diff --check custom-main...HEAD
git diff --stat custom-main...HEAD
git diff custom-main...HEAD -- backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_gateway_scheduling.go
```

Expected: core implementation is in new files; hot-file changes are limited to adapters and group-priority correction.
