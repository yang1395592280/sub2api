# Exclusive Group User Usage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a专属分组“查看用户”用量对比 view that shows each member's yesterday/today requests, tokens, account billed cost, and user billed cost using the same metric semantics as account management.

**Architecture:** Add a backend aggregation path near existing group member routes and reuse `usagestats.AccountStats` for the per-day metrics. The frontend keeps the existing `GroupMembersModal` structure, hides the entry for non-exclusive groups, and loads one batch comparison payload for the visible members.

**Tech Stack:** Go + Gin + SQL repository + existing `usagestats.AccountStats`; Vue 3 + TypeScript + Vitest; no new dependencies.

## Global Constraints

- Only groups with `is_exclusive = true` show the “查看用户” action.
- Usage rows show only today and yesterday.
- Metric semantics must match account management: `requests`, `tokens`, `cost`, `standard_cost`, `user_cost`.
- `cost` is account billed cost and can be rendered as `A $xx.xx`.
- `user_cost` is user/API Key billed cost and can be rendered as `U $xx.xx`.
- Aggregation scope must be `group_id + user_id + date window`.
- Do not add a request-detail table, 7-day/30-day range, or custom range.
- Keep changes focused; do not refactor unrelated group or usage code.

---

## File Structure

- `backend/internal/service/account_usage_service.go`
  - Extend `UsageLogRepository` with a batch group/user day-window aggregation method.
- `backend/internal/repository/usage_log_repo.go`
  - Implement the SQL aggregation using the same formulas as account management today stats.
- `backend/internal/repository/usage_log_repo_integration_test.go`
  - Add repository integration tests proving group/user/date isolation and zero-fill behavior.
- `backend/internal/service/admin_service.go`
  - No code changes planned; the handler will reuse existing `AdminService.GetGroup` for group validation.
- `backend/internal/service/dashboard_service.go`
  - Add service DTOs and a method that computes today/yesterday windows and delegates to the usage repository.
- `backend/internal/service/dashboard_service_test.go`
  - Add service tests for yesterday/today comparison and zero-fill behavior.
- `backend/internal/handler/admin/group_handler.go`
  - Add the HTTP handler for the comparison endpoint, request parsing, exclusive-group validation, and response mapping.
- `backend/internal/server/routes/admin.go`
  - Register `GET /api/v1/admin/groups/:id/members/usage-comparison`.
- `frontend/src/api/admin/groups.ts`
  - Add TypeScript types and API client method for the comparison endpoint.
- `frontend/src/views/admin/GroupsView.vue`
  - Hide “查看用户” for non-exclusive groups.
- `frontend/src/components/admin/group/GroupMembersModal.vue`
  - Fetch and render today/yesterday usage blocks for each member.
- `frontend/src/components/admin/group/__tests__/GroupMembersModal.spec.ts`
  - Extend component tests for usage display and non-blocking usage failures.
- `frontend/src/i18n/locales/zh.ts`
  - Add labels for today/yesterday, usage load failure, and empty usage.
- `frontend/src/i18n/locales/en.ts`
  - Add matching English labels.

---

### Task 1: Backend Repository Aggregation

**Files:**
- Modify: `backend/internal/service/account_usage_service.go`
- Modify: `backend/internal/repository/usage_log_repo.go`
- Test: `backend/internal/repository/usage_log_repo_integration_test.go`

**Interfaces:**
- Produces:
  - `UsageLogRepository.GetGroupUserDailyStatsBatch(ctx context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error)`
- Consumes:
  - Existing `usagestats.AccountStats` with `Requests`, `Tokens`, `Cost`, `StandardCost`, `UserCost`.

- [ ] **Step 1: Add the failing repository integration test**

Add this test near the existing `TestGetAccountWindowStats` section in `backend/internal/repository/usage_log_repo_integration_test.go`:

