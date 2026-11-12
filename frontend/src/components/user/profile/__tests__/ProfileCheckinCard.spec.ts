import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileCheckinCard from '../ProfileCheckinCard.vue'
const mocks = vi.hoisted(() => ({ status: vi.fn(), play: vi.fn(), settings: vi.fn(), refresh: vi.fn(), success: vi.fn(), error: vi.fn() }))
vi.mock('@/api/checkin', () => ({ checkinAPI: { status: mocks.status, play: mocks.play } }))
vi.mock('@/api/auth', () => ({ getPublicSettings: mocks.settings }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser: mocks.refresh }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const status = () => ({ enabled: true, min_reward: 1, max_reward: 1, bonus_enabled: true, bonus_available: true, bonus_success_rate: 50, stats: { total_reward: -1, total_checkins: 2, checked_in_today: true, records: [] } })
const mountCard = () => mount(ProfileCheckinCard, { global: { stubs: { CaptchaChallenge: { template: '<div />', methods: { verifyAction: () => null, reset: vi.fn() } } } } })
beforeEach(() => {
  vi.clearAllMocks()
  mocks.status.mockResolvedValue(status())
  mocks.settings.mockResolvedValue({ turnstile_enabled: false, tencent_captcha_enabled: false, aliyun_captcha_enabled: false })
  mocks.play.mockResolvedValue({})
})
describe('check-in wallet interactions', () => {
  it('allows a bonus after an earlier negative reward and refreshes the wallet', async () => {
    const w = mountCard(); await flushPromises()
    const bonus = w.findAll('button').find(b => b.text() === 'wallet.bonus')!
    expect(bonus.attributes('disabled')).toBeUndefined()
    await bonus.trigger('click'); await flushPromises()
    expect(mocks.play).toHaveBeenCalledWith(true, { website: '' })
    expect(mocks.refresh).toHaveBeenCalledTimes(1)
    expect(mocks.success).toHaveBeenCalled()
    w.unmount()
  })
  it('shows the confiscation reason and removes claim actions after suspension', async () => {
    mocks.status.mockResolvedValue({ ...status(), enabled: false, risk: { case_id: 7, reason: '已核实自动签到', cleared_amount: 3 } })
    const w = mountCard(); await flushPromises()
    expect(w.text()).toContain('已核实自动签到')
    expect(w.text()).toContain('$3')
    expect(w.findAll('button').some(b => b.text() === 'wallet.bonus')).toBe(false)
    w.unmount()
  })
  it('does not submit claims when action captcha is cancelled', async () => {
    mocks.settings.mockResolvedValue({ tencent_captcha_enabled: true, tencent_captcha_app_id: 'app' })
    const w = mountCard(); await flushPromises()
    await w.findAll('button').find(b => b.text() === 'wallet.bonus')!.trigger('click'); await flushPromises()
    expect(mocks.play).not.toHaveBeenCalled()
    w.unmount()
  })
})
