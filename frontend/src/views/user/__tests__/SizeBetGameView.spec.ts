import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import SizeBetGameView from '../SizeBetGameView.vue'
const { getCurrent, getRules, placeBet, showError, showSuccess } = vi.hoisted(() => ({
  getCurrent: vi.fn(),
  getRules: vi.fn(),
  placeBet: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))
vi.mock('@/api', () => ({
  sizeBetAPI: {
    getCurrent,
    getRules,
    placeBet,
  },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))
const messages: Record<string, string | ((params?: Record<string, unknown>) => string)> = {
  'common.loading': '加载中...',
  'common.balance': '余额',
  'common.confirm': '确认',
  'common.retry': '重试',
  'sizeBet.title': '大小中竞猜',
  'sizeBet.heroSubtitle': '封盘前完成选择，等待系统随机开奖',
  'sizeBet.seedTitle': '种子承诺',
  'sizeBet.phase.betting': '下注中',
  'sizeBet.phase.closed': '封盘中',
  'sizeBet.phase.maintenance': '维护中',
  'sizeBet.loadError.badge': '加载失败',
  'sizeBet.loadError.title': '活动加载失败',
  'sizeBet.loadError.description': '请检查网络后重试',
  'sizeBet.countdownLabel': '本局倒计时',
  'sizeBet.betClosesIn': '封盘倒计时',
  'sizeBet.dealer.title': '庄家台',
  'sizeBet.dealer.roundLabel': ({ round }: Record<string, unknown> = {}) => `第 ${round} 期`,
  'sizeBet.dealer.probability': ({ small, mid, big }: Record<string, unknown> = {}) =>
    `概率 小 ${small}% / 中 ${mid}% / 大 ${big}%`,
  'sizeBet.dealer.odds': ({ small, mid, big }: Record<string, unknown> = {}) =>
    `赔率 小 ${small} / 中 ${mid} / 大 ${big}`,
  'sizeBet.dealer.duration': ({ seconds, close }: Record<string, unknown> = {}) =>
    `每局 ${seconds} 秒，前 ${close} 秒可下注`,
  'sizeBet.player.title': '玩家台',
  'sizeBet.player.currentSelection': '当前选择',
  'sizeBet.player.pending': '待下注',
  'sizeBet.player.myBet': ({ direction, stake }: Record<string, unknown> = {}) => `已下注 ${direction} / ${stake}`,
  'sizeBet.player.chooseDirection': '选择方向',
  'sizeBet.player.chooseStake': '选择筹码',
  'sizeBet.player.submit': '确认下注',
  'sizeBet.player.submitting': '提交中...',
  'sizeBet.player.openHint': '现在还在下注窗口内',
  'sizeBet.player.closedHint': '当前已封盘',
  'sizeBet.player.placedHint': '本局已下注',
  'sizeBet.player.selectDirection': '请选择方向',
  'sizeBet.player.selectStake': '请选择金额',
  'sizeBet.player.placedSuccess': '下注成功',
  'sizeBet.rules.title': '活动规则',
  'sizeBet.previousRound.title': '上期开奖结果',
  'sizeBet.previousRound.empty': '暂无最近开奖',
  'sizeBet.previousRound.reveal': ({ seed }: Record<string, unknown> = {}) => `服务端种子：${seed}`,
  'sizeBet.previousRound.result': ({ round, number, direction }: Record<string, unknown> = {}) =>
    `第 ${round} 期 ${number} / ${direction}`,
  'sizeBet.maintenance.title': '活动暂未开启',
  'sizeBet.maintenance.description': '管理员已关闭该活动',
  'sizeBet.directions.small': '小',
  'sizeBet.directions.mid': '中',
  'sizeBet.directions.big': '大',
}
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const value = messages[key]
        if (typeof value === 'function') {
          return value(params)
        }
        return value ?? key
      },
    }),
  }
})
const AppLayoutStub = { template: '<div><slot /></div>' }
const LoadingSpinnerStub = { template: '<div>loading</div>' }
const EmptyStateStub = {
  props: ['title', 'description', 'actionText'],
  emits: ['action'],
  template:
    '<div data-test="empty-state"><div>{{ title }}</div><div>{{ description }}</div><button v-if="actionText" data-test="empty-action" @click="$emit(\'action\')">{{ actionText }}</button><slot /></div>',
}
function buildCurrentView(overrides: Record<string, any> = {}) {
  return {
    enabled: true,
    phase: 'betting',
    server_time: '2026-04-23T12:00:10Z',
    round: {
      id: 1001,
      round_no: 1001,
      status: 'open',
      starts_at: '2026-04-23T12:00:00Z',
      bet_closes_at: '2026-04-23T12:00:18Z',
      settles_at: '2026-04-23T12:00:28Z',
      prob_small: 45,
      prob_mid: 10,
      prob_big: 45,
      odds_small: 2,
      odds_mid: 10,
      odds_big: 2,
      allowed_stakes: [2, 5, 10, 20],
      server_seed_hash: 'hash-1001',
      countdown_seconds: 18,
      bet_countdown_seconds: 8,
    },
    my_bet: null,
    previous_round: null,
    ...overrides,
  }
}
function mountView() {
  return mount(SizeBetGameView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        LoadingSpinner: LoadingSpinnerStub,
        EmptyState: EmptyStateStub,
      },
    },
  })
}
function mockRules() {
  getRules.mockResolvedValue({
    enabled: true,
    round_duration_seconds: 60,
    bet_close_offset_seconds: 50,
    allowed_stakes: [2, 5, 10, 20],
    probabilities: { small: 45, mid: 10, big: 45 },
    odds: { small: 2, mid: 10, big: 2 },
    rules_markdown: '## 规则\n\n- 这里是测试规则',
  })
}
describe('SizeBetGameView', () => {
  beforeEach(() => {
    getCurrent.mockReset()
    getRules.mockReset()
    placeBet.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })
  it('recomputes countdown from timestamps and catches up after timer advance', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-23T12:00:10Z'))
    getCurrent.mockResolvedValue(
      buildCurrentView({
        round: {
          ...buildCurrentView().round,
          countdown_seconds: 99,
          bet_countdown_seconds: 77,
        },
      })
    )
    mockRules()
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('18')
    expect(wrapper.text()).toContain('8')
    expect(wrapper.text()).not.toContain('99')
    expect(wrapper.text()).not.toContain('77')
    await vi.advanceTimersByTimeAsync(5000)
    await nextTick()
    expect(wrapper.text()).toContain('13')
    expect(wrapper.text()).toContain('3')
  })
  it('shows authoritative bet state and disables all controls when user already has a bet', async () => {
    getCurrent.mockResolvedValue(
      buildCurrentView({
        my_bet: {
          id: 501,
          round_id: 1001,
          direction: 'big',
        stake_amount: 10,
        payout_amount: 0,
        net_result_amount: 0,
          status: 'placed',
          placed_at: '2026-04-23T12:00:03Z',
        },
      })
    )
    mockRules()
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('10')
    expect(wrapper.text()).toContain('已下注 大 / 10')
    expect(wrapper.find('[data-test="bet-submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('[data-test^="direction-"]').every(button => button.attributes('disabled') !== undefined)).toBe(true)
    expect(wrapper.findAll('[data-test^="stake-"]').every(button => button.attributes('disabled') !== undefined)).toBe(true)
  })
  it('disables direction and stake controls together with submit after close', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-23T12:00:10Z'))
    getCurrent.mockResolvedValue(
      buildCurrentView({
        round: {
          ...buildCurrentView().round,
          bet_closes_at: '2026-04-23T12:00:11Z',
          settles_at: '2026-04-23T12:00:13Z',
          countdown_seconds: 3,
          bet_countdown_seconds: 1,
        },
      })
    )
    mockRules()
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-test="direction-big"]').trigger('click')
    await wrapper.find('[data-test="stake-10"]').trigger('click')
    expect(wrapper.find('[data-test="bet-submit"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="direction-big"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-test="stake-10"]').attributes('disabled')).toBeUndefined()
    await vi.advanceTimersByTimeAsync(1000)
    await nextTick()
    expect(wrapper.find('[data-test="bet-submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.findAll('[data-test^="direction-"]').every(button => button.attributes('disabled') !== undefined)).toBe(true)
    expect(wrapper.findAll('[data-test^="stake-"]').every(button => button.attributes('disabled') !== undefined)).toBe(true)
  })
  it('shows load error state instead of maintenance when initial load fails', async () => {
    getCurrent.mockRejectedValue(new Error('load current failed'))
    mockRules()
    const wrapper = mountView()
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    expect(wrapper.text()).toContain('活动加载失败')
    expect(wrapper.text()).toContain('重试')
    expect(wrapper.text()).not.toContain('活动暂未开启')
  })
  it('recovers from maintenance mode after a later successful poll', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-23T12:00:10Z'))
    getCurrent
      .mockResolvedValueOnce(
        buildCurrentView({
          enabled: false,
          phase: 'maintenance',
          round: null,
        })
      )
      .mockResolvedValueOnce(
        buildCurrentView({
          round: {
            ...buildCurrentView().round,
            id: 1003,
            round_no: 1003,
            server_seed_hash: 'hash-1003',
          },
        })
      )
    mockRules()
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('活动暂未开启')
    await vi.advanceTimersByTimeAsync(15000)
    await nextTick()
    await flushPromises()
    expect(getCurrent).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('1003')
    expect(wrapper.text()).not.toContain('活动暂未开启')
  })
})
