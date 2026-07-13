# Zenxiang Liyu Lucky Coin and User Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce the lucky-coin miss result to a net loss of half the original reward and let administrators expand a user's daily draw records from the user statistics table.

**Architecture:** Keep lucky-coin settlement in the repository transaction and update the database mutation guard through a new forward migration. Expose the existing per-user/date record query through an admin-only paginated endpoint, then load those records lazily into a single expanded row in the existing statistics page.

**Tech Stack:** Go, Gin, database/sql, PostgreSQL migrations, sqlmock, Vue 3, TypeScript, Vitest, Vue Test Utils, Tailwind CSS.

## Global Constraints

- A lucky-coin hit remains `adjustment = +R` and final reward `2R`.
- A lucky-coin miss becomes `adjustment = -1.5R` and final reward `-0.5R`.
- Existing settled records are not recalculated.
- Existing migration `180_zenxiang_liyu_lucky_coin_mutation_allowlist.sql` remains immutable; add migration `181_zenxiang_liyu_lucky_coin_half_loss.sql`.
- Admin details are restricted to the selected `user_id` and `YYYY-MM-DD` date and use the existing `ZenxiangLiyuRecord` DTO.
- Only one user detail row is expanded at a time, and details are loaded lazily.

---

### Task 1: Lucky-Coin Half-Loss Settlement

**Files:**
- Modify: `backend/internal/repository/zenxiang_liyu_repo_test.go`
- Modify: `backend/internal/repository/zenxiang_liyu_repo.go`
- Create: `backend/migrations/181_zenxiang_liyu_lucky_coin_half_loss.sql`
- Modify: `backend/migrations/auth_identity_payment_migrations_regression_test.go`
- Modify: `backend/internal/service/admin_user.go`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Interfaces:**
- Consumes: `zenxiangLiyuRepository.PlayLuckyCoin(ctx, service.ZenxiangLiyuLuckyCoinCommand)` and the existing `zenxiang_liyu_records_prevent_mutation()` trigger function.
- Produces: Miss settlement with `AdjustmentAmount == -1.5 * OriginalReward`; migration 181 permits exactly that append-only record mutation.

- [ ] **Step 1: Write the failing repository miss test**

Add `TestZenxiangLiyuRepositoryPlayLuckyCoinMissLosesHalfReward` beside the existing hit test. Use reward `1`, probability `60`, roll `80`, and sqlmock expectations for:

```go
mock.ExpectQuery(`UPDATE users`).
	WithArgs(-1.5, int64(42)).
	WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(8.5))
mock.ExpectQuery(`UPDATE zenxiang_liyu_records`).
	WithArgs("zero", -1.5, 8.5, int64(9), int64(42)).
	WillReturnRows(sqlmock.NewRows([]string{"lucky_coin_played_at"}).AddRow(playedAt))
```

Assert outcome `zero`, adjustment `-1.5`, balance `8.5`, and all SQL expectations.

- [ ] **Step 2: Run the repository test and verify RED**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository -run TestZenxiangLiyuRepositoryPlayLuckyCoinMissLosesHalfReward -count=1
```

Expected: FAIL because the current implementation sends adjustment `-2` to `UPDATE users`.

- [ ] **Step 3: Write the failing migration regression test**

Add `TestMigration181AllowsLuckyCoinHalfLoss` that reads `181_zenxiang_liyu_lucky_coin_half_loss.sql` and asserts the function replacement, `NEW.lucky_coin_adjustment = -1.5 * OLD.reward_amount`, the user/system accounting equations, JSON field allowlist, and immutable-record exception.

- [ ] **Step 4: Run the migration test and verify RED**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./migrations -run TestMigration181AllowsLuckyCoinHalfLoss -count=1
```

Expected: FAIL because migration 181 does not exist.

- [ ] **Step 5: Implement the new settlement formula and migration**

Change the miss default in `PlayLuckyCoin` to:

```go
outcome := "zero"
adjustment := -1.5 * record.reward
```

