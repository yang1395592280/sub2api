import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAdminComplianceStore } from '@/stores/adminCompliance'
import adminComplianceAPI from '@/api/admin/compliance'

vi.mock('@/api/admin/compliance', () => ({
  default: {
    getStatus: vi.fn(),
    accept: vi.fn()
  }
}))

describe('useAdminComplianceStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('并发获取合规状态时复用同一个请求', async () => {
    vi.mocked(adminComplianceAPI.getStatus).mockResolvedValue({
      required: false,
      version: 'v1',
      document_path_zh: '',
      document_path_en: '',
      document_url_zh: '',
      document_url_en: '',
      ack_phrase_zh: '',
      ack_phrase_en: ''
    })

    const store = useAdminComplianceStore()
    await Promise.all([store.fetchStatus(), store.fetchStatus()])

    expect(adminComplianceAPI.getStatus).toHaveBeenCalledTimes(1)
  })
})
