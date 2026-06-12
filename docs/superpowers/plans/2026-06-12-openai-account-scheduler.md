# OpenAI Account Scheduler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an OpenAI account health scheduler that ranks accounts by runtime health, splits them into primary/standby/observe/degraded tiers, exposes admin APIs, and adds an admin page for visibility and limited manual actions.

**Architecture:** Extend the existing `OpenAIAccountScheduler` instead of replacing it. Runtime health state stays in memory on the scheduler, `accounts.priority` remains manual, settings are stored in the existing settings store, and admin APIs read the scheduler snapshot through `OpenAIGatewayService`.

**Tech Stack:** Go, Gin, Wire, Ent-backed repositories, existing SettingService, Vue 3, TypeScript, Vite, Vitest, Tailwind CSS.

---

## File Structure

Backend service files:

- Modify `backend/internal/service/openai_account_scheduler.go`
  - Add health settings, tier constants, score snapshot types, runtime health state, tiered selection, manual actions, and public snapshot methods.
- Add `backend/internal/service/openai_scheduler_health_test.go`
  - Unit tests for health score, tier transitions, recovery, manual cooldown, and open/closed setting behavior.
- Add `backend/internal/service/openai_scheduler_tier_selection_test.go`
  - Unit tests for primary-first and standby fallback selection order.

Backend admin API files:

- Add `backend/internal/handler/admin/openai_scheduler_handler.go`
  - Admin handler for overview, accounts list, account detail, actions, settings get/update.
- Add `backend/internal/handler/admin/openai_scheduler_handler_test.go`
  - Handler tests for JSON shape, validation, and error handling.
- Modify `backend/internal/handler/handler.go`
  - Add `OpenAIScheduler *admin.OpenAISchedulerHandler` to `AdminHandlers`.
- Modify `backend/internal/handler/wire.go`
  - Wire `admin.NewOpenAISchedulerHandler` and add it to `ProvideAdminHandlers`.
- Modify `backend/internal/server/routes/admin.go`
  - Register `/admin/openai-scheduler` routes.

Frontend API and routing files:

- Add `frontend/src/api/admin/openaiScheduler.ts`
  - Typed API client for scheduler endpoints.
- Modify `frontend/src/api/admin/index.ts`
  - Export `openaiSchedulerAPI` under `adminAPI.openaiScheduler`.
- Modify `frontend/src/router/index.ts`
  - Add `/admin/openai-scheduler` route.
- Modify `frontend/src/i18n/locales/zh.ts`
  - Add Chinese labels and page copy.
- Modify `frontend/src/i18n/locales/en.ts`
  - Add English labels and page copy.
- Modify `frontend/src/components/layout/AppSidebar.vue`
  - Add admin sidebar entry.

Frontend page files:

- Add `frontend/src/views/admin/OpenAISchedulerView.vue`
  - Page layout, summary cards, settings panel, account table, action dialog.
- Add `frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts`
  - Component tests for load, rendering, settings validation, and actions.
- Add `frontend/src/api/admin/__tests__/openaiScheduler.spec.ts`
  - API client tests, if this folder pattern exists during implementation; otherwise keep API behavior covered through component tests.

Documentation files:

- Modify `docs/superpowers/plans/2026-06-12-openai-account-scheduler.md`
  - Track implementation progress by checking off steps during execution.

---

## Task 1: Backend Health Model And Settings

**Files:**

- Modify: `backend/internal/service/openai_account_scheduler.go`
- Test: `backend/internal/service/openai_scheduler_health_test.go`

- [ ] **Step 1: Write failing tests for default health settings and score calculation**

Create `backend/internal/service/openai_scheduler_health_test.go` with:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerHealthSettings_Defaults(t *testing.T) {
	settings := defaultOpenAISchedulerHealthSettings()

	require.False(t, settings.HealthRankingEnabled)
	require.Equal(t, 0.30, settings.PrimaryRatio)
	require.Equal(t, 1, settings.PrimaryMinCount)
	require.Equal(t, 2500, settings.TTFTDegradeMS)
	require.Equal(t, 0.35, settings.ErrorRateDegradeThreshold)
	require.Equal(t, 3, settings.ConsecutiveFailureThreshold)
	require.Equal(t, 5, settings.RecoverSuccessThreshold)
	require.Equal(t, 10*time.Minute, settings.Cooldown)
	require.Equal(t, 0.0, settings.ObserveProbeRatio)
}

func TestOpenAISchedulerHealthScore_SuccessLowLatencyPrimary(t *testing.T) {
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	stat := openAIAccountHealthRuntime{
		successEWMA:        0.98,
		errorEWMA:          0.02,
		ttftEWMA:           900,
		consecutiveSuccess: 8,
	}

	snapshot := buildOpenAIAccountHealthSnapshot(101, stat, settings, time.Now())

	require.Equal(t, OpenAISchedulerTierPrimary, snapshot.Tier)
	require.GreaterOrEqual(t, snapshot.HealthScore, 90.0)
	require.Equal(t, "", snapshot.DegradeReason)
}

func TestOpenAISchedulerHealthScore_HighLatencyObserve(t *testing.T) {
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	stat := openAIAccountHealthRuntime{
		successEWMA:        0.95,
		errorEWMA:          0.05,
		ttftEWMA:           4200,
		consecutiveSuccess: 4,
	}

	snapshot := buildOpenAIAccountHealthSnapshot(102, stat, settings, time.Now())

	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeHighLatency, snapshot.DegradeReason)
	require.Less(t, snapshot.HealthScore, 90.0)
}

func TestOpenAISchedulerHealthScore_ConsecutiveFailuresDegraded(t *testing.T) {
	now := time.Now()
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	settings.Cooldown = 5 * time.Minute
	stat := openAIAccountHealthRuntime{
		successEWMA:          0.30,
		errorEWMA:            0.70,
		ttftEWMA:             1200,
		consecutiveFailures:  3,
		lastDegradeReason:    OpenAISchedulerDegradeUpstream5xx,
		cooldownUntilUnixSec: now.Add(settings.Cooldown).Unix(),
	}

	snapshot := buildOpenAIAccountHealthSnapshot(103, stat, settings, now)

	require.Equal(t, OpenAISchedulerTierDegraded, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeUpstream5xx, snapshot.DegradeReason)
	require.NotNil(t, snapshot.CooldownUntil)
}

func TestOpenAISchedulerHealthScore_CooldownExpiredObserve(t *testing.T) {
	now := time.Now()
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	stat := openAIAccountHealthRuntime{
		successEWMA:          0.60,
		errorEWMA:            0.40,
		ttftEWMA:             1300,
		cooldownUntilUnixSec: now.Add(-time.Minute).Unix(),
		lastDegradeReason:    OpenAISchedulerDegradeTimeout,
	}

	snapshot := buildOpenAIAccountHealthSnapshot(104, stat, settings, now)

	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeRecovering, snapshot.DegradeReason)
	require.Nil(t, snapshot.CooldownUntil)
}

func TestDefaultOpenAIAccountScheduler_ReportResultUpdatesHealth(t *testing.T) {
	stats := newOpenAIAccountRuntimeStats()
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          stats,
		healthSettings: defaultOpenAISchedulerHealthSettings(),
	}
	ttft := 800

	scheduler.ReportResult(201, true, &ttft)
	scheduler.ReportResult(201, false, nil)

	snapshot, ok := scheduler.SnapshotAccountHealth(context.Background(), 201)
	require.True(t, ok)
	require.Equal(t, int64(201), snapshot.AccountID)
	require.Greater(t, snapshot.ErrorRateEWMA, 0.0)
	require.Greater(t, snapshot.TTFTEWMAMS, 0.0)
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/service -run 'TestOpenAISchedulerHealth|TestDefaultOpenAIAccountScheduler_ReportResultUpdatesHealth' -count=1
```

Expected:

- FAIL with undefined symbols such as `defaultOpenAISchedulerHealthSettings`, `OpenAISchedulerTierPrimary`, `buildOpenAIAccountHealthSnapshot`, or `SnapshotAccountHealth`.

- [ ] **Step 3: Add health constants, settings, runtime state, and snapshot builders**

In `backend/internal/service/openai_account_scheduler.go`, add near the existing OpenAI scheduler constants:

```go
const (
	OpenAISchedulerTierPrimary  = "primary"
	OpenAISchedulerTierStandby  = "standby"
	OpenAISchedulerTierObserve  = "observe"
	OpenAISchedulerTierDegraded = "degraded"
)

const (
	OpenAISchedulerDegradeHighLatency = "high_latency"
	OpenAISchedulerDegradeRateLimited = "rate_limited"
	OpenAISchedulerDegradeUpstream5xx = "upstream_5xx"
	OpenAISchedulerDegradeTimeout     = "timeout"
	OpenAISchedulerDegradeRecovering  = "recovering"
	OpenAISchedulerDegradeManual      = "manual"
)
```

Add the settings and snapshot types near `OpenAIAccountSchedulerMetricsSnapshot`:

```go
type OpenAISchedulerHealthSettings struct {
	HealthRankingEnabled         bool
	PrimaryRatio                 float64
	PrimaryMinCount              int
	TTFTDegradeMS                int
	ErrorRateDegradeThreshold    float64
	ConsecutiveFailureThreshold  int
	RecoverSuccessThreshold      int
	Cooldown                     time.Duration
	ObserveProbeRatio            float64
}

