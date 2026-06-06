import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import { gameCenterAPI, luckyWheelAPI, sizeBetAPI } from '@/api/gameCenter'

describe('game center api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('requests game center overview with pagination/timezone params', async () => {
    await gameCenterAPI.getOverview({ page: 1, page_size: 5, timezone: 'Asia/Shanghai' })

    expect(get).toHaveBeenCalledWith('/game-center/overview', {
      params: { page: 1, page_size: 5, timezone: 'Asia/Shanghai' },
    })
  })

  it('requests points ledger without legacy exchange fields', async () => {
    await gameCenterAPI.getLedger({ start_date: '2026-06-01', end_date: '2026-06-06', page: 2 })

    expect(get).toHaveBeenCalledWith('/game-center/ledger', {
      params: { start_date: '2026-06-01', end_date: '2026-06-06', page: 2 },
    })
  })

  it('uses lucky wheel points-only endpoints', async () => {
    await luckyWheelAPI.getOverview()
    await luckyWheelAPI.spin()

    expect(get).toHaveBeenCalledWith('/game/lucky-wheel/overview')
    expect(post).toHaveBeenCalledWith('/game/lucky-wheel/spin')
  })

  it('uses size bet endpoints with scope/date pagination params only', async () => {
    await sizeBetAPI.getLeaderboard({ scope: 'daily' })
    await sizeBetAPI.getStatsOverview({ date: '2026-06-06' })

    expect(get).toHaveBeenCalledWith('/game/size-bet/leaderboard', {
      params: { scope: 'daily' },
    })
    expect(get).toHaveBeenCalledWith('/game/size-bet/stats/overview', {
      params: { date: '2026-06-06' },
    })
  })
})
