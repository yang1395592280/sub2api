import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get
  }
}))

import { getGroupCapacityUsers, type GroupCapacityUserDetail } from '@/api/admin/groups'

describe('admin groups capacity users api', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('requests paginated active capacity users for a group', async () => {
    const response = {
      items: [
        {
          user_id: 1,
          username: 'alpha',
          email: 'a@example.com',
          notes: '',
          status: 'active',
          current_concurrency: 2,
          concurrency_limit: 3,
          current_rpm: 4,
          effective_rpm_limit: 6,
          rpm_limit_source: 'override',
          rpm_override: 6,
          group_rpm_limit: 10,
          user_rpm_limit: 20
        } satisfies GroupCapacityUserDetail
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    }
    get.mockResolvedValue({ data: response })

    const result = await getGroupCapacityUsers(10, 1, 20, true)

    expect(get).toHaveBeenCalledWith('/admin/groups/10/capacity-users', {
      params: { page: 1, page_size: 20, active_only: true }
    })
    expect(result).toEqual(response)
  })
})
