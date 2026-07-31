import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { mount } from '@vue/test-utils'
import AppHeader from '@/components/layout/AppHeader.vue'

const appStoreState = reactive({
  cachedPublicSettings: {
    join_group_enabled: false,
    join_group_url: '',
    join_group_popup_image: '',
    site_subtitle: '',
    custom_menu_items: [],
  } as Record<string, unknown>,
  publicSettingsLoaded: true,
  siteName: 'Sub2API',
  siteLogo: '',
  contactInfo: '',
  docUrl: '',
  toggleMobileSidebar: vi.fn(),
  fetchPublicSettings: vi.fn(),
})

const authStoreState = reactive({
  user: null as null | Record<string, unknown>,
  isAdmin: false,
  isSimpleMode: false,
  logout: vi.fn(),
})

const adminSettingsStoreState = reactive({
  customMenuItems: [] as Array<Record<string, unknown>>,
})

const onboardingStoreState = {
  replay: vi.fn(),
}

vi.mock('@/stores', () => ({
  useAppStore: () => appStoreState,
  useAuthStore: () => authStoreState,
  useOnboardingStore: () => onboardingStoreState,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => adminSettingsStoreState,
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ name: 'Dashboard', meta: {}, params: {} }),
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/common/LocaleSwitcher.vue', () => ({
  default: { template: '<div data-testid="locale-switcher-stub" />' },
}))

vi.mock('@/components/common/SubscriptionProgressMini.vue', () => ({
  default: { template: '<div data-testid="subscription-progress-mini-stub" />' },
}))

vi.mock('@/components/common/AnnouncementBell.vue', () => ({
  default: { template: '<div data-testid="announcement-bell-stub" />' },
}))

describe('join group layout entries', () => {
  beforeEach(() => {
    appStoreState.cachedPublicSettings = {
      join_group_enabled: false,
      join_group_url: '',
      join_group_popup_image: '',
      site_subtitle: '',
      custom_menu_items: [],
    }
    appStoreState.fetchPublicSettings.mockReset()
    appStoreState.toggleMobileSidebar.mockReset()
    authStoreState.user = null
    authStoreState.logout.mockReset()
    onboardingStoreState.replay.mockReset()
    adminSettingsStoreState.customMenuItems = []
  })

  it('hides the app header entry when join group url is empty', () => {
    appStoreState.cachedPublicSettings = {
      ...appStoreState.cachedPublicSettings,
      join_group_enabled: true,
      join_group_url: '   ',
      join_group_popup_image: '',
    }

    const wrapper = mount(AppHeader, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          SubscriptionProgressMini: true,
          AnnouncementBell: true,
          'router-link': true,
        },
      },
    })

    expect(wrapper.text()).not.toContain('加入群聊')
  })

  it('renders join group link in app header when enabled', () => {
    appStoreState.cachedPublicSettings = {
      ...appStoreState.cachedPublicSettings,
      join_group_enabled: true,
      join_group_url: 'https://qm.qq.com/q/example',
      join_group_popup_image: '',
    }

    const appHeader = mount(AppHeader, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          SubscriptionProgressMini: true,
          AnnouncementBell: true,
          'router-link': true,
        },
      },
    })

    expect(appHeader.get('a[href="https://qm.qq.com/q/example"]').text()).toContain('加入群聊')
  })

  it('opens a popup dialog when join group image is configured', async () => {
    appStoreState.cachedPublicSettings = {
      ...appStoreState.cachedPublicSettings,
      join_group_enabled: true,
      join_group_url: 'https://qm.qq.com/q/example',
      join_group_popup_image: 'data:image/png;base64,QUJD',
    }

    const appHeader = mount(AppHeader, {
      global: {
        stubs: {
          Icon: true,
          LocaleSwitcher: true,
          SubscriptionProgressMini: true,
          AnnouncementBell: true,
          'router-link': true,
        },
      },
      attachTo: document.body,
    })

    expect(appHeader.find('a[href="https://qm.qq.com/q/example"]').exists()).toBe(false)

    await appHeader.get('button[title="点击链接加入群聊【Loomex】"]').trigger('click')

    expect(document.body.textContent).toContain('加入群聊')
    const preview = document.body.querySelector('img[src="data:image/png;base64,QUJD"]')
    expect(preview).not.toBeNull()
    const actionLink = document.body.querySelector('a[href="https://qm.qq.com/q/example"]')
    expect(actionLink?.textContent).toContain('立即跳转')

    appHeader.unmount()
  })
})
