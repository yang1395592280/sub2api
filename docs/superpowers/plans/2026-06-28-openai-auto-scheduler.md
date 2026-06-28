# OpenAI Auto Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an OpenAI-only automatic scheduler that uses global and per-group enablement, score-based routing, slow/error penalties, circuit breaking, periodic recovery probes, and an admin score dashboard.

**Architecture:** Keep the current OpenAI scheduling pipeline as the source of truth for eligibility, grouping, model support, concurrency, sticky sessions, rate limits, runtime blocks, and failover. Add a focused OpenAI auto-scheduler layer after candidate filtering: settings resolve whether the feature is active for the current group, a score service maintains `account_id + group_id + model` health, a selector ranks candidates, and a probe runner restores scores outside the request hot path.

**Tech Stack:** Go service layer, Ent schemas and generated code, PostgreSQL migrations, existing `settings` table, existing Gin admin handlers/routes, Vue 3 + TypeScript admin UI, Vitest, Go unit tests.

## Global Constraints

- Scope is OpenAI only; do not alter Anthropic, Gemini, Antigravity, or Grok scheduling.
- Automatic scheduling is active only when global OpenAI auto scheduler is enabled and the current request group has `openai_auto_scheduler_enabled=true`.
- Ungrouped API keys do not use auto scheduling.
- Existing scheduler eligibility remains authoritative: account status, schedulable flags, model support, upstream pricing restrictions, concurrency, sticky sessions, rate limit cooldown, runtime block, and failover must keep working.
- `channel_price` affects scheduling score only; it must not affect billing or usage cost calculation.
- Missing score data, disabled settings, repository errors, or cache errors must degrade to existing scheduling.
- Score display is `0.0000` to `1.0000`; internal score storage uses integer basis points from `0` to `10000`.
- UI follows `docs/superpowers/specs/assets/2026-06-28-openai-auto-scheduler-ui.png`.

---

## File Structure

Create backend files:

- `backend/ent/schema/openai_auto_scheduler_score_state.go`: Ent schema for current score state.
- `backend/ent/schema/openai_auto_scheduler_score_event.go`: Ent schema for audit/debug score events.
- `backend/internal/service/openai_auto_scheduler_types.go`: settings, state constants, event types, DTOs.
- `backend/internal/service/openai_auto_scheduler_score.go`: pure scoring state machine.
- `backend/internal/service/openai_auto_scheduler_service.go`: service facade for settings, group enablement, score updates, list APIs.
- `backend/internal/service/openai_auto_scheduler_selector.go`: candidate ranking and circuit skip logic.
- `backend/internal/service/openai_auto_scheduler_probe_runner.go`: periodic recovery probe runner.
- `backend/internal/repository/openai_auto_scheduler_repo.go`: Ent repository implementation.
- `backend/internal/handler/admin/openai_auto_scheduler_handler.go`: admin HTTP handler.
- `backend/migrations/117_openai_auto_scheduler.sql`: idempotent SQL migration.

Modify backend files:

- `backend/ent/schema/group.go`: add per-group enablement field.
- `backend/internal/service/group.go`: expose `OpenAIAutoSchedulerEnabled`.
- `backend/internal/service/admin_service.go`: carry group field through create/update/list.
- `backend/internal/handler/admin/group_handler.go`: carry JSON field through group APIs.
- `backend/internal/repository/group_repo.go`: map group field to/from Ent.
- `backend/internal/service/openai_gateway_service.go`: inject selector and result recorder at the OpenAI hot path.
- `backend/internal/service/wire.go`, `backend/internal/repository/wire.go`, `backend/internal/handler/admin/wire.go`, `backend/internal/server/routes/admin.go`: wire service, repository, handler, routes.

Create frontend files:

- `frontend/src/api/admin/openaiAutoScheduler.ts`: typed admin API wrapper.
- `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`: admin dashboard page.
- `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`: UI behavior tests.
- `frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts`: API wrapper tests.

Modify frontend files:

- `frontend/src/router/index.ts`: add `/admin/openai-auto-scheduler`.
- `frontend/src/components/layout/AppSidebar.vue`: add sidebar entry near channel monitor/scheduling.
- `frontend/src/types/index.ts`: add group field to `AdminGroup`, `CreateGroupRequest`, and `UpdateGroupRequest`, and add auto scheduler DTOs.
- `frontend/src/api/admin/groups.ts`: include `openai_auto_scheduler_enabled` in create/update payload types through shared `UpdateGroupRequest`.

Generated files:

- Ent generated files under `backend/ent/**` after running the project’s existing Ent generation command.

---

### Task 1: Data Model and Migrations

**Files:**
- Create: `backend/ent/schema/openai_auto_scheduler_score_state.go`
- Create: `backend/ent/schema/openai_auto_scheduler_score_event.go`
- Create: `backend/migrations/117_openai_auto_scheduler.sql`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/internal/service/group.go`
- Modify: `backend/internal/repository/group_repo.go`
- Modify: `backend/internal/service/admin_service.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Modify: `frontend/src/types/index.ts`

**Interfaces:**
- Produces: `Group.OpenAIAutoSchedulerEnabled bool`
- Produces: JSON field `openai_auto_scheduler_enabled`
- Produces: tables `openai_auto_scheduler_score_states` and `openai_auto_scheduler_score_events`

- [ ] **Step 1: Write failing schema tests**

Add tests in `backend/ent/schema/auth_identity_schema_test.go` or create `backend/ent/schema/openai_auto_scheduler_schema_test.go` using the existing `entc/load` pattern:

