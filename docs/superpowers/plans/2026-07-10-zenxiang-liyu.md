# 臻享礼遇 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `臻享礼遇`站内余额礼遇活动，包括用户端参与、后台配置、概率模拟、账务流水和统计。

**Architecture:** Add an isolated Zenxiang Liyu module across Ent schema, SQL migrations, repository, service, handlers, routes, and Vue pages. Backend remains the only trusted source for eligibility, probability selection, ticket deduction, reward crediting, idempotency, and statistics. Frontend consumes backend status for menu visibility and renders user/admin pages without deciding rewards locally.

**Tech Stack:** Go, Gin, Ent, PostgreSQL SQL migrations, Wire, Vue 3, TypeScript, Pinia, Axios, Vitest.

## Global Constraints

- 名称为 `臻享礼遇`。
- 余额为站内余额，不支持提现、现金兑换或站外资金流转。
- 功能默认全局关闭。
- 全局开启后，所有满足条件的普通用户可参与。
- 全局关闭时，可单独授权指定用户参与。
- 用户端使用独立菜单 `臻享礼遇`。
- 余额不足时菜单仍显示，页面提示余额需大于最低参与余额。
- 最低参与余额默认 `10`，判断口径为用户余额必须大于该值。
- 门票金额默认 `2`。
- 每人每日次数上限默认 `5`。
- 奖项档位后台可配置，不固定。
- 每次参与严格按当前启用档位概率独立随机。
- 不做用户级暗控。
- 不做每日奖池配额。
- 后端是唯一可信源，前端不传奖项、金额或概率。
- 扣门票、抽奖、派奖、写流水必须在数据库事务内完成。
- 模拟器不写真实用户余额和真实流水。

---

## File Structure

Backend files to create:

- `backend/ent/schema/zenxiang_liyu_setting.go`: Ent schema for singleton activity settings.
- `backend/ent/schema/zenxiang_liyu_prize.go`: Ent schema for configurable prize tiers.
- `backend/ent/schema/zenxiang_liyu_user_grant.go`: Ent schema for per-user access grants.
- `backend/ent/schema/zenxiang_liyu_record.go`: Ent schema for immutable play ledger rows.
- `backend/migrations/172_zenxiang_liyu.sql`: Idempotent SQL migration creating tables and indexes.
- `backend/internal/repository/zenxiang_liyu_repo.go`: SQL-backed transactional repository for play, settings, grants, stats, and simulation reads.
- `backend/internal/repository/zenxiang_liyu_repo_test.go`: Repository unit tests with `sqlmock` for transaction-critical SQL.
- `backend/internal/service/zenxiang_liyu_service.go`: Domain types, validation, eligibility, probability picker, simulator, recommendation algorithm, and play orchestration.
- `backend/internal/service/zenxiang_liyu_service_test.go`: Service tests for validation, probability boundaries, eligibility, idempotency, and simulation.
- `backend/internal/handler/zenxiang_liyu_handler.go`: User API handler.
- `backend/internal/handler/zenxiang_liyu_handler_test.go`: User handler tests.
- `backend/internal/handler/admin/zenxiang_liyu_handler.go`: Admin API handler.
- `backend/internal/handler/admin/zenxiang_liyu_handler_test.go`: Admin handler tests.

Backend files to modify:

- `backend/ent/schema/user.go`: Add edges to Zenxiang Liyu grants and records.
- `backend/internal/service/wire.go`: Register repository/service providers.
- `backend/internal/handler/handler.go`: Add user and admin Zenxiang handlers to handler structs.
- `backend/internal/handler/wire.go`: Register user/admin handler providers.
- `backend/internal/server/routes/user.go`: Register `/zenxiang-liyu` user routes.
- `backend/internal/server/routes/admin.go`: Register `/admin/zenxiang-liyu` admin routes.
- `backend/cmd/server/wire_gen.go`: Regenerate after Wire provider changes.

Frontend files to create:

- `frontend/src/api/zenxiangLiyu.ts`: User API types and calls.
- `frontend/src/api/admin/zenxiangLiyu.ts`: Admin API types and calls.
- `frontend/src/stores/zenxiangLiyu.ts`: Cached user status for sidebar visibility.
- `frontend/src/views/user/ZenxiangLiyuView.vue`: User page.
- `frontend/src/views/admin/ZenxiangLiyuAdminView.vue`: Admin operations page with tabs.
- `frontend/src/views/user/__tests__/ZenxiangLiyuView.spec.ts`: User page tests.
- `frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts`: Admin page tests.
- `frontend/src/api/__tests__/zenxiangLiyu.spec.ts`: User API tests.
- `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts`: Admin API tests.

Frontend files to modify:

- `frontend/src/api/admin/index.ts`: Export admin Zenxiang API.
- `frontend/src/router/index.ts`: Add `/zenxiang-liyu` and `/admin/zenxiang-liyu` routes.
- `frontend/src/components/layout/AppSidebar.vue`: Add user/admin menu items controlled by status/store.
- `frontend/src/i18n/locales/zh.ts`: Add Chinese labels.
- `frontend/src/i18n/locales/en.ts`: Add English fallback labels.

---

### Task 1: Database Schema, Ent Models, and Migration

**Files:**
- Create: `backend/ent/schema/zenxiang_liyu_setting.go`
- Create: `backend/ent/schema/zenxiang_liyu_prize.go`
- Create: `backend/ent/schema/zenxiang_liyu_user_grant.go`
- Create: `backend/ent/schema/zenxiang_liyu_record.go`
- Create: `backend/migrations/172_zenxiang_liyu.sql`
- Modify: `backend/ent/schema/user.go`
- Generated: `backend/ent/*`

**Interfaces:**
- Produces Ent models: `ZenxiangLiyuSetting`, `ZenxiangLiyuPrize`, `ZenxiangLiyuUserGrant`, `ZenxiangLiyuRecord`.
- Produces SQL tables: `zenxiang_liyu_settings`, `zenxiang_liyu_prizes`, `zenxiang_liyu_user_grants`, `zenxiang_liyu_records`.
- Later tasks rely on `zenxiang_liyu_records.request_id` being unique and `zenxiang_liyu_user_grants.user_id` being unique.

- [ ] **Step 1: Add migration first**

Create `backend/migrations/172_zenxiang_liyu.sql`:

```sql
CREATE TABLE IF NOT EXISTS zenxiang_liyu_settings (
    id BIGSERIAL PRIMARY KEY,
    global_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ticket_amount NUMERIC(20,8) NOT NULL DEFAULT 2,
    minimum_balance NUMERIC(20,8) NOT NULL DEFAULT 10,
    daily_play_limit INTEGER NOT NULL DEFAULT 5,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_settings_positive_ticket CHECK (ticket_amount > 0),
    CONSTRAINT zenxiang_liyu_settings_non_negative_minimum CHECK (minimum_balance >= 0),
    CONSTRAINT zenxiang_liyu_settings_positive_daily_limit CHECK (daily_play_limit > 0)
);

INSERT INTO zenxiang_liyu_settings (id, global_enabled, ticket_amount, minimum_balance, daily_play_limit)
VALUES (1, FALSE, 2, 10, 5)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS zenxiang_liyu_prizes (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL,
    probability NUMERIC(12,8) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_prizes_non_negative_reward CHECK (reward_amount >= 0),
    CONSTRAINT zenxiang_liyu_prizes_probability_range CHECK (probability >= 0 AND probability <= 100)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_prizes_enabled_sort
    ON zenxiang_liyu_prizes (enabled, sort_order, id);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_user_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    granted_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_user_grants_user_unique UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_user_grants_enabled
    ON zenxiang_liyu_user_grants (enabled);

CREATE TABLE IF NOT EXISTS zenxiang_liyu_records (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    play_date DATE NOT NULL,
    ticket_amount NUMERIC(20,8) NOT NULL,
    reward_amount NUMERIC(20,8) NOT NULL,
    user_net_amount NUMERIC(20,8) NOT NULL,
    system_revenue NUMERIC(20,8) NOT NULL,
    system_expense NUMERIC(20,8) NOT NULL,
    system_profit NUMERIC(20,8) NOT NULL,
    prize_id BIGINT NULL REFERENCES zenxiang_liyu_prizes(id) ON DELETE SET NULL,
    prize_name_snapshot VARCHAR(100) NOT NULL,
    probability_snapshot NUMERIC(12,8) NOT NULL,
    config_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after_ticket NUMERIC(20,8) NOT NULL,
    balance_after_reward NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zenxiang_liyu_records_request_unique UNIQUE (request_id),
    CONSTRAINT zenxiang_liyu_records_non_negative_ticket CHECK (ticket_amount > 0),
    CONSTRAINT zenxiang_liyu_records_non_negative_reward CHECK (reward_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_user_date
    ON zenxiang_liyu_records (user_id, play_date);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_play_date
    ON zenxiang_liyu_records (play_date);

CREATE INDEX IF NOT EXISTS idx_zenxiang_liyu_records_prize
    ON zenxiang_liyu_records (prize_id);
```

- [ ] **Step 2: Add Ent schemas**

Create the four schema files with table annotations matching the SQL names, decimal schema types for money/probability fields, and `TimeMixin` where appropriate. Use hard-delete semantics for all four tables, because records and config rows are operational artifacts and do not need soft delete filters.

Example field block for `ZenxiangLiyuPrize`:

```go
field.String("name").MaxLen(100).NotEmpty(),
field.Float("reward_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
field.Float("probability").SchemaType(map[string]string{dialect.Postgres: "decimal(12,8)"}),
field.Bool("enabled").Default(true),
field.Int("sort_order").Default(0),
```

- [ ] **Step 3: Add user edges**

Modify `backend/ent/schema/user.go` edges:

```go
edge.To("zenxiang_liyu_grants", ZenxiangLiyuUserGrant.Type),
edge.To("zenxiang_liyu_records", ZenxiangLiyuRecord.Type),
```

- [ ] **Step 4: Generate Ent code**

Run:

```bash
cd backend
go generate ./ent
```

Expected: command exits `0` and generated Ent files include `zenxiangliyuprize`, `zenxiangliyurecord`, `zenxiangliyusetting`, and `zenxiangliyuusergrant` packages.

- [ ] **Step 5: Verify migration and schema compile**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./ent ./internal/repository -run 'TestMigrations|Test.*Schema' -count=1
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add backend/ent backend/migrations/172_zenxiang_liyu.sql
git commit -m "feat: add zenxiang liyu schema"
```

---

### Task 2: Service Domain, Validation, Probability, and Simulation

**Files:**
- Create: `backend/internal/service/zenxiang_liyu_service.go`
- Create: `backend/internal/service/zenxiang_liyu_service_test.go`
- Modify: `backend/internal/service/wire.go`

**Interfaces:**
- Produces constructor: `func NewZenxiangLiyuService(repo ZenxiangLiyuRepository, clock func() time.Time, rng *rand.Rand) *ZenxiangLiyuService`.
- Produces interface: `type ZenxiangLiyuRepository interface`.
- Produces public service methods:
  - `GetStatus(ctx context.Context, userID int64) (*ZenxiangLiyuStatus, error)`
  - `Play(ctx context.Context, userID int64, requestID string) (*ZenxiangLiyuPlayResult, error)`
  - `GetSettings(ctx context.Context) (*ZenxiangLiyuSettings, error)`
  - `UpdateSettings(ctx context.Context, req ZenxiangLiyuSettingsUpdate) (*ZenxiangLiyuSettings, error)`
  - `ListPrizes(ctx context.Context) ([]ZenxiangLiyuPrize, error)`
  - `SavePrize(ctx context.Context, req ZenxiangLiyuPrizeUpdate) (*ZenxiangLiyuPrize, error)`
  - `DeletePrize(ctx context.Context, id int64) error`
  - `Simulate(ctx context.Context, req ZenxiangLiyuSimulationRequest) (*ZenxiangLiyuSimulationResult, error)`
  - `Recommend(ctx context.Context, req ZenxiangLiyuRecommendationRequest) (*ZenxiangLiyuRecommendationResult, error)`

- [ ] **Step 1: Write failing validation tests**

Add tests:

```go
func TestZenxiangLiyuValidatePrizesRequiresEnabledProbabilityTotal100(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 40, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 50, Enabled: true},
	}
	err := ValidateZenxiangLiyuPrizes(prizes)
	require.ErrorIs(t, err, ErrZenxiangLiyuInvalidProbabilityTotal)
}

