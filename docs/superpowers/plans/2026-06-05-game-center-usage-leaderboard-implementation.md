# Game Center And Usage Leaderboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the game center, sign-in bonus flow, lucky wheel, and size-bet features from the legacy fork into the current codebase as a pure points platform, add a user-facing usage leaderboard with dashboard preview, and append per-account elapsed time in the admin batch account test modal.

**Architecture:** Rebuild the legacy game-center feature set against the current `sub2api` structure instead of replaying legacy commits wholesale. Keep game points fully isolated from the current balance system, expose user-facing leaderboard data through dedicated DTOs with server-side masking, and add the batch-test elapsed-time enhancement as a localized admin accounts UI change.

**Tech Stack:** Go, Gin, SQL migrations, Vue 3, TypeScript, Vite, Vitest, Go unit/integration tests

---

### Task 1: Map current extension points and lock down settings/data boundaries

**Files:**
- Inspect: `docs/superpowers/specs/2026-06-05-game-center-usage-leaderboard-design.md`
- Inspect: `backend/internal/service/domain_constants.go`
- Inspect: `backend/internal/service/setting_service.go`
- Inspect: `backend/internal/service/settings_view.go`
- Inspect: `backend/internal/handler/dto/settings.go`
- Inspect: `backend/internal/server/routes/user.go`
- Inspect: `backend/internal/server/routes/admin.go`
- Inspect: `frontend/src/stores/app.ts`
- Inspect: `frontend/src/types/index.ts`
- Inspect: `frontend/src/router/index.ts`
- Inspect: `frontend/src/components/layout/AppSidebar.vue`

- [ ] **Step 1: Review the approved spec and extract the immutable rules**

Capture these rules in working notes before touching code:

```text
1. Points and balance must never convert into each other.
2. Sign-in, sign-in lucky bonus, lucky wheel, and size-bet all settle in points.
3. Usage leaderboard is login-only and must mask user identity server-side.
4. Dashboard gets preview cards; full leaderboard remains a dedicated page.
5. Batch account test remains the same flow, with per-account elapsed seconds appended.
```

- [ ] **Step 2: Inspect current public/admin settings exposure and list the insertion points**

Run:

```bash
rg -n "SettingKey|PublicSettings|GetPublicSettings|cachedPublicSettings|titleKey" \
  backend/internal/service/domain_constants.go \
  backend/internal/service/setting_service.go \
  backend/internal/service/settings_view.go \
  backend/internal/handler/dto/settings.go \
  frontend/src/stores/app.ts \
  frontend/src/types/index.ts \
  frontend/src/router/index.ts \
  frontend/src/components/layout/AppSidebar.vue
```

Expected:
- Existing settings exposure chain is identified end-to-end.
- Menu and route extension points are known before adding new game-center flags.

- [ ] **Step 3: Commit the discovery notes to the task log, not the repo**

Record a concise working checklist such as:

```text
- Public settings path: setting repo -> setting_service.GetPublicSettings -> dto/settings.go -> app store cache -> AppSidebar/routes.
- Admin settings path: admin setting handler read/write -> setting service -> DTO.
- User feature routes live in frontend router + backend/internal/server/routes/user.go.
- Admin feature routes live in frontend router + backend/internal/server/routes/admin.go.
```

No repository changes in this task.

### Task 2: Add failing backend tests for settings, point isolation, and route surfaces

**Files:**
- Modify: `backend/internal/service/setting_service_public_test.go`
- Modify: `backend/internal/service/setting_service_update_test.go`
- Create: `backend/internal/service/game_center_isolation_test.go`
- Create: `backend/internal/server/routes/game_center_routes_test.go`

- [ ] **Step 1: Add failing public-settings tests for game center and sign-in visibility**

Add cases in `backend/internal/service/setting_service_public_test.go` similar to the existing join-group tests:

```go
func TestSettingService_GetPublicSettings_ExposesGameCenterFlags(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyGameCenterEnabled:            "true",
			SettingKeyCheckinEnabled:               "true",
			SettingKeyCheckinLuckyBonusEnabled:     "true",
			SettingKeyLuckyWheelEnabled:            "true",
			SettingKeySizeBetEnabled:               "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.GameCenterEnabled)
	require.True(t, settings.CheckinEnabled)
	require.True(t, settings.CheckinLuckyBonusEnabled)
	require.True(t, settings.LuckyWheelEnabled)
	require.True(t, settings.SizeBetEnabled)
}
```

- [ ] **Step 2: Add failing update-path tests that reject balance/points coupling**

In `backend/internal/service/setting_service_update_test.go`, add a case that verifies no balance-to-points or points-to-balance settings are accepted in the new implementation contract:

```go
func TestSettingService_UpdateSettings_DoesNotPersistPointExchangeSettings(t *testing.T) {
	// Arrange an update payload that only contains pure game-center settings.
	// Assert the persisted keys include game-center/sign-in/game toggles,
	// and do not include any legacy exchange setting keys.
}
```

