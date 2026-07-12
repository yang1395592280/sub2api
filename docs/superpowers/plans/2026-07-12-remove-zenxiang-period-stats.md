# Remove Zenxiang Period Stats Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the overview and period sections from the Zenxiang Liyu admin statistics page and stop issuing their expensive API requests.

**Architecture:** Keep the existing backend endpoints for compatibility while removing their two frontend call sites and all dependent component state. Preserve prize hit-rate calculations by deriving the total play count from the prize-stat response instead of the removed overview response.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, pnpm

## Global Constraints

- Keep the user-statistics, prize-statistics, and grants requests and displays.
- Do not remove or change backend API routes.
- Do not call `getOverviewStats()` or `listPeriodStats()` when opening or refreshing the statistics tab.
- Follow TDD: verify the focused test fails before modifying the component.

---

### Task 1: Remove Slow Statistics UI and Requests

**Files:**
- Modify: `frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts`
- Modify: `frontend/src/views/admin/ZenxiangLiyuAdminView.vue`

**Interfaces:**
- Consumes: `adminAPI.zenxiangLiyu.listUserStats`, `listPrizeStats`, and `listGrants`.
- Produces: a statistics tab that displays only the date selector, user statistics, and prize statistics.

- [ ] **Step 1: Write the failing component test**

Replace the existing statistics test with assertions that the retained requests execute, the slow requests do not execute, and the removed sections are absent:

```ts
it('loads only user, prize, and grant data for the stats tab', async () => {
  const wrapper = mountView()
  await flushPromises()
  vi.clearAllMocks()

  await wrapper.find('[data-testid="zenxiang-tab-stats"]').trigger('click')
  await flushPromises()

  expect(api.listUserStats).toHaveBeenCalledWith(expect.objectContaining({
    page_size: 100,
    date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
  }))
  expect(api.listPrizeStats).toHaveBeenCalledOnce()
  expect(api.listGrants).toHaveBeenCalledWith({ page_size: 100 })
  expect(api.getOverviewStats).not.toHaveBeenCalled()
  expect(api.listPeriodStats).not.toHaveBeenCalled()
  expect(wrapper.text()).not.toContain('admin.zenxiangLiyu.periodStats')
  expect(wrapper.text()).not.toContain('admin.zenxiangLiyu.totalDraws')
  expect(wrapper.text()).toContain('+15%')
})
```

Update the prize-stat fixture to contain 12 total hits so the retained `+15%` assertion continues to represent `9 / 12 - 60%`:

```ts
api.listPrizeStats.mockResolvedValue([
  { prize_name: '礼遇一档', prize_id: 1, probability: 60, hit_count: 9, reward_amount: 9 },
  { prize_name: '礼遇二档', prize_id: 2, probability: 30, hit_count: 3, reward_amount: 9 },
])
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: FAIL because `getOverviewStats` and `listPeriodStats` are still called and the removed UI is still rendered.

- [ ] **Step 3: Implement the minimal component change**

In `ZenxiangLiyuAdminView.vue`:

- Remove the period selector, overview metrics, and period table from the template.
- Remove `ZenxiangLiyuOverviewStats`, `ZenxiangLiyuPeriodStats`, `ZenxiangLiyuStatsPeriod`, `overview`, `statsPeriod`, `periodStats`, `periodOptions`, `overviewAverageReward`, `periodStatsRows`, `formatPeriodLabel`, and `changeStatsPeriod`.
- Derive the prize denominator from prize statistics:

```ts
const prizeStatsTotalHits = computed(() => prizeStats.value.reduce((sum, row) => sum + row.hit_count, 0))
const prizeStatsRows = computed(() => prizeStats.value.map((row) => {
  const actualRate = prizeStatsTotalHits.value ? row.hit_count / prizeStatsTotalHits.value : 0
  const configuredRate = Number(row.probability || 0) / 100
  return [row.prize_name, `${formatNumber(row.probability)}%`, formatPercent(actualRate), formatSignedPercent(actualRate - configuredRate), row.hit_count]
}))
```

- Restrict `loadStats()` to the retained calls:

```ts
const [usersResult, prizesResult, grantsResult] = await Promise.all([
  adminAPI.zenxiangLiyu.listUserStats({ page_size: 100, date: statsDate.value }),
  adminAPI.zenxiangLiyu.listPrizeStats(),
  adminAPI.zenxiangLiyu.listGrants({ page_size: 100 }),
])
userStats.value = usersResult.items
prizeStats.value = prizesResult
grants.value = grantsResult.items
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```bash
cd frontend && pnpm test:run src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts
```

Expected: all tests in the file PASS.

- [ ] **Step 5: Run frontend verification**

Run:

```bash
cd frontend && pnpm typecheck
cd frontend && pnpm build
```

Expected: both commands exit with status 0.

- [ ] **Step 6: Verify the page behavior**

Start the frontend development server, open the statistics tab, and confirm:

- the overview metrics, period controls, and period table are absent;
- user statistics and prize statistics remain visible;
- the statistics refresh completes without requests to `/stats/overview` or `/stats/periods`.

- [ ] **Step 7: Commit the implementation**

```bash
git add frontend/src/views/admin/ZenxiangLiyuAdminView.vue frontend/src/views/admin/__tests__/ZenxiangLiyuAdminView.spec.ts docs/superpowers/plans/2026-07-12-remove-zenxiang-period-stats.md
git commit -m "perf: remove slow zenxiang period stats"
```
