import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAISchedulerView from '../OpenAISchedulerView.vue'

const appStoreMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getAll: vi.fn().mockResolvedValue([
        { id: 33, name: 'codex-plus高速渠道', platform: 'openai', status: 'active' },
      ]),
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
        primary_count: 1,
        standby_count: 1,
        observe_count: 0,
        degraded_count: 1,
      }),
      listAccounts: vi.fn().mockResolvedValue({
        items: [
          {
            account_id: 1,
            account_name: 'openai-fast',
            platform: 'openai',
            type: 'oauth',
            status: 'active',
            manual_priority: 10,
            groups: [1],
            health: {
              account_id: 1,
              health_score: 98,
              tier: 'primary',
              degrade_reason: '',
              success_rate_ewma: 0.99,
              error_rate_ewma: 0.01,
              ttft_ewma_ms: 820,
              consecutive_errors: 0,
              consecutive_ok: 8,
              decision_reason: 'fast and healthy',
            },
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      }),
      updateSettings: vi.fn(),
      applyAction: vi.fn(),
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

describe('OpenAISchedulerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders scheduler accounts', async () => {
    const wrapper = mount(OpenAISchedulerView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: {
            props: ['data'],
            template: '<div><div v-for="row in data" :key="row.account_id">{{ row.account_name }} {{ row.health.tier }}</div></div>',
          },
          Pagination: true,
          Toggle: true,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('openai-fast')
    expect(wrapper.text()).toContain('primary')
  })

  it('loads scheduler data for the selected OpenAI group', async () => {
    const { adminAPI } = await import('@/api/admin')

    mount(OpenAISchedulerView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: true,
          Pagination: true,
          Toggle: true,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(adminAPI.groups.getAll).toHaveBeenCalledWith('openai')
    expect(adminAPI.openaiScheduler.getOverview).toHaveBeenCalledWith({ group_id: 33 })
    expect(adminAPI.openaiScheduler.listAccounts).toHaveBeenCalledWith(expect.objectContaining({ group_id: 33 }))
  })

  it('shows backend scheduler action errors', async () => {
    const { adminAPI } = await import('@/api/admin')
    vi.mocked(adminAPI.openaiScheduler.applyAction).mockRejectedValueOnce({
      message: 'openai advanced scheduler is not enabled',
    })

    const wrapper = mount(OpenAISchedulerView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: {
            props: ['data'],
            template: '<div><div v-for="row in data" :key="row.account_id"><slot name="cell-actions" :row="row" /></div></div>',
          },
          Pagination: true,
          Toggle: true,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })

    await flushPromises()
    const promoteButton = wrapper.findAll('button').find((button) => button.text() === 'admin.openaiScheduler.actions.promoteObserve')
    expect(promoteButton).toBeTruthy()
    await promoteButton!.trigger('click')
    await flushPromises()

    expect(adminAPI.openaiScheduler.applyAction).toHaveBeenCalledWith(1, { action: 'promote_observe' })
    expect(appStoreMocks.showError).toHaveBeenCalledWith('openai advanced scheduler is not enabled')
  })
})
