import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import VersionBadge from '../VersionBadge.vue'

const storeMocks = vi.hoisted(() => ({
  authStore: {
    isAdmin: true,
  },
  appStore: {
    versionLoading: false,
    currentVersion: '1.0.0',
    latestVersion: '',
    hasUpdate: false,
    releaseInfo: null,
    buildType: 'source',
    fetchVersion: vi.fn(),
    clearVersionCache: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => storeMocks.authStore,
  useAppStore: () => storeMocks.appStore,
}))

vi.mock('@/api/admin/system', () => ({
  performUpdate: vi.fn(),
  restartService: vi.fn(),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('VersionBadge', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    storeMocks.authStore.isAdmin = true
    storeMocks.appStore.versionLoading = false
    storeMocks.appStore.currentVersion = '1.0.0'
    storeMocks.appStore.latestVersion = ''
    storeMocks.appStore.hasUpdate = false
    storeMocks.appStore.releaseInfo = null
    storeMocks.appStore.buildType = 'source'
  })

  it('管理员进入页面时不自动请求版本检查，打开下拉后才触发', async () => {
    const wrapper = mount(VersionBadge, {
      props: { version: '1.0.0' },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(storeMocks.appStore.fetchVersion).not.toHaveBeenCalled()

    await wrapper.find('button').trigger('click')

    expect(storeMocks.appStore.fetchVersion).toHaveBeenCalledTimes(1)
    expect(storeMocks.appStore.fetchVersion).toHaveBeenCalledWith(false)
  })
})