```go
func TestOpenAIAutoSchedulerSchemas(t *testing.T) {
	graph, err := load.NewGraph("github.com/Wei-Shaw/sub2api/ent/schema", nil)
	require.NoError(t, err)
	schemas := map[string]*load.Schema{}
	for _, spec := range graph.Schemas {
		schemas[spec.Name] = spec
	}

	group := requireSchema(t, schemas, "Group")
	requireSchemaFields(t, group, "openai_auto_scheduler_enabled")

	state := requireSchema(t, schemas, "OpenAIAutoSchedulerScoreState")
	requireSchemaFields(t, state,
		"account_id", "group_id", "model", "final_score", "base_score",
		"latency_score", "error_score", "recovery_score", "cost_score",
		"state", "consecutive_slow_count", "consecutive_error_count",
		"consecutive_success_count", "request_count", "ttfb_sample_count",
		"slow_rate", "error_rate", "stuck_rate", "cooldown_until",
		"last_latency_ms", "last_ttfb_ms", "last_status_code",
		"last_error", "reason", "last_checked_at")
	requireHasUniqueIndex(t, state, "account_id", "group_id", "model")

	event := requireSchema(t, schemas, "OpenAIAutoSchedulerScoreEvent")
	requireSchemaFields(t, event,
		"account_id", "group_id", "model", "event_type", "score_before",
		"score_after", "latency_ms", "ttfb_ms", "status_code", "message")
}
```

- [ ] **Step 2: Run schema tests and verify failure**

Run:

```bash
cd backend
go test ./ent/schema -run TestOpenAIAutoSchedulerSchemas
```

Expected: FAIL because the new schemas and group field do not exist.

- [ ] **Step 3: Add Ent schemas and group field**

In `backend/ent/schema/group.go`, add:

```go
field.Bool("openai_auto_scheduler_enabled").
	Default(false).
	Comment("Enable OpenAI automatic score-based scheduling for this group."),
```

Create `backend/ent/schema/openai_auto_scheduler_score_state.go`:

```go
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OpenAIAutoSchedulerScoreState struct{ ent.Schema }

func (OpenAIAutoSchedulerScoreState) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "openai_auto_scheduler_score_states"}}
}

func (OpenAIAutoSchedulerScoreState) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (OpenAIAutoSchedulerScoreState) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.String("model").Default("").MaxLen(200),
		field.Int("final_score").Default(6000),
		field.Int("base_score").Default(6000),
		field.Int("latency_score").Default(0),
		field.Int("error_score").Default(0),
		field.Int("recovery_score").Default(0),
		field.Int("cost_score").Default(0),
		field.String("state").Default("running").MaxLen(20),
		field.Int("consecutive_slow_count").Default(0),
		field.Int("consecutive_error_count").Default(0),
		field.Int("consecutive_success_count").Default(0),
		field.Int64("request_count").Default(0),
		field.Int64("ttfb_sample_count").Default(0),
		field.Float("slow_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("error_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Float("stuck_rate").Default(0).SchemaType(map[string]string{dialect.Postgres: "decimal(8,4)"}),
		field.Time("cooldown_until").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int("last_latency_ms").Optional().Nillable(),
		field.Int("last_ttfb_ms").Optional().Nillable(),
		field.Int("last_status_code").Optional().Nillable(),
		field.String("last_error").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("reason").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("last_checked_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAIAutoSchedulerScoreState) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "group_id", "model").Unique(),
		index.Fields("group_id", "final_score"),
		index.Fields("group_id", "state"),
		index.Fields("cooldown_until"),
	}
}
```

Create `backend/ent/schema/openai_auto_scheduler_score_event.go`:

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type OpenAIAutoSchedulerScoreEvent struct{ ent.Schema }

func (OpenAIAutoSchedulerScoreEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "openai_auto_scheduler_score_events"}}
}

func (OpenAIAutoSchedulerScoreEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id"),
		field.Int64("group_id"),
		field.String("model").Default("").MaxLen(200),
		field.String("event_type").MaxLen(40),
		field.Int("score_before"),
		field.Int("score_after"),
		field.Int("latency_ms").Optional().Nillable(),
		field.Int("ttfb_ms").Optional().Nillable(),
		field.Int("status_code").Optional().Nillable(),
		field.String("message").Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Default(time.Now).Immutable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpenAIAutoSchedulerScoreEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id", "group_id", "model", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("event_type", "created_at"),
	}
}
```

- [ ] **Step 4: Add SQL migration**

Create `backend/migrations/117_openai_auto_scheduler.sql`:

```sql
ALTER TABLE groups
  ADD COLUMN IF NOT EXISTS openai_auto_scheduler_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS openai_auto_scheduler_score_states (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  model VARCHAR(200) NOT NULL DEFAULT '',
  final_score INTEGER NOT NULL DEFAULT 6000,
  base_score INTEGER NOT NULL DEFAULT 6000,
  latency_score INTEGER NOT NULL DEFAULT 0,
  error_score INTEGER NOT NULL DEFAULT 0,
  recovery_score INTEGER NOT NULL DEFAULT 0,
  cost_score INTEGER NOT NULL DEFAULT 0,
  state VARCHAR(20) NOT NULL DEFAULT 'running',
  consecutive_slow_count INTEGER NOT NULL DEFAULT 0,
  consecutive_error_count INTEGER NOT NULL DEFAULT 0,
  consecutive_success_count INTEGER NOT NULL DEFAULT 0,
  request_count BIGINT NOT NULL DEFAULT 0,
  ttfb_sample_count BIGINT NOT NULL DEFAULT 0,
  slow_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  error_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  stuck_rate DECIMAL(8,4) NOT NULL DEFAULT 0,
  cooldown_until TIMESTAMPTZ NULL,
  last_latency_ms INTEGER NULL,
  last_ttfb_ms INTEGER NULL,
  last_status_code INTEGER NULL,
  last_error TEXT NULL,
  reason TEXT NOT NULL DEFAULT '',
  last_checked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT openai_auto_scheduler_score_states_score_check
    CHECK (final_score >= 0 AND final_score <= 10000)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_key
  ON openai_auto_scheduler_score_states (account_id, group_id, model);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_group_score
  ON openai_auto_scheduler_score_states (group_id, final_score);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_group_state
  ON openai_auto_scheduler_score_states (group_id, state);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_state_cooldown
  ON openai_auto_scheduler_score_states (cooldown_until);

CREATE TABLE IF NOT EXISTS openai_auto_scheduler_score_events (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  model VARCHAR(200) NOT NULL DEFAULT '',
  event_type VARCHAR(40) NOT NULL,
  score_before INTEGER NOT NULL,
  score_after INTEGER NOT NULL,
  latency_ms INTEGER NULL,
  ttfb_ms INTEGER NULL,
  status_code INTEGER NULL,
  message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_account_group_model_created
  ON openai_auto_scheduler_score_events (account_id, group_id, model, created_at);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_group_created
  ON openai_auto_scheduler_score_events (group_id, created_at);
CREATE INDEX IF NOT EXISTS idx_openai_auto_scheduler_score_events_type_created
  ON openai_auto_scheduler_score_events (event_type, created_at);
```

- [ ] **Step 5: Map group field through backend and frontend DTOs**

Add `OpenAIAutoSchedulerEnabled bool` to `backend/internal/service/group.go`. Add `OpenAIAutoSchedulerEnabled bool` to `CreateGroupInput` and `*bool` to `UpdateGroupInput`.

Add `OpenAIAutoSchedulerEnabled` fields to `CreateGroupRequest` and `UpdateGroupRequest` in `backend/internal/handler/admin/group_handler.go`:

```go
OpenAIAutoSchedulerEnabled bool `json:"openai_auto_scheduler_enabled"`
```

```go
OpenAIAutoSchedulerEnabled *bool `json:"openai_auto_scheduler_enabled"`
```

Pass these fields into service create/update input. In `UpdateGroup`, assign:

```go
if input.OpenAIAutoSchedulerEnabled != nil {
	group.OpenAIAutoSchedulerEnabled = *input.OpenAIAutoSchedulerEnabled
}
```

- [ ] **Step 6: Generate Ent files**

Run:

```bash
cd backend
go generate ./ent
```

If the project uses a different Ent generation command, use the command already documented in `backend/go:generate` or `Makefile`.

- [ ] **Step 7: Run schema and group tests**

Run:

```bash
cd backend
go test ./ent/schema -run TestOpenAIAutoSchedulerSchemas
go test ./internal/service -run 'TestAdminService_.*Group|TestGroup'
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/ent backend/migrations/117_openai_auto_scheduler.sql backend/internal/service/group.go backend/internal/service/admin_service.go backend/internal/handler/admin/group_handler.go backend/internal/repository/group_repo.go frontend/src/types/index.ts
git commit -m "feat: add openai auto scheduler data model"
```

---

### Task 2: Settings and Pure Scoring Engine

**Files:**
- Create: `backend/internal/service/openai_auto_scheduler_types.go`
- Create: `backend/internal/service/openai_auto_scheduler_score.go`
- Test: `backend/internal/service/openai_auto_scheduler_score_test.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_service.go`

**Interfaces:**
- Produces: `OpenAIAutoSchedulerSettings`
- Produces: `DefaultOpenAIAutoSchedulerSettings() OpenAIAutoSchedulerSettings`
- Produces: `ApplyOpenAIAutoSchedulerEvent(now time.Time, state OpenAIAutoSchedulerScoreState, input OpenAIAutoSchedulerEventInput, settings OpenAIAutoSchedulerSettings) OpenAIAutoSchedulerScoreState`
- Produces: `(*SettingService).GetOpenAIAutoSchedulerSettings(ctx context.Context) OpenAIAutoSchedulerSettings`

- [ ] **Step 1: Write failing scoring tests**

Create `backend/internal/service/openai_auto_scheduler_score_test.go`:

```go
func TestOpenAIAutoSchedulerScore_ErrorTriggersCircuit(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.ConsecutiveErrorBreakerThreshold = 2
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	state = ApplyOpenAIAutoSchedulerEvent(now, state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventError,
		StatusCode: ptrInt(500),
		Message: "upstream HTTP 500",
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateObserving, state.State)

	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Second), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventError,
		StatusCode: ptrInt(502),
		Message: "upstream HTTP 502",
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateOpen, state.State)
	require.NotNil(t, state.CooldownUntil)
	require.Less(t, state.FinalScore, 1000)
}

