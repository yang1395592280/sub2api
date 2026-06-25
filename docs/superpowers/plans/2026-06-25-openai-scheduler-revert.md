# OpenAI Scheduler Revert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the custom OpenAI scheduler dashboard, health board, health-tier routing, daily scheduler stats, and the accounts table routing-status column while keeping the built-in account scheduling protections.

**Architecture:** Delete the admin-only scheduler/health surfaces first, then simplify the OpenAI scheduler core so account selection still flows through previous response, sticky session, and load-balance layers without health tiers. Finally remove the accounts-list `stability` contract from backend and frontend.

**Tech Stack:** Go 1.x backend with Gin, Wire-style dependency files, Ent repositories, Vue 3 + TypeScript frontend, Vitest frontend tests.

## Global Constraints

- Default reply language is Simplified Chinese.
- Work on `/Volumes/workspace/中转站/sub2api`, currently `custom-main`.
- Do not remove built-in temporary unschedulable behavior: `temp_unschedulable_until`, retry-based temporary account removal, and rate-limit scheduling exclusion must remain.
- Do not remove OpenAI OAuth, API Key, model mapping, billing, sticky session, group isolation, connection pool, or failover retry behavior.
- Do not add a destructive migration to drop `openai_scheduler_daily_stats`.
- Do not run `git push`, `rebase`, `reset --hard`, or force operations.

---

## File Structure

- Backend admin surface removal:
  - Delete `backend/internal/handler/admin/openai_scheduler_handler.go`
  - Delete `backend/internal/handler/admin/openai_health_handler.go`
  - Delete `backend/internal/handler/admin/openai_scheduler_handler_test.go`
  - Modify `backend/internal/handler/handler.go`
  - Modify `backend/internal/handler/wire.go`
  - Modify `backend/internal/server/routes/admin.go`
  - Modify `backend/cmd/server/wire_gen.go`
- Backend stats and health-board removal:
  - Delete `backend/internal/service/openai_scheduler_stats.go`
  - Delete `backend/internal/service/openai_scheduler_daily_stats_test.go`
  - Delete `backend/internal/repository/openai_scheduler_stats_repo.go`
  - Delete `backend/internal/repository/openai_scheduler_stats_repo_test.go`
  - Delete `backend/internal/service/openai_health_service.go`
  - Delete `backend/internal/service/openai_health_service_test.go`
  - Modify `backend/internal/repository/wire.go`
  - Modify `backend/internal/service/channel_monitor_types.go`
- Backend scheduler-core revert:
  - Modify `backend/internal/service/openai_account_scheduler.go`
  - Delete `backend/internal/service/openai_scheduler_health_test.go`
  - Delete `backend/internal/service/openai_scheduler_tier_selection_test.go`
  - Modify `backend/internal/service/openai_account_scheduler_test.go`
  - Modify any compile-time tests that call `NewOpenAIGatewayService`
- Backend account-list status removal:
  - Modify `backend/internal/handler/admin/account_handler.go`
  - Modify `backend/internal/handler/admin/account_handler_list_test.go`
  - Keep `backend/internal/service/account_stability.go` only if still referenced outside admin accounts; otherwise delete it and its tests.
- Frontend scheduler/health UI removal:
  - Delete `frontend/src/views/admin/OpenAISchedulerView.vue`
  - Delete `frontend/src/views/admin/OpenAIHealthView.vue`
  - Delete `frontend/src/api/admin/openaiScheduler.ts`
  - Delete `frontend/src/api/admin/openaiHealth.ts`
  - Delete `frontend/src/router/__tests__/openai-health-route.spec.ts`
  - Modify `frontend/src/router/index.ts`
  - Modify `frontend/src/api/admin/index.ts`
  - Modify `frontend/src/components/layout/AppSidebar.vue`
  - Modify `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
  - Modify `frontend/src/i18n/locales/zh.ts`
  - Modify `frontend/src/i18n/locales/en.ts`
- Frontend accounts routing-status column removal:
  - Modify `frontend/src/views/admin/AccountsView.vue`
  - Modify `frontend/src/types/index.ts`
  - Modify `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`

---

### Task 1: Remove Backend OpenAI Scheduler And Health Admin Routes

**Files:**
- Delete: `backend/internal/handler/admin/openai_scheduler_handler.go`
- Delete: `backend/internal/handler/admin/openai_health_handler.go`
- Delete: `backend/internal/handler/admin/openai_scheduler_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes: Existing `admin.OpenAIOAuthHandler`, `service.OpenAIGatewayService`, and `service.ChannelMonitorService`.
- Produces: No `AdminHandlers.OpenAIScheduler`, no `AdminHandlers.OpenAIHealth`, no `/api/v1/admin/openai-scheduler/*`, no `/api/v1/admin/openai-health/*`.

