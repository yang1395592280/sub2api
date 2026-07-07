import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import PageTabs from '../PageTabs.vue'
import { usePageTabsStore } from '@/stores/pageTabs'

function createTestRouter(): Router {
  return createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/admin/dashboard', component: { template: '<div>dashboard</div>' } },
      { path: '/admin/accounts', component: { template: '<div>accounts</div>' } },
      { path: '/admin/users', component: { template: '<div>users</div>' } }
    ]
  })
}

describe('PageTabs', () => {
  let router: Router

  beforeEach(async () => {
    setActivePinia(createPinia())
    router = createTestRouter()
    await router.push('/admin/accounts')
    await router.isReady()

    const store = usePageTabsStore()
    store.ensurePinnedTab('/admin/dashboard', '管理控制台')
    store.addTabFromRoute(
      { path: '/admin/accounts', fullPath: '/admin/accounts', name: 'AdminAccounts', meta: {} },
      '账号管理'
    )
    store.addTabFromRoute(
      { path: '/admin/users', fullPath: '/admin/users', name: 'AdminUsers', meta: {} },
      '用户管理'
    )
  })

  it('renders opened tabs and marks the active tab', () => {
    const wrapper = mount(PageTabs, {
      global: {
        plugins: [router]
      }
    })

    expect(wrapper.findAll('[data-testid="page-tab"]').map((item) => item.text())).toEqual([
      '管理控制台',
      '账号管理',
      '用户管理'
    ])
    expect(wrapper.find('[data-active="true"]').text()).toContain('账号管理')
  })

  it('navigates when clicking another tab', async () => {
    const wrapper = mount(PageTabs, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('[data-testid="page-tab-/admin/users"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/admin/users')
  })

  it('does not render a close button for the pinned dashboard tab', () => {
    const wrapper = mount(PageTabs, {
      global: {
        plugins: [router]
      }
    })

    expect(wrapper.find('[data-testid="page-tab-close-/admin/dashboard"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="page-tab-close-/admin/accounts"]').exists()).toBe(true)
  })

  it('closes a regular current tab and navigates to the left neighbor', async () => {
    const wrapper = mount(PageTabs, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('[data-testid="page-tab-close-/admin/accounts"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/admin/dashboard')
    expect(usePageTabsStore().tabs.map((tab) => tab.path)).toEqual([
      '/admin/dashboard',
      '/admin/users'
    ])
  })

  it('refreshes the selected tab from the context menu', async () => {
    const wrapper = mount(PageTabs, {
      attachTo: document.body,
      global: {
        plugins: [router]
      }
    })
    const store = usePageTabsStore()
    const refreshSpy = vi.spyOn(store, 'refreshTab')

    await wrapper.find('[data-testid="page-tab-/admin/accounts"]').trigger('contextmenu')
    const refreshButton = document.body.querySelector('[data-testid="page-tab-refresh"]') as HTMLButtonElement
    refreshButton.click()

    expect(refreshSpy).toHaveBeenCalledWith('/admin/accounts')
    expect(store.routeCacheKey({ path: '/admin/accounts' })).toBe('/admin/accounts:1')

    wrapper.unmount()
  })
})
