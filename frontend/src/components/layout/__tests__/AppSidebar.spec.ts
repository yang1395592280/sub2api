import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

const storeMocks = vi.hoisted(() => ({
  appStore: {
    sidebarCollapsed: false,
    mobileOpen: true,
    backendModeEnabled: false,
    siteName: 'Test Site',
    siteLogo: '',
    siteVersion: '1.0.0',
    publicSettingsLoaded: true,
    cachedPublicSettings: {
      payment_enabled: false,
      custom_menu_items: []
    },
    toggleSidebar: vi.fn(),
    setMobileOpen: vi.fn()
  },
  authStore: {
    isAdmin: true,
    isSimpleMode: false
  },
  onboardingStore: {
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn()
  },
  adminSettingsStore: {
    opsMonitoringEnabled: true,
    paymentEnabled: false,
    customMenuItems: [],
    sizeBetEnabled: true,
    fetch: vi.fn()
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => storeMocks.appStore,
  useAuthStore: () => storeMocks.authStore,
  useOnboardingStore: () => storeMocks.onboardingStore,
  useAdminSettingsStore: () => storeMocks.adminSettingsStore
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

async function mountSidebar() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/dashboard', component: { template: '<div />' } },
      { path: '/:pathMatch(.*)*', component: { template: '<div />' } }
    ]
  })
  await router.push('/admin/dashboard')
  await router.isReady()

  const { default: AppSidebar } = await import('../AppSidebar.vue')

  const wrapper = mount(AppSidebar, {
    global: {
      plugins: [router],
      stubs: {
        VersionBadge: { template: '<div data-test="version-badge" />' }
      }
    }
  })

  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  storeMocks.appStore.sidebarCollapsed = false
  storeMocks.appStore.mobileOpen = true
  storeMocks.appStore.backendModeEnabled = false
  storeMocks.appStore.cachedPublicSettings = {
    payment_enabled: false,
    custom_menu_items: []
  }
  storeMocks.authStore.isAdmin = true
  storeMocks.authStore.isSimpleMode = false
  storeMocks.adminSettingsStore.opsMonitoringEnabled = true
  storeMocks.adminSettingsStore.paymentEnabled = false
  storeMocks.adminSettingsStore.customMenuItems = []
  storeMocks.adminSettingsStore.sizeBetEnabled = true
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn()
  })
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
    matches: false,
    media: '',
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn()
  }))
})

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n  \}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar admin checkin analytics navigation', () => {
  it('contains the admin checkin analytics route entry', () => {
    expect(componentSource).toContain("/admin/checkin-analytics")
    expect(componentSource).toContain("t('nav.checkinAnalytics')")
  })
})

describe('AppSidebar size bet admin navigation visibility', () => {
  it('hides the size bet participation and stats menus when the activity is disabled', async () => {
    storeMocks.authStore.isAdmin = false
    storeMocks.adminSettingsStore.sizeBetEnabled = false
    storeMocks.appStore.cachedPublicSettings = {
      payment_enabled: false,
      custom_menu_items: [],
      size_bet_enabled: false
    }

    const wrapper = await mountSidebar()

    expect(wrapper.text()).not.toContain('sizeBet.nav')
    expect(wrapper.text()).not.toContain('sizeBet.statsNav')
  })

  it('shows the size bet participation and stats menus when the activity is enabled', async () => {
    storeMocks.authStore.isAdmin = false
    storeMocks.adminSettingsStore.sizeBetEnabled = true
    storeMocks.appStore.cachedPublicSettings = {
      payment_enabled: false,
      custom_menu_items: [],
      size_bet_enabled: true
    }

    const wrapper = await mountSidebar()

    expect(wrapper.text()).toContain('sizeBet.nav')
    expect(wrapper.text()).toContain('sizeBet.statsNav')
  })
})
