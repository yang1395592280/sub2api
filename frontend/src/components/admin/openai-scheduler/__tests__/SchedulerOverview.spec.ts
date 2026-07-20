import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerOverview from '../SchedulerOverview.vue'
import { createSchedulerTestI18n } from './testI18n'

vi.mock('../SchedulerTTFTChart.vue', () => ({
  default: { props: ['points'], template: '<div data-testid="ttft-chart">{{ points.length }}</div>' },
}))

const overview = {
  e2e_ttft_p50_ms: 2970,
  e2e_ttft_p90_ms: 7210,
  selection_p95_ms: 18,
  probe_ratio: 0.24,
  groups: [
    { id: 33, name: 'Codex', enabled: true, account_count: 6, e2e_ttft_p90_ms: 7210, alert_level: 'warning' as const },
  ],
  trend: [{ bucket: '2026-07-14T10:00:00Z', e2e_ttft_p50_ms: 2970, e2e_ttft_p90_ms: 7210 }],
  slow_causes: [{ reason: 'upstream_ttft' as const, count: 12, ratio: 0.6 }],
  runtime: {
    exploration_allowed_total: 8,
    exploration_rejected_total: 0,
    exploration_interval_total: 3,
    exploration_hourly_total: 2,
    exploration_error_total: 1,
    low_confidence_fallback_total: 4,
    unified_health_reads_total: 10,
    unified_health_dimensions_total: 40,
    unified_health_fallbacks_total: 2,
  },
}

describe('SchedulerOverview', () => {
  it('shows the four scheduler metrics and slow cause', () => {
    const wrapper = mount(SchedulerOverview, {
      props: { overview, loading: false, selectedGroupId: 33 }, global: { plugins: [createSchedulerTestI18n()] },
    })

    expect(wrapper.findAll('[data-testid^="scheduler-metric-"]')).toHaveLength(4)
    expect(wrapper.text()).toContain('2.97s')
    expect(wrapper.text()).toContain('7.21s')
    expect(wrapper.text()).toContain('18ms')
    expect(wrapper.text()).toContain('24%')
    expect(wrapper.text()).toContain('上游首字延迟')
    expect(wrapper.text()).toContain('当前实例自启动累计')
    expect(wrapper.text()).toContain('探索放行')
    expect(wrapper.text()).toContain('低置信降级命中')
    expect(wrapper.get('[data-testid="ttft-chart"]').text()).toBe('1')
  })

  it('uses stable placeholders when metrics are unavailable', () => {
    const wrapper = mount(SchedulerOverview, {
      props: {
        overview: { ...overview, e2e_ttft_p50_ms: null, e2e_ttft_p90_ms: null, selection_p95_ms: null },
        loading: false,
        selectedGroupId: 33,
      }, global: { plugins: [createSchedulerTestI18n()] },
    })
    expect(wrapper.text()).toContain('—')
  })
})
