import { describe, expect, it, vi } from 'vitest'
const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, put } }))
import { checkinAPI } from '../checkin'

describe('wallet policy API', () => {
  it('round trips the shared policy endpoint and preserves a zero welfare share', async () => {
    const policy = { paid_units: 3, welfare_units: 0 }
    get.mockResolvedValue({ data: policy }); put.mockResolvedValue({ data: policy })
    expect(await checkinAPI.walletPolicy()).toEqual(policy)
    expect(get).toHaveBeenCalledWith('/admin/checkins/wallet-policy')
    expect(await checkinAPI.saveWalletPolicy(policy)).toEqual(policy)
    expect(put).toHaveBeenCalledWith('/admin/checkins/wallet-policy', policy)
  })
})
