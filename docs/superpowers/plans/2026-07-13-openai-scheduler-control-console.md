# OpenAI Scheduler Control Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single dense auto-scheduler page with the approved B-style operations console backed by real end-to-end scheduler metrics.

**Architecture:** Keep `OpenAIAutoSchedulerView.vue` as the route container and split overview, group navigation, health table, detail drawer, events, and settings into focused components. A dashboard composable owns API loading, abort handling, polling, and selected group/window state; components receive typed data and emit commands without reimplementing backend scheduling rules.

**Tech Stack:** Vue 3 Composition API, TypeScript, Vue Test Utils, Vitest, Chart.js/vue-chartjs, Tailwind and the existing component library.

## Global Constraints

- Execute only after the backend overview/health APIs in `2026-07-13-openai-unified-scheduler-engine.md` pass.
- Preserve the existing admin route and all current auto-scheduler operations.
- Use the approved B structure: overview, account health, scheduling events, grayscale/configuration.
- Desktop UI is dense and operations-focused; mobile reflows without overlapping text or controls.
- Use existing buttons, toggles, form controls, modal/drawer patterns, and Lucide/icon library already enabled by the project.
- Do not duplicate scheduler scoring or health-state logic in TypeScript.
- Keep cards to the four summary metrics and the account detail drawer; do not nest cards.
- Add Chinese and English locale keys together.

---

## File Structure

- Modify `frontend/src/api/admin/openaiAutoScheduler.ts`: overview/health/settings types and calls.
- Modify `frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts`: request contract tests.
- Create `frontend/src/composables/useOpenAISchedulerDashboard.ts`: page loading, polling, filters, selected group, and commands.
- Create `frontend/src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerOverview.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerGroupList.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerTTFTChart.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerHealthTable.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerAccountDrawer.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerEventsPanel.vue`.
- Create `frontend/src/components/admin/openai-scheduler/SchedulerSettingsPanel.vue`.
- Create matching focused component tests under `frontend/src/components/admin/openai-scheduler/__tests__/`.
- Refactor `frontend/src/views/admin/OpenAIAutoSchedulerView.vue` into the route container.
- Modify `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`.
- Modify Chinese and English admin locale files used by the existing page.

### Task 1: Extend the Typed Admin API

**Files:**
- Modify: `frontend/src/api/admin/openaiAutoScheduler.ts`
- Modify: `frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts`

**Interfaces:**
- Produces: `getOverview(params, options)`, `listHealth(params, options)`, and backward-compatible expanded settings types.
- Consumes: backend endpoints from unified scheduler Task 8.

- [ ] **Step 1: Write failing API request tests**

```ts
it('requests the scheduler overview with group and window', async () => {
  const overviewFixture: OpenAISchedulerOverview = {
    e2e_ttft_p50_ms: 2970,
    e2e_ttft_p90_ms: 7210,
    selection_p95_ms: 18,
    probe_ratio: 0.24,
    groups: [],
    trend: [],
    slow_causes: [],
  }
  get.mockResolvedValue({ data: overviewFixture })

  const result = await openaiAutoSchedulerAPI.getOverview({ group_id: 33, window: '6h' })

  expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/overview', {
    params: { group_id: 33, window: '6h' },
  })
  expect(result.e2e_ttft_p50_ms).toBe(2970)
})

it('requests paginated account health filters', async () => {
  const healthPageFixture: OpenAIAutoSchedulerListResponse<OpenAISchedulerHealthRow> = {
    items: [], total: 0, page: 1, page_size: 20, pages: 1,
  }
  get.mockResolvedValue({ data: healthPageFixture })
  await openaiAutoSchedulerAPI.listHealth({ group_id: 33, state: 'slow', page: 1, page_size: 20 })
  expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/health', expect.objectContaining({
    params: expect.objectContaining({ group_id: 33, state: 'slow' }),
  }))
})
```

- [ ] **Step 2: Run API tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts`

Expected: FAIL because overview/health methods and types do not exist.

- [ ] **Step 3: Add exact response types**

```ts
export interface OpenAISchedulerOverview {
  e2e_ttft_p50_ms: number | null
  e2e_ttft_p90_ms: number | null
  selection_p95_ms: number | null
  probe_ratio: number
  groups: OpenAISchedulerGroupSummary[]
  trend: OpenAISchedulerTrendPoint[]
  slow_causes: OpenAISchedulerSlowCause[]
}

