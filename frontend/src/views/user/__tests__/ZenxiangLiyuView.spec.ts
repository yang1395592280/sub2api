import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const { getZenxiangLiyuStatus, playZenxiangLiyu } = vi.hoisted(() => ({
  getZenxiangLiyuStatus: vi.fn(),
  playZenxiangLiyu: vi.fn(),
}))

vi.mock('@/api/zenxiangLiyu', () => ({
  getZenxiangLiyuStatus,
  playZenxiangLiyu,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const messages: Record<string, string> = {
          'zenxiangLiyu.balanceUnit': '元',
          'zenxiangLiyu.insufficientBalance': `余额需大于 ${params?.amount} 元才可参与`,
          'zenxiangLiyu.latestBalance': `最新站内余额：${params?.amount} 元`,
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
  minimum_balance: 10,
  daily_play_limit: 5,
  today_play_count: 0,
  remaining_plays: 5,
  prizes: [
    { id: 1, name: '1元', reward_amount: 1, probability: 70, enabled: true, sort_order: 1 },
    { id: 2, name: '3元', reward_amount: 3, probability: 30, enabled: true, sort_order: 2 },
  ],
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

describe('ZenxiangLiyuView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    getZenxiangLiyuStatus.mockReset()
    playZenxiangLiyu.mockReset()
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

    expect(wrapper.text()).toContain('余额需大于 10 元才可参与')
    expect(wrapper.find('[data-testid="zenxiang-play"]').attributes('disabled')).toBeDefined()
  })

  it('plays once and displays reward from backend result', async () => {
    getZenxiangLiyuStatus.mockResolvedValue(makePlayableStatus())
    playZenxiangLiyu.mockResolvedValue({
      applied: true,
      request_id: 'request-id',
      prize_id: 2,
      prize_name: '3元',
      reward_amount: 3,
      ticket_amount: 2,
      user_net_amount: 1,
      balance_before: 12,
      balance_after_ticket: 10,
      balance_after_reward: 13,
      played_at: '2026-07-10T00:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-play"]').trigger('click')
    await flushPromises()

    expect(playZenxiangLiyu).toHaveBeenCalledWith(expect.any(String))
    expect(wrapper.text()).toContain('3元')
    expect(wrapper.text()).toContain('13')
  })
})
