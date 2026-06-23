# OpenAI Routing Priority Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an explainable OpenAI routing priority status for the account management page, then add scheduler strategy controls that make cost, speed, Top-K, and observe probing behavior explicit.

**Architecture:** Phase 1 adds read-only routing explanation APIs and UI, without changing production routing decisions. Phase 2 extends scheduler configuration and scoring behavior behind explicit settings, with tests proving that existing behavior remains the default.

**Tech Stack:** Go backend with Gin handlers and service tests; Vue 3 + TypeScript frontend; Vitest for frontend tests; existing OpenAI scheduler, account, concurrency, and settings services.

## Global Constraints

- Default response language for user-facing summaries is Simplified Chinese.
- Phase 1 must not change real OpenAI account selection behavior.
- Account list payloads must stay lightweight; full candidate detail is loaded on demand.
- Routing status must distinguish persistent account state, advanced scheduler health, runtime block, and UI countdown state.
- Reason codes must be stable strings and localized in frontend.
- Strategy changes in Phase 2 must be opt-in and covered by tests.

---

## File Structure

Backend service:

- Create `backend/internal/service/openai_routing_explain.go`: routing explanation types, reason codes, score breakdown, and `OpenAIGatewayService` methods.
- Create `backend/internal/service/openai_routing_explain_test.go`: unit tests for ranking, reason codes, score breakdown, and strategy behavior.
- Modify `backend/internal/service/openai_account_scheduler.go`: expose reusable scoring helpers and add Phase 2 strategy settings.
- Modify `backend/internal/config/config.go`: add scheduler strategy defaults and validation for Phase 2.

Backend handler and routes:

- Modify `backend/internal/handler/admin/openai_scheduler_handler.go`: add `GetRoutingRanking` and `GetRoutingExplain`.
- Modify `backend/internal/handler/admin/openai_scheduler_handler_test.go`: handler response-shape tests.
- Modify `backend/internal/server/routes/admin.go`: register new endpoints.

Frontend API and types:

- Modify `frontend/src/api/admin/openaiScheduler.ts`: add routing explanation types and API functions.
- Modify `frontend/src/types/index.ts`: add optional `routing_priority` summary to `Account`.

Frontend UI:

- Create `frontend/src/components/account/RoutingPriorityBadge.vue`: compact account-list badge.
- Create `frontend/src/components/account/OpenAIRoutingExplainModal.vue`: full explanation modal.
- Modify `frontend/src/components/account/AccountStatusIndicator.vue`: local countdown tick for persisted cooldown state.
- Modify `frontend/src/views/admin/AccountsView.vue`: add routing priority column, load summaries, open modal.
- Modify `frontend/src/i18n/locales/zh.ts` and `frontend/src/i18n/locales/en.ts`: reason labels and UI strings.

Frontend tests:

- Create `frontend/src/components/account/__tests__/RoutingPriorityBadge.spec.ts`.
- Create `frontend/src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts`.
- Modify `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`.
- Modify `frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts` or add `frontend/src/views/admin/__tests__/AccountsView.routingPriority.spec.ts`.

---

### Task 1: Backend Routing Explanation Model

**Files:**
- Create: `backend/internal/service/openai_routing_explain.go`
- Create: `backend/internal/service/openai_routing_explain_test.go`
- Modify: `backend/internal/service/openai_account_scheduler.go`

**Interfaces:**
- Consumes: `OpenAIGatewayService.listSchedulableAccounts(ctx, groupID)`, `OpenAIAccountHealthSnapshot`, `AccountLoadInfo`, existing `Account` helpers.
- Produces:
  - `type OpenAIRoutingReasonCode string`
  - `type OpenAIRoutingExplainParams struct`
  - `func (s *OpenAIGatewayService) ExplainOpenAIRouting(ctx context.Context, params OpenAIRoutingExplainParams) (*OpenAIRoutingExplainResponse, error)`
  - `func (s *OpenAIGatewayService) ExplainOpenAIRoutingForAccount(ctx context.Context, accountID int64, params OpenAIRoutingExplainParams) (*OpenAIRoutingAccountExplain, error)`

- [ ] **Step 1: Write failing service tests**

Add these tests to `backend/internal/service/openai_routing_explain_test.go`:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRoutingExplainRanksCandidatesAndExplainsScore(t *testing.T) {
	cheap := 0.05
	expensive := 0.20
	groupID := int64(9001)
	accounts := []Account{
		{ID: 1, Name: "cheap-fast", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, ChannelPrice: &cheap, GroupIDs: []int64{groupID}},
		{ID: 2, Name: "expensive-fast", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, ChannelPrice: &expensive, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: accounts},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}
	scheduler := svc.getOpenAIAccountScheduler(context.Background()).(*defaultOpenAIAccountScheduler)
	scheduler.ReportResult(1, true, intPtrForTest(420))
	scheduler.ReportResult(2, true, intPtrForTest(430))

	got, err := svc.ExplainOpenAIRouting(context.Background(), OpenAIRoutingExplainParams{
		GroupID: &groupID,
		Model:   "gpt-5.1",
	})

	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, int64(1), got.Items[0].AccountID)
	require.Equal(t, 1, got.Items[0].Rank)
	require.True(t, got.Items[0].IsSchedulableNow)
	require.Greater(t, got.Items[0].Score.Total, got.Items[1].Score.Total)
	require.Greater(t, got.Items[0].Score.Price, got.Items[1].Score.Price)
	require.Equal(t, "成本优", got.Items[0].SummaryReasons[0])
}

func TestOpenAIRoutingExplainReportsPersistentBlockReasons(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Minute)
	groupID := int64(9002)
	accounts := []Account{
		{ID: 3, Name: "blocked-429", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Priority: 1, Concurrency: 5, RateLimitResetAt: &resetAt, GroupIDs: []int64{groupID}},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: accounts},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}

	got, err := svc.ExplainOpenAIRouting(context.Background(), OpenAIRoutingExplainParams{GroupID: &groupID})

	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.False(t, got.Items[0].IsSchedulableNow)
	require.Contains(t, got.Items[0].BlockReasons, OpenAIRoutingReasonRateLimited)
	require.Equal(t, "跳过", got.Items[0].StatusLabel)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIRoutingExplain' -count=1
```

Expected: FAIL with undefined `OpenAIRoutingExplainParams` and undefined `ExplainOpenAIRouting`.

- [ ] **Step 3: Implement routing explanation types**

Create `backend/internal/service/openai_routing_explain.go` with these public types:

```go
package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

type OpenAIRoutingReasonCode string

