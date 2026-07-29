import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  createKey,
  updateKey,
  getUsageStats,
  getAvailableGroups,
  getUserGroupRates,
  fetchPublicSettings,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  updateKey: vi.fn(),
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
    update: updateKey,
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
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params?.name !== undefined ? `${key}:${params.name}` : key,
    }),
  }
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
    return () =>
      h(
        'div',
        props.data.length
          ? props.data.flatMap((row: any) => [
              slots['cell-group']?.({ row }),
              slots['cell-actions']?.({ row }),
            ])
          : slots.empty?.(),
      )
  },
})

describe('KeysView OpenAI auto cheapest group', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listKeys.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    createKey.mockResolvedValue({ id: 1 })
    updateKey.mockResolvedValue({ id: 1 })
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

  it('submits auto cheapest mode with a null group id and the default max rate', async () => {
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
      0.2,
    )
    expect(showError).not.toHaveBeenCalledWith('keys.groupRequired')
  })

  it('submits max rate multiplier for auto cheapest mode', async () => {
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
    await wrapper.get('[data-tour="key-form-name"]').setValue('auto-budget')
    const groupSelect = wrapper
      .findAllComponents({ name: 'Select' })
      .find((select) => select.attributes('data-tour') === 'key-form-group')
    expect(groupSelect).toBeTruthy()
    groupSelect.vm.$emit('update:modelValue', 'openai_auto_cheapest')
    await flushPromises()
    await wrapper.get('[data-tour="key-form-openai-auto-max-rate"]').setValue('0.8')
    await wrapper.get('form#key-form').trigger('submit')
    await flushPromises()

    expect(createKey).toHaveBeenCalledWith(
      'auto-budget',
      null,
      undefined,
      [],
      [],
      0,
      undefined,
      { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 },
      'openai_auto_cheapest',
      0.8,
    )
  })

  it('allows setting max rate when changing an existing key to auto cheapest from the row group dropdown', async () => {
    listKeys.mockResolvedValue({
      items: [
        {
          id: 7,
          key: 'sk-row',
          name: 'row-key',
          group_id: 2,
          group_select_mode: 'fixed',
          group: {
            id: 2,
            name: 'OpenAI Cheap',
            platform: 'openai',
            subscription_type: 'standard',
            rate_multiplier: 0.1,
          },
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          quota: 0,
          quota_used: 0,
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          last_used_at: null,
          expires_at: null,
          last_effective_group_id: null,
          last_effective_group_at: null,
        },
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })

    const wrapper = mount(KeysView, {
      attachTo: document.body,
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

    await wrapper.get('.group\\/dropdown button').trigger('click')
    await flushPromises()
    const autoButton = Array.from(document.body.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('keys.openaiAutoCheapest.label'),
    )
    expect(autoButton).toBeTruthy()
    autoButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    expect(updateKey).not.toHaveBeenCalled()
    const maxRateInput = wrapper.get('[data-test="row-auto-cheapest-max-rate"]')

    expect((maxRateInput.element as HTMLInputElement).value).toBe('0.2')
    await maxRateInput.setValue('0.25')
    await wrapper.get('[data-test="row-auto-cheapest-submit"]').trigger('click')
    await flushPromises()

    expect(updateKey).toHaveBeenCalledWith(7, {
      group_id: null,
      group_select_mode: 'openai_auto_cheapest',
      openai_auto_group_max_rate_multiplier: 0.25,
    })

    wrapper.unmount()
  })

  it('uses 0.2 when the inline auto cheapest max rate is cleared', async () => {
    listKeys.mockResolvedValue({
      items: [
        {
          id: 12,
          key: 'sk-fixed-default-rate',
          name: 'fixed-default-rate',
          group_id: 1,
          group_select_mode: 'fixed',
          group: { id: 1, name: 'Default', platform: 'openai', rate_multiplier: 1 },
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          quota: 0,
          quota_used: 0,
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          last_used_at: null,
          expires_at: null,
          last_effective_group_id: null,
          last_effective_group_at: null,
        },
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })

    const wrapper = mount(KeysView, {
      attachTo: document.body,
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

    await wrapper.get('.group\\/dropdown button').trigger('click')
    await flushPromises()
    const autoButton = Array.from(document.body.querySelectorAll('button')).find((button) =>
      button.textContent?.includes('keys.openaiAutoCheapest.label'),
    )
    autoButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    const maxRateInput = wrapper.get('[data-test="row-auto-cheapest-max-rate"]')
    await maxRateInput.setValue('')
    await wrapper.get('[data-test="row-auto-cheapest-submit"]').trigger('click')
    await flushPromises()

    expect(updateKey).toHaveBeenCalledWith(12, {
      group_id: null,
      group_select_mode: 'openai_auto_cheapest',
      openai_auto_group_max_rate_multiplier: 0.2,
    })

    wrapper.unmount()
  })

  it('shows last effective group name from last_effective_group_id', async () => {
    listKeys.mockResolvedValue({
      items: [
        {
          id: 8,
          key: 'sk-auto',
          name: 'auto-key',
          group_id: null,
          group_select_mode: 'openai_auto_cheapest',
          group: null,
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          quota: 0,
          quota_used: 0,
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          last_used_at: null,
          expires_at: null,
          last_effective_group_id: 2,
          last_effective_group_at: '2026-06-30T01:00:00Z',
          last_effective_group: null,
          openai_auto_group_max_rate_multiplier: 0.8,
        },
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })

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

    expect(wrapper.text()).toContain('keys.openaiAutoCheapest.currentEffective')
    expect(wrapper.text()).toContain('OpenAI Cheap')
    expect(wrapper.text()).toContain('keys.openaiAutoCheapest.maxRateCurrent')
    expect(wrapper.text()).not.toContain('keys.openaiAutoCheapest.waitingFirstUse')
  })

  it('allows editing max rate for an existing auto cheapest key from the row chip', async () => {
    listKeys.mockResolvedValue({
      items: [
        {
          id: 11,
          key: 'sk-auto-edit',
          name: 'auto-edit',
          group_id: null,
          group_select_mode: 'openai_auto_cheapest',
          group: null,
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          quota: 0,
          quota_used: 0,
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          last_used_at: null,
          expires_at: null,
          last_effective_group_id: 2,
          last_effective_group_at: '2026-06-30T01:00:00Z',
          last_effective_group: null,
          openai_auto_group_max_rate_multiplier: 0.8,
        },
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })

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

    await wrapper.get('[data-test="row-auto-cheapest-max-rate-chip"]').trigger('click')
    const maxRateInput = wrapper.get('[data-test="row-auto-cheapest-max-rate"]')
    expect((maxRateInput.element as HTMLInputElement).value).toBe('0.8')
    await maxRateInput.setValue('0.3')
    await wrapper.get('[data-test="row-auto-cheapest-submit"]').trigger('click')
    await flushPromises()

    expect(updateKey).toHaveBeenCalledWith(11, {
      group_id: null,
      group_select_mode: 'openai_auto_cheapest',
      openai_auto_group_max_rate_multiplier: 0.3,
    })
  })

  it('shows waiting text when auto cheapest key has no last effective group id', async () => {
    listKeys.mockResolvedValue({
      items: [
        {
          id: 9,
          key: 'sk-auto-waiting',
          name: 'auto-key-waiting',
          group_id: null,
          group_select_mode: 'openai_auto_cheapest',
          group: null,
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          quota: 0,
          quota_used: 0,
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          created_at: '2026-06-30T00:00:00Z',
          updated_at: '2026-06-30T00:00:00Z',
          last_used_at: null,
          expires_at: null,
          last_effective_group_id: null,
          last_effective_group_at: null,
          last_effective_group: null,
        },
      ],
      total: 1,
      page: 1,
      page_size: 10,
      pages: 1,
    })

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

    expect(wrapper.text()).toContain('keys.openaiAutoCheapest.waitingFirstUse')
  })
})
