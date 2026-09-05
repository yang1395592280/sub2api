import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import HomeView from '@/views/HomeView.vue'

const appStoreState = reactive({
  cachedPublicSettings: {
    join_group_enabled: false,
    join_group_url: '',
    join_group_popup_image: '',
    site_name: 'Sub2API',
    site_logo: '',
    site_subtitle: '',
    doc_url: '',
    home_content: '',
  } as Record<string, unknown>,
  siteName: 'Sub2API',
  siteLogo: '',
  docUrl: '',
  fetchPublicSettings: vi.fn(),
})

const authStoreState = reactive({
  isAuthenticated: false,
  isAdmin: false,
  user: null as null | Record<string, unknown>,
  checkAuth: vi.fn(),
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStoreState,
  useAuthStore: () => authStoreState,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreState,
}))

vi.mock('@/api/auth', () => ({
  getPublicHomeContent: vi.fn().mockResolvedValue({ home_content: '' }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/components/common/LocaleSwitcher.vue', () => ({
  default: { template: '<div data-testid="locale-switcher-stub" />' },
}))

Object.defineProperty(globalThis, 'localStorage', {
  value: {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
    clear: vi.fn(),
  },
  configurable: true,
})

Object.defineProperty(window, 'matchMedia', {
  value: vi.fn().mockImplementation(() => ({
    matches: false,
    media: '',
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
  configurable: true,
})

describe('HomeView join group entry', () => {
  beforeEach(() => {
    appStoreState.cachedPublicSettings = {
      join_group_enabled: false,
      join_group_url: '',
      join_group_popup_image: '',
      site_name: 'Sub2API',
      site_logo: '',
      site_subtitle: '',
      doc_url: '',
      home_content: '',
    }
    appStoreState.fetchPublicSettings.mockReset()
    authStoreState.isAuthenticated = false
    authStoreState.isAdmin = false
    authStoreState.user = null
    authStoreState.checkAuth.mockReset()
  })

  it('hides the join group button when the feature is disabled', async () => {
    appStoreState.cachedPublicSettings = {
      ...appStoreState.cachedPublicSettings,
      join_group_enabled: false,
      join_group_url: 'https://qm.qq.com/q/example',
      join_group_popup_image: '',
    }

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('加入群聊')
    expect(wrapper.find('a[href="https://qm.qq.com/q/example"]').exists()).toBe(false)
  })

  it('shows the configured join group button when enabled and url is present', async () => {
    appStoreState.cachedPublicSettings = {
      ...appStoreState.cachedPublicSettings,
      join_group_enabled: true,
      join_group_url: 'https://qm.qq.com/q/example',
      join_group_popup_image: '',
    }

    const wrapper = mount(HomeView, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          'router-link': true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('a[href="https://qm.qq.com/q/example"]').text()).toContain('加入群聊')
  })
})