```go
// --- GetGroupUserDailyStatsBatch ---

func (s *UsageLogRepoSuite) TestGetGroupUserDailyStatsBatch_GroupUserAndWindowIsolation() {
	user1 := mustCreateUser(s.T(), s.client, &service.User{Email: "group-user-usage-1@test.com"})
	user2 := mustCreateUser(s.T(), s.client, &service.User{Email: "group-user-usage-2@test.com"})
	apiKey1 := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user1.ID, Key: "sk-group-user-usage-1", Name: "k1"})
	apiKey2 := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: user2.ID, Key: "sk-group-user-usage-2", Name: "k2"})
	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "acc-group-user-usage"})

	groupA := mustCreateGroup(s.T(), s.client, &service.Group{Name: "usage-group-a", Platform: service.PlatformAnthropic, IsExclusive: true})
	groupB := mustCreateGroup(s.T(), s.client, &service.Group{Name: "usage-group-b", Platform: service.PlatformAnthropic, IsExclusive: true})

	start := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	s.createUsageLogWithGroup(user1, apiKey1, account, groupA.ID, 10, 20, 1.25, 0.75, start.Add(2*time.Hour))
	s.createUsageLogWithGroup(user1, apiKey1, account, groupA.ID, 30, 40, 2.50, 1.50, start.Add(3*time.Hour))
	s.createUsageLogWithGroup(user2, apiKey2, account, groupA.ID, 50, 60, 3.75, 2.25, start.Add(4*time.Hour))
	s.createUsageLogWithGroup(user1, apiKey1, account, groupB.ID, 70, 80, 9.99, 8.88, start.Add(5*time.Hour))
	s.createUsageLogWithGroup(user1, apiKey1, account, groupA.ID, 90, 100, 7.77, 6.66, start.Add(-time.Hour))
	s.createUsageLogWithGroup(user1, apiKey1, account, groupA.ID, 110, 120, 5.55, 4.44, end.Add(time.Minute))

	stats, err := s.repo.GetGroupUserDailyStatsBatch(s.ctx, groupA.ID, []int64{user1.ID, user2.ID, 999999}, start, end)
	s.Require().NoError(err)

	s.Require().Equal(int64(2), stats[user1.ID].Requests)
	s.Require().Equal(int64(100), stats[user1.ID].Tokens)
	s.Require().InDelta(3.75, stats[user1.ID].StandardCost, 1e-9)
	s.Require().InDelta(3.75, stats[user1.ID].Cost, 1e-9)
	s.Require().InDelta(2.25, stats[user1.ID].UserCost, 1e-9)

	s.Require().Equal(int64(1), stats[user2.ID].Requests)
	s.Require().Equal(int64(110), stats[user2.ID].Tokens)
	s.Require().InDelta(3.75, stats[user2.ID].Cost, 1e-9)
	s.Require().InDelta(2.25, stats[user2.ID].UserCost, 1e-9)

	s.Require().NotNil(stats[int64(999999)])
	s.Require().Equal(int64(0), stats[int64(999999)].Requests)
	s.Require().Equal(int64(0), stats[int64(999999)].Tokens)
}

func (s *UsageLogRepoSuite) createUsageLogWithGroup(user *service.User, apiKey *service.APIKey, account *service.Account, groupID int64, inputTokens, outputTokens int, totalCost, actualCost float64, createdAt time.Time) *service.UsageLog {
	log := &service.UsageLog{
		UserID:       user.ID,
		APIKeyID:     apiKey.ID,
		AccountID:    account.ID,
		GroupID:      &groupID,
		RequestID:    uuid.New().String(),
		Model:        "claude-3",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalCost:    totalCost,
		ActualCost:   actualCost,
		CreatedAt:    createdAt,
	}
	_, err := s.repo.Create(s.ctx, log)
	s.Require().NoError(err)
	return log
}
```

