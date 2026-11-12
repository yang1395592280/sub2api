import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserBalanceModal from '../UserBalanceModal.vue'
import type { AdminUser } from '@/types'
const mocks = vi.hoisted(() => ({ updateBalance: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
describe('balance operation error feedback', () => {
  it.each([
    [{ message: 'User balance changed; retry' }, 'User balance changed; retry'],
    [{ response: { data: { detail: 'Legacy detail' } } }, 'Legacy detail'],
    [{}, 'common.error'],
  ])('shows the available API error and keeps the dialog open', async (error, message) => {
    mocks.updateBalance.mockRejectedValueOnce(error)
    const w = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await w.get('input[type="number"]').setValue('1')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith(message)
    expect(w.emitted('success')).toBeUndefined(); expect(w.emitted('close')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})

const user = { id: 1, email: 'a@example.com', balance: 5.12345678, checkin_balance: -1, liyu_balance: 2.12345678, gift_balance: 4 } as AdminUser
function refundDialog() {
  return mount(UserBalanceModal, { props: { show: true, operation: 'subtract', user },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name=footer /></div>' } } } })
}
describe('refund paid and selected promotional credit', () => {
  it.each([['gift', 0, 2], ['liyu', 1, 2], ['gift', 3, 0]])('refunds source %s with paid %s and welfare %s', async (source, paid, welfare) => {
    mocks.updateBalance.mockResolvedValueOnce(user)
    const w = refundDialog()
    await w.get('#welfare-refund-source').setValue(source)
    await w.get('#paid-balance-amount').setValue(paid)
    await w.get('#welfare-gift-amount').setValue(welfare)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, paid, 'subtract', '', welfare, source, expect.any(String))
    expect(w.emitted('success')).toHaveLength(1)
  })
  it.each([['checkin', 0.01], ['liyu', 2.13], ['gift', 5]])('rejects over-refund from %s without using other sources', async (source, amount) => {
    const w = refundDialog()
    await w.get('#welfare-refund-source').setValue(source)
    await w.get('#welfare-gift-amount').setValue(amount)
    await w.get('form').trigger('submit')
    expect(mocks.updateBalance).not.toHaveBeenCalled()
    expect(mocks.showError).toHaveBeenCalledWith('wallet.insufficientRefundBalance')
  })
  it('keeps all-balance refunds exact despite rounding the preview', async () => {
    const w = refundDialog()
    await w.get('#welfare-refund-source').setValue('liyu')
    const all = w.findAll('button').filter(b => b.text() === 'admin.users.withdrawAll')
    await all[0]!.trigger('click'); await all[1]!.trigger('click')
    expect(w.text()).toContain('$5.12')
    expect(w.text()).toContain('$2.12')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 5.12345678, 'subtract', '', 2.12345678, 'liyu', expect.any(String))
  })
  it('reuses the operation key after a network error, changing it when the request changes', async () => {
    mocks.updateBalance.mockRejectedValue(new Error('network'))
    const w = refundDialog()
    await w.get('#welfare-gift-amount').setValue(1)
    await w.get('form').trigger('submit'); await flushPromises()
    const key = mocks.updateBalance.mock.calls[0]![6]
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance.mock.calls[1]![6]).toBe(key)
    await w.get('#welfare-gift-amount').setValue(2)
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance.mock.calls[2]![6]).not.toBe(key)
  })
  it('does not enable refund with no positive amount', async () => {
    const w = refundDialog()
    expect(w.get('button[type=submit]').attributes('disabled')).toBeDefined()
    await w.get('form').trigger('submit')
    expect(mocks.updateBalance).not.toHaveBeenCalled()
  })
})
