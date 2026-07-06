# Group Upstream Price Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build group-level upstream balance refresh and price multiplier guard so enabled groups periodically refresh all member upstream accounts and pause only price-guard-blocked accounts when upstream price exceeds the group limit.

**Architecture:** Add group persistence fields, expose them through existing admin group DTOs/forms, then add a focused runner that scans enabled groups and calls the existing `OpenAIUpstreamBalanceService.Refresh`. Price guard state is stored in account `extra`; scheduling is blocked via existing `temp_unschedulable_until/reason` so manual `schedulable=false` remains untouched.

**Tech Stack:** Go, Ent, PostgreSQL migrations, Gin handlers, Vue 3, TypeScript, Vitest.

## Global Constraints

- Default language for user-facing status is Simplified Chinese.
- Existing project style, Ent schema, repository, service, DTO, and Vue form patterns must be followed.
- New group fields default disabled or unlimited so existing groups keep current behavior.
- `upstream_price_max_multiplier=0` means no price blocking.
- Price recovery may only clear `temp_unschedulable_reason` values beginning with `upstream_price_guard:`.
- No change to user billing rate, API Key quota, or usage billing semantics.

---

## File Structure

- `backend/migrations/160_group_upstream_price_guard.sql`: add group columns.
- `backend/ent/schema/group.go`: add Ent fields for the new group columns.
- `backend/internal/service/group.go`: add service-level `Group` fields and validation helper.
- `backend/internal/service/admin_service.go`: validate and copy new fields during create/update.
- `backend/internal/repository/group_repo.go`: persist and hydrate group fields.
- `backend/internal/handler/dto/types.go` and `backend/internal/handler/dto/mappers.go`: expose new fields in admin group responses.
- `backend/internal/handler/admin/group_handler.go`: accept new fields in create/update payloads.
- `backend/internal/service/account.go`: add constants/helpers for price guard status keys and reason prefix.
- `backend/internal/service/openai_upstream_price_guard.go`: focused service helpers for applying and clearing price guard status.
- `backend/internal/service/group_upstream_balance_refresh_runner.go`: new group-scoped runner.
- `backend/internal/repository/account_repo.go`: add group-scoped balance refresh candidate query.
- `backend/internal/service/wire.go`, `backend/cmd/server/wire.go`, `backend/cmd/server/wire_gen.go`, `backend/cmd/server/wire_gen_test.go`: wire runner construction/start/stop.
- `frontend/src/types/index.ts`: add group and account extra fields.
- `frontend/src/views/admin/GroupsView.vue`: add form fields and validation.
- `frontend/src/views/admin/AccountsView.vue`: show price guard state near upstream group/rate.
- Tests beside touched code: focused Go unit tests and Vitest specs.

---

### Task 1: Persist Group-Level Refresh And Price Guard Config

**Files:**
- Create: `backend/migrations/160_group_upstream_price_guard.sql`
- Modify: `backend/ent/schema/group.go`
- Modify: `backend/internal/service/group.go`
- Modify: `backend/internal/repository/group_repo.go`
- Modify: `backend/internal/handler/dto/types.go`
- Modify: `backend/internal/handler/dto/mappers.go`
- Modify: `backend/internal/handler/admin/group_handler.go`
- Modify: `backend/internal/service/admin_service.go`
- Test: `backend/ent/schema/openai_auto_scheduler_schema_test.go`
- Test: `backend/internal/repository/migrations_schema_integration_test.go`
- Test: `backend/internal/service/admin_service_group_test.go`

**Interfaces:**
- Produces: `Group.UpstreamBalanceRefreshEnabled bool`
- Produces: `Group.UpstreamBalanceRefreshIntervalSeconds int`
- Produces: `Group.UpstreamPriceMaxMultiplier float64`

- [ ] **Step 1: Write failing schema and service tests**

Add assertions to `backend/ent/schema/openai_auto_scheduler_schema_test.go`:

```go
func TestGroupUpstreamPriceGuardSchemaFields(t *testing.T) {
	schemas := []ent.Interface{
		Group{},
	}
	group := requireSchema(t, schemas, "Group")
	requireSchemaFields(t, group,
		"upstream_balance_refresh_enabled",
		"upstream_balance_refresh_interval_seconds",
		"upstream_price_max_multiplier",
	)
}
```

Add migration checks to `backend/internal/repository/migrations_schema_integration_test.go`:

```go
requireColumn(t, tx, "groups", "upstream_balance_refresh_enabled", "boolean", 0, false)
requireColumn(t, tx, "groups", "upstream_balance_refresh_interval_seconds", "integer", 0, false)
requireColumn(t, tx, "groups", "upstream_price_max_multiplier", "numeric", 0, false)
```

Add admin validation cases to `backend/internal/service/admin_service_group_test.go`:

