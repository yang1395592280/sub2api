import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIHealthView from '../OpenAIHealthView.vue'

const appStoreMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getAll: vi.fn().mockResolvedValue([
        { id: 33, name: 'GPT Plus', platform: 'openai', status: 'active' },
      ]),
    },
    openaiHealth: {
      getOverview: vi.fn(),
    },
    openaiScheduler: {
      getOverview: vi.fn().mockResolvedValue({
        settings: {
          health_ranking_enabled: true,
          primary_ratio: 0.3,
          primary_min_count: 1,
          ttft_degrade_ms: 2500,
          error_rate_degrade_threshold: 0.35,
          consecutive_failure_threshold: 3,
          recover_success_threshold: 5,
          cooldown_seconds: 600,
          observe_probe_ratio: 0,
        },
        tier_counts: {
          primary: 1,
          standby: 0,
          observe: 0,
          degraded: 0,
        },
      }),
      listAccounts: vi.fn().mockResolvedValue({
        items: [
          {
            account_id: 7,
            account_name: 'Kedaya',
            platform: 'openai',
            type: 'oauth',
            status: 'active',
            manual_priority: 10,
            channel_price: 0.08,
            groups: [33],
            health: {
              account_id: 7,
              health_score: 96.4,
              tier: 'primary',
              degrade_reason: '',
              success_rate_ewma: 0.93,
              error_rate_ewma: 0.07,
              ttft_ewma_ms: 1186,
              consecutive_errors: 0,
              consecutive_ok: 8,
              decision_reason: 'healthy',
            },
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
      getDailyStats: vi.fn().mockResolvedValue({
        date: '2026-06-16',
        group_id: 33,
        total_selects: 30,
        accounts: [
          {
            account_id: 7,
            select_count: 30,
            select_ratio: 1,
            last_selected_at: '2026-06-16T09:50:00Z',
          },
        ],
      }),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreMocks,
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

describe('OpenAIHealthView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads scheduler account health rows instead of channel monitor rows', async () => {
    const { adminAPI } = await import('@/api/admin')

    const wrapper = mount(OpenAIHealthView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: {
            props: ['data'],
            template: `
              <div>
                <div v-for="row in data" :key="row.account_id">
                  <slot name="cell-account_name" :row="row" />
                  <slot name="cell-tier" :row="row" />
                  <slot name="cell-health_score" :row="row" />
                  <slot name="cell-success_rate" :row="row" />
                  <slot name="cell-ttft" :row="row" />
                  <slot name="cell-select_count" :row="row" />
                  <slot name="cell-last_selected_at" :row="row" />
                </div>
              </div>
            `,
          },
          Pagination: true,
          EmptyState: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(adminAPI.groups.getAll).toHaveBeenCalledWith('openai')
    expect(adminAPI.openaiHealth.getOverview).not.toHaveBeenCalled()
    expect(adminAPI.openaiScheduler.getOverview).toHaveBeenCalledWith({ group_id: 33 })
    expect(adminAPI.openaiScheduler.listAccounts).toHaveBeenCalledWith(expect.objectContaining({ group_id: 33 }), expect.any(Object))
    expect(adminAPI.openaiScheduler.getDailyStats).toHaveBeenCalledWith(expect.objectContaining({ group_id: 33 }))
    expect(wrapper.text()).toContain('Kedaya')
    expect(wrapper.text()).toContain('#7')
    expect(wrapper.text()).toContain('oauth')
    expect(wrapper.text()).toContain('96.4')
    expect(wrapper.text()).toContain('93%')
    expect(wrapper.text()).toContain('1186ms')
    expect(wrapper.text()).toContain('30')
  })
})