- [ ] **Step 2: Run the repository test and verify it fails**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test -tags=integration ./internal/repository -run TestUsageLogRepoSuite/TestGetGroupUserDailyStatsBatch_GroupUserAndWindowIsolation
```

Expected: FAIL because `GetGroupUserDailyStatsBatch` is not defined.

- [ ] **Step 3: Add the repository interface method**

In `backend/internal/service/account_usage_service.go`, add this method to `UsageLogRepository` near the account stats methods:

```go
GetGroupUserDailyStatsBatch(ctx context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error)
```

- [ ] **Step 4: Implement the aggregation**

In `backend/internal/repository/usage_log_repo.go`, add this function near `GetAccountWindowStatsBatch`:

```go
// GetGroupUserDailyStatsBatch aggregates account-management-compatible stats by user for one group and one time window.
func (r *usageLogRepository) GetGroupUserDailyStatsBatch(ctx context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	result := make(map[int64]*usagestats.AccountStats, len(userIDs))
	if groupID <= 0 || len(userIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT
			user_id,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(total_cost), 0) as standard_cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE group_id = $1
		  AND user_id = ANY($2)
		  AND created_at >= $3
		  AND created_at < $4
		GROUP BY user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, groupID, pq.Array(userIDs), startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		stats := &usagestats.AccountStats{}
		if err := rows.Scan(
			&userID,
			&stats.Requests,
			&stats.Tokens,
			&stats.Cost,
			&stats.StandardCost,
			&stats.UserCost,
		); err != nil {
			return nil, err
		}
		result[userID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, userID := range userIDs {
		if _, ok := result[userID]; !ok {
			result[userID] = &usagestats.AccountStats{}
		}
	}
	return result, nil
}
```

- [ ] **Step 5: Run the repository test and verify it passes**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test -tags=integration ./internal/repository -run TestUsageLogRepoSuite/TestGetGroupUserDailyStatsBatch_GroupUserAndWindowIsolation
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/account_usage_service.go backend/internal/repository/usage_log_repo.go backend/internal/repository/usage_log_repo_integration_test.go
git commit -m "feat: aggregate group user usage stats"
```

---

### Task 2: Backend Dashboard Service And HTTP Endpoint

**Files:**
- Modify: `backend/internal/service/dashboard_service.go`
- Test: `backend/internal/service/dashboard_service_test.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Modify: `backend/internal/server/routes/admin.go`

**Interfaces:**
- Consumes:
  - `UsageLogRepository.GetGroupUserDailyStatsBatch(ctx, groupID, userIDs, startTime, endTime)`.
  - Existing `AdminService.GetGroup(ctx, groupID)` in the HTTP handler.
- Produces:
  - `DashboardService.GetGroupUserUsageComparison(ctx context.Context, groupID int64, userIDs []int64, todayStart time.Time) (*GroupUserUsageComparisonResult, error)`.
  - `GET /api/v1/admin/groups/:id/members/usage-comparison?user_ids=1,2&timezone=Asia/Shanghai`.

- [ ] **Step 1: Add the failing dashboard service test**

Add this test to `backend/internal/service/dashboard_service_test.go`:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type groupUserUsageDashboardRepoStub struct {
	UsageLogRepository

	stats map[string]map[int64]*usagestats.AccountStats
}

func (s *groupUserUsageDashboardRepoStub) GetGroupUserDailyStatsBatch(_ context.Context, groupID int64, userIDs []int64, startTime, endTime time.Time) (map[int64]*usagestats.AccountStats, error) {
	key := startTime.Format(time.RFC3339) + "|" + endTime.Format(time.RFC3339)
	out := make(map[int64]*usagestats.AccountStats, len(userIDs))
	for _, userID := range userIDs {
		if s.stats != nil && s.stats[key] != nil && s.stats[key][userID] != nil {
			out[userID] = s.stats[key][userID]
		} else {
			out[userID] = &usagestats.AccountStats{}
		}
	}
	_ = groupID
	return out, nil
}

func TestDashboardService_GetGroupUserUsageComparison(t *testing.T) {
	today := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)
	tomorrow := today.AddDate(0, 0, 1)

	usageRepo := &groupUserUsageDashboardRepoStub{
		stats: map[string]map[int64]*usagestats.AccountStats{
			today.Format(time.RFC3339) + "|" + tomorrow.Format(time.RFC3339): {
				1: {Requests: 2, Tokens: 100, Cost: 3.75, StandardCost: 3.75, UserCost: 2.25},
			},
			yesterday.Format(time.RFC3339) + "|" + today.Format(time.RFC3339): {
				1: {Requests: 1, Tokens: 40, Cost: 1.25, StandardCost: 1.25, UserCost: 0.75},
			},
		},
	}
	svc := NewDashboardService(usageRepo, nil, nil, nil)

	got, err := svc.GetGroupUserUsageComparison(context.Background(), 10, []int64{1, 2}, today)
	require.NoError(t, err)
	require.Equal(t, int64(10), got.GroupID)
	require.Equal(t, "2026-06-26", got.Today)
	require.Equal(t, "2026-06-25", got.Yesterday)
	require.Equal(t, int64(2), got.Stats[1].Today.Requests)
	require.Equal(t, int64(1), got.Stats[1].Yesterday.Requests)
	require.Equal(t, int64(0), got.Stats[2].Today.Requests)
	require.Equal(t, int64(0), got.Stats[2].Yesterday.Requests)
}

func TestDashboardService_GetGroupUserUsageComparison_EmptyUsers(t *testing.T) {
	today := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	svc := NewDashboardService(&groupUserUsageDashboardRepoStub{}, nil, nil, nil)

	got, err := svc.GetGroupUserUsageComparison(context.Background(), 10, nil, today)
	require.NoError(t, err)
	require.Equal(t, int64(10), got.GroupID)
	require.Equal(t, "2026-06-26", got.Today)
	require.Equal(t, "2026-06-25", got.Yesterday)
	require.Empty(t, got.Stats)
}
```

- [ ] **Step 2: Run the dashboard service test and verify it fails**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GetGroupUserUsageComparison'
```

Expected: FAIL because the dashboard service method and DTOs do not exist.

- [ ] **Step 3: Add dashboard service DTOs**

In `backend/internal/service/dashboard_service.go`, add these structs near the other dashboard service types:

```go
type GroupUserUsageComparisonResult struct {
	GroupID   int64
	Today    string
	Yesterday string
	Stats    map[int64]GroupUserUsageComparison
}

type GroupUserUsageComparison struct {
	Today     *usagestats.AccountStats
	Yesterday *usagestats.AccountStats
}
```

`backend/internal/service/dashboard_service.go` already imports `github.com/Wei-Shaw/sub2api/internal/pkg/usagestats`.

- [ ] **Step 4: Implement the dashboard service method**

Add this method near `GetGroupUsageSummary`:

```go
// GetGroupUserUsageComparison returns yesterday/today usage stats for selected users in one group.
func (s *DashboardService) GetGroupUserUsageComparison(ctx context.Context, groupID int64, userIDs []int64, todayStart time.Time) (*GroupUserUsageComparisonResult, error) {
	result := &GroupUserUsageComparisonResult{
		GroupID:    groupID,
		Today:     todayStart.Format("2006-01-02"),
		Yesterday: todayStart.AddDate(0, 0, -1).Format("2006-01-02"),
		Stats:     make(map[int64]GroupUserUsageComparison, len(userIDs)),
	}
	for _, userID := range userIDs {
		result.Stats[userID] = GroupUserUsageComparison{
			Today:     &usagestats.AccountStats{},
			Yesterday: &usagestats.AccountStats{},
		}
	}
	if len(userIDs) == 0 || s.usageRepo == nil {
		return result, nil
	}

	tomorrowStart := todayStart.AddDate(0, 0, 1)
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	todayStats, err := s.usageRepo.GetGroupUserDailyStatsBatch(ctx, groupID, userIDs, todayStart, tomorrowStart)
	if err != nil {
		return nil, fmt.Errorf("get group user today usage: %w", err)
	}
	yesterdayStats, err := s.usageRepo.GetGroupUserDailyStatsBatch(ctx, groupID, userIDs, yesterdayStart, todayStart)
	if err != nil {
		return nil, fmt.Errorf("get group user yesterday usage: %w", err)
	}

	for _, userID := range userIDs {
		comparison := result.Stats[userID]
		if todayStats[userID] != nil {
			comparison.Today = todayStats[userID]
		}
		if yesterdayStats[userID] != nil {
			comparison.Yesterday = yesterdayStats[userID]
		}
		result.Stats[userID] = comparison
	}
	return result, nil
}
```

- [ ] **Step 5: Add the HTTP handler with exclusive-group validation**

In `backend/internal/handler/admin/group_handler.go`, add helper request/response structs near `GetGroupMembers`:

```go
type groupMemberUsageStatsResponse struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
	UserCost     float64 `json:"user_cost"`
}