func TestZenxiangLiyuValidatePrizesAcceptsConfiguredTiers(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "1元", RewardAmount: 1, Probability: 70, Enabled: true},
		{ID: 2, Name: "3元", RewardAmount: 3, Probability: 20, Enabled: true},
		{ID: 3, Name: "10元", RewardAmount: 10, Probability: 10, Enabled: true},
	}
	require.NoError(t, ValidateZenxiangLiyuPrizes(prizes))
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyuValidatePrizes' -count=1
```

Expected: fails because the symbols do not exist.

- [ ] **Step 2: Implement domain types and validation**

Add errors and types:

```go
var (
	ErrZenxiangLiyuDisabled                = errors.New("zenxiang liyu is disabled")
	ErrZenxiangLiyuUnauthorized           = errors.New("zenxiang liyu unauthorized")
	ErrZenxiangLiyuInvalidSettings        = errors.New("zenxiang liyu invalid settings")
	ErrZenxiangLiyuInvalidProbabilityTotal = errors.New("zenxiang liyu invalid probability total")
	ErrZenxiangLiyuInsufficientBalance    = errors.New("zenxiang liyu insufficient balance")
	ErrZenxiangLiyuDailyLimitReached      = errors.New("zenxiang liyu daily limit reached")
	ErrZenxiangLiyuRequestIDRequired      = errors.New("zenxiang liyu request id required")
)

type ZenxiangLiyuSettings struct {
	GlobalEnabled  bool    `json:"global_enabled"`
	TicketAmount   float64 `json:"ticket_amount"`
	MinimumBalance float64 `json:"minimum_balance"`
	DailyPlayLimit int     `json:"daily_play_limit"`
}

type ZenxiangLiyuPrize struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	RewardAmount float64 `json:"reward_amount"`
	Probability  float64 `json:"probability"`
	Enabled      bool    `json:"enabled"`
	SortOrder    int     `json:"sort_order"`
}
```

Implement `ValidateZenxiangLiyuPrizes` with epsilon `0.000001`:

```go
func ValidateZenxiangLiyuPrizes(prizes []ZenxiangLiyuPrize) error {
	total := 0.0
	enabled := 0
	for _, prize := range prizes {
		if strings.TrimSpace(prize.Name) == "" || prize.RewardAmount < 0 || prize.Probability < 0 || prize.Probability > 100 {
			return ErrZenxiangLiyuInvalidSettings
		}
		if prize.Enabled {
			enabled++
			total += prize.Probability
		}
	}
	if enabled == 0 {
		return ErrZenxiangLiyuInvalidProbabilityTotal
	}
	if math.Abs(total-100) > 0.000001 {
		return ErrZenxiangLiyuInvalidProbabilityTotal
	}
	return nil
}
```

- [ ] **Step 3: Write failing probability tests**

Add:

```go
func TestPickZenxiangLiyuPrizeUsesProbabilityBoundaries(t *testing.T) {
	prizes := []ZenxiangLiyuPrize{
		{ID: 1, Name: "A", RewardAmount: 1, Probability: 70, Enabled: true},
		{ID: 2, Name: "B", RewardAmount: 3, Probability: 30, Enabled: true},
	}
	picked, err := PickZenxiangLiyuPrize(prizes, 69.9999)
	require.NoError(t, err)
	require.EqualValues(t, 1, picked.ID)
	picked, err = PickZenxiangLiyuPrize(prizes, 70.0000)
	require.NoError(t, err)
	require.EqualValues(t, 2, picked.ID)
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestPickZenxiangLiyuPrize' -count=1
```

Expected: fails because `PickZenxiangLiyuPrize` does not exist.

- [ ] **Step 4: Implement probability picker**

Implement:

```go
func PickZenxiangLiyuPrize(prizes []ZenxiangLiyuPrize, roll float64) (*ZenxiangLiyuPrize, error) {
	if err := ValidateZenxiangLiyuPrizes(prizes); err != nil {
		return nil, err
	}
	cumulative := 0.0
	for i := range prizes {
		if !prizes[i].Enabled {
			continue
		}
		cumulative += prizes[i].Probability
		if roll < cumulative || math.Abs(cumulative-100) <= 0.000001 {
			picked := prizes[i]
			return &picked, nil
		}
	}
	return nil, ErrZenxiangLiyuInvalidProbabilityTotal
}
```

- [ ] **Step 5: Write simulation and recommendation tests**

Add tests asserting:

```go
func TestZenxiangLiyuSimulationComputesProfitAndUserDistribution(t *testing.T) {
	req := ZenxiangLiyuSimulationRequest{
		UserCount:       2,
		PlaysPerUser:    2,
		InitialBalance:  100,
		TicketAmount:    2,
		MinimumBalance:  10,
		DailyPlayLimit:  5,
		Prizes: []ZenxiangLiyuPrize{
			{ID: 1, Name: "1元", RewardAmount: 1, Probability: 100, Enabled: true},
		},
	}
	svc := NewZenxiangLiyuService(nil, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))
	result, err := svc.Simulate(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 4, result.TotalPlays)
	require.InDelta(t, 8, result.TotalRevenue, 0.000001)
	require.InDelta(t, 4, result.TotalExpense, 0.000001)
	require.InDelta(t, 4, result.NetProfit, 0.000001)
	require.Equal(t, 0, result.ProfitableUsers)
	require.Equal(t, 2, result.LosingUsers)
}

