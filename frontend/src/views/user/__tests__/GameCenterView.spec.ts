import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

const {
  getOverview,
  claimPoints,
  exchangeBalanceToPoints,
  exchangePointsToBalance,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  getOverview: vi.fn(),
  claimPoints: vi.fn(),
  exchangeBalanceToPoints: vi.fn(),
  exchangePointsToBalance: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/gameCenter', () => ({
  gameCenterAPI: {
    getOverview,
    claimPoints,
    exchangeBalanceToPoints,
    exchangePointsToBalance,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    gameCenterEnabled: true,
    cachedPublicSettings: { game_center_enabled: true },
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { username: 'tester' },
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'gameCenter.hero.tag': 'GAME CENTER',
    'gameCenter.hero.title': '游戏中心',
    'gameCenter.hero.subtitle': '当前积分',
    'gameCenter.exchange.entry': '积分兑换',
    'gameCenter.claim.title': '每日领取',
    'gameCenter.claim.status.claimable': '可领取',
    'gameCenter.claim.status.claimed': '已领取',
    'gameCenter.launch.quick': '快速开始',
    'gameCenter.launch.fullscreen': '全屏打开',
    'gameCenter.embed.title': '快速开始窗口',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }

function mockOverview() {
  getOverview.mockResolvedValue({
    points: 2460,
    claim_batches: [
      { batch_key: 'night', status: 'claimable', points_amount: 100, claim_time: '20:00' },
      { batch_key: 'morning', status: 'claimed', points_amount: 30, claim_time: '08:00' },
    ],
    exchange: {
      balance_to_points_enabled: true,
      points_to_balance_enabled: true,
      balance_to_points_rate: 100,
      points_to_balance_rate: 0.01,
      min_balance_amount: 1,
      min_points_amount: 100,
    },
    catalogs: [
      {
        game_key: 'size_bet',
        name: '猜大小',
        subtitle: '经典快节奏竞猜',
        default_open_mode: 'dual',
      },
    ],
    recent_ledger: [],
  })
}

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/game-center', component: { template: '<div />' } },
      { path: '/game-center/:gameKey', component: { template: '<div />' } },
    ],
  })
  await router.push('/game-center')
  await router.isReady()

  const { default: GameCenterView } = await import('../GameCenterView.vue')
  const wrapper = mount(GameCenterView, {
    global: {
      plugins: [router],
      stubs: {
        AppLayout: AppLayoutStub,
      },
    },
  })
  await flushPromises()
  return { wrapper, router }
}

beforeEach(() => {
  getOverview.mockReset()
  claimPoints.mockReset()
  exchangeBalanceToPoints.mockReset()
  exchangePointsToBalance.mockReset()
  showSuccess.mockReset()
  showError.mockReset()
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
  })
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false }))
  mockOverview()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('GameCenterView', () => {
  it('renders points hero, claim status, exchange entry and dual actions', async () => {
    const { wrapper } = await mountView()
    expect(wrapper.text()).toContain('2,460')
    expect(wrapper.text()).toContain('积分兑换')
    expect(wrapper.text()).toContain('可领取')
    expect(wrapper.text()).toContain('已领取')
    expect(wrapper.text()).toContain('快速开始')
    expect(wrapper.text()).toContain('全屏打开')
  })

  it('opens embedded game on desktop quick start', async () => {
    const { wrapper } = await mountView()
    await wrapper.get('[data-test="quick-start-size_bet"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="embedded-frame"]').attributes('src')).toBe('/game/size-bet')
  })

  it('redirects to game shell on mobile quick start', async () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))
    const { wrapper, router } = await mountView()
    await wrapper.get('[data-test="quick-start-size_bet"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/game-center/size_bet')
  })
})