Use the actual update helper style already present in the file.

- [ ] **Step 3: Add a failing isolation test for points-only settlement semantics**

Create `backend/internal/service/game_center_isolation_test.go` with assertions like:

```go
func TestGameCenterSettlement_DoesNotTouchBalanceFields(t *testing.T) {
	// Stub repository/service collaborators and assert point mutations happen
	// without invoking balance history or recharge-related collaborators.
}
```

At this stage the test may fail due to missing service seams or missing types; that is acceptable.

- [ ] **Step 4: Add route-surface tests for new user/admin endpoints**

Create `backend/internal/server/routes/game_center_routes_test.go` mirroring the route tests style in the repo:

```go
func TestGameCenterAndLeaderboardRoutesExist(t *testing.T) {
	expected := map[string]struct{}{
		"GET /api/v1/game-center/overview": {},
		"GET /api/v1/game-center/ledger": {},
		"GET /api/v1/usage-leaderboard/overview": {},
		"GET /api/v1/usage-leaderboard/items": {},
		"GET /api/v1/user/checkin": {},
		"POST /api/v1/user/checkin": {},
		"POST /api/v1/user/checkin/lucky-bonus": {},
	}
	_ = expected
}
```

- [ ] **Step 5: Run targeted Go tests and verify they fail for missing implementation**

Run:

```bash
go test ./backend/internal/service -run "TestSettingService_GetPublicSettings_ExposesGameCenterFlags|TestSettingService_UpdateSettings_DoesNotPersistPointExchangeSettings|TestGameCenterSettlement_DoesNotTouchBalanceFields" -count=1
go test ./backend/internal/server/routes -run "TestGameCenterAndLeaderboardRoutesExist" -count=1
```

Expected:
- FAIL because the new flags, service behavior, or routes are not implemented yet.

- [ ] **Step 6: Commit the failing backend test scaffolding**

```bash
git add backend/internal/service/setting_service_public_test.go \
  backend/internal/service/setting_service_update_test.go \
  backend/internal/service/game_center_isolation_test.go \
  backend/internal/server/routes/game_center_routes_test.go
git commit -m "test(game-center): 添加后端设计约束用例"
```

### Task 3: Implement migrations and backend point-domain repositories

**Files:**
- Create: `backend/migrations/137_create_game_center_points.sql`
- Create: `backend/migrations/138_create_checkin_records.sql`
- Create: `backend/migrations/139_create_size_bet_game_tables.sql`
- Create: `backend/migrations/140_create_lucky_wheel_game_tables.sql`
- Create: `backend/migrations/141_enable_game_center_defaults.sql`
- Create: `backend/internal/repository/game_center_repo.go`
- Create: `backend/internal/repository/checkin_repo.go`
- Create: `backend/internal/repository/lucky_wheel_repo.go`
- Create: `backend/internal/repository/size_bet_repo.go`
- Create: `backend/internal/repository/game_center_repo_test.go`
- Create: `backend/internal/repository/checkin_repo_test.go`
- Create: `backend/internal/repository/lucky_wheel_repo_test.go`
- Create: `backend/internal/repository/size_bet_repo_test.go`

- [ ] **Step 1: Port the legacy game-center points schema into new forward-only migrations**

Use `../sub2api-大转盘/backend/migrations/113_add_game_center_points.sql` as the base, but remove any exchange-related tables or semantics. The new migration should include:

```sql
ALTER TABLE users
ADD COLUMN IF NOT EXISTS points BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS game_points_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    entry_type VARCHAR(64) NOT NULL,
    delta_points BIGINT NOT NULL,
    points_before BIGINT NOT NULL,
    points_after BIGINT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    related_claim_batch_key VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- [ ] **Step 2: Port check-in storage with lucky-bonus support**

Use `108_create_checkin_records.sql` and `109_add_checkin_lucky_bonus_columns.sql` from the legacy repo as source material. Keep the reward fields in points-oriented naming/comments and remove any wording that implies balance settlement.

- [ ] **Step 3: Port size-bet persistence tables**

Use `110_create_size_bet_game_tables.sql` and `111_preserve_size_bet_audit_history.sql` as the base. Keep:
- `game_rounds`
- `game_bets`
- `game_wallet_ledger`
- `game_rank_snapshots`

Ensure comments and naming clarify they are game-point ledgers, not balance ledgers.

- [ ] **Step 4: Port lucky-wheel persistence tables and default prize pool**

Use `115_add_lucky_wheel_game.sql` as the base. Retain prize definitions and history tables, but make sure labels and settlement semantics stay points-only.

- [ ] **Step 5: Add repository implementations adapted to current DB/repository patterns**

Port and adapt:
- `../sub2api-大转盘/backend/internal/repository/game_center_repo.go`
- `../sub2api-大转盘/backend/internal/repository/checkin_repo.go`
- `../sub2api-大转盘/backend/internal/repository/lucky_wheel_repo.go`
- `../sub2api-大转盘/backend/internal/repository/size_bet_repo.go`

Key constraints:

```text
- Do not call balance-history repositories.
- Do not reuse recharge/order/billing tables.
- Keep methods focused on point mutation + game record persistence.
```

- [ ] **Step 6: Add repository tests for core point-domain persistence**

Create:

```go
func TestGameCenterRepository_AdjustPoints_AppendsLedger(t *testing.T) {}
func TestCheckinRepository_CreateAndCredit_PersistsPointReward(t *testing.T) {}
func TestLuckyWheelRepository_CreateSpinRecord_PersistsHistory(t *testing.T) {}
func TestSizeBetRepository_CreateBetAndLedger_PersistsRoundData(t *testing.T) {}
```

Use the repository test style already present in `backend/internal/repository`.

- [ ] **Step 7: Run focused repository/migration verification**

Run:

```bash
go test ./backend/internal/repository -run "Test.*(SizeBet|Usage|Dashboard)" -count=1
```

Expected:
- Existing unrelated repository tests still pass.
- New repository tests pass.

- [ ] **Step 8: Commit the persistence layer**

```bash
git add backend/migrations/137_create_game_center_points.sql \
  backend/migrations/138_create_checkin_records.sql \
  backend/migrations/139_create_size_bet_game_tables.sql \
  backend/migrations/140_create_lucky_wheel_game_tables.sql \
  backend/migrations/141_enable_game_center_defaults.sql \
  backend/internal/repository/game_center_repo.go \
  backend/internal/repository/checkin_repo.go \
  backend/internal/repository/lucky_wheel_repo.go \
  backend/internal/repository/size_bet_repo.go