func TestOpenAIAutoSchedulerScore_SlowResponsesDegradeThenRecover(t *testing.T) {
	settings := DefaultOpenAIAutoSchedulerSettings()
	settings.Enabled = true
	settings.SlowThresholdMS = 10000
	settings.ConsecutiveSlowBreakerThreshold = 3
	settings.HalfOpenSuccessThreshold = 2
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	state := NewOpenAIAutoSchedulerScoreState(10, 20, "gpt-5")

	for i := 0; i < 3; i++ {
		state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Duration(i)*time.Second), state, OpenAIAutoSchedulerEventInput{
			EventType: OpenAIAutoSchedulerEventSlow,
			LatencyMS: ptrInt(12000),
		}, settings)
	}
	require.Equal(t, OpenAIAutoSchedulerStateOpen, state.State)

	state.CooldownUntil = ptrTime(now.Add(-time.Second))
	state = ApplyOpenAIAutoSchedulerEvent(now.Add(time.Minute), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventProbeSuccess,
		LatencyMS: ptrInt(1100),
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateHalfOpen, state.State)

	state = ApplyOpenAIAutoSchedulerEvent(now.Add(2*time.Minute), state, OpenAIAutoSchedulerEventInput{
		EventType: OpenAIAutoSchedulerEventProbeSuccess,
		LatencyMS: ptrInt(900),
	}, settings)
	require.Equal(t, OpenAIAutoSchedulerStateRunning, state.State)
	require.Greater(t, state.FinalScore, 6000)
}
```

- [ ] **Step 2: Run scoring tests and verify failure**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerScore
```

Expected: FAIL because types and functions do not exist.

- [ ] **Step 3: Implement settings and score types**

Create `backend/internal/service/openai_auto_scheduler_types.go` with constants:

```go
const (
	OpenAIAutoSchedulerStateRunning  = "running"
	OpenAIAutoSchedulerStateObserving = "observing"
	OpenAIAutoSchedulerStateOpen     = "open"
	OpenAIAutoSchedulerStateHalfOpen = "half_open"

	OpenAIAutoSchedulerEventSuccess      = "success"
	OpenAIAutoSchedulerEventSlow         = "slow"
	OpenAIAutoSchedulerEventSevereSlow   = "severe_slow"
	OpenAIAutoSchedulerEventError        = "error"
	OpenAIAutoSchedulerEventRateLimited  = "rate_limited"
	OpenAIAutoSchedulerEventProbeSuccess = "probe_success"
	OpenAIAutoSchedulerEventProbeError   = "probe_error"
	OpenAIAutoSchedulerEventManualReset  = "manual_reset"
)
```

Define settings:

```go
type OpenAIAutoSchedulerSettings struct {
	Enabled                          bool    `json:"enabled"`
	ProbeIntervalSeconds             int     `json:"probe_interval_seconds"`
	SlowThresholdMS                  int     `json:"slow_threshold_ms"`
	SevereSlowThresholdMS            int     `json:"severe_slow_threshold_ms"`
	ConsecutiveSlowBreakerThreshold  int     `json:"consecutive_slow_breaker_threshold"`
	ConsecutiveErrorBreakerThreshold int     `json:"consecutive_error_breaker_threshold"`
	CooldownSeconds                  int     `json:"cooldown_seconds"`
	HalfOpenSuccessThreshold         int     `json:"half_open_success_threshold"`
	CostWeight                       float64 `json:"cost_weight"`
	RecoveryStep                     int     `json:"recovery_step"`
}

func DefaultOpenAIAutoSchedulerSettings() OpenAIAutoSchedulerSettings {
	return OpenAIAutoSchedulerSettings{
		Enabled: false,
		ProbeIntervalSeconds: 60,
		SlowThresholdMS: 10000,
		SevereSlowThresholdMS: 20000,
		ConsecutiveSlowBreakerThreshold: 3,
		ConsecutiveErrorBreakerThreshold: 2,
		CooldownSeconds: 120,
		HalfOpenSuccessThreshold: 3,
		CostWeight: 0.2,
		RecoveryStep: 800,
	}
}
```

- [ ] **Step 4: Implement pure score state machine**

Create `backend/internal/service/openai_auto_scheduler_score.go`. Include clamp helpers and deterministic state transitions:

```go
func clampScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 10000 {
		return 10000
	}
	return v
}

func NewOpenAIAutoSchedulerScoreState(accountID, groupID int64, model string) OpenAIAutoSchedulerScoreState {
	return OpenAIAutoSchedulerScoreState{
		AccountID: accountID,
		GroupID: groupID,
		Model: strings.TrimSpace(model),
		BaseScore: 6000,
		FinalScore: 6000,
		State: OpenAIAutoSchedulerStateRunning,
	}
}
```

The state transition must:

- increment slow count for `slow` and `severe_slow`
- increment error count for `error` and `probe_error`
- reset slow/error counts on success
- move to `open` when threshold is reached
- set `CooldownUntil = now + settings.CooldownSeconds`
- move expired `open` to `half_open` on the next success/probe success
- move `half_open` to `running` after `HalfOpenSuccessThreshold`

- [ ] **Step 5: Add settings service access**

In `backend/internal/service/domain_constants.go`, add:

```go
SettingKeyOpenAIAutoSchedulerSettings = "openai_auto_scheduler_settings"
```

In `backend/internal/service/setting_service.go`, add:

```go
func (s *SettingService) GetOpenAIAutoSchedulerSettings(ctx context.Context) OpenAIAutoSchedulerSettings {
	settings := DefaultOpenAIAutoSchedulerSettings()
	if s == nil || s.settingRepo == nil {
		return settings
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIAutoSchedulerSettings)
	if err != nil || strings.TrimSpace(raw) == "" {
		return settings
	}
	var parsed OpenAIAutoSchedulerSettings
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return settings
	}
	return normalizeOpenAIAutoSchedulerSettings(parsed)
}
```

Add `SetOpenAIAutoSchedulerSettings(ctx, settings)` mirroring existing settings setters.

