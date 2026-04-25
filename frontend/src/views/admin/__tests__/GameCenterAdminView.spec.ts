import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const {
  getSettings,
  updateSettings,
  getCatalog,
  updateCatalog,
  listLedger,
  listClaims,
  listExchanges,
  adjustPoints,
  searchUsers,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getCatalog: vi.fn(),
  updateCatalog: vi.fn(),
  listLedger: vi.fn(),
  listClaims: vi.fn(),
  listExchanges: vi.fn(),
  adjustPoints: vi.fn(),
  searchUsers: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

const storageStub = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn()
}

vi.stubGlobal('localStorage', storageStub)
vi.stubGlobal('sessionStorage', storageStub)

vi.mock('@/api/admin/gameCenter', () => ({
  getSettings,
  updateSettings,
  getCatalog,
  updateCatalog,
  listLedger,
  listClaims,
  listExchanges,
  adjustPoints,
  default: {
    getSettings,
    updateSettings,
    getCatalog,
    updateCatalog,
    listLedger,
    listClaims,
    listExchanges,
    adjustPoints
  }
}))

vi.mock('@/api/admin/usage', () => ({
  searchUsers,
  default: {
    searchUsers
  }
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN'
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError,
    fetchPublicSettings: vi.fn().mockResolvedValue(null)
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const dict: Record<string, string> = {
          'admin.gameCenter.title': '游戏中心管理',
          'admin.gameCenter.sections.claim': '每日领取配置',
          'admin.gameCenter.sections.exchange': '兑换配置',
          'admin.gameCenter.sections.catalog': '游戏目录配置',
          'admin.gameCenter.catalog.openGameSettings': '进入游戏配置',
          'admin.gameCenter.exchange.balanceToPoints': '余额兑积分',
          'admin.gameCenter.exchange.pointsToBalance': '积分兑余额',
          'common.save': '保存'
        }
        return dict[key] ?? key
      }
    })
  }
})

function buildSettings() {
  return {
    game_center_enabled: true,
    claim_enabled: true,
    claim_schedule: [
      { batch_key: 'night', claim_time: '20:00', points_amount: 100, enabled: true }
    ],
    exchange: {
      balance_to_points_enabled: true,
      points_to_balance_enabled: true,
      balance_to_points_rate: 100,
      points_to_balance_rate: 0.01,
      min_balance_amount: 1,
      min_points_amount: 100
    }
  }
}

function buildCatalog() {
  return [
    {
      game_key: 'size_bet',
      name: '猜大小',
      subtitle: '经典快节奏竞猜',
      description: '现有猜大小游戏接入游戏中心',
      enabled: true,
      sort_order: 0,
      default_open_mode: 'dual',
      supports_embed: true,
      supports_standalone: true
    }
  ]
}

function buildPaginated(items: any[]) {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 20,
    pages: 1
  }
}

async function mountView() {
  const { default: GameCenterAdminView } = await import('../GameCenterAdminView.vue')
  return mount(GameCenterAdminView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' },
        Toggle: {
          props: ['modelValue', 'disabled'],
          emits: ['update:modelValue'],
          template: `
            <input
              data-test="toggle"
              type="checkbox"
              :checked="modelValue"
              :disabled="disabled"
              @change="$emit('update:modelValue', $event.target.checked)"
            />
          `
        }
      }
    }
  })
}

describe('GameCenterAdminView', () => {
  beforeEach(() => {
    storageStub.getItem.mockClear()
    storageStub.setItem.mockClear()
    storageStub.removeItem.mockClear()
    storageStub.clear.mockClear()

    getSettings.mockReset()
    updateSettings.mockReset()
    getCatalog.mockReset()
    updateCatalog.mockReset()
    listLedger.mockReset()
    listClaims.mockReset()
    listExchanges.mockReset()
    adjustPoints.mockReset()
    searchUsers.mockReset()
    showSuccess.mockReset()
    showError.mockReset()

    getSettings.mockResolvedValue(buildSettings())
    getCatalog.mockResolvedValue(buildCatalog())
    updateSettings.mockResolvedValue({ message: 'ok' })
    updateCatalog.mockResolvedValue({ message: 'ok' })
    listLedger.mockResolvedValue(buildPaginated([{ id: 1, user_id: 7, entry_type: 'admin_adjust', delta_points: 20, points_before: 80, points_after: 100, reason: '运营补发', related_game_key: '', created_at: '2026-04-25T10:00:00Z' }]))
    listClaims.mockResolvedValue(buildPaginated([{ id: 1, user_id: 7, claim_date: '2026-04-25', batch_key: 'night', points_amount: 100, claimed_at: '2026-04-25T20:00:00Z' }]))
    listExchanges.mockResolvedValue(buildPaginated([{ id: 1, user_id: 7, direction: 'balance_to_points', source_amount: 1, source_points: 0, target_amount: 0, target_points: 100, rate: 100, status: 'completed', reason: '兑换', created_at: '2026-04-25T09:00:00Z' }]))
    adjustPoints.mockResolvedValue({ message: 'ok' })
    searchUsers.mockResolvedValue([{ id: 7, email: 'user@example.com', username: '测试用户' }])
  })

  it('loads claim settings, exchange settings and catalog sections', async () => {
    const wrapper = await mountView()
    await flushPromises()

    expect(getSettings).toHaveBeenCalledTimes(1)
    expect(getCatalog).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="claim-config-section"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="exchange-config-section"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="catalog-config-section"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="operations-section"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="audit-section"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('进入游戏配置')
    expect((wrapper.get('[data-test="claim-time-0"]').element as HTMLInputElement).value).toBe('20:00')
    expect(wrapper.text()).toContain('余额兑积分')
    expect(wrapper.text()).toContain('猜大小')
    expect(wrapper.text()).toContain('运营补发')
  })

  it('saves settings and catalog changes', async () => {
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.get('[data-test="save-settings"]').trigger('click')
    await wrapper.get('[data-test="save-catalog-size_bet"]').trigger('click')
    await flushPromises()

    expect(updateSettings).toHaveBeenCalledTimes(1)
    expect(updateCatalog).toHaveBeenCalledWith(
      'size_bet',
      expect.objectContaining({
        enabled: true
      })
    )
    expect(showSuccess).toHaveBeenCalled()
  })

  it('submits manual points adjustment', async () => {
    const wrapper = await mountView()
    await flushPromises()

    ;(wrapper.vm as any).selectAdjustUser({ id: 7, email: 'user@example.com', username: '测试用户' })
    await wrapper.get('[data-test="adjust-delta"]').setValue('25')
    await wrapper.get('[data-test="adjust-reason"]').setValue('补偿')
    await wrapper.get('[data-test="submit-adjust"]').trigger('click')
    await flushPromises()

    expect(adjustPoints).toHaveBeenCalledWith(7, {
      delta_points: 25,
      reason: '补偿'
    })
  })
})
