import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import RedeemView from '../RedeemView.vue'

const { listRedeemCodes, batchUpdateRedeemCodes, generateCodes, listCampaigns, createCampaign, updateCampaign, getAllGroups, showSuccess, showError, showInfo } =
  vi.hoisted(() => ({
    listRedeemCodes: vi.fn(),
    batchUpdateRedeemCodes: vi.fn(),
    generateCodes: vi.fn(),
    listCampaigns: vi.fn(),
    createCampaign: vi.fn(),
    updateCampaign: vi.fn(),
    getAllGroups: vi.fn(),
    showSuccess: vi.fn(),
    showError: vi.fn(),
    showInfo: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    redeem: {
      list: listRedeemCodes,
      generate: generateCodes,
      listCampaigns,
      createCampaign,
      updateCampaign,
      getCampaign: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      batchUpdate: batchUpdateRedeemCodes,
      exportCodes: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    showInfo
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
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

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <table>
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key">
            <slot :name="'header-' + column.key" :column="column">{{ column.label }}</slot>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td v-for="column in columns" :key="column.key">
            <slot :name="'cell-' + column.key" :row="row" :value="row[column.key]">
              {{ row[column.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  setup(props: { options: Array<{ value: unknown; label: string }> }, { emit }: { emit: (event: string, ...args: unknown[]) => void }) {
    const onChange = (event: Event) => {
      const raw = (event.target as HTMLSelectElement).value
      const option = props.options.find((item) => String(item.value ?? '') === raw)
      const value = option ? option.value : raw
      emit('update:modelValue', value)
      emit('change', value, option ?? null)
    }
    return { onChange }
  },
  template: `
    <select v-bind="$attrs" :value="modelValue ?? ''" @change="onChange">
      <option v-for="option in options" :key="String(option.value ?? '')" :value="option.value ?? ''">
        {{ option.label }}
      </option>
    </select>
  `
}

describe('admin RedeemView batch update', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''

    listRedeemCodes.mockReset()
    batchUpdateRedeemCodes.mockReset()
    getAllGroups.mockReset()
    generateCodes.mockReset()
    listCampaigns.mockReset()
    createCampaign.mockReset()
    updateCampaign.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    showInfo.mockReset()

    listRedeemCodes.mockResolvedValue({
      items: [
        {
          id: 1,
          code: 'CODE-1',
          type: 'balance',
          value: 10,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        },
        {
          id: 2,
          code: 'CODE-2',
          type: 'balance',
          value: 20,
          status: 'unused',
          used_by: null,
          used_at: null,
          created_at: '2026-01-01T00:00:00Z',
          expires_at: null
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    batchUpdateRedeemCodes.mockResolvedValue({ updated: 1, message: 'ok' })
    getAllGroups.mockResolvedValue([])
    generateCodes.mockResolvedValue([])
    listCampaigns.mockResolvedValue({
      items: [{ id: 7, name: '周五福利', per_user_limit: 1 }],
      total: 1, page: 1, page_size: 20, pages: 1
    })
  })

  it('submits only checked fields for selected redeem codes', async () => {
    const wrapper = mount(RedeemView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          Select: SelectStub,
          GroupBadge: true,
          GroupOptionItem: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.findAll('[data-test="select-code"]')[0].setValue(true)
    await wrapper.get('[data-test="batch-update-open"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="batch-field-status"]').setValue(true)
    await wrapper.get('[data-test="batch-status-select"]').setValue('disabled')
    await wrapper.get('[data-test="batch-field-notes"]').setValue(true)
    await wrapper.get('[data-test="batch-notes-input"]').setValue('maintenance')
    await wrapper.get('[data-test="batch-update-form"]').trigger('submit')
    await flushPromises()

    expect(batchUpdateRedeemCodes).toHaveBeenCalledWith([1], {
      status: 'disabled',
      notes: 'maintenance'
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.redeem.batchUpdateSuccess')
  })

  const mountCampaignView = () => mount(RedeemView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        DataTable: DataTableStub, Pagination: true, ConfirmDialog: true,
        Select: SelectStub, GroupBadge: true, GroupOptionItem: true, Icon: true, Teleport: true
      }
    }
  })

  it.each(['balance', 'lottery_ticket'])('associates %s codes with the selected campaign', async (type) => {
    const wrapper = mountCampaignView()
    await flushPromises()
    await wrapper.get('[data-test="generate-open"]').trigger('click')
    await wrapper.get('[data-test="generate-type"]').setValue(type)
    await wrapper.get('[data-test="campaign-choose-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="campaign-select"]').trigger('click')
    expect(wrapper.get('[data-test="generate-campaign"]').text()).toContain('周五福利')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()
    expect(generateCodes).toHaveBeenCalledWith(1, type, type === 'balance' ? 10 : 1, undefined, undefined, undefined, 7)
    wrapper.unmount()
  })

  it('creates a campaign with an editable limit and selects it for generation', async () => {
    createCampaign.mockResolvedValue({ id: 8, name: '新福利', per_user_limit: 3 })
    const wrapper = mountCampaignView()
    await flushPromises()
    await wrapper.get('[data-test="generate-open"]').trigger('click')
    await wrapper.get('[data-test="campaign-choose-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="campaign-name"]').setValue('新福利')
    await wrapper.get('[data-test="campaign-limit"]').setValue(3)
    await wrapper.get('[data-test="campaign-form"]').trigger('submit')
    await flushPromises()
    expect(createCampaign).toHaveBeenCalledWith({ name: '新福利', per_user_limit: 3 }, expect.any(String))
    expect(wrapper.get('[data-test="generate-campaign"]').text()).toContain('新福利')
    await wrapper.get('[data-test="generate-form"]').trigger('submit')
    await flushPromises()
    expect(generateCodes.mock.calls[0][6]).toBe(8)
    wrapper.unmount()
  })

  it('edits an existing campaign limit and rejects zero before sending a request', async () => {
    updateCampaign.mockResolvedValue({ id: 7, name: '周五福利', per_user_limit: 3 })
    const wrapper = mountCampaignView()
    await flushPromises()
    await wrapper.get('[data-test="campaign-manage-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="campaign-edit"]').trigger('click')
    await wrapper.get('[data-test="campaign-limit"]').setValue(0)
    await wrapper.get('[data-test="campaign-form"]').trigger('submit')
    expect(updateCampaign).not.toHaveBeenCalled()
    await wrapper.get('[data-test="campaign-limit"]').setValue(3)
    await wrapper.get('[data-test="campaign-form"]').trigger('submit')
    await flushPromises()
    expect(updateCampaign).toHaveBeenCalledWith(7, { name: '周五福利', per_user_limit: 3 }, expect.any(String))
    wrapper.unmount()
  })

  it('reuses the operation key after a failed save and keeps the entered limit', async () => {
    createCampaign.mockRejectedValueOnce({ message: 'Network unavailable' }).mockResolvedValueOnce({ id: 8, name: '福利', per_user_limit: 2 })
    const wrapper = mountCampaignView()
    await flushPromises()
    await wrapper.get('[data-test="campaign-manage-open"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="campaign-name"]').setValue('福利')
    await wrapper.get('[data-test="campaign-limit"]').setValue(2)
    await wrapper.get('[data-test="campaign-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('Network unavailable')
    expect((wrapper.get('[data-test="campaign-limit"]').element as HTMLInputElement).value).toBe('2')
    await wrapper.get('[data-test="campaign-form"]').trigger('submit')
    await flushPromises()
    expect(createCampaign.mock.calls[0][1]).toBe(createCampaign.mock.calls[1][1])
    wrapper.unmount()
  })
})
