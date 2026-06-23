import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OpenAIRoutingExplainModal from '../OpenAIRoutingExplainModal.vue'

const messages: Record<string, string> = {
  'admin.accounts.routingPriority.modal.title': '路由优先级详情',
  'admin.accounts.routingPriority.status.candidate': '候选',
  'admin.accounts.routingPriority.summary.high_priority': '人工优先级高',
  'admin.accounts.routingPriority.summary.low_load': '低负载',
  'admin.accounts.routingPriority.reasons.temp_unschedulable': '临时不可调度',
  'admin.accounts.routingPriority.blockSources.ui_countdown_state': '前端倒计时状态',
  'admin.accounts.routingPriority.notes.sticky_may_override_ranking': '会话保持命中时，实际路由可能覆盖当前排名。',
  'admin.accounts.routingPriority.notes.weighted_top_k_not_strict_best': '加权 Top-K 模式下，排名第一不代表每次都会被选中。',
  'admin.accounts.routingPriority.score.total': '总分',
  'admin.accounts.routingPriority.score.priority': '优先级',
  'admin.accounts.routingPriority.score.load': '负载',
  'admin.accounts.routingPriority.score.queue': '排队',
  'admin.accounts.routingPriority.score.error_rate': '错误率',
  'admin.accounts.routingPriority.score.ttft': '首包延迟',
  'admin.accounts.routingPriority.score.price': '价格',
  'admin.accounts.routingPriority.score.health': '健康度',
  'admin.accounts.routingPriority.sections.summary': '当前账号',
  'admin.accounts.routingPriority.sections.score': '分数拆解',
  'admin.accounts.routingPriority.sections.blockReasons': '阻塞原因',
  'admin.accounts.routingPriority.sections.topCandidates': 'Top 候选',
  'admin.accounts.routingPriority.sections.notes': '说明',
  'common.loading': '加载中...',
  'common.close': '关闭',
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

describe('OpenAIRoutingExplainModal', () => {
  it('renders localized score breakdown, block details and notes', () => {
    const wrapper = mount(OpenAIRoutingExplainModal, {
      props: {
        show: true,
        accountId: 1,
        loading: false,
        explain: {
          account: {
            account_id: 1,
            account_name: 'cheap-fast',
            rank: 1,
            tier: 'primary',
            status_label: 'candidate',
            summary_reason: 'high_priority',
            summary_reasons: ['high_priority'],
            is_schedulable_now: true,
            block_reasons: ['temp_unschedulable'],
            block_details: [
              {
                reason: 'temp_unschedulable',
                source: 'ui_countdown_state',
                until: '2026-06-23T01:00:00Z',
                snapshot_at: '2026-06-23T00:00:00Z',
              },
            ],
            score: { total: 3.2, priority: 1, load: 0.8, queue: 1, error_rate: 1, ttft: 0.9, price: 1, health: 1 },
            snapshot_at: '2026-06-23T00:00:00Z',
          },
          top: [
            {
              account_id: 2,
              account_name: 'steady-one',
              rank: 2,
              tier: 'observe',
              status_label: 'candidate',
              summary_reason: 'low_load',
              summary_reasons: ['low_load'],
              is_schedulable_now: true,
              score: { total: 2.4, priority: 0.9, load: 0.7, queue: 0.6, error_rate: 0.8, ttft: 0.7, price: 0.6, health: 0.8 },
              snapshot_at: '2026-06-23T00:00:00Z',
            },
          ],
          notes: ['sticky_may_override_ranking', 'weighted_top_k_not_strict_best'],
        },
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title', 'width'],
            template: '<div data-testid="base-dialog"><div>{{ title }}</div><slot /><slot name="footer" /></div>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('路由优先级详情')
    expect(wrapper.text()).toContain('cheap-fast')
    expect(wrapper.text()).toContain('人工优先级高')
    expect(wrapper.text()).toContain('价格')
    expect(wrapper.text()).toContain('临时不可调度')
    expect(wrapper.text()).toContain('前端倒计时状态')
    expect(wrapper.text()).toContain('steady-one')
    expect(wrapper.text()).toContain('低负载')
    expect(wrapper.text()).toContain('会话保持命中时，实际路由可能覆盖当前排名。')
    expect(wrapper.text()).toContain('加权 Top-K 模式下，排名第一不代表每次都会被选中。')
    expect(wrapper.text()).not.toContain('high_priority')
    expect(wrapper.text()).not.toContain('ui_countdown_state')
    expect(wrapper.text()).not.toContain('sticky_may_override_ranking')
  })
})
