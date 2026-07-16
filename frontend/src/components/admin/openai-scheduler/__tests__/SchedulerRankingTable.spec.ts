import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SchedulerRankingTable from '../SchedulerRankingTable.vue'
import { createSchedulerTestI18n } from './testI18n'

const item = {
  partition: { group_id: 33, model_family: 'gpt-5.4', endpoint: 'responses', transport: 'http_sse' },
  rank: 1, account_id: 10, account_name: 'fast-account', eligibility: 'eligible' as const, eligibility_reason: '',
  utility_score: 91.2, target_share: 0.64, actual_share: 0.58, selected_requests: 58,
  predicted_ttft_ms: 820, ttft_p50_ms: 900, ttft_p90_ms: 1700,
  error_rate: 0.01, rate_limited_rate: 0.002, server_error_rate: 0.001,
  load_inflight: 2, load_capacity: 10, waiting_count: 0, channel_price: 0.7, estimated_cost: 40.6,
  confidence: 'high', real_sample_count: 50, probe_sample_count: 2, snapshot_age_ms: 500,
  latency_score: 1, reliability_score: 0.99, cost_score: 0.5, capacity_score: 0.8, quota_score: 0.5, priority_score: 1,
  deviation_reasons: [], decision_summary: 'highest_utility',
}

describe('SchedulerRankingTable', () => {
  it('shows effective policy and target versus actual traffic', async () => {
    const wrapper = mount(SchedulerRankingTable, {
      props: {
        groupName: 'Codex', loading: false, filters: { window: '1h', page: 1, page_size: 20 },
        result: {
          policy_context: { engine_enabled: true, global_enabled: true, group_enabled: true, configured_mode: 'balanced', effective_mode: 'balanced', shadow_mode: false, policy_version: 'v2', calculated_at: '2026-07-15T12:00:00Z' },
          summary: { candidate_count: 1, eligible_count: 1, rejected_count: 0, low_confidence_count: 0, request_count: 100 },
          items: [item], total: 1, page: 1, page_size: 20,
        },
      },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Pagination: true } },
    })

    expect(wrapper.text()).toContain('均衡模式 · 真实生效')
    expect(wrapper.text()).toContain('64.0% 目标')
    expect(wrapper.text()).toContain('58.0% 实际')
    await wrapper.get('tbody tr').trigger('click')
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({ account_id: 10 })
  })

  it('explains that shadow mode does not affect real traffic', () => {
    const wrapper = mount(SchedulerRankingTable, {
      props: {
        groupName: 'Codex', loading: false, filters: { window: '1h' },
        result: {
          policy_context: { engine_enabled: true, global_enabled: true, group_enabled: true, configured_mode: 'cost_first', effective_mode: 'legacy', shadow_mode: true, policy_version: 'v2', calculated_at: '2026-07-15T12:00:00Z' },
          summary: { candidate_count: 0, eligible_count: 0, rejected_count: 0, low_confidence_count: 0, request_count: 0 },
          items: [], total: 0, page: 1, page_size: 20,
        },
      },
      global: { plugins: [createSchedulerTestI18n()], stubs: { Pagination: true } },
    })

    expect(wrapper.text()).toContain('成本优先 · 影子计算，真实流量仍使用 Legacy')
  })
})
