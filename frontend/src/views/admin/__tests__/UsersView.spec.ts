import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getUserBalanceSummary,
  deleteUser,
  showError,
  showSuccess,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes,
  batchAddBalanceToUsers,
  batchDeleteUsers
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getUserBalanceSummary: vi.fn(),
  deleteUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn(),
  batchAddBalanceToUsers: vi.fn(),
  batchDeleteUsers: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      getUserBalanceSummary,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      batchAddBalanceToUsers,
      batchDeleteUsers,
      batchAddGroupToUsers: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: { count?: number }) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <template v-for="col in columns" :key="col.key">
        <slot :name="'header-' + col.key" :column="col" />
      </template>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-select" :row="row" />
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
      </div>
    </div>
  `
}

const PaginationStub = {
  emits: ['update:page'],
  template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>'
}

const BulkEditUserModalStub = {
  props: ['show', 'selectedIds'],
  emits: ['close', 'success'],
  template: `
    <div v-if="show" data-test="bulk-modal">
      <span data-test="bulk-modal-ids">{{ selectedIds.join(',') }}</span>
      <button data-test="bulk-success" @click="$emit('success', selectedIds.length)">success</button>
    </div>
  `
}

const mountBulkDeleteView = () => mount(UsersView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: DataTableStub,
      Pagination: PaginationStub,
      ConfirmDialog: {
        props: ['show', 'message'],
        emits: ['confirm', 'cancel'],
        template: `<div v-if="show" data-test="delete-dialog">
          <span>{{ message }}</span>
          <button data-test="confirm-delete" @click="$emit('confirm')">confirm</button>
          <button data-test="cancel-delete" @click="$emit('cancel')">cancel</button>
        </div>`
      },
      EmptyState: true,
      GroupBadge: true,
      Select: true,
      UserAttributesConfigModal: true,
      UserConcurrencyCell: true,
      UserCreateModal: true,
      UserEditModal: true,
      BulkEditUserModal: true,
      UserPlatformQuotaModal: true,
      UserApiKeysModal: true,
      UserAllowedGroupsModal: true,
      UserBalanceModal: true,
      UserBalanceHistoryModal: true,
      GroupReplaceModal: true,
      Icon: true,
      Teleport: true
    }
  }
})
describe('admin UsersView', () => {
  beforeEach(() => {
    vi.useRealTimers()
    localStorage.clear()

    listUsers.mockReset()
    getUserBalanceSummary.mockReset()
    deleteUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()
    batchAddBalanceToUsers.mockReset()
    batchDeleteUsers.mockReset()

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getUserBalanceSummary.mockResolvedValue({
      total_balance: 66.6,
      user_count: 1
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('cancels bulk deletion without deleting or clearing selected users', async () => {
    const wrapper = mountBulkDeleteView()
    await flushPromises()

    expect(wrapper.find('[data-test="bulk-delete-users"]').exists()).toBe(false)
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    expect(wrapper.get('[data-test="delete-dialog"]').text()).toContain('admin.users.bulkDelete.confirm:1')
    expect(deleteUser).not.toHaveBeenCalled()

    await wrapper.get('[data-test="cancel-delete"]').trigger('click')
    expect(wrapper.find('[data-test="delete-dialog"]').exists()).toBe(false)
    expect(deleteUser).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    wrapper.unmount()
  })

  it.each([
    { failedIds: [], remaining: '', deleted: 2 },
    { failedIds: [43], remaining: '43', deleted: 1 },
    { failedIds: [42, 43], remaining: '42,43', deleted: 0 }
  ])('deletes across pages and retains failures: $remaining', async ({ failedIds, remaining, deleted }) => {
    listUsers.mockImplementation(async (page: number) => ({
      items: [createAdminUser({ id: page === 2 ? 43 : 42 })],
      total: 2, page, page_size: 20, pages: 2
    }))
    deleteUser.mockImplementation(async (id: number) => {
      if (failedIds.includes(id)) throw new Error('Cannot delete user')
    })
    const wrapper = mountBulkDeleteView()
    await flushPromises()
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="select-43"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    expect(deleteUser).not.toHaveBeenCalled()

    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    await flushPromises()

    expect(deleteUser.mock.calls).toEqual([[42], [43]])
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe(remaining)
    expect(wrapper.find('[data-test="delete-dialog"]').exists()).toBe(false)
    if (deleted) {
      expect(showSuccess).toHaveBeenCalledWith(`admin.users.bulkDelete.success:${deleted}`)
      expect(listUsers.mock.lastCall?.[0]).toBe(1)
    } else {
      expect(showSuccess).not.toHaveBeenCalled()
    }
    if (failedIds.length) expect(showError).toHaveBeenCalledWith(`admin.users.bulkDelete.failed:${failedIds.length}`)
    else expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('deletes the confirmed selection while preserving users selected during deletion', async () => {
    listUsers.mockResolvedValue({
      items: [createAdminUser({ id: 42 }), createAdminUser({ id: 43 })],
      total: 2, page: 1, page_size: 20, pages: 1
    })
    let finishDelete!: () => void
    deleteUser.mockImplementation(() => new Promise<void>(resolve => { finishDelete = resolve }))
    const wrapper = mountBulkDeleteView()
    await flushPromises()
    await wrapper.get('[data-test="select-42"]').trigger('click')
    await wrapper.get('[data-test="bulk-delete-users"]').trigger('click')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    expect(wrapper.get('[data-test="bulk-delete-users"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="select-43"]').trigger('click')
    finishDelete()
    await flushPromises()

    expect(deleteUser.mock.calls).toEqual([[42]])
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('43')
    wrapper.unmount()
  })

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserBulkActionsBar: {
            props: ['selectedIds'],
            template: '<div data-test="bulk-actions">{{ selectedIds.length }}</div>'
          },
          UserBatchAddGroupModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(visibleColumns).not.toContain('last_login_at')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('loads and displays the non-admin balance summary and shows bulk actions after selecting a user', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserBulkActionsBar: {
            props: ['selectedIds'],
            template: '<div data-test="bulk-actions">{{ selectedIds.length }}</div>'
          },
          UserBatchAddGroupModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(getUserBalanceSummary).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('66.6')

    await wrapper.get('input[type="checkbox"]').setValue(true)
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-actions"]').text()).toBe('1')
  })

  it('clears usage current-page sort when switching to last_used_at server sort', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-column-settings-version', '3')
    localStorage.setItem(
      'user-hidden-columns',
      JSON.stringify([
        'notes',
        'groups',
        'subscriptions',
        'concurrency',
        'usage_anthropic',
        'usage_openai',
        'usage_gemini',
        'usage_antigravity',
        'balance_platform_quota'
      ])
    )

    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'last-used-first@example.com' }),
        createAdminUser({ id: 2, email: 'usage-first@example.com' })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getBatchUsersUsage.mockResolvedValue({
      stats: {
        1: { user_id: 1, today_actual_cost: 1, total_actual_cost: 1, by_platform: [] },
        2: { user_id: 2, today_actual_cost: 9, total_actual_cost: 9, by_platform: [] }
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('usage-first@example.com,last-used-first@example.com')
    expect(localStorage.getItem('admin-users-usage-sort')).toContain('"key":"usage"')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('opens the batch add balance modal from bulk actions after selecting a user', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserBulkActionsBar: {
            props: ['selectedIds'],
            emits: ['addBalance'],
            template: '<button data-test="open-batch-balance" @click="$emit(\'addBalance\')">{{ selectedIds.length }}</button>'
          },
          UserBatchAddGroupModal: true,
          UserBatchBalanceModal: {
            props: ['show', 'userIds'],
            template: '<div data-test="batch-balance-modal">{{ show ? userIds.join(\',\') : \'hidden\' }}</div>'
          },
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-test="open-batch-balance"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="batch-balance-modal"]').text()).toBe('42')
  })

  it('confirms and sends selected user ids to the batch delete endpoint', async () => {
    batchDeleteUsers.mockResolvedValue({ deleted: 1 })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: {
            props: ['show', 'message'],
            emits: ['confirm'],
            template: '<button v-if="show" data-test="confirm-batch-delete" @click="$emit(\'confirm\')">{{ message }}</button>'
          },
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserBulkActionsBar: {
            props: ['selectedIds'],
            emits: ['delete'],
            template: '<button data-test="open-batch-delete" @click="$emit(\'delete\')">{{ selectedIds.length }}</button>'
          },
          UserBatchAddGroupModal: true,
          UserBatchBalanceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await flushPromises()
    await wrapper.get('[data-test="open-batch-delete"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="confirm-batch-delete"]').trigger('click')
    await flushPromises()

    expect(batchDeleteUsers).toHaveBeenCalledWith([42])
    expect(listUsers).toHaveBeenCalledTimes(2)
  })

  it('passes comma-separated email filters through email_list without replacing fuzzy search', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserBulkActionsBar: true,
          UserBatchAddGroupModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    listUsers.mockClear()

    const inputs = wrapper.findAll('input[type="text"]')
    await inputs[0].setValue('fuzzy-keyword')
    await inputs[1].setValue('a@example.com,b@example.com')
    await new Promise((resolve) => setTimeout(resolve, 350))
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        search: 'fuzzy-keyword',
        email_list: 'a@example.com,b@example.com'
      }),
      expect.any(Object)
    )
  })
})