type groupMemberUsageComparisonResponse struct {
	Today     groupMemberUsageStatsResponse `json:"today"`
	Yesterday groupMemberUsageStatsResponse `json:"yesterday"`
}
```

Add this handler:

```go
// GetGroupMemberUsageComparison handles yesterday/today usage comparison for users in an exclusive group.
// GET /api/v1/admin/groups/:id/members/usage-comparison?user_ids=1,2&timezone=Asia/Shanghai
func (h *GroupHandler) GetGroupMemberUsageComparison(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	userIDs, err := parseInt64CSV(c.Query("user_ids"))
	if err != nil {
		response.BadRequest(c, "Invalid user_ids")
		return
	}

	group, err := h.adminService.GetGroup(c.Request.Context(), groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !group.IsExclusive {
		response.ErrorFrom(c, infraerrors.Forbidden("GROUP_USAGE_EXCLUSIVE_ONLY", "group user usage comparison is only available for exclusive groups"))
		return
	}

	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	todayStart := timezone.StartOfDayInUserLocation(now, userTZ)

	result, err := h.dashboardService.GetGroupUserUsageComparison(c.Request.Context(), groupID, userIDs, todayStart)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := struct {
		GroupID   int64                                      `json:"group_id"`
		Today    string                                     `json:"today"`
		Yesterday string                                    `json:"yesterday"`
		Stats    map[string]groupMemberUsageComparisonResponse `json:"stats"`
	}{
		GroupID:    result.GroupID,
		Today:     result.Today,
		Yesterday: result.Yesterday,
		Stats:     make(map[string]groupMemberUsageComparisonResponse, len(result.Stats)),
	}
	for userID, comparison := range result.Stats {
		out.Stats[strconv.FormatInt(userID, 10)] = groupMemberUsageComparisonResponse{
			Today:     groupMemberUsageStatsFromService(comparison.Today),
			Yesterday: groupMemberUsageStatsFromService(comparison.Yesterday),
		}
	}
	response.Success(c, out)
}

func groupMemberUsageStatsFromService(stats *usagestats.AccountStats) groupMemberUsageStatsResponse {
	if stats == nil {
		return groupMemberUsageStatsResponse{}
	}
	return groupMemberUsageStatsResponse{
		Requests:     stats.Requests,
		Tokens:       stats.Tokens,
		Cost:         stats.Cost,
		StandardCost: stats.StandardCost,
		UserCost:     stats.UserCost,
	}
}

func parseInt64CSV(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int64{}, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid id %q", part)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}
```

Add these imports to `backend/internal/handler/admin/group_handler.go`:

```go
infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
```

- [ ] **Step 6: Register the route**

In `backend/internal/server/routes/admin.go`, add this before `groups.GET("/:id/members", ...)`:

```go
groups.GET("/:id/members/usage-comparison", h.Admin.Group.GetGroupMemberUsageComparison)
```

- [ ] **Step 7: Run dashboard service tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GetGroupUserUsageComparison'
```

Expected: PASS.

- [ ] **Step 8: Run route/API contract tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/server -run TestAPIContract
```

Expected: exit 0. Acceptable output is either PASS or `testing: warning: no tests to run`.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/service/dashboard_service.go backend/internal/service/dashboard_service_test.go backend/internal/handler/admin/group_handler.go backend/internal/server/routes/admin.go
git commit -m "feat: expose exclusive group member usage comparison"
```

---

### Task 3: Frontend API And Members Modal Display

**Files:**
- Modify: `frontend/src/api/admin/groups.ts`
- Modify: `frontend/src/components/admin/group/GroupMembersModal.vue`
- Modify: `frontend/src/components/admin/group/__tests__/GroupMembersModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes:
  - `GET /admin/groups/:id/members/usage-comparison`.
- Produces:
  - `groupsAPI.getGroupMemberUsageComparison(id: number, userIds: number[], timezone?: string): Promise<GroupMemberUsageComparisonResponse>`.
  - `GroupMembersModal` renders two usage rows per member when `group.is_exclusive` and members are fixed.

- [ ] **Step 1: Add failing component tests**

Update the API mock in `frontend/src/components/admin/group/__tests__/GroupMembersModal.spec.ts`:

```ts
getGroupMemberUsageComparison: vi.fn(),
```

Reset it in `beforeEach`:

```ts
vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockReset()
```

Add these tests:

```ts
it('loads and renders yesterday and today usage for exclusive members', async () => {
  vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
    group_id: 12,
    has_fixed_members: true,
    total: 1,
    items: [{ id: 1, username: 'alice', email: 'alice@test.com', notes: '', status: 'active' }]
  })
  vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockResolvedValue({
    group_id: 12,
    today: '2026-06-26',
    yesterday: '2026-06-25',
    stats: {
      '1': {
        today: { requests: 2600, tokens: 232900000, cost: 281.55, standard_cost: 250.1, user_cost: 41.79 },
        yesterday: { requests: 1900, tokens: 180200000, cost: 210.3, standard_cost: 198.4, user_cost: 35.12 }
      }
    }
  })

  const wrapper = mountModal()
  await flushPromises()

  expect(adminAPI.groups.getGroupMemberUsageComparison).toHaveBeenCalledWith(
    12,
    [1],
    expect.any(String)
  )
  expect(wrapper.text()).toContain('今天')
  expect(wrapper.text()).toContain('昨天')
  expect(wrapper.text()).toContain('2.6K req')
  expect(wrapper.text()).toContain('232.90M token')
  expect(wrapper.text()).toContain('A $281.55')
  expect(wrapper.text()).toContain('U $41.79')
})