const (
	OpenAIRoutingReasonStatusError         OpenAIRoutingReasonCode = "status_error"
	OpenAIRoutingReasonStatusInactive      OpenAIRoutingReasonCode = "status_inactive"
	OpenAIRoutingReasonManualUnschedulable OpenAIRoutingReasonCode = "manual_unschedulable"
	OpenAIRoutingReasonRateLimited         OpenAIRoutingReasonCode = "rate_limited"
	OpenAIRoutingReasonOverloaded          OpenAIRoutingReasonCode = "overloaded"
	OpenAIRoutingReasonTempUnschedulable   OpenAIRoutingReasonCode = "temp_unschedulable"
	OpenAIRoutingReasonRuntimeBlocked      OpenAIRoutingReasonCode = "runtime_blocked"
	OpenAIRoutingReasonHealthDegraded      OpenAIRoutingReasonCode = "health_degraded"
	OpenAIRoutingReasonModelUnsupported    OpenAIRoutingReasonCode = "model_unsupported"
	OpenAIRoutingReasonCapabilityUnsupported OpenAIRoutingReasonCode = "capability_unsupported"
	OpenAIRoutingReasonTransportUnsupported  OpenAIRoutingReasonCode = "transport_unsupported"
	OpenAIRoutingReasonGroupMismatch         OpenAIRoutingReasonCode = "group_mismatch"
	OpenAIRoutingReasonPrivacyNotSet         OpenAIRoutingReasonCode = "privacy_not_set"
	OpenAIRoutingReasonQuotaAutoPaused       OpenAIRoutingReasonCode = "quota_auto_paused"
	OpenAIRoutingReasonConcurrencyFull       OpenAIRoutingReasonCode = "concurrency_full"
	OpenAIRoutingReasonChannelRestricted     OpenAIRoutingReasonCode = "channel_restricted"
	OpenAIRoutingReasonCompactUnsupported    OpenAIRoutingReasonCode = "compact_unsupported"
)

type OpenAIRoutingExplainParams struct {
	GroupID            *int64
	Model              string
	Platform           string
	RequiredCapability OpenAIEndpointCapability
	RequiredTransport  OpenAIUpstreamTransport
	RequireCompact     bool
}

type OpenAIRoutingScoreBreakdown struct {
	Total     float64 `json:"total"`
	Priority  float64 `json:"priority"`
	Load      float64 `json:"load"`
	Queue     float64 `json:"queue"`
	ErrorRate float64 `json:"error_rate"`
	TTFT      float64 `json:"ttft"`
	Price     float64 `json:"price"`
	Health    float64 `json:"health"`
}

type OpenAIRoutingSummary struct {
	AccountID        int64                     `json:"account_id"`
	AccountName      string                    `json:"account_name"`
	Rank             int                       `json:"rank,omitempty"`
	Tier             string                    `json:"tier"`
	Score            OpenAIRoutingScoreBreakdown `json:"score"`
	StatusLabel      string                    `json:"status_label"`
	SummaryReason    string                    `json:"summary_reason"`
	SummaryReasons   []string                  `json:"summary_reasons"`
	IsSchedulableNow bool                      `json:"is_schedulable_now"`
	BlockReasons     []OpenAIRoutingReasonCode `json:"block_reasons,omitempty"`
	SnapshotAt        time.Time                 `json:"snapshot_at"`
}

type OpenAIRoutingExplainResponse struct {
	Items      []OpenAIRoutingSummary `json:"items"`
	Source     string                 `json:"source"`
	SnapshotAt time.Time              `json:"snapshot_at"`
}

type OpenAIRoutingAccountExplain struct {
	Account OpenAIRoutingSummary   `json:"account"`
	Top     []OpenAIRoutingSummary `json:"top"`
	Notes   []string               `json:"notes"`
}
```

- [ ] **Step 4: Implement explanation methods**

Append these methods in `backend/internal/service/openai_routing_explain.go`:

```go
func (s *OpenAIGatewayService) ExplainOpenAIRouting(ctx context.Context, params OpenAIRoutingExplainParams) (*OpenAIRoutingExplainResponse, error) {
	if strings.TrimSpace(params.Platform) == "" {
		params.Platform = PlatformOpenAI
	}
	if params.RequiredTransport == "" {
		params.RequiredTransport = OpenAIUpstreamTransportAny
	}
	now := time.Now()
	if s == nil || (s.accountRepo == nil && s.schedulerSnapshot == nil) {
		return &OpenAIRoutingExplainResponse{
			Items:      []OpenAIRoutingSummary{},
			Source:     "empty",
			SnapshotAt: now,
		}, nil
	}

	accounts, err := s.listSchedulableAccounts(ctx, params.GroupID)
	if err != nil {
		return nil, err
	}
	summaries := make([]OpenAIRoutingSummary, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if acc.Platform != params.Platform {
			continue
		}
		summaries = append(summaries, s.explainOpenAIRoutingAccount(ctx, acc, params, now))
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		a, b := summaries[i], summaries[j]
		if a.IsSchedulableNow != b.IsSchedulableNow {
			return a.IsSchedulableNow
		}
		if a.Score.Total != b.Score.Total {
			return a.Score.Total > b.Score.Total
		}
		return a.AccountID < b.AccountID
	})
	rank := 1
	for i := range summaries {
		if summaries[i].IsSchedulableNow {
			summaries[i].Rank = rank
			rank++
		}
	}

	return &OpenAIRoutingExplainResponse{
		Items:      summaries,
		Source:     "scheduler_snapshot",
		SnapshotAt: now,
	}, nil
}

func (s *OpenAIGatewayService) ExplainOpenAIRoutingForAccount(ctx context.Context, accountID int64, params OpenAIRoutingExplainParams) (*OpenAIRoutingAccountExplain, error) {
	ranking, err := s.ExplainOpenAIRouting(ctx, params)
	if err != nil {
		return nil, err
	}
	var selected *OpenAIRoutingSummary
	for i := range ranking.Items {
		if ranking.Items[i].AccountID == accountID {
			selected = &ranking.Items[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrAccountNotFound
	}
	top := ranking.Items
	if len(top) > 10 {
		top = top[:10]
	}
	return &OpenAIRoutingAccountExplain{
		Account: *selected,
		Top:     top,
		Notes: []string{
			"真实请求仍会先检查 previous_response_id 和 session_hash 粘性。",
			"Top-K 加权模式下候选排名第一不代表每次必选。",
		},
	}, nil
}
```

- [ ] **Step 5: Implement account reason and score helper**

Append the helper methods in the same file:

```go
func (s *OpenAIGatewayService) explainOpenAIRoutingAccount(ctx context.Context, account *Account, params OpenAIRoutingExplainParams, now time.Time) OpenAIRoutingSummary {
	reasons := s.openAIRoutingBlockReasons(ctx, account, params, now)
	health, ok := s.SnapshotOpenAIAccountHealth(ctx, account.ID)
	if !ok {
		health = buildOpenAIAccountHealthSnapshot(account.ID, openAIAccountHealthRuntime{successEWMA: 1}, defaultOpenAISchedulerHealthSettings(), now)
	}
	score := s.openAIRoutingScore(account, health)
	statusLabel := "候选"
	if len(reasons) > 0 {
		statusLabel = "跳过"
	} else if health.Tier == OpenAISchedulerTierDegraded {
		statusLabel = "隔离"
		reasons = append(reasons, OpenAIRoutingReasonHealthDegraded)
	}
	summaryReasons := openAIRoutingSummaryReasons(account, score, health, reasons)
	summaryReason := ""
	if len(summaryReasons) > 0 {
		summaryReason = summaryReasons[0]
	}
	return OpenAIRoutingSummary{
		AccountID:        account.ID,
		AccountName:      account.Name,
		Tier:             health.Tier,
		Score:            score,
		StatusLabel:      statusLabel,
		SummaryReason:    summaryReason,
		SummaryReasons:   summaryReasons,
		IsSchedulableNow: len(reasons) == 0 && health.Tier != OpenAISchedulerTierDegraded,
		BlockReasons:     reasons,
		SnapshotAt:        now,
	}
}

func (s *OpenAIGatewayService) openAIRoutingBlockReasons(ctx context.Context, account *Account, params OpenAIRoutingExplainParams, now time.Time) []OpenAIRoutingReasonCode {
	reasons := make([]OpenAIRoutingReasonCode, 0, 4)
	if account == nil {
		return []OpenAIRoutingReasonCode{OpenAIRoutingReasonStatusInactive}
	}
	if account.Status == StatusError {
		reasons = append(reasons, OpenAIRoutingReasonStatusError)
	}
	if account.Status != StatusActive && account.Status != StatusError {
		reasons = append(reasons, OpenAIRoutingReasonStatusInactive)
	}
	if !account.Schedulable {
		reasons = append(reasons, OpenAIRoutingReasonManualUnschedulable)
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonRateLimited)
	}
	if account.OverloadUntil != nil && account.OverloadUntil.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonOverloaded)
	}
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(now) {
		reasons = append(reasons, OpenAIRoutingReasonTempUnschedulable)
	}
	if s != nil && s.isOpenAIAccountRuntimeBlocked(account) {
		reasons = append(reasons, OpenAIRoutingReasonRuntimeBlocked)
	}
	if params.Model != "" && !account.IsModelSupported(params.Model) {
		reasons = append(reasons, OpenAIRoutingReasonModelUnsupported)
	}
	if params.RequiredCapability != "" && !account.SupportsOpenAIEndpointCapability(params.RequiredCapability) {
		reasons = append(reasons, OpenAIRoutingReasonCapabilityUnsupported)
	}
	if params.RequiredTransport != "" && params.RequiredTransport != OpenAIUpstreamTransportAny && !s.isOpenAIAccountTransportCompatible(account, params.RequiredTransport) {
		reasons = append(reasons, OpenAIRoutingReasonTransportUnsupported)
	}
	if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
		reasons = append(reasons, OpenAIRoutingReasonQuotaAutoPaused)
	}
	if params.RequireCompact && openAICompactSupportTier(account) == 0 {
		reasons = append(reasons, OpenAIRoutingReasonCompactUnsupported)
	}
	return reasons
}