```go
func TestValidateGroupUpstreamPriceGuardConfig(t *testing.T) {
	require.NoError(t, ValidateGroupUpstreamPriceGuardConfig(false, 0, 0))
	require.NoError(t, ValidateGroupUpstreamPriceGuardConfig(true, 600, 0.08))
	require.ErrorContains(t, ValidateGroupUpstreamPriceGuardConfig(true, 59, 0), "upstream_balance_refresh_interval_seconds")
	require.ErrorContains(t, ValidateGroupUpstreamPriceGuardConfig(false, 600, -0.1), "upstream_price_max_multiplier")
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository ./internal/service -run 'TestGroupUpstreamPriceGuardSchemaFields|TestMigrationSchema|TestValidateGroupUpstreamPriceGuardConfig'
```

Expected: fail because fields and validation helper do not exist.

- [ ] **Step 3: Add migration**

Create `backend/migrations/160_group_upstream_price_guard.sql`:

```sql
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_balance_refresh_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_balance_refresh_interval_seconds INTEGER NOT NULL DEFAULT 600;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS upstream_price_max_multiplier DECIMAL(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN groups.upstream_balance_refresh_enabled IS '是否启用分组级上游余额定时刷新';
COMMENT ON COLUMN groups.upstream_balance_refresh_interval_seconds IS '分组级上游余额刷新间隔秒数';
COMMENT ON COLUMN groups.upstream_price_max_multiplier IS '分组级上游价格倍率上限，0 表示不限制';
```

- [ ] **Step 4: Add Ent and service fields**

In `backend/ent/schema/group.go`, add fields near `openai_auto_scheduler_enabled`:

```go
field.Bool("upstream_balance_refresh_enabled").
	Default(false).
	Comment("是否启用分组级上游余额定时刷新"),
field.Int("upstream_balance_refresh_interval_seconds").
	Default(600).
	Comment("分组级上游余额刷新间隔秒数"),
field.Float("upstream_price_max_multiplier").
	SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}).
	Default(0).
	Comment("分组级上游价格倍率上限，0 表示不限制"),
```

In `backend/internal/service/group.go`, add fields:

```go
UpstreamBalanceRefreshEnabled         bool
UpstreamBalanceRefreshIntervalSeconds int
UpstreamPriceMaxMultiplier            float64
```

Add helper:

```go
const DefaultUpstreamBalanceRefreshIntervalSeconds = 600
const MinUpstreamBalanceRefreshIntervalSeconds = 60

func ValidateGroupUpstreamPriceGuardConfig(enabled bool, intervalSeconds int, maxMultiplier float64) error {
	if maxMultiplier < 0 {
		return errors.New("upstream_price_max_multiplier must be >= 0")
	}
	if !enabled {
		return nil
	}
	if intervalSeconds < MinUpstreamBalanceRefreshIntervalSeconds {
		return fmt.Errorf("upstream_balance_refresh_interval_seconds must be >= %d", MinUpstreamBalanceRefreshIntervalSeconds)
	}
	return nil
}
```

- [ ] **Step 5: Wire persistence and DTOs**

In `backend/internal/repository/group_repo.go`, add setters in create/update:

```go
SetUpstreamBalanceRefreshEnabled(groupIn.UpstreamBalanceRefreshEnabled).
SetUpstreamBalanceRefreshIntervalSeconds(groupIn.UpstreamBalanceRefreshIntervalSeconds).
SetUpstreamPriceMaxMultiplier(groupIn.UpstreamPriceMaxMultiplier)
```

In `groupEntityToService`, map:

```go
UpstreamBalanceRefreshEnabled:         m.UpstreamBalanceRefreshEnabled,
UpstreamBalanceRefreshIntervalSeconds: m.UpstreamBalanceRefreshIntervalSeconds,
UpstreamPriceMaxMultiplier:            m.UpstreamPriceMaxMultiplier,
```

In `backend/internal/handler/dto/types.go`, add JSON fields to `Group`:

```go
UpstreamBalanceRefreshEnabled         bool    `json:"upstream_balance_refresh_enabled"`
UpstreamBalanceRefreshIntervalSeconds int     `json:"upstream_balance_refresh_interval_seconds"`
UpstreamPriceMaxMultiplier            float64 `json:"upstream_price_max_multiplier"`
```

In `backend/internal/handler/dto/mappers.go`, map the same fields from `service.Group`.

- [ ] **Step 6: Accept create/update payloads**

In `backend/internal/handler/admin/group_handler.go`, add to `CreateGroupRequest`:

```go
UpstreamBalanceRefreshEnabled         bool    `json:"upstream_balance_refresh_enabled"`
UpstreamBalanceRefreshIntervalSeconds int     `json:"upstream_balance_refresh_interval_seconds"`
UpstreamPriceMaxMultiplier            float64 `json:"upstream_price_max_multiplier"`
```

Add to `UpdateGroupRequest`:

```go
UpstreamBalanceRefreshEnabled         *bool    `json:"upstream_balance_refresh_enabled"`
UpstreamBalanceRefreshIntervalSeconds *int     `json:"upstream_balance_refresh_interval_seconds"`
UpstreamPriceMaxMultiplier            *float64 `json:"upstream_price_max_multiplier"`
```