- [ ] **Step 6: Run tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestOpenAIAutoSchedulerScore|TestSettingService'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_types.go backend/internal/service/openai_auto_scheduler_score.go backend/internal/service/openai_auto_scheduler_score_test.go backend/internal/service/domain_constants.go backend/internal/service/setting_service.go
git commit -m "feat: add openai auto scheduler scoring"
```

---

### Task 3: Repository and Service Facade

**Files:**
- Create: `backend/internal/repository/openai_auto_scheduler_repo.go`
- Create: `backend/internal/service/openai_auto_scheduler_service.go`
- Test: `backend/internal/service/openai_auto_scheduler_service_test.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/internal/service/wire.go`

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerSettings`
- Consumes: `ApplyOpenAIAutoSchedulerEvent`
- Produces: `OpenAIAutoSchedulerRepository`
- Produces: `OpenAIAutoSchedulerService`
- Produces: `Record(ctx, OpenAIAutoSchedulerRecordInput) error`
- Produces: `ListScores(ctx, OpenAIAutoSchedulerListParams) (*OpenAIAutoSchedulerScoreListResult, error)`
- Produces: `IsEnabledForGroup(ctx context.Context, groupID *int64) bool`

- [ ] **Step 1: Write failing service tests**

Create tests that use in-memory fake repos:

```go
func TestOpenAIAutoSchedulerService_IsEnabledForGroupRequiresGlobalAndGroup(t *testing.T) {
	repo := &fakeOpenAIAutoSchedulerRepo{
		groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: true, Hydrated: true, Status: StatusActive},
			11: {ID: 11, Platform: PlatformOpenAI, OpenAIAutoSchedulerEnabled: false, Hydrated: true, Status: StatusActive},
		},
	}
	svc := NewOpenAIAutoSchedulerService(repo, staticOpenAIAutoSchedulerSettings(DefaultOpenAIAutoSchedulerSettingsWithEnabled(true)))

	require.True(t, svc.IsEnabledForGroup(context.Background(), ptrInt64(10)))
	require.False(t, svc.IsEnabledForGroup(context.Background(), ptrInt64(11)))
	require.False(t, svc.IsEnabledForGroup(context.Background(), nil))
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerService
```

Expected: FAIL because service/repo do not exist.

- [ ] **Step 3: Define service repository interface**

In `backend/internal/service/openai_auto_scheduler_service.go`:

```go
type OpenAIAutoSchedulerRepository interface {
	GetGroup(ctx context.Context, groupID int64) (*Group, error)
	GetScoreState(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error)
	UpsertScoreState(ctx context.Context, state OpenAIAutoSchedulerScoreState) error
	InsertScoreEvent(ctx context.Context, event OpenAIAutoSchedulerScoreEvent) error
	ListScoreStates(ctx context.Context, params OpenAIAutoSchedulerListParams) ([]OpenAIAutoSchedulerScoreState, int64, error)
	ListEnabledOpenAIGroups(ctx context.Context) ([]Group, error)
}
```

- [ ] **Step 4: Implement service facade**

Implement:

```go
func (s *OpenAIAutoSchedulerService) IsEnabledForGroup(ctx context.Context, groupID *int64) bool
func (s *OpenAIAutoSchedulerService) Record(ctx context.Context, input OpenAIAutoSchedulerRecordInput) error
func (s *OpenAIAutoSchedulerService) ListScores(ctx context.Context, params OpenAIAutoSchedulerListParams) (*OpenAIAutoSchedulerScoreListResult, error)
func (s *OpenAIAutoSchedulerService) ResetScore(ctx context.Context, accountID, groupID int64, model string) error
```

`IsEnabledForGroup` must return false when:

- settings disabled
- groupID nil or <= 0
- repository errors
- group not OpenAI
- group status inactive
- group `OpenAIAutoSchedulerEnabled` false

- [ ] **Step 5: Implement Ent repository**

Create `backend/internal/repository/openai_auto_scheduler_repo.go` using `*ent.Client`.

Repository methods must:

- trim model to `""` for default scope
- use `OnConflictColumns(account_id, group_id, model).UpdateNewValues()` for upsert
- order scores by `final_score DESC, updated_at DESC`
- cap page size to 200
- truncate event message to 1000 characters

- [ ] **Step 6: Run tests**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerService
go test ./internal/repository -run OpenAIAutoScheduler
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_service.go backend/internal/service/openai_auto_scheduler_service_test.go backend/internal/repository/openai_auto_scheduler_repo.go backend/internal/repository/wire.go backend/internal/service/wire.go
git commit -m "feat: add openai auto scheduler service"
```

---

### Task 4: Selector and OpenAI Gateway Integration

**Files:**
- Create: `backend/internal/service/openai_auto_scheduler_selector.go`
- Test: `backend/internal/service/openai_auto_scheduler_selector_test.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Test: `backend/internal/service/openai_account_scheduler_test.go`
- Test: `backend/internal/service/openai_gateway_service_test.go`

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerService.IsEnabledForGroup`
- Consumes: `OpenAIAutoSchedulerService.Record`
- Produces: `RankOpenAIAutoSchedulerCandidates(ctx context.Context, groupID *int64, requestedModel string, candidates []*Account) ([]*Account, bool)`

- [ ] **Step 1: Write failing selector tests**

Create `backend/internal/service/openai_auto_scheduler_selector_test.go`:

```go
func TestOpenAIAutoSchedulerSelector_GroupGate(t *testing.T) {
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerService{enabledGroups: map[int64]bool{10: true}})
	accounts := []*Account{{ID: 1}, {ID: 2}}

	ranked, used := selector.Rank(context.Background(), nil, "gpt-5", accounts)
	require.False(t, used)
	require.Equal(t, accounts, ranked)

	groupID := int64(10)
	ranked, used = selector.Rank(context.Background(), &groupID, "gpt-5", accounts)
	require.True(t, used)
	require.Len(t, ranked, 2)
}