type OpenAIAccountHealthSnapshot struct {
	AccountID          int64      `json:"account_id"`
	HealthScore       float64    `json:"health_score"`
	Tier              string     `json:"tier"`
	DegradeReason     string     `json:"degrade_reason"`
	CooldownUntil     *time.Time `json:"cooldown_until,omitempty"`
	SuccessRateEWMA   float64    `json:"success_rate_ewma"`
	ErrorRateEWMA     float64    `json:"error_rate_ewma"`
	TTFTEWMAMS        float64    `json:"ttft_ewma_ms"`
	ConsecutiveErrors int        `json:"consecutive_errors"`
	ConsecutiveOK     int        `json:"consecutive_ok"`
	LastSelectedAt    *time.Time `json:"last_selected_at,omitempty"`
	LastErrorAt       *time.Time `json:"last_error_at,omitempty"`
	DecisionReason    string     `json:"decision_reason"`
}
```

Add runtime health state near `openAIAccountRuntimeStat`:

```go
type openAIAccountHealthRuntime struct {
	successEWMA          float64
	errorEWMA            float64
	ttftEWMA             float64
	consecutiveFailures  int
	consecutiveSuccess   int
	lastDegradeReason    string
	cooldownUntilUnixSec int64
	lastSelectedUnixSec  int64
	lastErrorUnixSec     int64
}
```

Extend `openAIAccountRuntimeStat`:

```go
type openAIAccountRuntimeStat struct {
	errorRateEWMABits atomic.Uint64
	ttftEWMABits      atomic.Uint64
	healthMu          sync.RWMutex
	health            openAIAccountHealthRuntime
}
```

Add helper functions:

```go
func defaultOpenAISchedulerHealthSettings() OpenAISchedulerHealthSettings {
	return OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        false,
		PrimaryRatio:                0.30,
		PrimaryMinCount:             1,
		TTFTDegradeMS:               2500,
		ErrorRateDegradeThreshold:   0.35,
		ConsecutiveFailureThreshold: 3,
		RecoverSuccessThreshold:     5,
		Cooldown:                    10 * time.Minute,
		ObserveProbeRatio:           0,
	}
}

func clampOpenAISchedulerRatio(v float64, fallback float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1 {
		return fallback
	}
	return v
}

func buildOpenAIAccountHealthSnapshot(accountID int64, stat openAIAccountHealthRuntime, settings OpenAISchedulerHealthSettings, now time.Time) OpenAIAccountHealthSnapshot {
	if now.IsZero() {
		now = time.Now()
	}
	successRate := clamp01(stat.successEWMA)
	errorRate := clamp01(stat.errorEWMA)
	if successRate == 0 && errorRate == 0 {
		successRate = 1
	}

	latencyPenalty := 0.0
	if settings.TTFTDegradeMS > 0 && stat.ttftEWMA > float64(settings.TTFTDegradeMS) {
		latencyPenalty = clamp01((stat.ttftEWMA - float64(settings.TTFTDegradeMS)) / float64(settings.TTFTDegradeMS))
	}

	score := 100.0
	score -= errorRate * 55
	score -= latencyPenalty * 25
	score -= float64(stat.consecutiveFailures) * 6
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	tier := OpenAISchedulerTierStandby
	reason := stat.lastDegradeReason
	var cooldownUntil *time.Time
	if stat.cooldownUntilUnixSec > 0 {
		cooldownAt := time.Unix(stat.cooldownUntilUnixSec, 0)
		if cooldownAt.After(now) {
			tier = OpenAISchedulerTierDegraded
			cooldownUntil = &cooldownAt
			if reason == "" {
				reason = OpenAISchedulerDegradeManual
			}
		} else {
			tier = OpenAISchedulerTierObserve
			reason = OpenAISchedulerDegradeRecovering
		}
	} else if settings.ConsecutiveFailureThreshold > 0 && stat.consecutiveFailures >= settings.ConsecutiveFailureThreshold {
		tier = OpenAISchedulerTierDegraded
		if reason == "" {
			reason = OpenAISchedulerDegradeUpstream5xx
		}
	} else if settings.ErrorRateDegradeThreshold > 0 && errorRate >= settings.ErrorRateDegradeThreshold {
		tier = OpenAISchedulerTierObserve
		if reason == "" {
			reason = OpenAISchedulerDegradeRecovering
		}
	} else if settings.TTFTDegradeMS > 0 && stat.ttftEWMA > float64(settings.TTFTDegradeMS) {
		tier = OpenAISchedulerTierObserve
		reason = OpenAISchedulerDegradeHighLatency
	} else if stat.consecutiveSuccess >= settings.RecoverSuccessThreshold && score >= 90 {
		tier = OpenAISchedulerTierPrimary
		reason = ""
	} else if score >= 90 {
		tier = OpenAISchedulerTierPrimary
		reason = ""
	}

	return OpenAIAccountHealthSnapshot{
		AccountID:          accountID,
		HealthScore:       math.Round(score*10) / 10,
		Tier:              tier,
		DegradeReason:     reason,
		CooldownUntil:     cooldownUntil,
		SuccessRateEWMA:   successRate,
		ErrorRateEWMA:     errorRate,
		TTFTEWMAMS:        stat.ttftEWMA,
		ConsecutiveErrors: stat.consecutiveFailures,
		ConsecutiveOK:     stat.consecutiveSuccess,
		DecisionReason:    openAIHealthDecisionReason(tier, reason),
	}
}

func openAIHealthDecisionReason(tier, reason string) string {
	switch tier {
	case OpenAISchedulerTierPrimary:
		return "health score is high and account is eligible for primary routing"
	case OpenAISchedulerTierStandby:
		return "account is healthy but below the primary cutoff"
	case OpenAISchedulerTierObserve:
		if reason != "" {
			return "account is being observed after " + reason
		}
		return "account is being observed before returning to active routing"
	case OpenAISchedulerTierDegraded:
		if reason != "" {
			return "account is degraded because of " + reason
		}
		return "account is degraded"
	default:
		return "health state is unknown"
	}
}
```

- [ ] **Step 4: Update runtime reporting and expose snapshots**

Update `openAIAccountRuntimeStats.report` so it also updates `health` under lock:

```go
func (s *openAIAccountRuntimeStats) report(accountID int64, success bool, firstTokenMs *int) {
	if s == nil || accountID <= 0 {
		return
	}
	const alpha = 0.2
	stat := s.loadOrCreate(accountID)

	errorSample := 1.0
	successSample := 0.0
	if success {
		errorSample = 0.0
		successSample = 1.0
	}
	updateEWMAAtomic(&stat.errorRateEWMABits, errorSample, alpha)

	var ttftValue float64
	if firstTokenMs != nil && *firstTokenMs > 0 {
		ttftValue = float64(*firstTokenMs)
		ttftBits := math.Float64bits(ttftValue)
		for {
			oldBits := stat.ttftEWMABits.Load()
			oldValue := math.Float64frombits(oldBits)
			if math.IsNaN(oldValue) {
				if stat.ttftEWMABits.CompareAndSwap(oldBits, ttftBits) {
					break
				}
				continue
			}
			newValue := alpha*ttftValue + (1-alpha)*oldValue
			if stat.ttftEWMABits.CompareAndSwap(oldBits, math.Float64bits(newValue)) {
				ttftValue = newValue
				break
			}
		}
	}

	nowUnix := time.Now().Unix()
	stat.healthMu.Lock()
	h := stat.health
	h.errorEWMA = alpha*errorSample + (1-alpha)*h.errorEWMA
	h.successEWMA = alpha*successSample + (1-alpha)*h.successEWMA
	if ttftValue > 0 {
		if h.ttftEWMA <= 0 {
			h.ttftEWMA = ttftValue
		} else {
			h.ttftEWMA = alpha*ttftValue + (1-alpha)*h.ttftEWMA
		}
	}
	if success {
		h.consecutiveSuccess++
		h.consecutiveFailures = 0
	} else {
		h.consecutiveFailures++
		h.consecutiveSuccess = 0
		h.lastErrorUnixSec = nowUnix
		if h.lastDegradeReason == "" {
			h.lastDegradeReason = OpenAISchedulerDegradeUpstream5xx
		}
	}
	stat.health = h
	stat.healthMu.Unlock()
}
```

Add snapshot methods:

```go
func (s *openAIAccountRuntimeStats) healthSnapshot(accountID int64, settings OpenAISchedulerHealthSettings, now time.Time) (OpenAIAccountHealthSnapshot, bool) {
	if s == nil || accountID <= 0 {
		return OpenAIAccountHealthSnapshot{}, false
	}
	value, ok := s.accounts.Load(accountID)
	if !ok {
		return buildOpenAIAccountHealthSnapshot(accountID, openAIAccountHealthRuntime{successEWMA: 1}, settings, now), true
	}
	stat, _ := value.(*openAIAccountRuntimeStat)
	if stat == nil {
		return OpenAIAccountHealthSnapshot{}, false
	}
	stat.healthMu.RLock()
	health := stat.health
	stat.healthMu.RUnlock()
	return buildOpenAIAccountHealthSnapshot(accountID, health, settings, now), true
}
```

Extend `OpenAIAccountScheduler` interface:

```go
	SnapshotAccountHealth(ctx context.Context, accountID int64) (OpenAIAccountHealthSnapshot, bool)
```

Add field to `defaultOpenAIAccountScheduler`:

```go
	healthSettings OpenAISchedulerHealthSettings
```

Update constructor:

```go
func newDefaultOpenAIAccountScheduler(service *OpenAIGatewayService, stats *openAIAccountRuntimeStats) OpenAIAccountScheduler {
	if stats == nil {
		stats = newOpenAIAccountRuntimeStats()
	}
	return &defaultOpenAIAccountScheduler{
		service:        service,
		stats:          stats,
		healthSettings: defaultOpenAISchedulerHealthSettings(),
	}
}
```

Add method:

```go
func (s *defaultOpenAIAccountScheduler) SnapshotAccountHealth(ctx context.Context, accountID int64) (OpenAIAccountHealthSnapshot, bool) {
	if s == nil || s.stats == nil {
		return OpenAIAccountHealthSnapshot{}, false
	}
	settings := s.healthSettings
	return s.stats.healthSnapshot(accountID, settings, time.Now())
}
```

- [ ] **Step 5: Run tests and verify they pass**

Run:

```bash
go test ./internal/service -run 'TestOpenAISchedulerHealth|TestDefaultOpenAIAccountScheduler_ReportResultUpdatesHealth' -count=1
```

Expected:

- PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_scheduler_health_test.go
git commit -m "feat: 增加OpenAI调度健康模型"
```

