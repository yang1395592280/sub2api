import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking, listOpenAIAutoCheapestUsers } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  listOpenAIAutoCheapestUsers: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking,
      listOpenAIAutoCheapestUsers
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.count !== undefined ? `${key}:${params.count}` : key
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  openai_auto_cheapest_users: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()
    listOpenAIAutoCheapestUsers.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
    listOpenAIAutoCheapestUsers.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 10,
      pages: 0
    })
  })

  it('uses today as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          TopUsersLeaderboard: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true,
          BaseDialog: { template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>', props: ['show', 'title'] },
          Pagination: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const today = formatLocalDate(now)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: today,
      end_date: today,
      granularity: 'hour'
    }))
  })

  it('shows OpenAI auto cheapest user count in API key card', async () => {
    getSnapshotV2.mockResolvedValue({
      stats: {
        ...createDashboardStats(),
        total_api_keys: 16,
        active_api_keys: 9,
        openai_auto_cheapest_users: 12
      },
      trend: [],
      models: []
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          TopUsersLeaderboard: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true,
          BaseDialog: { template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>', props: ['show', 'title'] },
          Pagination: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.dashboard.openaiAutoCheapestUsers:12')
  })

  it('opens paginated OpenAI auto cheapest users dialog from API key card', async () => {
    getSnapshotV2.mockResolvedValue({
      stats: {
        ...createDashboardStats(),
        openai_auto_cheapest_users: 2
      },
      trend: [],
      models: []
    })
    listOpenAIAutoCheapestUsers.mockResolvedValue({
      items: [
        {
          id: 11,
          email: 'auto@example.com',
          username: 'auto-user',
          role: 'user',
          balance: 0,
          concurrency: 0,
          status: 'active',
          allowed_groups: [],
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          notes: '',
          last_used_at: '2026-06-30T01:00:00Z',
          auto_group_max_rate_multipliers: [0.8],
          has_unlimited_auto_group_max_rate: false
        }
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1
    })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          TopUsersLeaderboard: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true,
          BaseDialog: { template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>', props: ['show', 'title'] },
          Pagination: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('button[title="admin.dashboard.openaiAutoCheapestUsers:2"]').trigger('click')
    await flushPromises()

    expect(listOpenAIAutoCheapestUsers).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('admin.dashboard.openaiAutoCheapestUsersDialogTitle')
    expect(wrapper.text()).toContain('admin.dashboard.openaiAutoCheapestMaxRate')
    expect(wrapper.text()).toContain('admin.users.lastUsedAt')
    expect(wrapper.text()).toContain('auto@example.com')
    expect(wrapper.text()).toContain('≤ 0.8')
  })
})
