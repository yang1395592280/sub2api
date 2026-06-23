import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  getRoutingRanking,
  getRoutingExplain,
  showError,
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  getRoutingRanking: vi.fn(),
  getRoutingExplain: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
    },
    proxies: {
      getAll: getAllProxies,
    },
    groups: {
      getAll: getAllGroups,
    },
  },
}))

vi.mock('@/api/admin/openaiScheduler', () => ({
  openaiSchedulerAPI: {
    getRoutingRanking,
    getRoutingExplain,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token',
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      te: () => false,
    }),
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div data-test="data-table">
      <span v-for="column in columns" :key="column.key" data-test="column-key">{{ column.key }}</span>
      <div v-for="row in data" :key="row.id" :data-row-id="String(row.id)">
        <slot name="cell-routing_priority" :row="row" />
      </div>
    </div>
  `,
}

const AccountTableFiltersStub = {
  emits: ['change', 'update:searchQuery', 'update:filters'],
  template: `
    <div>
      <button data-test="filter-change" type="button" @click="$emit('change')">change</button>
      <button data-test="filter-search" type="button" @click="$emit('update:searchQuery', 'cheap')">search</button>
    </div>
  `,
}

const PaginationStub = {
  props: ['page', 'pageSize', 'total'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-next" type="button" @click="$emit('update:page', Number(page) + 1)">next</button>
      <button data-test="page-size-50" type="button" @click="$emit('update:pageSize', 50)">size 50</button>
    </div>
  `,
}

const RoutingPriorityBadgeStub = {
  props: ['summary'],
  emits: ['open'],
  template: `
    <button
      v-if="summary"
      type="button"
      data-test="routing-priority-badge"
      @click="$emit('open')"
    >
      {{ summary.rank ? '#' + summary.rank : summary.status_label }} {{ summary.summary_reason }}
    </button>
    <span v-else data-test="routing-priority-empty">empty</span>
  `,
}

const OpenAIRoutingExplainModalStub = {
  props: ['show', 'loading', 'explain', 'accountId'],
  emits: ['close'],
  template: `
    <div
      data-test="routing-explain-modal"
      :data-show="String(show)"
      :data-loading="String(loading)"
      :data-account-id="accountId == null ? '' : String(accountId)"
    >
      {{ explain?.account?.account_name ?? '' }}
    </div>
  `,
}

function buildRankingSummary(accountId: number, summaryReason: string) {
  return {
    account_id: accountId,
    account_name: `account-${accountId}`,
    rank: 1,
    tier: 'primary' as const,
    status_label: 'candidate' as const,
    summary_reason: summaryReason as any,
    summary_reasons: [summaryReason as any],
    is_schedulable_now: true,
    score: {
      total: 3,
      priority: 1,
      load: 1,
      queue: 1,
      error_rate: 1,
      ttft: 1,
      price: 1,
      health: 1,
    },
    snapshot_at: '2026-06-23T00:00:00Z',
  }
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
        },
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: AccountTableFiltersStub,
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        BatchAccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        RoutingPriorityBadge: RoutingPriorityBadgeStub,
        OpenAIRoutingExplainModal: OpenAIRoutingExplainModalStub,
        Icon: true,
        HelpTooltip: true,
      },
    },
  })
}

