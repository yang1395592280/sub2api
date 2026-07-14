import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerHealthTable from '../SchedulerHealthTable.vue'
import { createSchedulerTestI18n } from './testI18n'

export const healthRows = [
  {
    account_id: 12512,
    account_name: 'main-account',
    group_id: 33,
    model_family: 'gpt-5.4',
    endpoint: 'responses',
    transport: 'http_sse',
    state: 'half_open',
    predicted_ttft_ms: 10940,
    real_sample_count: 21,
    probe_sample_count: 4,
    error_rate: 0.08,
    rate_limited_rate: 0.02,
    server_error_rate: 0.01,
    load_inflight: 3,
    load_capacity: 10,
    waiting_count: 2,
    channel_price: 0.25,
    decision: 'circuit_rejected',
    decision_reason: 'half_open',
    scheduler_mode: 'balanced',
    shadow_mode: true,
    sticky_escape_reason: null,
    snapshot_age_ms: 1200,
    cooldown_until: null,
  },
]

describe('SchedulerHealthTable', () => {
  it('renders human health and decision labels without exposing raw states', () => {
    const wrapper = mount(SchedulerHealthTable, {
      props: { rows: healthRows, loading: false, total: 1, page: 1, pageSize: 20 },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Icon: { template: '<span />' }, Pagination: true } },
    })

    expect(wrapper.text()).toContain('#12512')
    expect(wrapper.text()).toContain('10.94s')
    expect(wrapper.text()).toContain('恢复验证')
    expect(wrapper.text()).toContain('熔断排除')
    expect(wrapper.text()).not.toContain('half_open')
    expect(wrapper.text()).not.toContain('context_required')
  })

  it('emits one row command per action', async () => {
    const wrapper = mount(SchedulerHealthTable, {
      props: { rows: healthRows, loading: false, total: 1, page: 1, pageSize: 20 },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Icon: { template: '<span />' }, Pagination: true } },
    })
    await wrapper.get('[data-testid="health-row-12512"]').trigger('click')
    await wrapper.get('[data-testid="health-probe-12512"]').trigger('click')
    await wrapper.get('[data-testid="health-reset-12512"]').trigger('click')

    expect(wrapper.emitted('select')).toEqual([[healthRows[0]]])
    expect(wrapper.emitted('probe')).toEqual([[healthRows[0]]])
    expect(wrapper.emitted('reset')).toEqual([[healthRows[0]]])
  })
})