- [ ] **Step 1: Run a focused search to capture current backend admin surface**

Run:

```bash
rg -n "OpenAIScheduler|OpenAIHealth|openai-scheduler|openai-health" backend/internal/handler backend/internal/server/routes backend/cmd/server/wire_gen.go
```

Expected: Matches in `handler.go`, `wire.go`, `routes/admin.go`, `wire_gen.go`, and the two admin handler files.

- [ ] **Step 2: Delete admin scheduler and health handler tests and implementations**

Delete these files with `apply_patch` delete hunks:

```text
backend/internal/handler/admin/openai_scheduler_handler.go
backend/internal/handler/admin/openai_health_handler.go
backend/internal/handler/admin/openai_scheduler_handler_test.go
```

- [ ] **Step 3: Remove admin handler fields and provider parameters**

In `backend/internal/handler/handler.go`, remove these fields from `AdminHandlers`:

```go
OpenAIScheduler        *admin.OpenAISchedulerHandler
OpenAIHealth           *admin.OpenAIHealthHandler
```

In `backend/internal/handler/wire.go`, remove these parameters from `ProvideAdminHandlers`:

```go
openaiSchedulerHandler *admin.OpenAISchedulerHandler,
openaiHealthHandler *admin.OpenAIHealthHandler,
```

Remove these assignments:

```go
OpenAIScheduler:        openaiSchedulerHandler,
OpenAIHealth:           openaiHealthHandler,
```

Remove these providers from `ProviderSet`:

```go
admin.NewOpenAISchedulerHandler,
admin.NewOpenAIHealthHandler,
```

- [ ] **Step 4: Remove admin route registration**

In `backend/internal/server/routes/admin.go`, remove these calls:

```go
registerOpenAISchedulerRoutes(admin, h)
registerOpenAIHealthRoutes(admin, h)
```

Delete the entire function declarations named:

```go
func registerOpenAISchedulerRoutes(admin *gin.RouterGroup, h *handler.Handlers)
func registerOpenAIHealthRoutes(admin *gin.RouterGroup, h *handler.Handlers)
```

- [ ] **Step 5: Update generated wire file manually**

In `backend/cmd/server/wire_gen.go`, remove:

```go
openAISchedulerHandler := admin.NewOpenAISchedulerHandler(openAIGatewayService)
openAIHealthHandler := admin.NewOpenAIHealthHandler(channelMonitorService)
```

Remove those variables from the `handler.ProvideAdminHandlers(...)` call. The call should still include `openAIOAuthHandler`, then continue with `geminiOAuthHandler`.

- [ ] **Step 6: Run compile check for handler packages**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler ./internal/server/routes -run 'TestOpenAIScheduler|TestOpenAIHealth|TestNonExistent'
```

Expected: `ok` for packages or `? ... [no test files]`; no compile errors referencing removed handlers or routes.

---

### Task 2: Remove Scheduler Daily Stats And OpenAI Health Board Services

**Files:**
- Delete: `backend/internal/service/openai_scheduler_stats.go`
- Delete: `backend/internal/service/openai_scheduler_daily_stats_test.go`
- Delete: `backend/internal/repository/openai_scheduler_stats_repo.go`
- Delete: `backend/internal/repository/openai_scheduler_stats_repo_test.go`
- Delete: `backend/internal/service/openai_health_service.go`
- Delete: `backend/internal/service/openai_health_service_test.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/internal/service/channel_monitor_types.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes: Existing OpenAI gateway constructor and channel monitor domain types.
- Produces: `OpenAIGatewayService` no longer accepts or stores `OpenAISchedulerStatsRepository`; no daily scheduler stats writes; no OpenAI health-board DTOs.

