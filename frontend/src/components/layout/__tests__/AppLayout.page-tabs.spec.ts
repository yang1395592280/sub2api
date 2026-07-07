import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import AppLayout from '../AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { usePageTabsStore } from '@/stores/pageTabs'

vi.mock('@/composables/useOnboardingTour', () => ({
  useOnboardingTour: () => ({
    replayTour: vi.fn()
  })
}))

function createTestRouter(): Router {
  return createRouter({
    history: createWebHistory(),
    routes: [
      {
        path: '/admin/dashboard',
        component: { template: '<div>dashboard</div>' },
        meta: { title: '管理控制台', requiresAuth: true, requiresAdmin: true }
      },
      {
        path: '/admin/users',
        component: { template: '<div>users</div>' },
        meta: { title: '用户管理', requiresAuth: true, requiresAdmin: true }
      }
    ]
  })
}

describe('AppLayout page tabs integration', () => {
  let router: Router

  beforeEach(async () => {
    setActivePinia(createPinia())
    router = createTestRouter()
    await router.push('/admin/users')
    await router.isReady()

    const authStore = useAuthStore()
    authStore.user = { id: 1, email: 'admin@example.com', role: 'admin' } as typeof authStore.user
  })

  it('renders the page tabs bar and registers pinned dashboard plus current route', async () => {
    mount(AppLayout, {
      slots: {
        default: '<div>content</div>'
      },
      global: {
        plugins: [router],
        stubs: {
          AppSidebar: { template: '<aside />' },
          AppHeader: { template: '<header />' },
          PageTabs: { template: '<nav data-testid="page-tabs" />' }
        }
      }
    })

    await nextTick()

    const pageTabsStore = usePageTabsStore()
    expect(pageTabsStore.tabs.map((tab) => ({ path: tab.path, title: tab.title, pinned: tab.pinned }))).toEqual([
      { path: '/admin/dashboard', title: '管理控制台', pinned: true },
      { path: '/admin/users', title: '用户管理', pinned: false }
    ])
  })
})
