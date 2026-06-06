import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import { checkinAPI } from '@/api/checkin'

describe('checkin api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('gets checkin status with month/timezone query only', async () => {
    await checkinAPI.getStatus({ month: '2026-06', timezone: 'Asia/Shanghai' })

    expect(get).toHaveBeenCalledWith('/user/checkin', {
      params: { month: '2026-06', timezone: 'Asia/Shanghai' },
    })
  })

  it('claims checkin reward as points-only response', async () => {
    await checkinAPI.checkin({ turnstile_token: 'token', timezone: 'Asia/Shanghai' })

    expect(post).toHaveBeenCalledWith('/user/checkin', {
      turnstile_token: 'token',
      timezone: 'Asia/Shanghai',
    })
  })

  it('plays lucky bonus through the dedicated checkin endpoint', async () => {
    await checkinAPI.playLuckyBonus({ timezone: 'Asia/Shanghai' })

    expect(post).toHaveBeenCalledWith('/user/checkin/lucky-bonus', {
      timezone: 'Asia/Shanghai',
    })
  })
})
