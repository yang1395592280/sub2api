import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { getZenxiangLiyuStatus, listZenxiangLiyuRecords, playZenxiangLiyu, playZenxiangLiyuLuckyCoin } = vi.hoisted(() => ({
  getZenxiangLiyuStatus: vi.fn(),
  listZenxiangLiyuRecords: vi.fn(),
  playZenxiangLiyu: vi.fn(),
  playZenxiangLiyuLuckyCoin: vi.fn(),
}))

vi.mock('@/api/zenxiangLiyu', () => ({
  getZenxiangLiyuStatus,
  listZenxiangLiyuRecords,
  playZenxiangLiyu,
  playZenxiangLiyuLuckyCoin,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const messages: Record<string, string> = {
          'zenxiangLiyu.balanceUnit': '积分',
          'zenxiangLiyu.insufficientBalance': `积分需大于 ${params?.amount} 积分才可参与`,
          'zenxiangLiyu.latestBalance': `最新积分：${params?.amount} 积分`,
          'zenxiangLiyu.nextTicketMissing': `距离下一张抽奖券还差 ${params?.amount} 积分`,
          'zenxiangLiyu.ticketRetentionHint': `抽奖券有效期 ${params?.days} 天，最多累计 ${params?.limit} 张`,
          'zenxiangLiyu.finalRewardAmount': `最终奖励：${params?.amount}`,
          'zenxiangLiyu.finalRewardShort': `最终 ${params?.amount}`,
          'zenxiangLiyu.recordLuckyCoinWin': `翻倍 ${params?.amount}`,
          'zenxiangLiyu.recordLuckyCoinLose': `扣减 ${params?.amount}`,
        }
        return messages[key] ?? key
      },
    }),
  }
})

import ZenxiangLiyuView from '../ZenxiangLiyuView.vue'

const makePlayableStatus = () => ({
  visible: true,
  can_play: true,
  balance: 12,
  ticket_amount: 2,
  effective_ticket_amount: 2,
  minimum_balance: 10,
  daily_play_limit: 5,
  today_play_count: 0,
  remaining_plays: 5,
  today_usage_amount: 0,
  free_play_usage_threshold: 5,
  free_play_available: false,
  free_play_used: false,
  ticket_usage_threshold: 5,
  daily_ticket_limit: 3,
  ticket_capacity: 5,
  ticket_retention_days: 2,
  tickets_available: 1,
  today_tickets_earned: 1,
  today_tickets_used: 0,
  today_tickets_available: 1,
  next_ticket_usage_target: 10,
  next_ticket_usage_missing: 5,
  lucky_coin_enabled: true,
  lucky_coin_double_probability: 50,
  prizes: [
    { id: 1, name: '1元', reward_amount: 1, probability: 70, enabled: true, sort_order: 1 },
    { id: 2, name: '3元', reward_amount: 3, probability: 30, enabled: true, sort_order: 2 },
  ],
})

const makePlayResult = () => ({
  id: 9,
  applied: true,
  request_id: 'request-id',
  prize_id: 2,
  prize_name: '3元',
  reward_amount: 3,
  ticket_amount: 2,
  free_play: false,
  user_net_amount: 1,
  balance_before: 12,
  balance_after_ticket: 10,
  balance_after_reward: 13,
  played_at: '2026-07-10T00:00:00Z',
  lucky_coin_available: true,
  lucky_coin_played: false,
})

function mountView() {
  return mount(ZenxiangLiyuView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
      },
    },
  })
}

async function finishSpinAnimation() {
  vi.advanceTimersByTime(20)
  await flushPromises()
  vi.advanceTimersByTime(4200)
  await flushPromises()
}