export type OpenAISchedulerWindow = '1h' | '6h' | '24h' | '7d'

export interface OpenAISchedulerOverviewParams {
  group_id?: number
  window: OpenAISchedulerWindow
}

export interface OpenAISchedulerRequestOptions {
  signal?: AbortSignal
}

export interface OpenAISchedulerGroupSummary {
  id: number
  name: string
  enabled: boolean
  account_count: number
  e2e_ttft_p90_ms: number | null
  alert_level: 'ok' | 'warning' | 'critical' | 'disabled'
}

export interface OpenAISchedulerTrendPoint {
  bucket: string
  e2e_ttft_p50_ms: number | null
  e2e_ttft_p90_ms: number | null
}

export interface OpenAISchedulerSlowCause {
  reason: 'upstream_ttft' | 'queue' | 'retry'
  count: number
  ratio: number
}

export interface OpenAISchedulerHealthRow {
  account_id: number
  account_name: string
  group_id: number
  model_family: string
  endpoint: string
  transport: string
  state: string
  predicted_ttft_ms: number | null
  real_sample_count: number
  probe_sample_count: number
  error_rate: number
  load_inflight: number
  load_capacity: number
  waiting_count: number
  channel_price: number | null
  decision: string
  sticky_escape_reason: string | null
  snapshot_age_ms: number | null
}

export interface OpenAISchedulerHealthParams {
  group_id?: number
  state?: string
  model_family?: string
  endpoint?: string
  transport?: string
  page?: number
  page_size?: number
}
```

Add expanded settings fields using the backend JSON names from engine Task 5.

- [ ] **Step 4: Implement API methods with AbortSignal support**

```ts
async function getOverview(params: OpenAISchedulerOverviewParams, options: OpenAISchedulerRequestOptions = {}) {
  const config = options.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAISchedulerOverview>(`${basePath}/overview`, config)
  return data
}
```

Follow the existing pagination unwrap shape for `listHealth`.

- [ ] **Step 5: Run API tests**

Run: `pnpm --dir frontend exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts`

Expected: PASS.

- [ ] **Step 6: Commit API contracts**

```bash
git add frontend/src/api/admin/openaiAutoScheduler.ts frontend/src/api/admin/__tests__/openaiAutoScheduler.spec.ts
git commit -m "feat: add OpenAI scheduler console APIs"
```

### Task 2: Add the Dashboard Composable and Route Shell

**Files:**
- Create: `frontend/src/composables/useOpenAISchedulerDashboard.ts`
- Test: `frontend/src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts`
- Modify: `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`
- Modify: `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`

**Interfaces:**
- Produces: selected tab/group/window refs, overview/health/events/settings state, loading/error refs, filters, refresh, mutation commands, and cleanup.
- Consumes: Task 1 API client.

- [ ] **Step 1: Write failing composable tests**

```ts
it('selects the first enabled group and loads its overview', async () => {
  listGroups.mockResolvedValue([
    { id: 33, name: 'Codex', status: 'active', enabled: true },
    { id: 94, name: 'Control', status: 'active', enabled: false },
  ])
  const dashboard = useOpenAISchedulerDashboard()
  await dashboard.initialize()
  expect(dashboard.selectedGroupId.value).toBe(33)
  expect(getOverview).toHaveBeenCalledWith(expect.objectContaining({ group_id: 33, window: '6h' }), expect.anything())
})

it('aborts stale overview requests when group changes', async () => {
  const dashboard = useOpenAISchedulerDashboard()
  await dashboard.initialize()
  dashboard.selectGroup(82)
  expect(observedSignals[0].aborted).toBe(true)
})
```

- [ ] **Step 2: Run composable tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts`

Expected: FAIL because the composable does not exist.

- [ ] **Step 3: Implement one owner for loading and polling**

```ts
export function useOpenAISchedulerDashboard() {
  const activeTab = ref<'overview' | 'health' | 'events' | 'settings'>('overview')
  const selectedGroupId = ref<number | null>(null)
  const window = ref<'1h' | '6h' | '24h' | '7d'>('6h')
  const overview = ref<OpenAISchedulerOverview | null>(null)
  const healthPage = ref<OpenAIAutoSchedulerListResponse<OpenAISchedulerHealthRow> | null>(null)
  let overviewController: AbortController | null = null

  async function loadOverview() {
    overviewController?.abort()
    overviewController = new AbortController()
    overview.value = await adminAPI.openaiAutoScheduler.getOverview(
      { group_id: selectedGroupId.value ?? undefined, window: window.value },
      { signal: overviewController.signal },
    )
  }

  return { activeTab, selectedGroupId, window, overview, healthPage, loadOverview }
}
```