git commit -m "feat(game-center): 增加积分与游戏持久化"
```

### Task 4: Implement backend settings, DTOs, and route wiring for game-center features

**Files:**
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/setting_service.go`
- Modify: `backend/internal/service/settings_view.go`
- Modify: `backend/internal/handler/dto/settings.go`
- Modify: `backend/internal/server/routes/user.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `backend/internal/service/setting_service_public_test.go`
- Modify: `backend/internal/service/setting_service_update_test.go`
- Modify: `backend/internal/server/routes/game_center_routes_test.go`

- [ ] **Step 1: Add setting keys for game-center, sign-in, and game toggles**

In `backend/internal/service/domain_constants.go`, add constants modeled after the legacy repo:

```go
const (
	SettingKeyGameCenterEnabled            = "game_center_enabled"
	SettingKeyCheckinEnabled               = "checkin_enabled"
	SettingKeyCheckinMinReward             = "checkin_min_reward"
	SettingKeyCheckinMaxReward             = "checkin_max_reward"
	SettingKeyCheckinDistributionEnabled   = "checkin_distribution_enabled"
	SettingKeyCheckinDistributionConfig    = "checkin_distribution_config"
	SettingKeyCheckinLuckyBonusEnabled     = "checkin_lucky_bonus_enabled"
	SettingKeyCheckinLuckyBonusSuccessRate = "checkin_lucky_bonus_success_rate"
	SettingKeyLuckyWheelEnabled            = "lucky_wheel_enabled"
	SettingKeySizeBetEnabled               = "size_bet_enabled"
)
```

- [ ] **Step 2: Expose the new settings through public and admin settings views**

Adapt `setting_service.go`, `settings_view.go`, and `dto/settings.go` so public settings include booleans/numeric config needed by user pages. Do not add any point-exchange settings.

Representative DTO fields:

```go
GameCenterEnabled            bool    `json:"game_center_enabled"`
CheckinEnabled               bool    `json:"checkin_enabled"`
CheckinMinReward             float64 `json:"checkin_min_reward"`
CheckinMaxReward             float64 `json:"checkin_max_reward"`
CheckinLuckyBonusEnabled     bool    `json:"checkin_lucky_bonus_enabled"`
CheckinLuckyBonusSuccessRate float64 `json:"checkin_lucky_bonus_success_rate"`
LuckyWheelEnabled            bool    `json:"lucky_wheel_enabled"`
SizeBetEnabled               bool    `json:"size_bet_enabled"`
```

- [ ] **Step 3: Wire user and admin route groups for new feature families**

Update `backend/internal/server/routes/user.go` and `admin.go` to register:

```go
user.GET("/checkin", h.User.GetCheckinStatus)
user.POST("/checkin", h.User.DoCheckin)
user.POST("/checkin/lucky-bonus", h.User.PlayCheckinLuckyBonus)

gameCenter := auth.Group("/game-center")
gameCenter.GET("/overview", h.GameCenter.GetOverview)
gameCenter.GET("/ledger", h.GameCenter.ListLedger)

