import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post },
}))

import {
  getZenxiangLiyuDailySummary,
  getZenxiangLiyuStatus,
  listZenxiangLiyuRecords,
  playZenxiangLiyu,
  playZenxiangLiyuLuckyCoin,
} from '@/api/zenxiangLiyu'

describe('zenxiang liyu user api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('loads status from the user endpoint', async () => {
    const status = { visible: true, can_play: false }
    get.mockResolvedValueOnce({ data: status })

    await expect(getZenxiangLiyuStatus()).resolves.toEqual(status)

    expect(get).toHaveBeenCalledWith('/zenxiang-liyu/status')
  })

  it('posts only the play request id', async () => {
    const result = { reward_amount: 3 }
    post.mockResolvedValueOnce({ data: result })

    await expect(playZenxiangLiyu('req-1')).resolves.toEqual(result)

    expect(post).toHaveBeenCalledWith('/zenxiang-liyu/play', { request_id: 'req-1' })
  })

  it('posts lucky coin by record id', async () => {
    const result = { record_id: 9, outcome: 'double' }
    post.mockResolvedValueOnce({ data: result })

    await expect(playZenxiangLiyuLuckyCoin(9)).resolves.toEqual(result)

    expect(post).toHaveBeenCalledWith('/zenxiang-liyu/records/9/lucky-coin', {})
  })

  it('loads records with pagination parameters', async () => {
    const records = { items: [], total: 0, page: 2, page_size: 10, pages: 0 }
    get.mockResolvedValueOnce({ data: records })

    await expect(listZenxiangLiyuRecords({ page: 2, page_size: 10 })).resolves.toEqual(records)

    expect(get).toHaveBeenCalledWith('/zenxiang-liyu/records', { params: { page: 2, page_size: 10 } })
  })

  it('loads the daily summary', async () => {
    const summary = { play_count: 2, ticket_amount: 4 }
    get.mockResolvedValueOnce({ data: summary })

    await expect(getZenxiangLiyuDailySummary()).resolves.toEqual(summary)

    expect(get).toHaveBeenCalledWith('/zenxiang-liyu/daily-summary')
  })
})