Complete the returned contract for initialization, health/events paging, mutations, polling pause while hidden, and `onScopeDispose` abort cleanup.

- [ ] **Step 4: Reduce the route view to composition and tabs**

The route owns the page title, global switch, mode badge, tab buttons, component selection, and settings modal/drawer. It does not format score fields or make direct HTTP calls.

- [ ] **Step 5: Run composable and route tests**

Run: `pnpm --dir frontend exec vitest run src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`

Expected: PASS; existing global/group toggle expectations remain covered.

- [ ] **Step 6: Commit the page shell**

```bash
git add frontend/src/composables/useOpenAISchedulerDashboard.ts \
  frontend/src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts \
  frontend/src/views/admin/OpenAIAutoSchedulerView.vue \
  frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts
git commit -m "refactor: split OpenAI scheduler page state"
```

### Task 3: Build the Overview and Group Navigation

**Files:**
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerOverview.vue`
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerGroupList.vue`
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerTTFTChart.vue`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerOverview.spec.ts`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerGroupList.spec.ts`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerTTFTChart.spec.ts`

**Interfaces:**
- Overview props: `overview`, `loading`, `selectedGroupId`.
- Overview emits: `select-group`, `show-health-filter`.
- Chart props: typed trend points; no API calls.

- [ ] **Step 1: Write failing component tests**

```ts
it('shows the four approved scheduler metrics', () => {
  const wrapper = mount(SchedulerOverview, { props: { overview: overviewFixture, loading: false, selectedGroupId: 33 } })
  expect(wrapper.text()).toContain('2.97s')
  expect(wrapper.text()).toContain('7.21s')
  expect(wrapper.text()).toContain('18ms')
  expect(wrapper.text()).toContain('24%')
})

it('emits the selected group', async () => {
  const wrapper = mount(SchedulerGroupList, { props: { groups: groupFixtures, modelValue: 33 } })
  await wrapper.get('[data-testid="scheduler-group-82"]').trigger('click')
  expect(wrapper.emitted('update:modelValue')).toEqual([[82]])
})
```

- [ ] **Step 2: Run component tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerOverview.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerGroupList.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerTTFTChart.spec.ts`

Expected: FAIL because components do not exist.

- [ ] **Step 3: Implement the approved overview hierarchy**

Render exactly four summary cards, group list, one highest-priority alert, P50/P90 line chart, slow-cause list, and a compact actual-ranking preview. Use `vue-chartjs` with a responsive, non-animated initial render and theme-aware existing chart helpers.

- [ ] **Step 4: Add deterministic empty/error/loading states**

No data displays `—` and a concise empty state; API error uses the existing alert component with retry emit; skeleton dimensions match loaded metrics to prevent layout shift.

- [ ] **Step 5: Run overview tests**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerOverview.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerGroupList.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerTTFTChart.spec.ts`

Expected: PASS, including empty/error/loading cases.

- [ ] **Step 6: Commit overview components**

```bash
git add frontend/src/components/admin/openai-scheduler/SchedulerOverview.vue \
  frontend/src/components/admin/openai-scheduler/SchedulerGroupList.vue \
  frontend/src/components/admin/openai-scheduler/SchedulerTTFTChart.vue \
  frontend/src/components/admin/openai-scheduler/__tests__
git commit -m "feat: add OpenAI scheduler overview"
```

### Task 4: Build Account Health Table and Detail Drawer

**Files:**
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerHealthTable.vue`
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerAccountDrawer.vue`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerHealthTable.spec.ts`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerAccountDrawer.spec.ts`

**Interfaces:**
- Health table props: paginated rows, filters, loading.
- Emits: filter/page changes, row selection, manual probe, reset.
- Drawer props: selected health row and recent events.

- [ ] **Step 1: Write failing table/drawer tests**

```ts
it('renders the actual decision fields without exposing raw state names', () => {
  const wrapper = mount(SchedulerHealthTable, { props: { rows: healthRows, loading: false, total: 3, page: 1, pageSize: 20 } })
  expect(wrapper.text()).toContain('#12512')
  expect(wrapper.text()).toContain('10.94s')
  expect(wrapper.text()).toContain('降权，允许会话逃逸')
  expect(wrapper.text()).not.toContain('half_open')
})

it('shows real and probe samples separately in the drawer', () => {
  const wrapper = mount(SchedulerAccountDrawer, { props: { open: true, account: healthRows[0], events: [] } })
  expect(wrapper.text()).toContain('真实请求样本')
  expect(wrapper.text()).toContain('探测样本')
})
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerHealthTable.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerAccountDrawer.spec.ts`

