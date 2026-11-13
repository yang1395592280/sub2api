import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import TestEstimatedCost from '../TestEstimatedCost.vue'
import pelican from '@/i18n/locales/zh/pelican'
import type { PelicanTest } from '@/api/admin/accounts'

const render = (amount: number | null) => mount(TestEstimatedCost, {
  props: { test: { estimated_cost_usd: amount, cost_model_id: 'gpt-5.5', upstream_rate_multiplier: 0.1 } as PelicanTest },
  global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { pelican } } })] }
})

describe('Upstream estimated cost', () => {
  it('distinguishes missing evidence from a free request', () => {
    expect(render(null).text()).toBe('上游消耗：不可估算')
    expect(render(0).text()).toBe('上游消耗（估算）$0')
  })
  it('preserves nonzero micro charges and explains the calculation basis', () => {
    const wrapper = render(0.00000002)
    expect(wrapper.text()).toContain('$2.00e-8')
    expect(wrapper.attributes('title')).toContain('gpt-5.5')
    expect(wrapper.attributes('title')).toContain('0.1x')
    expect(wrapper.attributes('title')).toContain('可能与上游实扣不同')
  })
})