Create migration 181 by copying the complete trigger function from migration 180 and changing only the miss condition to:

```sql
OR (NEW.lucky_coin_outcome = 'zero' AND NEW.lucky_coin_adjustment = -1.5 * OLD.reward_amount)
```

Keep the prize foreign-key cleanup allowance, all accounting checks, the JSON field allowlist, and the final immutable-record exception unchanged.

- [ ] **Step 6: Update rule descriptions**

Use wording that matches the new behavior:

```text
中文设置提示：中奖后可翻牌，未中时最终损失原奖励的一半
English setting hint: After winning, users can flip once. A miss results in a net loss of half the original reward.
余额明细：幸运金币未中，减半损失：<奖品>
```

- [ ] **Step 7: Run focused tests and verify GREEN**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository ./migrations -run 'TestZenxiangLiyuRepositoryPlayLuckyCoin|TestMigration18(0|1)' -count=1
```

Expected: PASS, including the unchanged lucky-coin hit and migration 180 regression tests.

- [ ] **Step 8: Commit Task 1**

```bash
git add backend/internal/repository/zenxiang_liyu_repo.go backend/internal/repository/zenxiang_liyu_repo_test.go backend/migrations/181_zenxiang_liyu_lucky_coin_half_loss.sql backend/migrations/auth_identity_payment_migrations_regression_test.go backend/internal/service/admin_user.go frontend/src/i18n/locales/zh/admin/resources.ts frontend/src/i18n/locales/en/admin/resources.ts
git commit -m "fix: reduce zenxiang lucky coin miss loss"
```

### Task 2: Admin User Record Endpoint

**Files:**
- Modify: `backend/internal/service/zenxiang_liyu_service.go`
- Modify: `backend/internal/handler/admin/zenxiang_liyu_handler.go`
- Modify: `backend/internal/handler/admin/zenxiang_liyu_handler_test.go`
- Modify: `backend/internal/server/routes/admin.go`

**Interfaces:**
- Consumes: repository method `ListUserRecords(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]ZenxiangLiyuRecord, int, error)`.
- Produces: service method `ListUserRecordsByDate(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]ZenxiangLiyuRecord, int, error)` and `GET /admin/zenxiang-liyu/stats/users/:user_id/records`.

- [ ] **Step 1: Write failing handler tests**

Extend `stubAdminZenxiangLiyuService` with captured `userID`, `playDate`, `page`, and `pageSize`, then add:

```go
func TestAdminZenxiangLiyuGetUserRecordsMapsUserDateAndPagination(t *testing.T)
func TestAdminZenxiangLiyuGetUserRecordsRejectsInvalidUserID(t *testing.T)
func TestAdminZenxiangLiyuGetUserRecordsRejectsInvalidDate(t *testing.T)
```

The success request is:

```text
GET /admin/zenxiang-liyu/stats/users/42/records?date=2026-07-13&page=2&page_size=10
```

Assert status `200`, captured ID `42`, UTC date `2026-07-13`, page `2`, page size `10`, and a paginated response containing one record. Invalid ID/date requests must return `400` without invoking the service.

- [ ] **Step 2: Run handler tests and verify RED**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin -run TestAdminZenxiangLiyuGetUserRecords -count=1
```

Expected: build failure because the admin service interface and handler do not expose the records method.

- [ ] **Step 3: Add the date-aware service method**

Implement:

```go
func (s *ZenxiangLiyuService) ListUserRecordsByDate(ctx context.Context, userID int64, playDate time.Time, page, pageSize int) ([]ZenxiangLiyuRecord, int, error) {
	if s.repo == nil || userID <= 0 || playDate.IsZero() {
		return nil, 0, ErrZenxiangLiyuInvalidSettings
	}
	return s.repo.ListUserRecords(ctx, userID, playDate, normalizedZenxiangLiyuPage(page), normalizedZenxiangLiyuPageSize(pageSize))
}
```

Make the existing user-facing `ListUserRecords` delegate to this method with `s.playDate()` so both paths share validation and pagination behavior.