describe('admin AccountsView routing priority', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    getRoutingRanking.mockReset()
    getRoutingExplain.mockReset()
    showError.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          id: 10,
          name: 'cheap-fast',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          priority: 1,
          concurrency: 5,
          error_message: null,
          last_used_at: null,
          expires_at: null,
          auto_pause_on_expired: false,
          created_at: '2026-06-23T00:00:00Z',
          updated_at: '2026-06-23T00:00:00Z',
          proxy_id: null,
        },
        {
          id: 20,
          name: 'other-platform',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          priority: 2,
          concurrency: 2,
          error_message: null,
          last_used_at: null,
          expires_at: null,
          auto_pause_on_expired: false,
          created_at: '2026-06-23T00:00:00Z',
          updated_at: '2026-06-23T00:00:00Z',
          proxy_id: null,
        },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listWithEtag.mockResolvedValue({
      notModified: false,
      etag: 'etag-1',
      data: {
        items: [
          {
            id: 10,
            name: 'cheap-fast',
            platform: 'openai',
            type: 'apikey',
            status: 'active',
            schedulable: true,
            priority: 1,
            concurrency: 5,
            error_message: null,
            last_used_at: null,
            expires_at: null,
            auto_pause_on_expired: false,
            created_at: '2026-06-23T00:00:00Z',
            updated_at: '2026-06-24T00:00:00Z',
            proxy_id: null,
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
        pages: 1,
      },
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    getRoutingRanking
      .mockResolvedValueOnce({
        items: [buildRankingSummary(10, 'cost_advantage')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-23T00:00:00Z',
      })
      .mockResolvedValueOnce({
        items: [buildRankingSummary(10, 'low_load')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-24T00:00:00Z',
      })
    getRoutingExplain.mockResolvedValue({
      account: buildRankingSummary(10, 'low_load'),
      top: [buildRankingSummary(10, 'low_load')],
      notes: [],
    })
  })

  it('renders routing priority for OpenAI accounts and loads explain on demand', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getRoutingRanking).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('[data-test="column-key"]').map((node) => node.text())).toContain('routing_priority')

    const badges = wrapper.findAll('[data-test="routing-priority-badge"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toContain('#1')
    expect(badges[0].text()).toContain('cost_advantage')
    expect(wrapper.text()).toContain('-')

    await badges[0].trigger('click')
    await flushPromises()

    expect(getRoutingExplain).toHaveBeenCalledWith(10, {})
    const modal = wrapper.get('[data-test="routing-explain-modal"]')
    expect(modal.attributes('data-show')).toBe('true')
    expect(modal.attributes('data-account-id')).toBe('10')
    expect(modal.text()).toContain('account-10')
  })

  it('preserves previous routing priority until incremental refresh ranking arrives', async () => {
    vi.useFakeTimers()
    localStorage.setItem('account-auto-refresh', JSON.stringify({
      enabled: true,
      interval_seconds: 5,
    }))

    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(getRoutingRanking).toHaveBeenCalledTimes(2)
    const badges = wrapper.findAll('[data-test="routing-priority-badge"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toContain('low_load')
  })

  it('refreshes routing priorities after debounced filter reload completes', async () => {
    vi.useFakeTimers()
    getRoutingRanking.mockReset()
    getRoutingRanking
      .mockResolvedValueOnce({
        items: [buildRankingSummary(10, 'cost_advantage')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-23T00:00:00Z',
      })
      .mockResolvedValueOnce({
        items: [buildRankingSummary(10, 'filter_reload')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-24T00:00:00Z',
      })

    const wrapper = mountView()
    await flushPromises()

    expect(getRoutingRanking).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="routing-priority-badge"]').text()).toContain('cost_advantage')

    await wrapper.get('[data-test="filter-search"]').trigger('click')
    await vi.advanceTimersByTimeAsync(301)
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledTimes(2)
    expect(getRoutingRanking).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="routing-priority-badge"]').text()).toContain('filter_reload')
  })

  it('refreshes routing priorities after pagination and page size reloads complete', async () => {
    vi.useFakeTimers()
    getRoutingRanking.mockReset()
    getRoutingRanking
      .mockResolvedValueOnce({
        items: [buildRankingSummary(10, 'cost_advantage')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-23T00:00:00Z',
      })
      .mockResolvedValueOnce({
        items: [buildRankingSummary(11, 'page_two')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-24T00:00:00Z',
      })
      .mockResolvedValueOnce({
        items: [buildRankingSummary(12, 'page_size_reload')],
        source: 'scheduler_snapshot',
        snapshot_at: '2026-06-25T00:00:00Z',
      })

    listAccounts.mockImplementation(async (page: number, pageSize: number) => {
      if (page === 2) {
        return {
          items: [
            {
              id: 11,
              name: 'page-two-openai',
              platform: 'openai',
              type: 'apikey',
              status: 'active',
              schedulable: true,
              priority: 1,
              concurrency: 3,
              error_message: null,
              last_used_at: null,
              expires_at: null,
              auto_pause_on_expired: false,
              created_at: '2026-06-23T00:00:00Z',
              updated_at: '2026-06-24T00:00:00Z',
              proxy_id: null,
            },
          ],
          total: 2,
          page: 2,
          page_size: pageSize,
          pages: 2,
        }
      }

      if (pageSize === 50) {
        return {
          items: [
            {
              id: 12,
              name: 'page-size-openai',
              platform: 'openai',
              type: 'apikey',
              status: 'active',
              schedulable: true,
              priority: 1,
              concurrency: 4,
              error_message: null,
              last_used_at: null,
              expires_at: null,
              auto_pause_on_expired: false,
              created_at: '2026-06-23T00:00:00Z',
              updated_at: '2026-06-25T00:00:00Z',
              proxy_id: null,
            },
          ],
          total: 1,
          page: 1,
          page_size: 50,
          pages: 1,
        }
      }

      return {
        items: [
          {
            id: 10,
            name: 'cheap-fast',
            platform: 'openai',
            type: 'apikey',
            status: 'active',
            schedulable: true,
            priority: 1,
            concurrency: 5,
            error_message: null,
            last_used_at: null,
            expires_at: null,
            auto_pause_on_expired: false,
            created_at: '2026-06-23T00:00:00Z',
            updated_at: '2026-06-23T00:00:00Z',
            proxy_id: null,
          },
        ],
        total: 2,
        page: 1,
        page_size: pageSize,
        pages: 2,
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="page-next"]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(getRoutingRanking).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="routing-priority-badge"]').text()).toContain('page_two')

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(getRoutingRanking).toHaveBeenCalledTimes(3)
    expect(wrapper.find('[data-test="routing-priority-badge"]').text()).toContain('page_size_reload')
  })
})
