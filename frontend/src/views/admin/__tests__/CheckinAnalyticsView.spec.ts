import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn()
  })
  vi.stubGlobal('matchMedia', vi.fn(() => ({
    matches: true,
    media: '(min-width: 768px)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn()
  })))
})

import CheckinAnalyticsView from '../CheckinAnalyticsView.vue'

const { getAnalytics, list } = vi.hoisted(() => ({
  getAnalytics: vi.fn(),
  list: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    checkins: {
      getAnalytics,
      list
    }
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    template: '<div data-test="line-chart"></div>'
  },
  Bar: {
    name: 'Bar',
    template: '<div data-test="bar-chart"></div>'
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('CheckinAnalyticsView', () => {
  beforeEach(() => {
    getAnalytics.mockReset()
    list.mockReset()

    getAnalytics.mockResolvedValue({
      overview: {
        total_checkins: 12,
        total_reward_amount: 0.42,
        today_checkins: 2,
        avg_reward_amount: 0.035
      },
      trend: [{ date: '2026-04-21', checkin_count: 5, reward_amount: 0.12 }],
      reward_distribution: [],
      top_users: []
    })
    list.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
  })

  it('loads analytics and filters details by selected trend day', async () => {
    const wrapper = mount(CheckinAnalyticsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' }
        }
      }
    })

    await flushPromises()

    expect(getAnalytics).toHaveBeenCalledTimes(1)
    expect(list).toHaveBeenCalledTimes(1)

    wrapper.findComponent({ name: 'CheckinTrendChart' }).vm.$emit('select-date', '2026-04-21')
    await flushPromises()

    expect(list).toHaveBeenLastCalledWith(
      1,
      expect.any(Number),
      expect.objectContaining({ date: '2026-04-21' }),
      expect.anything()
    )
  })
})
