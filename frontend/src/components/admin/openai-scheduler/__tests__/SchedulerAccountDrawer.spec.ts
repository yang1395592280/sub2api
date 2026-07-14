import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerAccountDrawer from '../SchedulerAccountDrawer.vue'
import { createSchedulerTestI18n } from './testI18n'

const account = {
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
  decision: 'context_required',
  decision_reason: 'request_context_required',
  scheduler_mode: 'balanced',
  shadow_mode: true,
  sticky_escape_reason: null,
  snapshot_age_ms: 1200,
  cooldown_until: null,
}

describe('SchedulerAccountDrawer', () => {
  it('shows real and probe health evidence separately', () => {
    const wrapper = mount(SchedulerAccountDrawer, {
      props: { open: true, account, events: [] },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Icon: { template: '<span />' }, Teleport: true } },
    })

    expect(wrapper.get('[data-testid="scheduler-account-drawer"]').text()).toContain('真实请求样本')
    expect(wrapper.text()).toContain('探测样本')
    expect(wrapper.text()).toContain('429 比例')
    expect(wrapper.text()).toContain('3 / 10')
    expect(wrapper.text()).toContain('影子观察')
  })

  it('emits close and account operations', async () => {
    const wrapper = mount(SchedulerAccountDrawer, {
      props: { open: true, account, events: [] },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Icon: { template: '<span />' }, Teleport: true } },
    })

    await wrapper.get('[data-testid="drawer-close"]').trigger('click')
    await wrapper.get('[data-testid="drawer-probe"]').trigger('click')
    await wrapper.get('[data-testid="drawer-reset"]').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(wrapper.emitted('probe')).toEqual([[account]])
    expect(wrapper.emitted('reset')).toEqual([[account]])
  })
})
