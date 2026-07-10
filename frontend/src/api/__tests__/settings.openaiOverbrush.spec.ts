import { describe, expect, it, vi } from 'vitest'
import {
  getOpenAIOverbrushSettings,
  updateOpenAIOverbrushSettings
} from '@/api/admin/settings'
import { apiClient } from '@/api/client'

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
    put: vi.fn()
  }
}))

describe('admin settings openai overbrush helpers', () => {
  it('gets OpenAI overbrush settings', async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: { consecutive_429_threshold: 10 } })

    const result = await getOpenAIOverbrushSettings()

    expect(apiClient.get).toHaveBeenCalledWith('/admin/settings/openai-overbrush')
    expect(result.consecutive_429_threshold).toBe(10)
  })

  it('updates OpenAI overbrush settings', async () => {
    vi.mocked(apiClient.put).mockResolvedValueOnce({ data: { consecutive_429_threshold: 12 } })

    const result = await updateOpenAIOverbrushSettings({ consecutive_429_threshold: 12 })

    expect(apiClient.put).toHaveBeenCalledWith('/admin/settings/openai-overbrush', { consecutive_429_threshold: 12 })
    expect(result.consecutive_429_threshold).toBe(12)
  })
})
