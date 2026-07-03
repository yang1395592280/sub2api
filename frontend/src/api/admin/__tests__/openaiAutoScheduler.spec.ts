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
