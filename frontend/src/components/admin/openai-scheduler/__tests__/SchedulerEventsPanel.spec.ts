import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerEventsPanel from '../SchedulerEventsPanel.vue'
import { createSchedulerTestI18n } from './testI18n'

const events = [
  {
    account_id: 12487,
    group_id: 33,
    model: 'gpt-5.4',
    event_type: 'slow' as const,
    score_before: 8200,
    score_before_percent: 82,
    score_after: 7600,
    score_after_percent: 76,
    latency_ms: 3200,
    ttfb_ms: 2100,
    status_code: 200,
    message: 'upstream first token exceeded target',
    created_at: '2026-07-14T10:00:00Z',
  },
]

describe('SchedulerEventsPanel', () => {
  it('renders the real event contract and raw detail', () => {
    const wrapper = mount(SchedulerEventsPanel, {
      props: { events, loading: false, total: 1, page: 1, pageSize: 20 },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Pagination: true } },
    })

    expect(wrapper.text()).toContain('#12487')
    expect(wrapper.text()).toContain('慢响应')
    expect(wrapper.text()).toContain('TTFB 2.10s')
    expect(wrapper.text()).toContain('0.8200 → 0.7600')
    expect(wrapper.text()).toContain('upstream first token exceeded target')
  })
})
