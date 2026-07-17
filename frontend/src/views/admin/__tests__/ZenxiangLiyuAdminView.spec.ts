import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import ZenxiangLiyuAdminView from '../ZenxiangLiyuAdminView.vue'

const api = vi.hoisted(() => ({
  getSettings: vi.fn(), updateSettings: vi.fn(), listPrizes: vi.fn(), replacePrizes: vi.fn(),
  listGrants: vi.fn(), createGrant: vi.fn(), deleteGrant: vi.fn(), getOverviewStats: vi.fn(),
  listUserStats: vi.fn(), listUserRecords: vi.fn(), listPrizeStats: vi.fn(), listPeriodStats: vi.fn(), resetGrantDailyPlays: vi.fn(),
  simulate: vi.fn(), recommend: vi.fn(), applySimulation: vi.fn(),
}))
const usersAPI = vi.hoisted(() => ({ list: vi.fn() }))
const notifications = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { zenxiangLiyu: api, users: usersAPI } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => notifications }))
vi.mock('@/utils/apiError', () => ({ extractApiErrorMessage: (_error: unknown, fallback: string) => fallback }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.zenxiangLiyu.probabilityWarning') return `概率合计 ${params?.total}，必须为 100`
        if (key === 'admin.zenxiangLiyu.planSummary') return `理论盈亏 ${params?.profit}，理论盈利率 ${params?.rate}`
        return key
      },
    }),
  }
})

const settings = {
  global_enabled: false,
  ticket_amount: 2,
  minimum_balance: 10,
  daily_play_limit: 3,
  ticket_usage_threshold: 5,
  daily_ticket_limit: 3,
  unit_sale_price: 0.1,
  unit_cost_price: 0.05,
  lucky_coin_enabled: true,
  lucky_coin_double_probability: 50,
  guess_size_enabled: false,
  guess_big_probability: 50,
  guess_small_probability: 50,
}
const prizes = [
  { id: 1, name: '礼遇一档', reward_amount: 1, probability: 60, enabled: true, sort_order: 1 },
  { id: 2, name: '礼遇二档', reward_amount: 3, probability: 30, enabled: true, sort_order: 2 },
]

const AppLayoutStub = { template: '<div><slot /></div>' }
const ToggleStub = defineComponent({
  props: { modelValue: { type: Boolean, required: true } }, emits: ['update:modelValue'],
  setup(props, { emit }) { return () => h('button', { type: 'button', role: 'switch', onClick: () => emit('update:modelValue', !props.modelValue) }, String(props.modelValue)) },
})

function mountView() {
  return mount(ZenxiangLiyuAdminView, { global: { stubs: { AppLayout: AppLayoutStub, Toggle: ToggleStub, Icon: true } } })
}

