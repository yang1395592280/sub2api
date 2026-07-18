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

  it('formats blocked status when only actual multiplier exists', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'blocked',
      upstream_price_guard_actual_multiplier: '0.12',
      upstream_price_guard_max_multiplier: null
    })).toBe('价格超限 0.12x')
  })

  it('formats blocked status when only max multiplier exists', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'blocked',
      upstream_price_guard_actual_multiplier: null,
      upstream_price_guard_max_multiplier: '0.08'
    })).toBe('价格超限 > 0.08x')
  })

  it('formats blocked status when multipliers are missing', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'blocked'
    })).toBe('价格超限')
  })

  it('hides ok and empty statuses', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'ok' })).toBe('')
    expect(getUpstreamPriceGuardLabel({})).toBe('')
  })

  it('formats unsupported and error states', () => {
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'unsupported' })).toBe('价格未知')
    expect(getUpstreamPriceGuardLabel({ upstream_price_guard_status: 'error' })).toBe('价格检查失败')
  })

  it('hides a stale unsupported state when the current channel price is known and within the limit', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'unsupported',
      upstream_price_guard_max_multiplier: 0.05
    }, 0.04)).toBe('')
  })

  it('keeps warning when the current channel price exceeds the cached guard limit', () => {
    expect(getUpstreamPriceGuardLabel({
      upstream_price_guard_status: 'unsupported',
      upstream_price_guard_max_multiplier: 0.03
    }, 0.04)).toBe('价格超限 0.04x > 0.03x')
  })
})