adminGameCenter := admin.Group("/game-center")
adminGameCenter.GET("/settings", h.Admin.GameCenter.GetSettings)
```

Also add the usage leaderboard endpoints.

- [ ] **Step 4: Re-run the failing tests from Task 2 and make them pass**

Run:

```bash
go test ./backend/internal/service -run "TestSettingService_GetPublicSettings_ExposesGameCenterFlags|TestSettingService_UpdateSettings_DoesNotPersistPointExchangeSettings" -count=1
go test ./backend/internal/server/routes -run "TestGameCenterAndLeaderboardRoutesExist" -count=1
```

Expected:
- PASS

- [ ] **Step 5: Commit the settings and route wiring**

```bash
git add backend/internal/service/domain_constants.go \
  backend/internal/service/setting_service.go \
  backend/internal/service/settings_view.go \
  backend/internal/handler/dto/settings.go \
  backend/internal/server/routes/user.go \
  backend/internal/server/routes/admin.go \
  backend/internal/service/setting_service_public_test.go \
  backend/internal/service/setting_service_update_test.go \
  backend/internal/server/routes/game_center_routes_test.go
git commit -m "feat(game-center): 暴露开关与路由"
```

### Task 5: Implement backend sign-in, lucky bonus, and game-center overview services/handlers

**Files:**
- Create: `backend/internal/service/checkin_distribution.go`
- Create: `backend/internal/service/checkin_service.go`
- Create: `backend/internal/service/game_center_service.go`
- Create: `backend/internal/handler/user_checkin_handler.go`
- Create: `backend/internal/handler/game_center_handler.go`
- Create: `backend/internal/service/checkin_service_test.go`
- Create: `backend/internal/service/game_center_service_test.go`

- [ ] **Step 1: Port the sign-in distribution helper and write failing unit tests**

Port `../sub2api-大转盘/backend/internal/service/checkin_distribution.go` and add tests covering:
- invalid JSON
- incomplete range coverage
- valid weighted config

Representative failing test:

```go
func TestParseCheckinDistribution_RejectsIncompleteCoverage(t *testing.T) {
	_, err := ParseCheckinDistribution(`[{"start":0,"end":50,"weight":1}]`)
	require.Error(t, err)
}
```

- [ ] **Step 2: Port sign-in service logic with points-only settlement**

Base it on the legacy `checkin_service.go`, but remove any dependency on balance reward flows. Ensure these methods exist:

```go
func (s *CheckinService) GetStatus(ctx context.Context, userID int64, tz string) (*CheckinStatus, error)
func (s *CheckinService) Checkin(ctx context.Context, userID int64, tz string) (*CheckinRecord, error)
func (s *CheckinService) PlayLuckyBonus(ctx context.Context, userID int64, tz string) (*CheckinRecord, error)
```

- [ ] **Step 3: Port game-center overview service**

Base it on legacy `game_center_service.go`, but remove exchange sections. The overview DTO should expose:

```go
type GameCenterOverview struct {
	GameCenterEnabled bool
	Points            int64
	ClaimBatches      []GameCenterClaimBatch
	Catalogs          []GameCatalog
	PointsLeaderboard []GameCenterPointsLeaderboardItem
}
```

No `balance_to_points_*` or `points_to_balance_*` fields.

- [ ] **Step 4: Add user handlers and route wiring tests**

Implement:
- `GetCheckinStatus`
- `DoCheckin`
- `PlayCheckinLuckyBonus`
- `GetOverview`
- `ListLedger`

Follow current handler style and DTO mapping patterns.

- [ ] **Step 5: Run focused Go tests**

Run:

```bash
go test ./backend/internal/service -run "Test.*(Checkin|GameCenter)" -count=1
go test ./backend/internal/handler -run "Test.*(Checkin|GameCenter)" -count=1
```

Expected:
- PASS

- [ ] **Step 6: Commit the sign-in and overview backend**

```bash
git add backend/internal/service/checkin_distribution.go \
  backend/internal/service/checkin_service.go \
  backend/internal/service/game_center_service.go \
  backend/internal/handler/user_checkin_handler.go \
  backend/internal/handler/game_center_handler.go \
  backend/internal/service/checkin_service_test.go \
  backend/internal/service/game_center_service_test.go
