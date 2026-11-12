import { describe, expect, it } from 'vitest'
import { formatWalletAmount } from '../walletDisplay'

describe('wallet amount display without changing billing precision', () => {
  it.each([
    [undefined, '0.00'], [null, '0.00'], [NaN, '0.00'], [Infinity, '0.00'],
    [-0, '0.00'], [-0.00000001, '0.00'], [-0.0049, '0.00'], [-0.005, '-0.01'],
    [1.005, '1.01'], [-1.005, '-1.01'], [12, '12.00'], [-0.0414776, '-0.04'],
    [12345.6789, '12345.68']
  ])('formats %s as %s', (amount, expected) => {
    expect(formatWalletAmount(amount)).toBe(expected)
  })
})
