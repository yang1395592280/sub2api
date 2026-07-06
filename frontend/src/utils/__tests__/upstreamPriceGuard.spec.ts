import { describe, expect, it } from 'vitest'

import { getUpstreamPriceGuardLabel } from '../upstreamPriceGuard'

describe('getUpstreamPriceGuardLabel', () => {
  it('formats blocked price guard status', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'blocked',
      upstream_price_guard_actual_multiplier: 0.12,
      upstream_price_guard_max_multiplier: 0.08
    })).toBe('价格超限 0.12x > 0.08x')
  })

  it('hides ok and empty statuses', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'ok' })).toBe('')
    expect(getUpstreamPriceGuardLabel({})).toBe('')
  })

  it('formats unsupported and error states', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'unsupported' })).toBe('价格未知')
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'error' })).toBe('价格检查失败')
  })
})
