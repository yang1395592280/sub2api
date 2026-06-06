import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
  },
}))

import { usageLeaderboardAPI } from '@/api/usageLeaderboard'

describe('usage leaderboard api', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockResolvedValue({ data: {} })
  })

  it('requests overview with requests/tokens metric contract', async () => {
    await usageLeaderboardAPI.getOverview({ metric: 'requests', date: '2026-06-01' })

    expect(get).toHaveBeenCalledWith('/usage-leaderboard/overview', {
      params: { metric: 'requests', date: '2026-06-01' },
    })
  })

  it('requests paginated leaderboard items', async () => {
    await usageLeaderboardAPI.getItems({ metric: 'tokens', page: 2, page_size: 5 })

    expect(get).toHaveBeenCalledWith('/usage-leaderboard/items', {
      params: { metric: 'tokens', page: 2, page_size: 5 },
    })
  })
})