it('keeps members visible when usage comparison fails', async () => {
  vi.mocked(adminAPI.groups.getGroupMembers).mockResolvedValue({
    group_id: 12,
    has_fixed_members: true,
    total: 1,
    items: [{ id: 1, username: 'alice', email: 'alice@test.com', notes: '', status: 'active' }]
  })
  vi.mocked(adminAPI.groups.getGroupMemberUsageComparison).mockRejectedValue(new Error('boom'))

  const wrapper = mountModal()
  await flushPromises()

  expect(wrapper.text()).toContain('alice')
  expect(wrapper.text()).toContain('用量加载失败')
})
```

- [ ] **Step 2: Run the component test and verify it fails**

Run:

```bash
cd frontend
pnpm vitest run src/components/admin/group/__tests__/GroupMembersModal.spec.ts
```

Expected: FAIL because the API method and UI do not exist.

- [ ] **Step 3: Add frontend API types and method**

In `frontend/src/api/admin/groups.ts`, import `WindowStats` from `@/types` if it is not already imported:

```ts
import type { WindowStats } from '@/types'
```

Add these exports near `GroupMembersResponse`:

```ts
export interface GroupMemberUsageComparison {
  today: WindowStats
  yesterday: WindowStats
}

export interface GroupMemberUsageComparisonResponse {
  group_id: number
  today: string
  yesterday: string
  stats: Record<string, GroupMemberUsageComparison>
}