Pass them into `service.CreateGroupInput` and `service.UpdateGroupInput`.

In `backend/internal/service/admin_service.go`, add fields to inputs, validate during create/update, and copy onto `Group`. Use default `600` when create receives enabled false and interval zero.

- [ ] **Step 7: Run tests and commit**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository ./internal/service -run 'TestGroupUpstreamPriceGuardSchemaFields|TestMigrationSchema|TestValidateGroupUpstreamPriceGuardConfig|TestAdminService.*Group'
```

Expected: pass.

Commit:

```bash
git add backend/migrations/160_group_upstream_price_guard.sql backend/ent/schema/group.go backend/internal/service/group.go backend/internal/repository/group_repo.go backend/internal/handler/dto/types.go backend/internal/handler/dto/mappers.go backend/internal/handler/admin/group_handler.go backend/internal/service/admin_service.go backend/ent/schema/openai_auto_scheduler_schema_test.go backend/internal/repository/migrations_schema_integration_test.go backend/internal/service/admin_service_group_test.go
git commit -m "feat: add group upstream price guard config"
```

---

### Task 2: Add Price Guard Policy Helpers

**Files:**
- Create: `backend/internal/service/openai_upstream_price_guard.go`
- Test: `backend/internal/service/openai_upstream_price_guard_test.go`
- Modify: `backend/internal/service/account.go`

**Interfaces:**
- Consumes: `AccountRepository.SetTempUnschedulable(ctx, id, until, reason)`
- Consumes: `AccountRepository.ClearTempUnschedulable(ctx, id)`
- Produces: `ApplyGroupUpstreamPriceGuard(ctx context.Context, repo AccountRepository, account *Account, group Group, now time.Time) error`
- Produces: `UpstreamPriceGuardReasonPrefix = "upstream_price_guard:"`

- [ ] **Step 1: Write failing policy tests**

Create `backend/internal/service/openai_upstream_price_guard_test.go`:

```go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamPriceGuardRepoStub struct {
	AccountRepository
	setUntil    time.Time
	setReason   string
	clearCalled bool
	updates     map[string]any
}

func (r *upstreamPriceGuardRepoStub) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.setUntil = until
	r.setReason = reason
	return nil
}

func (r *upstreamPriceGuardRepoStub) ClearTempUnschedulable(context.Context, int64) error {
	r.clearCalled = true
	return nil
}

func (r *upstreamPriceGuardRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = updates
	return nil
}

func TestApplyGroupUpstreamPriceGuard_BlocksWhenPriceExceedsLimit(t *testing.T) {
	price := 0.12
	account := &Account{ID: 7, ChannelPrice: &price}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, now)

	require.NoError(t, err)
	require.Contains(t, repo.setReason, UpstreamPriceGuardReasonPrefix)
	require.Equal(t, "blocked", repo.updates["upstream_price_guard_status"])
	require.Equal(t, int64(3), repo.updates["upstream_price_guard_group_id"])
}

func TestApplyGroupUpstreamPriceGuard_ClearsOnlyOwnReasonWhenPriceRecovers(t *testing.T) {
	price := 0.06
	account := &Account{ID: 7, ChannelPrice: &price, TempUnschedulableReason: UpstreamPriceGuardReasonPrefix + " group_id=3"}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.True(t, repo.clearCalled)
	require.Equal(t, "ok", repo.updates["upstream_price_guard_status"])
}

func TestApplyGroupUpstreamPriceGuard_DoesNotClearOtherTempReason(t *testing.T) {
	price := 0.06
	account := &Account{ID: 7, ChannelPrice: &price, TempUnschedulableReason: "token refresh retry exhausted"}
	group := Group{ID: 3, UpstreamPriceMaxMultiplier: 0.08}
	repo := &upstreamPriceGuardRepoStub{}

	err := ApplyGroupUpstreamPriceGuard(context.Background(), repo, account, group, time.Now())

	require.NoError(t, err)
	require.False(t, repo.clearCalled)
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyGroupUpstreamPriceGuard'
```

Expected: fail because helper and constants do not exist.

- [ ] **Step 3: Implement helper**

Create `backend/internal/service/openai_upstream_price_guard.go`:

```go
package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	UpstreamPriceGuardReasonPrefix = "upstream_price_guard:"
	upstreamPriceGuardBlockTTL     = 24 * time.Hour
)

