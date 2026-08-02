import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const baseProps = {
  totalResults: 45,
  selectingAll: false,
  allResultsSelected: false
}

describe('AccountBulkActionsBar', () => {
  it('emits refresh-balance for selected accounts', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        ...baseProps,
        selectedIds: [1, 2]
      }
    })

    await wrapper.get('button[data-test="bulk-refresh-balance"]').trigger('click')

    expect(wrapper.emitted('refresh-balance')).toHaveLength(1)
  })

  it('disables refresh-balance while balance refresh is running', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        ...baseProps,
        selectedIds: [1, 2],
        balanceRefreshing: true
      }
    })

    const button = wrapper.get('button[data-test="bulk-refresh-balance"]')

    expect(button.attributes('disabled')).toBeDefined()
    await button.trigger('click')
    expect(wrapper.emitted('refresh-balance')).toBeUndefined()
  })

  it('allows selecting all results before any row is selected', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        ...baseProps,
        selectedIds: []
      }
    })

    const button = wrapper.findAll('button').find(item =>
      item.text().includes('admin.accounts.bulkActions.selectAllResults')
    )

    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('select-all-results')).toHaveLength(1)
  })

  it('preserves the upstream billing probe action', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        ...baseProps,
        selectedIds: [1]
      }
    })

    const button = wrapper.findAll('button').find(item =>
      item.text().includes('admin.accounts.bulkActions.probeUpstreamBilling')
    )

    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('probe-upstream-billing')).toHaveLength(1)
  })
})
