import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerSettingsPanel from '../SchedulerSettingsPanel.vue'
import { createSchedulerTestI18n } from './testI18n'

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

describe('SchedulerSettingsPanel', () => {
  it('emits the balanced settings as numeric values', async () => {
    const wrapper = mount(SchedulerSettingsPanel, {
      props: { modelValue: settings, saving: false },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Toggle: true } },
    })

    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      mode: 'balanced',
      top_k: 3,
      exploration_rate: 0.03,
      session_escape_min_gap_ms: 1000,
      session_escape_ratio: 0.25,
      health_ttl_seconds: 1800,
      probe_jitter_seconds: 6,
    })
  })

  it('blocks invalid threshold ordering', async () => {
    const wrapper = mount(SchedulerSettingsPanel, {
      props: { modelValue: settings, saving: false },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Toggle: true } },
    })
    await wrapper.get<HTMLInputElement>('#scheduler-severe-threshold').setValue('5000')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.text()).toContain('严重慢响应阈值不能低于慢响应阈值')
    expect(wrapper.emitted('save')).toBeUndefined()
  })

  it('blocks values outside the backend scheduler ranges', async () => {
    const wrapper = mount(SchedulerSettingsPanel, {
      props: { modelValue: settings, saving: false },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Toggle: true } },
    })
    await wrapper.get<HTMLInputElement>('#scheduler-session-gap').setValue('30001')
    await wrapper.get('form').trigger('submit')

    expect(wrapper.text()).toContain('会话逃逸差值必须在 0 到 30000 ms 之间')
    expect(wrapper.emitted('save')).toBeUndefined()
  })
})