---

## Task 2: Health Tier Selection In Existing Scheduler

**Files:**

- Modify: `backend/internal/service/openai_account_scheduler.go`
- Test: `backend/internal/service/openai_scheduler_tier_selection_test.go`

- [ ] **Step 1: Write failing tests for tier ordering**

Create `backend/internal/service/openai_scheduler_tier_selection_test.go` with:

```go
package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerBuildSelectionOrder_PrimaryBeforeStandby(t *testing.T) {
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: settings,
	}

	primary := &Account{ID: 1, Priority: 50, Concurrency: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}
	standby := &Account{ID: 2, Priority: 1, Concurrency: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}

	scheduler.seedHealthForTest(1, openAIAccountHealthRuntime{successEWMA: 0.98, errorEWMA: 0.02, ttftEWMA: 700, consecutiveSuccess: 8})
	scheduler.seedHealthForTest(2, openAIAccountHealthRuntime{successEWMA: 0.75, errorEWMA: 0.25, ttftEWMA: 1500, consecutiveSuccess: 1})

	plan := openAIAccountLoadPlan{
		candidates: []openAIAccountCandidateScore{
			{account: standby, loadInfo: &AccountLoadInfo{AccountID: 2}},
			{account: primary, loadInfo: &AccountLoadInfo{AccountID: 1}},
		},
		topK: 2,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{}, plan)

	require.Len(t, order, 2)
	require.Equal(t, int64(1), order[0].account.ID)
	require.Equal(t, int64(2), order[1].account.ID)
}

func TestOpenAISchedulerBuildSelectionOrder_DegradedLast(t *testing.T) {
	now := time.Now()
	settings := defaultOpenAISchedulerHealthSettings()
	settings.HealthRankingEnabled = true
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: settings,
	}

	healthy := &Account{ID: 10, Priority: 50, Concurrency: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}
	degraded := &Account{ID: 11, Priority: 1, Concurrency: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}

	scheduler.seedHealthForTest(10, openAIAccountHealthRuntime{successEWMA: 0.92, errorEWMA: 0.08, ttftEWMA: 1100, consecutiveSuccess: 6})
	scheduler.seedHealthForTest(11, openAIAccountHealthRuntime{successEWMA: 0.10, errorEWMA: 0.90, ttftEWMA: 1000, consecutiveFailures: 4, cooldownUntilUnixSec: now.Add(5 * time.Minute).Unix(), lastDegradeReason: OpenAISchedulerDegradeRateLimited})

	plan := openAIAccountLoadPlan{
		candidates: []openAIAccountCandidateScore{
			{account: degraded, loadInfo: &AccountLoadInfo{AccountID: 11}},
			{account: healthy, loadInfo: &AccountLoadInfo{AccountID: 10}},
		},
		topK: 2,
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{}, plan)

	require.Len(t, order, 2)
	require.Equal(t, int64(10), order[0].account.ID)
	require.Equal(t, int64(11), order[1].account.ID)
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/service -run 'TestOpenAISchedulerBuildSelectionOrder' -count=1
```

Expected:

- FAIL because `seedHealthForTest` is undefined or tier ordering is not applied.

- [ ] **Step 3: Add tier weight to candidate score and ordering**

In `openAIAccountCandidateScore`, add:

```go
	health OpenAIAccountHealthSnapshot
```

In `buildOpenAIAccountLoadPlan`, after runtime snapshot:

```go
		health := buildOpenAIAccountHealthSnapshot(account.ID, openAIAccountHealthRuntime{successEWMA: 1}, s.healthSettings, time.Now())
		if s.stats != nil {
			if snapshot, ok := s.stats.healthSnapshot(account.ID, s.healthSettings, time.Now()); ok {
				health = snapshot
			}
		}
```

Include `health: health` in the appended candidate.

Update scoring to include health when enabled:

```go
		if s.healthSettings.HealthRankingEnabled {
			healthFactor := clamp01(item.health.HealthScore / 100)
			item.score = item.score*0.65 + healthFactor*0.35
		}
```

Add tier order helpers:

```go
func openAISchedulerTierRank(tier string) int {
	switch tier {
	case OpenAISchedulerTierPrimary:
		return 0
	case OpenAISchedulerTierStandby:
		return 1
	case OpenAISchedulerTierObserve:
		return 2
	case OpenAISchedulerTierDegraded:
		return 3
	default:
		return 1
	}
}

func splitOpenAISelectionPools(candidates []openAIAccountCandidateScore) [][]openAIAccountCandidateScore {
	pools := make([][]openAIAccountCandidateScore, 4)
	for _, candidate := range candidates {
		rank := openAISchedulerTierRank(candidate.health.Tier)
		if rank < 0 || rank >= len(pools) {
			rank = 1
		}
		pools[rank] = append(pools[rank], candidate)
	}
	return pools
}
```

Update `buildOpenAISelectionOrder` non-compact branch:

```go
	if s.healthSettings.HealthRankingEnabled {
		selectionOrder := make([]openAIAccountCandidateScore, 0, len(plan.candidates))
		for _, pool := range splitOpenAISelectionPools(plan.candidates) {
			selectionOrder = append(selectionOrder, buildSelectionOrder(pool)...)
		}
		return selectionOrder
	}
	return buildSelectionOrder(plan.candidates)
```

For compact mode, apply the same split within supported and unknown pools:

```go
		appendTiered := func(dst []openAIAccountCandidateScore, pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
			if !s.healthSettings.HealthRankingEnabled {
				return append(dst, buildSelectionOrder(pool)...)
			}
			for _, tierPool := range splitOpenAISelectionPools(pool) {
				dst = append(dst, buildSelectionOrder(tierPool)...)
			}
			return dst
		}
		selectionOrder := make([]openAIAccountCandidateScore, 0, len(plan.allCandidates))
		selectionOrder = appendTiered(selectionOrder, supported)
		selectionOrder = appendTiered(selectionOrder, unknown)
```

- [ ] **Step 4: Add test-only health seeding helper**

Add in `openai_account_scheduler.go`:

```go
func (s *defaultOpenAIAccountScheduler) seedHealthForTest(accountID int64, health openAIAccountHealthRuntime) {
	if s == nil {
		return
	}
	if s.stats == nil {
		s.stats = newOpenAIAccountRuntimeStats()
	}
	stat := s.stats.loadOrCreate(accountID)
	stat.healthMu.Lock()
	stat.health = health
	stat.healthMu.Unlock()
}
```

This helper is unexported and only used by package-level tests.

- [ ] **Step 5: Run tier selection tests**

Run:

```bash
go test ./internal/service -run 'TestOpenAISchedulerBuildSelectionOrder' -count=1
```

Expected:

- PASS.

- [ ] **Step 6: Run existing OpenAI scheduler tests**

Run:

```bash
go test ./internal/service -run 'TestDefaultOpenAIAccountScheduler|TestOpenAIGatewayService_OpenAIAccountScheduler|TestOpenAISelectAccount' -count=1
```

Expected:

- PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_scheduler_tier_selection_test.go
git commit -m "feat: 接入OpenAI健康分层调度"
```

---

## Task 3: Settings And Manual Health Actions

**Files:**

- Modify: `backend/internal/service/openai_account_scheduler.go`
- Test: `backend/internal/service/openai_scheduler_health_test.go`

- [ ] **Step 1: Add failing tests for settings update and manual cooldown**

Append to `backend/internal/service/openai_scheduler_health_test.go`:

```go
func TestDefaultOpenAIAccountScheduler_UpdateHealthSettingsClampsValues(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: defaultOpenAISchedulerHealthSettings(),
	}

	scheduler.UpdateHealthSettings(OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        true,
		PrimaryRatio:                2,
		PrimaryMinCount:             -4,
		TTFTDegradeMS:               -1,
		ErrorRateDegradeThreshold:   9,
		ConsecutiveFailureThreshold: -2,
		RecoverSuccessThreshold:     -5,
		Cooldown:                    -time.Second,
		ObserveProbeRatio:           3,
	})

	settings := scheduler.SnapshotHealthSettings()
	require.True(t, settings.HealthRankingEnabled)
	require.Equal(t, 0.30, settings.PrimaryRatio)
	require.Equal(t, 1, settings.PrimaryMinCount)
	require.Equal(t, 2500, settings.TTFTDegradeMS)
	require.Equal(t, 0.35, settings.ErrorRateDegradeThreshold)
	require.Equal(t, 3, settings.ConsecutiveFailureThreshold)
	require.Equal(t, 5, settings.RecoverSuccessThreshold)
	require.Equal(t, 10*time.Minute, settings.Cooldown)
	require.Equal(t, 0.0, settings.ObserveProbeRatio)
}

func TestDefaultOpenAIAccountScheduler_ManualCooldownAndClear(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{
		stats:          newOpenAIAccountRuntimeStats(),
		healthSettings: defaultOpenAISchedulerHealthSettings(),
	}
	scheduler.healthSettings.HealthRankingEnabled = true

	err := scheduler.ApplyHealthAction(301, OpenAISchedulerHealthAction{
		Action:   "cooldown",
		Reason:   "manual maintenance",
		Duration: time.Minute,
	})
	require.NoError(t, err)

	snapshot, ok := scheduler.SnapshotAccountHealth(context.Background(), 301)
	require.True(t, ok)
	require.Equal(t, OpenAISchedulerTierDegraded, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeManual, snapshot.DegradeReason)
	require.NotNil(t, snapshot.CooldownUntil)

	err = scheduler.ApplyHealthAction(301, OpenAISchedulerHealthAction{Action: "clear_cooldown"})
	require.NoError(t, err)

	snapshot, ok = scheduler.SnapshotAccountHealth(context.Background(), 301)
	require.True(t, ok)
	require.Equal(t, OpenAISchedulerTierObserve, snapshot.Tier)
	require.Equal(t, OpenAISchedulerDegradeRecovering, snapshot.DegradeReason)
}