git commit -m "feat(game-center): 实现签到与中心总览"
```

### Task 6: Implement backend lucky-wheel and size-bet runtime/admin flows

**Files:**
- Create: `backend/internal/service/lucky_wheel_service.go`
- Create: `backend/internal/service/lucky_wheel_admin_service.go`
- Create: `backend/internal/service/size_bet_service.go`
- Create: `backend/internal/service/size_bet_runtime_service.go`
- Create: `backend/internal/service/size_bet_admin_service.go`
- Create: `backend/internal/service/size_bet_types.go`
- Create: `backend/internal/handler/lucky_wheel_handler.go`
- Create: `backend/internal/handler/size_bet_handler.go`
- Create: `backend/internal/handler/admin/lucky_wheel_handler.go`
- Create: `backend/internal/handler/admin/size_bet_handler.go`
- Create tests adapted from the legacy repo for the above files

- [ ] **Step 1: Add failing tests for lucky-wheel point settlement**

Port/adapt from `../sub2api-大转盘/backend/internal/service/lucky_wheel_service_test.go`:

```go
func TestLuckyWheelService_SpinMutatesPointsAndHistory(t *testing.T) {
	// Assert points_before/points_after and leaderboard fields are populated.
}
```

- [ ] **Step 2: Implement lucky-wheel service and handler**

Port `lucky_wheel_service.go` and `lucky_wheel_handler.go`, preserving:
- prize probabilities
- daily limits
- leaderboard
- recent history

Ensure no balance collaborator exists in constructor or service methods.

- [ ] **Step 3: Add failing tests for size-bet round lifecycle and stats**

Adapt tests from:
- `size_bet_service_test.go`
- `size_bet_runtime_service_test.go`
- `size_bet_handler_test.go`
- `admin/size_bet_handler_test.go`

Cover:
- current round retrieval
- placing a bet
- history listing
- stats overview/users
- admin round/ledger listing

- [ ] **Step 4: Implement size-bet runtime/admin services and handlers**

Port the legacy size-bet files and keep the public/admin route split. Preserve idempotency-key support and refund/stat flows where already modeled in the legacy implementation.

- [ ] **Step 5: Run focused Go tests**

Run:

```bash
go test ./backend/internal/service -run "Test.*(LuckyWheel|SizeBet)" -count=1
go test ./backend/internal/handler -run "Test.*(LuckyWheel|SizeBet)" -count=1
go test ./backend/internal/server/routes -run "Test.*SizeBet" -count=1
```

Expected:
- PASS

- [ ] **Step 6: Commit the game runtime/admin backend**

```bash
git add backend/internal/service/lucky_wheel_service.go \
  backend/internal/service/lucky_wheel_admin_service.go \
  backend/internal/service/size_bet_service.go \
  backend/internal/service/size_bet_runtime_service.go \
  backend/internal/service/size_bet_admin_service.go \
  backend/internal/service/size_bet_types.go \
  backend/internal/handler/lucky_wheel_handler.go \
  backend/internal/handler/size_bet_handler.go \
  backend/internal/handler/admin/lucky_wheel_handler.go \
  backend/internal/handler/admin/size_bet_handler.go
git commit -m "feat(game-center): 实现游戏玩法后端"
```

### Task 7: Implement backend usage leaderboard aggregation with masked DTOs

**Files:**
- Create: `backend/internal/service/usage_leaderboard_service.go`
- Create: `backend/internal/handler/usage_leaderboard_handler.go`
- Create: `backend/internal/repository/usage_leaderboard_repo.go`
- Create: `backend/internal/service/usage_leaderboard_service_test.go`
- Create: `backend/internal/handler/usage_leaderboard_handler_test.go`
- Modify: `backend/internal/server/routes/user.go`

- [ ] **Step 1: Write failing tests for requests/tokens ranking and masking**

Example service-level cases:

```go
func TestUsageLeaderboardService_BuildsRequestsRanking(t *testing.T) {}
func TestUsageLeaderboardService_BuildsTokenRanking(t *testing.T) {}
func TestUsageLeaderboardService_MasksUsernamesAndEmails(t *testing.T) {}
```

Ensure the token total uses:

```text
input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens
```

- [ ] **Step 2: Implement repository queries against existing usage data**

Create a repository that aggregates by date and user. The SQL should explicitly select only needed columns and compute:

```sql
COUNT(*) FILTER (WHERE success = true) AS success_requests,
SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens) AS total_tokens
```

Adjust to the actual schema names in `usage_logs`.

- [ ] **Step 3: Implement service masking helpers and output DTOs**

Add helpers like:

```go
func maskUsername(raw string) string
func maskEmail(raw string) string
```

Make handler responses expose only masked names, rank, metric value, and date metadata.

- [ ] **Step 4: Implement handler endpoints and route registration**

Expose:

```go
GET /api/v1/usage-leaderboard/overview
GET /api/v1/usage-leaderboard/items
```

Validate:
- `date`
- `metric` in `requests|tokens`

- [ ] **Step 5: Run focused Go tests**

Run:

```bash
go test ./backend/internal/service -run "Test.*UsageLeaderboard" -count=1
go test ./backend/internal/handler -run "Test.*UsageLeaderboard" -count=1
```

Expected:
- PASS

- [ ] **Step 6: Commit the leaderboard backend**

```bash
git add backend/internal/service/usage_leaderboard_service.go \
  backend/internal/handler/usage_leaderboard_handler.go \
  backend/internal/repository/usage_leaderboard_repo.go \
  backend/internal/service/usage_leaderboard_service_test.go \
  backend/internal/handler/usage_leaderboard_handler_test.go \
  backend/internal/server/routes/user.go
git commit -m "feat(usage): 增加用户排行榜接口"
```

### Task 8: Add frontend API/types/store support for game-center and leaderboard features

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/types/gameCenter.ts`
- Create: `frontend/src/types/luckyWheel.ts`
- Create: `frontend/src/types/sizeBet.ts`
- Modify: `frontend/src/stores/app.ts`
- Create: `frontend/src/api/gameCenter.ts`
- Create: `frontend/src/api/checkin.ts`
- Create: `frontend/src/api/usageLeaderboard.ts`
- Modify: `frontend/src/api/index.ts`