- [ ] **Step 4: Add the admin handler and route**

Add the method to `zenxiangLiyuAdminService`. Implement `GetUserRecords` to parse a positive `user_id`, require and parse `date` as `2006-01-02`, parse pagination through `response.ParsePagination`, call `ListUserRecordsByDate`, and return `response.Paginated`.

Register:

```go
zenxiangLiyu.GET("/stats/users/:user_id/records", h.Admin.ZenxiangLiyu.GetUserRecords)
```

- [ ] **Step 5: Run focused handler/service tests and verify GREEN**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin ./internal/service -run 'TestAdminZenxiangLiyuGetUserRecords|TestZenxiangLiyu' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

```bash
git add backend/internal/service/zenxiang_liyu_service.go backend/internal/handler/admin/zenxiang_liyu_handler.go backend/internal/handler/admin/zenxiang_liyu_handler_test.go backend/internal/server/routes/admin.go
git commit -m "feat: expose zenxiang user draw details to admins"
```

### Task 3: Admin Frontend API Contract

**Files:**
- Modify: `frontend/src/api/admin/zenxiangLiyu.ts`
- Modify: `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts`

**Interfaces:**
- Consumes: `ZenxiangLiyuRecord` from `frontend/src/api/zenxiangLiyu.ts` and `PaginatedResponse`.
- Produces: `listUserRecords(userId: number, params: ZenxiangLiyuPaginationParams & { date: string }): Promise<PaginatedResponse<ZenxiangLiyuRecord>>`.

- [ ] **Step 1: Write the failing API test**

Add a test that mocks a paginated record response, calls:

```ts
adminZenxiangLiyuAPI.listUserRecords(42, { date: '2026-07-13', page: 1, page_size: 100 })
```

and expects:

```ts
expect(get).toHaveBeenCalledWith('/admin/zenxiang-liyu/stats/users/42/records', {
  params: { date: '2026-07-13', page: 1, page_size: 100 },
})
```

- [ ] **Step 2: Run the API test and verify RED**

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/zenxiangLiyu.spec.ts
```

Expected: FAIL because `listUserRecords` is undefined.

- [ ] **Step 3: Implement and export the API method**

Import `ZenxiangLiyuRecord`, add a required-date parameter type, implement the GET request, and export `listUserRecords` through `adminZenxiangLiyuAPI`.

- [ ] **Step 4: Run the API test and verify GREEN**

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/zenxiangLiyu.spec.ts
```

Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add frontend/src/api/admin/zenxiangLiyu.ts frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts
git commit -m "feat: add admin zenxiang user records api"
```

### Task 4: Expandable User Statistics Details

**Files:**
- Modify: `frontend/src/views/admin/ZenxiangLiyuAdminView.vue`
- Modify: `frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Interfaces:**
- Consumes: `adminAPI.zenxiangLiyu.listUserRecords`, `ZenxiangLiyuUserStats`, `ZenxiangLiyuRecord`, `formatDateTime`, and the current `statsDate`.
- Produces: a clickable email row with `data-testid="zenxiang-user-stats-toggle-<user_id>"` and detail row `data-testid="zenxiang-user-stats-details-<user_id>"`.

- [ ] **Step 1: Write failing expand/collapse tests**

Seed `listUserStats` with user `42` and seed `listUserRecords` with a record containing a played time, prize, reward `1`, outcome `zero`, adjustment `-1.5`, and net amount `-0.5`. Add a test that clicks the email and asserts:

```ts
expect(api.listUserRecords).toHaveBeenCalledWith(42, {
  date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
  page: 1,
  page_size: 100,
})
expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('礼遇一档')
expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('-1.5')
expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('-0.5')
```

Click again and assert the detail row no longer exists. Add a second user and assert expanding it closes user 42.

- [ ] **Step 2: Write failing cache reset and state tests**

Add tests for:

- Switching `input[type="date"]` clears the expanded row and a subsequent expand performs a new request.
- A pending promise shows the loading copy.
- An empty response shows the empty copy.
- A rejected request shows the error copy and another click retries.

- [ ] **Step 3: Run the view tests and verify RED**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: FAIL because user emails are plain table cells and no detail API call exists.

- [ ] **Step 4: Implement view state and lazy loading**

Replace only the user `StatsTable` instance with a dedicated table in the stats section. Add:

```ts
const expandedUserID = ref<number | null>(null)
const userRecordCache = ref<Record<string, ZenxiangLiyuRecord[]>>({})
const userRecordLoading = ref(false)
const userRecordError = ref('')
```

Use cache keys `${statsDate.value}:${userID}`. `toggleUserDetails` closes the same user, switches directly to cached users, or loads page 1/page size 100 and stores successful results. A failed request keeps the row open with an error and does not populate the cache, allowing retry.

At the beginning of `loadStats`, set `expandedUserID` to `null`, replace the cache with `{}`, and clear loading/error state. Keep the existing prize statistics table unchanged.

- [ ] **Step 5: Render the detail table and states**

Render email as a text-style button with chevron icon and `aria-expanded`. Render a sibling `<tr>` with `colspan="6"` containing a horizontally scrollable, compact detail table. Use `formatDateTime(record.played_at)` and helpers for lucky-coin labels and signed amounts. Use stable table widths/minimum width so loading and result changes do not shift the surrounding layout.

Add Chinese and English strings for draw time, original reward, lucky result, adjustment amount, final net amount, not played, doubled, missed, loading, empty, and load failure.

- [ ] **Step 6: Run view tests and verify GREEN**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: PASS.

- [ ] **Step 7: Commit Task 4**

```bash
git add frontend/src/views/admin/ZenxiangLiyuAdminView.vue frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts frontend/src/i18n/locales/zh/admin/resources.ts frontend/src/i18n/locales/en/admin/resources.ts
git commit -m "feat: expand zenxiang user draw details"
```

### Task 5: Full Verification and Visual QA

**Files:**
- Verify all files modified in Tasks 1-4.

**Interfaces:**
- Consumes: completed backend and frontend behavior.
- Produces: fresh test, build, lint, and visual evidence.

- [ ] **Step 1: Format changed Go files**

Run:

```bash
gofmt -w backend/internal/repository/zenxiang_liyu_repo.go backend/internal/repository/zenxiang_liyu_repo_test.go backend/internal/service/zenxiang_liyu_service.go backend/internal/service/admin_user.go backend/internal/handler/admin/zenxiang_liyu_handler.go backend/internal/handler/admin/zenxiang_liyu_handler_test.go backend/internal/server/routes/admin.go backend/migrations/auth_identity_payment_migrations_regression_test.go
```

- [ ] **Step 2: Run backend regression tests**

Run:

```bash
cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/repository ./internal/service ./internal/handler/admin ./migrations -count=1
```

Expected: PASS with zero failed packages.

- [ ] **Step 3: Run frontend tests and static checks**

Run:

```bash
cd frontend && pnpm test:run src/api/admin/__tests__/zenxiangLiyu.spec.ts src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts src/views/user/__tests__/ZenxiangLiyuView.spec.ts
cd frontend && pnpm typecheck
cd frontend && pnpm lint:check
cd frontend && pnpm build
```

Expected: all commands exit `0` with no test failures, type errors, lint errors, or build errors.

- [ ] **Step 4: Start the development server and inspect the page**

Run `pnpm dev --host 127.0.0.1` from `frontend`, use an available port, then inspect the admin Zenxiang Liyu statistics page at desktop and mobile viewports. Verify email affordance, expand/collapse, horizontal overflow, loading/empty/error states, dark mode, and that no text or controls overlap.

- [ ] **Step 5: Review the final diff and status**

Run:

```bash
git diff --check
git status --short --branch
git log -5 --oneline
```

Confirm no unrelated files are modified, no required test is missing, and no process needed for verification remains running except the user-facing dev server.