func TestDefaultOpenAIAccountScheduler_InvalidManualAction(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{stats: newOpenAIAccountRuntimeStats(), healthSettings: defaultOpenAISchedulerHealthSettings()}

	err := scheduler.ApplyHealthAction(302, OpenAISchedulerHealthAction{Action: "pin_primary"})

	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/service -run 'TestDefaultOpenAIAccountScheduler_(UpdateHealthSettings|ManualCooldown|InvalidManualAction)' -count=1
```

Expected:

- FAIL because `UpdateHealthSettings`, `SnapshotHealthSettings`, `OpenAISchedulerHealthAction`, or `ApplyHealthAction` is undefined.

- [ ] **Step 3: Add settings/action APIs to scheduler interface**

Extend `OpenAIAccountScheduler`:

```go
	SnapshotHealthSettings() OpenAISchedulerHealthSettings
	UpdateHealthSettings(settings OpenAISchedulerHealthSettings)
	ApplyHealthAction(accountID int64, action OpenAISchedulerHealthAction) error
```

Add action type:

```go
type OpenAISchedulerHealthAction struct {
	Action   string
	Reason   string
	Duration time.Duration
}
```

Add fields to `defaultOpenAIAccountScheduler`:

```go
	healthSettingsMu sync.RWMutex
```

- [ ] **Step 4: Implement settings validation and manual actions**

Add:

```go
func normalizeOpenAISchedulerHealthSettings(input OpenAISchedulerHealthSettings) OpenAISchedulerHealthSettings {
	defaults := defaultOpenAISchedulerHealthSettings()
	input.PrimaryRatio = clampOpenAISchedulerRatio(input.PrimaryRatio, defaults.PrimaryRatio)
	if input.PrimaryMinCount <= 0 {
		input.PrimaryMinCount = defaults.PrimaryMinCount
	}
	if input.TTFTDegradeMS <= 0 {
		input.TTFTDegradeMS = defaults.TTFTDegradeMS
	}
	input.ErrorRateDegradeThreshold = clampOpenAISchedulerRatio(input.ErrorRateDegradeThreshold, defaults.ErrorRateDegradeThreshold)
	if input.ErrorRateDegradeThreshold == 0 {
		input.ErrorRateDegradeThreshold = defaults.ErrorRateDegradeThreshold
	}
	if input.ConsecutiveFailureThreshold <= 0 {
		input.ConsecutiveFailureThreshold = defaults.ConsecutiveFailureThreshold
	}
	if input.RecoverSuccessThreshold <= 0 {
		input.RecoverSuccessThreshold = defaults.RecoverSuccessThreshold
	}
	if input.Cooldown <= 0 {
		input.Cooldown = defaults.Cooldown
	}
	input.ObserveProbeRatio = clampOpenAISchedulerRatio(input.ObserveProbeRatio, defaults.ObserveProbeRatio)
	return input
}

func (s *defaultOpenAIAccountScheduler) SnapshotHealthSettings() OpenAISchedulerHealthSettings {
	if s == nil {
		return defaultOpenAISchedulerHealthSettings()
	}
	s.healthSettingsMu.RLock()
	defer s.healthSettingsMu.RUnlock()
	return s.healthSettings
}

func (s *defaultOpenAIAccountScheduler) UpdateHealthSettings(settings OpenAISchedulerHealthSettings) {
	if s == nil {
		return
	}
	s.healthSettingsMu.Lock()
	s.healthSettings = normalizeOpenAISchedulerHealthSettings(settings)
	s.healthSettingsMu.Unlock()
}

func (s *defaultOpenAIAccountScheduler) ApplyHealthAction(accountID int64, action OpenAISchedulerHealthAction) error {
	if s == nil || accountID <= 0 {
		return fmt.Errorf("invalid account id")
	}
	if s.stats == nil {
		s.stats = newOpenAIAccountRuntimeStats()
	}
	stat := s.stats.loadOrCreate(accountID)
	now := time.Now()

	stat.healthMu.Lock()
	defer stat.healthMu.Unlock()
	h := stat.health
	switch strings.TrimSpace(action.Action) {
	case "cooldown":
		duration := action.Duration
		if duration <= 0 {
			duration = s.SnapshotHealthSettings().Cooldown
		}
		h.cooldownUntilUnixSec = now.Add(duration).Unix()
		h.lastDegradeReason = OpenAISchedulerDegradeManual
		h.lastErrorUnixSec = now.Unix()
	case "clear_cooldown":
		h.cooldownUntilUnixSec = now.Add(-time.Second).Unix()
		h.lastDegradeReason = OpenAISchedulerDegradeRecovering
		h.consecutiveFailures = 0
	case "promote_observe":
		h.cooldownUntilUnixSec = now.Add(-time.Second).Unix()
		h.lastDegradeReason = OpenAISchedulerDegradeRecovering
	case "run_probe":
		h.lastDegradeReason = OpenAISchedulerDegradeRecovering
	default:
		return fmt.Errorf("unsupported scheduler action: %s", action.Action)
	}
	stat.health = h
	return nil
}
```

Update `SnapshotAccountHealth` to use `SnapshotHealthSettings()` instead of reading `s.healthSettings` directly.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/service -run 'TestDefaultOpenAIAccountScheduler_(UpdateHealthSettings|ManualCooldown|InvalidManualAction)|TestOpenAISchedulerHealth' -count=1
```

Expected:

- PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_scheduler_health_test.go
git commit -m "feat: 支持OpenAI调度健康策略与操作"
```

---

## Task 4: Persistent Settings And Public Gateway Service Methods

**Files:**

- Modify: `backend/internal/service/openai_account_scheduler.go`
- Test: `backend/internal/service/openai_scheduler_health_test.go`

- [ ] **Step 1: Add failing tests for service-level settings methods**

Append to `backend/internal/service/openai_scheduler_health_test.go`:

```go
func TestOpenAIGatewayService_HealthSchedulerDisabledReturnsDefaults(t *testing.T) {
	svc := &OpenAIGatewayService{}

	settings := svc.SnapshotOpenAISchedulerHealthSettings()
	snapshot, ok := svc.SnapshotOpenAIAccountHealth(context.Background(), 1)

	require.False(t, settings.HealthRankingEnabled)
	require.False(t, ok)
	require.Equal(t, OpenAIAccountHealthSnapshot{}, snapshot)
}

func TestOpenAISchedulerHealthSettingsRoundTrip(t *testing.T) {
	repo := &openAISchedulerSettingRepoStub{values: map[string]string{}}
	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{
			settingService: &SettingService{settingRepo: repo},
		},
	}

	input := OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        true,
		PrimaryRatio:                0.4,
		PrimaryMinCount:             2,
		TTFTDegradeMS:               1800,
		ErrorRateDegradeThreshold:   0.25,
		ConsecutiveFailureThreshold: 2,
		RecoverSuccessThreshold:     4,
		Cooldown:                    3 * time.Minute,
		ObserveProbeRatio:           0.05,
	}

	require.NoError(t, svc.SaveOpenAISchedulerHealthSettings(context.Background(), input))
	got := svc.SnapshotOpenAISchedulerHealthSettings()

	require.True(t, got.HealthRankingEnabled)
	require.Equal(t, 0.4, got.PrimaryRatio)
	require.Equal(t, 2, got.PrimaryMinCount)
	require.Equal(t, 1800, got.TTFTDegradeMS)
	require.Equal(t, 0.25, got.ErrorRateDegradeThreshold)
	require.Equal(t, 2, got.ConsecutiveFailureThreshold)
	require.Equal(t, 4, got.RecoverSuccessThreshold)
	require.Equal(t, 3*time.Minute, got.Cooldown)
	require.Equal(t, 0.05, got.ObserveProbeRatio)
}

type openAISchedulerSettingRepoStub struct {
	values map[string]string
}

