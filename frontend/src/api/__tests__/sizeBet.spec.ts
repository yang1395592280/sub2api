import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrent, getHistory, getRules, placeBet } from '../sizeBet'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('../client', () => ({
  apiClient: {
    get,
    post,
  },
}))

describe('sizeBet API', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('gets current round view', async () => {
    const payload = {
      enabled: true,
      phase: 'betting',
      server_time: '2026-04-23T12:00:10Z',
      round: null,
      my_bet: null,
      previous_round: null,
    }
    get.mockResolvedValue({ data: payload })

    await expect(getCurrent()).resolves.toEqual(payload)
    expect(get).toHaveBeenCalledWith('/game/size-bet/current')
  })

  it('gets rules view', async () => {
    const payload = {
      enabled: true,
      round_duration_seconds: 60,
      bet_close_offset_seconds: 50,
      allowed_stakes: [2, 5, 10, 20],
      probabilities: { small: 45, mid: 10, big: 45 },
      odds: { small: 2, mid: 10, big: 2 },
      rules_markdown: '## 规则',
    }
    get.mockResolvedValue({ data: payload })

    await expect(getRules()).resolves.toEqual(payload)
    expect(get).toHaveBeenCalledWith('/game/size-bet/rules')
  })

  it('gets history with the expected paging params', async () => {
    const payload = {
      items: [],
      total: 0,
      page: 2,
      page_size: 5,
      pages: 0,
    }
    get.mockResolvedValue({ data: payload })

    await expect(getHistory(2, 5)).resolves.toEqual(payload)
    expect(get).toHaveBeenCalledWith('/game/size-bet/history', {
      params: { page: 2, page_size: 5 },
    })
  })

  it('posts a bet request', async () => {
    const payload = {
      id: 7,
      round_id: 1001,
      direction: 'big',
      stake_amount: 10,
      payout_amount: 0,
      net_result_amount: 0,
      status: 'placed',
      placed_at: '2026-04-23T12:00:10Z',
    }
    const request = {
      round_id: 1001,
      direction: 'big',
      stake_amount: 10,
      idempotency_key: 'idempotency-key',
    }
    post.mockResolvedValue({ data: payload })

    await expect(placeBet(request)).resolves.toEqual(payload)
    expect(post).toHaveBeenCalledWith('/game/size-bet/bet', request)
  })
})