- [ ] **Step 1: Delete stats and health service files**

Delete these files with `apply_patch` delete hunks:

```text
backend/internal/service/openai_scheduler_stats.go
backend/internal/service/openai_scheduler_daily_stats_test.go
backend/internal/repository/openai_scheduler_stats_repo.go
backend/internal/repository/openai_scheduler_stats_repo_test.go
backend/internal/service/openai_health_service.go
backend/internal/service/openai_health_service_test.go
```

- [ ] **Step 2: Remove repository provider**

In `backend/internal/repository/wire.go`, remove:

```go
NewOpenAISchedulerStatsRepository,
```

- [ ] **Step 3: Simplify OpenAI gateway constructor**

In `backend/internal/service/openai_gateway_service.go`, remove this field from `OpenAIGatewayService`:

```go
openAISchedulerStatsRepo OpenAISchedulerStatsRepository
```

Remove this parameter from `NewOpenAIGatewayService`:

```go
openAISchedulerStatsRepo OpenAISchedulerStatsRepository,
```

Remove this struct assignment:

```go
openAISchedulerStatsRepo: openAISchedulerStatsRepo,
```

- [ ] **Step 4: Remove daily selection recording call**

In `backend/internal/service/openai_account_scheduler.go`, remove this block from the `defer` in `Select`:

```go
if s.service != nil && decision.SelectedAccountID > 0 {
	s.service.recordOpenAISchedulerDailySelection(ctx, req.GroupID, decision.SelectedAccountID, time.Now())
}
```

- [ ] **Step 5: Remove health-board DTOs**

In `backend/internal/service/channel_monitor_types.go`, delete the complete type declarations named:

```go
type OpenAIHealthQuery
type OpenAIHealthOverview
type OpenAIHealthItem
type OpenAIHealthTrendPoint
```

Keep generic channel monitor types intact.

- [ ] **Step 6: Update constructor calls**

In `backend/cmd/server/wire_gen.go`, remove:

```go
openAISchedulerStatsRepository := repository.NewOpenAISchedulerStatsRepository(db)
```

Remove `openAISchedulerStatsRepository` from every call to this constructor:

```go
service.NewOpenAIGatewayService
```

In any tests calling `NewOpenAIGatewayService`, remove the fourth argument that represented scheduler stats.

