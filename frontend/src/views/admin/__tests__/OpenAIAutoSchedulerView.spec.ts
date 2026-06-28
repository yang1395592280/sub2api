import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIAutoSchedulerView from '../OpenAIAutoSchedulerView.vue'

const {
  getSettings,
  updateSettings,
  listGroups,
  updateGroup,
  listScores,
  resetScore,
  probeScore,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listGroups: vi.fn(),
  updateGroup: vi.fn(),
  listScores: vi.fn(),
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
      listScores,
      resetScore,
      probeScore,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_err: unknown, fallback: string) => fallback,
}))

const settings = {
  enabled: true,
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
  { id: 20, name: 'plus特惠临时分组', status: 'active', enabled: true },
  { id: 21, name: 'openai-backup', status: 'active', enabled: false },
]

const scores = [
  {
    account_id: 101,
    account_name: 'plus特惠临时分组渠道',
    group_id: 20,
    model: 'gpt-5.4',
    base_score: 6000,
    base_score_percent: 60,
    final_score: 8200,
    final_score_percent: 82,
    latency_score: 7000,
    latency_score_percent: 70,
    error_score: 10000,
    error_score_percent: 100,
    recovery_score: 9000,
    recovery_score_percent: 90,
    cost_score: 8000,
    cost_score_percent: 80,
    state: 'observing',
    consecutive_slow_count: 1,
    consecutive_error_count: 0,
    consecutive_success_count: 2,
    request_count: 12,
    ttfb_sample_count: 7,
    slow_rate: 0.25,
    error_rate: 0.05,
    stuck_rate: 0,
    cooldown_until: null,
    last_latency_ms: 1200,
    last_ttfb_ms: 420,
    last_status_code: 200,
    last_error: null,
    reason: 'latency above target',
    last_checked_at: '2026-06-28T03:00:00Z',
  },
  {
    account_id: 102,
    account_name: 'codex-pro备用渠道',
    group_id: 21,
    model: 'gpt-5.5',
    base_score: 10000,
    base_score_percent: 100,
    final_score: 3200,
    final_score_percent: 32,
    latency_score: 3000,
    latency_score_percent: 30,
    error_score: 4000,
    error_score_percent: 40,
    recovery_score: 2000,
    recovery_score_percent: 20,
    cost_score: 8000,
    cost_score_percent: 80,
    state: 'open',
    consecutive_slow_count: 3,
    consecutive_error_count: 2,
    consecutive_success_count: 0,
    request_count: 20,
    ttfb_sample_count: 10,
    slow_rate: 0.5,
    error_rate: 0.35,
    stuck_rate: 0.1,
    cooldown_until: '2026-06-28T03:05:00Z',
    last_latency_ms: 22000,
    last_ttfb_ms: 1200,
    last_status_code: 500,
    last_error: 'context deadline exceeded',
    reason: 'breaker open',
    last_checked_at: '2026-06-28T03:01:00Z',
  },
]

const AppLayoutStub = { template: '<div><slot /></div>' }
const EmptyStateStub = defineComponent({
  props: ['title', 'description'],
  template: '<div>{{ title }} {{ description }}</div>',
})
const IconStub = defineComponent({
  props: ['name'],
  template: '<span>{{ name }}</span>',
})
const PaginationStub = defineComponent({
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: '<div data-testid="pagination">{{ page }} / {{ pageSize }} / {{ total }}</div>',
})
const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      required: true,
    },
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h(
        'button',
        {
          type: 'button',
          role: 'switch',
          'aria-checked': String(props.modelValue),
          onClick: () => emit('update:modelValue', !props.modelValue),
        },
        String(props.modelValue)
      )
  },
})

function mountView() {
  return mount(OpenAIAutoSchedulerView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        EmptyState: EmptyStateStub,
        Icon: IconStub,
        Pagination: PaginationStub,
        Toggle: ToggleStub,
      },
    },
  })
}

