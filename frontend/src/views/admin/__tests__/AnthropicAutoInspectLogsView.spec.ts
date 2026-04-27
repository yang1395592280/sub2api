import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getSettings, listLogs, listBatches, updateSettings, runNow } = vi.hoisted(() => ({
  getSettings: vi.fn(),
  listLogs: vi.fn(),
  listBatches: vi.fn(),
  updateSettings: vi.fn(),
  runNow: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    anthropicAutoInspect: {
      getSettings,
      listLogs,
      listBatches,
      updateSettings,
      runNow,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string | null) => value ?? '-',
}))

describe('AnthropicAutoInspectLogsView', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', {
      getItem: vi.fn(() => null),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      key: vi.fn(() => null),
      length: 0,
    })

    getSettings.mockReset()
    listLogs.mockReset()
    listBatches.mockReset()
    updateSettings.mockReset()
    runNow.mockReset()

    getSettings.mockResolvedValue({
      enabled: true,
      interval_minutes: 1,
      error_cooldown_minutes: 30,
    })
    listLogs.mockResolvedValue({
      items: [
        {
          id: 1,
          batch_id: 9,
          account_id: 42,
          account_name_snapshot: 'anthropic-main',
          platform: 'anthropic',
          account_type: 'apikey',
          result: 'rate_limited',
          skip_reason: '',
          response_text: 'rate limited until 2026-04-26 12:34:56',
          error_message: '',
          rate_limit_reset_at: '2026-04-26T12:34:56Z',
          temp_unschedulable_until: '2026-04-26T12:34:56Z',
          schedulable_changed: true,
          started_at: '2026-04-26T12:30:00Z',
          finished_at: '2026-04-26T12:30:01Z',
          latency_ms: 1000,
          created_at: '2026-04-26T12:30:01Z',
        },
      ],
      pagination: { total: 1, page: 1, page_size: 20, pages: 1 },
    })
    listBatches.mockResolvedValue({
      items: [],
      pagination: { total: 0, page: 1, page_size: 10, pages: 0 },
    })
    updateSettings.mockResolvedValue(undefined)
    runNow.mockResolvedValue(undefined)
  })

  it('loads and renders anthropic auto inspect logs', async () => {
    const { default: AnthropicAutoInspectLogsView } = await import('../AnthropicAutoInspectLogsView.vue')
    const wrapper = mount(AnthropicAutoInspectLogsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Pagination: true,
        },
      },
    })

    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(1)
    expect(listLogs).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('anthropic-main')
    expect(wrapper.text()).toContain('rate_limited')
  })
})
