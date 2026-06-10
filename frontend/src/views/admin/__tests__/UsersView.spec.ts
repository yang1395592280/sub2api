import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getUserBalanceSummary,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes,
  batchAddBalanceToUsers
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getUserBalanceSummary: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn(),
  batchAddBalanceToUsers: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      getUserBalanceSummary,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      batchAddBalanceToUsers,
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
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
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

const createAdminUser = (): AdminUser => ({
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
  current_concurrency: 0
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-select" :row="row" />
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
      </div>
    </div>
  `
}

describe('admin UsersView', () => {
  beforeEach(() => {
    localStorage.clear()

    listUsers.mockReset()
    getUserBalanceSummary.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()
    batchAddBalanceToUsers.mockReset()

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
