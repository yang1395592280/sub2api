import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ProfileCheckinCard from '@/components/user/profile/ProfileCheckinCard.vue'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  doCheckin: vi.fn(),
  playBonus: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  showError: vi.fn(),
  refreshUser: vi.fn()
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params?: Record<string, string | number>) => {
      if (params) {
        return `${key}:${JSON.stringify(params)}`
      }
      return key
    }
  })
}))

vi.mock('@/api', () => ({
  checkinAPI: {
    getStatus: (...args: unknown[]) => mocks.getStatus(...args),
    doCheckin: (...args: unknown[]) => mocks.doCheckin(...args),
    playBonus: (...args: unknown[]) => mocks.playBonus(...args)
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: mocks.showSuccess,
    showWarning: mocks.showWarning,
    showError: mocks.showError
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: mocks.refreshUser
  })
}))

vi.mock('@/components/TurnstileWidget.vue', () => ({
  default: {
    name: 'TurnstileWidget',
    props: ['siteKey'],
    template: '<div class="turnstile-widget" :data-site-key="siteKey"></div>'
  }
}))

describe('ProfileCheckinCard', () => {
  beforeEach(() => {
    mocks.getStatus.mockReset()
    mocks.doCheckin.mockReset()
    mocks.playBonus.mockReset()
    mocks.showSuccess.mockReset()
    mocks.showWarning.mockReset()
    mocks.showError.mockReset()
    mocks.refreshUser.mockReset()

    mocks.getStatus.mockResolvedValue({
      enabled: true,
      min_reward: 0.002,
      max_reward: 0.02,
      bonus_enabled: true,
      bonus_available: false,
      bonus_success_rate: 50,
      today_record: null,
      stats: {
        total_reward: 0.345,
        total_checkins: 12,
        checkin_count: 2,
        checked_in_today: false,
        records: [
          { checkin_date: '2026-04-02', reward_amount: 0.01 },
          { checkin_date: '2026-04-01', reward_amount: 0.02 }
        ]
      }
    })
  })

  it('挂载后加载并展示签到统计', async () => {
    const wrapper = mount(ProfileCheckinCard, {
      props: {
        enabled: true,
        minReward: 0.002,
        maxReward: 0.02,
        turnstileEnabled: false,
        turnstileSiteKey: ''
      }
    })

    await flushPromises()

    expect(mocks.getStatus).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('$0.34')
    expect(wrapper.text()).toContain('$0.0300')
  })

  it('开启 turnstile 时点击签到先展示验证组件', async () => {
    const wrapper = mount(ProfileCheckinCard, {
      props: {
        enabled: true,
        minReward: 0.002,
        maxReward: 0.02,
        turnstileEnabled: true,
        turnstileSiteKey: 'site-key'
      }
    })

    await flushPromises()

    const button = wrapper.find('button.btn.btn-primary')
    await button.trigger('click')
    await flushPromises()

    expect(mocks.doCheckin).not.toHaveBeenCalled()
    expect(wrapper.find('.turnstile-widget').exists()).toBe(true)
  })

  it('未签到时奖励翻倍按钮禁用，签到后可点击触发', async () => {
    mocks.getStatus
      .mockResolvedValueOnce({
        enabled: true,
        min_reward: 0.002,
        max_reward: 0.02,
        bonus_enabled: true,
        bonus_available: false,
        bonus_success_rate: 65,
        today_record: null,
        stats: {
          total_reward: 0.345,
          total_checkins: 12,
          checkin_count: 2,
          checked_in_today: false,
          records: []
        }
      })
      .mockResolvedValueOnce({
        enabled: true,
        min_reward: 0.002,
        max_reward: 0.02,
        bonus_enabled: true,
        bonus_available: true,
        bonus_success_rate: 65,
        today_record: {
          checkin_date: '2026-04-22',
          reward_amount: 20,
          base_reward_amount: 10,
          bonus_status: 'win',
          bonus_delta_amount: 10
        },
        stats: {
          total_reward: 20,
          total_checkins: 12,
          checkin_count: 2,
          checked_in_today: true,
          records: []
        }
      })

    mocks.doCheckin.mockResolvedValue({
      checkin_date: '2026-04-22',
      reward_amount: 10
    })
    mocks.playBonus.mockResolvedValue({
      checkin_date: '2026-04-22',
      reward_amount: 20,
      base_reward_amount: 10,
      bonus_status: 'win',
      bonus_delta_amount: 10
    })

    const wrapper = mount(ProfileCheckinCard, {
      props: {
        enabled: true,
        minReward: 0.002,
        maxReward: 0.02,
        turnstileEnabled: false,
        turnstileSiteKey: ''
      }
    })

    await flushPromises()

    const bonusButtonBefore = wrapper.get('[data-testid=\"checkin-bonus-button\"]')
    expect((bonusButtonBefore.element as HTMLButtonElement).disabled).toBe(true)

    const checkinButton = wrapper.find('button.btn.btn-primary')
    await checkinButton.trigger('click')
    await flushPromises()

    const bonusButtonAfter = wrapper.get('[data-testid=\"checkin-bonus-button\"]')
    expect((bonusButtonAfter.element as HTMLButtonElement).disabled).toBe(false)

    await bonusButtonAfter.trigger('click')
    await flushPromises()

    expect(mocks.playBonus).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('profile.checkin.luckyBonusResultWin')
  })
})
