import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'

const {
  getSettings,
  listGroups,
  getOverview,
  listHealth,
  listEvents,
  updateSettings,
  updateGroup,
  resetScore,
  probeScore,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  listGroups: vi.fn(),
  getOverview: vi.fn(),
  listHealth: vi.fn(),
  listEvents: vi.fn(),
  updateSettings: vi.fn(),
  updateGroup: vi.fn(),
  resetScore: vi.fn(),
  probeScore: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    openaiAutoScheduler: {
      getSettings,
      listGroups,
      getOverview,
      listHealth,
      listEvents,
      updateSettings,
      updateGroup,
      resetScore,
      probeScore,
    },
  },
}))

import { useOpenAISchedulerDashboard } from '@/composables/useOpenAISchedulerDashboard'

const settings = {
  enabled: true,
  mode: 'balanced' as const,
  shadow_mode: true,
  top_k: 3,
  exploration_rate: 0.03,
  session_escape_min_gap_ms: 1000,
  session_escape_ratio: 0.25,
  health_ttl_seconds: 1800,
  real_sample_fresh_seconds: 300,
  probe_jitter_seconds: 6,
  probe_model: 'gpt-5.4',
  probe_interval_seconds: 60,
  slow_threshold_ms: 10000,
  severe_slow_threshold_ms: 20000,
  consecutive_slow_breaker_threshold: 3,
  consecutive_error_breaker_threshold: 2,
  cooldown_seconds: 120,
  half_open_success_threshold: 3,
  cost_weight: 0.2,
  recovery_step: 800,
}

const emptyOverview = {
  e2e_ttft_p50_ms: null,
  e2e_ttft_p90_ms: null,
  selection_p95_ms: null,
  probe_ratio: 0,
  groups: [],
  trend: [],
  slow_causes: [],
}

const emptyPage = { items: [], total: 0, page: 1, page_size: 20, pages: 1 }

describe('useOpenAISchedulerDashboard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({ ...settings })
    listGroups.mockResolvedValue([
      { id: 10, name: 'disabled', status: 'active', enabled: false },
      { id: 33, name: 'Codex', status: 'active', enabled: true },
      { id: 82, name: 'Control', status: 'active', enabled: true },
    ])
    getOverview.mockResolvedValue({ ...emptyOverview })
    listHealth.mockResolvedValue({ ...emptyPage })
    listEvents.mockResolvedValue({ ...emptyPage })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('selects the first enabled group and loads its overview', async () => {
    const scope = effectScope()
    const dashboard = scope.run(() => useOpenAISchedulerDashboard())!

    await dashboard.initialize()

    expect(dashboard.selectedGroupId.value).toBe(33)
    expect(getOverview).toHaveBeenCalledWith(
      { group_id: 33, window: '6h' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    scope.stop()
  })

  it('aborts the stale overview request when the selected group changes', async () => {
    const signals: AbortSignal[] = []
    let resolveFirstOverview: ((value: typeof emptyOverview) => void) | undefined
    getOverview.mockImplementationOnce((_params, options) => {
      signals.push(options.signal)
      return new Promise((resolve) => {
        resolveFirstOverview = resolve
      })
    })
    getOverview.mockImplementationOnce((_params, options) => {
      signals.push(options.signal)
      return Promise.resolve({ ...emptyOverview })
    })
    const scope = effectScope()
    const dashboard = scope.run(() => useOpenAISchedulerDashboard())!

    const initializing = dashboard.initialize()
    await Promise.resolve()
    await Promise.resolve()
    const selecting = dashboard.selectGroup(82)

    expect(signals[0]?.aborted).toBe(true)
    resolveFirstOverview?.({ ...emptyOverview })
    await Promise.all([initializing, selecting])
    expect(getOverview).toHaveBeenLastCalledWith(
      { group_id: 82, window: '6h' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    scope.stop()
  })

  it('loads drawer events with the selected physical account identity', async () => {
    const scope = effectScope()
    const dashboard = scope.run(() => useOpenAISchedulerDashboard())!
    const row = {
      account_id: 12512,
      account_name: 'main',
      group_id: 33,
      model_family: 'gpt-5.4',
      endpoint: 'responses',
      transport: 'http_sse',
      state: 'running',
      predicted_ttft_ms: 900,
      real_sample_count: 1,
      probe_sample_count: 0,
      error_rate: 0,
      rate_limited_rate: 0,
      server_error_rate: 0,
      load_inflight: 0,
      load_capacity: 1,
      waiting_count: 0,
      channel_price: null,
      decision: 'context_required',
      decision_reason: 'request_context_required',
      scheduler_mode: 'balanced',
      shadow_mode: true,
      sticky_escape_reason: null,
      snapshot_age_ms: 100,
      cooldown_until: null,
    }

    await dashboard.loadAccountEvents(row)

    expect(listEvents).toHaveBeenCalledWith(
      { account_id: 12512, group_id: 33, model: 'gpt-5.4', page: 1, page_size: 20 },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    scope.stop()
  })

  it('opens group health without issuing a discarded overview request', async () => {
    const scope = effectScope()
    const dashboard = scope.run(() => useOpenAISchedulerDashboard())!
    await dashboard.initialize()
    vi.clearAllMocks()

    await dashboard.showGroupHealth(82)

    expect(dashboard.activeTab.value).toBe('health')
    expect(dashboard.selectedGroupId.value).toBe(82)
    expect(getOverview).not.toHaveBeenCalled()
    expect(listHealth).toHaveBeenCalledTimes(1)
    expect(listHealth).toHaveBeenCalledWith(
      expect.objectContaining({ group_id: 82, page: 1 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    scope.stop()
  })

  it('does not reload when the active navigation choice is selected again', async () => {
    const scope = effectScope()
    const dashboard = scope.run(() => useOpenAISchedulerDashboard())!
    await dashboard.initialize()
    vi.clearAllMocks()

    await dashboard.selectGroup(33)
    await dashboard.selectTab('overview')
    await dashboard.selectWindow('6h')

    expect(getOverview).not.toHaveBeenCalled()
    scope.stop()
  })
})
