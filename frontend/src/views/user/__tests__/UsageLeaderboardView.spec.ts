import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageLeaderboardView from '../UsageLeaderboardView.vue'

const { getOverview, getItems, showError } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  getItems: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/usageLeaderboard', () => ({
  usageLeaderboardAPI: {
    getOverview,
    getItems,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('UsageLeaderboardView', () => {
  beforeEach(() => {
    getOverview.mockReset()
    getItems.mockReset()
    showError.mockReset()

    getOverview.mockResolvedValue({
      date: '2026-06-06',
      metric: 'requests',
      participant_count: 3,
      top_items: [],
      current_user: undefined,
    })
    getItems.mockResolvedValue({
      items: [
        {
          rank: 1,
          user_id: 1,
          username: 'alpha',
          email: 'alpha@example.com',
          requests: 120,
          tokens: 3456,
          value: 120,
          metric: 'requests',
          is_current_user: false,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
  })

  it('loads leaderboard by date and requests metric', async () => {
    const wrapper = mount(UsageLeaderboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          LoadingSpinner: true,
          Pagination: true,
        },
      },
    })

    await flushPromises()

    expect(getOverview).toHaveBeenCalledWith({
      date: expect.any(String),
      metric: 'requests',
    })
    expect(getItems).toHaveBeenCalledWith({
      date: expect.any(String),
      metric: 'requests',
      page: 1,
      page_size: 20,
    })

    await (wrapper.vm as any).changeMetric('tokens')
    await flushPromises()

    expect(getOverview).toHaveBeenLastCalledWith({
      date: expect.any(String),
      metric: 'tokens',
    })
    expect(getItems).toHaveBeenLastCalledWith({
      date: expect.any(String),
      metric: 'tokens',
      page: 1,
      page_size: 20,
    })
  })
})