func (r *openAISchedulerSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *openAISchedulerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (r *openAISchedulerSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *openAISchedulerSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *openAISchedulerSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *openAISchedulerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *openAISchedulerSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
go test ./internal/service -run 'TestOpenAIGatewayService_HealthSchedulerDisabledReturnsDefaults|TestOpenAISchedulerHealthSettingsRoundTrip' -count=1
```

Expected:

- FAIL because service-level methods or setting keys are undefined.

- [ ] **Step 3: Add setting keys and parse/encode helpers**

In `backend/internal/service/openai_account_scheduler.go`, add:

```go
const (
	openAISchedulerHealthRankingEnabledKey        = "openai_scheduler_health_ranking_enabled"
	openAISchedulerPrimaryRatioKey                = "openai_scheduler_primary_ratio"
	openAISchedulerPrimaryMinCountKey             = "openai_scheduler_primary_min_count"
	openAISchedulerTTFTDegradeMSKey               = "openai_scheduler_ttft_degrade_ms"
	openAISchedulerErrorRateDegradeThresholdKey   = "openai_scheduler_error_rate_degrade_threshold"
	openAISchedulerConsecutiveFailureThresholdKey = "openai_scheduler_consecutive_failure_threshold"
	openAISchedulerRecoverSuccessThresholdKey     = "openai_scheduler_recover_success_threshold"
	openAISchedulerCooldownSecondsKey             = "openai_scheduler_cooldown_seconds"
	openAISchedulerObserveProbeRatioKey           = "openai_scheduler_observe_probe_ratio"
)

var openAISchedulerHealthSettingKeys = []string{
	openAISchedulerHealthRankingEnabledKey,
	openAISchedulerPrimaryRatioKey,
	openAISchedulerPrimaryMinCountKey,
	openAISchedulerTTFTDegradeMSKey,
	openAISchedulerErrorRateDegradeThresholdKey,
	openAISchedulerConsecutiveFailureThresholdKey,
	openAISchedulerRecoverSuccessThresholdKey,
	openAISchedulerCooldownSecondsKey,
	openAISchedulerObserveProbeRatioKey,
}

func parseOpenAISchedulerHealthSettings(values map[string]string) OpenAISchedulerHealthSettings {
	settings := defaultOpenAISchedulerHealthSettings()
	if raw := strings.TrimSpace(values[openAISchedulerHealthRankingEnabledKey]); raw != "" {
		settings.HealthRankingEnabled = strings.EqualFold(raw, "true")
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(values[openAISchedulerPrimaryRatioKey]), 64); err == nil {
		settings.PrimaryRatio = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(values[openAISchedulerPrimaryMinCountKey])); err == nil {
		settings.PrimaryMinCount = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(values[openAISchedulerTTFTDegradeMSKey])); err == nil {
		settings.TTFTDegradeMS = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(values[openAISchedulerErrorRateDegradeThresholdKey]), 64); err == nil {
		settings.ErrorRateDegradeThreshold = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(values[openAISchedulerConsecutiveFailureThresholdKey])); err == nil {
		settings.ConsecutiveFailureThreshold = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(values[openAISchedulerRecoverSuccessThresholdKey])); err == nil {
		settings.RecoverSuccessThreshold = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(values[openAISchedulerCooldownSecondsKey])); err == nil {
		settings.Cooldown = time.Duration(v) * time.Second
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(values[openAISchedulerObserveProbeRatioKey]), 64); err == nil {
		settings.ObserveProbeRatio = v
	}
	return normalizeOpenAISchedulerHealthSettings(settings)
}

func encodeOpenAISchedulerHealthSettings(settings OpenAISchedulerHealthSettings) map[string]string {
	settings = normalizeOpenAISchedulerHealthSettings(settings)
	return map[string]string{
		openAISchedulerHealthRankingEnabledKey:        strconv.FormatBool(settings.HealthRankingEnabled),
		openAISchedulerPrimaryRatioKey:                strconv.FormatFloat(settings.PrimaryRatio, 'f', -1, 64),
		openAISchedulerPrimaryMinCountKey:             strconv.Itoa(settings.PrimaryMinCount),
		openAISchedulerTTFTDegradeMSKey:               strconv.Itoa(settings.TTFTDegradeMS),
		openAISchedulerErrorRateDegradeThresholdKey:   strconv.FormatFloat(settings.ErrorRateDegradeThreshold, 'f', -1, 64),
		openAISchedulerConsecutiveFailureThresholdKey: strconv.Itoa(settings.ConsecutiveFailureThreshold),
		openAISchedulerRecoverSuccessThresholdKey:     strconv.Itoa(settings.RecoverSuccessThreshold),
		openAISchedulerCooldownSecondsKey:             strconv.Itoa(int(settings.Cooldown.Seconds())),
		openAISchedulerObserveProbeRatioKey:           strconv.FormatFloat(settings.ObserveProbeRatio, 'f', -1, 64),
	}
}
```

- [ ] **Step 4: Add service-level persistent wrapper methods**

In `backend/internal/service/openai_account_scheduler.go`, add:

```go
func (s *OpenAIGatewayService) SnapshotOpenAISchedulerHealthSettings() OpenAISchedulerHealthSettings {
	settings := defaultOpenAISchedulerHealthSettings()
	if repo := s.openAIAdvancedSchedulerSettingRepo(); repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), openAIAdvancedSchedulerSettingDBTimeout)
		defer cancel()
		if values, err := repo.GetMultiple(ctx, openAISchedulerHealthSettingKeys); err == nil {
			settings = parseOpenAISchedulerHealthSettings(values)
		}
	}
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler != nil {
		scheduler.UpdateHealthSettings(settings)
	}
	return settings
}

func (s *OpenAIGatewayService) SaveOpenAISchedulerHealthSettings(ctx context.Context, settings OpenAISchedulerHealthSettings) error {
	settings = normalizeOpenAISchedulerHealthSettings(settings)
	if repo := s.openAIAdvancedSchedulerSettingRepo(); repo != nil {
		if err := repo.SetMultiple(ctx, encodeOpenAISchedulerHealthSettings(settings)); err != nil {
			return err
		}
	}
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler != nil {
		scheduler.UpdateHealthSettings(settings)
	}
	return nil
}

func (s *OpenAIGatewayService) SnapshotOpenAIAccountHealth(ctx context.Context, accountID int64) (OpenAIAccountHealthSnapshot, bool) {
	scheduler := s.getOpenAIAccountScheduler(ctx)
	if scheduler == nil {
		return OpenAIAccountHealthSnapshot{}, false
	}
	return scheduler.SnapshotAccountHealth(ctx, accountID)
}

func (s *OpenAIGatewayService) ApplyOpenAISchedulerHealthAction(accountID int64, action OpenAISchedulerHealthAction) error {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return fmt.Errorf("openai advanced scheduler is not enabled")
	}
	return scheduler.ApplyHealthAction(accountID, action)
}
```

- [ ] **Step 5: Run service tests**

Run:

```bash
go test ./internal/service -run 'TestOpenAIGatewayService_HealthSchedulerDisabledReturnsDefaults|TestOpenAISchedulerHealthSettingsRoundTrip|TestOpenAISchedulerHealth' -count=1
```

Expected:

- PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_scheduler_health_test.go
git commit -m "feat: 持久化OpenAI调度健康策略"
```

---

## Task 5: Admin OpenAI Scheduler Handler And Routes

**Files:**

- Create: `backend/internal/handler/admin/openai_scheduler_handler.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/admin.go`
- Test: `backend/internal/handler/admin/openai_scheduler_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Create `backend/internal/handler/admin/openai_scheduler_handler_test.go` with:

```go
package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAISchedulerHandler_GetSettings_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/settings", h.GetSettings)

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"health_ranking_enabled":false`)
	require.Contains(t, w.Body.String(), `"primary_ratio":0.3`)
}

func TestOpenAISchedulerHandler_ListAccounts_NoAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/accounts", h.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/accounts?page=1&page_size=20", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items":[]`)
	require.Contains(t, w.Body.String(), `"total":0`)
}