func ApplyGroupUpstreamPriceGuard(ctx context.Context, repo AccountRepository, account *Account, group Group, now time.Time) error {
	if repo == nil || account == nil || account.ID <= 0 {
		return nil
	}
	actual := account.EffectiveChannelPrice()
	updates := map[string]any{
		"upstream_price_guard_group_id":          group.ID,
		"upstream_price_guard_max_multiplier":    group.UpstreamPriceMaxMultiplier,
		"upstream_price_guard_actual_multiplier": actual,
		"upstream_price_guard_checked_at":        now.UTC().Format(time.RFC3339),
		"upstream_price_guard_error":             "",
	}
	if group.UpstreamPriceMaxMultiplier <= 0 {
		updates["upstream_price_guard_status"] = "ok"
		return repo.UpdateExtra(ctx, account.ID, updates)
	}
	if account.ChannelPrice == nil || actual <= 0 {
		updates["upstream_price_guard_status"] = "unsupported"
		updates["upstream_price_guard_error"] = "missing upstream effective multiplier"
		return repo.UpdateExtra(ctx, account.ID, updates)
	}
	if actual > group.UpstreamPriceMaxMultiplier {
		updates["upstream_price_guard_status"] = "blocked"
		if err := repo.UpdateExtra(ctx, account.ID, updates); err != nil {
			return err
		}
		reason := fmt.Sprintf("%s group_id=%d actual=%.6f max=%.6f", UpstreamPriceGuardReasonPrefix, group.ID, actual, group.UpstreamPriceMaxMultiplier)
		return repo.SetTempUnschedulable(ctx, account.ID, now.Add(upstreamPriceGuardBlockTTL), reason)
	}
	updates["upstream_price_guard_status"] = "ok"
	if err := repo.UpdateExtra(ctx, account.ID, updates); err != nil {
		return err
	}
	if strings.HasPrefix(strings.TrimSpace(account.TempUnschedulableReason), UpstreamPriceGuardReasonPrefix) {
		return repo.ClearTempUnschedulable(ctx, account.ID)
	}
	return nil
}
```

- [ ] **Step 4: Run tests and commit**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestApplyGroupUpstreamPriceGuard'
```

Expected: pass.

Commit:

```bash
git add backend/internal/service/account.go backend/internal/service/openai_upstream_price_guard.go backend/internal/service/openai_upstream_price_guard_test.go
git commit -m "feat: add upstream price guard policy"
```

---

### Task 3: Add Group-Scoped Refresh Runner

**Files:**
- Create: `backend/internal/service/group_upstream_balance_refresh_runner.go`
- Test: `backend/internal/service/group_upstream_balance_refresh_runner_test.go`
- Modify: `backend/internal/repository/account_repo.go`
- Test: `backend/internal/repository/account_repo_checkin_candidates_test.go`
- Modify: `backend/internal/service/account.go`

**Interfaces:**
- Consumes: `OpenAIUpstreamBalanceService.Refresh(ctx, accountID) (*Account, error)`
- Consumes: `ApplyGroupUpstreamPriceGuard(ctx, repo, account, group, now)`
- Produces: `ListUpstreamBalanceRefreshCandidatesByGroupID(ctx context.Context, groupID int64, limit int) ([]Account, error)`
- Produces: `ListUpstreamBalanceRefreshEnabledGroups(ctx context.Context) ([]Group, error)` on `GroupRepository`

- [ ] **Step 1: Write failing runner tests**

Create `backend/internal/service/group_upstream_balance_refresh_runner_test.go` with a fake repo and fake balance service wrapper. Keep `runOnce` package-visible for direct testing:

```go
func TestGroupUpstreamBalanceRefreshRunner_RunOnceRefreshesGroupAccounts(t *testing.T) {
	group := Group{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600, UpstreamPriceMaxMultiplier: 0.08}
	price := 0.06
	account := Account{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ChannelPrice: &price}
	repo := &groupUpstreamRefreshRepoStub{groups: []Group{group}, accounts: map[int64][]Account{10: {account}}}
	balance := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{20: &account}}
	runner := NewGroupUpstreamBalanceRefreshRunner(repo, repo, balance)

	runner.runOnce(context.Background(), time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC))

	require.Equal(t, []int64{20}, balance.calls)
	require.Equal(t, "ok", repo.extraUpdates[20]["upstream_price_guard_status"])
}

func TestGroupUpstreamBalanceRefreshRunner_RespectsGroupInterval(t *testing.T) {
	group := Group{ID: 10, Status: StatusActive, UpstreamBalanceRefreshEnabled: true, UpstreamBalanceRefreshIntervalSeconds: 600}
	repo := &groupUpstreamRefreshRepoStub{groups: []Group{group}, accounts: map[int64][]Account{10: {{ID: 20, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}}}}
	balance := &groupUpstreamBalanceStub{refreshed: map[int64]*Account{20: {ID: 20}}}
	runner := NewGroupUpstreamBalanceRefreshRunner(repo, repo, balance)
	now := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)

	runner.runOnce(context.Background(), now)
	runner.runOnce(context.Background(), now.Add(time.Minute))

	require.Equal(t, []int64{20}, balance.calls)
}
```

- [ ] **Step 2: Write failing repository test**

In `backend/internal/repository/account_repo_checkin_candidates_test.go`, add a SQL matcher test for group-scoped candidates:

