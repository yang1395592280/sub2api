import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import SizeBetGameView from '../SizeBetGameView.vue'
const { getCurrent, getRules, getHistory, getRounds, placeBet, getOverview, showError, showSuccess, showWarning } = vi.hoisted(() => ({
  getCurrent: vi.fn(),
  getRules: vi.fn(),
  getHistory: vi.fn(),
  getRounds: vi.fn(),
  placeBet: vi.fn(),
  getOverview: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}))
vi.mock('@/api', () => ({
  gameCenterAPI: {
    getOverview,
  },
  sizeBetAPI: {
    getCurrent,
    getRules,
    getHistory,
    getRounds,
    placeBet,
  },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning,
  }),
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 42 },
  }),
}))
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))
const messages: Record<string, string | ((params?: Record<string, unknown>) => string)> = {
  'common.loading': '加载中...',
  'common.balance': '积分',
  'common.confirm': '确认',
  'common.retry': '重试',
  'sizeBet.title': '猜大小游戏',
  'sizeBet.heroSubtitle': '在截止前完成选择，等待系统随机开奖',
  'sizeBet.seedTitle': '种子承诺',
  'sizeBet.phase.betting': '参与中',
  'sizeBet.phase.closed': '等待开奖',
  'sizeBet.phase.preparing': '准备中',
  'sizeBet.phase.maintenance': '维护中',
  'sizeBet.countdownHint.betting': ({ seconds }: Record<string, unknown> = {}) => `距离封盘还有 ${seconds} 秒`,
  'sizeBet.countdownHint.closed': '本局已封盘，等待系统开奖',
  'sizeBet.countdownHint.preparing': '下一局准备中',
  'sizeBet.loadError.badge': '加载失败',
  'sizeBet.loadError.title': '活动加载失败',
  'sizeBet.loadError.description': '请检查网络后重试',
  'sizeBet.countdownLabel': '本局倒计时',
  'sizeBet.betClosesIn': '参与倒计时',
  'sizeBet.dealer.title': '系统开奖区',
  'sizeBet.dealer.roundLabel': ({ round }: Record<string, unknown> = {}) => `第 ${round} 期`,
  'sizeBet.dealer.probability': ({ small, mid, big }: Record<string, unknown> = {}) =>
    `概率 小 ${small}% / 中 ${mid}% / 大 ${big}%`,
  'sizeBet.dealer.odds': ({ small, mid, big }: Record<string, unknown> = {}) =>
    `赔率 小 ${small} / 中 ${mid} / 大 ${big}`,
  'sizeBet.dealer.duration': ({ seconds, close }: Record<string, unknown> = {}) => `每局 ${seconds} 秒，前 ${close} 秒可参与`,
  'sizeBet.player.title': '我的选择',
  'sizeBet.player.currentSelection': '当前选择',
  'sizeBet.player.pending': '待参与',
  'sizeBet.player.myBet': ({ direction, stake }: Record<string, unknown> = {}) => `已参与 ${direction} / ${stake}`,
  'sizeBet.player.chooseDirection': '选择方向',
  'sizeBet.player.chooseStake': '选择参与额度',
  'sizeBet.player.submit': '确认参与',
  'sizeBet.player.submitting': '提交中...',
  'sizeBet.player.openHint': '当前还在参与时间内',
  'sizeBet.player.closedHint': '当前已封盘',
  'sizeBet.player.placedHint': '本局已参与',
  'sizeBet.player.selectDirection': '请选择方向',
  'sizeBet.player.selectStake': '请选择金额',
  'sizeBet.player.placedSuccess': '参与成功',
  'sizeBet.rules.title': '活动规则',
  'sizeBet.previousRound.title': '上期开奖结果',
  'sizeBet.previousRound.empty': '暂无最近开奖',
  'sizeBet.previousRound.reveal': ({ seed }: Record<string, unknown> = {}) => `服务端种子：${seed}`,
  'sizeBet.previousRound.result': ({ round, number, direction }: Record<string, unknown> = {}) =>
    `第 ${round} 期 ${number} / ${direction}`,
  'sizeBet.rounds.title': '开奖结果',
  'sizeBet.rounds.empty': '暂无开奖记录',
  'sizeBet.rounds.roundLabel': ({ round }: Record<string, unknown> = {}) => `第 ${round} 期`,
  'sizeBet.rounds.result': ({ number, direction }: Record<string, unknown> = {}) => `结果：${number} / ${direction}`,
  'sizeBet.history.title': '我的参与记录',
  'sizeBet.history.subtitle': '查看最近记录与开奖结果',
  'sizeBet.history.refreshFailed': '最新记录暂未同步，请稍后重试',
  'sizeBet.history.refreshRetry': '重试同步',
  'sizeBet.history.toggleMore': '展开更多',
  'sizeBet.history.toggleLess': '收起记录',
  'sizeBet.history.empty': '暂无参与记录',
  'sizeBet.history.roundLabel': ({ round }: Record<string, unknown> = {}) => `第 ${round} 期`,
  'sizeBet.history.selection': ({ direction }: Record<string, unknown> = {}) => `我的选择：${direction}`,
  'sizeBet.history.result': ({ number, direction }: Record<string, unknown> = {}) => `开奖结果：${number} / ${direction}`,
  'sizeBet.history.pendingResult': '开奖结果待同步',
  'sizeBet.history.status.placed': '待结算',
  'sizeBet.history.status.won': '已获得奖励',
  'sizeBet.history.status.lost': '未获得奖励',
  'sizeBet.history.status.refunded': '已退回',
  'sizeBet.history.pendingAmount': '待开奖',
  'sizeBet.history.refundedAmount': ({ amount }: Record<string, unknown> = {}) => `已退回 ${amount}`,
  'sizeBet.resultModal.title': '结果通知',
  'sizeBet.resultModal.close': '知道了',
  'sizeBet.resultModal.roundLabel': ({ round }: Record<string, unknown> = {}) => `第 ${round} 期`,
  'sizeBet.resultModal.result': ({ number, direction }: Record<string, unknown> = {}) => `本期结果：${number} / ${direction}`,
  'sizeBet.resultModal.selection': ({ direction, stake }: Record<string, unknown> = {}) => `你的选择：${direction} / ${stake}`,
  'sizeBet.resultModal.summary.won': ({ amount }: Record<string, unknown> = {}) => `本次获得奖励 ${amount}`,
  'sizeBet.resultModal.summary.lost': ({ amount }: Record<string, unknown> = {}) => `本次结果 ${amount}`,
  'sizeBet.resultModal.summary.refunded': ({ amount }: Record<string, unknown> = {}) => `本次已退回 ${amount}`,
  'sizeBet.resultModal.message.won': '系统已完成本局结算，你获得了奖励。',
  'sizeBet.resultModal.message.lost': '系统已完成本局结算，本次未获得奖励。',
  'sizeBet.resultModal.message.refunded': '本局已退回参与额度，请留意后续开放时间。',
  'sizeBet.resultNotice.title': '本局结果',
  'sizeBet.resultNotice.subtitle': '系统会在这里固定展示最近一次结算结果',
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
const BaseDialogStub = {
  props: ['show', 'title'],
  template:
    '<div v-if="show" data-test="result-modal"><div>{{ title }}</div><slot /><slot name="footer" /></div>',
}
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
        BaseDialog: BaseDialogStub,
        LoadingSpinner: LoadingSpinnerStub,
        EmptyState: EmptyStateStub,
      },
    },
  })
}
function buildHistoryItem(overrides: Record<string, any> = {}) {
  return {
    bet_id: 1,
    round_no: 1001,
    direction: 'big',
    selection: 'big',
    result_number: 9,
    result_direction: 'big',
    stake_amount: 10,
    payout_amount: 20,
    net_result_amount: 10,
    points_after: 110,
    status: 'won',
    placed_at: '2026-04-23T12:00:10Z',
    settled_at: '2026-04-23T12:00:55Z',
    ...overrides,
  }
}
function mockRules() {
  getRules.mockResolvedValue({
    enabled: true,
    round_duration_seconds: 60,
    bet_close_offset_seconds: 50,
    allowed_stakes: [2, 5, 10, 20],
    custom_stake_min: 1,
    custom_stake_max: 9999,
    probabilities: { small: 45, mid: 10, big: 45 },
    odds: { small: 2, mid: 10, big: 2 },
    rules_markdown: '## 规则\n\n- 这里是测试规则',
  })
}
function mockHistory(payload?: Record<string, any>) {
  getHistory.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 10,
    pages: 0,
    ...payload,
  })
}
function mockRounds(payload?: Record<string, any>) {
  getRounds.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 5,
    pages: 1,
    ...payload,
  })
}
describe('SizeBetGameView', () => {
  beforeEach(() => {
    getCurrent.mockReset()
    getRules.mockReset()
    getHistory.mockReset()
    getRounds.mockReset()
    placeBet.mockReset()
    getOverview.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    showWarning.mockReset()
    sessionStorage.clear()
    getOverview.mockResolvedValue({ points: 321 })
    mockRounds()
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
    mockHistory()
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
    mockHistory()
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('10')
    expect(wrapper.text()).toContain('已参与 大 / 10')
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
    mockHistory()
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
    mockHistory()
    const wrapper = mountView()
    await flushPromises()
    expect(showError).toHaveBeenCalled()
    expect(wrapper.text()).toContain('活动加载失败')
    expect(wrapper.text()).toContain('重试')
    expect(wrapper.text()).not.toContain('活动暂未开启')
  })
  it('keeps maintenance empty state and recovers after a later successful poll', async () => {
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
    mockHistory()
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
  it('shows a result modal and recent records after settlement', async () => {
    getCurrent.mockResolvedValueOnce(
      buildCurrentView({
        phase: 'closed',
        server_time: '2026-04-23T12:00:55Z',
        my_bet: {
          id: 1,
          round_id: 1001,
          direction: 'big',
          stake_amount: 10,
          payout_amount: 0,
          net_result_amount: 0,
          status: 'placed',
          placed_at: '2026-04-23T12:00:10Z',
        },
      })
    )
    mockRules()
    mockHistory({
      items: [buildHistoryItem()],
      total: 1,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('我的参与记录')
    expect(wrapper.text()).toContain('已获得奖励')
    expect(wrapper.text()).toContain('+10')
    expect(wrapper.text()).toContain('本局结果')
    expect(wrapper.text()).toContain('本次获得奖励 +10')
  })

  it('refreshes displayed points balance after settlement sync updates user points', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-23T12:00:10Z'))
    getCurrent
      .mockResolvedValueOnce(
        buildCurrentView({
          phase: 'closed',
          round: {
            ...buildCurrentView().round,
            bet_closes_at: '2026-04-23T12:00:09Z',
            settles_at: '2026-04-23T12:00:10Z',
            countdown_seconds: 0,
            bet_countdown_seconds: 0,
          },
          my_bet: {
            id: 1,
            round_id: 1001,
            direction: 'big',
            stake_amount: 10,
            payout_amount: 0,
            net_result_amount: 0,
            status: 'placed',
            placed_at: '2026-04-23T12:00:10Z',
          },
        })
      )
      .mockResolvedValueOnce(
        buildCurrentView({
          phase: 'preparing',
          round: null,
          my_bet: null,
          previous_round: {
            id: 1001,
            round_no: 1001,
            status: 'settled',
            starts_at: '2026-04-23T12:00:00Z',
            settles_at: '2026-04-23T12:00:10Z',
            result_number: 9,
            result_direction: 'big',
            server_seed_hash: 'hash-1001',
            server_seed: 'seed-1001',
          },
        })
      )
    mockRules()
    getHistory
      .mockResolvedValueOnce({
        items: [buildHistoryItem({ status: 'placed', net_result_amount: 0, payout_amount: 0, result_number: null, result_direction: null, settled_at: null, points_after: 321 })],
        total: 1,
        page: 1,
        page_size: 10,
        pages: 1,
      })
      .mockResolvedValueOnce({
        items: [buildHistoryItem({ points_after: 654 })],
        total: 1,
        page: 1,
        page_size: 10,
        pages: 1,
      })
    getOverview
      .mockResolvedValueOnce({ points: 321 })
      .mockResolvedValueOnce({ points: 654 })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('321')

    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()

    expect(getOverview).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('654')
  })

  it('does not replay the latest settled result modal after remount when it was already seen', async () => {
    getCurrent.mockResolvedValue(
      buildCurrentView({
        phase: 'closed',
        server_time: '2026-04-23T12:00:55Z',
      })
    )
    mockRules()
    mockHistory({
      items: [buildHistoryItem()],
      total: 1,
      pages: 1,
    })

    const firstWrapper = mountView()
    await flushPromises()
    expect(firstWrapper.text()).toContain('本局结果')

    firstWrapper.unmount()

    const secondWrapper = mountView()
    await flushPromises()
    expect(secondWrapper.text()).not.toContain('本次获得奖励 +10')
  })

  it('surfaces history refresh failure with retry affordance instead of swallowing it silently', async () => {
    getCurrent.mockResolvedValue(buildCurrentView())
    mockRules()
    getHistory
      .mockRejectedValueOnce(new Error('history fetch failed'))
      .mockResolvedValueOnce({
        items: [buildHistoryItem({ status: 'placed', net_result_amount: 0, payout_amount: 0, result_number: null, result_direction: null, settled_at: null })],
        total: 1,
        page: 1,
        page_size: 10,
        pages: 1,
      })

    const wrapper = mountView()
    await flushPromises()

    expect(showWarning).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('最新记录暂未同步，请稍后重试')
    const retryButton = wrapper.find('[data-test="history-retry"]')
    expect(retryButton.exists()).toBe(true)

    await retryButton.trigger('click')
    await flushPromises()

    expect(getHistory).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('最新记录暂未同步，请稍后重试')
    expect(wrapper.text()).toContain('待开奖')
    expect(wrapper.text()).not.toContain('+0')
  })

  it('renders refunded history rows with a returned-amount label instead of +0', async () => {
    getCurrent.mockResolvedValue(buildCurrentView())
    mockRules()
    mockHistory({
      items: [buildHistoryItem({
        status: 'refunded',
        net_result_amount: 0,
        payout_amount: 0,
        result_number: null,
        result_direction: null,
        stake_amount: 10,
        settled_at: '2026-04-23T12:00:55Z',
      })],
      total: 1,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('已退回 10')
    expect(wrapper.text()).not.toContain('+0')
  })

  it('allows entering a custom stake amount within configured bounds', async () => {
    getCurrent.mockResolvedValue(buildCurrentView())
    mockRules()
    mockHistory()
    mockRounds()

    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-test="direction-big"]').trigger('click')
    const customStakeInput = wrapper.find('[data-test="custom-stake"]')
    await customStakeInput.setValue('33')
    await nextTick()

    expect(wrapper.text()).toContain('大 / 33')
    expect(wrapper.find('[data-test="bet-submit"]').attributes('disabled')).toBeUndefined()
  })

  it('shows preparing phase without active countdown after round settlement', async () => {
    getCurrent.mockResolvedValue({
      enabled: true,
      phase: 'preparing',
      server_time: '2026-04-23T12:01:05Z',
      round: null,
      my_bet: null,
      previous_round: null,
    })
    mockRules()
    mockHistory()
    mockRounds()

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('准备中')
    expect(wrapper.text()).toContain('下一局准备中')
    expect(wrapper.text()).toContain('--')
  })
})