- [ ] **Step 1: Add API contract tests for the new frontend helpers**

Create:

```ts
frontend/src/api/__tests__/gameCenter.spec.ts
frontend/src/api/__tests__/checkin.spec.ts
frontend/src/api/__tests__/usageLeaderboard.spec.ts
```

Cover:
- request path construction
- query parameter forwarding
- response shape passthrough

- [ ] **Step 2: Port and adapt legacy frontend types**

Base the new files on:
- `../sub2api-大转盘/frontend/src/types/gameCenter.ts`
- `../sub2api-大转盘/frontend/src/types/luckyWheel.ts`
- `../sub2api-大转盘/frontend/src/types/sizeBet.ts`

Remove exchange-related fields such as:

```ts
balance_to_points_enabled
points_to_balance_enabled
balance_to_points_rate
points_to_balance_rate
```

- [ ] **Step 3: Extend public settings cache shape**

In `frontend/src/stores/app.ts` and `frontend/src/types/index.ts`, add:

```ts
game_center_enabled: boolean
checkin_enabled: boolean
checkin_min_reward: number
checkin_max_reward: number
checkin_lucky_bonus_enabled: boolean
checkin_lucky_bonus_success_rate: number
lucky_wheel_enabled: boolean
size_bet_enabled: boolean
```

- [ ] **Step 4: Add frontend API modules**

Create thin API wrappers such as:

```ts
export const gameCenterAPI = {
  getOverview: () => api.get('/game-center/overview'),
  getLedger: (params) => api.get('/game-center/ledger', { params }),
}

export const checkinAPI = {
  getStatus: () => api.get('/user/checkin'),
  checkin: () => api.post('/user/checkin'),
  playLuckyBonus: () => api.post('/user/checkin/lucky-bonus'),
}

export const usageLeaderboardAPI = {
  getOverview: (date: string, metric: 'requests' | 'tokens') => ...,
  getItems: (params) => ...,
}
```

- [ ] **Step 5: Commit the frontend data contract layer**

```bash
git add frontend/src/types/index.ts \
  frontend/src/types/gameCenter.ts \
  frontend/src/types/luckyWheel.ts \
  frontend/src/types/sizeBet.ts \
  frontend/src/stores/app.ts \
  frontend/src/api/gameCenter.ts \
  frontend/src/api/checkin.ts \
  frontend/src/api/usageLeaderboard.ts \
  frontend/src/api/index.ts
git commit -m "feat(frontend): 增加游戏中心数据契约"
```

### Task 9: Implement frontend routes, sidebar entries, and user-facing game pages

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Modify: `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- Create: `frontend/src/views/user/GameCenterView.vue`
- Create: `frontend/src/views/user/GameCenterShellView.vue`
- Create: `frontend/src/views/user/LuckyWheelGameView.vue`
- Create: `frontend/src/views/user/SizeBetGameView.vue`
- Create: `frontend/src/views/user/SizeBetStatsView.vue`
- Create: `frontend/src/views/user/__tests__/GameCenterView.spec.ts`
- Create: `frontend/src/views/user/__tests__/GameCenterShellView.spec.ts`
- Create: `frontend/src/views/user/__tests__/SizeBetGameView.spec.ts`
- Create: `frontend/src/views/user/__tests__/SizeBetStatsView.spec.ts`

- [ ] **Step 1: Add failing sidebar/route tests for the new entries**

Extend `frontend/src/components/layout/__tests__/AppSidebar.spec.ts` with source-based expectations:

```ts
expect(componentSource).toContain("/game-center")
expect(componentSource).toContain("/usage-leaderboard")
```

Add router-guard expectations if current route tests already cover user route behavior.

- [ ] **Step 2: Add the new user routes**

Update `frontend/src/router/index.ts` with:

```ts
{ path: '/game-center', name: 'GameCenter', component: () => import('@/views/user/GameCenterView.vue'), meta: { requiresAuth: true, requiresAdmin: false, titleKey: 'nav.gameCenter' } }
{ path: '/game-center/:gameKey', ... }
{ path: '/game/lucky-wheel', ... }
{ path: '/game/size-bet', ... }
{ path: '/game/size-bet/stats', ... }
{ path: '/usage-leaderboard', ... }
```

- [ ] **Step 3: Add user sidebar entries gated by public settings**

In `AppSidebar.vue`, add feature-flagged user items:

```ts
{ path: '/game-center', label: t('nav.gameCenter'), icon: GiftIcon, featureFlag: () => appStore.cachedPublicSettings?.game_center_enabled !== false }
{ path: '/usage-leaderboard', label: t('nav.usageLeaderboard'), icon: ChartIcon }
```

- [ ] **Step 4: Port the user-facing game pages**

Use these legacy files as the base:
- `GameCenterView.vue`
- `GameCenterShellView.vue`
- `LuckyWheelGameView.vue`
- `SizeBetGameView.vue`
- `SizeBetStatsView.vue`

Mandatory adaptations:

```text
- Remove exchange UI and API calls.
- Replace any balance-related copy with point-only copy.
- Preserve sign-in and lucky bonus entry points.
- Preserve stats/leaderboard views for the games.
```

- [ ] **Step 5: Run focused frontend tests for sidebar and game pages**

Run:

```bash
npm test -- --runInBand frontend/src/components/layout/__tests__/AppSidebar.spec.ts
npm test -- --runInBand frontend/src/views/user/__tests__/GameCenterView.spec.ts frontend/src/views/user/__tests__/SizeBetStatsView.spec.ts
```

Expected:
- PASS

- [ ] **Step 6: Commit the user-facing game UI**

```bash
git add frontend/src/router/index.ts \
  frontend/src/components/layout/AppSidebar.vue \
  frontend/src/components/layout/__tests__/AppSidebar.spec.ts \
  frontend/src/views/user/GameCenterView.vue \
  frontend/src/views/user/GameCenterShellView.vue \
  frontend/src/views/user/LuckyWheelGameView.vue \
  frontend/src/views/user/SizeBetGameView.vue \
  frontend/src/views/user/SizeBetStatsView.vue \
  frontend/src/views/user/__tests__/GameCenterView.spec.ts \
  frontend/src/views/user/__tests__/GameCenterShellView.spec.ts \
  frontend/src/views/user/__tests__/SizeBetGameView.spec.ts \
  frontend/src/views/user/__tests__/SizeBetStatsView.spec.ts