func TestZenxiangLiyuRecommendReturnsProbabilityTotal100(t *testing.T) {
	svc := NewZenxiangLiyuService(nil, func() time.Time { return time.Unix(0, 0).UTC() }, rand.New(rand.NewSource(1)))
	result, err := svc.Recommend(context.Background(), ZenxiangLiyuRecommendationRequest{
		TargetProfitRate: 0.05,
		TicketAmount:     2,
		Prizes: []ZenxiangLiyuPrize{
			{ID: 1, Name: "1元", RewardAmount: 1, Enabled: true},
			{ID: 2, Name: "3元", RewardAmount: 3, Enabled: true},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Plans)
	require.InDelta(t, 100, result.Plans[0].ProbabilityTotal, 0.000001)
	require.InDelta(t, 0.05, result.Plans[0].TheoryProfitRate, 0.000001)
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyuSimulation|TestZenxiangLiyuRecommend' -count=1
```

Expected: fails before implementation.

- [ ] **Step 6: Implement simulator and recommendation**

Simulation:

- Clamp each user's plays by `DailyPlayLimit`.
- Skip a play if simulated balance is not greater than `MinimumBalance`.
- Deduct `TicketAmount`, pick prize, add reward, update user net.
- Aggregate total revenue, expense, profit, profit rate, user distribution, prize hit counts, and actual rates.

Recommendation:

- Compute `targetExpense = ticketAmount * (1 - targetProfitRate)`.
- For two or more enabled prizes, assign probability primarily to the closest lower reward and closest higher reward around `targetExpense`.
- If every reward is greater than `targetExpense`, put `99%` on the lowest reward and `1%` on the next lowest reward, then report achieved theory rate.
- If every reward is lower than `targetExpense`, put `99%` on the highest reward and `1%` on the next highest reward, then report achieved theory rate.

- [ ] **Step 7: Register service provider**

Modify `backend/internal/service/wire.go` provider set:

```go
NewZenxiangLiyuService,
```

Add provider helper if needed:

```go
func ProvideZenxiangLiyuService(repo ZenxiangLiyuRepository) *ZenxiangLiyuService {
	return NewZenxiangLiyuService(repo, time.Now, rand.New(rand.NewSource(time.Now().UnixNano())))
}
```

- [ ] **Step 8: Verify service package**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service -run 'TestZenxiangLiyu' -count=1
```

Expected: all Zenxiang service tests pass.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/service
git commit -m "feat: add zenxiang liyu service logic"
```

---

### Task 3: Repository and Transactional Play

**Files:**
- Create: `backend/internal/repository/zenxiang_liyu_repo.go`
- Create: `backend/internal/repository/zenxiang_liyu_repo_test.go`
- Modify: `backend/internal/service/zenxiang_liyu_service.go`
- Modify: `backend/internal/service/wire.go`

**Interfaces:**
- Consumes service repository interface from Task 2.
- Produces repository constructor: `func NewZenxiangLiyuRepository(client *dbent.Client, sqlDB *sql.DB) service.ZenxiangLiyuRepository`.
- Produces atomic method: `Play(ctx context.Context, cmd service.ZenxiangLiyuPlayCommand) (*service.ZenxiangLiyuPlayResult, error)`.

- [ ] **Step 1: Write repository unit test for idempotency**

Use `sqlmock` to assert:

```go
func TestZenxiangLiyuRepositoryPlayReturnsExistingRecordForSameRequestID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := &zenxiangLiyuRepository{db: db}
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM zenxiang_liyu_records WHERE request_id = \$1`).
		WithArgs("req-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "user_id", "ticket_amount", "reward_amount", "user_net_amount",
			"system_revenue", "system_expense", "system_profit", "prize_id", "prize_name_snapshot",
			"probability_snapshot", "balance_before", "balance_after_ticket", "balance_after_reward", "created_at",
		}).AddRow(9, "req-1", 42, 2.0, 3.0, 1.0, 2.0, 3.0, -1.0, 7, "3元", 20.0, 12.0, 10.0, 13.0, time.Unix(1, 0)))
	mock.ExpectCommit()

	result, err := repo.Play(ctx, service.ZenxiangLiyuPlayCommand{UserID: 42, RequestID: "req-1"})
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.InDelta(t, 3, result.RewardAmount, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestZenxiangLiyuRepositoryPlayReturnsExistingRecord' -count=1
```

Expected: fails because repository does not exist.

- [ ] **Step 2: Implement repository read/write models**

Implement SQL repository using `database/sql` for transaction control and row locks. Add methods:

```go
func (r *zenxiangLiyuRepository) GetSettings(ctx context.Context) (*service.ZenxiangLiyuSettings, error)
func (r *zenxiangLiyuRepository) ListPrizes(ctx context.Context) ([]service.ZenxiangLiyuPrize, error)
func (r *zenxiangLiyuRepository) IsUserGranted(ctx context.Context, userID int64) (bool, error)
func (r *zenxiangLiyuRepository) CountUserPlaysOnDate(ctx context.Context, userID int64, playDate time.Time) (int, error)
func (r *zenxiangLiyuRepository) Play(ctx context.Context, cmd service.ZenxiangLiyuPlayCommand) (*service.ZenxiangLiyuPlayResult, error)
```

- [ ] **Step 3: Write transactional play SQL test**

Test that `Play`:

- Begins a transaction.
- Checks existing `request_id`.
- Locks the user row with `FOR UPDATE`.
- Counts today's records.
- Updates balance to deduct ticket.
- Inserts record.
- Updates balance to add reward.
- Commits.

Expected SQL fragments:

```go
mock.ExpectQuery(`SELECT id, email, role, status, balance FROM users WHERE id = \$1 AND deleted_at IS NULL FOR UPDATE`)
mock.ExpectQuery(`SELECT COUNT\(\*\) FROM zenxiang_liyu_records WHERE user_id = \$1 AND play_date = \$2`)
mock.ExpectQuery(`UPDATE users SET balance = balance - \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`)
mock.ExpectQuery(`UPDATE users SET balance = balance \+ \$1, total_recharged = total_recharged \+ \$1, updated_at = NOW\(\) WHERE id = \$2 RETURNING balance`)
mock.ExpectQuery(`INSERT INTO zenxiang_liyu_records`)
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run 'TestZenxiangLiyuRepositoryPlayAppliesAtomically' -count=1
```

Expected: fails before implementation, passes after implementation.

- [ ] **Step 4: Implement atomic play**

Repository `Play` receives a command containing already validated settings, prizes, selected prize, play date, and config snapshot. It must still lock and re-check balance/count inside the transaction:

```go
type ZenxiangLiyuPlayCommand struct {
	UserID         int64
	RequestID      string
	PlayDate       time.Time
	Settings       ZenxiangLiyuSettings
	Prize          ZenxiangLiyuPrize
	ConfigSnapshot map[string]any
}
```

Inside transaction:

1. Query existing record by `request_id`; if found, return `Applied=false`.
2. `SELECT ... FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`.
3. Reject non-user role or inactive status.
4. Reject `balance <= minimum_balance`.
5. Count records for `(user_id, play_date)`.
6. Reject if count is greater than or equal to limit.
7. Deduct ticket with `UPDATE users`.
8. Insert ledger record.
9. Add reward with `UPDATE users`; because this is promotional站内余额, increment `total_recharged` consistently with existing positive `UpdateBalance` behavior.
10. Commit.

- [ ] **Step 5: Wire repository provider**

Modify `backend/internal/service/wire.go` or repository provider set file to include:

```go
repository.NewZenxiangLiyuRepository,
```

Use the existing repository provider set location in the project. Keep constructor return type as `service.ZenxiangLiyuRepository`.

- [ ] **Step 6: Verify repository and service**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository ./internal/service -run 'TestZenxiangLiyu' -count=1
```

Expected: all Zenxiang repository and service tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/repository backend/internal/service
git commit -m "feat: add zenxiang liyu transactional repository"
```

---

### Task 4: User and Admin HTTP APIs

**Files:**
- Create: `backend/internal/handler/zenxiang_liyu_handler.go`
- Create: `backend/internal/handler/zenxiang_liyu_handler_test.go`
- Create: `backend/internal/handler/admin/zenxiang_liyu_handler.go`
- Create: `backend/internal/handler/admin/zenxiang_liyu_handler_test.go`
- Modify: `backend/internal/handler/handler.go`
- Modify: `backend/internal/handler/wire.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/admin.go`
- Generated: `backend/cmd/server/wire_gen.go`

**Interfaces:**
- Consumes `*service.ZenxiangLiyuService`.
- Produces user routes:
  - `GET /api/v1/zenxiang-liyu/status`
  - `POST /api/v1/zenxiang-liyu/play`
  - `GET /api/v1/zenxiang-liyu/records`
  - `GET /api/v1/zenxiang-liyu/daily-summary`
- Produces admin routes under `/api/v1/admin/zenxiang-liyu`.

- [ ] **Step 1: Write user handler tests**

Add tests for:

```go
func TestZenxiangLiyuHandlerPlayRejectsMissingRequestID(t *testing.T) {
	h := NewZenxiangLiyuHandler(&stubZenxiangLiyuService{})
	router := gin.New()
	router.POST("/zenxiang-liyu/play", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		h.Play(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/zenxiang-liyu/play", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestZenxiangLiyuHandlerStatusReturnsServicePayload(t *testing.T) {
	svc := &stubZenxiangLiyuService{status: &service.ZenxiangLiyuStatus{Visible: true, CanPlay: true, TicketAmount: 2}}
	h := NewZenxiangLiyuHandler(svc)
	router := gin.New()
	router.GET("/zenxiang-liyu/status", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		h.GetStatus(c)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/zenxiang-liyu/status", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"visible":true`)
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler -run 'TestZenxiangLiyuHandler' -count=1
```

Expected: fails before handler exists.

- [ ] **Step 2: Implement user handler**

Create `ZenxiangLiyuHandler` with methods:

```go
func NewZenxiangLiyuHandler(service zenxiangLiyuUserService) *ZenxiangLiyuHandler
func (h *ZenxiangLiyuHandler) GetStatus(c *gin.Context)
func (h *ZenxiangLiyuHandler) Play(c *gin.Context)
func (h *ZenxiangLiyuHandler) ListRecords(c *gin.Context)
func (h *ZenxiangLiyuHandler) GetDailySummary(c *gin.Context)
```

`Play` request:

```go
type zenxiangLiyuPlayRequest struct {
	RequestID string `json:"request_id" binding:"required"`
}
```

Map service errors:

- `ErrZenxiangLiyuRequestIDRequired`: 400
- `ErrZenxiangLiyuDisabled`, `ErrZenxiangLiyuUnauthorized`: 403
- `ErrZenxiangLiyuInsufficientBalance`, `ErrZenxiangLiyuDailyLimitReached`: 400
- other errors: 500

- [ ] **Step 3: Write admin handler tests**

Add tests for settings and simulation:

```go
func TestAdminZenxiangLiyuUpdateSettingsMapsPayload(t *testing.T) {
	svc := &stubAdminZenxiangLiyuService{}
	h := admin.NewZenxiangLiyuHandler(svc)
	router := gin.New()
	router.PUT("/admin/zenxiang-liyu/settings", h.UpdateSettings)
	body := `{"global_enabled":true,"ticket_amount":2,"minimum_balance":10,"daily_play_limit":5}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/admin/zenxiang-liyu/settings", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, svc.lastSettings.GlobalEnabled)
}

func TestAdminZenxiangLiyuSimulateReturnsResult(t *testing.T) {
	svc := &stubAdminZenxiangLiyuService{simulation: &service.ZenxiangLiyuSimulationResult{TotalPlays: 10, NetProfit: 2}}
	h := admin.NewZenxiangLiyuHandler(svc)
	router := gin.New()
	router.POST("/admin/zenxiang-liyu/simulate", h.Simulate)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/zenxiang-liyu/simulate", strings.NewReader(`{"user_count":1,"plays_per_user":10}`)))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"total_plays":10`)
}
```

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run 'TestAdminZenxiangLiyu' -count=1
```

Expected: fails before admin handler exists.

- [ ] **Step 4: Implement admin handler**

Methods:

```go
func NewZenxiangLiyuHandler(service zenxiangLiyuAdminService) *ZenxiangLiyuHandler
func (h *ZenxiangLiyuHandler) GetSettings(c *gin.Context)
func (h *ZenxiangLiyuHandler) UpdateSettings(c *gin.Context)
func (h *ZenxiangLiyuHandler) ListPrizes(c *gin.Context)
func (h *ZenxiangLiyuHandler) SavePrize(c *gin.Context)
func (h *ZenxiangLiyuHandler) DeletePrize(c *gin.Context)
func (h *ZenxiangLiyuHandler) ListGrants(c *gin.Context)
func (h *ZenxiangLiyuHandler) CreateGrant(c *gin.Context)
func (h *ZenxiangLiyuHandler) DeleteGrant(c *gin.Context)
func (h *ZenxiangLiyuHandler) GetOverviewStats(c *gin.Context)
func (h *ZenxiangLiyuHandler) GetUserStats(c *gin.Context)
func (h *ZenxiangLiyuHandler) GetPrizeStats(c *gin.Context)
func (h *ZenxiangLiyuHandler) Simulate(c *gin.Context)
func (h *ZenxiangLiyuHandler) Recommend(c *gin.Context)
func (h *ZenxiangLiyuHandler) ApplySimulation(c *gin.Context)
```

- [ ] **Step 5: Register handlers and routes**

Modify `backend/internal/handler/handler.go`:

```go
ZenxiangLiyu *ZenxiangLiyuHandler
```

Inside `AdminHandlers`:

```go
ZenxiangLiyu *admin.ZenxiangLiyuHandler
```

Modify `backend/internal/server/routes/user.go`:

```go
zenxiangLiyu := authenticated.Group("/zenxiang-liyu")
{
	zenxiangLiyu.GET("/status", h.ZenxiangLiyu.GetStatus)
	zenxiangLiyu.POST("/play", h.ZenxiangLiyu.Play)
	zenxiangLiyu.GET("/records", h.ZenxiangLiyu.ListRecords)
	zenxiangLiyu.GET("/daily-summary", h.ZenxiangLiyu.GetDailySummary)
}
```

Modify `backend/internal/server/routes/admin.go`:

```go
zenxiangLiyu := admin.Group("/zenxiang-liyu")
{
	zenxiangLiyu.GET("/settings", h.Admin.ZenxiangLiyu.GetSettings)
	zenxiangLiyu.PUT("/settings", h.Admin.ZenxiangLiyu.UpdateSettings)
	zenxiangLiyu.GET("/prizes", h.Admin.ZenxiangLiyu.ListPrizes)
	zenxiangLiyu.POST("/prizes", h.Admin.ZenxiangLiyu.SavePrize)
	zenxiangLiyu.PUT("/prizes/:id", h.Admin.ZenxiangLiyu.SavePrize)
	zenxiangLiyu.DELETE("/prizes/:id", h.Admin.ZenxiangLiyu.DeletePrize)
	zenxiangLiyu.GET("/grants", h.Admin.ZenxiangLiyu.ListGrants)
	zenxiangLiyu.POST("/grants", h.Admin.ZenxiangLiyu.CreateGrant)
	zenxiangLiyu.DELETE("/grants/:user_id", h.Admin.ZenxiangLiyu.DeleteGrant)
	zenxiangLiyu.GET("/stats/overview", h.Admin.ZenxiangLiyu.GetOverviewStats)
	zenxiangLiyu.GET("/stats/users", h.Admin.ZenxiangLiyu.GetUserStats)
	zenxiangLiyu.GET("/stats/prizes", h.Admin.ZenxiangLiyu.GetPrizeStats)
	zenxiangLiyu.POST("/simulate", h.Admin.ZenxiangLiyu.Simulate)
	zenxiangLiyu.POST("/simulate/recommend", h.Admin.ZenxiangLiyu.Recommend)
	zenxiangLiyu.POST("/simulate/apply", h.Admin.ZenxiangLiyu.ApplySimulation)
}
```

- [ ] **Step 6: Regenerate Wire**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache GOMODCACHE=/tmp/sub2api-go-modcache go generate ./cmd/server
```

Expected: `backend/cmd/server/wire_gen.go` updates and command exits `0`.

- [ ] **Step 7: Verify handler packages**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'TestZenxiangLiyu|Test.*Route' -count=1
```

Expected: command exits `0`.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handler backend/internal/server/routes backend/cmd/server/wire_gen.go
git commit -m "feat: expose zenxiang liyu APIs"
```

---

### Task 5: Frontend APIs, Store, Routes, and Sidebar Visibility

**Files:**
- Create: `frontend/src/api/zenxiangLiyu.ts`
- Create: `frontend/src/api/admin/zenxiangLiyu.ts`
- Create: `frontend/src/stores/zenxiangLiyu.ts`
- Create: `frontend/src/api/__tests__/zenxiangLiyu.spec.ts`
- Create: `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts`
- Modify: `frontend/src/api/admin/index.ts`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes backend JSON contracts from Task 4.
- Produces `useZenxiangLiyuStore()` with `status`, `loadStatus()`, `visible`, and `canPlay`.
- Produces frontend routes `/zenxiang-liyu` and `/admin/zenxiang-liyu`.

- [ ] **Step 1: Write API tests**

Create `frontend/src/api/__tests__/zenxiangLiyu.spec.ts` asserting:

```ts
it('loads status from user endpoint', async () => {
  mock.onGet('/zenxiang-liyu/status').reply(200, { code: 0, data: { visible: true, can_play: false } })
  await expect(getZenxiangLiyuStatus()).resolves.toMatchObject({ visible: true, can_play: false })
})

it('posts play request id only', async () => {
  mock.onPost('/zenxiang-liyu/play', { request_id: 'req-1' }).reply(200, { code: 0, data: { reward_amount: 3 } })
  await expect(playZenxiangLiyu('req-1')).resolves.toMatchObject({ reward_amount: 3 })
})
```

Create `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts` asserting settings and simulation paths:

```ts
it('updates admin settings', async () => {
  const payload = { global_enabled: true, ticket_amount: 2, minimum_balance: 10, daily_play_limit: 5 }
  mock.onPut('/admin/zenxiang-liyu/settings', payload).reply(200, { code: 0, data: payload })
  await expect(adminZenxiangLiyuAPI.updateSettings(payload)).resolves.toEqual(payload)
})
```

Run:

```bash
cd frontend
pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts
```

Expected: fails before API files exist.

- [ ] **Step 2: Implement API modules**

User API:

```ts
export interface ZenxiangLiyuStatus {
  visible: boolean
  can_play: boolean
  reason?: string
  balance?: number
  ticket_amount: number
  minimum_balance: number
  daily_play_limit: number
  today_play_count: number
  remaining_plays: number
  prizes: ZenxiangLiyuPrize[]
}

export async function getZenxiangLiyuStatus(): Promise<ZenxiangLiyuStatus> {
  const { data } = await apiClient.get<ZenxiangLiyuStatus>('/zenxiang-liyu/status')
  return data
}

export async function playZenxiangLiyu(requestId: string): Promise<ZenxiangLiyuPlayResult> {
  const { data } = await apiClient.post<ZenxiangLiyuPlayResult>('/zenxiang-liyu/play', { request_id: requestId })
  return data
}
```

Admin API path base: `'/admin/zenxiang-liyu'`.

- [ ] **Step 3: Implement Pinia store**

Create `frontend/src/stores/zenxiangLiyu.ts`:

```ts
export const useZenxiangLiyuStore = defineStore('zenxiangLiyu', () => {
  const status = ref<ZenxiangLiyuStatus | null>(null)
  const loading = ref(false)
  const loaded = ref(false)

  const visible = computed(() => status.value?.visible === true)
  const canPlay = computed(() => status.value?.can_play === true)

  async function loadStatus() {
    if (loading.value) return
    loading.value = true
    try {
      status.value = await getZenxiangLiyuStatus()
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  return { status, loading, loaded, visible, canPlay, loadStatus }
})
```

- [ ] **Step 4: Add routes**

Modify `frontend/src/router/index.ts` user route section:

```ts
{
  path: '/zenxiang-liyu',
  name: 'ZenxiangLiyu',
  component: () => import('@/views/user/ZenxiangLiyuView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: false,
    title: 'Zenxiang Liyu',
    titleKey: 'zenxiangLiyu.title'
  }
}
```

Add admin route:

```ts
{
  path: '/admin/zenxiang-liyu',
  name: 'AdminZenxiangLiyu',
  component: () => import('@/views/admin/ZenxiangLiyuAdminView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,
    title: 'Zenxiang Liyu Operations',
    titleKey: 'admin.zenxiangLiyu.title',
    descriptionKey: 'admin.zenxiangLiyu.description'
  }
}
```

- [ ] **Step 5: Add sidebar menu items**

Import store and an icon from existing lucide imports. Use `SparklesIcon` if already available.

Add:

```ts
const zenxiangLiyuStore = useZenxiangLiyuStore()
const flagZenxiangLiyu = () => zenxiangLiyuStore.visible
```

On auth-ready lifecycle, call `zenxiangLiyuStore.loadStatus()` for authenticated users. Add user nav item after `/workbench`:

```ts
{ path: '/zenxiang-liyu', label: t('nav.zenxiangLiyu'), icon: SparklesIcon, featureFlag: flagZenxiangLiyu },
```

Add admin nav item near运营/统计 entries:

```ts
{ path: '/admin/zenxiang-liyu', label: t('nav.zenxiangLiyuOps'), icon: SparklesIcon, hideInSimpleMode: true },
```

- [ ] **Step 6: Add i18n keys**

Chinese:

```ts
nav: {
  zenxiangLiyu: '臻享礼遇',
  zenxiangLiyuOps: '臻享礼遇运营',
}
```

English:

```ts
nav: {
  zenxiangLiyu: 'Premium Rewards',
  zenxiangLiyuOps: 'Premium Rewards Ops',
}
```

- [ ] **Step 7: Verify frontend API and route tests**

Run:

```bash
cd frontend
pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts src/router/__tests__/guards.spec.ts
```

Expected: command exits `0`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/api frontend/src/stores/zenxiangLiyu.ts frontend/src/router/index.ts frontend/src/components/layout/AppSidebar.vue frontend/src/i18n/locales
git commit -m "feat: add zenxiang liyu frontend plumbing"
```

---

### Task 6: User Page

**Files:**
- Create: `frontend/src/views/user/ZenxiangLiyuView.vue`
- Create: `frontend/src/views/user/__tests__/ZenxiangLiyuView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes `useZenxiangLiyuStore`, `getZenxiangLiyuStatus`, and `playZenxiangLiyu`.
- Produces user flow for status, insufficent balance, daily limit, play result, and record refresh hook.

- [ ] **Step 1: Write page tests**

Create tests:

```ts
it('shows insufficient balance reason and disables play', async () => {
  getZenxiangLiyuStatus.mockResolvedValue({
    visible: true,
    can_play: false,
    reason: 'insufficient_balance',
    balance: 10,
    ticket_amount: 2,
    minimum_balance: 10,
    daily_play_limit: 5,
    today_play_count: 0,
    remaining_plays: 5,
    prizes: [{ id: 1, name: '1元', reward_amount: 1, probability: 100, enabled: true, sort_order: 1 }],
  })
  const wrapper = mount(ZenxiangLiyuView, { global: testPlugins })
  await flushPromises()
  expect(wrapper.text()).toContain('余额需大于')
  expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeDefined()
})

it('plays once and displays reward from backend result', async () => {
  getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
  playZenxiangLiyu.mockResolvedValue({ reward_amount: 3, user_net_amount: 1, balance: 13, remaining_plays: 4, prize_name: '3元' })
  const wrapper = mount(ZenxiangLiyuView, { global: testPlugins })
  await flushPromises()
  await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
  await flushPromises()
  expect(playZenxiangLiyu).toHaveBeenCalledWith(expect.any(String))
  expect(wrapper.text()).toContain('3')
})
```

Run:

```bash
cd frontend
pnpm test --run src/views/user/__tests__/ZenxiangLiyuView.spec.ts
```

Expected: fails before view exists.

- [ ] **Step 2: Implement user page**

Page structure:

- Header with title `臻享礼遇`.
- Compact metrics row: current balance, ticket amount, remaining plays.
- Wheel-like interactive section using CSS conic gradient and prize labels.
- Prize list table/cards showing configured rewards and probabilities if backend includes them.
- Primary button `开启礼遇`.
- Result modal/panel showing backend returned prize name, reward amount, net amount, latest balance.

Use `data-testid="zenxiang-play"` for tests.

Do not hardcode reward tiers in the component; render `status.prizes`.

- [ ] **Step 3: Implement request id generation**

Use browser crypto:

```ts
function newRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `zenxiang-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
```

- [ ] **Step 4: Add i18n copy**

Add Chinese copy:

```ts
zenxiangLiyu: {
  title: '臻享礼遇',
  open: '开启礼遇',
  opening: '开启中',
  insufficientBalance: '余额需大于 {amount} 元才可参与',
  dailyLimitReached: '今日礼遇次数已用完',
  maintenance: '活动维护中',
  rewardResult: '恭喜获得 {amount} 元礼遇额度',
}
```

- [ ] **Step 5: Verify user page**

Run:

```bash
cd frontend
pnpm test --run src/views/user/__tests__/ZenxiangLiyuView.spec.ts
```

Expected: command exits `0`.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/user/ZenxiangLiyuView.vue frontend/src/views/user/__tests__/ZenxiangLiyuView.spec.ts frontend/src/i18n/locales
git commit -m "feat: add zenxiang liyu user page"
```

---

### Task 7: Admin Operations Page

**Files:**
- Create: `frontend/src/views/admin/ZenxiangLiyuAdminView.vue`
- Create: `frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Consumes `adminAPI.zenxiangLiyu`.
- Produces tabs: activity settings, prize probability, stats, simulator.

- [ ] **Step 1: Write admin page tests**

Create tests:

```ts
it('validates prize probability total before saving', async () => {
  adminZenxiangLiyuAPI.getSettings.mockResolvedValue(makeSettings())
  adminZenxiangLiyuAPI.listPrizes.mockResolvedValue([
    { id: 1, name: '1元', reward_amount: 1, probability: 60, enabled: true, sort_order: 1 },
    { id: 2, name: '3元', reward_amount: 3, probability: 30, enabled: true, sort_order: 2 },
  ])
  const wrapper = mount(ZenxiangLiyuAdminView, { global: testPlugins })
  await flushPromises()
  expect(wrapper.text()).toContain('90')
  expect(wrapper.text()).toContain('100')
})

it('runs simulator and shows profit result', async () => {
  adminZenxiangLiyuAPI.simulate.mockResolvedValue({ total_plays: 100, total_revenue: 200, total_expense: 180, net_profit: 20, profit_rate: 0.1, prize_hits: [] })
  const wrapper = mount(ZenxiangLiyuAdminView, { global: testPlugins })
  await flushPromises()
  await wrapper.find('[data-testid="zenxiang-tab-simulator"]').trigger('click')
  await wrapper.find('[data-testid="zenxiang-simulate"]').trigger('click')
  await flushPromises()
  expect(wrapper.text()).toContain('20')
})
```

Run:

```bash
cd frontend
pnpm test --run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: fails before view exists.

- [ ] **Step 2: Implement activity settings tab**

Fields:

- Global enabled toggle.
- Ticket amount numeric input.
- Minimum balance numeric input.
- Daily play limit integer input.
- Grant search input by user ID/email keyword.
- Grant list with enable/remove actions.

Save action calls `adminAPI.zenxiangLiyu.updateSettings`.

- [ ] **Step 3: Implement prize probability tab**

Fields per row:

- Name.
- Reward amount.
- Probability.
- Enabled toggle.
- Sort order.
- Save/delete actions.

Computed metrics:

```ts
const enabledPrizes = computed(() => prizes.value.filter((p) => p.enabled))
const probabilityTotal = computed(() => enabledPrizes.value.reduce((sum, p) => sum + Number(p.probability || 0), 0))
const theoreticalExpense = computed(() => enabledPrizes.value.reduce((sum, p) => sum + Number(p.reward_amount || 0) * Number(p.probability || 0) / 100, 0))
const theoreticalProfit = computed(() => Number(settings.value.ticket_amount || 0) - theoreticalExpense.value)
const theoreticalProfitRate = computed(() => settings.value.ticket_amount > 0 ? theoreticalProfit.value / settings.value.ticket_amount : 0)
```

Show a blocking warning when `probabilityTotal !== 100`.

- [ ] **Step 4: Implement stats tab**

Controls:

- Start date.
- End date.
- Refresh button.

Sections:

- Overview cards: revenue, expense, profit, profit rate, participants, plays.
- User table: user, date, plays, ticket total, reward total, net total.
- Prize table: configured probability, actual hit rate, hit count, diff.

- [ ] **Step 5: Implement simulator tab**

Inputs:

- User count.
- Plays per user.
- Initial balance.
- Ticket amount.
- Minimum balance.
- Daily play limit.
- Editable prize list.
- Target profit rate.

Actions:

- Run simulation.
- Generate recommendation.
- Apply selected recommendation to formal prize config.

Use `data-testid="zenxiang-simulate"` and `data-testid="zenxiang-tab-simulator"` for tests.

- [ ] **Step 6: Verify admin page**

Run:

```bash
cd frontend
pnpm test --run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: command exits `0`.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/admin/ZenxiangLiyuAdminView.vue frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts frontend/src/i18n/locales
git commit -m "feat: add zenxiang liyu admin console"
```

---

### Task 8: Full Verification and Integration Cleanup

**Files:**
- Modify only files required by failed verification.

**Interfaces:**
- Consumes all prior tasks.
- Produces verified backend/frontend integration.

- [ ] **Step 1: Run backend focused tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'TestZenxiangLiyu|TestAdminZenxiangLiyu' -count=1
```

Expected: command exits `0`.

- [ ] **Step 2: Run Wire compile test**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./cmd/server -run 'TestWire' -count=1
```

Expected: command exits `0`.

- [ ] **Step 3: Run frontend focused tests**

Run:

```bash
cd frontend
pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts src/views/user/__tests__/ZenxiangLiyuView.spec.ts src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: command exits `0`.

- [ ] **Step 4: Run formatting**

Run:

```bash
cd backend
gofmt -w ent/schema/zenxiang_liyu_*.go internal/service/zenxiang_liyu*.go internal/repository/zenxiang_liyu*.go internal/handler/zenxiang_liyu*.go internal/handler/admin/zenxiang_liyu*.go internal/server/routes/user.go internal/server/routes/admin.go internal/handler/handler.go internal/handler/wire.go internal/service/wire.go
```

Expected: command exits `0`.

- [ ] **Step 5: Run final backend package tests**

Run:

```bash
cd backend
GOCACHE=/tmp/sub2api-go-cache go test ./internal/service ./internal/repository ./internal/handler ./internal/handler/admin ./internal/server/routes ./cmd/server -count=1
```

Expected: command exits `0`.

- [ ] **Step 6: Run final frontend tests**

Run:

```bash
cd frontend
pnpm test --run
```

Expected: command exits `0`.

- [ ] **Step 7: Optional browser verification**

Start frontend dev server:

```bash
cd frontend
pnpm dev --host 127.0.0.1 --port 3000
```

Expected: server starts. Manually verify:

- User sidebar hides `臻享礼遇` when backend status `visible=false`.
- User sidebar shows `臻享礼遇` when backend status `visible=true`.
- User page disables play when `can_play=false`.
- Admin page tabs render and simulator result appears after running.

- [ ] **Step 8: Commit final cleanup**

If verification changed files:

```bash
git add backend frontend
git commit -m "test: verify zenxiang liyu integration"
```

If verification did not change files, do not create an empty commit.

---

## Self-Review

Spec coverage:

- User-facing standalone `臻享礼遇` menu is covered by Tasks 5 and 6.
- Default off, global enable, and per-user grant are covered by Tasks 2, 3, 4, 5, and 7.
- Configurable ticket, minimum balance, daily limit, prize amount, probability, enabled state, and order are covered by Tasks 1, 2, 4, and 7.
- Independent probability draw is covered by Task 2.
- Transactional ticket deduction, reward credit, ledger snapshot, idempotency, and daily limit are covered by Task 3.
- User/admin stats are covered by Tasks 3, 4, and 7.
- Visual simulator and target-profit recommendation are covered by Tasks 2, 4, and 7.
- Backend-only trust boundary is covered by Tasks 3 and 4.
- Tests and verification are covered in every task and Task 8.

Placeholder scan:

- No placeholder tokens or empty implementation slots are intentionally left in this plan.
- Every task has concrete files, interfaces, commands, and expected outcomes.

Type consistency:

- Backend service names consistently use `ZenxiangLiyu`.
- Frontend file and API names consistently use `zenxiangLiyu`.
- Route paths consistently use `/zenxiang-liyu` and `/admin/zenxiang-liyu`.
