import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'

const {
  createGroup,
  updateGroup,
  listGroups,
  getAllGroups,
  getAllGroupsIncludingInactive,
  getModelsListCandidates,
  getUsageSummary,
  getCapacitySummary,
  getGroupCapacityUsers,
  listAccounts,
  showError,
  showSuccess,
  isCurrentStep,
  nextStep
} = vi.hoisted(() => ({
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  listGroups: vi.fn(),
  getAllGroups: vi.fn(),
  getAllGroupsIncludingInactive: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getGroupCapacityUsers: vi.fn(),
  listAccounts: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn()
}))

const messages: Record<string, string> = {
  'admin.groups.createGroup': '创建分组',
  'admin.groups.form.name': '名称',
  'admin.groups.form.description': '描述',
  'admin.groups.form.platform': '平台',
  'admin.groups.form.rateMultiplier': '倍率',
  'admin.groups.form.rpmLimit': 'RPM 限制',
  'admin.groups.form.upstreamBalanceRefreshEnabled': '启用上游余额自动刷新',
  'admin.groups.form.upstreamBalanceRefreshIntervalSeconds': '刷新间隔（秒）',
  'admin.groups.form.upstreamPriceMaxMultiplier': '价格倍率上限',
  'admin.groups.form.upstreamPriceGroupingEnabled': '按渠道价格自动归组',
  'admin.groups.form.upstreamPriceGroupingMin': '最低价格（含）',
  'admin.groups.form.upstreamPriceGroupingMax': '最高价格（含）',
  'admin.groups.nameRequired': '请输入分组名称',
  'admin.groups.groupCreated': '分组创建成功',
  'admin.groups.groupUpdated': '分组更新成功',
  'admin.groups.validation.upstreamBalanceRefreshIntervalMin': '上游余额自动刷新间隔不能小于 60 秒',
  'admin.groups.validation.upstreamPriceMaxMultiplierMin': '价格倍率上限不能小于 0',
  'common.edit': '编辑',
  'common.update': '更新',
  'common.create': '创建'
}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      create: createGroup,
      list: listGroups,
      getAll: getAllGroups,
      getAllIncludingInactive: getAllGroupsIncludingInactive,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary,
      getGroupCapacityUsers,
      getLiveCapability: vi.fn().mockResolvedValue({ supported: true }),
      update: updateGroup,
      delete: vi.fn(),
      updateSortOrder: vi.fn()
    },
    accounts: {
      list: listAccounts
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key
    })
  }
})

const createAdminGroup = (overrides: Partial<AdminGroup> = {}): AdminGroup => ({
  id: 1,
  name: 'Core Anthropic',
  description: null,
  platform: 'anthropic',
  group_role: 'standard',
  self_hosted_pool_group_id: null,
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  upstream_balance_refresh_enabled: false,
  upstream_balance_refresh_interval_seconds: 600,
  upstream_price_max_multiplier: 0,
  upstream_price_grouping_enabled: false,
  upstream_price_grouping_min: 0,
  upstream_price_grouping_max: 0,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  model_routing: null,
  model_routing_enabled: false,
  models_list_config: undefined,
  account_count: 0,
  active_account_count: 0,
  rate_limited_account_count: 0,
  sort_order: 10,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
  ...overrides
})

const AppLayoutStub = {
  template: '<div><slot /></div>'
}

