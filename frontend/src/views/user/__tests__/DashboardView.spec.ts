import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { computed } from 'vue'

import DashboardView from '../DashboardView.vue'

const {
  getDashboardStats,
  getDashboardTrend,
  getDashboardModels,
  getByDateRange,
  getMyPlatformQuotas,
  gameCenterOverview,
  usageLeaderboardOverview,
  refreshUser,
} = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
  getMyPlatformQuotas: vi.fn(),
  gameCenterOverview: vi.fn(),
  usageLeaderboardOverview: vi.fn(),
  refreshUser: vi.fn(),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats,
    getDashboardTrend,
    getDashboardModels,
    getByDateRange,
  },
}))

vi.mock('@/api/user', () => ({
  getMyPlatformQuotas,
}))

vi.mock('@/api/gameCenter', () => ({
  gameCenterAPI: {
    getOverview: gameCenterOverview,
  },
}))

vi.mock('@/api/usageLeaderboard', () => ({
  usageLeaderboardAPI: {
    getOverview: usageLeaderboardOverview,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: computed(() => ({
      id: 1,
      username: 'demo',
      email: 'demo@example.com',
      role: 'user',
      balance: 0,
    })),
    isSimpleMode: false,
    refreshUser,
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

describe('DashboardView', () => {
  beforeEach(() => {
    getDashboardStats.mockReset()
    getDashboardTrend.mockReset()
    getDashboardModels.mockReset()
    getByDateRange.mockReset()
    getMyPlatformQuotas.mockReset()
    gameCenterOverview.mockReset()
    usageLeaderboardOverview.mockReset()
    refreshUser.mockReset()

    refreshUser.mockResolvedValue(undefined)
    getDashboardStats.mockResolvedValue({
      today_requests: 0,
      total_requests: 0,
      today_input_tokens: 0,
      today_output_tokens: 0,
      today_tokens: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_tokens: 0,
      avg_duration_ms: 0,
      model_stats: [],
    })
    getDashboardTrend.mockResolvedValue({ trend: [] })
    getDashboardModels.mockResolvedValue({ models: [] })
    getByDateRange.mockResolvedValue({ items: [] })
    getMyPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
    gameCenterOverview.mockResolvedValue({
      enabled: true,
      points: 288,
      catalogs: [
        {
          game_key: 'lucky-wheel',
          name: 'Lucky Wheel',
          subtitle: 'Spin for points',
          cover_image: '',
          description: 'Daily spin game',
          enabled: true,
          sort_order: 1,
          default_open_mode: 'embedded',
          supports_embed: true,
          supports_standalone: true,
        },
      ],
      recent_ledger: [],
      checkin: {
        enabled: true,
        min_reward_points: 3,
        max_reward_points: 9,
        bonus_enabled: true,
        bonus_available: false,
        bonus_success_rate: 0.2,
        stats: {
          total_reward_points: 108,
          total_checkins: 18,
          checkin_count: 18,
          checked_in_today: true,
          records: [],
        },
        today_record: {
          checkin_date: '2026-06-06',
          reward_points: 8,
          base_reward_points: 6,
          bonus_status: 'won',
          bonus_delta_points: 2,
        },
      },
    })
    usageLeaderboardOverview.mockResolvedValue({
      date: '2026-06-06',
      metric: 'tokens',
      participant_count: 32,
      top_items: [
        {
          rank: 1,
          user_id: 1,
          username: 'alpha',
          email: 'alpha@example.com',
          requests: 100,
          tokens: 88888,
          value: 88888,
          metric: 'tokens',
          is_current_user: false,
        },
      ],
      current_user: {
        rank: 9,
        user_id: 2,
        username: 'demo',
        email: 'demo@example.com',
        requests: 22,
        tokens: 12345,
        value: 12345,
        metric: 'tokens',
        is_current_user: true,
      },
    })
  })

  it('renders game center preview and usage leaderboard preview cards', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          UserDashboardStats: true,
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === \'string\' ? to : \'#\'"><slot /></a>',
          },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Lucky Wheel')
    expect(text).toContain('2026-06-06')
    expect(text).toContain('alpha')
    expect(text).toContain('9')
  })
})
