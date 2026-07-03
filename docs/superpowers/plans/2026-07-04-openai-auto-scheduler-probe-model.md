# OpenAI Auto Scheduler Probe Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the OpenAI auto scheduler use one configurable probe model for probing, circuit filtering, speed priority, and admin UI display.

**Architecture:** Add `probe_model` to scheduler settings and normalize it in service settings. Use that model everywhere the scheduler currently hardcodes or implicitly aggregates model health. Add a lightweight account-list scheduler summary so the account table can show scheduler state and speed rank without per-row requests.

**Tech Stack:** Go backend with Ent repositories and Gin handlers; Vue 3 + TypeScript frontend; Vitest for frontend tests; Go unit tests with `testify`.

## Global Constraints

- Default response language is Simplified Chinese for user-facing summary.
- Do not introduce new dependencies.
- Do not change base account filtering semantics: status, schedulable, group, expiration, rate limit, overload, and temporary unschedulable filters stay unchanged.
- Do not add database migrations for this change; reuse existing JSON settings and scheduler state tables.
- `probe_model` defaults to `gpt-5.4` when missing or blank.
- Configured `probe_model` is the health baseline used by auto probing, account-level circuit checks, speed priority, and default UI model selection.

---

## File Structure

- Modify `backend/internal/service/openai_auto_scheduler_types.go`: add `ProbeModel`, default, normalization, and helper for speed metric.
- Modify `backend/internal/service/openai_auto_scheduler_probe_runner.go`: read `settings.ProbeModel` instead of hardcoded `gpt-5.4`.
- Modify `backend/internal/service/openai_auto_scheduler_selector.go`: use `settings.ProbeModel` for state and circuit checks; sort same-tier accounts by speed before score fallback.
- Modify `backend/internal/service/openai_auto_scheduler_service.go`: expose account summary method and compute speed ranks for a group/model.
- Modify `backend/internal/repository/openai_auto_scheduler_repo.go`: make circuit query model-scoped and add batch state lookup if needed.
- Modify `backend/internal/handler/admin/openai_auto_scheduler_handler.go`: validate and return `probe_model`.
- Modify `backend/internal/handler/admin/account_handler.go`: inject scheduler summary dependency and enrich account list DTOs.
- Modify `backend/internal/handler/dto/types.go` and `backend/internal/handler/dto/mappers.go`: add scheduler summary fields.
- Modify `backend/internal/service/wire.go` and generated wiring files if present: pass scheduler service into account handler.
- Modify `frontend/src/api/admin/openaiAutoScheduler.ts`: add `probe_model`.
- Modify `frontend/src/types/index.ts`: add account scheduler summary type.
- Modify `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`: add settings field, default filter sync, labels.
- Modify `frontend/src/views/admin/AccountsView.vue`: add account table column and cell.
- Modify `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts`: add column labels and status copy.

---

### Task 1: Configurable Probe Model Settings

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_types.go`
- Modify: `backend/internal/service/setting_service.go`
- Test: `backend/internal/service/openai_auto_scheduler_settings_test.go`
- Test: `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`

**Interfaces:**
- Produces: `OpenAIAutoSchedulerSettings.ProbeModel string`
- Produces: `OpenAIAutoSchedulerDefaultProbeModel = "gpt-5.4"`
- Produces: normalized settings where `ProbeModel` is never blank

- [ ] **Step 1: Write failing settings normalization test**

Add assertions in `TestSettingService_OpenAIAutoSchedulerSettingsDefaultsAndNormalization`:

```go
require.Equal(t, OpenAIAutoSchedulerDefaultProbeModel, settings.ProbeModel)
```

Add `probe_model` to the persisted JSON test input:

```go
ProbeModel: "  gpt-5.5  ",
```

Add saved JSON assertion:

```go
require.Equal(t, "gpt-5.5", saved.ProbeModel)
```

- [ ] **Step 2: Run failing backend test**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestSettingService_OpenAIAutoSchedulerSettings' -count=1
```

Expected: compile failure for missing `ProbeModel` or assertion failure.

- [ ] **Step 3: Implement settings field and normalization**

In `backend/internal/service/openai_auto_scheduler_types.go`, add:

```go
const OpenAIAutoSchedulerDefaultProbeModel = "gpt-5.4"

type OpenAIAutoSchedulerSettings struct {
    Enabled                          bool    `json:"enabled"`
    ProbeModel                       string  `json:"probe_model"`
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
```

Set default:

```go
ProbeModel: OpenAIAutoSchedulerDefaultProbeModel,
```

Normalize:

```go
settings.ProbeModel = strings.TrimSpace(settings.ProbeModel)
if settings.ProbeModel == "" {
    settings.ProbeModel = defaults.ProbeModel
}
```

- [ ] **Step 4: Add handler validation test**

In `backend/internal/handler/admin/openai_auto_scheduler_handler_test.go`, update settings request/response tests to include:

```json
"probe_model":"gpt-5.5"
```

Assert response contains:

```go
require.Equal(t, "gpt-5.5", gjson.Get(body, "data.probe_model").String())
```

- [ ] **Step 5: Run settings and handler tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'OpenAIAutoSchedulerSettings|OpenAIAutoSchedulerHandler' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_types.go backend/internal/service/openai_auto_scheduler_settings_test.go backend/internal/handler/admin/openai_auto_scheduler_handler_test.go
git commit -m "feat: add configurable openai scheduler probe model"
```

---

### Task 2: Probe Runner and Circuit Selection Use Probe Model

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_probe_runner.go`
- Modify: `backend/internal/service/openai_auto_scheduler_probe_runner_test.go`
- Modify: `backend/internal/service/openai_auto_scheduler_selector.go`
- Modify: `backend/internal/service/openai_auto_scheduler_selector_test.go`
- Modify: `backend/internal/repository/openai_auto_scheduler_repo.go`
- Modify: `backend/internal/repository/openai_auto_scheduler_repo_test.go`

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerSettings.ProbeModel`
- Produces: `HasOpenCircuitForSelection(ctx, accountID, groupID int64, model string) (bool, error)`
- Produces: `HasOpenCircuitScoreState(ctx, accountID, groupID int64, model string) (bool, error)`

- [ ] **Step 1: Write failing probe runner test**

In `TestOpenAIAutoSchedulerProbeRunner_ProbesEnabledOpenAIGroups`, set:

```go
settings.ProbeModel = "gpt-5.5"
```

Use:

```go
openAIAutoSchedulerProbeKey(1, 10, "gpt-5.5")
```

Assert:

```go
require.Equal(t, "gpt-5.5", record.Model)
```

- [ ] **Step 2: Write failing selector circuit scope test**

Replace the old “any model” test with:

```go
func TestOpenAIAutoSchedulerSelector_UsesProbeModelForStateAndCircuit(t *testing.T) {
    groupID := int64(10)
    settings := enabledOpenAIAutoSchedulerSettings()
    settings.ProbeModel = "gpt-5.5"
    svc := &fakeAutoSchedulerSelectorService{
        enabledGroups: map[int64]bool{10: true},
        settings: settings,
        statesByKey: map[string]OpenAIAutoSchedulerScoreState{
            selectorStateKey(1, "gpt-5.4"): {AccountID: 1, GroupID: 10, Model: "gpt-5.4", FinalScore: 9000, State: OpenAIAutoSchedulerStateOpen},
            selectorStateKey(1, "gpt-5.5"): {AccountID: 1, GroupID: 10, Model: "gpt-5.5", FinalScore: 9000, State: OpenAIAutoSchedulerStateRunning},
            selectorStateKey(2, "gpt-5.5"): {AccountID: 2, GroupID: 10, Model: "gpt-5.5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning},
        },
        openCircuitByKey: map[string]bool{
            selectorStateKey(1, "gpt-5.4"): true,
        },
    }
    selector := NewOpenAIAutoSchedulerSelector(svc)

    ranked, used := selector.Rank(context.Background(), &groupID, "gpt-4o", []*Account{{ID: 1}, {ID: 2}})

    require.True(t, used)
    require.Equal(t, []int64{1, 2}, selectorAccountIDs(ranked))
    require.Contains(t, svc.stateLookups, selectorStateKey(1, "gpt-5.5"))
    require.Contains(t, svc.circuitLookups, selectorStateKey(1, "gpt-5.5"))
}
```

Extend fake service:

```go
openCircuitByKey map[string]bool
stateLookups []string
circuitLookups []string
```

- [ ] **Step 3: Write failing repository circuit test**

In `TestOpenAIAutoSchedulerRepository_HasOpenCircuitScoreStateIgnoresExpiredCooldown`, insert one open state for `gpt-5.4` and assert `gpt-5.5` does not block:

```go
blocked, err := repo.HasOpenCircuitScoreState(ctx, 19001, 82, "gpt-5.5")
require.NoError(t, err)
require.False(t, blocked)

blocked, err = repo.HasOpenCircuitScoreState(ctx, 19001, 82, "gpt-5.4")
require.NoError(t, err)
require.True(t, blocked)
```

- [ ] **Step 4: Run failing tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'OpenAIAutoSchedulerProbeRunner_ProbesEnabledOpenAIGroups|OpenAIAutoSchedulerSelector_UsesProbeModelForStateAndCircuit|OpenAIAutoSchedulerRepository_HasOpenCircuitScoreState' -count=1
```

Expected: FAIL before implementation.

- [ ] **Step 5: Implement probe model usage**

In `openai_auto_scheduler_probe_runner.go`, replace:

```go
model := selectOpenAIAutoSchedulerProbeModel()
```

with:

```go
model := settings.ProbeModel
```

Keep helper as:

```go
func selectOpenAIAutoSchedulerProbeModel(settings OpenAIAutoSchedulerSettings) string {
    return normalizeOpenAIAutoSchedulerSettings(settings).ProbeModel
}
```

- [ ] **Step 6: Implement selector model scope**

In `openai_auto_scheduler_selector.go`, compute:

```go
settings := s.service.GetSettingsForSelection(ctx)
probeModel := normalizeOpenAIAutoSchedulerSettings(settings).ProbeModel
```

Replace state lookup model with `probeModel`:

```go
state, err := s.service.GetStateForSelection(ctx, account.ID, *groupID, probeModel)
```

Replace neutral state model with `probeModel`, and circuit call with:

```go
openCircuit, err := s.service.HasOpenCircuitForSelection(ctx, account.ID, *groupID, probeModel)
```

Update `IsAccountTemporarilyBlocked` the same way.

- [ ] **Step 7: Implement repository model-scoped circuit**

Change service and repository interfaces to accept `model string`; in `openai_auto_scheduler_repo.go` add:

```go
model = strings.TrimSpace(model)
if model == "" {
    return false, nil
}
```

Add query predicate:

```go
openaiautoschedulerscorestate.ModelEQ(model),
```

- [ ] **Step 8: Run tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'OpenAIAutoSchedulerProbeRunner|OpenAIAutoSchedulerSelector|OpenAIAutoSchedulerRepository_HasOpenCircuitScoreState' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_probe_runner.go backend/internal/service/openai_auto_scheduler_probe_runner_test.go backend/internal/service/openai_auto_scheduler_selector.go backend/internal/service/openai_auto_scheduler_selector_test.go backend/internal/repository/openai_auto_scheduler_repo.go backend/internal/repository/openai_auto_scheduler_repo_test.go
git commit -m "fix: scope openai scheduler health to probe model"
```

---

### Task 3: Speed Priority and Account Summary Backend

**Files:**
- Modify: `backend/internal/service/openai_auto_scheduler_types.go`
- Modify: `backend/internal/service/openai_auto_scheduler_selector.go`
- Modify: `backend/internal/service/openai_auto_scheduler_selector_test.go`
- Modify: `backend/internal/service/openai_auto_scheduler_service.go`
- Modify: `backend/internal/service/openai_auto_scheduler_service_test.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/handler/admin/account_handler.go`
- Modify: `backend/internal/handler/admin/admin_service_stub_test.go`
- Modify: `backend/internal/service/wire.go`

**Interfaces:**
- Produces: `OpenAIAutoSchedulerAccountSummary`
- Produces: `ListAccountSummaries(ctx context.Context, groupID int64, accountIDs []int64) (map[int64]OpenAIAutoSchedulerAccountSummary, error)`
- Produces: `dto.Account.OpenAIAutoScheduler *dto.OpenAIAutoSchedulerAccountSummary`

- [ ] **Step 1: Write failing selector speed test**

Add to `openai_auto_scheduler_selector_test.go`:

```go
func TestOpenAIAutoSchedulerSelector_RunningAccountsPreferLowerTTFB(t *testing.T) {
    groupID := int64(10)
    settings := enabledOpenAIAutoSchedulerSettings()
    settings.ProbeModel = "gpt-5.5"
    slow := 1200
    fast := 220
    selector := NewOpenAIAutoSchedulerSelector(&fakeAutoSchedulerSelectorService{
        enabledGroups: map[int64]bool{10: true},
        settings: settings,
        statesByKey: map[string]OpenAIAutoSchedulerScoreState{
            selectorStateKey(1, "gpt-5.5"): {AccountID: 1, GroupID: 10, Model: "gpt-5.5", FinalScore: 9000, State: OpenAIAutoSchedulerStateRunning, LastTtfbMS: &slow},
            selectorStateKey(2, "gpt-5.5"): {AccountID: 2, GroupID: 10, Model: "gpt-5.5", FinalScore: 6000, State: OpenAIAutoSchedulerStateRunning, LastTtfbMS: &fast},
        },
    })

    ranked, used := selector.Rank(context.Background(), &groupID, "gpt-4o", []*Account{{ID: 1}, {ID: 2}})

    require.True(t, used)
    require.Equal(t, []int64{2, 1}, selectorAccountIDs(ranked))
}
```

- [ ] **Step 2: Implement speed metric helper**

In `openai_auto_scheduler_types.go` add:

```go
func openAIAutoSchedulerSpeedMS(state OpenAIAutoSchedulerScoreState) (int, bool) {
    if state.LastTtfbMS != nil && *state.LastTtfbMS > 0 {
        return *state.LastTtfbMS, true
    }
    if state.LastLatencyMS != nil && *state.LastLatencyMS > 0 {
        return *state.LastLatencyMS, true
    }
    return 0, false
}
```

Use it in selector same-tier comparison before `effectiveScore`:

```go
if openAIAutoSchedulerStateTier(a.state) == 1 {
    speedA, okA := openAIAutoSchedulerSpeedMS(a.state)
    speedB, okB := openAIAutoSchedulerSpeedMS(b.state)
    if okA != okB {
        return okA
    }
    if okA && speedA != speedB {
        return speedA < speedB
    }
}
```

- [ ] **Step 3: Write failing account summary service test**

Add to `openai_auto_scheduler_service_test.go`:

```go
func TestOpenAIAutoSchedulerService_ListAccountSummariesRanksBySpeed(t *testing.T) {
    repo := newFakeOpenAIAutoSchedulerRepo()
    settings := enabledOpenAIAutoSchedulerSettings()
    settings.ProbeModel = "gpt-5.5"
    fast := 200
    slow := 900
    repo.states[openAIAutoSchedulerStateKey(1, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 1, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &slow}
    repo.states[openAIAutoSchedulerStateKey(2, 10, "gpt-5.5")] = OpenAIAutoSchedulerScoreState{AccountID: 2, GroupID: 10, Model: "gpt-5.5", State: OpenAIAutoSchedulerStateRunning, FinalScore: 6000, LastTtfbMS: &fast}
    svc := NewOpenAIAutoSchedulerService(repo, fakeOpenAIAutoSchedulerSettingsProvider{settings: settings})

    summaries, err := svc.ListAccountSummaries(context.Background(), 10, []int64{1, 2})

    require.NoError(t, err)
    require.Equal(t, 2, summaries[1].SpeedPriority)
    require.Equal(t, 1, summaries[2].SpeedPriority)
    require.Equal(t, "gpt-5.5", summaries[1].ProbeModel)
}
```

- [ ] **Step 4: Implement summary type and method**

Add type:

```go
type OpenAIAutoSchedulerAccountSummary struct {
    State         string
    SpeedPriority int
    SpeedMS       *int
    ProbeModel    string
    LastTtfbMS    *int
    LastLatencyMS *int
    LastError     *string
    Reason        string
    LastCheckedAt *time.Time
}
```

Add `ListAccountSummaries` to `OpenAIAutoSchedulerService`; fetch states for `settings.ProbeModel`, compute sorted running/known-speed ranks, and return map keyed by account ID.

- [ ] **Step 5: Add DTO fields and handler enrichment**

In `dto/types.go` add DTO summary:

```go
type OpenAIAutoSchedulerAccountSummary struct {
    State         string     `json:"state"`
    SpeedPriority int       `json:"speed_priority"`
    SpeedMS       *int      `json:"speed_ms,omitempty"`
    ProbeModel    string    `json:"probe_model"`
    LastTtfbMS    *int      `json:"last_ttfb_ms,omitempty"`
    LastLatencyMS *int      `json:"last_latency_ms,omitempty"`
    LastError     *string   `json:"last_error,omitempty"`
    Reason        string    `json:"reason,omitempty"`
    LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
}
```

Add to `dto.Account`:

```go
OpenAIAutoScheduler *OpenAIAutoSchedulerAccountSummary `json:"openai_auto_scheduler,omitempty"`
```

In account handler, enrich only when `groupID > 0` and the listed accounts include OpenAI accounts:

```go
summaries, _ := h.openAIAutoScheduler.ListAccountSummaries(ctx, groupID, openAIAccountIDs)
item.Account.OpenAIAutoScheduler = dto.OpenAIAutoSchedulerAccountSummaryFromService(summary)
```

- [ ] **Step 6: Run backend tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/dto ./internal/handler/admin -run 'OpenAIAutoSchedulerSelector_RunningAccountsPreferLowerTTFB|OpenAIAutoSchedulerService_ListAccountSummariesRanksBySpeed|Account' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_auto_scheduler_types.go backend/internal/service/openai_auto_scheduler_selector.go backend/internal/service/openai_auto_scheduler_selector_test.go backend/internal/service/openai_auto_scheduler_service.go backend/internal/service/openai_auto_scheduler_service_test.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/handler/admin/account_handler.go backend/internal/handler/admin/admin_service_stub_test.go backend/internal/service/wire.go
git commit -m "feat: rank openai scheduler accounts by speed"
```

---

### Task 4: OpenAI Auto Scheduler UI Config

**Files:**
- Modify: `frontend/src/api/admin/openaiAutoScheduler.ts`
- Modify: `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`
- Modify: `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes: `OpenAIAutoSchedulerSettings.probe_model`
- Produces: settings form field and filter default sync

- [ ] **Step 1: Write failing frontend test**

In `OpenAIAutoSchedulerView.spec.ts`, mock settings with:

```ts
probe_model: 'gpt-5.5',
```

Assert:

```ts
expect(wrapper.text()).toContain('gpt-5.5')
expect((wrapper.find('#scheduler-filter-model').element as HTMLSelectElement).value).toBe('gpt-5.5')
```

- [ ] **Step 2: Update API type**

Add to `OpenAIAutoSchedulerSettings`:

```ts
probe_model: string
```

- [ ] **Step 3: Update page state and template**

Add top stat card:

```vue
<div class="scheduler-stat">
  <span class="scheduler-stat-label">检测模型</span>
  <span class="scheduler-stat-value">{{ settings?.probe_model || 'gpt-5.4' }}</span>
</div>
```

Add settings form input:

```vue
<div>
  <label class="input-label" for="scheduler-settings-probe-model">检测模型</label>
  <input id="scheduler-settings-probe-model" v-model.trim="settingsForm.probe_model" class="input" />
</div>
```

Change group label:

```vue
{{ group.enabled ? '参与自动调度' : '不参与自动调度' }} · 检测模型 {{ configuredProbeModel }}
```

Set model options:

```ts
const configuredProbeModel = computed(() => settings.value?.probe_model?.trim() || 'gpt-5.4')
const schedulerModelOptions = computed(() => {
  const models = [configuredProbeModel.value, 'gpt-5.4', 'gpt-5.5']
  return [...new Set(models.filter(Boolean))]
})
```

Sync filter after loading settings:

```ts
if (!filters.model || filters.model === 'gpt-5.4') filters.model = configuredProbeModel.value
```

- [ ] **Step 4: Run frontend test**

Run:

```bash
pnpm --dir frontend test -- OpenAIAutoSchedulerView.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/admin/openaiAutoScheduler.ts frontend/src/views/admin/OpenAIAutoSchedulerView.vue frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: configure openai scheduler probe model in ui"
```

---

### Task 5: Account Management Scheduler Column

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/views/admin/__tests__/AccountsView.sparkShadow.spec.ts` or create focused account column test if existing harness supports it.
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes: `account.openai_auto_scheduler`
- Produces: account table column key `openai_auto_scheduler`

- [ ] **Step 1: Add frontend account type**

In `frontend/src/types/index.ts` add:

```ts
export interface OpenAIAutoSchedulerAccountSummary {
  state: 'running' | 'observing' | 'open' | 'half_open'
  speed_priority: number
  speed_ms?: number | null
  probe_model: string
  last_ttfb_ms?: number | null
  last_latency_ms?: number | null
  last_error?: string | null
  reason?: string
  last_checked_at?: string | null
}
```

Add to `Account`:

```ts
openai_auto_scheduler?: OpenAIAutoSchedulerAccountSummary | null
```

- [ ] **Step 2: Write failing UI test**

Use an existing AccountsView test fixture and add an OpenAI account:

```ts
openai_auto_scheduler: {
  state: 'running',
  speed_priority: 1,
  speed_ms: 220,
  probe_model: 'gpt-5.5',
  last_ttfb_ms: 220,
  last_latency_ms: 900,
  last_error: null,
  reason: 'success',
  last_checked_at: '2026-07-04T00:00:00Z'
}
```

Assert rendered text contains:

```ts
expect(wrapper.text()).toContain('#1')
expect(wrapper.text()).toContain('220ms')
expect(wrapper.text()).toContain('gpt-5.5')
```

- [ ] **Step 3: Add column and cell**

In `DEFAULT_HIDDEN_COLUMNS`, include `openai_auto_scheduler` only if the column should start hidden. If it should be immediately visible, do not add it.

Add column:

```ts
{ key: 'openai_auto_scheduler', label: t('admin.accounts.columns.openaiAutoScheduler'), sortable: false },
```

Add cell template:

```vue
<template #cell-openai_auto_scheduler="{ row }">
  <div v-if="row.openai_auto_scheduler" class="flex min-w-[9rem] flex-col gap-1">
    <span :class="openAIAutoSchedulerStateClass(row.openai_auto_scheduler.state)">
      {{ openAIAutoSchedulerStateLabel(row.openai_auto_scheduler.state) }}
    </span>
    <span class="text-xs text-gray-600 dark:text-gray-300">
      #{{ row.openai_auto_scheduler.speed_priority }} · {{ formatMs(row.openai_auto_scheduler.speed_ms) }}
    </span>
    <span class="text-xs text-gray-500 dark:text-dark-400">
      {{ row.openai_auto_scheduler.probe_model }}
    </span>
  </div>
  <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
</template>
```

Add helper functions:

```ts
function openAIAutoSchedulerStateLabel(state: string): string {
  const labels: Record<string, string> = { running: '正常', observing: '观察中', open: '熔断中', half_open: '半开探测' }
  return labels[state] || state || '-'
}
```

- [ ] **Step 4: Run frontend account test**

Run:

```bash
pnpm --dir frontend test -- AccountsView
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/index.ts frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/__tests__ frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: show openai scheduler status in account list"
```

---

### Task 6: Final Verification

**Files:**
- No new files. Verify full relevant backend and frontend suites.

**Interfaces:**
- Consumes all previous tasks.
- Produces verified branch ready for user review.

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w backend/internal/service/openai_auto_scheduler_types.go backend/internal/service/openai_auto_scheduler_probe_runner.go backend/internal/service/openai_auto_scheduler_selector.go backend/internal/service/openai_auto_scheduler_service.go backend/internal/repository/openai_auto_scheduler_repo.go backend/internal/handler/admin/openai_auto_scheduler_handler.go backend/internal/handler/admin/account_handler.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/service/wire.go
```

Expected: no output.

- [ ] **Step 2: Run backend focused tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler/dto -run 'OpenAIAutoScheduler|Account' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend focused tests**

Run:

```bash
pnpm --dir frontend test -- OpenAIAutoSchedulerView AccountsView
```

Expected: PASS.

- [ ] **Step 4: Run broader smoke tests**

Run:

```bash
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/handler/dto -count=1
pnpm --dir frontend test --run
```

Expected: PASS.

- [ ] **Step 5: Inspect diff**

Run:

```bash
git diff --stat HEAD
git status --short
```

Expected: only intended files modified; no generated junk.