func TestOpenAISchedulerHandler_ActionInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.POST("/accounts/:id/actions", h.ApplyAction)

	req := httptest.NewRequest(http.MethodPost, "/accounts/bad/actions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler' -count=1
```

Expected:

- FAIL because `NewOpenAISchedulerHandler` is undefined.

- [ ] **Step 3: Implement handler**

Create `backend/internal/handler/admin/openai_scheduler_handler.go`:

```go
package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type OpenAISchedulerHandler struct {
	gatewayService *service.OpenAIGatewayService
}

func NewOpenAISchedulerHandler(gatewayService *service.OpenAIGatewayService) *OpenAISchedulerHandler {
	return &OpenAISchedulerHandler{gatewayService: gatewayService}
}

type openAISchedulerSettingsDTO struct {
	HealthRankingEnabled        bool    `json:"health_ranking_enabled"`
	PrimaryRatio                float64 `json:"primary_ratio"`
	PrimaryMinCount             int     `json:"primary_min_count"`
	TTFTDegradeMS               int     `json:"ttft_degrade_ms"`
	ErrorRateDegradeThreshold   float64 `json:"error_rate_degrade_threshold"`
	ConsecutiveFailureThreshold int     `json:"consecutive_failure_threshold"`
	RecoverSuccessThreshold     int     `json:"recover_success_threshold"`
	CooldownSeconds             int     `json:"cooldown_seconds"`
	ObserveProbeRatio           float64 `json:"observe_probe_ratio"`
}

type openAISchedulerActionRequest struct {
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	DurationSeconds int    `json:"duration_seconds"`
}

func openAISchedulerSettingsToDTO(settings service.OpenAISchedulerHealthSettings) openAISchedulerSettingsDTO {
	return openAISchedulerSettingsDTO{
		HealthRankingEnabled:        settings.HealthRankingEnabled,
		PrimaryRatio:                settings.PrimaryRatio,
		PrimaryMinCount:             settings.PrimaryMinCount,
		TTFTDegradeMS:               settings.TTFTDegradeMS,
		ErrorRateDegradeThreshold:   settings.ErrorRateDegradeThreshold,
		ConsecutiveFailureThreshold: settings.ConsecutiveFailureThreshold,
		RecoverSuccessThreshold:     settings.RecoverSuccessThreshold,
		CooldownSeconds:             int(settings.Cooldown.Seconds()),
		ObserveProbeRatio:           settings.ObserveProbeRatio,
	}
}

func openAISchedulerSettingsFromDTO(dto openAISchedulerSettingsDTO) service.OpenAISchedulerHealthSettings {
	return service.OpenAISchedulerHealthSettings{
		HealthRankingEnabled:        dto.HealthRankingEnabled,
		PrimaryRatio:                dto.PrimaryRatio,
		PrimaryMinCount:             dto.PrimaryMinCount,
		TTFTDegradeMS:               dto.TTFTDegradeMS,
		ErrorRateDegradeThreshold:   dto.ErrorRateDegradeThreshold,
		ConsecutiveFailureThreshold: dto.ConsecutiveFailureThreshold,
		RecoverSuccessThreshold:     dto.RecoverSuccessThreshold,
		Cooldown:                    time.Duration(dto.CooldownSeconds) * time.Second,
		ObserveProbeRatio:           dto.ObserveProbeRatio,
	}
}

func (h *OpenAISchedulerHandler) GetOverview(c *gin.Context) {
	metrics := h.gatewayService.SnapshotOpenAIAccountSchedulerMetrics()
	settings := h.gatewayService.SnapshotOpenAISchedulerHealthSettings()
	response.Success(c, gin.H{
		"settings": openAISchedulerSettingsToDTO(settings),
		"metrics":  metrics,
	})
}

func (h *OpenAISchedulerHandler) ListAccounts(c *gin.Context) {
	response.Success(c, gin.H{
		"items":     []any{},
		"total":     0,
		"page":      parsePositiveQueryInt(c, "page", 1),
		"page_size": parsePositiveQueryInt(c, "page_size", 20),
	})
}

func (h *OpenAISchedulerHandler) GetAccount(c *gin.Context) {
	accountID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	snapshot, found := h.gatewayService.SnapshotOpenAIAccountHealth(c.Request.Context(), accountID)
	if !found {
		response.NotFound(c, "scheduler health snapshot not found")
		return
	}
	response.Success(c, snapshot)
}

func (h *OpenAISchedulerHandler) ApplyAction(c *gin.Context) {
	accountID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	var req openAISchedulerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil && c.Request.ContentLength > 0 {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	err := h.gatewayService.ApplyOpenAISchedulerHealthAction(accountID, service.OpenAISchedulerHealthAction{
		Action:   req.Action,
		Reason:   req.Reason,
		Duration: time.Duration(req.DurationSeconds) * time.Second,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *OpenAISchedulerHandler) GetSettings(c *gin.Context) {
	response.Success(c, openAISchedulerSettingsToDTO(h.gatewayService.SnapshotOpenAISchedulerHealthSettings()))
}

func (h *OpenAISchedulerHandler) UpdateSettings(c *gin.Context) {
	var req openAISchedulerSettingsDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := h.gatewayService.SaveOpenAISchedulerHealthSettings(c.Request.Context(), openAISchedulerSettingsFromDTO(req)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, openAISchedulerSettingsToDTO(h.gatewayService.SnapshotOpenAISchedulerHealthSettings()))
}

func parsePathInt64(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "Invalid id")
		return 0, false
	}
	return id, true
}

func parsePositiveQueryInt(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
```

If `response.NotFound` does not exist in this repo, use:

```go
response.Error(c, http.StatusNotFound, "scheduler health snapshot not found")
```

- [ ] **Step 4: Wire handler and routes**

Modify `backend/internal/handler/handler.go`:

```go
	OpenAIScheduler        *admin.OpenAISchedulerHandler
```

Modify `backend/internal/handler/wire.go`:

- Add parameter to `ProvideAdminHandlers`:

```go
	openaiSchedulerHandler *admin.OpenAISchedulerHandler,
```

- Add assignment:

```go
		OpenAIScheduler:        openaiSchedulerHandler,
```

- Add provider:

```go
	admin.NewOpenAISchedulerHandler,
```

Modify `backend/internal/server/routes/admin.go`:

- Add call in `RegisterAdminRoutes` after OpenAI OAuth or Ops:

```go
		// OpenAI 调度管理
		registerOpenAISchedulerRoutes(admin, h)
```

- Add function:

```go
func registerOpenAISchedulerRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	scheduler := admin.Group("/openai-scheduler")
	{
		scheduler.GET("/overview", h.Admin.OpenAIScheduler.GetOverview)
		scheduler.GET("/accounts", h.Admin.OpenAIScheduler.ListAccounts)
		scheduler.GET("/accounts/:id", h.Admin.OpenAIScheduler.GetAccount)
		scheduler.POST("/accounts/:id/actions", h.Admin.OpenAIScheduler.ApplyAction)
		scheduler.GET("/settings", h.Admin.OpenAIScheduler.GetSettings)
		scheduler.PUT("/settings", h.Admin.OpenAIScheduler.UpdateSettings)
	}
}
```

- [ ] **Step 5: Run handler tests**

Run:

```bash
go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler' -count=1
```

Expected:

- PASS.

- [ ] **Step 6: Regenerate wire if required**

Run:

```bash
go generate ./internal/...
```

Expected:

- PASS or no-op.
- If the project has no wire generation target for these files, no generated files change.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/handler/admin/openai_scheduler_handler.go backend/internal/handler/admin/openai_scheduler_handler_test.go backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/admin.go
git add backend/internal/server/wire_gen.go backend/internal/cmd/wire_gen.go
git commit -m "feat: 增加OpenAI调度管理接口"
```

If no `wire_gen.go` files changed, omit them from `git add`.

---

## Task 6: Admin API Returns Account Snapshots

**Files:**

- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/handler/admin/openai_scheduler_handler.go`
- Test: `backend/internal/handler/admin/openai_scheduler_handler_test.go`

- [ ] **Step 1: Write failing test for account list shape**

Append to `backend/internal/handler/admin/openai_scheduler_handler_test.go`:

```go
func TestOpenAISchedulerHandler_ListAccounts_ResponseShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	router := gin.New()
	router.GET("/accounts", h.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)
	require.Contains(t, w.Body.String(), `"page"`)
	require.Contains(t, w.Body.String(), `"page_size"`)
	require.Contains(t, w.Body.String(), `"total"`)
}
```

- [ ] **Step 2: Add service list snapshot method**

Add a DTO type in service:

```go
type OpenAISchedulerAccountSnapshot struct {
	AccountID      int64                       `json:"account_id"`
	AccountName    string                      `json:"account_name"`
	Platform       string                      `json:"platform"`
	Type           string                      `json:"type"`
	Status         string                      `json:"status"`
	ManualPriority int                         `json:"manual_priority"`
	Groups         []int64                     `json:"groups"`
	Health         OpenAIAccountHealthSnapshot `json:"health"`
}
```

Add method:

```go
func (s *OpenAIGatewayService) ListOpenAISchedulerAccountSnapshots(ctx context.Context, groupID *int64) ([]OpenAISchedulerAccountSnapshot, error) {
	if s == nil {
		return nil, nil
	}
	accounts, err := s.listSchedulableAccounts(ctx, groupID)
	if err != nil {
		return nil, err
	}
	items := make([]OpenAISchedulerAccountSnapshot, 0, len(accounts))
	for i := range accounts {
		acc := accounts[i]
		if !acc.IsOpenAI() {
			continue
		}
		health, ok := s.SnapshotOpenAIAccountHealth(ctx, acc.ID)
		if !ok {
			health = buildOpenAIAccountHealthSnapshot(acc.ID, openAIAccountHealthRuntime{successEWMA: 1}, s.SnapshotOpenAISchedulerHealthSettings(), time.Now())
		}
		items = append(items, OpenAISchedulerAccountSnapshot{
			AccountID:      acc.ID,
			AccountName:    acc.Name,
			Platform:       acc.Platform,
			Type:           acc.Type,
			Status:         acc.Status,
			ManualPriority: acc.Priority,
			Groups:         acc.GroupIDs,
			Health:         health,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if openAISchedulerTierRank(a.Health.Tier) != openAISchedulerTierRank(b.Health.Tier) {
			return openAISchedulerTierRank(a.Health.Tier) < openAISchedulerTierRank(b.Health.Tier)
		}
		if a.Health.HealthScore != b.Health.HealthScore {
			return a.Health.HealthScore > b.Health.HealthScore
		}
		if a.ManualPriority != b.ManualPriority {
			return a.ManualPriority < b.ManualPriority
		}
		return a.AccountID < b.AccountID
	})
	return items, nil
}
```

- [ ] **Step 3: Update handler list and overview**

In `ListAccounts`, parse optional `group_id`, call `ListOpenAISchedulerAccountSnapshots`, filter by `tier` and `search`, then paginate:

```go
items, err := h.gatewayService.ListOpenAISchedulerAccountSnapshots(c.Request.Context(), groupID)
if err != nil {
	response.InternalError(c, err.Error())
	return
}
```

Use simple in-memory filtering:

```go
tier := c.Query("tier")
search := strings.ToLower(strings.TrimSpace(c.Query("search")))
filtered := make([]service.OpenAISchedulerAccountSnapshot, 0, len(items))
for _, item := range items {
	if tier != "" && item.Health.Tier != tier {
		continue
	}
	if search != "" && !strings.Contains(strings.ToLower(item.AccountName), search) {
		continue
	}
	filtered = append(filtered, item)
}
```

Paginate:

```go
page := parsePositiveQueryInt(c, "page", 1)
pageSize := parsePositiveQueryInt(c, "page_size", 20)
start := (page - 1) * pageSize
if start > len(filtered) {
	start = len(filtered)
}
end := start + pageSize
if end > len(filtered) {
	end = len(filtered)
}
response.Success(c, gin.H{
	"items": filtered[start:end],
	"total": len(filtered),
	"page": page,
	"page_size": pageSize,
})
```

Update overview by counting tiers from `ListOpenAISchedulerAccountSnapshots`.

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler' -count=1
go test ./internal/service -run 'TestOpenAISchedulerHealth|TestOpenAISchedulerBuildSelectionOrder' -count=1
```

Expected:

- PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/handler/admin/openai_scheduler_handler.go backend/internal/handler/admin/openai_scheduler_handler_test.go
git commit -m "feat: 返回OpenAI调度账号快照"
```

---

## Task 7: Frontend API Client And Route

**Files:**

- Create: `frontend/src/api/admin/openaiScheduler.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: Create API client**

Create `frontend/src/api/admin/openaiScheduler.ts`:

```ts
import { apiClient } from '../client'

export type OpenAISchedulerTier = 'primary' | 'standby' | 'observe' | 'degraded'

export interface OpenAISchedulerSettings {
  health_ranking_enabled: boolean
  primary_ratio: number
  primary_min_count: number
  ttft_degrade_ms: number
  error_rate_degrade_threshold: number
  consecutive_failure_threshold: number
  recover_success_threshold: number
  cooldown_seconds: number
  observe_probe_ratio: number
}

export interface OpenAIAccountHealth {
  account_id: number
  health_score: number
  tier: OpenAISchedulerTier
  degrade_reason: string
  cooldown_until?: string | null
  success_rate_ewma: number
  error_rate_ewma: number
  ttft_ewma_ms: number
  consecutive_errors: number
  consecutive_ok: number
  decision_reason: string
}

export interface OpenAISchedulerAccount {
  account_id: number
  account_name: string
  platform: string
  type: string
  status: string
  manual_priority: number
  groups: number[]
  health: OpenAIAccountHealth
}

export interface OpenAISchedulerOverview {
  settings: OpenAISchedulerSettings
  metrics?: Record<string, unknown>
  primary_count?: number
  standby_count?: number
  observe_count?: number
  degraded_count?: number
  average_health_score?: number
  average_ttft_ms?: number
}

export interface ListAccountsParams {
  page?: number
  page_size?: number
  group_id?: number
  tier?: OpenAISchedulerTier | ''
  search?: string
}

export interface ListAccountsResponse {
  items: OpenAISchedulerAccount[]
  total: number
  page: number
  page_size: number
}

export interface SchedulerActionRequest {
  action: 'run_probe' | 'promote_observe' | 'cooldown' | 'clear_cooldown'
  reason?: string
  duration_seconds?: number
}

export async function getOverview(): Promise<OpenAISchedulerOverview> {
  const { data } = await apiClient.get<OpenAISchedulerOverview>('/admin/openai-scheduler/overview')
  return data
}

export async function listAccounts(params: ListAccountsParams = {}, options?: { signal?: AbortSignal }): Promise<ListAccountsResponse> {
  const { data } = await apiClient.get<ListAccountsResponse>('/admin/openai-scheduler/accounts', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function getAccount(id: number): Promise<OpenAISchedulerAccount> {
  const { data } = await apiClient.get<OpenAISchedulerAccount>(`/admin/openai-scheduler/accounts/${id}`)
  return data
}

export async function applyAction(id: number, payload: SchedulerActionRequest): Promise<{ success: boolean }> {
  const { data } = await apiClient.post<{ success: boolean }>(`/admin/openai-scheduler/accounts/${id}/actions`, payload)
  return data
}

export async function getSettings(): Promise<OpenAISchedulerSettings> {
  const { data } = await apiClient.get<OpenAISchedulerSettings>('/admin/openai-scheduler/settings')
  return data
}

export async function updateSettings(payload: OpenAISchedulerSettings): Promise<OpenAISchedulerSettings> {
  const { data } = await apiClient.put<OpenAISchedulerSettings>('/admin/openai-scheduler/settings', payload)
  return data
}

export const openaiSchedulerAPI = {
  getOverview,
  listAccounts,
  getAccount,
  applyAction,
  getSettings,
  updateSettings,
}

export default openaiSchedulerAPI
```

- [ ] **Step 2: Export API**

Modify `frontend/src/api/admin/index.ts`:

```ts
import openaiSchedulerAPI from './openaiScheduler'
```

Add to `adminAPI`:

```ts
  openaiScheduler: openaiSchedulerAPI,
```

Add to named exports:

```ts
  openaiSchedulerAPI,
```

- [ ] **Step 3: Add router entry**

Modify `frontend/src/router/index.ts`, near admin ops/accounts routes:

```ts
  {
    path: '/admin/openai-scheduler',
    name: 'AdminOpenAIScheduler',
    component: () => import('@/views/admin/OpenAISchedulerView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'OpenAI Scheduler',
      titleKey: 'admin.openaiScheduler.title',
      descriptionKey: 'admin.openaiScheduler.description'
    }
  },
```

- [ ] **Step 4: Add i18n keys**

Modify `frontend/src/i18n/locales/zh.ts`, add under `nav`:

```ts
    openaiScheduler: 'OpenAI 调度',
```

Add under `admin`:

```ts
    openaiScheduler: {
      title: 'OpenAI 调度',
      description: '按账号健康分自动分层调度 OpenAI 上游账号',
      refresh: '刷新',
      saveSettings: '保存策略',
      settingsSaved: '策略已保存',
      loadError: '加载 OpenAI 调度数据失败',
      settingsLoadError: '加载调度策略失败',
      actionSuccess: '操作已执行',
      actionFailed: '操作执行失败',
      columns: {
        account: '账号',
        tier: '分层',
        health: '健康分',
        successRate: '成功率',
        ttft: '首包延迟',
        priority: '人工优先级',
        reason: '调度原因',
        actions: '操作'
      },
      tier: {
        primary: '主力',
        standby: '备用',
        observe: '观察',
        degraded: '降级'
      },
      actions: {
        cooldown: '冷却',
        clearCooldown: '解除冷却',
        promoteObserve: '进入观察',
        runProbe: '立即探测'
      }
    },
```

Modify `frontend/src/i18n/locales/en.ts` with equivalent English:

```ts
    openaiScheduler: 'OpenAI Scheduler',
```

```ts
    openaiScheduler: {
      title: 'OpenAI Scheduler',
      description: 'Route OpenAI upstream accounts by runtime health tiers',
      refresh: 'Refresh',
      saveSettings: 'Save Policy',
      settingsSaved: 'Policy saved',
      loadError: 'Failed to load OpenAI scheduler data',
      settingsLoadError: 'Failed to load scheduler policy',
      actionSuccess: 'Action completed',
      actionFailed: 'Action failed',
      columns: {
        account: 'Account',
        tier: 'Tier',
        health: 'Health',
        successRate: 'Success',
        ttft: 'TTFT',
        priority: 'Manual Priority',
        reason: 'Decision Reason',
        actions: 'Actions'
      },
      tier: {
        primary: 'Primary',
        standby: 'Standby',
        observe: 'Observe',
        degraded: 'Degraded'
      },
      actions: {
        cooldown: 'Cooldown',
        clearCooldown: 'Clear Cooldown',
        promoteObserve: 'Observe',
        runProbe: 'Probe Now'
      }
    },
```

- [ ] **Step 5: Run frontend type check or targeted tests**

Run:

```bash
npm --prefix frontend test -- --run frontend/src/router/__tests__/title.spec.ts
```

Expected:

- PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/admin/openaiScheduler.ts frontend/src/api/admin/index.ts frontend/src/router/index.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: 增加OpenAI调度前端入口"
```

---

## Task 8: Frontend Scheduler Page

**Files:**

- Create: `frontend/src/views/admin/OpenAISchedulerView.vue`
- Test: `frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts`

- [ ] **Step 1: Write failing component test**

Create `frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import OpenAISchedulerView from '../OpenAISchedulerView.vue'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    openaiScheduler: {
      getOverview: vi.fn().mockResolvedValue({
        settings: {
          health_ranking_enabled: true,
          primary_ratio: 0.3,
          primary_min_count: 1,
          ttft_degrade_ms: 2500,
          error_rate_degrade_threshold: 0.35,
          consecutive_failure_threshold: 3,
          recover_success_threshold: 5,
          cooldown_seconds: 600,
          observe_probe_ratio: 0,
        },
        primary_count: 1,
        standby_count: 1,
        observe_count: 0,
        degraded_count: 1,
      }),
      listAccounts: vi.fn().mockResolvedValue({
        items: [
          {
            account_id: 1,
            account_name: 'openai-fast',
            platform: 'openai',
            type: 'oauth',
            status: 'active',
            manual_priority: 10,
            groups: [1],
            health: {
              account_id: 1,
              health_score: 98,
              tier: 'primary',
              degrade_reason: '',
              success_rate_ewma: 0.99,
              error_rate_ewma: 0.01,
              ttft_ewma_ms: 820,
              consecutive_errors: 0,
              consecutive_ok: 8,
              decision_reason: 'fast and healthy',
            },
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
      updateSettings: vi.fn(),
      applyAction: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('OpenAISchedulerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders scheduler accounts', async () => {
    const wrapper = mount(OpenAISchedulerView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: {
            props: ['data'],
            template: '<div><div v-for="row in data" :key="row.account_id">{{ row.account_name }} {{ row.health.tier }}</div></div>',
          },
          Pagination: true,
          Toggle: true,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })

    await Promise.resolve()
    await Promise.resolve()

    expect(wrapper.text()).toContain('openai-fast')
    expect(wrapper.text()).toContain('primary')
  })
})
```

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
npm --prefix frontend test -- --run frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts
```

Expected:

- FAIL because `OpenAISchedulerView.vue` does not exist.

- [ ] **Step 3: Implement page**

Create `frontend/src/views/admin/OpenAISchedulerView.vue`:

```vue
<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiScheduler.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.openaiScheduler.description') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn btn-secondary" @click="reload">{{ t('admin.openaiScheduler.refresh') }}</button>
          <button class="btn btn-primary" @click="saveSettings">{{ t('admin.openaiScheduler.saveSettings') }}</button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div v-for="metric in metrics" :key="metric.key" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
        </div>
      </div>

      <div class="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
        <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiScheduler.saveSettings') }}</h2>
            <Toggle :modelValue="settings.health_ranking_enabled" @update:modelValue="settings.health_ranking_enabled = $event" />
          </div>
          <div class="space-y-3">
            <label class="block">
              <span class="input-label">Primary Ratio</span>
              <input v-model.number="settings.primary_ratio" class="input" type="number" min="0" max="1" step="0.05">
            </label>
            <label class="block">
              <span class="input-label">TTFT Degrade MS</span>
              <input v-model.number="settings.ttft_degrade_ms" class="input" type="number" min="1">
            </label>
            <label class="block">
              <span class="input-label">Error Rate Threshold</span>
              <input v-model.number="settings.error_rate_degrade_threshold" class="input" type="number" min="0" max="1" step="0.05">
            </label>
            <label class="block">
              <span class="input-label">Cooldown Seconds</span>
              <input v-model.number="settings.cooldown_seconds" class="input" type="number" min="1">
            </label>
          </div>
        </section>

        <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-wrap items-center gap-2 border-b border-gray-200 p-3 dark:border-dark-700">
            <input v-model="search" class="input max-w-xs" placeholder="Search account" @input="handleSearch">
            <button v-for="tier in tierFilters" :key="tier.value" class="btn btn-sm" :class="tierFilter === tier.value ? 'btn-primary' : 'btn-secondary'" @click="setTier(tier.value)">
              {{ tier.label }}
            </button>
          </div>
          <DataTable :columns="columns" :data="accounts" :loading="loading">
            <template #cell-account_name="{ row }">
              <div>
                <div class="font-medium text-gray-900 dark:text-white">{{ row.account_name }}</div>
                <div class="text-xs text-gray-500">#{{ row.account_id }} · {{ row.type }} · P{{ row.manual_priority }}</div>
              </div>
            </template>
            <template #cell-tier="{ row }">
              <span class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium" :class="tierClass(row.health.tier)">
                {{ t(`admin.openaiScheduler.tier.${row.health.tier}`) }}
              </span>
            </template>
            <template #cell-health="{ row }">{{ row.health.health_score.toFixed(1) }}</template>
            <template #cell-success="{ row }">{{ formatPercent(row.health.success_rate_ewma) }}</template>
            <template #cell-ttft="{ row }">{{ formatLatency(row.health.ttft_ewma_ms) }}</template>
            <template #cell-actions="{ row }">
              <div class="flex justify-end gap-1">
                <button class="btn btn-xs btn-secondary" @click="apply(row.account_id, 'promote_observe')">{{ t('admin.openaiScheduler.actions.promoteObserve') }}</button>
                <button class="btn btn-xs btn-secondary" @click="apply(row.account_id, 'clear_cooldown')">{{ t('admin.openaiScheduler.actions.clearCooldown') }}</button>
              </div>
            </template>
          </DataTable>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { OpenAISchedulerAccount, OpenAISchedulerSettings, OpenAISchedulerTier } from '@/api/admin/openaiScheduler'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<OpenAISchedulerAccount[]>([])
const overview = ref<Record<string, unknown>>({})
const search = ref('')
const tierFilter = ref<OpenAISchedulerTier | ''>('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

const settings = reactive<OpenAISchedulerSettings>({
  health_ranking_enabled: false,
  primary_ratio: 0.3,
  primary_min_count: 1,
  ttft_degrade_ms: 2500,
  error_rate_degrade_threshold: 0.35,
  consecutive_failure_threshold: 3,
  recover_success_threshold: 5,
  cooldown_seconds: 600,
  observe_probe_ratio: 0,
})

const columns = computed<Column[]>(() => [
  { key: 'account_name', label: t('admin.openaiScheduler.columns.account'), sortable: false },
  { key: 'tier', label: t('admin.openaiScheduler.columns.tier'), sortable: false },
  { key: 'health', label: t('admin.openaiScheduler.columns.health'), sortable: false },
  { key: 'success', label: t('admin.openaiScheduler.columns.successRate'), sortable: false },
  { key: 'ttft', label: t('admin.openaiScheduler.columns.ttft'), sortable: false },
  { key: 'actions', label: t('admin.openaiScheduler.columns.actions'), sortable: false },
])

const tierFilters = computed(() => [
  { value: '' as const, label: 'All' },
  { value: 'primary' as const, label: t('admin.openaiScheduler.tier.primary') },
  { value: 'standby' as const, label: t('admin.openaiScheduler.tier.standby') },
  { value: 'observe' as const, label: t('admin.openaiScheduler.tier.observe') },
  { value: 'degraded' as const, label: t('admin.openaiScheduler.tier.degraded') },
])

const metrics = computed(() => [
  { key: 'primary', label: t('admin.openaiScheduler.tier.primary'), value: overview.value.primary_count ?? 0 },
  { key: 'standby', label: t('admin.openaiScheduler.tier.standby'), value: overview.value.standby_count ?? 0 },
  { key: 'observe', label: t('admin.openaiScheduler.tier.observe'), value: overview.value.observe_count ?? 0 },
  { key: 'degraded', label: t('admin.openaiScheduler.tier.degraded'), value: overview.value.degraded_count ?? 0 },
])

function assignSettings(next: OpenAISchedulerSettings) {
  Object.assign(settings, next)
}

async function reload() {
  loading.value = true
  try {
    const [overviewRes, accountsRes] = await Promise.all([
      adminAPI.openaiScheduler.getOverview(),
      adminAPI.openaiScheduler.listAccounts({
        page: 1,
        page_size: 50,
        tier: tierFilter.value,
        search: search.value.trim(),
      }),
    ])
    overview.value = overviewRes
    if (overviewRes.settings) assignSettings(overviewRes.settings)
    accounts.value = accountsRes.items || []
  } catch (err) {
    appStore.showError(t('admin.openaiScheduler.loadError'))
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  try {
    const updated = await adminAPI.openaiScheduler.updateSettings({ ...settings })
    assignSettings(updated)
    appStore.showSuccess(t('admin.openaiScheduler.settingsSaved'))
  } catch (err) {
    appStore.showError(t('admin.openaiScheduler.settingsLoadError'))
  }
}

async function apply(accountId: number, action: 'promote_observe' | 'clear_cooldown') {
  try {
    await adminAPI.openaiScheduler.applyAction(accountId, { action })
    appStore.showSuccess(t('admin.openaiScheduler.actionSuccess'))
    await reload()
  } catch (err) {
    appStore.showError(t('admin.openaiScheduler.actionFailed'))
  }
}

function handleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(reload, 300)
}

function setTier(tier: OpenAISchedulerTier | '') {
  tierFilter.value = tier
  reload()
}

function formatPercent(value: number): string {
  return `${Math.round((value || 0) * 100)}%`
}

function formatLatency(value: number): string {
  if (!value) return '-'
  return `${Math.round(value)}ms`
}

function tierClass(tier: OpenAISchedulerTier): string {
  if (tier === 'primary') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (tier === 'standby') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (tier === 'observe') return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}

onMounted(reload)
</script>
```

- [ ] **Step 4: Run component test**

Run:

```bash
npm --prefix frontend test -- --run frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts
```

Expected:

- PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/admin/OpenAISchedulerView.vue frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts
git commit -m "feat: 新增OpenAI调度管理页面"
```

---

## Task 9: Sidebar Entry And I18n Verification

**Files:**

- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Test: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- Test: `frontend/src/router/__tests__/title.spec.ts`

- [ ] **Step 1: Inspect existing sidebar menu structure**

Run:

```bash
sed -n '1,260p' frontend/src/components/layout/AppSidebar.vue
```

Expected:

- Identify the admin menu item array or template section containing `/admin/accounts`, `/admin/ops`, and `/admin/channels/monitor`.

- [ ] **Step 2: Add sidebar entry**

In `frontend/src/components/layout/AppSidebar.vue`, add an admin navigation item near `AdminAccounts` or `AdminOps`:

```ts
{
  name: 'AdminOpenAIScheduler',
  path: '/admin/openai-scheduler',
  labelKey: 'nav.openaiScheduler',
  icon: 'activity'
}
```

If this file uses a different item shape, mirror the existing `/admin/ops` item and only change route/name/label/icon.

- [ ] **Step 3: Add or update sidebar test**

In `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`, add an assertion matching existing style:

```ts
expect(wrapper.text()).toContain('OpenAI')
```

If tests use route names, assert:

```ts
expect(wrapper.html()).toContain('/admin/openai-scheduler')
```

- [ ] **Step 4: Run sidebar and title tests**

Run:

```bash
npm --prefix frontend test -- --run frontend/src/components/layout/__tests__/AppSidebar.spec.ts frontend/src/router/__tests__/title.spec.ts
```

Expected:

- PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/layout/AppSidebar.vue frontend/src/components/layout/__tests__/AppSidebar.spec.ts
git commit -m "feat: 添加OpenAI调度菜单入口"
```

---

## Task 10: Integration Verification And Cleanup

**Files:**

- Modify only if verification finds issues.

- [ ] **Step 1: Run backend scheduler tests**

Run:

```bash
go test ./internal/service -run 'TestOpenAIScheduler|TestDefaultOpenAIAccountScheduler|TestOpenAIGatewayService_Health' -count=1
```

Expected:

- PASS.

- [ ] **Step 2: Run backend admin handler tests**

Run:

```bash
go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler' -count=1
```

Expected:

- PASS.

- [ ] **Step 3: Run frontend tests**

Run:

```bash
npm --prefix frontend test -- --run frontend/src/views/admin/__tests__/OpenAISchedulerView.spec.ts frontend/src/components/layout/__tests__/AppSidebar.spec.ts frontend/src/router/__tests__/title.spec.ts
```

Expected:

- PASS.

- [ ] **Step 4: Run focused build checks**

Run:

```bash
go test ./internal/service ./internal/handler/admin ./internal/server/routes -count=1
npm --prefix frontend run build
```

Expected:

- PASS.

- [ ] **Step 5: Check Java temporary test rule**

Run:

```bash
git status --short
```

Expected:

- No `*Test.java`, `*Tests.java`, Java test resources, or Java test helpers were created.
- No unstaged implementation changes remain except intended final changes.

- [ ] **Step 6: Final commit if fixes were needed**

If verification required fixes:

```bash
git add <changed-files>
git commit -m "fix: 完善OpenAI调度验证问题"
```

If no fixes were needed, do not create an empty commit.

---

## Self-Review

Spec coverage:

- OpenAI-only scope is covered by service, handler, route, and frontend route names.
- Account-level health score and tiers are covered by Tasks 1-3.
- Primary-first scheduling and standby fallback are covered by Task 2.
- Admin APIs are covered by Tasks 5-6.
- Frontend page and visual direction are covered by Tasks 7-9.
- Rollback switch is partially covered by `HealthRankingEnabled`; implementation uses runtime settings and keeps default false.

Persistence check:

- Health strategy settings are stored through `SettingRepository.SetMultiple` using dedicated `openai_scheduler_*` keys, then synchronized into the runtime scheduler.

Plan quality check:

- No unresolved placeholder instructions are intentionally left in task steps.

Type consistency:

- `OpenAISchedulerHealthSettings`, `OpenAIAccountHealthSnapshot`, `OpenAISchedulerHealthAction`, and `OpenAISchedulerAccountSnapshot` are defined before use by admin handlers and frontend types.