const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
}

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="groups-table">
      <div v-for="row in data" :key="row.id">
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  methods: {
    handleChange(event: Event) {
      const value = (event.target as HTMLSelectElement).value
      const option = this.options.find((item: { value: unknown }) => String(item.value ?? '') === value)
      this.$emit('update:modelValue', option?.value ?? null)
      this.$emit('change')
    }
  },
  template: `
    <select
      :value="modelValue"
      @change="handleChange"
    >
      <option v-for="option in options" :key="String(option.value)" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
}

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>'
}

async function mountView() {
  const wrapper = mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupCapacityUsersModal: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: { template: '<div><slot /></div>' }
      }
    }
  })

  await flushPromises()
  return wrapper
}

describe('admin GroupsView upstream price guard settings', () => {
  beforeEach(() => {
    localStorage.clear()

    createGroup.mockReset()
    updateGroup.mockReset()
    listGroups.mockReset()
    getAllGroups.mockReset()
    getAllGroupsIncludingInactive.mockReset()
    getModelsListCandidates.mockReset()
    getUsageSummary.mockReset()
    getCapacitySummary.mockReset()
    getGroupCapacityUsers.mockReset()
    listAccounts.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    createGroup.mockResolvedValue(createAdminGroup())
    updateGroup.mockResolvedValue(createAdminGroup())
    listGroups.mockResolvedValue({
      items: [createAdminGroup()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getAllGroupsIncludingInactive.mockResolvedValue([])
    getModelsListCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getGroupCapacityUsers.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    isCurrentStep.mockReturnValue(false)
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('submits upstream refresh settings in create payload', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-tour="group-form-name"]').setValue('OpenAI 池')
    await wrapper.get('[data-test="group-upstream-refresh-enabled"]').setValue(true)
    await wrapper.get('[data-test="group-upstream-refresh-interval"]').setValue('600')
    await wrapper.get('[data-test="group-upstream-price-max-multiplier"]').setValue('0.08')

    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledTimes(1)
    expect(createGroup.mock.calls[0]?.[0]).toMatchObject({
      upstream_balance_refresh_enabled: true,
      upstream_balance_refresh_interval_seconds: 600,
      upstream_price_max_multiplier: 0.08
    })
  })

  it('submits inclusive channel price grouping range for OpenAI groups', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    await flushPromises()
    await wrapper.get('[data-tour="group-form-name"]').setValue('OpenAI 0.01-0.05')
    await wrapper.get('[data-test="group-upstream-refresh-enabled"]').setValue(true)
    await wrapper.get('[data-test="group-upstream-price-grouping-enabled"]').setValue(true)
    await wrapper.get('[data-test="group-upstream-price-grouping-min"]').setValue('0.01')
    await wrapper.get('[data-test="group-upstream-price-grouping-max"]').setValue('0.05')

    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({
        upstream_price_grouping_enabled: true,
        upstream_price_grouping_min: 0.01,
        upstream_price_grouping_max: 0.05
      })
    )
  })

  it('creates a self-hosted pool without showing standard group settings', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    await flushPromises()
    await wrapper.get('[data-test="create-group-role"]').setValue('self_hosted_pool')
    await flushPromises()

    expect(wrapper.find('[data-test="group-upstream-refresh-enabled"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="group-upstream-price-grouping-enabled"]').exists()).toBe(false)
    await wrapper.get('[data-tour="group-form-name"]').setValue('共享自建号池')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: 'openai',
        group_role: 'self_hosted_pool',
        self_hosted_pool_group_id: null
      })
    )
  })

  it('allows a standard group to bind an inactive shared pool', async () => {
    const pool = createAdminGroup({
      id: 99,
      name: '共享号池',
      platform: 'openai',
      group_role: 'self_hosted_pool',
      status: 'inactive'
    })
    getAllGroupsIncludingInactive.mockResolvedValue([pool])
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    await flushPromises()

    const poolSelect = wrapper.get('[data-test="create-self-hosted-pool"]')
    expect(poolSelect.text()).toContain('共享号池')
    await poolSelect.setValue('99')
    await wrapper.get('[data-tour="group-form-name"]').setValue('0.15 分组')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledWith(
      expect.objectContaining({
        group_role: 'standard',
        self_hosted_pool_group_id: 99
      })
    )
  })

  it('hydrates and submits upstream settings in edit payload', async () => {
    const existingGroup = createAdminGroup({
      platform: 'openai',
      upstream_balance_refresh_enabled: true,
      upstream_balance_refresh_interval_seconds: 900,
      upstream_price_max_multiplier: 0.12,
      upstream_price_grouping_enabled: true,
      upstream_price_grouping_min: 0.06,
      upstream_price_grouping_max: 0.08
    })
    listGroups.mockResolvedValueOnce({
      items: [existingGroup],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = await mountView()

    await wrapper.get('[data-test="group-edit-button"]').trigger('click')
    await flushPromises()

    expect((wrapper.get('[data-test="edit-group-upstream-refresh-enabled"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-test="edit-group-upstream-refresh-interval"]').element as HTMLInputElement).value).toBe('900')
    expect((wrapper.get('[data-test="edit-group-upstream-price-max-multiplier"]').element as HTMLInputElement).value).toBe('0.12')
    expect((wrapper.get('[data-test="edit-group-upstream-price-grouping-enabled"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-test="edit-group-upstream-price-grouping-min"]').element as HTMLInputElement).value).toBe('0.06')
    expect((wrapper.get('[data-test="edit-group-upstream-price-grouping-max"]').element as HTMLInputElement).value).toBe('0.08')

    await wrapper.get('[data-test="edit-group-upstream-refresh-interval"]').setValue('120')
    await wrapper.get('[data-test="edit-group-upstream-price-max-multiplier"]').setValue('0.05')
    await wrapper.get('[data-test="edit-group-upstream-price-grouping-min"]').setValue('0.01')
    await wrapper.get('[data-test="edit-group-upstream-price-grouping-max"]').setValue('0.05')

    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(updateGroup).toHaveBeenCalledWith(
      existingGroup.id,
      expect.objectContaining({
        upstream_balance_refresh_enabled: true,
        upstream_balance_refresh_interval_seconds: 120,
        upstream_price_max_multiplier: 0.05,
        upstream_price_grouping_enabled: true,
        upstream_price_grouping_min: 0.01,
        upstream_price_grouping_max: 0.05
      })
    )
  })

  it('submits disabled auto cheapest scheduling from the OpenAI group editor', async () => {
    const existingGroup = createAdminGroup({
      platform: 'openai',
      allow_auto_cheapest_scheduling: true
    })
    listGroups.mockResolvedValueOnce({
      items: [existingGroup],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = await mountView()

    await wrapper.get('[data-test="group-edit-button"]').trigger('click')
    await flushPromises()

    const autoCheapestSwitch = wrapper.get('[data-test="edit-group-allow-auto-cheapest-scheduling"]')
    expect(autoCheapestSwitch.attributes('aria-checked')).toBe('true')
    await autoCheapestSwitch.trigger('click')
    expect(autoCheapestSwitch.attributes('aria-checked')).toBe('false')

    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledWith(
      existingGroup.id,
      expect.objectContaining({ allow_auto_cheapest_scheduling: false })
    )
  })

  it('blocks create when upstream refresh interval is below 60 seconds', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-tour="group-form-name"]').setValue('OpenAI 池')
    await wrapper.get('[data-test="group-upstream-refresh-enabled"]').setValue(true)
    await wrapper.get('[data-test="group-upstream-refresh-interval"]').setValue('59')

    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('上游余额自动刷新间隔不能小于 60 秒')
  })

  it('blocks create when upstream price max multiplier is negative', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-tour="group-form-name"]').setValue('OpenAI 池')
    await wrapper.get('[data-test="group-upstream-price-max-multiplier"]').setValue('-0.01')

    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('价格倍率上限不能小于 0')
  })
})
