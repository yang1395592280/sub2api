import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GameCenterView from '../GameCenterView.vue'

const { getOverview, getLedger, exchangeBalanceToPoints, exchangePointsToBalance } = vi.hoisted(() => ({
  getOverview: vi.fn(), getLedger: vi.fn(), exchangeBalanceToPoints: vi.fn(), exchangePointsToBalance: vi.fn()
}))

vi.mock('@/api/gameCenter', () => ({ gameCenterAPI: { getOverview, getLedger, exchangeBalanceToPoints, exchangePointsToBalance } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 999, welfare_balance: 888, points: 100 }, refreshUser: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: {}, showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const overview = () => ({
  points: 100,
  welfare_balance: 2.3456,
  claim_batches: [], catalogs: [], recent_ledger: [],
  exchange: {
    balance_to_points_enabled: true, points_to_balance_enabled: true,
    balance_to_points_rate: 37, points_to_balance_rate: 0.025,
    min_balance_amount: 0.1, min_points_amount: 4
  }
})

async function mountExchange() {
  const wrapper = mount(GameCenterView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' }, RouterLink: true, BaseDialog: true, Pagination: true,
    LoadingSpinner: true, EmptyState: true
  } } })
  await flushPromises()
  await wrapper.get('[data-test="exchange-entry"]').trigger('click')
  return wrapper
}

describe('Game Center exchanges', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOverview.mockResolvedValue(overview())
    getLedger.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
    exchangeBalanceToPoints.mockResolvedValue({})
    exchangePointsToBalance.mockResolvedValue({})
  })

  it('uses authoritative welfare assets, configured rates and matching round rules', async () => {
    const wrapper = await mountExchange()
    expect(wrapper.text()).toContain('¥2.35')
    expect(wrapper.text()).not.toContain('888')
    expect(wrapper.text()).toContain('汇率 1 元 = 37 积分')
    expect(wrapper.text()).toContain('汇率 40 积分 = 1 元')
    const [balance, points] = wrapper.findAll('input[type="number"]')
    expect(balance.attributes('min')).toBe('0.1')
    expect(points.attributes('min')).toBe('4')
    await balance.setValue(0.1)
    expect(wrapper.text()).toContain('预计获得积分: 4')
    await points.setValue(20)
    expect(wrapper.text()).toContain('预计获得福利余额: ¥0.50')
    const all = wrapper.findAll('button').filter(button => button.text() === '全部')[0]
    await all.trigger('click')
    expect((balance.element as HTMLInputElement).value).toBe('2.34')
    wrapper.unmount()
  })

  it('enforces configured minimums and submits the chosen points', async () => {
    const wrapper = await mountExchange()
    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[1].setValue(3)
    const submit = wrapper.findAll('button').find(button => button.text() === '确认变现为福利余额')!
    expect(submit.attributes('disabled')).toBeDefined()
    await inputs[1].setValue(4)
    expect(submit.attributes('disabled')).toBeUndefined()
    await submit.trigger('click')
    await flushPromises()
    expect(exchangePointsToBalance).toHaveBeenCalledWith({ points: 4 })
    wrapper.unmount()
  })

  it('does not enable exchange using a fabricated fallback rate', async () => {
    const data = overview()
    data.exchange.balance_to_points_rate = 0
    data.exchange.points_to_balance_rate = 0
    getOverview.mockResolvedValue(data)
    const wrapper = await mountExchange()
    const submit = wrapper.findAll('button').filter(button => ['确认变现为福利余额', '确认划转至游戏筹码'].includes(button.text()))
    expect(submit).toHaveLength(2)
    expect(submit.every(button => button.attributes('disabled') !== undefined)).toBe(true)
    wrapper.unmount()
  })
})