func TestOpenAIAutoSchedulerSelector_SkipsOpenCircuitAndSortsByScore(t *testing.T) {
	groupID := int64(10)
	selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerService{
		enabledGroups: map[int64]bool{10: true},
		states: map[int64]OpenAIAutoSchedulerScoreState{
			1: {AccountID: 1, GroupID: 10, Model: "gpt-5", FinalScore: 1000, State: OpenAIAutoSchedulerStateOpen, CooldownUntil: ptrTime(time.Now().Add(time.Minute))},
			2: {AccountID: 2, GroupID: 10, Model: "gpt-5", FinalScore: 9000, State: OpenAIAutoSchedulerStateRunning},
			3: {AccountID: 3, GroupID: 10, Model: "gpt-5", FinalScore: 7000, State: OpenAIAutoSchedulerStateRunning},
		},
	})
	ranked, used := selector.Rank(context.Background(), &groupID, "gpt-5", []*Account{{ID: 1}, {ID: 3}, {ID: 2}})
	require.True(t, used)
	require.Equal(t, []int64{2, 3}, accountIDs(ranked))
}
```

- [ ] **Step 2: Run selector tests and verify failure**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerSelector
```

Expected: FAIL because selector does not exist.

- [ ] **Step 3: Implement selector**

Create `backend/internal/service/openai_auto_scheduler_selector.go`:

```go
type OpenAIAutoSchedulerSelector struct {
	service openAIAutoSchedulerSelectorService
}

type openAIAutoSchedulerSelectorService interface {
	IsEnabledForGroup(ctx context.Context, groupID *int64) bool
	GetStateForSelection(ctx context.Context, accountID, groupID int64, model string) (*OpenAIAutoSchedulerScoreState, error)
}
```

`Rank` must:

- return original candidates and `used=false` when group disabled
- skip `open` states only when `CooldownUntil` is nil or in the future
- treat missing state as neutral score 6000/running
- sort by state tier, final score desc, account priority asc, `LastUsedAt` oldest first
- preserve fallback to original order on service errors

- [ ] **Step 4: Inject selector into gateway**

In `OpenAIGatewayService`, add:

```go
openAIAutoSchedulerSelector *OpenAIAutoSchedulerSelector
openAIAutoSchedulerService  *OpenAIAutoSchedulerService
```

Add setter methods if constructor churn is too large:

```go
func (s *OpenAIGatewayService) SetOpenAIAutoScheduler(selector *OpenAIAutoSchedulerSelector, svc *OpenAIAutoSchedulerService) {
	s.openAIAutoSchedulerSelector = selector
	s.openAIAutoSchedulerService = svc
}
```

In `selectAccountWithLoadAwareness`, after building `candidates`, call:

```go
if s.openAIAutoSchedulerSelector != nil {
	if ranked, used := s.openAIAutoSchedulerSelector.Rank(ctx, groupID, requestedModel, candidates); used {
		candidates = ranked
	}
}
```

Apply the same candidate ordering to non-load-aware `selectBestAccount` by adding an ordered helper before final comparison.

- [ ] **Step 5: Record request outcomes asynchronously**

Add a helper near request completion paths:

```go
func (s *OpenAIGatewayService) recordOpenAIAutoSchedulerOutcome(ctx context.Context, account *Account, groupID *int64, requestedModel string, outcome OpenAIAutoSchedulerRecordInput) {
	if s == nil || s.openAIAutoSchedulerService == nil || account == nil || groupID == nil {
		return
	}
	outcome.AccountID = account.ID
	outcome.GroupID = *groupID
	outcome.Model = requestedModel
	go func() {
		recordCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.openAIAutoSchedulerService.Record(recordCtx, outcome)
	}()
}
```

Call it from successful and error finalization points. For streaming, record first-token latency when available and final completion status when stream ends.

- [ ] **Step 6: Run gateway tests**

Run:

```bash
cd backend
go test ./internal/service -run 'TestOpenAIAutoSchedulerSelector|TestOpenAIGatewayService_SelectAccountWithScheduler|TestOpenAI.*Scheduler'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_selector.go backend/internal/service/openai_auto_scheduler_selector_test.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_account_scheduler_test.go backend/internal/service/openai_gateway_service_test.go
git commit -m "feat: route openai accounts by auto scheduler score"
```

---

### Task 5: Periodic Probe Runner

