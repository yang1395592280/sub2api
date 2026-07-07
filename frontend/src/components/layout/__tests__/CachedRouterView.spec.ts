import { defineComponent, h, ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import { beforeEach, describe, expect, it } from 'vitest'
import CachedRouterView from '../CachedRouterView.vue'
import { usePageTabsStore } from '@/stores/pageTabs'

const CounterPage = defineComponent({
  name: 'CounterPage',
  setup() {
    const count = ref(0)
    return () =>
      h('button', {
        'data-testid': 'counter',
        onClick: () => {
          count.value += 1
        }
      }, String(count.value))
  }
})

const OtherPage = defineComponent({
  name: 'OtherPage',
  setup() {
    return () => h('div', { 'data-testid': 'other' }, 'other')
  }
})

function createTestRouter(): Router {
  return createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/counter', component: CounterPage },
      { path: '/other', component: OtherPage }
    ]
  })
}

describe('CachedRouterView', () => {
  let router: Router

  beforeEach(async () => {
    setActivePinia(createPinia())
    router = createTestRouter()
    await router.push('/counter')
    await router.isReady()
  })

  it('keeps route component state when switching tabs', async () => {
    const wrapper = mount(CachedRouterView, {
      global: {
        plugins: [router]
      }
    })

    await wrapper.find('[data-testid="counter"]').trigger('click')
    expect(wrapper.find('[data-testid="counter"]').text()).toBe('1')

    await router.push('/other')
    await flushPromises()
    expect(wrapper.find('[data-testid="other"]').exists()).toBe(true)

    await router.push('/counter')
    await flushPromises()
    expect(wrapper.find('[data-testid="counter"]').text()).toBe('1')
  })

  it('recreates the current route component when its tab is refreshed', async () => {
    const wrapper = mount(CachedRouterView, {
      global: {
        plugins: [router]
      }
    })
    const store = usePageTabsStore()
    store.addTabFromRoute(
      { path: '/counter', fullPath: '/counter', name: 'CounterPage', meta: {} },
      'Counter'
    )

    await wrapper.find('[data-testid="counter"]').trigger('click')
    expect(wrapper.find('[data-testid="counter"]').text()).toBe('1')

    store.refreshTab('/counter')
    await flushPromises()

    expect(wrapper.find('[data-testid="counter"]').text()).toBe('0')
  })
})
