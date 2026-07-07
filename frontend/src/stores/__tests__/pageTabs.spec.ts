import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePageTabsStore } from '../pageTabs'

function route(path: string, title = path) {
  return {
    path,
    fullPath: path,
    name: title,
    meta: {}
  }
}

describe('page tabs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps the current dashboard pinned and blocks closing it', () => {
    const store = usePageTabsStore()

    store.ensurePinnedTab('/admin/dashboard', '管理控制台')
    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')

    expect(store.tabs.map((tab) => tab.path)).toEqual(['/admin/dashboard', '/admin/users'])
    expect(store.tabs[0]).toMatchObject({ title: '管理控制台', pinned: true })

    const fallback = store.closeTab('/admin/dashboard')

    expect(fallback?.path).toBe('/admin/dashboard')
    expect(store.tabs.map((tab) => tab.path)).toEqual(['/admin/dashboard', '/admin/users'])
  })

  it('updates an existing page tab instead of duplicating query variants', () => {
    const store = usePageTabsStore()

    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')
    store.addTabFromRoute(
      { path: '/admin/users', fullPath: '/admin/users?page=2', name: 'AdminUsers', meta: {} },
      '用户管理'
    )

    expect(store.tabs).toHaveLength(1)
    expect(store.tabs[0].fullPath).toBe('/admin/users?page=2')
  })

  it('returns the left neighbor after closing the current regular tab', () => {
    const store = usePageTabsStore()

    store.ensurePinnedTab('/admin/dashboard', '管理控制台')
    store.addTabFromRoute(route('/admin/accounts', '账号管理'), '账号管理')
    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')

    const fallback = store.closeTab('/admin/users')

    expect(fallback?.path).toBe('/admin/accounts')
    expect(store.tabs.map((tab) => tab.path)).toEqual(['/admin/dashboard', '/admin/accounts'])
  })

  it('closes other and right-side regular tabs while preserving pinned tabs', () => {
    const store = usePageTabsStore()

    store.ensurePinnedTab('/admin/dashboard', '管理控制台')
    store.addTabFromRoute(route('/admin/accounts', '账号管理'), '账号管理')
    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')
    store.addTabFromRoute(route('/admin/groups', '分组管理'), '分组管理')

    store.closeRightTabs('/admin/accounts')
    expect(store.tabs.map((tab) => tab.path)).toEqual(['/admin/dashboard', '/admin/accounts'])

    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')
    store.closeOtherTabs('/admin/users')
    expect(store.tabs.map((tab) => tab.path)).toEqual(['/admin/dashboard', '/admin/users'])
  })

  it('increments the cache key when refreshing a tab', () => {
    const store = usePageTabsStore()

    store.addTabFromRoute(route('/admin/users', '用户管理'), '用户管理')

    expect(store.routeCacheKey({ path: '/admin/users' })).toBe('/admin/users:0')
    store.refreshTab('/admin/users')
    expect(store.routeCacheKey({ path: '/admin/users' })).toBe('/admin/users:1')
  })
})