**Files:**
- Create: `backend/internal/service/openai_auto_scheduler_probe_runner.go`
- Test: `backend/internal/service/openai_auto_scheduler_probe_runner_test.go`
- Modify: `backend/internal/service/wire.go`
- Modify: app startup wire provider where background runners are started

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerService.ListEnabledOpenAIGroups`
- Consumes: `AccountRepository.ListSchedulableByGroupIDAndPlatform`
- Consumes: `OpenAIAutoSchedulerProbeChecker.Check(ctx, account, model, timeout)`
- Produces: `OpenAIAutoSchedulerProbeRunner.Start()`
- Produces: `OpenAIAutoSchedulerProbeRunner.Stop()`

- [ ] **Step 1: Write failing runner tests**

Test interval gating and in-flight dedupe:

```go
func TestOpenAIAutoSchedulerProbeRunner_SkipsWhenDisabled(t *testing.T) {
	svc := &fakeProbeSchedulerService{settings: DefaultOpenAIAutoSchedulerSettings()}
	runner := newOpenAIAutoSchedulerProbeRunnerForTest(svc, fakeProbeAccountRepo{}, fakeProbeChecker{})
	runner.runOnce(context.Background())
	require.Zero(t, svc.listGroupsCalls)
}
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerProbeRunner
```

Expected: FAIL because runner does not exist.

- [ ] **Step 3: Implement runner**

Runner behavior:

- Reads settings each tick.
- If disabled, skips without querying groups.
- Lists OpenAI groups with `OpenAIAutoSchedulerEnabled=true`.
- Lists schedulable OpenAI accounts per group.
- Uses a worker limit of 5 by default.
- Skips duplicate account/group/model checks when already in flight.
- Records `probe_success` or `probe_error`.

- [ ] **Step 4: Add the probe checker**

Create an `OpenAIAutoSchedulerProbeChecker` wrapper. First try to call the existing `AccountTestService` OpenAI check path from the wrapper; if that call cannot target one account without side effects, keep the wrapper interface and implement the HTTP check inside the wrapper with a minimal non-stream OpenAI request and timeout. The runner must depend only on this wrapper interface:

```go
type OpenAIAutoSchedulerProbeChecker interface {
	Check(ctx context.Context, account *Account, model string, timeout time.Duration) OpenAIAutoSchedulerProbeResult
}
```

- [ ] **Step 5: Run tests**

Run:

```bash
cd backend
go test ./internal/service -run TestOpenAIAutoSchedulerProbeRunner
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_probe_runner.go backend/internal/service/openai_auto_scheduler_probe_runner_test.go backend/internal/service/wire.go
git commit -m "feat: add openai auto scheduler probes"
```

---

### Task 6: Admin API

**Files:**
- Create: `backend/internal/handler/admin/openai_auto_scheduler_handler.go`
- Test: `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/handler/admin/wire.go`
- Modify: `backend/internal/server/api_contract_test.go`

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerService`
- Produces: `GET /api/v1/admin/openai-auto-scheduler/settings`
- Produces: `PUT /api/v1/admin/openai-auto-scheduler/settings`
- Produces: `GET /api/v1/admin/openai-auto-scheduler/groups`
- Produces: `PUT /api/v1/admin/openai-auto-scheduler/groups/:id`
- Produces: `GET /api/v1/admin/openai-auto-scheduler/scores`
- Produces: `GET /api/v1/admin/openai-auto-scheduler/events`
- Produces: `POST /api/v1/admin/openai-auto-scheduler/scores/:id/reset`
- Produces: `POST /api/v1/admin/openai-auto-scheduler/scores/:id/probe`

- [ ] **Step 1: Write failing handler tests**

Add tests for:

- invalid settings rejected
- group toggle persists only OpenAI groups
- scores list returns paginated rows
- reset invokes service

- [ ] **Step 2: Run handler tests and verify failure**

Run:

```bash
cd backend
go test ./internal/handler/admin -run TestOpenAIAutoScheduler
```

Expected: FAIL because handler/routes do not exist.

- [ ] **Step 3: Implement handler**

Create a handler with methods:

```go
func (h *OpenAIAutoSchedulerHandler) GetSettings(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) UpdateSettings(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) ListGroups(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) UpdateGroup(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) ListScores(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) ListEvents(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) ResetScore(c *gin.Context)
func (h *OpenAIAutoSchedulerHandler) ProbeScore(c *gin.Context)
```

Use existing `response.Success` and error helpers used by nearby admin handlers.

- [ ] **Step 4: Add routes**

In `backend/internal/server/routes/admin.go`:

```go
openAIAutoScheduler := admin.Group("/openai-auto-scheduler")
{
	openAIAutoScheduler.GET("/settings", h.Admin.OpenAIAutoScheduler.GetSettings)
	openAIAutoScheduler.PUT("/settings", h.Admin.OpenAIAutoScheduler.UpdateSettings)
	openAIAutoScheduler.GET("/groups", h.Admin.OpenAIAutoScheduler.ListGroups)
	openAIAutoScheduler.PUT("/groups/:id", h.Admin.OpenAIAutoScheduler.UpdateGroup)
	openAIAutoScheduler.GET("/scores", h.Admin.OpenAIAutoScheduler.ListScores)
	openAIAutoScheduler.GET("/events", h.Admin.OpenAIAutoScheduler.ListEvents)
	openAIAutoScheduler.POST("/scores/:id/reset", h.Admin.OpenAIAutoScheduler.ResetScore)
	openAIAutoScheduler.POST("/scores/:id/probe", h.Admin.OpenAIAutoScheduler.ProbeScore)
}
```

- [ ] **Step 5: Run backend API tests**

Run:

```bash
cd backend
go test ./internal/handler/admin -run TestOpenAIAutoScheduler
go test ./internal/server -run TestAPIContract
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handler/admin/openai_auto_scheduler_handler.go backend/internal/handler/admin/openai_auto_scheduler_handler_test.go backend/internal/server/routes/admin.go backend/internal/handler/admin/wire.go backend/internal/server/api_contract_test.go
git commit -m "feat: expose openai auto scheduler admin api"
```

---

### Task 7: Admin Frontend Page