describe('ZenxiangLiyuView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getZenxiangLiyuStatus.mockReset()
    listZenxiangLiyuRecords.mockReset()
    playZenxiangLiyu.mockReset()
    playZenxiangLiyuLuckyCoin.mockReset()
    listZenxiangLiyuRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads today records when entering the page', async () => {
    getZenxiangLiyuStatus.mockResolvedValue({
      ...makePlayableStatus(),
      visible: false,
      can_play: false,
      reason: 'disabled',
    })

    mountView()
    await flushPromises()

    expect(listZenxiangLiyuRecords).toHaveBeenCalledWith({ page: 1, page_size: 20 })
  })

  it('shows the five-ticket cap and two-day retention period', async () => {
    getZenxiangLiyuStatus.mockResolvedValue({
      ...makePlayableStatus(),
      tickets_available: 4,
      today_tickets_available: 4,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('4')
    expect(wrapper.text()).toContain('抽奖券有效期 2 天，最多累计 5 张')
  })

  it('shows insufficient balance reason and disables play', async () => {
    getZenxiangLiyuStatus.mockResolvedValue({
      ...makePlayableStatus(),
      can_play: false,
      reason: 'insufficient_balance',
      balance: 10,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('积分需大于 10 积分才可参与')
    expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeDefined()
  })

  it('plays once and displays reward from backend result', async () => {
    vi.useFakeTimers()
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockResolvedValue(makePlayResult())

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()
    await finishSpinAnimation()

    expect(playZenxiangLiyu).toHaveBeenCalledWith(expect.any(String))
    expect(wrapper.text()).toContain('3元')
    expect(wrapper.text()).toContain('13')
    vi.useRealTimers()
  })

  it('shows daily limit reason and disables play', async () => {
    getZenxiangLiyuStatus.mockResolvedValue({
      ...makePlayableStatus(),
      can_play: false,
      reason: 'daily_limit_reached',
      remaining_plays: 0,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('zenxiangLiyu.dailyLimitReached')
    expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeDefined()
  })

  it('enables draw when daily usage earns a ticket', async () => {
    getZenxiangLiyuStatus.mockResolvedValue({
      ...makePlayableStatus(),
      balance: 0,
      can_play: true,
      today_usage_amount: 5.01,
      effective_ticket_amount: 0,
      today_tickets_earned: 1,
      today_tickets_used: 0,
      today_tickets_available: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('zenxiangLiyu.availableTickets')
    expect(wrapper.text()).toContain('zenxiangLiyu.ticketPlayHint')
    expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeUndefined()
  })

  it('disables play while participation is pending', async () => {
    vi.useFakeTimers()
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    let resolvePlay: (result: ReturnType<typeof makePlayResult>) => void = () => undefined
    playZenxiangLiyu.mockImplementation(() => new Promise(resolve => {
      resolvePlay = resolve
    }))

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')

    expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeDefined()

    resolvePlay(makePlayResult())
    await flushPromises()
    await finishSpinAnimation()
    vi.useRealTimers()
  })

  it('shows backend message when play fails', async () => {
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockRejectedValue({ message: '今日暂无可用抽奖券' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('今日暂无可用抽奖券')
  })

  it('keeps the reward result visible when the post-play status refresh fails', async () => {
    vi.useFakeTimers()
    getZenxiangLiyuStatus
      .mockResolvedValueOnce(makePlayableStatus())
      .mockRejectedValueOnce(new Error('status refresh failed'))
    playZenxiangLiyu.mockResolvedValue(makePlayResult())

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()
    await finishSpinAnimation()

    expect(wrapper.text()).toContain('3元')
    expect(wrapper.text()).toContain('13')
    expect(wrapper.text()).not.toContain('zenxiangLiyu.loadFailed')
    expect(wrapper.text()).toContain('zenxiangLiyu.statusRefreshFailed')
    vi.useRealTimers()
  })

  it('shows only the bottom lucky coin action and uses it to play lucky coin', async () => {
    vi.useFakeTimers()
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockResolvedValue(makePlayResult())
    playZenxiangLiyuLuckyCoin.mockResolvedValue({
      record_id: 9,
      outcome: 'double',
      original_reward: 3,
      adjustment_amount: 3,
      balance_after: 16,
      double_probability: 50,
      played_at: '2026-07-10T00:00:00Z',
      lucky_coin_available: false,
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()
    await finishSpinAnimation()

    const luckyButtons = wrapper.findAll('[data-testid="zenxiang-lucky-coin"]')
    expect(luckyButtons).toHaveLength(1)

    await luckyButtons[0].trigger('click')
    vi.advanceTimersByTime(900)
    await flushPromises()

    expect(playZenxiangLiyuLuckyCoin).toHaveBeenCalledWith(9)
    expect(wrapper.text()).toContain('最终奖励：+6')
    expect(wrapper.text()).toContain('最终 +6')
    expect(wrapper.text()).toContain('翻倍 +3')
    expect(wrapper.text()).toContain('最新积分：16 积分')
    vi.useRealTimers()
  })

  it('shows backend message when lucky coin fails', async () => {
    vi.useFakeTimers()
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockResolvedValue(makePlayResult())
    playZenxiangLiyuLuckyCoin.mockRejectedValue({ message: '幸运金币已参与过' })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()
    await finishSpinAnimation()

    await wrapper.find('[data-testid="zenxiang-lucky-coin"]').trigger('click')
    vi.advanceTimersByTime(900)
    await flushPromises()

    expect(wrapper.text()).toContain('幸运金币已参与过')
    vi.useRealTimers()
  })
})