export async function getGroupMemberUsageComparison(
  id: number,
  userIds: number[],
  timezone?: string
): Promise<GroupMemberUsageComparisonResponse> {
  const { data } = await apiClient.get<GroupMemberUsageComparisonResponse>(
    `/admin/groups/${id}/members/usage-comparison`,
    {
      params: {
        user_ids: userIds.join(','),
        ...(timezone ? { timezone } : {})
      }
    }
  )
  return data
}
```

Add `getGroupMemberUsageComparison` to `groupsAPI`.

- [ ] **Step 4: Add i18n keys**

In `frontend/src/i18n/locales/zh.ts`, under `admin.groups`, add:

```ts
memberUsageToday: '今天',
memberUsageYesterday: '昨天',
memberUsageLoadFailed: '用量加载失败',
memberUsageNoData: '暂无用量',
```

In `frontend/src/i18n/locales/en.ts`, under `admin.groups`, add:

```ts
memberUsageToday: 'Today',
memberUsageYesterday: 'Yesterday',
memberUsageLoadFailed: 'Failed to load usage',
memberUsageNoData: 'No usage',
```

- [ ] **Step 5: Implement modal state and loading**

In `frontend/src/components/admin/group/GroupMembersModal.vue`, update imports:

```ts
import type { GroupMemberUsageComparison, GroupMembersResponse } from '@/api/admin/groups'
import type { AdminGroup, WindowStats } from '@/types'
```

Add state near `members`:

```ts
const usageLoading = ref(false)
const usageError = ref<string | null>(null)
const usageDates = reactive({ today: '', yesterday: '' })
const usageByUserId = ref<Record<string, GroupMemberUsageComparison>>({})

