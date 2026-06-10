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

import {
  batchAddBalanceToUsers,
  batchAddGroupToUsers,
  getUserBalanceSummary,
  type BatchAddBalanceToUsersResponse,
  type BatchAddGroupToUsersResponse,
  type UserBalanceSummaryResponse,
} from '@/api/admin/users'

describe('admin users summary and batch api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('gets the admin user balance summary from the summary endpoint', async () => {
    const response: UserBalanceSummaryResponse = {
      total_balance: 123.45,
      user_count: 3,
    }
    get.mockResolvedValue({ data: response })

    const result = await getUserBalanceSummary()

    expect(get).toHaveBeenCalledWith('/admin/users/summary')
    expect(result).toEqual(response)
  })

  it('posts selected user ids and target group to the batch add group endpoint', async () => {
    const response: BatchAddGroupToUsersResponse = {
      group_id: 9,
      processed_users: 2,
    }
    post.mockResolvedValue({ data: response })

    const result = await batchAddGroupToUsers([7, 8], 9)

    expect(post).toHaveBeenCalledWith('/admin/users/batch-add-group', {
      user_ids: [7, 8],
      group_id: 9,
    })
    expect(result).toEqual(response)
  })

  it('posts selected user ids and balance to the batch add balance endpoint', async () => {
    const response: BatchAddBalanceToUsersResponse = {
      affected: 2,
    }
    post.mockResolvedValue({ data: response })

    const result = await batchAddBalanceToUsers([7, 8], 1.5, 'bonus')

    expect(post).toHaveBeenCalledWith('/admin/users/batch-balance', {
      user_ids: [7, 8],
      balance: 1.5,
      notes: 'bonus',
    })
    expect(result).toEqual(response)
  })
})