Expected: FAIL because components do not exist.

- [ ] **Step 3: Implement dense table and filters**

Columns: account/channel, health, model/endpoint/transport, predicted TTFT, real/probe samples, load/waiting, channel price, actual decision, actions. Long names and errors truncate in the table and remain complete in the drawer.

- [ ] **Step 4: Implement the detail drawer**

Show real-vs-probe timing, E2E/upstream TTFT, errors/429/5xx, queue/retry, circuit/cooldown, recent decisions, sticky escape reason, manual probe, and reset. Destructive reset uses the existing confirmation modal.

- [ ] **Step 5: Run table/drawer tests**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerHealthTable.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerAccountDrawer.spec.ts`

Expected: PASS; operation buttons emit one command and do not call APIs directly.

- [ ] **Step 6: Commit health components**

```bash
git add frontend/src/components/admin/openai-scheduler/SchedulerHealthTable.vue \
  frontend/src/components/admin/openai-scheduler/SchedulerAccountDrawer.vue \
  frontend/src/components/admin/openai-scheduler/__tests__/SchedulerHealthTable.spec.ts \
  frontend/src/components/admin/openai-scheduler/__tests__/SchedulerAccountDrawer.spec.ts
git commit -m "feat: add OpenAI scheduler health inspector"
```

### Task 5: Build Events and Grayscale Settings Views

**Files:**
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerEventsPanel.vue`
- Create: `frontend/src/components/admin/openai-scheduler/SchedulerSettingsPanel.vue`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerEventsPanel.spec.ts`
- Test: `frontend/src/components/admin/openai-scheduler/__tests__/SchedulerSettingsPanel.spec.ts`
- Modify: `frontend/src/views/admin/OpenAIAutoSchedulerView.vue`

**Interfaces:**
- Events consumes existing event pagination plus new decision metadata.
- Settings edits the expanded backend settings contract and emits one validated payload.

- [ ] **Step 1: Write failing events/settings tests**

```ts
it('shows the full decision chain for a request event', () => {
  const wrapper = mount(SchedulerEventsPanel, { props: { events: [decisionEventFixture], loading: false } })
  expect(wrapper.text()).toContain('候选 6')
  expect(wrapper.text()).toContain('过滤 2')
  expect(wrapper.text()).toContain('最终账号 #12487')
})

it('emits balanced defaults as numeric values', async () => {
  const wrapper = mount(SchedulerSettingsPanel, { props: { modelValue: settingsFixture, saving: false } })
  await wrapper.get('form').trigger('submit')
  expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
    mode: 'balanced', top_k: 3, exploration_rate: 0.03,
    session_escape_min_gap_ms: 1000, session_escape_ratio: 0.25,
  })
})
```

- [ ] **Step 2: Run tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerEventsPanel.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerSettingsPanel.spec.ts`

Expected: FAIL because components do not exist.

- [ ] **Step 3: Implement decision-oriented events**

Default event rows show time, request/account, hard-filter summary, sticky decision, selected account, retry count, final outcome, and duration. Raw message remains available in an expandable detail area.

- [ ] **Step 4: Implement grouped settings and confirmations**

Sections: mode/shadow, balanced selection, slow/circuit thresholds, probe policy, recorder/health status. Global disable, shadow-to-live switch, and score reset require the existing confirmation dialog with affected group count.

- [ ] **Step 5: Run events/settings tests**

Run: `pnpm --dir frontend exec vitest run src/components/admin/openai-scheduler/__tests__/SchedulerEventsPanel.spec.ts src/components/admin/openai-scheduler/__tests__/SchedulerSettingsPanel.spec.ts`

Expected: PASS with validation errors shown next to the field and no request emitted for invalid values.

- [ ] **Step 6: Commit event/config views**

