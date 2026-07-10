# Task 5 Report: Frontend APIs, Store, Routes, and Sidebar Visibility

## Status

Completed. The frontend plumbing for Zenxiang Liyu is implemented and committed separately from other in-progress task reports.

## Changed Files

- `frontend/src/api/zenxiangLiyu.ts`
- `frontend/src/api/admin/zenxiangLiyu.ts`
- `frontend/src/api/__tests__/zenxiangLiyu.spec.ts`
- `frontend/src/api/admin/__tests__/zenxiangLiyu.spec.ts`
- `frontend/src/views/user/ZenxiangLiyuView.vue`
- `frontend/src/views/admin/ZenxiangLiyuAdminView.vue`
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
- Added the admin operations item, hidden only in simple mode. Both routes point to minimal parseable placeholder components pending Task 6/7 page implementation.

## Verification

Passed after this review fix:

```text
cd frontend && pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts src/router/__tests__/guards.spec.ts
3 test files passed, 44 tests passed

cd frontend && pnpm typecheck
Exited 0
```

## Review Fixes

- Added minimal parseable user and admin placeholder views for the lazy-loaded routes.
- Added optional `balance?: number` to `ZenxiangLiyuStatus`.
- Removed unrelated legacy report content from the top of this report.

### Review Fix Verification

Executed on 2026-07-10:

```text
cd frontend && pnpm test --run src/api/__tests__/zenxiangLiyu.spec.ts src/api/admin/__tests__/zenxiangLiyu.spec.ts src/router/__tests__/guards.spec.ts
3 test files passed, 44 tests passed

cd frontend && pnpm typecheck
Exited 0
```
