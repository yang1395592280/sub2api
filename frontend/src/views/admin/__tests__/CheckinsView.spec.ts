import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { api, showError, showSuccess } = vi.hoisted(() => ({
  api: { walletPolicy: vi.fn(), saveWalletPolicy: vi.fn(), settings: vi.fn(), records: vi.fn(), risks: vi.fn() },
  showError: vi.fn(), showSuccess: vi.fn()
}))
vi.mock('@/api/checkin', () => ({ checkinAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
import CheckinsView from '../CheckinsView.vue'

beforeEach(() => {
  vi.clearAllMocks()
  api.walletPolicy.mockResolvedValue({ paid_units: 2, welfare_units: 1 })
  api.saveWalletPolicy.mockImplementation(async policy => ({ ...policy }))
  api.settings.mockResolvedValue({ enabled: true, min_reward: 1, max_reward: 1, bonus_enabled: true, bonus_success_rate: 50, distribution: null })
  api.records.mockResolvedValue({ items: [], total: 0 }); api.risks.mockResolvedValue({ items: [], total: 0 })
})
const render = () => mount(CheckinsView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })

describe('wallet usage policy settings', () => {
  it('loads independently of check-in settings and saves the selected ratio', async () => {
    const wrapper = render(); await flushPromises()
    const form = wrapper.get('[data-testid=wallet-policy]')
    expect((form.get('[name=paid_units]').element as HTMLInputElement).value).toBe('2')
    await form.get('[name=paid_units]').setValue('3'); await form.get('[name=welfare_units]').setValue('1')
    await form.trigger('submit'); await flushPromises()
    expect(api.saveWalletPolicy).toHaveBeenCalledWith({ paid_units: 3, welfare_units: 1 })
    expect(showSuccess).toHaveBeenCalled(); expect(showError).not.toHaveBeenCalled()
  })
  it('supports disabling subsidies and retains integer input bounds', async () => {
    const wrapper = render(); await flushPromises()
    const form = wrapper.get('[data-testid=wallet-policy]')
    expect(form.get('[name=paid_units]').attributes()).toMatchObject({ min: '1', max: '10000', step: '1', required: '' })
    expect(form.get('[name=welfare_units]').attributes()).toMatchObject({ min: '0', max: '10000', step: '1', required: '' })
    await form.get('[name=welfare_units]').setValue('0'); await form.trigger('submit'); await flushPromises()
    expect(api.saveWalletPolicy).toHaveBeenCalledWith({ paid_units: 2, welfare_units: 0 })
  })
  it('surfaces persistence failures without claiming success', async () => {
    api.saveWalletPolicy.mockRejectedValue(new Error('failed'))
    const wrapper = render(); await flushPromises()
    await wrapper.get('[data-testid=wallet-policy]').trigger('submit'); await flushPromises()
    expect(showError).toHaveBeenCalled(); expect(showSuccess).not.toHaveBeenCalled()
  })
})
