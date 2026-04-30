import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAdminSettingsStore } from '@/stores/adminSettings'

const apiMocks = vi.hoisted(() => ({
  settingsGetSettings: vi.fn(),
  paymentGetConfig: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getSettings: apiMocks.settingsGetSettings,
    },
    payment: {
      getConfig: apiMocks.paymentGetConfig,
    },
  },
}))

describe('useAdminSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    const storage = new Map<string, string>()
    vi.stubGlobal('localStorage', {
      getItem: vi.fn((key: string) => storage.get(key) ?? null),
      setItem: vi.fn((key: string, value: string) => {
        storage.set(key, value)
      }),
      removeItem: vi.fn((key: string) => {
        storage.delete(key)
      }),
      clear: vi.fn(() => {
        storage.clear()
      }),
    })
  })

  it('fetch 只请求 admin settings，不在首屏请求 payment config', async () => {
    localStorage.setItem('payment_enabled_cached', 'true')
    apiMocks.settingsGetSettings.mockResolvedValue({
      ops_monitoring_enabled: false,
      ops_realtime_monitoring_enabled: true,
      ops_query_mode_default: 'manual',
      custom_menu_items: [],
    })
    apiMocks.paymentGetConfig.mockResolvedValue({
      data: { enabled: false },
    })

    const store = useAdminSettingsStore()
    await store.fetch()

    expect(apiMocks.settingsGetSettings).toHaveBeenCalledTimes(1)
    expect(apiMocks.paymentGetConfig).not.toHaveBeenCalled()
    expect(store.paymentEnabled).toBe(true)
    expect(store.opsMonitoringEnabled).toBe(false)
    expect(store.opsQueryModeDefault).toBe('manual')
  })
})
