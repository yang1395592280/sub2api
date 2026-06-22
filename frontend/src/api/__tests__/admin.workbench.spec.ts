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

import adminWorkbenchAPI from '@/api/admin/workbench'

describe('admin workbench api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('lists conversations with filters', async () => {
    const params = {
      page: 1,
      page_size: 20,
      mode: 'image' as const,
      user_id: 8,
      has_images: true,
      older_than_days: 7,
    }

    await adminWorkbenchAPI.listConversations(params)

    expect(get).toHaveBeenCalledWith('/admin/workbench/conversations', { params })
  })

  it('gets stats and detail', async () => {
    await adminWorkbenchAPI.getStats(7)
    await adminWorkbenchAPI.getConversation(9)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/workbench/stats', { params: { retention_days: 7 } })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/workbench/conversations/9')
  })

  it('hard deletes selected and expired conversations', async () => {
    await adminWorkbenchAPI.batchDeleteConversations([1, 2])
    await adminWorkbenchAPI.cleanupExpiredConversations(7)

    expect(post).toHaveBeenNthCalledWith(1, '/admin/workbench/conversations/batch-delete', { conversation_ids: [1, 2] })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/workbench/conversations/cleanup-expired', { retention_days: 7 })
  })
})