const emptyWindowStats = (): WindowStats => ({
  requests: 0,
  tokens: 0,
  cost: 0,
  standard_cost: 0,
  user_cost: 0
})
```

Add helpers:

```ts
function resetUsage() {
  usageLoading.value = false
  usageError.value = null
  usageDates.today = ''
  usageDates.yesterday = ''
  usageByUserId.value = {}
}

async function loadUsageComparison(groupId: number) {
  if (!props.group?.is_exclusive || !members.has_fixed_members || members.items.length === 0) {
    resetUsage()
    return
  }

  usageLoading.value = true
  usageError.value = null
  try {
    const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
    const userIds = members.items.map((member) => member.id)
    const data = await adminAPI.groups.getGroupMemberUsageComparison(groupId, userIds, timezone)
    usageDates.today = data.today
    usageDates.yesterday = data.yesterday
    usageByUserId.value = data.stats || {}
  } catch (error) {
    usageByUserId.value = {}
    usageError.value = t('admin.groups.memberUsageLoadFailed')
    console.error('Error loading group member usage comparison:', error)
  } finally {
    usageLoading.value = false
  }
}

function getUsageForUser(userId: number): GroupMemberUsageComparison {
  return usageByUserId.value[String(userId)] || {
    today: emptyWindowStats(),
    yesterday: emptyWindowStats()
  }
}

function formatCompactNumber(value: number): string {
  if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return String(value)
}

function formatTokens(tokens: number): string {
  if (tokens >= 1000000000) return `${(tokens / 1000000000).toFixed(2)}B`
  if (tokens >= 1000000) return `${(tokens / 1000000).toFixed(2)}M`
  if (tokens >= 1000) return `${(tokens / 1000).toFixed(1)}K`
  return String(tokens)
}

function formatMoney(value: number | undefined): string {
  const amount = Number(value || 0)
  if (amount >= 1000) return amount.toFixed(0)
  if (amount >= 100) return amount.toFixed(2)
  return amount.toFixed(2)
}
```

Update `resetMembers()` to call `resetUsage()`.

Update `loadMembers()` so that after assigning members, it calls:

```ts
await loadUsageComparison(groupId)
```

- [ ] **Step 6: Render usage rows**

In the table header of `GroupMembersModal.vue`, add a usage column before the action column:

```vue
<th class="min-w-[260px] px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">
  {{ t('admin.groups.columns.usage') }}
</th>
```

In each row, add this cell before the action cell:

```vue
<td class="px-3 py-2">
  <div v-if="usageLoading" class="space-y-1">
    <div class="h-5 w-56 animate-pulse rounded bg-gray-100 dark:bg-dark-600"></div>
    <div class="h-5 w-52 animate-pulse rounded bg-gray-100 dark:bg-dark-600"></div>
  </div>
  <div v-else-if="usageError" class="text-xs text-amber-600 dark:text-amber-400">
    {{ usageError }}
  </div>
  <div v-else class="space-y-1 text-xs">
    <div class="flex flex-wrap items-center gap-1.5">
      <span class="w-10 font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.memberUsageToday') }}</span>
      <span class="text-gray-400">{{ usageDates.today }}</span>
      <span class="rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatCompactNumber(getUsageForUser(member.id).today.requests) }} req</span>
      <span class="rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatTokens(getUsageForUser(member.id).today.tokens) }} token</span>
      <span class="rounded bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">A ${{ formatMoney(getUsageForUser(member.id).today.cost) }}</span>
      <span class="rounded bg-sky-50 px-1.5 py-0.5 font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">U ${{ formatMoney(getUsageForUser(member.id).today.user_cost) }}</span>
    </div>
    <div class="flex flex-wrap items-center gap-1.5">
      <span class="w-10 font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.memberUsageYesterday') }}</span>
      <span class="text-gray-400">{{ usageDates.yesterday }}</span>
      <span class="rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatCompactNumber(getUsageForUser(member.id).yesterday.requests) }} req</span>
      <span class="rounded bg-gray-100 px-1.5 py-0.5 font-medium text-gray-700 dark:bg-dark-600 dark:text-gray-300">{{ formatTokens(getUsageForUser(member.id).yesterday.tokens) }} token</span>
      <span class="rounded bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300">A ${{ formatMoney(getUsageForUser(member.id).yesterday.cost) }}</span>
      <span class="rounded bg-sky-50 px-1.5 py-0.5 font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">U ${{ formatMoney(getUsageForUser(member.id).yesterday.user_cost) }}</span>
    </div>
  </div>