```go
func TestListUpstreamBalanceRefreshCandidatesByGroupID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var capturedSQL string
	mock.ExpectQuery("SELECT a.id").
		WillReturnRows(sqlmock.NewRows([]string{"id"})).
		WillDelayFor(0)

	repo := &accountRepository{sql: captureQuerySQL{db: db, captured: &capturedSQL}}

	accounts, err := repo.ListUpstreamBalanceRefreshCandidatesByGroupID(context.Background(), 42, 25)
	require.NoError(t, err)
	require.Empty(t, accounts)

	normalized := normalizeSQLWhitespace(capturedSQL)
	require.Contains(t, normalized, "FROM accounts a")
	require.Contains(t, normalized, "JOIN account_groups ag ON ag.account_id = a.id")
	require.Contains(t, normalized, "ag.group_id = $1")
	require.Contains(t, normalized, "a.deleted_at IS NULL")
	require.Contains(t, normalized, "a.status = 'active'")
	require.Contains(t, normalized, "a.type = 'apikey'")
	require.Contains(t, normalized, "a.platform IN ('openai', 'anthropic')")
	require.Contains(t, normalized, "a.credentials ? 'api_key'")
	require.Contains(t, normalized, "ORDER BY a.priority ASC, a.id ASC")
	require.Contains(t, normalized, "LIMIT $2")
	require.NoError(t, mock.ExpectationsWereMet())
}
```

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'TestGroupUpstreamBalanceRefreshRunner|TestListUpstreamBalanceRefreshCandidatesByGroupID'
```

Expected: fail because runner and repository methods do not exist.

- [ ] **Step 4: Implement repository methods**

Add to the existing `AccountRepository` interface in `backend/internal/service/admin_service.go`:

```go
ListUpstreamBalanceRefreshCandidatesByGroupID(ctx context.Context, groupID int64, limit int) ([]Account, error)
```

In `backend/internal/repository/account_repo.go`, implement with SQL and reuse `GetByIDs` so hydration stays consistent:

```go
func (r *accountRepository) ListUpstreamBalanceRefreshCandidatesByGroupID(ctx context.Context, groupID int64, limit int) ([]service.Account, error) {
	if r.sql == nil {
		return nil, errors.New("account repository SQL executor not configured")
	}
	query := `
		SELECT a.id
		FROM accounts a
		JOIN account_groups ag ON ag.account_id = a.id
		WHERE a.deleted_at IS NULL
			AND ag.deleted_at IS NULL
			AND ag.group_id = $1
			AND a.status = 'active'
			AND a.type = 'apikey'
			AND a.platform IN ('openai', 'anthropic')
			AND a.credentials ? 'api_key'
		ORDER BY a.priority ASC, a.id ASC
	`
	var (
		rows *sql.Rows
		err  error
	)
	if limit > 0 {
		query += " LIMIT $2"
		rows, err = r.sql.QueryContext(ctx, query, groupID, limit)
	} else {
		rows, err = r.sql.QueryContext(ctx, query, groupID)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	accounts, err := r.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account != nil {
			out = append(out, *account)
		}
	}
	return out, nil
}
```

Add to `GroupRepository`:

```go
ListUpstreamBalanceRefreshEnabled(ctx context.Context) ([]Group, error)
```

Implement in `backend/internal/repository/group_repo.go` by querying active groups with `upstream_balance_refresh_enabled=true`, ordered by `sort_order,id`.

- [ ] **Step 5: Implement runner**

Create `backend/internal/service/group_upstream_balance_refresh_runner.go`:

```go
package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const groupUpstreamBalanceRefreshScanInterval = time.Minute
const groupUpstreamBalanceRefreshCandidateLimit = 0

type groupUpstreamBalanceGroupRepo interface {
	ListUpstreamBalanceRefreshEnabled(ctx context.Context) ([]Group, error)
}

type groupUpstreamBalanceAccountRepo interface {
	ListUpstreamBalanceRefreshCandidatesByGroupID(ctx context.Context, groupID int64, limit int) ([]Account, error)
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
	SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error
	ClearTempUnschedulable(ctx context.Context, id int64) error
}

type groupUpstreamBalanceRefresher interface {
	Refresh(ctx context.Context, accountID int64) (*Account, error)
}

type GroupUpstreamBalanceRefreshRunner struct {
	groupRepo   groupUpstreamBalanceGroupRepo
	accountRepo groupUpstreamBalanceAccountRepo
	refresher   groupUpstreamBalanceRefresher
	lastRun     map[int64]time.Time
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
}

func NewGroupUpstreamBalanceRefreshRunner(groupRepo groupUpstreamBalanceGroupRepo, accountRepo groupUpstreamBalanceAccountRepo, refresher groupUpstreamBalanceRefresher) *GroupUpstreamBalanceRefreshRunner {
	return &GroupUpstreamBalanceRefreshRunner{groupRepo: groupRepo, accountRepo: accountRepo, refresher: refresher, lastRun: map[int64]time.Time{}, stopCh: make(chan struct{})}
}