describe('ZenxiangLiyuAdminView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getSettings.mockResolvedValue({ ...settings })
    api.listPrizes.mockResolvedValue(prizes.map((prize) => ({ ...prize })))
    api.listGrants.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    api.getOverviewStats.mockResolvedValue({ total_plays: 12, total_revenue: 24, total_expense: 18, net_profit: 6, participating_users: 4 })
    api.listUserStats.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    api.listUserRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
    api.listPrizeStats.mockResolvedValue([
      { prize_name: '礼遇一档', prize_id: 1, probability: 60, hit_count: 9, reward_amount: 9 },
      { prize_name: '礼遇二档', prize_id: 2, probability: 30, hit_count: 3, reward_amount: 9 },
    ])
    api.listPeriodStats.mockResolvedValue([{ period_start: '2026-07-11T00:00:00Z', period_label: '2026-07-11', play_count: 12, participant_count: 4, usage_amount: 80, tickets_used: 12, ticket_amount: 0, reward_amount: 18, average_reward: 1.5, user_net_amount: 18, system_revenue: 0, system_expense: 18, system_profit: -18, most_hit_prize_name: '礼遇一档', most_hit_prize_count: 9 }])
    api.resetGrantDailyPlays.mockResolvedValue({ user_id: 42, play_date: '2026-07-11T00:00:00Z', previous_play_count: 3, effective_play_count: 0, remaining_plays: 5 })
    api.updateSettings.mockResolvedValue({ ...settings })
    api.replacePrizes.mockResolvedValue(prizes.map((prize) => ({ ...prize })))
    api.applySimulation.mockResolvedValue(prizes.map((prize) => ({ ...prize })))
    usersAPI.list.mockResolvedValue({ items: [{ id: 42, email: 'user@example.com' }], total: 1, page: 1, page_size: 10, pages: 1 })
  })

  it('validates prize probability total before saving', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-tab-prizes"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('90')
    expect(wrapper.text()).toContain('100')
    expect(wrapper.find('[data-testid="zenxiang-save-prizes"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-testid="zenxiang-save-prizes"]').trigger('click')
    expect(api.replacePrizes).not.toHaveBeenCalled()
  })

  it('saves activity settings', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-save-settings"]').trigger('click')
    await flushPromises()
    expect(api.updateSettings).toHaveBeenCalledWith(settings)
  })

  it('runs simulator and shows profit result', async () => {
    api.simulate.mockResolvedValue({ total_plays: 100, total_revenue: 200, total_expense: 180, net_profit: 20, profit_rate: 0.1, profitable_users: 40, losing_users: 30, break_even_users: 30, prize_hits: [] })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-tab-simulator"]').trigger('click')
    await wrapper.find('[data-testid="zenxiang-simulate"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('20')
  })

  it('applies recommendation as prize configuration only', async () => {
    api.recommend.mockResolvedValue({ target_expense: 1.8, plans: [{ prizes: [{ ...prizes[0], probability: 70 }, { ...prizes[1], probability: 30 }], probability_total: 100, theory_expense: 1.8, theory_profit: 0.2, theory_profit_rate: 0.1 }] })
    api.applySimulation.mockResolvedValue([{ ...prizes[0], probability: 70 }, { ...prizes[1], probability: 30 }])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-tab-simulator"]').trigger('click')
    await wrapper.find('[data-testid="zenxiang-recommend"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="zenxiang-apply-recommendation-0"]').trigger('click')
    await flushPromises()
    expect(api.applySimulation).toHaveBeenCalledWith(expect.arrayContaining([
      expect.objectContaining({ probability: 70 }),
    ]))
  })

  it('searches users by keyword before granting access', async () => {
    const wrapper = mountView()
    await flushPromises()
    const input = wrapper.find('input[type="search"]')
    await input.setValue('user@example.com')
    await new Promise((resolve) => setTimeout(resolve, 350))
    await flushPromises()
    expect(usersAPI.list).toHaveBeenCalledWith(1, 10, { role: 'user', search: 'user@example.com' })
    await wrapper.findAll('button').find((button) => button.text() === 'admin.zenxiangLiyu.grant')?.trigger('click')
    await flushPromises()
    expect(api.createGrant).toHaveBeenCalledWith({ user_id: 42, enabled: true })
  })

  it('loads only user, prize, and grant data for the stats tab', async () => {
    const wrapper = mountView()
    await flushPromises()
    vi.clearAllMocks()

    await wrapper.find('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()

    expect(api.listUserStats).toHaveBeenCalledWith(expect.objectContaining({
      page_size: 100,
      date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
    }))
    expect(api.listPrizeStats).toHaveBeenCalledOnce()
    expect(api.listGrants).toHaveBeenCalledWith({ page_size: 100 })
    expect(api.getOverviewStats).not.toHaveBeenCalled()
    expect(api.listPeriodStats).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('admin.zenxiangLiyu.periodStats')
    expect(wrapper.text()).not.toContain('admin.zenxiangLiyu.totalDraws')
    expect(wrapper.text()).toContain('+15%')
  })

  it('expands, collapses, and switches user draw details', async () => {
    api.listUserStats.mockResolvedValue({
      items: [
        { user_id: 42, user_email: 'first@example.com', balance: 10, usage_amount: 5, play_count: 1, ticket_amount: 0, reward_amount: -0.5, user_net_amount: -0.5 },
        { user_id: 43, user_email: 'second@example.com', balance: 20, usage_amount: 10, play_count: 1, ticket_amount: 0, reward_amount: 2, user_net_amount: 2 },
      ],
      total: 2, page: 1, page_size: 100, pages: 1,
    })
    api.listUserRecords.mockImplementation((userId: number) => Promise.resolve({
      items: [{
        id: userId, request_id: `request-${userId}`, ticket_amount: 0, reward_amount: userId === 42 ? 1 : 2,
        user_net_amount: userId === 42 ? -0.5 : 2, lucky_coin_played: userId === 42,
        lucky_coin_outcome: userId === 42 ? 'zero' : '', lucky_coin_adjustment: userId === 42 ? -1.5 : 0,
        prize_name: userId === 42 ? '礼遇一档' : '礼遇二档', probability: 50, played_at: '2026-07-13T10:00:00Z',
      }],
      total: 1, page: 1, page_size: 100, pages: 1,
    }))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await flushPromises()
    expect(api.listUserRecords).toHaveBeenCalledWith(42, {
      date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), page: 1, page_size: 100,
    })
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('礼遇一档')
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('-1.5')
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('-0.5')

    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    expect(wrapper.find('[data-testid="zenxiang-user-stats-details-42"]').exists()).toBe(false)
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-43"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="zenxiang-user-stats-details-42"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="zenxiang-user-stats-details-43"]').exists()).toBe(true)
  })

  it('keeps statistics tables full width for expanded desktop details', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="zenxiang-stats-tables"]').classes()).not.toContain('xl:grid-cols-2')
  })

  it('clears expanded details and cache when the statistics date changes', async () => {
    api.listUserStats.mockResolvedValue({ items: [{ user_id: 42, user_email: 'user@example.com', balance: 10, usage_amount: 5, play_count: 1, ticket_amount: 0, reward_amount: 1, user_net_amount: 1 }], total: 1, page: 1, page_size: 100, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await flushPromises()

    await wrapper.get('input[type="date"]').setValue('2026-07-12')
    await flushPromises()
    expect(wrapper.find('[data-testid="zenxiang-user-stats-details-42"]').exists()).toBe(false)
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await flushPromises()
    expect(api.listUserRecords).toHaveBeenCalledTimes(2)
    expect(api.listUserRecords).toHaveBeenLastCalledWith(42, { date: '2026-07-12', page: 1, page_size: 100 })
  })

  it('ignores stale user statistics when date requests finish out of order', async () => {
    let resolveFirst: (value: unknown) => void = () => undefined
    let resolveSecond: (value: unknown) => void = () => undefined
    api.listUserStats
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockReturnValueOnce(new Promise((resolve) => { resolveSecond = resolve }))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await wrapper.vm.$nextTick()
    await wrapper.get('input[type="date"]').setValue('2026-07-12')
    await wrapper.vm.$nextTick()

    resolveSecond({ items: [{ user_id: 43, user_email: 'new-date@example.com', balance: 20, usage_amount: 10, play_count: 1, ticket_amount: 0, reward_amount: 2, user_net_amount: 2 }], total: 1, page: 1, page_size: 100, pages: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('new-date@example.com')

    resolveFirst({ items: [{ user_id: 42, user_email: 'stale-date@example.com', balance: 10, usage_amount: 5, play_count: 1, ticket_amount: 0, reward_amount: 1, user_net_amount: 1 }], total: 1, page: 1, page_size: 100, pages: 1 })
    await flushPromises()
    expect(wrapper.text()).toContain('new-date@example.com')
    expect(wrapper.text()).not.toContain('stale-date@example.com')
  })

  it('shows loading and empty states for user draw details', async () => {
    api.listUserStats.mockResolvedValue({ items: [{ user_id: 42, user_email: 'user@example.com', balance: 10, usage_amount: 5, play_count: 1, ticket_amount: 0, reward_amount: 1, user_net_amount: 1 }], total: 1, page: 1, page_size: 100, pages: 1 })
    let resolveRecords: (value: unknown) => void = () => undefined
    api.listUserRecords.mockReturnValue(new Promise((resolve) => { resolveRecords = resolve }))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('admin.zenxiangLiyu.userRecordsLoading')

    resolveRecords({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
    await flushPromises()
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('admin.zenxiangLiyu.userRecordsEmpty')
  })

  it('shows a retry action when user draw details fail to load', async () => {
    api.listUserStats.mockResolvedValue({ items: [{ user_id: 42, user_email: 'user@example.com', balance: 10, usage_amount: 5, play_count: 1, ticket_amount: 0, reward_amount: 1, user_net_amount: 1 }], total: 1, page: 1, page_size: 100, pages: 1 })
    api.listUserRecords.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-tab-stats"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="zenxiang-user-stats-toggle-42"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('admin.zenxiangLiyu.userRecordsLoadFailed')

    await wrapper.get('[data-testid="zenxiang-user-records-retry-42"]').trigger('click')
    await flushPromises()
    expect(api.listUserRecords).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="zenxiang-user-stats-details-42"]').text()).toContain('admin.zenxiangLiyu.userRecordsEmpty')
  })

  it('resets a granted user daily plays', async () => {
    api.listGrants.mockResolvedValue({ items: [{ user_id: 42, user_email: 'user@example.com', enabled: true, notes: '', created_at: '', updated_at: '' }], total: 1, page: 1, page_size: 100, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === 'admin.zenxiangLiyu.resetDailyPlays')?.trigger('click')
    await flushPromises()
    expect(api.resetGrantDailyPlays).toHaveBeenCalledWith(42)
  })
})