</td>
```

- [ ] **Step 7: Run the component test and verify it passes**

Run:

```bash
cd frontend
pnpm vitest run src/components/admin/group/__tests__/GroupMembersModal.spec.ts
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/api/admin/groups.ts frontend/src/components/admin/group/GroupMembersModal.vue frontend/src/components/admin/group/__tests__/GroupMembersModal.spec.ts frontend/src/i18n/locales/zh.ts frontend/src/i18n/locales/en.ts
git commit -m "feat: show exclusive group member usage comparison"
```

---

### Task 4: Hide Non-Exclusive Entry And Run Full Verification

**Files:**
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Test: existing frontend group tests if available; otherwise targeted typecheck/build.

**Interfaces:**
- Consumes:
  - Existing `AdminGroup.is_exclusive`.
- Produces:
  - Non-exclusive groups no longer show the “查看用户” action.

- [ ] **Step 1: Update the action button condition**

In `frontend/src/views/admin/GroupsView.vue`, wrap the “查看用户” button:

```vue
<button
  v-if="row.is_exclusive"
  @click="handleViewMembers(row)"
  class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-sky-50 hover:text-sky-600 dark:hover:bg-dark-700 dark:hover:text-sky-400"
>
  <Icon name="users" size="sm" />
  <span class="text-xs">{{ t("admin.groups.viewMembers") }}</span>
</button>
```

- [ ] **Step 2: Verify the template condition through typecheck and smoke**

Do not create a new `GroupsView` test harness for this one-line template guard. The existing group page does not have a focused unit spec, and a new full-page route/store harness would be larger than the behavior under test. Verification for this task is the frontend typecheck/build plus the browser smoke checklist below.

- [ ] **Step 3: Run frontend tests**

Run:

```bash
cd frontend
pnpm vitest run src/components/admin/group/__tests__/GroupMembersModal.spec.ts
```

Expected: PASS.

- [ ] **Step 4: Run backend targeted tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestDashboardService_GetGroupUserUsageComparison'
GOCACHE=/tmp/sub2api-go-cache go test -tags=integration ./internal/repository -run TestUsageLogRepoSuite/TestGetGroupUserDailyStatsBatch_GroupUserAndWindowIsolation
```

Expected: both PASS.

- [ ] **Step 5: Run typecheck/build checks**

Run:

```bash
cd frontend
pnpm typecheck
```

Expected: PASS. If `pnpm typecheck` is not defined, run:

```bash
cd frontend
pnpm build
```

Expected: PASS.

- [ ] **Step 6: Browser smoke check**

Start the dev server:

```bash
cd frontend
pnpm dev --host 127.0.0.1
```

Open the admin groups page and verify:

- Public groups do not show “查看用户”.
- Exclusive groups show “查看用户”.
- The members modal shows each member with 今天 and 昨天 usage rows.
- Long metric text wraps inside the usage column without overlapping action buttons.

Expected: the server starts and the checks above pass. If the server cannot start because dependencies are missing or the port is unavailable, record the exact error in the final implementation summary and rely on Steps 3-5 for automated verification.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/admin/GroupsView.vue
git commit -m "fix: limit group member view to exclusive groups"
```

- [ ] **Step 8: Final verification before completion**

Run:

```bash
git status --short
git log --oneline -5
```

Expected:

- Worktree is clean, or only unrelated user changes remain.
- Recent commits include the repository aggregation, API endpoint, frontend usage display, and exclusive-only action commits.