git commit -m "feat(frontend): 增加用户游戏中心页面"
```

### Task 10: Implement admin game-center pages and sign-in management UI

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/AppSidebar.vue`
- Create: `frontend/src/api/admin/checkins.ts`
- Create: `frontend/src/views/admin/GameCenterAdminView.vue`
- Create: `frontend/src/views/admin/LuckyWheelAdminView.vue`
- Create: `frontend/src/views/admin/SizeBetAdminView.vue`
- Create: `frontend/src/views/admin/CheckinsView.vue`
- Create: `frontend/src/views/admin/CheckinAnalyticsView.vue`
- Create supporting admin checkin analytics components as needed

- [ ] **Step 1: Port admin API helpers and add route/menu entries**

Use the legacy admin views and API files as the starting point. Add admin routes for:
- `/admin/game-center`
- `/admin/games/lucky-wheel`
- `/admin/games/size-bet`
- `/admin/checkins`
- `/admin/checkin-analytics`

- [ ] **Step 2: Port admin pages with point-only wording**

Update all labels and help text so they talk about points, sign-in rewards, and game history only. Remove balance exchange management.

- [ ] **Step 3: Add router/source tests for the admin entries**

Create or extend source-based tests so they assert the admin router/sidebar references:

```ts
expect(componentSource).toContain("/admin/game-center")
expect(componentSource).toContain("/admin/games/lucky-wheel")
expect(componentSource).toContain("/admin/games/size-bet")
expect(componentSource).toContain("/admin/checkins")
expect(componentSource).toContain("/admin/checkin-analytics")
```

- [ ] **Step 4: Commit the admin UI**

```bash
git add frontend/src/router/index.ts \
  frontend/src/components/layout/AppSidebar.vue \
  frontend/src/api/admin/checkins.ts \
  frontend/src/views/admin/GameCenterAdminView.vue \
  frontend/src/views/admin/LuckyWheelAdminView.vue \
  frontend/src/views/admin/SizeBetAdminView.vue \
  frontend/src/views/admin/CheckinsView.vue \
  frontend/src/views/admin/CheckinAnalyticsView.vue
git commit -m "feat(admin): 增加游戏中心后台页面"
```

### Task 11: Implement frontend usage leaderboard page and dashboard previews

**Files:**
- Create: `frontend/src/views/user/UsageLeaderboardView.vue`
- Modify: `frontend/src/views/user/DashboardView.vue`
- Create: `frontend/src/components/user/dashboard/UserGameCenterPreviewCard.vue`
- Create: `frontend/src/components/user/dashboard/UserUsageLeaderboardPreviewCard.vue`
- Create tests for the new leaderboard page and dashboard preview

- [ ] **Step 1: Add a failing test or shallow render for the leaderboard page**

At minimum cover:
- requests/tokens toggle
- date input
- masked item rendering

Representative assertions:

```ts
expect(wrapper.text()).toContain('请求')
expect(wrapper.text()).toContain('Token')
expect(wrapper.text()).toContain('ben***')
```

- [ ] **Step 2: Implement `UsageLeaderboardView.vue`**

The page should include:
- date selector
- metric toggle
- top 3 hero area
- top 10 list area

Match the approved visual direction from the brainstormed mockup rather than reusing the generic usage table layout.

- [ ] **Step 3: Extend the dashboard with preview cards**