```bash
git add frontend/src/components/admin/openai-scheduler/SchedulerEventsPanel.vue \
  frontend/src/components/admin/openai-scheduler/SchedulerSettingsPanel.vue \
  frontend/src/components/admin/openai-scheduler/__tests__/SchedulerEventsPanel.spec.ts \
  frontend/src/components/admin/openai-scheduler/__tests__/SchedulerSettingsPanel.spec.ts \
  frontend/src/views/admin/OpenAIAutoSchedulerView.vue
git commit -m "feat: add OpenAI scheduler events and rollout controls"
```

### Task 6: Add Locales and Complete Responsive Verification

**Files:**
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`
- Modify: `frontend/src/i18n/__tests__/openaiAutoSchedulerLocales.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts`

**Interfaces:**
- Produces complete Chinese/English copy and responsive acceptance coverage.

- [ ] **Step 1: Write failing locale and layout contract tests**

```ts
it('defines every scheduler console locale key in zh and en', () => {
  function resolveLocale(root: Record<string, unknown>, path: string): unknown {
    return path.split('.').reduce<unknown>((value, segment) => {
      if (!value || typeof value !== 'object') return undefined
      return (value as Record<string, unknown>)[segment]
    }, root)
  }
  const keys = [
    'overview.e2eP50', 'overview.e2eP90', 'overview.selectionP95', 'overview.probeRatio',
    'health.realSamples', 'health.probeSamples', 'health.decision',
    'events.candidateCount', 'settings.shadowMode', 'settings.sessionEscapeGap',
  ]
  for (const key of keys) {
    expect(resolveLocale(zh.admin.openaiAutoScheduler as Record<string, unknown>, key)).toBeTruthy()
    expect(resolveLocale(en.admin.openaiAutoScheduler as Record<string, unknown>, key)).toBeTruthy()
  }
})
```

- [ ] **Step 2: Run locale tests and confirm RED**

Run: `pnpm --dir frontend exec vitest run src/i18n/__tests__/openaiAutoSchedulerLocales.spec.ts`

Expected: FAIL for missing console keys.

- [ ] **Step 3: Add concise operational copy**

Use user-facing terms such as “运行中 / 观察中 / 已熔断 / 恢复验证”, “真实请求样本”, “探测样本”, and “允许会话逃逸”; do not expose `running`, `half_open`, or raw score component names in primary UI text.

- [ ] **Step 4: Run all page/component tests**

Run: `pnpm --dir frontend exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts src/components/admin/openai-scheduler/__tests__ src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts src/i18n/__tests__/openaiAutoSchedulerLocales.spec.ts`

Expected: PASS.

- [ ] **Step 5: Run lint, typecheck, and production build**

Run:

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Expected: all commands exit 0.

- [ ] **Step 6: Start the app and verify desktop/mobile screenshots**

Run: `pnpm --dir frontend dev --host 127.0.0.1`

Verify at approximately 1440x900, 1024x768, 390x844, and 320x720:

- four metrics do not overflow;
- group list moves above content on narrow screens;
- health table uses controlled horizontal scrolling;
- drawer becomes full-width on mobile;
- settings labels and buttons remain readable;
- charts render nonblank and text does not overlap.

- [ ] **Step 7: Commit locales and responsive fixes**

```bash
git add frontend/src/i18n frontend/src/views/admin frontend/src/components/admin/openai-scheduler frontend/src/composables frontend/src/api/admin
git commit -m "fix: polish OpenAI scheduler console responsiveness"
```

### Task 7: Cross-Phase Acceptance

**Files:**
- Test only.

**Interfaces:**
- Verifies the approved B console against live backend contracts.

- [ ] **Step 1: Run frontend verification suite**

Run:

```bash
pnpm --dir frontend exec vitest run src/api/admin/__tests__/openaiAutoScheduler.spec.ts \
  src/composables/__tests__/useOpenAISchedulerDashboard.spec.ts \
  src/components/admin/openai-scheduler/__tests__ \
  src/views/admin/__tests__/OpenAIAutoSchedulerView.spec.ts \
  src/i18n/__tests__/openaiAutoSchedulerLocales.spec.ts
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

Expected: all commands exit 0.

- [ ] **Step 2: Run backend API contract tests**

Run: `cd backend && GOCACHE=/tmp/sub2api-go-cache go test ./internal/handler/admin ./internal/server -run 'OpenAIScheduler|APIContract' -count=1`

Expected: PASS.

- [ ] **Step 3: Verify diff and commit history**

Run:

```bash
git diff --check custom-main...HEAD
git status --short
git log --oneline custom-main..HEAD
```

Expected: clean worktree; focused commits for API, shell, overview, health, events/settings, and responsive polish.
