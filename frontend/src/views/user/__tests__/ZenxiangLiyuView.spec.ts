import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { getZenxiangLiyuStatus, listZenxiangLiyuRecords, playZenxiangLiyu } = vi.hoisted(() => ({
  getZenxiangLiyuStatus: vi.fn(),
  listZenxiangLiyuRecords: vi.fn(),
  playZenxiangLiyu: vi.fn(),
}))

vi.mock('@/api/zenxiangLiyu', () => ({
  getZenxiangLiyuStatus,
  listZenxiangLiyuRecords,
  playZenxiangLiyu,
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
  today_tickets_earned: 1,
  today_tickets_used: 0,
  today_tickets_available: 1,
  ticket_expires_at: '2026-07-11T16:00:00Z',
  prizes: [
    { id: 1, name: '1元', reward_amount: 1, probability: 70, enabled: true, sort_order: 1 },
    { id: 2, name: '3元', reward_amount: 3, probability: 30, enabled: true, sort_order: 2 },
  ],
})

const makePlayResult = () => ({
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
    listZenxiangLiyuRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
  })

  afterEach(() => {
    vi.useRealTimers()
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

    expect(wrapper.text()).toContain('zenxiangLiyu.todayTickets')
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

  it('shows a participation error when play fails', async () => {
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockRejectedValue(new Error('request failed'))

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('zenxiangLiyu.playFailed')
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
})