func (s *OpenAIGatewayService) openAIRoutingScore(account *Account, health OpenAIAccountHealthSnapshot) OpenAIRoutingScoreBreakdown {
	priority := 1 / (1 + float64(maxInt(account.Priority, 0)))
	price := 0.5
	if account.ChannelPrice != nil && *account.ChannelPrice > 0 {
		price = 1 / (1 + *account.ChannelPrice)
	}
	errorRate := 1 - clamp01(health.ErrorRateEWMA)
	ttft := 0.5
	if health.TTFTEWMAMS > 0 {
		ttft = 1 / (1 + health.TTFTEWMAMS/1000)
	}
	healthScore := clamp01(health.HealthScore / 100)
	total := priority + price + errorRate + ttft + healthScore
	return OpenAIRoutingScoreBreakdown{
		Total:     math.Round(total*1000) / 1000,
		Priority:  math.Round(priority*1000) / 1000,
		Load:      0.5,
		Queue:     0.5,
		ErrorRate: math.Round(errorRate*1000) / 1000,
		TTFT:      math.Round(ttft*1000) / 1000,
		Price:     math.Round(price*1000) / 1000,
		Health:    math.Round(healthScore*1000) / 1000,
	}
}

func openAIRoutingSummaryReasons(account *Account, score OpenAIRoutingScoreBreakdown, health OpenAIAccountHealthSnapshot, reasons []OpenAIRoutingReasonCode) []string {
	if len(reasons) > 0 {
		return []string{string(reasons[0])}
	}
	out := make([]string, 0, 3)
	if score.Price >= 0.9 {
		out = append(out, "成本优")
	}
	if health.TTFTEWMAMS > 0 && health.TTFTEWMAMS <= 1000 {
		out = append(out, "低延迟")
	}
	if account.Priority <= 5 {
		out = append(out, "高优先级")
	}
	if len(out) == 0 {
		out = append(out, "可调度")
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 6: Run service tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAIRoutingExplain' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 1**

```bash
git add backend/internal/service/openai_routing_explain.go backend/internal/service/openai_routing_explain_test.go backend/internal/service/openai_account_scheduler.go
git commit -m "feat: add OpenAI routing explanation service"
```

---

### Task 2: Backend Admin API Endpoints

**Files:**
- Modify: `backend/internal/handler/admin/openai_scheduler_handler.go`
- Modify: `backend/internal/handler/admin/openai_scheduler_handler_test.go`
- Modify: `backend/internal/server/routes/admin.go`

**Interfaces:**
- Consumes: `ExplainOpenAIRouting` and `ExplainOpenAIRoutingForAccount`.
- Produces:
  - `GET /api/v1/admin/openai-scheduler/ranking`
  - `GET /api/v1/admin/openai-scheduler/accounts/:id/routing-explain`

- [ ] **Step 1: Write failing handler tests**

Append to `backend/internal/handler/admin/openai_scheduler_handler_test.go`:

```go
func TestOpenAISchedulerHandler_RoutingRanking_ResponseShape(t *testing.T) {
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	r := gin.New()
	r.GET("/ranking", h.GetRoutingRanking)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ranking?model=gpt-5.1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"items"`)
	require.Contains(t, w.Body.String(), `"snapshot_at"`)
}

