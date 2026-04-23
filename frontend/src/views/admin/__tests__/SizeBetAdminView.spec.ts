import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const {
  getSettings,
  updateSettings,
  listRounds,
  listBets,
  listLedger,
  refundRound
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  listRounds: vi.fn(),
  listBets: vi.fn(),
  listLedger: vi.fn(),
  refundRound: vi.fn()
}))

const storageStub = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn()
}

vi.stubGlobal('localStorage', storageStub)
vi.stubGlobal('sessionStorage', storageStub)

vi.mock('@/api/admin/sizeBet', () => ({
  getSettings,
  updateSettings,
  listRounds,
  listBets,
  listLedger,
  refundRound,
  default: {
    getSettings,
    updateSettings,
    listRounds,
    listBets,
    listLedger,
    refundRound
  }
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN'
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, values?: Record<string, unknown>) =>
        values ? `${key} ${Object.values(values).join(' ')}` : key
    })
  }
})

describe('SizeBetAdminView', () => {
  beforeEach(() => {
    getSettings.mockReset()
    updateSettings.mockReset()
    listRounds.mockReset()
    listBets.mockReset()
    listLedger.mockReset()
    refundRound.mockReset()

    listRounds.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listBets.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listLedger.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1
    })
  })

  it('loads settings and renders probability/odds controls', async () => {
    const { default: SizeBetAdminView } = await import('../SizeBetAdminView.vue')

    getSettings.mockResolvedValue({
      enabled: true,
      round_duration_seconds: 60,
      bet_close_offset_seconds: 50,
      allowed_stakes: [2, 5, 10, 20],
      probabilities: { small: 45, mid: 10, big: 45 },
      odds: { small: 2, mid: 10, big: 2 },
      rules_markdown: 'rule-body'
    })

    const wrapper = mount(SizeBetAdminView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Toggle: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<input type="checkbox" :checked="modelValue" />'
          },
          DataTable: {
            props: ['data'],
            template: '<div><slot />{{ data.length }}</div>'
          },
          Pagination: true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('45')
    expect(wrapper.text()).toContain('10')
    expect(wrapper.find('[data-test="save-settings"]').exists()).toBe(true)
  })
})
