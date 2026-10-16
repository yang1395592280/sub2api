import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { getUsageSummary } from '@/api/admin/groups'

describe('admin group usage summary API', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: [] })
  })

  it('does not send browser timezone parameters', async () => {
    const summary = [
      { group_id: 1, today_cost: 1.25, yesterday_cost: 2.5, total_cost: 9.75 },
    ]
    get.mockResolvedValue({ data: summary })

    await expect(getUsageSummary()).resolves.toEqual(summary)

    expect(get).toHaveBeenCalledWith('/admin/groups/usage-summary')
  })

  it('requests snapshot metadata and an explicit refresh with cancellation', async () => {
    const controller = new AbortController()
    const snapshot = { items: [], updated_at: '2026-10-16T09:00:00Z', stale: false, refresh_failed: false }
    get.mockResolvedValue({ data: snapshot })
    await expect(getUsageSummary({ refresh: true, signal: controller.signal })).resolves.toEqual(snapshot)
    expect(get).toHaveBeenCalledWith('/admin/groups/usage-summary', {
      params: { include_metadata: true, refresh: true },
      signal: controller.signal,
    })
  })

  it('does not force a query during automatic snapshot reads', async () => {
    await getUsageSummary({ refresh: false })
    expect(get).toHaveBeenCalledWith('/admin/groups/usage-summary', {
      params: { include_metadata: true },
      signal: undefined,
    })
  })
})
