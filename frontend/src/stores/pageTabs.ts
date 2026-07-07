import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

export interface PageTab {
  path: string
  fullPath: string
  title: string
  name?: string
  pinned: boolean
  refreshVersion: number
}

type RouteLike = Pick<RouteLocationNormalizedLoaded, 'path' | 'fullPath' | 'name' | 'meta'>

const PINNED_DASHBOARD_PATHS = new Set(['/dashboard', '/admin/dashboard'])

function normalizePath(path: string): string {
  return path || '/'
}

function isPinnedPath(path: string): boolean {
  return PINNED_DASHBOARD_PATHS.has(path)
}

function makeTab(route: RouteLike, title: string, pinned = isPinnedPath(route.path)): PageTab {
  const path = normalizePath(route.path)
  return {
    path,
    fullPath: route.fullPath || path,
    title,
    name: typeof route.name === 'string' ? route.name : undefined,
    pinned,
    refreshVersion: 0
  }
}

export const usePageTabsStore = defineStore('pageTabs', () => {
  const tabs = ref<PageTab[]>([])

  const openedTabs = computed(() => tabs.value)

  function findTabIndex(path: string): number {
    return tabs.value.findIndex((tab) => tab.path === normalizePath(path))
  }

  function ensurePinnedTab(path: string, title: string): PageTab {
    const normalizedPath = normalizePath(path)
    const existingIndex = findTabIndex(normalizedPath)
    if (existingIndex >= 0) {
      const existing = tabs.value[existingIndex]
      existing.title = title
      existing.pinned = true
      return existing
    }

    const tab: PageTab = {
      path: normalizedPath,
      fullPath: normalizedPath,
      title,
      pinned: true,
      refreshVersion: 0
    }
    tabs.value.unshift(tab)
    return tab
  }

  function addTabFromRoute(route: RouteLike, title: string): PageTab {
    const path = normalizePath(route.path)
    const existingIndex = findTabIndex(path)
    if (existingIndex >= 0) {
      const existing = tabs.value[existingIndex]
      existing.fullPath = route.fullPath || path
      existing.title = title
      existing.name = typeof route.name === 'string' ? route.name : existing.name
      existing.pinned = existing.pinned || isPinnedPath(path)
      return existing
    }

    const tab = makeTab(route, title)
    tabs.value.push(tab)
    return tab
  }

  function closeTab(path: string): PageTab | null {
    const index = findTabIndex(path)
    if (index < 0) return tabs.value[0] ?? null

    const target = tabs.value[index]
    if (target.pinned) return target

    tabs.value.splice(index, 1)
    return tabs.value[index - 1] ?? tabs.value[index] ?? tabs.value[0] ?? null
  }

  function closeOtherTabs(path: string): void {
    const normalizedPath = normalizePath(path)
    tabs.value = tabs.value.filter((tab) => tab.pinned || tab.path === normalizedPath)
  }

  function closeRightTabs(path: string): void {
    const index = findTabIndex(path)
    if (index < 0) return

    tabs.value = tabs.value.filter((tab, tabIndex) => tab.pinned || tabIndex <= index)
  }

  function refreshTab(path: string): void {
    const tab = tabs.value[findTabIndex(path)]
    if (!tab) return
    tab.refreshVersion += 1
  }

  function routeCacheKey(route: Pick<RouteLocationNormalizedLoaded, 'path'>): string {
    const path = normalizePath(route.path)
    const tab = tabs.value[findTabIndex(path)]
    return `${path}:${tab?.refreshVersion ?? 0}`
  }

  function reset(): void {
    tabs.value = []
  }

  return {
    tabs: openedTabs,
    ensurePinnedTab,
    addTabFromRoute,
    closeTab,
    closeOtherTabs,
    closeRightTabs,
    refreshTab,
    routeCacheKey,
    reset
  }
})