func TestOpenAISchedulerHandler_RoutingExplain_InvalidID(t *testing.T) {
	h := NewOpenAISchedulerHandler(&service.OpenAIGatewayService{})
	r := gin.New()
	r.GET("/accounts/:id/routing-explain", h.GetRoutingExplain)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/accounts/bad/routing-explain", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler_Routing' -count=1
```

Expected: FAIL with undefined `GetRoutingRanking` and `GetRoutingExplain`.

- [ ] **Step 3: Add handler methods**

Append to `backend/internal/handler/admin/openai_scheduler_handler.go`:

```go
func (h *OpenAISchedulerHandler) GetRoutingRanking(c *gin.Context) {
	groupID, ok := parseOptionalQueryInt64(c, "group_id")
	if !ok {
		return
	}
	params := service.OpenAIRoutingExplainParams{
		GroupID:  groupID,
		Model:    strings.TrimSpace(c.Query("model")),
		Platform: strings.TrimSpace(c.DefaultQuery("platform", service.PlatformOpenAI)),
	}
	result, err := h.gatewayService.ExplainOpenAIRouting(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *OpenAISchedulerHandler) GetRoutingExplain(c *gin.Context) {
	accountID, ok := parsePathInt64(c, "id")
	if !ok {
		return
	}
	groupID, ok := parseOptionalQueryInt64(c, "group_id")
	if !ok {
		return
	}
	params := service.OpenAIRoutingExplainParams{
		GroupID:  groupID,
		Model:    strings.TrimSpace(c.Query("model")),
		Platform: strings.TrimSpace(c.DefaultQuery("platform", service.PlatformOpenAI)),
	}
	result, err := h.gatewayService.ExplainOpenAIRoutingForAccount(c.Request.Context(), accountID, params)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			response.NotFound(c, "routing explanation not found")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}
```

Also add `errors` to the import list if it is not already present.

- [ ] **Step 4: Register routes**

Modify `registerOpenAISchedulerRoutes` in `backend/internal/server/routes/admin.go`:

```go
scheduler.GET("/ranking", h.Admin.OpenAIScheduler.GetRoutingRanking)
scheduler.GET("/accounts/:id/routing-explain", h.Admin.OpenAIScheduler.GetRoutingExplain)
```

Place `/ranking` before `/accounts/:id`, and place `/accounts/:id/routing-explain` before `/accounts/:id`; Gin matches in registration order, so the generic `/accounts/:id` route must not be registered first.

- [ ] **Step 5: Run handler tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestOpenAISchedulerHandler_Routing|TestOpenAISchedulerHandler_GetSettings_Defaults' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```bash
git add backend/internal/handler/admin/openai_scheduler_handler.go backend/internal/handler/admin/openai_scheduler_handler_test.go backend/internal/server/routes/admin.go
git commit -m "feat: expose OpenAI routing explanation APIs"
```

---

### Task 3: Frontend API Types

**Files:**
- Modify: `frontend/src/api/admin/openaiScheduler.ts`
- Modify: `frontend/src/types/index.ts`

**Interfaces:**
- Consumes: backend JSON from Task 2.
- Produces:
  - `getRoutingRanking(params)`
  - `getRoutingExplain(id, params)`
  - `OpenAIRoutingSummary`
  - `OpenAIRoutingAccountExplain`

- [ ] **Step 1: Add API types**

Append to `frontend/src/api/admin/openaiScheduler.ts` after `OpenAISchedulerOverview`:

```ts
export type OpenAIRoutingReasonCode =
  | 'status_error'
  | 'status_inactive'
  | 'manual_unschedulable'
  | 'rate_limited'
  | 'overloaded'
  | 'temp_unschedulable'
  | 'runtime_blocked'
  | 'health_degraded'
  | 'model_unsupported'
  | 'capability_unsupported'
  | 'transport_unsupported'
  | 'group_mismatch'
  | 'privacy_not_set'
  | 'quota_auto_paused'
  | 'concurrency_full'
  | 'channel_restricted'
  | 'compact_unsupported'

export interface OpenAIRoutingScoreBreakdown {
  total: number
  priority: number
  load: number
  queue: number
  error_rate: number
  ttft: number
  price: number
  health: number
}

export interface OpenAIRoutingSummary {
  account_id: number
  account_name: string
  rank?: number
  tier: OpenAISchedulerTier
  score: OpenAIRoutingScoreBreakdown
  status_label: string
  summary_reason: string
  summary_reasons: string[]
  is_schedulable_now: boolean
  block_reasons?: OpenAIRoutingReasonCode[]
  snapshot_at: string
}

export interface OpenAIRoutingRankingResponse {
  items: OpenAIRoutingSummary[]
  source: string
  snapshot_at: string
}

export interface OpenAIRoutingAccountExplain {
  account: OpenAIRoutingSummary
  top: OpenAIRoutingSummary[]
  notes: string[]
}
```

Also add a top-level type import to `frontend/src/types/index.ts`:

```ts
import type { OpenAIRoutingSummary } from '@/api/admin/openaiScheduler'
```

- [ ] **Step 2: Add API functions**

Append before `openaiSchedulerAPI`:

```ts
export async function getRoutingRanking(
  params: { group_id?: number; model?: string; platform?: string } = {},
  options?: { signal?: AbortSignal }
): Promise<OpenAIRoutingRankingResponse> {
  const { data } = await apiClient.get<OpenAIRoutingRankingResponse>('/admin/openai-scheduler/ranking', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function getRoutingExplain(
  id: number,
  params: { group_id?: number; model?: string; platform?: string } = {}
): Promise<OpenAIRoutingAccountExplain> {
  const { data } = await apiClient.get<OpenAIRoutingAccountExplain>(
    `/admin/openai-scheduler/accounts/${id}/routing-explain`,
    { params }
  )
  return data
}
```

Add both functions to `openaiSchedulerAPI`.

- [ ] **Step 3: Add account optional summary type**

Modify `frontend/src/types/index.ts` `Account` interface:

```ts
routing_priority?: OpenAIRoutingSummary
```

Place it near `stability?: AccountStability`.

- [ ] **Step 4: Run frontend type check**

Run:

```bash
cd frontend
pnpm exec vue-tsc --noEmit
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add frontend/src/api/admin/openaiScheduler.ts frontend/src/types/index.ts
git commit -m "feat: add OpenAI routing explanation frontend types"
```

---

### Task 4: Routing Priority Badge and Modal

**Files:**
- Create: `frontend/src/components/account/RoutingPriorityBadge.vue`
- Create: `frontend/src/components/account/OpenAIRoutingExplainModal.vue`
- Create: `frontend/src/components/account/__tests__/RoutingPriorityBadge.spec.ts`
- Create: `frontend/src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes: `OpenAIRoutingSummary`, `OpenAIRoutingAccountExplain`.
- Produces:
  - `<RoutingPriorityBadge :summary="summary" @open="..."/>`
  - `<OpenAIRoutingExplainModal :show="show" :account-id="id" .../>`

- [ ] **Step 1: Write badge test**

Create `frontend/src/components/account/__tests__/RoutingPriorityBadge.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RoutingPriorityBadge from '../RoutingPriorityBadge.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key.split('.').at(-1) || key,
      te: () => false,
    }),
  }
})

describe('RoutingPriorityBadge', () => {
  it('renders rank and summary for schedulable account', () => {
    const wrapper = mount(RoutingPriorityBadge, {
      props: {
        summary: {
          account_id: 1,
          account_name: 'cheap-fast',
          rank: 3,
          tier: 'primary',
          status_label: '候选',
          summary_reason: '成本优',
          summary_reasons: ['成本优', '低负载'],
          is_schedulable_now: true,
          score: { total: 3.2, priority: 1, load: 0.8, queue: 1, error_rate: 1, ttft: 0.9, price: 1, health: 1 },
          snapshot_at: '2026-06-23T00:00:00Z',
        },
      },
    })

    expect(wrapper.text()).toContain('#3')
    expect(wrapper.text()).toContain('成本优')
  })

  it('renders skipped state when blocked', () => {
    const wrapper = mount(RoutingPriorityBadge, {
      props: {
        summary: {
          account_id: 2,
          account_name: 'blocked',
          tier: 'degraded',
          status_label: '跳过',
          summary_reason: 'rate_limited',
          summary_reasons: ['rate_limited'],
          is_schedulable_now: false,
          block_reasons: ['rate_limited'],
          score: { total: 0, priority: 0, load: 0, queue: 0, error_rate: 0, ttft: 0, price: 0, health: 0 },
          snapshot_at: '2026-06-23T00:00:00Z',
        },
      },
    })

    expect(wrapper.text()).toContain('跳过')
    expect(wrapper.text()).toContain('rate_limited')
  })
})
```

- [ ] **Step 2: Implement badge**

Create `frontend/src/components/account/RoutingPriorityBadge.vue`:

```vue
<template>
  <button
    type="button"
    class="inline-flex min-w-[92px] items-center justify-center gap-1 rounded-md px-2 py-1 text-xs font-medium ring-1 ring-inset transition hover:bg-gray-50 dark:hover:bg-dark-700"
    :class="badgeClass"
    @click="$emit('open')"
  >
    <span v-if="summary?.is_schedulable_now && summary.rank">#{{ summary.rank }}</span>
    <span v-else>{{ summary?.status_label || '未知' }}</span>
    <span>{{ primaryText }}</span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpenAIRoutingSummary } from '@/api/admin/openaiScheduler'

const props = defineProps<{ summary?: OpenAIRoutingSummary | null }>()
defineEmits<{ (e: 'open'): void }>()
const { t, te } = useI18n()

const reasonLabel = (reason: string) => {
  const key = `admin.accounts.routingPriority.reasons.${reason}`
  return te(key) ? t(key) : reason
}

const primaryText = computed(() => {
  if (!props.summary) return '未计算'
  return reasonLabel(props.summary.summary_reason || props.summary.tier || 'schedulable')
})

const badgeClass = computed(() => {
  if (!props.summary) return 'text-gray-500 ring-gray-200 dark:ring-dark-600'
  if (!props.summary.is_schedulable_now) return 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-300 dark:ring-amber-800'
  if (props.summary.tier === 'primary') return 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-300 dark:ring-emerald-800'
  if (props.summary.tier === 'observe') return 'bg-blue-50 text-blue-700 ring-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:ring-blue-800'
  return 'bg-gray-50 text-gray-700 ring-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:ring-dark-600'
})
</script>
```

- [ ] **Step 3: Write modal test**

Create `frontend/src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OpenAIRoutingExplainModal from '../OpenAIRoutingExplainModal.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key.split('.').at(-1) || key,
      te: () => false,
    }),
  }
})

describe('OpenAIRoutingExplainModal', () => {
  it('renders score breakdown and top candidates', () => {
    const wrapper = mount(OpenAIRoutingExplainModal, {
      props: {
        show: true,
        loading: false,
        explain: {
          account: {
            account_id: 1,
            account_name: 'cheap-fast',
            rank: 1,
            tier: 'primary',
            status_label: '候选',
            summary_reason: '成本优',
            summary_reasons: ['成本优'],
            is_schedulable_now: true,
            score: { total: 3.2, priority: 1, load: 0.8, queue: 1, error_rate: 1, ttft: 0.9, price: 1, health: 1 },
            snapshot_at: '2026-06-23T00:00:00Z',
          },
          top: [],
          notes: ['Top-K 加权模式下候选排名第一不代表每次必选。'],
        },
      },
    })

    expect(wrapper.text()).toContain('cheap-fast')
    expect(wrapper.text()).toContain('price')
    expect(wrapper.text()).toContain('Top-K')
  })
})
```

- [ ] **Step 4: Implement modal**

Create `frontend/src/components/account/OpenAIRoutingExplainModal.vue`:

```vue
<template>
  <BaseDialog :show="show" title="调度优先级详情" width="wide" @close="$emit('close')">
    <div v-if="loading" class="py-8 text-center text-sm text-gray-500">加载中...</div>
    <div v-else-if="!explain" class="py-8 text-center text-sm text-gray-500">暂无调度解释</div>
    <div v-else class="space-y-4">
      <section>
        <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ explain.account.account_name }}</div>
        <div class="mt-1 text-xs text-gray-500">
          {{ explain.account.is_schedulable_now ? `#${explain.account.rank}` : explain.account.status_label }}
          · {{ explain.account.tier }} · {{ explain.account.summary_reason }}
        </div>
      </section>

      <section class="grid grid-cols-2 gap-2 text-xs md:grid-cols-4">
        <div v-for="item in scoreItems" :key="item.key" class="rounded-md border border-gray-200 p-2 dark:border-dark-600">
          <div class="text-gray-500">{{ item.key }}</div>
          <div class="font-semibold text-gray-900 dark:text-white">{{ item.value.toFixed(3) }}</div>
        </div>
      </section>

      <section v-if="explain.account.block_reasons?.length" class="text-xs">
        <div class="mb-1 font-semibold text-gray-700 dark:text-gray-200">排除原因</div>
        <div class="flex flex-wrap gap-1">
          <span v-for="reason in explain.account.block_reasons" :key="reason" class="rounded bg-amber-100 px-2 py-0.5 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
            {{ reasonLabel(reason) }}
          </span>
        </div>
      </section>

      <section>
        <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">Top 候选</div>
        <div class="space-y-1">
          <div v-for="row in explain.top" :key="row.account_id" class="flex items-center justify-between rounded-md bg-gray-50 px-2 py-1 text-xs dark:bg-dark-700">
            <span>#{{ row.rank || '-' }} {{ row.account_name }}</span>
            <span>{{ row.score.total.toFixed(3) }}</span>
          </div>
        </div>
      </section>

      <section class="space-y-1 text-xs text-gray-500">
        <p v-for="note in explain.notes" :key="note">{{ note }}</p>
      </section>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { OpenAIRoutingAccountExplain } from '@/api/admin/openaiScheduler'

const props = defineProps<{
  show: boolean
  loading: boolean
  explain: OpenAIRoutingAccountExplain | null
}>()

defineEmits<{ (e: 'close'): void }>()
const { t, te } = useI18n()

const reasonLabel = (reason: string) => {
  const key = `admin.accounts.routingPriority.reasons.${reason}`
  return te(key) ? t(key) : reason
}

const scoreItems = computed(() => {
  const score = props.explain?.account.score
  if (!score) return []
  return Object.entries(score).map(([key, value]) => ({ key, value }))
})
</script>
```

- [ ] **Step 5: Add routing priority i18n keys**

In `frontend/src/i18n/locales/zh.ts`, under `admin.accounts`, add:

```ts
routingPriority: {
  loadFailed: '调度优先级详情加载失败',
  reasons: {
    schedulable: '可调度',
    status_error: '错误状态',
    status_inactive: '非启用状态',
    manual_unschedulable: '手动停调度',
    rate_limited: '429 冷却',
    overloaded: '过载冷却',
    temp_unschedulable: '临时不可调度',
    runtime_blocked: '运行时阻断',
    health_degraded: '健康隔离',
    model_unsupported: '模型不支持',
    capability_unsupported: '能力不支持',
    transport_unsupported: '传输不支持',
    group_mismatch: '分组不匹配',
    privacy_not_set: '隐私状态未设置',
    quota_auto_paused: '额度自动暂停',
    concurrency_full: '并发已满',
    channel_restricted: '渠道限制',
    compact_unsupported: 'compact 不支持',
  },
},
```

In `frontend/src/i18n/locales/en.ts`, add matching keys:

```ts
routingPriority: {
  loadFailed: 'Failed to load routing priority details',
  reasons: {
    schedulable: 'Schedulable',
    status_error: 'Error status',
    status_inactive: 'Inactive',
    manual_unschedulable: 'Manually disabled',
    rate_limited: '429 cooldown',
    overloaded: 'Overload cooldown',
    temp_unschedulable: 'Temporarily disabled',
    runtime_blocked: 'Runtime blocked',
    health_degraded: 'Health degraded',
    model_unsupported: 'Model unsupported',
    capability_unsupported: 'Capability unsupported',
    transport_unsupported: 'Transport unsupported',
    group_mismatch: 'Group mismatch',
    privacy_not_set: 'Privacy unset',
    quota_auto_paused: 'Quota auto-paused',
    concurrency_full: 'Concurrency full',
    channel_restricted: 'Channel restricted',
    compact_unsupported: 'Compact unsupported',
  },
},
```

- [ ] **Step 6: Run component tests**

Run:

```bash
cd frontend
pnpm vitest run src/components/account/__tests__/RoutingPriorityBadge.spec.ts src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add frontend/src/components/account/RoutingPriorityBadge.vue frontend/src/components/account/OpenAIRoutingExplainModal.vue frontend/src/components/account/__tests__/RoutingPriorityBadge.spec.ts frontend/src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: add OpenAI routing priority UI components"
```

---

### Task 5: Account Management Page Integration

**Files:**
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Create: `frontend/src/views/admin/__tests__/AccountsView.routingPriority.spec.ts`

**Interfaces:**
- Consumes: `openaiSchedulerAPI.getRoutingRanking`, `openaiSchedulerAPI.getRoutingExplain`.
- Produces: Account table column `routing_priority` and modal open behavior.

- [ ] **Step 1: Write failing page test**

Create `frontend/src/views/admin/__tests__/AccountsView.routingPriority.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountsView from '../AccountsView.vue'

vi.mock('@/api/admin/openaiScheduler', () => ({
  openaiSchedulerAPI: {
    getRoutingRanking: vi.fn().mockResolvedValue({
      items: [{
        account_id: 10,
        account_name: 'cheap-fast',
        rank: 1,
        tier: 'primary',
        status_label: '候选',
        summary_reason: '成本优',
        summary_reasons: ['成本优'],
        is_schedulable_now: true,
        score: { total: 3, priority: 1, load: 1, queue: 1, error_rate: 1, ttft: 1, price: 1, health: 1 },
        snapshot_at: '2026-06-23T00:00:00Z',
      }],
      source: 'scheduler_snapshot',
      snapshot_at: '2026-06-23T00:00:00Z',
    }),
    getRoutingExplain: vi.fn(),
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  list: vi.fn().mockResolvedValue({
    items: [{
      id: 10,
      name: 'cheap-fast',
      platform: 'openai',
      type: 'apikey',
      status: 'active',
      schedulable: true,
      priority: 1,
      concurrency: 5,
      rate_limit_reset_at: null,
      overload_until: null,
      temp_unschedulable_until: null,
    }],
    total: 1,
  }),
}))

describe('AccountsView routing priority', () => {
  it('renders routing priority summary for OpenAI account', async () => {
    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          DataTable: { template: '<div><slot name="cell-routing_priority" :row="{ id: 10, routing_priority: { rank: 1, summary_reason: `成本优`, is_schedulable_now: true, tier: `primary`, score: { total: 3 } } }" /></div>' },
          TablePageLayout: { template: '<div><slot name="table" /></div>' },
        },
      },
    })

    await vi.dynamicImportSettled()
    expect(wrapper.text()).toContain('#1')
    expect(wrapper.text()).toContain('成本优')
  })
})
```

- [ ] **Step 2: Add imports and state**

Modify `frontend/src/views/admin/AccountsView.vue` imports:

```ts
import RoutingPriorityBadge from '@/components/account/RoutingPriorityBadge.vue'
import OpenAIRoutingExplainModal from '@/components/account/OpenAIRoutingExplainModal.vue'
import { openaiSchedulerAPI, type OpenAIRoutingAccountExplain } from '@/api/admin/openaiScheduler'
```

Add state near other refs:

```ts
const routingExplainVisible = ref(false)
const routingExplainLoading = ref(false)
const routingExplain = ref<OpenAIRoutingAccountExplain | null>(null)
const routingExplainAccountId = ref<number | null>(null)
```

- [ ] **Step 3: Add table column**

Add to `allColumns` in `AccountsView.vue`:

```ts
{ key: 'routing_priority', label: t('admin.accounts.columns.routingPriority'), sortable: false },
```

Place it after `stability` and before `usage`.

- [ ] **Step 4: Render column and modal**

Add table cell template:

```vue
<template #cell-routing_priority="{ row }">
  <RoutingPriorityBadge
    :summary="row.routing_priority"
    @open="openRoutingExplain(row)"
  />
</template>
```

Add modal near existing modals:

```vue
<OpenAIRoutingExplainModal
  :show="routingExplainVisible"
  :loading="routingExplainLoading"
  :explain="routingExplain"
  @close="closeRoutingExplain"
/>
```

- [ ] **Step 5: Load routing summaries**

Add function in `AccountsView.vue`:

```ts
const refreshRoutingPriorities = async () => {
  const openAIAccounts = accounts.value.filter((account) => account.platform === 'openai')
  if (openAIAccounts.length === 0) return
  try {
    const result = await openaiSchedulerAPI.getRoutingRanking({})
    const byID = new Map(result.items.map((item) => [item.account_id, item]))
    accounts.value = accounts.value.map((account) => ({
      ...account,
      routing_priority: byID.get(account.id) ?? account.routing_priority,
    }))
  } catch (error) {
    console.error('Failed to refresh OpenAI routing priorities:', error)
  }
}
```

Call it after successful account list load, after `mergeAccountsIncrementally(result.data.items || [])`, and inside manual refresh:

```ts
await refreshRoutingPriorities()
```

Update `mergeAccountsIncrementally` so replaced rows preserve the last known routing summary until the ranking refresh returns:

```ts
const mergeRoutingPriority = (next: Account, current?: Account): Account => ({
  ...next,
  routing_priority: next.routing_priority ?? current?.routing_priority,
})
```

Use `mergeRoutingPriority(nextRow, currentRow)` for new and replaced rows.

- [ ] **Step 6: Add modal open/close functions**

Add:

```ts
const openRoutingExplain = async (account: Account) => {
  routingExplainAccountId.value = account.id
  routingExplainVisible.value = true
  routingExplainLoading.value = true
  routingExplain.value = null
  try {
    routingExplain.value = await openaiSchedulerAPI.getRoutingExplain(account.id, {})
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.routingPriority.loadFailed'))
  } finally {
    routingExplainLoading.value = false
  }
}

const closeRoutingExplain = () => {
  routingExplainVisible.value = false
  routingExplainLoading.value = false
  routingExplain.value = null
  routingExplainAccountId.value = null
}
```

- [ ] **Step 7: Add i18n keys**

In `frontend/src/i18n/locales/zh.ts`, under `admin.accounts.columns` add:

```ts
routingPriority: '调度优先级',
```

In `frontend/src/i18n/locales/en.ts`, add:

```ts
routingPriority: 'Routing Priority',
```

- [ ] **Step 8: Run page test**

Run:

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/AccountsView.routingPriority.spec.ts
```

Expected: PASS.

- [ ] **Step 9: Commit Task 5**

```bash
git add frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/__tests__/AccountsView.routingPriority.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: show routing priority on account list"
```

---

### Task 6: Cooldown Countdown Refresh

**Files:**
- Modify: `frontend/src/components/account/AccountStatusIndicator.vue`
- Modify: `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`

**Interfaces:**
- Consumes: account cooldown fields already present on `Account`.
- Produces: UI recomputes status from a reactive `nowMs` tick.

- [ ] **Step 1: Write failing countdown test**

Append to `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts`:

```ts
it('updates rate-limit state when countdown expires', async () => {
  vi.useFakeTimers()
  const resetAt = new Date(Date.now() + 1000).toISOString()
  const wrapper = mount(AccountStatusIndicator, {
    props: {
      account: {
        ...makeAccount({}),
        rate_limit_reset_at: resetAt,
      },
    },
  })

  expect(wrapper.text()).toContain('429')
  vi.advanceTimersByTime(1500)
  await nextTick()
  expect(wrapper.text()).not.toContain('429')
  vi.useRealTimers()
})
```

If `nextTick` is not imported in this test file, update the imports to:

```ts
import { nextTick } from 'vue'
```

- [ ] **Step 2: Add reactive clock**

Modify `<script setup>` in `AccountStatusIndicator.vue`:

```ts
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
```

Add:

```ts
const nowMs = ref(Date.now())
let nowTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  nowTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (nowTimer !== null) {
    clearInterval(nowTimer)
    nowTimer = null
  }
})
```

Replace direct `new Date()` comparisons in computed values with:

```ts
new Date(props.account.rate_limit_reset_at).getTime() > nowMs.value
```

Apply the same pattern for `overload_until`, `temp_unschedulable_until`, and model-level reset checks.

- [ ] **Step 3: Run status tests**

Run:

```bash
cd frontend
pnpm vitest run src/components/account/__tests__/AccountStatusIndicator.spec.ts
```

Expected: PASS.

- [ ] **Step 4: Commit Task 6**

```bash
git add frontend/src/components/account/AccountStatusIndicator.vue frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts
git commit -m "fix: refresh account cooldown status countdowns"
```

---

### Task 7: Phase 2 Scheduler Strategy Settings

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_scheduler_tier_selection_test.go`
- Modify: `backend/internal/service/openai_scheduler_health_test.go`

**Interfaces:**
- Consumes: existing scheduler scoring path.
- Produces:
  - `gateway.openai_scheduler.routing_strategy`
  - `gateway.openai_scheduler.selection_mode`
  - real use of `ObserveProbeRatio`.

- [ ] **Step 1: Write failing strategy tests**

Append to `backend/internal/service/openai_scheduler_tier_selection_test.go`:

```go
func TestOpenAISchedulerCostFirstBoostsCheapComparableAccount(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 0.5
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Price = 0.6
	cfg.Gateway.OpenAIScheduler.RoutingStrategy = "cost_first"
	scheduler := &defaultOpenAIAccountScheduler{stats: newOpenAIAccountRuntimeStats(), service: &OpenAIGatewayService{cfg: cfg}}

	cheapPrice := 0.05
	expensivePrice := 0.20
	cheap := &Account{ID: 701, Priority: 1, ChannelPrice: &cheapPrice}
	expensive := &Account{ID: 702, Priority: 1, ChannelPrice: &expensivePrice}
	scheduler.stats.report(cheap.ID, true, intPtrForTest(700))
	scheduler.stats.report(expensive.ID, true, intPtrForTest(350))

	plan := scheduler.buildOpenAIAccountLoadPlan(OpenAIAccountScheduleRequest{SessionHash: "cost-first"}, []*Account{cheap, expensive}, map[int64]*AccountLoadInfo{
		cheap.ID:     {AccountID: cheap.ID},
		expensive.ID: {AccountID: expensive.ID},
	})

	require.Equal(t, cheap.ID, plan.selectionOrder[0].account.ID)
}

func TestOpenAISchedulerStrictBestKeepsHighestScoreFirst(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.SelectionMode = "strict_best"
	scheduler := &defaultOpenAIAccountScheduler{stats: newOpenAIAccountRuntimeStats(), service: &OpenAIGatewayService{cfg: cfg}}
	candidates := []openAIAccountCandidateScore{
		{account: &Account{ID: 801, Priority: 1}, loadInfo: &AccountLoadInfo{}, score: 1},
		{account: &Account{ID: 802, Priority: 1}, loadInfo: &AccountLoadInfo{}, score: 3},
	}

	order := scheduler.buildOpenAISelectionOrder(OpenAIAccountScheduleRequest{SessionHash: "strict-best"}, openAIAccountLoadPlan{candidates: candidates, topK: 2})

	require.Equal(t, int64(802), order[0].account.ID)
}
```

- [ ] **Step 2: Extend config struct and defaults**

Modify `GatewayOpenAISchedulerConfig` in `backend/internal/config/config.go`:

```go
RoutingStrategy string `mapstructure:"routing_strategy"`
SelectionMode   string `mapstructure:"selection_mode"`
```

Add defaults:

```go
viper.SetDefault("gateway.openai_scheduler.routing_strategy", "balanced")
viper.SetDefault("gateway.openai_scheduler.selection_mode", "weighted_top_k")
```

Also add post-unmarshal fallbacks beside the existing `StickyEscape*` and `PriceBoost*` defaults:

```go
if strings.TrimSpace(cfg.Gateway.OpenAIScheduler.RoutingStrategy) == "" {
	cfg.Gateway.OpenAIScheduler.RoutingStrategy = "balanced"
}
if strings.TrimSpace(cfg.Gateway.OpenAIScheduler.SelectionMode) == "" {
	cfg.Gateway.OpenAIScheduler.SelectionMode = "weighted_top_k"
}
```

Add validation:

```go
switch c.Gateway.OpenAIScheduler.RoutingStrategy {
case "", "balanced", "cost_first", "speed_first":
default:
	return fmt.Errorf("gateway.openai_scheduler.routing_strategy must be balanced, cost_first, or speed_first")
}
switch c.Gateway.OpenAIScheduler.SelectionMode {
case "", "weighted_top_k", "strict_best":
default:
	return fmt.Errorf("gateway.openai_scheduler.selection_mode must be weighted_top_k or strict_best")
}
```

- [ ] **Step 3: Apply routing strategy to weights**

Modify `openAIWSSchedulerWeights` or add a new helper in `openai_account_scheduler.go`:

```go
func (s *OpenAIGatewayService) adjustedOpenAIWSSchedulerWeights() GatewayOpenAIWSSchedulerScoreWeightsView {
	weights := s.openAIWSSchedulerWeights()
	if s == nil || s.cfg == nil {
		return weights
	}
	switch s.cfg.Gateway.OpenAIScheduler.RoutingStrategy {
	case "cost_first":
		weights.Price *= 2.5
		weights.TTFT *= 0.8
	case "speed_first":
		weights.TTFT *= 2.0
		weights.ErrorRate *= 1.5
		weights.Price *= 0.5
	}
	return weights
}
```

Use `adjustedOpenAIWSSchedulerWeights()` in `buildOpenAIAccountLoadPlan`.

- [ ] **Step 4: Add strict best selection mode**

Modify `buildOpenAISelectionOrder`:

```go
if s != nil && s.service != nil && s.service.cfg != nil && s.service.cfg.Gateway.OpenAIScheduler.SelectionMode == "strict_best" {
	return selectTopKOpenAICandidates(plan.candidates, plan.topK)
}
```

Place the check before weighted selection for non-compact paths.

- [ ] **Step 5: Implement observe probe**

In `buildOpenAISelectionOrder`, when `HealthRankingEnabled` is true and `ObserveProbeRatio > 0`, append a bounded number of observe candidates after primary/standby:

```go
probeLimit := int(math.Ceil(float64(limit) * settings.ObserveProbeRatio))
if probeLimit < 1 && settings.ObserveProbeRatio > 0 {
	probeLimit = 1
}
```

Only include `OpenAISchedulerTierObserve`. Do not include `OpenAISchedulerTierDegraded`.

- [ ] **Step 6: Run scheduler tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestOpenAISchedulerCostFirst|TestOpenAISchedulerStrictBest|TestOpenAISchedulerBuildOpenAIAccountLoadPlan|TestDefaultOpenAIAccountScheduler' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 7**

```bash
git add backend/internal/config/config.go backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_scheduler_tier_selection_test.go backend/internal/service/openai_scheduler_health_test.go
git commit -m "feat: add OpenAI scheduler strategy controls"
```

---

### Task 8: Phase 2 Health Failure Reason Classification

**Files:**
- Modify: `backend/internal/service/openai_account_scheduler.go`
- Modify: `backend/internal/service/openai_account_runtime_block_fastpath.go`
- Modify: `backend/internal/handler/openai_gateway_handler.go`
- Modify: `backend/internal/handler/openai_chat_completions.go`
- Modify: `backend/internal/handler/openai_images.go`
- Modify: `backend/internal/handler/openai_embeddings.go`
- Modify: `backend/internal/service/openai_scheduler_health_test.go`

**Interfaces:**
- Consumes: existing report call sites.
- Produces:
  - `ReportOpenAIAccountScheduleResultWithReason(accountID int64, success bool, firstTokenMs *int, reason string)`
  - reason-aware health snapshots.

- [ ] **Step 1: Write failing health reason test**

Append to `backend/internal/service/openai_scheduler_health_test.go`:

```go
func TestDefaultOpenAIAccountScheduler_ReportResultWithReason(t *testing.T) {
	scheduler := newDefaultOpenAIAccountScheduler(nil, newOpenAIAccountRuntimeStats()).(*defaultOpenAIAccountScheduler)

	scheduler.ReportResultWithReason(9101, false, nil, OpenAISchedulerDegradeRateLimited)
	snapshot, ok := scheduler.SnapshotAccountHealth(context.Background(), 9101)

	require.True(t, ok)
	require.Equal(t, OpenAISchedulerDegradeRateLimited, snapshot.DegradeReason)
}
```

- [ ] **Step 2: Add interface method**

Modify `OpenAIAccountScheduler`:

```go
ReportResultWithReason(accountID int64, success bool, firstTokenMs *int, reason string)
```

Implement on `defaultOpenAIAccountScheduler`:

```go
func (s *defaultOpenAIAccountScheduler) ReportResultWithReason(accountID int64, success bool, firstTokenMs *int, reason string) {
	if s == nil || s.stats == nil {
		return
	}
	s.stats.reportWithReason(accountID, success, firstTokenMs, reason)
}
```

- [ ] **Step 3: Add reason-aware stats report**

Add to `openAIAccountRuntimeStats`:

```go
func (s *openAIAccountRuntimeStats) reportWithReason(accountID int64, success bool, firstTokenMs *int, reason string) {
	s.report(accountID, success, firstTokenMs)
	if success || strings.TrimSpace(reason) == "" {
		return
	}
	stat := s.loadOrCreate(accountID)
	stat.healthMu.Lock()
	stat.health.lastDegradeReason = strings.TrimSpace(reason)
	stat.healthMu.Unlock()
}
```

- [ ] **Step 4: Add gateway facade**

Add to `OpenAIGatewayService`:

```go
func (s *OpenAIGatewayService) ReportOpenAIAccountScheduleResultWithReason(accountID int64, success bool, firstTokenMs *int, reason string) {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.ReportResultWithReason(accountID, success, firstTokenMs, reason)
}
```

- [ ] **Step 5: Update failure call sites**

Use concrete mappings:

```go
status 429 -> OpenAISchedulerDegradeRateLimited
status 403 -> OpenAISchedulerDegradeManual
status 500..599 -> OpenAISchedulerDegradeUpstream5xx
context deadline / timeout -> OpenAISchedulerDegradeTimeout
transport errors -> "transport_error"
```

Replace failure reports in OpenAI handlers with the reason-aware method when an upstream status or error class is available. Keep existing `ReportOpenAIAccountScheduleResult` for success.

- [ ] **Step 6: Run reason tests and handler compile**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler -run 'TestDefaultOpenAIAccountScheduler_ReportResultWithReason|TestOpenAI' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 8**

```bash
git add backend/internal/service/openai_account_scheduler.go backend/internal/service/openai_account_runtime_block_fastpath.go backend/internal/handler/openai_gateway_handler.go backend/internal/handler/openai_chat_completions.go backend/internal/handler/openai_images.go backend/internal/handler/openai_embeddings.go backend/internal/service/openai_scheduler_health_test.go
git commit -m "feat: classify OpenAI scheduler health failures"
```

---

## Final Verification

- [ ] Run backend targeted tests:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/handler/admin -run 'OpenAIRouting|OpenAIScheduler|AccountScheduler' -count=1
```

Expected: PASS.

- [ ] Run frontend targeted tests:

```bash
cd frontend
pnpm vitest run src/components/account/__tests__/RoutingPriorityBadge.spec.ts src/components/account/__tests__/OpenAIRoutingExplainModal.spec.ts src/components/account/__tests__/AccountStatusIndicator.spec.ts src/views/admin/__tests__/AccountsView.routingPriority.spec.ts
```

Expected: PASS.

- [ ] Run type checks:

```bash
cd frontend
pnpm exec vue-tsc --noEmit
```

Expected: PASS.

- [ ] Run formatting:

```bash
cd backend
gofmt -w internal/service/openai_routing_explain.go internal/service/openai_routing_explain_test.go internal/handler/admin/openai_scheduler_handler.go internal/handler/admin/openai_scheduler_handler_test.go internal/server/routes/admin.go internal/config/config.go internal/service/openai_account_scheduler.go internal/service/openai_scheduler_tier_selection_test.go internal/service/openai_scheduler_health_test.go
```

Expected: command exits 0.

## Self-Review Checklist

- Spec coverage:
  - Account list routing priority: Task 5.
  - Detail modal and reason display: Task 4 and Task 5.
  - Backend explanation APIs: Task 1 and Task 2.
  - State source distinction: Task 1 reason codes and Task 4 modal.
  - Countdown refresh: Task 6.
  - Cost/speed strategy: Task 7.
  - Top-K mode: Task 7.
  - Observe probe: Task 7.
  - Health failure reason classification: Task 8.
- Placeholder scan: no placeholder tasks are present.
- Type consistency:
  - Backend DTO names use `OpenAIRouting*`.
  - Frontend API names use `OpenAIRouting*`.
  - Account optional summary field is `routing_priority`.
