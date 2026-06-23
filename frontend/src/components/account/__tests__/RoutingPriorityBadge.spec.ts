import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import RoutingPriorityBadge from '../RoutingPriorityBadge.vue'

const messages: Record<string, string> = {
  'admin.accounts.routingPriority.status.candidate': '候选',
  'admin.accounts.routingPriority.status.skipped': '已跳过',
  'admin.accounts.routingPriority.summary.cost_advantage': '成本优先',
  'admin.accounts.routingPriority.summary.low_load': '低负载',
  'admin.accounts.routingPriority.reasons.rate_limited': '速率限制',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
      te: (key: string) => key in messages,
    }),
  }
})

describe('RoutingPriorityBadge', () => {
  it('renders localized rank badge for schedulable account', () => {
    const wrapper = mount(RoutingPriorityBadge, {
      props: {
        summary: {
          account_id: 1,
          account_name: 'cheap-fast',
          rank: 3,
          tier: 'primary',
          status_label: 'candidate',
          summary_reason: 'cost_advantage',
          summary_reasons: ['cost_advantage', 'low_load'],
          is_schedulable_now: true,
          score: { total: 3.2, priority: 1, load: 0.8, queue: 1, error_rate: 1, ttft: 0.9, price: 1, health: 1 },
          snapshot_at: '2026-06-23T00:00:00Z',
        },
      },
    })

    expect(wrapper.text()).toContain('#3')
    expect(wrapper.text()).toContain('候选')
    expect(wrapper.text()).toContain('成本优先')
    expect(wrapper.text()).not.toContain('candidate')
    expect(wrapper.text()).not.toContain('cost_advantage')
  })

  it('renders localized skipped state and emits open when clicked', async () => {
    const wrapper = mount(RoutingPriorityBadge, {
      props: {
        summary: {
          account_id: 2,
          account_name: 'blocked',
          tier: 'degraded',
          status_label: 'skipped',
          summary_reason: 'rate_limited',
          summary_reasons: ['rate_limited'],
          is_schedulable_now: false,
          block_reasons: ['rate_limited'],
          score: { total: 0, priority: 0, load: 0, queue: 0, error_rate: 0, ttft: 0, price: 0, health: 0 },
          snapshot_at: '2026-06-23T00:00:00Z',
        },
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('已跳过')
    expect(wrapper.text()).toContain('速率限制')
    expect(wrapper.text()).not.toContain('skipped')
    expect(wrapper.text()).not.toContain('rate_limited')
    expect(wrapper.emitted('open')).toHaveLength(1)
  })
})
