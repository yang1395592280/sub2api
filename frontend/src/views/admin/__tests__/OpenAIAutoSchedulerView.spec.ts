import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIAutoSchedulerView from '../OpenAIAutoSchedulerView.vue'
import { createSchedulerTestI18n } from '@/components/admin/openai-scheduler/__tests__/testI18n'

const {
  getSettings,
  updateSettings,
  listGroups,
  updateGroup,
  getOverview,
  listHealth,
  listEvents,
  resetScore,
  probeScore,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listGroups: vi.fn(),
  updateGroup: vi.fn(),
  getOverview: vi.fn(),
  listHealth: vi.fn(),
  listEvents: vi.fn(),
  resetScore: vi.fn(),
  probeScore: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    openaiAutoScheduler: {
      getSettings,
      updateSettings,
      listGroups,
      updateGroup,
      getOverview,
      listHealth,
      listEvents,
      resetScore,
      probeScore,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

const settings = {
  enabled: true,
  mode: 'balanced' as const,
  shadow_mode: true,
  top_k: 3,
  exploration_rate: 0.03,
  session_escape_min_gap_ms: 1000,
  session_escape_ratio: 0.25,
  health_ttl_seconds: 1800,
  real_sample_fresh_seconds: 300,
  probe_jitter_seconds: 6,
  probe_model: 'gpt-5.4',
  probe_interval_seconds: 60,
  slow_threshold_ms: 10000,
  severe_slow_threshold_ms: 20000,
  consecutive_slow_breaker_threshold: 3,
  consecutive_error_breaker_threshold: 2,
  cooldown_seconds: 120,
  half_open_success_threshold: 3,
  cost_weight: 0.2,
  recovery_step: 800,
}

const groups = [
  { id: 10, name: 'disabled', status: 'active', enabled: false },
  { id: 33, name: 'Codex', status: 'active', enabled: true },
  { id: 82, name: 'Control', status: 'active', enabled: true },
]

const healthRow = {
  account_id: 12512,
  account_name: 'main-account',
  group_id: 33,
  model_family: 'gpt-5.4',
  endpoint: 'responses',
  transport: 'http_sse',
  state: 'running',
  predicted_ttft_ms: 920,
  real_sample_count: 21,
  probe_sample_count: 4,
  error_rate: 0.01,
  rate_limited_rate: 0,
  server_error_rate: 0,
  load_inflight: 2,
  load_capacity: 10,
  waiting_count: 0,
  channel_price: 0.25,
  decision: 'context_required',
  decision_reason: 'request_context_required',
  scheduler_mode: 'balanced',
  shadow_mode: true,
  sticky_escape_reason: null,
  snapshot_age_ms: 500,
  cooldown_until: null,
}

const AppLayoutStub = { template: '<div><slot /></div>' }
const IconStub = { template: '<span />' }
const ToggleStub = defineComponent({
  props: ['modelValue'],
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('button', { type: 'button', 'data-testid': 'global-toggle', onClick: () => emit('update:modelValue', !props.modelValue) })
  },
})
const GroupListStub = defineComponent({
  props: ['groups', 'modelValue'],
  emits: ['update:modelValue', 'toggle'],
  template: '<div><button data-testid="select-82" @click="$emit(\'update:modelValue\', 82)">Control</button><button data-testid="toggle-82" @click="$emit(\'toggle\', 82, false)">toggle</button></div>',
})
const OverviewStub = defineComponent({ props: ['overview'], template: '<div data-testid="overview">{{ overview?.e2e_ttft_p50_ms }}</div>' })
const HealthStub = defineComponent({
  props: ['rows'],
  emits: ['select', 'probe', 'reset', 'filter', 'page'],
  template: '<div data-testid="health"><span>{{ rows.length }}</span><button data-testid="select-health" @click="$emit(\'select\', rows[0])">select</button><button data-testid="reset-health" @click="$emit(\'reset\', rows[0])">reset</button></div>',
})
const DrawerStub = defineComponent({ props: ['open', 'account'], template: '<div v-if="open" data-testid="drawer">{{ account?.account_name }}</div>' })
const EventsStub = { template: '<div data-testid="events" />' }
const SettingsStub = { template: '<div data-testid="settings" />' }
const ConfirmStub = defineComponent({
  props: ['show'],
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" data-testid="confirm-reset" @click="$emit(\'confirm\')">confirm</button>',
})

function mountView() {
  return mount(OpenAIAutoSchedulerView, {
    global: {
      plugins: [createSchedulerTestI18n()],
      stubs: {
        AppLayout: AppLayoutStub,
        Icon: IconStub,
        Toggle: ToggleStub,
        SchedulerGroupList: GroupListStub,
        SchedulerOverview: OverviewStub,
        SchedulerHealthTable: HealthStub,
        SchedulerAccountDrawer: DrawerStub,
        SchedulerEventsPanel: EventsStub,
        SchedulerSettingsPanel: SettingsStub,
        ConfirmDialog: ConfirmStub,
      },
    },
  })
}

describe('OpenAIAutoSchedulerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({ ...settings })
    updateSettings.mockImplementation((payload) => Promise.resolve({ ...payload }))
    listGroups.mockResolvedValue(groups.map((group) => ({ ...group })))
    updateGroup.mockImplementation((id, payload) => Promise.resolve({ ...groups.find((group) => group.id === id), ...payload }))
    getOverview.mockResolvedValue({
      e2e_ttft_p50_ms: 970,
      e2e_ttft_p90_ms: 2100,
      selection_p95_ms: 18,
      probe_ratio: 0.2,
      groups: [],
      trend: [],
      slow_causes: [],
    })
    listHealth.mockResolvedValue({ items: [healthRow], total: 1, page: 1, page_size: 20, pages: 1 })
    listEvents.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    resetScore.mockResolvedValue({ message: 'reset' })
    probeScore.mockResolvedValue({ success: true, event_type: 'probe_success', message: 'ok', latency_ms: 800, ttfb_ms: 600 })
  })

  it('initializes the B console on the first enabled group', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('OpenAI 调度控制台')
    expect(wrapper.text()).toContain('均衡模式')
    expect(wrapper.text()).toContain('影子观察')
    expect(wrapper.findAll('[data-testid^="scheduler-tab-"]')).toHaveLength(4)
    expect(getOverview).toHaveBeenCalledWith(
      { group_id: 33, window: '6h' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  it('loads account health when switching tabs', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="scheduler-tab-health"]').trigger('click')
    await flushPromises()

    expect(listHealth).toHaveBeenCalledWith(
      expect.objectContaining({ group_id: 33, page: 1, page_size: 20 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    expect(wrapper.get('[data-testid="health"]').text()).toContain('1')
  })

  it('reloads the active overview when selecting another group', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="select-82"]').trigger('click')
    await flushPromises()

    expect(getOverview).toHaveBeenLastCalledWith(
      { group_id: 82, window: '6h' },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  it('updates the global switch through the settings contract', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="global-toggle"]').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({ enabled: false, mode: 'balanced' }))
    expect(showSuccess).toHaveBeenCalled()
  })

  it('opens health detail and confirms reset using row identity', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="scheduler-tab-health"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="select-health"]').trigger('click')
    expect(wrapper.get('[data-testid="drawer"]').text()).toContain('main-account')
    await wrapper.get('[data-testid="reset-health"]').trigger('click')
    await wrapper.get('[data-testid="confirm-reset"]').trigger('click')
    await flushPromises()

    expect(resetScore).toHaveBeenCalledWith(12512, { group_id: 33, model: 'gpt-5.4' })
  })
})
