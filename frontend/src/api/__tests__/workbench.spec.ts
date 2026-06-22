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

import { workbenchAPI } from '@/api/workbench'

describe('workbench api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('uses an extended timeout for long-running sends', async () => {
    const payload = {
      mode: 'chat' as const,
      api_key_id: 7,
      endpoint: 'chat_completions' as const,
      model: 'gpt-5.5',
      input: 'hi',
    }

    await workbenchAPI.send(1, payload)

    expect(post).toHaveBeenCalledWith('/workbench/conversations/1/send', payload, { timeout: 120000 })
  })
})