func (r *GroupUpstreamBalanceRefreshRunner) Start() {
	if r == nil || r.groupRepo == nil || r.accountRepo == nil || r.refresher == nil {
		return
	}
	r.wg.Add(1)
	go r.loop()
}

func (r *GroupUpstreamBalanceRefreshRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stopCh) })
	r.wg.Wait()
}

func (r *GroupUpstreamBalanceRefreshRunner) loop() {
	defer r.wg.Done()
	ticker := time.NewTicker(groupUpstreamBalanceRefreshScanInterval)
	defer ticker.Stop()
	r.runOnce(context.Background(), time.Now())
	for {
		select {
		case <-ticker.C:
			r.runOnce(context.Background(), time.Now())
		case <-r.stopCh:
			return
		}
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) runOnce(ctx context.Context, now time.Time) {
	groups, err := r.groupRepo.ListUpstreamBalanceRefreshEnabled(ctx)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_groups_failed", "error", err)
		return
	}
	for _, group := range groups {
		interval := time.Duration(group.UpstreamBalanceRefreshIntervalSeconds) * time.Second
		if interval <= 0 {
			interval = DefaultUpstreamBalanceRefreshIntervalSeconds * time.Second
		}
		if last, ok := r.lastRun[group.ID]; ok && now.Sub(last) < interval {
			continue
		}
		r.lastRun[group.ID] = now
		r.refreshGroup(ctx, group, now)
	}
}

func (r *GroupUpstreamBalanceRefreshRunner) refreshGroup(ctx context.Context, group Group, now time.Time) {
	accounts, err := r.accountRepo.ListUpstreamBalanceRefreshCandidatesByGroupID(ctx, group.ID, groupUpstreamBalanceRefreshCandidateLimit)
	if err != nil {
		slog.Warn("group_upstream_balance_refresh.list_accounts_failed", "group_id", group.ID, "error", err)
		return
	}
	for i := range accounts {
		refreshed, err := r.refresher.Refresh(ctx, accounts[i].ID)
		if err != nil {
			slog.Warn("group_upstream_balance_refresh.refresh_failed", "group_id", group.ID, "account_id", accounts[i].ID, "error", err)
			continue
		}
		if err := ApplyGroupUpstreamPriceGuard(ctx, r.accountRepo, refreshed, group, now); err != nil {
			slog.Warn("group_upstream_balance_refresh.price_guard_failed", "group_id", group.ID, "account_id", accounts[i].ID, "error", err)
		}
	}
}
```

- [ ] **Step 6: Run tests and commit**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository -run 'TestGroupUpstreamBalanceRefreshRunner|TestListUpstreamBalanceRefreshCandidatesByGroupID'
```

Expected: pass.

Commit:

```bash
git add backend/internal/service/group_upstream_balance_refresh_runner.go backend/internal/service/group_upstream_balance_refresh_runner_test.go backend/internal/repository/account_repo.go backend/internal/repository/account_repo_checkin_candidates_test.go backend/internal/service/admin_service.go backend/internal/repository/group_repo.go
git commit -m "feat: refresh upstream balances by group"
```

---

### Task 4: Wire Runner Startup

