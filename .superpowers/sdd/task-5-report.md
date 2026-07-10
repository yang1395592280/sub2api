### Task 5 Report: Frontend Automatic Group Option and Submission

Status: DONE_WITH_CONCERNS

Changed files:
- `frontend/src/types/index.ts`
- `frontend/src/api/keys.ts`
- `frontend/src/views/user/keyGroupOptions.ts`
- `frontend/src/views/user/KeysView.vue`
- `frontend/src/views/user/__tests__/keyGroupOptions.spec.ts`
- `frontend/src/views/user/__tests__/KeysView.autoGroup.spec.ts`

Summary:
- Added frontend API key group selection mode types and last effective group fields.
- Added the `openai_auto_cheapest` sentinel option to the key group option builder.
- Updated key creation and update flows to send `group_select_mode`.
- Added a computed selected group value in `KeysView.vue` so automatic mode maps to `group_id: null`.
- Added automatic mode display in the key table with the latest effective group name when present.
- Kept inline row group switching fixed-group only and made it force `group_select_mode: fixed`.
- Treated automatic-mode keys as OpenAI when opening key usage/import flows that otherwise infer platform from `row.group`.

Tests run:
- `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts`
  - Passed, 2 tests.
- `cd frontend && pnpm test:run src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - First run failed because the test layout stub did not render named slots; fixed the test stub.
  - Passed, 1 test.
- `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - Passed, 2 files / 3 tests.
- `cd frontend && pnpm typecheck`
  - Passed.
- After local review fix, re-ran:
  - `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - `cd frontend && pnpm typecheck`
  - Both passed.
- `git diff --check -- frontend/src/api/keys.ts frontend/src/types/index.ts frontend/src/views/user/KeysView.vue frontend/src/views/user/keyGroupOptions.ts frontend/src/views/user/__tests__/keyGroupOptions.spec.ts frontend/src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - Passed with no output.

Concerns:
- The specified key group option test was already green when resuming because `keyGroupOptions.ts` had partial production changes before this handoff.
- Subagent review/explorer attempt failed with upstream `502 Bad Gateway`, so Task 5 review must be handled locally or retried later.

Review:
- Task reviewer returned Spec verdict: pass.
- Task reviewer returned Quality verdict: fail due to Minor i18n findings only.
- Fixed the i18n findings by adding `keys.openaiAutoCheapest.*` messages in zh/en and passing localized text into the group option builder.
- Re-ran:
  - `cd frontend && pnpm test:run src/views/user/__tests__/keyGroupOptions.spec.ts src/views/user/__tests__/KeysView.autoGroup.spec.ts`
  - `cd frontend && pnpm typecheck`
  - Both passed.

---

# Task 5 Report: Frontend APIs, Store, Routes, and Sidebar Visibility

## Status

Completed. The frontend plumbing for Zenxiang Liyu is implemented and committed separately from other in-progress task reports.

## Changed Files

- `frontend/src/api/zenxiangLiyu.ts`
- `frontend/src/api/admin/zenxiangLiyu.ts`
- `frontend/src/api/__tests__/zenxiangLiyu.spec.ts`
- `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts`
- `frontend/src/api/index.ts`
- `frontend/src/api/admin/index.ts`
- `frontend/src/stores/zenxiangLiyu.ts`
- `frontend/src/stores/index.ts`
- `frontend/src/router/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

## API Coverage

User API:

- `GET /zenxiang-liyu/status`
- `POST /zenxiang-liyu/play`
- `GET /zenxiang-liyu/records`
- `GET /zenxiang-liyu/daily-summary`

Admin API:

- `GET|PUT /admin/zenxiang-liyu/settings`
- `GET|POST|PUT /admin/zenxiang-liyu/prizes`
- `PUT|DELETE /admin/zenxiang-liyu/prizes/:id`
- `GET|POST /admin/zenxiang-liyu/grants`
- `DELETE /admin/zenxiang-liyu/grants/:user_id`
- `GET /admin/zenxiang-liyu/stats/overview`
- `GET /admin/zenxiang-liyu/stats/users`
- `GET /admin/zenxiang-liyu/stats/prizes`
- `POST /admin/zenxiang-liyu/simulate`
- `POST /admin/zenxiang-liyu/simulate/recommend`
- `POST /admin/zenxiang-liyu/simulate/apply`

All functions return the already-unwrapped `response.data` payload supplied by the shared client interceptor.

## Routes and Navigation

- Added authenticated user route `/zenxiang-liyu` and admin route `/admin/zenxiang-liyu` with localized title and description metadata.
- Added the `useZenxiangLiyuStore()` in-memory status cache. The user navigation item depends on backend `visible`; insufficient balance remains visible when the backend reports `visible: true`.
- Added the admin operations item, hidden only in simple mode. Both routes intentionally point to the Task 6/7 page components, which are outside this task's write scope.

## Verification

Passed:

```text
cd frontend && pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts
2 test files passed, 8 tests passed

cd frontend && pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts src/router/__tests__/guards.spec.ts
3 test files passed, 44 tests passed

cd frontend && pnpm typecheck
Exited 0
```

`git diff --check` also exited cleanly.

## Risks and Follow-up

- The lazily imported user/admin page components are intentionally not created here, per the Task 5 boundary; Task 6 and Task 7 must add them before navigating to the new routes in a running application.
- API request/response types reflect Task 4B's current Go contracts. Contract changes should update these types and their request-boundary tests together.