**Files:**
- Create: `frontend/src/api/admin/openaiAutoScheduler.ts`
- Create: `frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts`
- Create: `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`
- Create: `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: translation files if this project localizes nav/settings labels

**Interfaces:**
- Consumes: Admin API from Task 6
- Produces: `/admin/openai-auto-scheduler` route

- [ ] **Step 1: Write failing API wrapper tests**

Test endpoint paths:

```ts
it('loads openai auto scheduler scores with filters', async () => {
  mock.onGet('/admin/openai-auto-scheduler/scores').reply(config => {
    expect(config.params.group_id).toBe(10)
    expect(config.params.state).toBe('open')
    return [200, { items: [], total: 0 }]
  })
  const result = await openaiAutoSchedulerAPI.listScores({ group_id: 10, state: 'open' })
  expect(result.total).toBe(0)
})
```

- [ ] **Step 2: Run frontend tests and verify failure**

Run:

```bash
cd frontend
pnpm test -- openaiAutoScheduler
```

Expected: FAIL because API wrapper does not exist.

- [ ] **Step 3: Implement API wrapper**

Create `frontend/src/api/admin/openaiAutoScheduler.ts` with types:

```ts
export interface OpenAIAutoSchedulerSettings {
  enabled: boolean
  probe_interval_seconds: number
  slow_threshold_ms: number
  severe_slow_threshold_ms: number
  consecutive_slow_breaker_threshold: number
  consecutive_error_breaker_threshold: number
  cooldown_seconds: number
  half_open_success_threshold: number
  cost_weight: number
  recovery_step: number
}
```

Add `getSettings`, `updateSettings`, `listGroups`, `updateGroup`, `listScores`, `listEvents`, `resetScore`, `probeScore`.

- [ ] **Step 4: Implement dashboard page**

Build `OpenAIAutoSchedulerView.vue` with:

- compact top settings strip
- group selector and per-group toggle
- score list matching approved design
- score color formatting
- state badges
- filters for group/model/state/search
- reset/probe row actions

Use existing app components where available: `Toggle`, `Select`, `DataTable`, `Icon`, and existing admin page spacing.

- [ ] **Step 5: Add route and sidebar**

In `frontend/src/router/index.ts`, add:

```ts
{
  path: '/admin/openai-auto-scheduler',
  name: 'AdminOpenAIAutoScheduler',
  component: () => import('@/views/admin/OpenAIAutoSchedulerView.vue'),
  meta: { requiresAuth: true, requiresAdmin: true, title: 'OpenAI 自动调度', icon: 'activity' }
}
```

In `AppSidebar.vue`, add an admin entry near channel monitor:

```ts
{ path: '/admin/openai-auto-scheduler', label: 'OpenAI 自动调度', icon: ActivityIcon }
```

- [ ] **Step 6: Run frontend tests**

Run:

```bash
cd frontend
pnpm test -- openaiAutoScheduler OpenAIAutoSchedulerView
pnpm type-check
```

Expected: PASS.

- [ ] **Step 7: Visual verification**

Start frontend and backend dev server using existing project commands. Open `/admin/openai-auto-scheduler` and verify:

- settings text does not overlap at 1365x768 and mobile width
- group toggle is visible and understandable
- three score states render clearly: running, observing, open
- long channel names truncate cleanly

- [ ] **Step 8: Commit**

```bash
git add frontend/src/api/admin/openaiAutoScheduler.ts frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts frontend/src/views/admin/OpenAIAutoSchedulerView.vue frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue
git commit -m "feat: add openai auto scheduler dashboard"
```

---

### Task 8: End-to-End Verification and Hardening

**Files:**
- Modify: `docs/superpowers/specs/2026-06-28-openai-auto-scheduler-design.md` only if behavior changed during implementation
- Create or modify focused tests discovered during implementation

**Interfaces:**
- Consumes: all previous tasks
- Produces: verified feature branch ready for merge or PR

- [ ] **Step 1: Run focused backend tests**

Run:

```bash
cd backend
go test ./internal/service -run 'OpenAIAutoScheduler|OpenAIGatewayService_SelectAccount|OpenAI.*Scheduler'
go test ./internal/handler/admin -run OpenAIAutoScheduler
go test ./internal/repository -run OpenAIAutoScheduler
```

Expected: PASS.

- [ ] **Step 2: Run broader backend tests likely affected**

Run:

```bash
cd backend
go test ./internal/service ./internal/handler/admin ./internal/server ./internal/repository
```

Expected: PASS.

- [ ] **Step 3: Run frontend tests**

Run:

```bash
cd frontend
pnpm test -- OpenAIAutoScheduler openaiAutoScheduler router AppSidebar
pnpm type-check
```

Expected: PASS.

- [ ] **Step 4: Manual scenario check**

Use a local or test environment:

1. Create two OpenAI groups.
2. Enable global OpenAI auto scheduler.
3. Enable auto scheduler only for group A.
4. Send OpenAI traffic through group A and group B.
5. Confirm group A uses score ordering.
6. Confirm group B uses existing scheduler.
7. Force one account to respond slowly.
8. Confirm slow account score drops.
9. Force one account to return 5xx.
10. Confirm error account enters `open` and is skipped.
11. Let probe success run.
12. Confirm account moves through `half_open` and then `running`.

- [ ] **Step 5: Inspect git diff for scope**

Run:

```bash
git diff --stat main...HEAD
git diff --name-only main...HEAD
```

Expected: changed files are limited to OpenAI auto scheduler, group field plumbing, generated Ent files, admin route/API, and frontend dashboard.

- [ ] **Step 6: Commit final fixes**

If verification required fixes:

```bash
git add <fixed-files>
git commit -m "fix: harden openai auto scheduler"
```

If no fixes were required, do not create an empty commit.