**Files:**
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/cmd/server/wire.go`
- Modify: `backend/cmd/server/wire_gen.go`
- Modify: `backend/cmd/server/wire_gen_test.go`
- Test: `backend/internal/service/wire_test.go`

**Interfaces:**
- Consumes: `NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, upstreamBalanceService)`
- Produces: `ProvideGroupUpstreamBalanceRefreshRunner(...) *GroupUpstreamBalanceRefreshRunner`

- [ ] **Step 1: Write failing provider test**

In `backend/internal/service/wire_test.go`, add:

```go
func TestProvideGroupUpstreamBalanceRefreshRunner_StartsWorker(t *testing.T) {
	groupRepo := &groupUpstreamBalanceProviderGroupRepoStub{called: make(chan struct{}, 1)}
	accountRepo := &groupUpstreamBalanceProviderAccountRepoStub{}
	balance := &groupUpstreamBalanceProviderRefreshStub{}

	svc := ProvideGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, balance)
	requireNotNil(t, svc)
	t.Cleanup(func() { svc.Stop() })

	select {
	case <-groupRepo.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected group upstream balance worker to query groups after provider start")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestProvideGroupUpstreamBalanceRefreshRunner'
```

Expected: fail because provider does not exist.

- [ ] **Step 3: Implement provider**

In `backend/internal/service/wire.go`, add:

```go
func ProvideGroupUpstreamBalanceRefreshRunner(groupRepo GroupRepository, accountRepo AccountRepository, upstreamBalanceService *OpenAIUpstreamBalanceService) *GroupUpstreamBalanceRefreshRunner {
	svc := NewGroupUpstreamBalanceRefreshRunner(groupRepo, accountRepo, upstreamBalanceService)
	svc.Start()
	return svc
}
```

Add provider to `ProviderSet`.

- [ ] **Step 4: Wire cleanup**

In `backend/cmd/server/wire.go` and generated `backend/cmd/server/wire_gen.go`, add `groupUpstreamBalanceRefreshRunner *service.GroupUpstreamBalanceRefreshRunner` to `provideCleanup` parameters and add cleanup step:

```go
{"GroupUpstreamBalanceRefreshRunner", func() error {
	if groupUpstreamBalanceRefreshRunner != nil {
		groupUpstreamBalanceRefreshRunner.Stop()
	}
	return nil
}},
```

In `initializeApplication` wiring in `wire_gen.go`, construct:

```go
groupUpstreamBalanceRefreshRunner := service.ProvideGroupUpstreamBalanceRefreshRunner(groupRepository, accountRepository, openAIUpstreamBalanceService)
```

Pass it into `provideCleanup`.

- [ ] **Step 5: Keep manual refresh unchanged**

Do not modify `backend/internal/handler/admin/account_handler.go` in this task. Manual `POST /admin/accounts/:id/upstream-balance/refresh` remains balance-only; group price guard is enforced by the group runner on the next scheduled group refresh. This keeps the handler dependency graph unchanged.

- [ ] **Step 6: Run wire-related tests and commit**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server ./internal/service -run 'TestProvideCleanup|TestProvideGroupUpstreamBalanceRefreshRunner'
```

Expected: pass.

Commit:

```bash
git add backend/internal/service/wire.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go backend/cmd/server/wire_gen_test.go backend/internal/service/wire_test.go
git commit -m "feat: start group upstream balance refresh runner"
```

---

### Task 5: Add Admin UI Fields And Account Price Guard Display

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/utils/upstreamPriceGuard.ts`
- Test: `frontend/src/utils/__tests__/upstreamPriceGuard.spec.ts`
- Modify: `frontend/src/views/admin/GroupsView.vue`
- Modify: `frontend/src/views/admin/AccountsView.vue`
- Test: `frontend/src/views/admin/__tests__/GroupsView.spec.ts`

**Interfaces:**
- Consumes: `Group.upstream_balance_refresh_enabled`
- Consumes: `Group.upstream_balance_refresh_interval_seconds`
- Consumes: `Group.upstream_price_max_multiplier`
- Consumes account extra keys `upstream_price_guard_status`, `upstream_price_guard_actual_multiplier`, `upstream_price_guard_max_multiplier`, `upstream_price_guard_checked_at`, `upstream_price_guard_error`

- [ ] **Step 1: Write failing utility and UI tests**

Create `frontend/src/utils/__tests__/upstreamPriceGuard.spec.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { getUpstreamPriceGuardLabel } from '../upstreamPriceGuard'

describe('getUpstreamPriceGuardLabel', () => {
  it('formats blocked price guard status', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'blocked',
      upstream_price_guard_actual_multiplier: 0.12,
      upstream_price_guard_max_multiplier: 0.08
    })).toBe('价格超限 0.12x > 0.08x')
  })

  it('hides ok and empty statuses', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'ok' })).toBe('')
    expect(getUpstreamPriceGuardLabel({})).toBe('')
  })

  it('formats unsupported and error states', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'unsupported' })).toBe('价格未知')
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'error' })).toBe('价格检查失败')
  })
})
```

In `frontend/src/views/admin/__tests__/GroupsView.spec.ts`, add this assertion to the create payload test after form submission:

```ts
expect(submittedPayload).toMatchObject({
  upstream_balance_refresh_enabled: true,
  upstream_balance_refresh_interval_seconds: 600,
  upstream_price_max_multiplier: 0.08
})
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
cd frontend && pnpm test -- --run GroupsView upstreamPriceGuard
```

Expected: fail because fields/helpers do not exist.

- [ ] **Step 3: Update TypeScript types**

In `frontend/src/types/index.ts`, add to `Group`:

```ts
upstream_balance_refresh_enabled: boolean
upstream_balance_refresh_interval_seconds: number
upstream_price_max_multiplier: number
```

Add to create/update requests as optional fields:

```ts
upstream_balance_refresh_enabled?: boolean
upstream_balance_refresh_interval_seconds?: number
upstream_price_max_multiplier?: number
```

Add account extra keys:

```ts
upstream_price_guard_status?: 'ok' | 'blocked' | 'unsupported' | 'error' | string
upstream_price_guard_group_id?: number
upstream_price_guard_max_multiplier?: number
upstream_price_guard_actual_multiplier?: number
upstream_price_guard_checked_at?: string
upstream_price_guard_error?: string
```

- [ ] **Step 4: Add group form controls**

In `frontend/src/views/admin/GroupsView.vue`, initialize form defaults:

```ts
upstream_balance_refresh_enabled: false,
upstream_balance_refresh_interval_seconds: 600,
upstream_price_max_multiplier: 0,
```

Add controls near scheduling/OpenAI group settings:

```vue
<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
  <label class="flex items-center gap-2">
    <input v-model="groupForm.upstream_balance_refresh_enabled" type="checkbox" />
    <span>{{ t('admin.groups.form.upstreamBalanceRefreshEnabled') }}</span>
  </label>
  <Input
    v-model.number="groupForm.upstream_balance_refresh_interval_seconds"
    type="number"
    min="60"
    :disabled="!groupForm.upstream_balance_refresh_enabled"
  />
  <Input
    v-model.number="groupForm.upstream_price_max_multiplier"
    type="number"
    min="0"
    step="0.0001"
  />
