import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        if (key === 'admin.accounts.bulkActions.refreshBalance') return '批量刷新余额'
        return key
      }
    })
  }
})

describe('AccountBulkActionsBar', () => {
  it('emits refresh-balance for selected accounts', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1, 2]
      }
    })

    await wrapper.get('button[data-test="bulk-refresh-balance"]').trigger('click')

    expect(wrapper.emitted('refresh-balance')).toHaveLength(1)
  })

  it('disables refresh-balance while balance refresh is running', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1, 2],
        balanceRefreshing: true
      }
    })

    const button = wrapper.get('button[data-test="bulk-refresh-balance"]')

    expect(button.attributes('disabled')).toBeDefined()
    await button.trigger('click')
    expect(wrapper.emitted('refresh-balance')).toBeUndefined()
  })
})
