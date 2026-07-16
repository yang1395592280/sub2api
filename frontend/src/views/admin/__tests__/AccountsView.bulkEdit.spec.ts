import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  refreshUpstreamBalance,
  showError,
  showSuccess,
  showInfo,
  showWarning
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  refreshUpstreamBalance: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  showWarning: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
	  getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
	  probeUpstreamBillingBatch: vi.fn().mockResolvedValue([]),
      refreshUpstreamBalance
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo,
    showWarning
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.accounts.bulkActions.refreshBalancePartial') {
          return `balance partial ${params?.success}/${params?.failed}`
        }
        if (key === 'admin.accounts.bulkActions.refreshBalanceSuccess') {
          return `balance success ${params?.count}`
        }
        return key
      }
    })
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <span v-for="column in columns" :key="column.key" data-test="column-key">{{ column.key }}</span>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-created_at" :value="row.created_at" :row="row" />
        <slot name="cell-upstream_group" :row="row" />
      </div>
    </div>
  `
}

const AccountBulkActionsBarStub = {
  props: ['selectedIds'],
  emits: ['edit-filtered', 'refresh-balance'],
  template: '<div><button data-test="edit-filtered" @click="$emit(\'edit-filtered\')">edit filtered</button><button data-test="refresh-balance" @click="$emit(\'refresh-balance\')">refresh balance</button></div>'
}

const BulkEditAccountModalStub = {
  props: ['show', 'target'],
  template: '<div data-test="bulk-edit-modal" :data-show="String(show)" :data-target-mode="target?.mode ?? \'\'"></div>'
}

const BatchAccountTestModalStub = {
  props: ['show', 'accounts'],
  template: '<div data-test="batch-test-modal" :data-show="String(show)" :data-account-count="String(accounts?.length ?? 0)"></div>'
}

describe('admin AccountsView bulk edit scope', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    refreshUpstreamBalance.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('opens bulk edit in filtered-results mode from the bulk actions dropdown', async () => {
    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-target-mode')).toBe('filtered')
  })

  it('renders the created_at column by default', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'test-account',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    const columnKeys = wrapper.findAll('[data-test="column-key"]').map(node => node.text())
    expect(columnKeys).toContain('created_at')
    const columns = wrapper.getComponent(DataTableStub).props('columns') as Array<{ key: string; label: string; sortable: boolean }>
    expect(columns.find(column => column.key === 'created_at')).toMatchObject({
      label: 'admin.accounts.columns.createdAt',
      sortable: true
    })
  })

  it('renders upstream group with effective and base rate multipliers', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'openai-sub2api',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z',
          extra: {
            upstream_group: '额度模式 - 标准',
            upstream_group_rate_multiplier: 0.4,
            upstream_effective_rate_multiplier: 0.09,
            upstream_rate_source: 'user_group_rate',
            upstream_price_guard_status: 'blocked',
            upstream_price_guard_actual_multiplier: 0.12,
            upstream_price_guard_max_multiplier: 0.08
          }
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('额度模式 - 标准')
    expect(wrapper.text()).toContain('真实 0.09x')
    expect(wrapper.text()).toContain('基础 0.4x')
    expect(wrapper.text()).toContain('价格超限 0.12x > 0.08x')
    const badge = wrapper.get('[data-test="upstream-group-badge"]')
    expect(badge.classes()).toEqual(expect.arrayContaining(['rounded-xl', 'bg-blue-50', 'border-blue-200', 'text-blue-950']))
    const priceGuardLabel = wrapper.findAll('span').find(node => node.text() === '价格超限 0.12x > 0.08x')
    expect(priceGuardLabel?.classes()).toEqual(
      expect.arrayContaining(['text-red-600', 'dark:text-red-300'])
    )
  })

  it('renders price guard status without upstream group text', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'openai-sub2api',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z',
          extra: {
            upstream_group: '',
            upstream_price_guard_status: 'blocked',
            upstream_price_guard_actual_multiplier: 0.12,
            upstream_price_guard_max_multiplier: 0.08
          }
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('价格超限 0.12x > 0.08x')
    expect(wrapper.find('[data-test="upstream-group-badge"]').exists()).toBe(true)
  })

  it('opens batch account test modal with selected accounts', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'acc-a',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        },
        {
          id: 2,
          name: 'acc-b',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const AccountBulkActionsBarTestStub = {
      props: ['selectedIds'],
      emits: ['test-selected'],
      template: '<button data-test="test-selected" @click="$emit(\'test-selected\')">test selected</button>'
    }

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarTestStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()
    await wrapper.vm.toggleSel(1)
    await wrapper.vm.toggleSel(2)
    await flushPromises()
    await wrapper.get('[data-test="test-selected"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="batch-test-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="batch-test-modal"]').attributes('data-account-count')).toBe('2')
  })

  it('refreshes upstream balance for selected OpenAI and Anthropic API Key accounts one by one and keeps going after failures', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const accounts = [
      {
        id: 1,
        name: 'openai-a',
        platform: 'openai',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        created_at: '2026-03-07T10:00:00Z',
        updated_at: '2026-03-07T10:00:00Z'
      },
      {
        id: 2,
        name: 'openai-b',
        platform: 'openai',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        created_at: '2026-03-07T10:00:00Z',
        updated_at: '2026-03-07T10:00:00Z'
      },
      {
        id: 3,
        name: 'anthropic-oauth',
        platform: 'anthropic',
        type: 'oauth',
        status: 'active',
        schedulable: true,
        created_at: '2026-03-07T10:00:00Z',
        updated_at: '2026-03-07T10:00:00Z'
      },
      {
        id: 4,
        name: 'anthropic-key',
        platform: 'anthropic',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        created_at: '2026-03-07T10:00:00Z',
        updated_at: '2026-03-07T10:00:00Z'
      },
      {
        id: 5,
        name: 'openai-oauth',
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        schedulable: true,
        created_at: '2026-03-07T10:00:00Z',
        updated_at: '2026-03-07T10:00:00Z'
      }
    ]

    listAccounts.mockResolvedValue({
      items: accounts,
      total: accounts.length,
      page: 1,
      page_size: 20,
      pages: 1
    })
    const firstRefresh = deferred<(typeof accounts)[number] & { extra: Record<string, unknown> }>()
    const secondRefresh = deferred<(typeof accounts)[number] & { extra: Record<string, unknown> }>()
    const thirdRefresh = deferred<(typeof accounts)[number] & { extra: Record<string, unknown> }>()
    refreshUpstreamBalance
      .mockReturnValueOnce(firstRefresh.promise)
      .mockReturnValueOnce(secondRefresh.promise)
      .mockReturnValueOnce(thirdRefresh.promise)

    const AccountBulkActionsBarRefreshBalanceStub = {
      props: ['selectedIds'],
      emits: ['refresh-balance'],
      template: '<button data-test="refresh-balance" @click="$emit(\'refresh-balance\')">refresh balance</button>'
    }

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarRefreshBalanceStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()
    await wrapper.vm.toggleSel(1)
    await wrapper.vm.toggleSel(2)
    await wrapper.vm.toggleSel(3)
    await wrapper.vm.toggleSel(4)
    await wrapper.vm.toggleSel(5)
    await flushPromises()

    await wrapper.get('[data-test="refresh-balance"]').trigger('click')
    await wrapper.get('[data-test="refresh-balance"]').trigger('click')
    await flushPromises()

    expect(refreshUpstreamBalance).toHaveBeenCalledTimes(1)
    expect(refreshUpstreamBalance).toHaveBeenLastCalledWith(1)

    firstRefresh.resolve({
      ...accounts[0],
      extra: {
        upstream_balance_status: 'ok',
        upstream_balance_remaining: 12.34,
        upstream_balance_unit: 'USD'
      }
    })
    await flushPromises()

    expect(refreshUpstreamBalance).toHaveBeenCalledTimes(2)
    expect(refreshUpstreamBalance).toHaveBeenLastCalledWith(2)
    expect(wrapper.vm.accounts.find(account => account.id === 1)?.extra?.upstream_balance_remaining).toBe(12.34)

    secondRefresh.reject(new Error('upstream unavailable'))
    await flushPromises()

    expect(refreshUpstreamBalance).toHaveBeenCalledTimes(3)
    expect(refreshUpstreamBalance).toHaveBeenLastCalledWith(4)

    thirdRefresh.resolve({
      ...accounts[3],
      extra: {
        upstream_balance_status: 'ok',
        upstream_balance_remaining: 56,
        upstream_balance_unit: 'USD'
      }
    })
    await flushPromises()

    expect(refreshUpstreamBalance).toHaveBeenCalledTimes(3)
    expect(refreshUpstreamBalance.mock.calls.map(([id]) => id)).toEqual([1, 2, 4])
    expect(wrapper.vm.accounts.find(account => account.id === 4)?.extra?.upstream_balance_remaining).toBe(56)
    expect(showError).toHaveBeenCalledWith('balance partial 2/1')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(consoleError).toHaveBeenCalledWith('Failed to refresh upstream balance:', expect.any(Error))
  })

  it('does not show local account groups as the upstream group fallback', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 21,
          name: 'openai-local-monitor-group',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z',
          extra: {
            upstream_group: ''
          },
          group_ids: [5],
          groups: [
            { id: 5, name: '监控渠道', rate_multiplier: 1 }
          ]
        },
        {
          id: 22,
          name: 'openai-upstream-plus',
          platform: 'openai',
          type: 'apikey',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z',
          extra: {
            upstream_group: 'GPT Plus'
          },
          group_ids: [5],
          groups: [
            { id: 5, name: '监控渠道', rate_multiplier: 1 }
          ]
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(AccountsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
          AccountTableFilters: { template: '<div></div>' },
          AccountBulkActionsBar: AccountBulkActionsBarStub,
          AccountActionMenu: true,
          ImportDataModal: true,
          ReAuthAccountModal: true,
          AccountTestModal: true,
          AccountStatsModal: true,
          ScheduledTestsPanel: true,
          SyncFromCrsModal: true,
          TempUnschedStatusModal: true,
          ErrorPassthroughRulesModal: true,
          TLSFingerprintProfilesModal: true,
          CreateAccountModal: true,
          EditAccountModal: true,
          BulkEditAccountModal: BulkEditAccountModalStub,
          BatchAccountTestModal: BatchAccountTestModalStub,
          PlatformTypeBadge: true,
          AccountCapacityCell: true,
          AccountStatusIndicator: true,
          AccountTodayStatsCell: true,
          AccountGroupsCell: true,
          AccountUsageCell: true,
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('监控渠道')
    expect(wrapper.text()).toContain('GPT Plus')
  })
})
