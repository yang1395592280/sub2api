import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, delete: remove } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, delete: remove },
}))

import adminZenxiangLiyuAPI from '@/api/admin/zenxiangLiyu'

describe('zenxiang liyu admin api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    remove.mockReset()
  })

  it('loads and updates settings', async () => {
    const payload = { global_enabled: true, ticket_amount: 2, minimum_balance: 10, daily_play_limit: 5 }
    get.mockResolvedValueOnce({ data: payload })
    put.mockResolvedValueOnce({ data: payload })

    await expect(adminZenxiangLiyuAPI.getSettings()).resolves.toEqual(payload)
    await expect(adminZenxiangLiyuAPI.updateSettings(payload)).resolves.toEqual(payload)

    expect(get).toHaveBeenCalledWith('/admin/zenxiang-liyu/settings')
    expect(put).toHaveBeenCalledWith('/admin/zenxiang-liyu/settings', payload)
  })

  it('manages prizes including complete replacement', async () => {
    const prize = { id: 1, name: 'Reward', reward_amount: 3, probability: 100, enabled: true, sort_order: 1 }
    const prizes = [prize]
    get.mockResolvedValueOnce({ data: prizes })
    post.mockResolvedValueOnce({ data: prize })
    put.mockResolvedValueOnce({ data: prizes })
    put.mockResolvedValueOnce({ data: prize })
    remove.mockResolvedValueOnce({ data: { id: 1 } })

    await expect(adminZenxiangLiyuAPI.listPrizes()).resolves.toEqual(prizes)
    await expect(adminZenxiangLiyuAPI.createPrize(prize)).resolves.toEqual(prize)
    await expect(adminZenxiangLiyuAPI.replacePrizes(prizes)).resolves.toEqual(prizes)
    await expect(adminZenxiangLiyuAPI.updatePrize(1, prize)).resolves.toEqual(prize)
    await expect(adminZenxiangLiyuAPI.deletePrize(1)).resolves.toEqual({ id: 1 })

    expect(get).toHaveBeenCalledWith('/admin/zenxiang-liyu/prizes')
    expect(post).toHaveBeenCalledWith('/admin/zenxiang-liyu/prizes', prize)
    expect(put).toHaveBeenNthCalledWith(1, '/admin/zenxiang-liyu/prizes', { prizes })
    expect(put).toHaveBeenNthCalledWith(2, '/admin/zenxiang-liyu/prizes/1', prize)
    expect(remove).toHaveBeenCalledWith('/admin/zenxiang-liyu/prizes/1')
  })

  it('manages grants and loads all statistics routes', async () => {
    const grant = { user_id: 7, enabled: true, notes: 'allowed' }
    const gift = { request_id: 'gift-1', user_id: 7, ticket_count: 2, notes: 'compensation' }
    const grants = { items: [grant], total: 1, page: 1, page_size: 20, pages: 1 }
    const overview = { total_plays: 1 }
    const periods = [{ period_label: '2026-07-11', play_count: 1 }]
    const users = { items: [], total: 0, page: 2, page_size: 10, pages: 0 }
    const prizeStats = []
    const resetResult = { user_id: 7, previous_play_count: 3, remaining_plays: 5 }
    get.mockResolvedValueOnce({ data: grants })
    post.mockResolvedValueOnce({ data: grant })
    remove.mockResolvedValueOnce({ data: { user_id: 7 } })
    post.mockResolvedValueOnce({ data: resetResult })
    post.mockResolvedValueOnce({ data: { id: 11, ...gift } })
    get.mockResolvedValueOnce({ data: overview })
    get.mockResolvedValueOnce({ data: periods })
    get.mockResolvedValueOnce({ data: users })
    get.mockResolvedValueOnce({ data: prizeStats })

    await expect(adminZenxiangLiyuAPI.listGrants()).resolves.toEqual(grants)
    await expect(adminZenxiangLiyuAPI.createGrant(grant)).resolves.toEqual(grant)
    await expect(adminZenxiangLiyuAPI.deleteGrant(7)).resolves.toEqual({ user_id: 7 })
    await expect(adminZenxiangLiyuAPI.resetGrantDailyPlays(7)).resolves.toEqual(resetResult)
    await expect(adminZenxiangLiyuAPI.giftTickets(gift)).resolves.toEqual({ id: 11, ...gift })
    await expect(adminZenxiangLiyuAPI.getOverviewStats()).resolves.toEqual(overview)
    await expect(adminZenxiangLiyuAPI.listPeriodStats('week')).resolves.toEqual(periods)
    await expect(adminZenxiangLiyuAPI.listUserStats({ page: 2, page_size: 10 })).resolves.toEqual(users)
    await expect(adminZenxiangLiyuAPI.listPrizeStats()).resolves.toEqual(prizeStats)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/zenxiang-liyu/grants', { params: {} })
    expect(post).toHaveBeenCalledWith('/admin/zenxiang-liyu/grants', grant)
    expect(remove).toHaveBeenCalledWith('/admin/zenxiang-liyu/grants/7')
    expect(post).toHaveBeenCalledWith('/admin/zenxiang-liyu/grants/7/reset-daily')
    expect(post).toHaveBeenCalledWith('/admin/zenxiang-liyu/tickets/gift', gift)
    expect(get).toHaveBeenNthCalledWith(2, '/admin/zenxiang-liyu/stats/overview')
    expect(get).toHaveBeenNthCalledWith(3, '/admin/zenxiang-liyu/stats/periods', { params: { period: 'week' } })
    expect(get).toHaveBeenNthCalledWith(4, '/admin/zenxiang-liyu/stats/users', { params: { page: 2, page_size: 10 } })
    expect(get).toHaveBeenNthCalledWith(5, '/admin/zenxiang-liyu/stats/prizes')
  })

  it('loads a selected users daily draw records', async () => {
    const records = {
      items: [{ id: 9, prize_name: 'Reward', reward_amount: 1, played_at: '2026-07-13T10:00:00Z' }],
      total: 1,
      page: 1,
      page_size: 100,
      pages: 1,
    }
    get.mockResolvedValueOnce({ data: records })

    await expect(adminZenxiangLiyuAPI.listUserRecords(42, {
      date: '2026-07-13',
      page: 1,
      page_size: 100,
    })).resolves.toEqual(records)

    expect(get).toHaveBeenCalledWith('/admin/zenxiang-liyu/stats/users/42/records', {
      params: { date: '2026-07-13', page: 1, page_size: 100 },
    })
  })

  it('uses the simulation, recommendation, and apply paths', async () => {
    const simulation = { user_count: 100, plays_per_user: 2, initial_balance: 10, ticket_amount: 1, minimum_balance: 1, daily_play_limit: 3, prizes: [] }
    const recommendation = { target_profit_rate: 0.2, ticket_amount: 1, prizes: [] }
    const prizes = [{ id: 1, name: 'Reward', reward_amount: 1, probability: 100, enabled: true, sort_order: 1 }]
    post.mockResolvedValueOnce({ data: { total_plays: 200 } })
    post.mockResolvedValueOnce({ data: { plans: [] } })
    post.mockResolvedValueOnce({ data: prizes })

    await expect(adminZenxiangLiyuAPI.simulate(simulation)).resolves.toEqual({ total_plays: 200 })
    await expect(adminZenxiangLiyuAPI.recommend(recommendation)).resolves.toEqual({ plans: [] })
    await expect(adminZenxiangLiyuAPI.applySimulation(prizes)).resolves.toEqual(prizes)

    expect(post).toHaveBeenNthCalledWith(1, '/admin/zenxiang-liyu/simulate', simulation)
    expect(post).toHaveBeenNthCalledWith(2, '/admin/zenxiang-liyu/simulate/recommend', recommendation)
    expect(post).toHaveBeenNthCalledWith(3, '/admin/zenxiang-liyu/simulate/apply', { prizes })
  })
})