- [ ] **Step 7: Run focused compile check**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'TestOpenAIScheduler|TestOpenAIHealth|TestNewOpenAIGatewayService|TestNonExistent'
```

Expected: no compile errors for removed stats repository, health DTOs, or constructor arity.

---

### Task 3: Revert OpenAI Scheduler Core To Built-In Routing

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Delete: `backend/internal/service/openai_scheduler_health_test.go`
- Delete: `backend/internal/service/openai_scheduler_tier_selection_test.go`
- Modify: `backend/internal/service/openai_account_scheduler_test.go`
- Modify: `backend/internal/service/account_test_service.go`

**Interfaces:**
- Consumes: `OpenAIAccountScheduler.Select(ctx, OpenAIAccountScheduleRequest)`.
- Produces: Scheduler retains `Select`, `ReportResult`, `ReportSwitch`, and `SnapshotMetrics`; removes health-tier snapshot/settings/action interface.

- [ ] **Step 1: Delete health-tier tests**

Delete these files:

```text
backend/internal/service/openai_scheduler_health_test.go
backend/internal/service/openai_scheduler_tier_selection_test.go
```

- [ ] **Step 2: Shrink `OpenAIAccountScheduler` interface**

In `backend/internal/service/openai_account_scheduler.go`, change:

```go
type OpenAIAccountScheduler interface {
	Select(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ReportResult(accountID int64, success bool, firstTokenMs *int)
	ReportSwitch()
	SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot
	SnapshotAccountHealth(ctx context.Context, accountID int64) (OpenAIAccountHealthSnapshot, bool)
	SnapshotHealthSettings() OpenAISchedulerHealthSettings
	UpdateHealthSettings(settings OpenAISchedulerHealthSettings)
	ApplyHealthAction(accountID int64, action OpenAISchedulerHealthAction) error
}
```

to:

```go
type OpenAIAccountScheduler interface {
	Select(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ReportResult(accountID int64, success bool, firstTokenMs *int)
	ReportSwitch()
	SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot
}
```

- [ ] **Step 3: Remove health-tier constants, structs, and settings helpers**

In `backend/internal/service/openai_account_scheduler.go`, delete:

```go
openAISchedulerHealthRankingEnabledKey
openAISchedulerPrimaryRatioKey
openAISchedulerPrimaryMinCountKey
openAISchedulerTTFTDegradeMSKey
openAISchedulerErrorRateDegradeThresholdKey
openAISchedulerConsecutiveFailureThresholdKey
openAISchedulerRecoverSuccessThresholdKey
openAISchedulerCooldownSecondsKey
openAISchedulerObserveProbeRatioKey
OpenAISchedulerTierPrimary
OpenAISchedulerTierStandby
OpenAISchedulerTierObserve
OpenAISchedulerTierDegraded
OpenAISchedulerDegradeHighLatency
OpenAISchedulerDegradeRateLimited
OpenAISchedulerDegradeUpstream5xx
OpenAISchedulerDegradeTimeout
OpenAISchedulerDegradeRecovering
OpenAISchedulerDegradeManual
openAIAdvancedSchedulerSettingDBTimeout
openAISchedulerHealthSettingKeys
OpenAISchedulerHealthSettings
OpenAISchedulerHealthAction
OpenAIAccountHealthSnapshot
OpenAISchedulerAccountSnapshot
```

Also delete helper functions that only serve those types:

```go
defaultOpenAISchedulerHealthSettings
normalizeOpenAISchedulerHealthSettings
parseOpenAISchedulerHealthSettings
encodeOpenAISchedulerHealthSettings
buildOpenAIAccountHealthSnapshot
SnapshotOpenAISchedulerHealthSettings
SaveOpenAISchedulerHealthSettings
SnapshotOpenAIAccountHealth
ApplyOpenAISchedulerHealthAction
ListOpenAISchedulerAccountSnapshots
ListAllOpenAISchedulerAccountSnapshots
buildOpenAISchedulerAccountSnapshots
```

- [ ] **Step 4: Remove health settings from scheduler struct and constructor**

In `defaultOpenAIAccountScheduler`, remove:

```go
healthSettingsMu sync.RWMutex
healthSettings   OpenAISchedulerHealthSettings
```

In `newDefaultOpenAIAccountScheduler`, remove:

```go
healthSettings: normalizeOpenAISchedulerHealthSettings(defaultOpenAISchedulerHealthSettings()),
```

- [ ] **Step 5: Simplify load-plan candidate construction**

In `buildOpenAIAccountLoadPlan`, replace the candidate setup loop with this shape:

```go
allCandidates := make([]openAIAccountCandidateScore, 0, len(filtered))
for _, account := range filtered {
	loadInfo := loadMap[account.ID]
	if loadInfo == nil {
		loadInfo = &AccountLoadInfo{AccountID: account.ID}
	}
	errorRate, ttft, hasTTFT := 0.0, 0.0, false
	if s.stats != nil {
		errorRate, ttft, hasTTFT = s.stats.snapshot(account.ID)
	}
	allCandidates = append(allCandidates, openAIAccountCandidateScore{
		account:   account,
		loadInfo:  loadInfo,
		errorRate: errorRate,
		ttft:      ttft,
		hasTTFT:   hasTTFT,
	})
}

candidates := append([]openAIAccountCandidateScore(nil), allCandidates...)
```

Remove the loop that excludes:

```go
if candidate.health.Tier == OpenAISchedulerTierDegraded {
	continue
}
```

- [ ] **Step 6: Remove tier-aware selection code**

In `openAIAccountCandidateScore`, remove the `health` field.

In `isAccountHealthRoutable`, either delete the method entirely and remove callers, or replace callers with no-op behavior. In `Select` and `selectBySessionHash`, remove checks that reject sticky accounts based on health routability:

```go
!s.isAccountHealthRoutable(...)
```

In `buildOpenAISelectionOrder`, remove any branch that sorts or buckets by health tier. Keep weighted top-K behavior based on existing score, priority, load, price, and TTFT.

- [ ] **Step 7: Keep sticky escape and price config**

Do not delete these config-backed helpers unless compile proves they only served deleted tiers:

```go
openAIStickyEscapeConfig
openAIPriceBoostSpeedGapMS
openAIPriceBoostMultiplier
```

They are part of built-in scheduler tuning and still referenced by sticky escape / price scoring.

- [ ] **Step 8: Remove account test service health action hook**

In `backend/internal/service/account_test_service.go`, remove the local interface and call around:

```go
ApplyOpenAISchedulerHealthAction(accountID int64, action OpenAISchedulerHealthAction) error
```

Keep the rest of account test recovery behavior intact.

- [ ] **Step 9: Run scheduler tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIAccountScheduler|TestOpenAIGatewayService_SelectAccountWithScheduler|TestOpenAIGatewayService_RecordUsage|TestSticky|TestGatewayGroupIsolation'
```

Expected: pass. If tests fail because they assert removed health tiers, delete or rewrite those assertions to cover built-in routing behavior.

---

### Task 4: Remove Backend Account List Routing Status Contract

**Files:**
- Modify: `backend/internal/handler/admin/account_handler.go`
- Modify: `backend/internal/handler/admin/account_handler_list_test.go`
- Possibly delete: `backend/internal/service/account_stability.go`
- Possibly delete: `backend/internal/service/account_stability_test.go`

**Interfaces:**
- Consumes: Admin account list endpoint.
- Produces: Account list response no longer contains `stability`.

- [ ] **Step 1: Remove response field**

In `backend/internal/handler/admin/account_handler.go`, change:

```go
type AccountWithConcurrency struct {
	*dto.Account
	CurrentConcurrency int                       `json:"current_concurrency"`
	Stability          *service.AccountStability `json:"stability,omitempty"`
	CurrentWindowCost *float64 `json:"current_window_cost,omitempty"`
	ActiveSessions    *int     `json:"active_sessions,omitempty"`
	CurrentRPM        *int     `json:"current_rpm,omitempty"`
}
```

to:

```go
type AccountWithConcurrency struct {
	*dto.Account
	CurrentConcurrency int `json:"current_concurrency"`
	CurrentWindowCost  *float64 `json:"current_window_cost,omitempty"`
	ActiveSessions     *int     `json:"active_sessions,omitempty"`
	CurrentRPM         *int     `json:"current_rpm,omitempty"`
}
```

- [ ] **Step 2: Remove stability helpers**

Delete the complete declarations named from `account_handler.go`:

```go
const accountStabilityWindowDays
type openAIAccountHealthSnapshotProvider
func buildOpenAISchedulerStability
func localizeOpenAISchedulerStabilityReason
func (h *AccountHandler) buildAccountStability
```

If `accountStabilityWindowDays` is no longer referenced, remove it.

- [ ] **Step 3: Remove stability stats query from List**

In `List`, remove:

```go
var stabilityStats map[int64]*service.AccountStabilityStats
```

Remove the block that calls:

```go
h.accountUsageService.GetAccountStabilityStatsBatch(...)
```

Remove this assignment from response construction:

```go
Stability: h.buildAccountStability(c.Request.Context(), acc, stabilityStats[acc.ID]),
```

- [ ] **Step 4: Remove related tests**

In `backend/internal/handler/admin/account_handler_list_test.go`, delete `TestAccountHandlerListUsesOpenAISchedulerHealthSnapshotForStability`.

Run:

```bash
rg -n "AccountStability|stability|buildAccountStability|GetAccountStabilityStatsBatch" backend/internal/handler backend/internal/service
```

If `AccountStability` and `BuildAccountStability` are only left in `backend/internal/service/account_stability.go` and its test, delete both files. If other features still reference them, keep them.

- [ ] **Step 5: Run admin account tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestAccountHandler|TestAdminService'
```

Expected: pass and no response assertions for `stability`.

---

### Task 5: Remove Frontend Scheduler And Health Pages

**Files:**
- Delete: `frontend/src/views/admin/OpenAISchedulerView.vue`
- Delete: `frontend/src/views/admin/OpenAIHealthView.vue`
- Delete: `frontend/src/api/admin/openaiScheduler.ts`
- Delete: `frontend/src/api/admin/openaiHealth.ts`
- Delete: `frontend/src/router/__tests__/openai-health-route.spec.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes: Admin layout/navigation and admin API export object.
- Produces: No `/admin/openai-scheduler` or `/admin/openai-health` routes, imports, nav entries, or i18n keys.

- [ ] **Step 1: Delete page and API files**

Delete:

```text
frontend/src/views/admin/OpenAISchedulerView.vue
frontend/src/views/admin/OpenAIHealthView.vue
frontend/src/api/admin/openaiScheduler.ts
frontend/src/api/admin/openaiHealth.ts
frontend/src/router/__tests__/openai-health-route.spec.ts
```

- [ ] **Step 2: Remove router entries**

In `frontend/src/router/index.ts`, delete the route objects with:

```ts
path: '/admin/openai-scheduler'
path: '/admin/openai-health'
```

- [ ] **Step 3: Remove API exports**

In `frontend/src/api/admin/index.ts`, remove:

```ts
import openaiSchedulerAPI from './openaiScheduler'
import openaiHealthAPI from './openaiHealth'
openaiScheduler: openaiSchedulerAPI,
openaiHealth: openaiHealthAPI,
openaiSchedulerAPI,
openaiHealthAPI,
```

- [ ] **Step 4: Remove sidebar entries**

In `frontend/src/components/layout/AppSidebar.vue`, delete the two entries:

```ts
{ path: '/admin/openai-scheduler', label: t('nav.openaiScheduler'), icon: SignalIcon },
{ path: '/admin/openai-health', label: t('nav.openaiHealth'), icon: ChartIcon },
```

If `SignalIcon` or `ChartIcon` imports become unused, remove the unused imports.

- [ ] **Step 5: Remove sidebar test assertions**

In `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`, remove expectations for:

```ts
'/admin/openai-scheduler'
'nav.openaiScheduler'
'/admin/openai-health'
'nav.openaiHealth'
```

- [ ] **Step 6: Remove i18n keys**

In `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts`, remove:

```ts
nav.openaiScheduler
nav.openaiHealth
admin.openaiScheduler
admin.openaiHealth
```

Keep unrelated `OpenAI` OAuth, quota, account, and monitor translations.

- [ ] **Step 7: Run frontend route/sidebar tests**

Run:

```bash
cd frontend && npm test -- --run src/components/layout/__tests__/AppSidebar.spec.ts
```

Expected: pass. If this repo uses `pnpm` instead of `npm`, run the equivalent existing test script from `frontend/package.json`.

---

### Task 6: Remove Frontend Accounts Routing-Status Column And Type

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes: Admin account list response without `stability`.
- Produces: Accounts table has no routing-status column and does not read `row.stability`.

- [ ] **Step 1: Remove account type field**

In `frontend/src/types/index.ts`, remove this field from the account type:

```ts
stability?: AccountStability
```

Delete the complete interface declaration named:

```ts
export interface AccountStability
```

- [ ] **Step 2: Remove AccountsView table slot and column**

In `frontend/src/views/admin/AccountsView.vue`, remove both complete template blocks whose names are:

```vue
#header-stability
#cell-stability
```

Remove the column entry:

```ts
{ key: 'stability', label: t('admin.accounts.columns.stability'), sortable: false },
```

- [ ] **Step 3: Remove helper functions**

In `AccountsView.vue`, delete the complete function declarations named:

```ts
stabilityBadgeClass
stabilityDotClass
stabilityTooltip
stabilityReason
```

Remove any imports only used by the deleted header tooltip or status rendering.

- [ ] **Step 4: Remove partial update preservation**

In `AccountsView.vue`, change any merge like:

```ts
stability: updatedAccount.stability ?? oldAccount.stability
```

to omit `stability` entirely.

- [ ] **Step 5: Remove i18n keys**

In `zh.ts` and `en.ts`, remove account keys:

```ts
admin.accounts.columns.stability
admin.accounts.stability
admin.accounts.stabilityHint
```

Keep unrelated status translations.

- [ ] **Step 6: Update account tests**

In `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`, remove:

```ts
<slot name="cell-stability" :row="row" />
```

Delete or rewrite tests that assert:

```ts
account.stability
stability.label
stability is preserved
```

Replacement assertion for partial account payload should check another retained field, for example account name, status, or upstream balance, not `stability`.

- [ ] **Step 7: Run accounts view tests**

Run:

```bash
cd frontend && npm test -- --run src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts
```

Expected: pass. If the package manager is `pnpm`, use the existing script from `frontend/package.json`.

---

### Task 7: Final Cleanup, Formatting, And Verification

**Files:**
- All files touched by Tasks 1-6.
- Possibly update `backend/cmd/server/wire_gen.go` formatting.

**Interfaces:**
- Produces: No custom OpenAI scheduler/health UI or API; built-in scheduling compiles and tests pass.

- [ ] **Step 1: Run global residual searches**

Run:

```bash
rg -n "openaiScheduler|openaiHealth|OpenAIScheduler|OpenAIHealth|openai-scheduler|openai-health|OpenAISchedulerStats|openai_scheduler_daily_stats" backend frontend
```

Expected: no matches except allowed configuration names such as `Gateway.OpenAIScheduler` for sticky escape / price scoring, and the preserved migration file `backend/migrations/151_openai_scheduler_daily_stats.sql`.

Run:

```bash
rg -n "stability|AccountStability|Routing Status|调度状态" backend/internal/handler frontend/src/views/admin frontend/src/types frontend/src/i18n/locales
```

Expected: no account-list routing-status matches. Remaining unrelated system stability text outside account management is acceptable.

- [ ] **Step 2: Format Go code**

Run:

```bash
gofmt -w backend/internal/handler/handler.go backend/internal/handler/wire.go backend/internal/server/routes/admin.go backend/cmd/server/wire_gen.go backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_gateway_service.go backend/internal/handler/admin/account_handler.go backend/internal/repository/wire.go
```

Expected: no output.

- [ ] **Step 3: Run backend focused tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin ./internal/handler ./internal/server/routes ./internal/repository -run 'TestOpenAI|TestAccountHandler|TestGatewayGroupIsolation|TestSticky|TestNonExistent'
```

Expected: pass or no test files; no compile failures.

- [ ] **Step 4: Run frontend focused tests**

Inspect package scripts:

```bash
cd frontend && npm pkg get scripts
```

Run the available test command for:

```text
src/components/layout/__tests__/AppSidebar.spec.ts
src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts
```

Expected: both pass. If no focused test script exists, run the closest existing `test` or `test:unit` script and report the exact command.

- [ ] **Step 5: Check git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only files from this plan changed; no unrelated files.

- [ ] **Step 6: Commit implementation**

After tests pass, commit:

```bash
git add backend frontend docs/superpowers/plans/2026-06-25-openai-scheduler-revert.md
git commit -m "refactor(openai): restore built-in scheduler"
```

Expected: commit succeeds on `custom-main`.

---

## Self-Review

- Spec coverage: Tasks 1 and 5 remove admin scheduler/health surfaces; Tasks 2 and 3 remove stats and health-tier routing; Tasks 4 and 6 remove account routing-status backend/frontend contract; Task 7 verifies residuals.
- Placeholder scan: no TBD/TODO/fill-in placeholders remain; each task has exact files, exact code targets, and commands.
- Type consistency: removed Go types are only referenced before deletion tasks; frontend `AccountStability` removal is paired with account table removal.
