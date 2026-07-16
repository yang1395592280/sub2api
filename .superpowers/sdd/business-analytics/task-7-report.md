# Task 7 Report: Frontend Business Analytics Page

Time: 2026-07-01 15:34:34 CST

## Summary

Implemented the admin Business Analytics page at `frontend/src/views/admin/BusinessAnalyticsView.vue`.

The page consumes `adminAPI.businessAnalytics` and provides:

- Overview metrics and trend table.
- Groups table.
- Channels table.
- Price impact comparison with group/date/window controls.
- Records table with pagination and historical approximation markers.
- Chinese i18n under `admin.businessAnalytics`.
- View tests covering default overview load, filter reload, tab API calls, records empty state, and approximate snapshot state.

## Changed Files

- `frontend/src/views/admin/BusinessAnalyticsView.vue`
- `frontend/src/views/admin/__tests__/BusinessAnalyticsView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`

## RED

Command:

```bash
cd frontend && pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts
```

Result: failed before implementation because `../BusinessAnalyticsView.vue` did not exist.

Failure summary:

- `Failed to resolve import "../BusinessAnalyticsView.vue"`

## GREEN

Commands:

```bash
cd frontend && pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts
cd frontend && pnpm vitest run src/api/admin/__tests__/businessAnalytics.spec.ts
cd frontend && pnpm exec vue-tsc --noEmit
```

Results:

- BusinessAnalyticsView spec: 4 tests passed.
- businessAnalytics API spec: 5 tests passed.
- vue-tsc: exit 0.

## API Field Gaps

The current backend API does not return per-record group rate snapshot or channel price snapshot fields, and price impact does not return user gained/lost counts.

The UI does not fabricate those values:

- Record snapshot cells show `API 暂未返回快照`.
- Rows are marked `历史近似` when the overview reports missing channel price records or when a record has zero channel cost.
- Price impact shows a note that user gained/lost counts are not currently returned.

## Visual Verification

Started dev server:

```bash
cd frontend && pnpm dev --host 127.0.0.1 --port 3000
```

Vite served at `http://127.0.0.1:3000/`.

Browser verification was blocked by the browser security policy for `http://127.0.0.1:3000`, so desktop/mobile screenshot validation could not be completed in-browser during this task.

## Concerns

- Visual QA still needs a real authenticated admin browser session or an allowed local browser target.
- Historical approximation is intentionally conservative because the API only exposes aggregate missing snapshot counts and not row-level snapshot metadata.

## 2026-07-01 Task 7 fix worker

- RED: `cd frontend && pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts` failed with 4 expected failures: missing average rate/price columns, missing price impact group selector, missing records summary test target with row-level approximation still shown, and count=0 records hint still rendered.
- GREEN: Added a loaded-groups based price impact group selector, removed overview-based row-level approximation markers, hid records approximation hint when count is 0, and surfaced API-missing average rate/price columns.
- Verification: `cd frontend && pnpm vitest run src/views/admin/__tests__/BusinessAnalyticsView.spec.ts` passed 6/6; `cd frontend && pnpm exec vue-tsc --noEmit` passed.
- Concerns: 区间平均倍率/价格仍依赖后端未来暴露字段；当前按 reviewer 要求明确显示“接口暂未返回”。
