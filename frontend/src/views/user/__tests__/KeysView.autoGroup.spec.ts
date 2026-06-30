import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  createKey,
  getUsageStats,
  getAvailableGroups,
  getUserGroupRates,
  fetchPublicSettings,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  getUsageStats: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: createKey,
    update: vi.fn(),
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  usageAPI: {
    getDashboardApiKeysUsage: getUsageStats,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings,
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn(async () => true) }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const Passthrough = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  },
})

const TablePageLayoutStub = defineComponent({
  setup(_, { slots }) {
    return () =>
      h('div', [
        slots.filters?.(),
        slots.actions?.(),
        slots.table?.(),
        slots.pagination?.(),
      ])
  },
})

const BaseDialogStub = defineComponent({
  props: { show: Boolean },
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', [slots.default?.(), slots.footer?.()])
        : null
  },
})

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  setup(props, { slots }) {
    return () => h('div', props.data.length ? slots.table?.() : slots.empty?.())
  },
})

describe('KeysView OpenAI auto cheapest group', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listKeys.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    createKey.mockResolvedValue({ id: 1 })
    getUsageStats.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([
      {
        id: 2,
        name: 'OpenAI Cheap',
        description: null,
        platform: 'openai',
        rate_multiplier: 0.1,
        is_exclusive: false,
        subscription_type: 'standard',
        sort_order: 1,
      },
    ])
    getUserGroupRates.mockResolvedValue({})
    fetchPublicSettings.mockResolvedValue({})
  })

  it('submits auto cheapest mode with a null group id', async () => {
    const wrapper = mount(KeysView, {
      global: {
        stubs: {
          AppLayout: Passthrough,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: true,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          EmptyState: true,
          SearchInput: true,
          EndpointPopover: true,
          UseKeyModal: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-tour="keys-create-btn"]').trigger('click')
    await wrapper.get('[data-tour="key-form-name"]').setValue('auto-openai')

    const groupSelect = wrapper
      .findAllComponents({ name: 'Select' })
      .find((select) => select.attributes('data-tour') === 'key-form-group')
    expect(groupSelect).toBeTruthy()
    groupSelect.vm.$emit('update:modelValue', 'openai_auto_cheapest')
    await wrapper.get('form#key-form').trigger('submit')
    await flushPromises()

    expect(createKey).toHaveBeenCalledWith(
      'auto-openai',
      null,
      undefined,
      [],
      [],
      0,
      undefined,
      { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 },
      'openai_auto_cheapest',
    )
    expect(showError).not.toHaveBeenCalledWith('keys.groupRequired')
  })
})
