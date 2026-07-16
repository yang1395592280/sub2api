import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
  },
}))

import openaiAutoSchedulerAPI, {
  type OpenAISchedulerHealthRow,
  type OpenAISchedulerOverview,
  type OpenAIAutoSchedulerSettings,
} from '@/api/admin/openaiAutoScheduler'

describe('openai auto scheduler admin api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    put.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('loads and updates settings through the scheduler settings endpoint', async () => {
    const settings: OpenAIAutoSchedulerSettings = {
      enabled: true,
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
    get.mockResolvedValueOnce({ data: settings })
    put.mockResolvedValueOnce({ data: settings })

    await expect(openaiAutoSchedulerAPI.getSettings()).resolves.toEqual(settings)
    await expect(openaiAutoSchedulerAPI.updateSettings(settings)).resolves.toEqual(settings)

    expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/settings')
    expect(put).toHaveBeenCalledWith('/admin/openai-auto-scheduler/settings', settings)
  })

  it('loads and updates OpenAI group participation state', async () => {
    const groups = [{ id: 20, name: 'openai-main', status: 'active', enabled: true }]
    get.mockResolvedValueOnce({ data: groups })
    put.mockResolvedValueOnce({ data: { ...groups[0], enabled: false } })

    await expect(openaiAutoSchedulerAPI.listGroups()).resolves.toEqual(groups)
    await expect(openaiAutoSchedulerAPI.updateGroup(20, { enabled: false })).resolves.toEqual({
      ...groups[0],
      enabled: false,
    })

    expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/groups')
    expect(put).toHaveBeenCalledWith('/admin/openai-auto-scheduler/groups/20', { enabled: false })
  })

  it('loads scores and events with filters', async () => {
    const scores = { items: [], total: 0, page: 1, page_size: 20, pages: 1 }
    const params = { group_id: 10, model: 'gpt-5', state: 'open' as const, search: 'main' }
    get.mockResolvedValueOnce({ data: scores })
    get.mockResolvedValueOnce({ data: scores })

    await expect(openaiAutoSchedulerAPI.listScores(params)).resolves.toEqual(scores)
    await expect(openaiAutoSchedulerAPI.listEvents({ group_id: 10, model: 'gpt-5' })).resolves.toEqual(scores)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/openai-auto-scheduler/scores', { params })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/openai-auto-scheduler/events', {
      params: { group_id: 10, model: 'gpt-5' },
    })
  })

  it('requests the scheduler overview with group, window, and cancellation', async () => {
    const overview: OpenAISchedulerOverview = {
      e2e_ttft_p50_ms: 2970,
      e2e_ttft_p90_ms: 7210,
      selection_p95_ms: 18,
      probe_ratio: 0.24,
      groups: [],
      trend: [],
      slow_causes: [],
    }
    const controller = new AbortController()
    get.mockResolvedValueOnce({ data: overview })

    await expect(
      openaiAutoSchedulerAPI.getOverview(
        { group_id: 33, window: '6h' },
        { signal: controller.signal }
      )
    ).resolves.toEqual(overview)

    expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/overview', {
      params: { group_id: 33, window: '6h' },
      signal: controller.signal,
    })
  })

  it('requests paginated scheduler health with exact backend filters', async () => {
    const health: OpenAISchedulerHealthRow = {
      account_id: 12512,
      account_name: 'openai-main',
      group_id: 33,
      model_family: 'gpt-5.4',
      endpoint: 'responses',
      transport: 'http_sse',
      state: 'running',
      predicted_ttft_ms: 10940,
      real_sample_count: 20,
      probe_sample_count: 4,
      error_rate: 0.01,
      rate_limited_rate: 0,
      server_error_rate: 0.01,
      load_inflight: 2,
      load_capacity: 10,
      waiting_count: 0,
      channel_price: 0.25,
      decision: 'context_required',
      decision_reason: 'request_context_required',
      scheduler_mode: 'balanced',
      shadow_mode: true,
      sticky_escape_reason: null,
      snapshot_age_ms: 500,
      cooldown_until: null,
    }
    const page = { items: [health], total: 1, page: 1, page_size: 20, pages: 1 }
    const params = {
      group_id: 33,
      state: 'running',
      model_family: 'gpt-5.4',
      endpoint: 'responses',
      transport: 'http_sse',
      sort: 'predicted_ttft_ms' as const,
      order: 'desc' as const,
      page: 1,
      page_size: 20,
    }
    get.mockResolvedValueOnce({ data: page })

    await expect(openaiAutoSchedulerAPI.listHealth(params)).resolves.toEqual(page)
    expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/health', { params })
  })

  it('requests scheduler rankings with policy partition filters', async () => {
    const result = { policy_context: {}, summary: {}, items: [], total: 0, page: 1, page_size: 20 }
    const params = { group_id: 33, window: '1h' as const, model_family: 'gpt-5.4', endpoint: 'responses', transport: 'http_sse', page: 1, page_size: 20 }
    const controller = new AbortController()
    get.mockResolvedValueOnce({ data: result })

    await expect(openaiAutoSchedulerAPI.listRankings(params, { signal: controller.signal })).resolves.toEqual(result)
    expect(get).toHaveBeenCalledWith('/admin/openai-auto-scheduler/rankings', { params, signal: controller.signal })
  })

  it('uses explicit account routes for reset and probe actions', async () => {
    const resetResult = { message: 'score reset' }
    const probeResult = {
      event_type: 'probe_success',
      success: true,
      message: 'ok',
      latency_ms: 840,
    }
    post.mockResolvedValueOnce({ data: resetResult })
    post.mockResolvedValueOnce({ data: probeResult })

    await expect(openaiAutoSchedulerAPI.resetScore(101, { group_id: 20, model: 'gpt-5' })).resolves.toEqual(resetResult)
    await expect(openaiAutoSchedulerAPI.probeScore(101, { group_id: 20, model: 'gpt-5' })).resolves.toEqual(probeResult)

    expect(post).toHaveBeenNthCalledWith(
      1,
      '/admin/openai-auto-scheduler/scores/accounts/101/reset',
      undefined,
      { params: { group_id: 20, model: 'gpt-5' } }
    )
    expect(post).toHaveBeenNthCalledWith(
      2,
      '/admin/openai-auto-scheduler/scores/accounts/101/probe',
      undefined,
      { params: { group_id: 20, model: 'gpt-5' } }
    )
  })
})