</div>
```

Add validation:

```ts
if (form.upstream_balance_refresh_enabled && Number(form.upstream_balance_refresh_interval_seconds) < 60) {
  errors.push(t('admin.groups.validation.upstreamBalanceRefreshIntervalMin'))
}
if (Number(form.upstream_price_max_multiplier) < 0) {
  errors.push(t('admin.groups.validation.upstreamPriceMaxMultiplierMin'))
}
```

- [ ] **Step 5: Add account display helper**

Create `frontend/src/utils/upstreamPriceGuard.ts`:

```ts
type PriceGuardExtra = Record<string, unknown>

function getNumber(extra: PriceGuardExtra, key: string): number | null {
  const value = extra[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function formatRate(value: number): string {
  return `${Number(value.toFixed(6)).toString()}x`
}

export function getUpstreamPriceGuardLabel(extra: PriceGuardExtra | null | undefined): string {
  const status = String(extra?.upstream_price_guard_status ?? '').toLowerCase()
  if (!status || status === 'ok') return ''
  const actual = extra ? getNumber(extra, 'upstream_price_guard_actual_multiplier') : null
  const max = extra ? getNumber(extra, 'upstream_price_guard_max_multiplier') : null
  if (status === 'blocked' && actual != null && max != null) {
    return `价格超限 ${formatRate(actual)} > ${formatRate(max)}`
  }
  if (status === 'unsupported') return '价格未知'
  if (status === 'error') return '价格检查失败'
  return status
}
```

In `frontend/src/views/admin/AccountsView.vue`, import `getUpstreamPriceGuardLabel` and render `getUpstreamPriceGuardLabel(row.extra)` under the upstream group/rate cell. Use warning text color when the label starts with `价格超限`.

- [ ] **Step 6: Run frontend tests and commit**

Run:

```bash
cd frontend && pnpm test -- --run GroupsView upstreamPriceGuard
```

Expected: pass.

Commit:

```bash
git add frontend/src/types/index.ts frontend/src/utils/upstreamPriceGuard.ts frontend/src/utils/__tests__/upstreamPriceGuard.spec.ts frontend/src/views/admin/GroupsView.vue frontend/src/views/admin/AccountsView.vue frontend/src/views/admin/__tests__/GroupsView.spec.ts
git commit -m "feat: expose upstream price guard settings"
```

---

### Task 6: Final Verification

**Files:**
- Verify all touched backend and frontend files.

**Interfaces:**
- Consumes all prior tasks.
- Produces final confidence that the feature builds and focused tests pass.

- [ ] **Step 1: Run backend focused tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository ./internal/service ./internal/handler/admin ./cmd/server
```

Expected: pass.

- [ ] **Step 2: Run frontend focused tests**

Run:

```bash
cd frontend && pnpm test -- --run GroupsView AccountsView OpenAIUpstreamBalanceCell
```

Expected: pass.

- [ ] **Step 3: Run formatting**

Run:

```bash
cd backend && gofmt -w ent/schema/group.go internal/service/group.go internal/service/openai_upstream_price_guard.go internal/service/group_upstream_balance_refresh_runner.go internal/service/wire.go internal/repository/group_repo.go internal/repository/account_repo.go internal/handler/admin/group_handler.go internal/handler/dto/types.go internal/handler/dto/mappers.go
```

Expected: command exits 0 and `git diff` contains only expected formatting.

- [ ] **Step 4: Run final status**

Run:

```bash
git status --short
git log --oneline -6
```

Expected: worktree clean after final task commits; recent commits correspond to this plan.

---

## Self-Review

Spec coverage:

- Group-level config fields are covered by Task 1.
- Group-scoped refresh of all member accounts is covered by Task 3.
- Price limit block and automatic recovery only for price-guard reasons are covered by Task 2.
- Runner startup and shutdown are covered by Task 4.
- Admin UI and account visibility are covered by Task 5.
- Focused backend/frontend verification is covered by Task 6.

Placeholder scan:

- No placeholder markers or intentionally vague implementation steps remain.

Type consistency:

- Group fields use `UpstreamBalanceRefreshEnabled`, `UpstreamBalanceRefreshIntervalSeconds`, and `UpstreamPriceMaxMultiplier` in Go.
- JSON and TypeScript fields use `upstream_balance_refresh_enabled`, `upstream_balance_refresh_interval_seconds`, and `upstream_price_max_multiplier`.
- Price guard reason prefix is consistently `upstream_price_guard:`.
