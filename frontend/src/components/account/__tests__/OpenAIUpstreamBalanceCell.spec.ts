import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Account } from '@/types'
import OpenAIUpstreamBalanceCell from '../OpenAIUpstreamBalanceCell.vue'
import { accountsAPI } from '@/api/admin/accounts'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'admin.accounts.upstreamBalance.updatedAt') {
          return `Updated ${params?.time ?? ''}`
        }
        if (key === 'admin.accounts.upstreamBalance.refresh') return 'Refresh balance'
        if (key === 'admin.accounts.upstreamBalance.unknown') return 'Not queried'
        if (key === 'admin.accounts.upstreamBalance.failed') return 'Balance query failed'
        if (key === 'admin.accounts.upstreamBalance.unsupported') return 'Unsupported'
        if (key === 'common.loading') return 'Loading'
        if (key === 'common.error') return 'Error'
        return key
      }
    })
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'openai',
    type: 'apikey',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-06-20T00:00:00Z',
    updated_at: '2026-06-20T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides
  }
}

describe('OpenAIUpstreamBalanceCell', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders cached USD balance for OpenAI API Key account', () => {
    const account = makeAccount({
      platform: 'openai',
      type: 'apikey',
      extra: {
        upstream_balance_status: 'ok',
        upstream_balance_provider: 'sub2api',
        upstream_balance_remaining: 12.34,
        upstream_balance_unit: 'USD',
        upstream_balance_updated_at: '2026-06-20T10:00:00Z'
      }
    })

    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: { account }
    })

    const balanceAmount = wrapper.get('[data-testid="openai-upstream-balance-amount"]')

    expect(balanceAmount.text()).toBe('$12.34')
    expect(balanceAmount.classes()).toEqual(expect.arrayContaining(['text-sm', 'font-semibold']))
    expect(wrapper.text()).toContain('sub2api')
  })

  it('renders cached USD balance for Anthropic API Key account', () => {
    const account = makeAccount({
      platform: 'anthropic',
      type: 'apikey',
      extra: {
        upstream_balance_status: 'ok',
        upstream_balance_provider: 'sub2api',
        upstream_balance_remaining: 6.78,
        upstream_balance_unit: 'USD',
        upstream_balance_updated_at: '2026-06-20T10:00:00Z'
      }
    })

    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: { account }
    })

    expect(wrapper.get('[data-testid="openai-upstream-balance-amount"]').text()).toBe('$6.78')
    expect(wrapper.text()).toContain('sub2api')
  })

  it.each(['kimi', 'deepseek'] as const)('%s configured upstream admin shows first-refresh action without a snapshot', (platform) => {
    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: {
        account: makeAccount({
          platform,
          type: 'apikey',
          credentials: { upstream_admin_type: 'sub2api' }
        })
      }
    })

    expect(wrapper.text()).toContain('Not queried')
    expect(wrapper.text()).toContain('Refresh balance')
  })

  it.each(['kimi', 'deepseek'] as const)('%s always exposes refresh action on a new account', (platform) => {
    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: { account: makeAccount({ platform, type: 'apikey' }) }
    })

    expect(wrapper.get('button').text()).toBe('Refresh balance')
  })

  it('renders quota unit without dollar sign', () => {
    const account = makeAccount({
      platform: 'openai',
      type: 'apikey',
      extra: {
        upstream_balance_status: 'ok',
        upstream_balance_remaining: 375000,
        upstream_balance_unit: 'quota',
        upstream_balance_provider: 'new-api'
      }
    })

    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: { account }
    })

    expect(wrapper.text()).toContain('375,000 quota')
    expect(wrapper.text()).toContain('quota')
    expect(wrapper.text()).not.toContain('$375,000')
  })

  it('emits refreshed account after manual refresh', async () => {
    vi.spyOn(accountsAPI, 'refreshUpstreamBalance').mockResolvedValue(
      makeAccount({
        id: 1,
        platform: 'openai',
        type: 'apikey',
        extra: {
          upstream_balance_status: 'ok',
          upstream_balance_remaining: 9,
          upstream_balance_unit: 'USD'
        }
      })
    )

    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: {
        account: makeAccount({
          id: 1,
          platform: 'openai',
          type: 'apikey'
        })
      }
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('refreshed')?.[0]?.[0].extra?.upstream_balance_remaining).toBe(9)
  })

  it('refreshes Anthropic API Key upstream balance manually', async () => {
    vi.spyOn(accountsAPI, 'refreshUpstreamBalance').mockResolvedValue(
      makeAccount({
        id: 2,
        platform: 'anthropic',
        type: 'apikey',
        extra: {
          upstream_balance_status: 'ok',
          upstream_balance_remaining: 8,
          upstream_balance_unit: 'USD',
          upstream_group: 'Claude Pro'
        }
      })
    )

    const wrapper = mount(OpenAIUpstreamBalanceCell, {
      props: {
        account: makeAccount({
          id: 2,
          platform: 'anthropic',
          type: 'apikey'
        })
      }
    })

    await wrapper.get('button').trigger('click')

    expect(accountsAPI.refreshUpstreamBalance).toHaveBeenCalledWith(2)
    expect(wrapper.emitted('refreshed')?.[0]?.[0].extra?.upstream_group).toBe('Claude Pro')
  })
})