Modify `frontend/src/views/user/DashboardView.vue` to add:

```text
- game-center quick status card
- usage leaderboard preview card
```

Keep existing stats/charts/recent usage hierarchy intact.

- [ ] **Step 4: Run focused frontend tests**

Run:

```bash
npm test -- --runInBand frontend/src/views/user/__tests__/UsageLeaderboardView.spec.ts
npm test -- --runInBand frontend/src/views/user/__tests__/DashboardView.spec.ts
```

Expected:
- PASS

- [ ] **Step 5: Commit the leaderboard and dashboard preview UI**

```bash
git add frontend/src/views/user/UsageLeaderboardView.vue \
  frontend/src/views/user/DashboardView.vue \
  frontend/src/views/user/__tests__/UsageLeaderboardView.spec.ts \
  frontend/src/views/user/__tests__/DashboardView.spec.ts
git commit -m "feat(frontend): 增加用量排行榜与首页预览"
```

### Task 12: Add per-account elapsed time to batch account test modal

**Files:**
- Modify: `frontend/src/components/admin/account/BatchAccountTestModal.vue`
- Modify: `frontend/src/components/admin/account/__tests__/BatchAccountTestModal.spec.ts`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1: Add a failing modal test for elapsed-time output**

Extend `BatchAccountTestModal.spec.ts` with a case like:

```ts
it('appends elapsed seconds for each account result', async () => {
  vi.spyOn(Date, 'now')
    .mockReturnValueOnce(0)
    .mockReturnValueOnce(2310)
  // trigger one account test
  expect(wrapper.text()).toContain('2.31')
})
```

- [ ] **Step 2: Implement per-account timing in the modal**

In `BatchAccountTestModal.vue`, wrap each account test with:

```ts
const startedAt = Date.now()
// await fetch + parse
const elapsedSeconds = ((Date.now() - startedAt) / 1000).toFixed(2)
addLine(`耗时 ${elapsedSeconds} 秒`, 'text-slate-400')
```

Prefer appending the elapsed time to the same per-account result block so users can associate it with the specific account.

- [ ] **Step 3: Add locale strings for elapsed output**

Add only the minimal strings required, e.g.:

```ts
bulkTestElapsed: '耗时 {seconds} 秒'
bulkTestElapsed: 'Elapsed {seconds}s'
```

- [ ] **Step 4: Run focused frontend test**

Run:

```bash
npm test -- --runInBand frontend/src/components/admin/account/__tests__/BatchAccountTestModal.spec.ts
```

Expected:
- PASS

- [ ] **Step 5: Commit the batch-test enhancement**

```bash
git add frontend/src/components/admin/account/BatchAccountTestModal.vue \
  frontend/src/components/admin/account/__tests__/BatchAccountTestModal.spec.ts \
  frontend/src/i18n/locales/zh.ts \
  frontend/src/i18n/locales/en.ts
git commit -m "feat(accounts): 显示批量测试单条耗时"
```

### Task 13: Final integration verification

**Files:**
- Review only: all changed files from Tasks 2-12

- [ ] **Step 1: Run targeted backend test groups**

Run:

```bash
go test ./backend/internal/service -run "Test.*(GameCenter|Checkin|LuckyWheel|SizeBet|UsageLeaderboard|SettingService)" -count=1
go test ./backend/internal/handler -run "Test.*(GameCenter|Checkin|LuckyWheel|SizeBet|UsageLeaderboard)" -count=1
go test ./backend/internal/server/routes -run "Test.*(GameCenter|SizeBet|Leaderboard)" -count=1
```

Expected:
- PASS

- [ ] **Step 2: Run targeted frontend tests**

Run:

```bash
npm test -- --runInBand \
  frontend/src/components/admin/account/__tests__/BatchAccountTestModal.spec.ts \
  frontend/src/components/layout/__tests__/AppSidebar.spec.ts \
  frontend/src/views/user/__tests__/GameCenterView.spec.ts \
  frontend/src/views/user/__tests__/GameCenterShellView.spec.ts \
  frontend/src/views/user/__tests__/SizeBetGameView.spec.ts \
  frontend/src/views/user/__tests__/SizeBetStatsView.spec.ts \
  frontend/src/views/user/__tests__/UsageLeaderboardView.spec.ts \
  frontend/src/views/user/__tests__/DashboardView.spec.ts
```

Expected:
- PASS

- [ ] **Step 3: Run frontend production build**

Run:

```bash
npm run build --prefix frontend
```

Expected:
- Build completes successfully with the new routes and pages.

- [ ] **Step 4: Run a final diff scope review**

Run:

```bash
git status --short
git diff --stat custom-main...
```

Expected:
- Only game-center, sign-in, leaderboard, dashboard preview, settings exposure, and batch-test timing changes are present.
- No accidental payment, balance, or unrelated account-flow regressions are included.

- [ ] **Step 5: Commit any final fixups and prepare for execution handoff**

```bash
git add -A
git commit -m "chore(game-center): 完成联调与验证"
```