describe('OpenAIAutoSchedulerView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({ ...settings })
    updateSettings.mockResolvedValue({ ...settings, enabled: false })
    listGroups.mockResolvedValue(groups.map((group) => ({ ...group })))
    updateGroup.mockImplementation((id: number, payload: { enabled: boolean }) =>
      Promise.resolve({ ...groups.find((group) => group.id === id), ...payload })
    )
    listScores.mockResolvedValue({
      items: scores.map((score) => ({ ...score })),
      total: scores.length,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    resetScore.mockResolvedValue({ message: 'score reset' })
    probeScore.mockResolvedValue({
      event_type: 'probe_success',
      success: true,
      message: 'ok',
      latency_ms: 800,
    })
  })

  it('loads settings, groups and score rows', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSettings).toHaveBeenCalled()
    expect(listGroups).toHaveBeenCalled()
    expect(listScores).toHaveBeenCalledWith(
      expect.objectContaining({ page: 1, group_id: 20, model: 'gpt-5.4' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    expect(wrapper.text()).toContain('plus特惠临时分组')
    expect(wrapper.text()).toContain('plus特惠临时分组渠道')
    expect(wrapper.text()).toContain('observing')
    expect(wrapper.text()).toContain('0.8200')
    expect(wrapper.get<HTMLSelectElement>('#scheduler-filter-model').element.value).toBe('gpt-5.4')
    expect(wrapper.text()).toContain('实际调度分')
    expect(wrapper.text()).toContain('当前分数 0.8200（已含成本修正 +0.8000）；同状态选择时再叠加组内价格修正')
    expect(wrapper.text()).not.toContain('实际调度分 = 健康分 0.8200 + 价格修正 +0.8000')
    expect(wrapper.text()).toContain('基础分 0.6000')
    expect(wrapper.text()).toContain('新渠道默认起点')
    expect(wrapper.text()).toContain('延迟修正')
    expect(wrapper.text()).toContain('错误惩罚')
    expect(wrapper.text()).toContain('恢复加分')
    expect(wrapper.text()).toContain('成本修正')
    expect(wrapper.text()).toContain('请求样本')
    expect(wrapper.text()).toContain('TTFB样本')
    expect(wrapper.text()).toContain('超时：context deadline exceeded')
  })

  it('renders the approved operations layout with group sidebar and channel table', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="scheduler-group-sidebar"]').text()).toContain('plus特惠临时分组')
    expect(wrapper.get('[data-testid="scheduler-group-sidebar"]').text()).toContain('参与自动调度')
    expect(wrapper.get('[data-testid="scheduler-score-table"]').text()).toContain('上游渠道')
    expect(wrapper.get('[data-testid="scheduler-score-table"]').text()).toContain('实际调度分')
    expect(wrapper.get('[data-testid="scheduler-score-table"]').text()).toContain('健康分拆解')
    expect(wrapper.get('[data-testid="scheduler-score-table"]').text()).toContain('探测样本')
    expect(wrapper.get('[data-testid="scheduler-score-table"]').text()).toContain('最近风险')
    expect(wrapper.text()).toContain('关闭后走系统原调度')
    expect(wrapper.text()).toContain('当前分组关闭时只展示分数，不参与自动调度')
  })

  it('selects groups from the sidebar and keeps the filter in sync', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="scheduler-group-card-21"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('openai-backup')
    expect(wrapper.text()).toContain('不参与自动调度')
    expect(listScores).toHaveBeenLastCalledWith(
      expect.objectContaining({ group_id: 21, model: 'gpt-5.4' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  it('updates selected group participation and applies group filter', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="scheduler-group-card-21"]').trigger('click')
    await flushPromises()

    expect(listScores).toHaveBeenLastCalledWith(
      expect.objectContaining({ group_id: 21 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )

    await wrapper.get('[data-testid="scheduler-group-card-21"]').get('[role="switch"]').trigger('click')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledWith(21, { enabled: true })
    expect(showSuccess).toHaveBeenCalledWith('分组已加入自动调度')
  })

  it('uses row account, group and model identity for probe and reset actions', async () => {
    const wrapper = mountView()
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const probe = buttons.find((button) => button.text().includes('探测'))

    await probe!.trigger('click')
    await flushPromises()

    expect(probeScore).toHaveBeenCalledWith(101, { group_id: 20, model: 'gpt-5.4' })

    const resetAfterProbe = wrapper.findAll('button').find((button) => button.text().includes('重置'))
    await resetAfterProbe!.trigger('click')
    await flushPromises()

    expect(resetScore).toHaveBeenCalledWith(101, { group_id: 20, model: 'gpt-5.4' })
  })

  it('edits scheduler settings from the configuration panel', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('编辑调度配置'))!.trigger('click')
    await flushPromises()

    await wrapper.get<HTMLInputElement>('#scheduler-settings-probe-interval').setValue('90')
    await wrapper.get<HTMLInputElement>('#scheduler-settings-cost-weight').setValue('35')
    await wrapper.find('form[data-testid="scheduler-settings-form"]').trigger('submit')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        probe_interval_seconds: 90,
        cost_weight: 0.35,
      })
    )
    expect(showSuccess).toHaveBeenCalledWith('调度配置已更新')
  })

  it('locally filters before paginating and reports filtered total when state filter is active', async () => {
    const openScores = Array.from({ length: 22 }, (_, index) => ({
      ...scores[1],
      account_id: 200 + index,
      account_name: `open渠道-${index + 1}`,
      model: `gpt-5.5-open-${index + 1}`,
      state: 'open',
    }))
    const runningScores = Array.from({ length: 5 }, (_, index) => ({
      ...scores[0],
      account_id: 300 + index,
      account_name: `running渠道-${index + 1}`,
      model: `gpt-5.4-running-${index + 1}`,
      state: 'running',
    }))
    listScores.mockResolvedValueOnce({
      items: scores.map((score) => ({ ...score })),
      total: scores.length,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    listScores.mockResolvedValueOnce({
      items: [...openScores, ...runningScores],
      total: 27,
      page: 1,
      page_size: 200,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get<HTMLSelectElement>('#scheduler-filter-state').setValue('open')
    await flushPromises()

    expect(listScores).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1, page_size: 200 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    expect(wrapper.text()).toContain('open渠道-1')
    expect(wrapper.text()).toContain('open渠道-20')
    expect(wrapper.text()).not.toContain('open渠道-21')
    expect(wrapper.text()).not.toContain('gpt-5.4-running')
    expect(wrapper.get('[data-testid="pagination"]').text()).toBe('1 / 20 / 22')
  })

  it('selecting the score group filter shows that group participation switch state', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get<HTMLSelectElement>('#scheduler-filter-group').setValue('21')
    await flushPromises()

    expect(wrapper.text()).toContain('openai-backup')
    expect(wrapper.text()).toContain('不参与自动调度')
    expect(wrapper.get('[data-testid="scheduler-group-card-21"]').get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(listScores).toHaveBeenLastCalledWith(
      expect.objectContaining({ group_id: 21 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })
})
