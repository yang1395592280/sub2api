<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />
      <PageTabs />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { RouteLocationNormalizedLoaded, RouteLocationResolved } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import { usePageTabsStore } from '@/stores/pageTabs'
import { i18n } from '@/i18n'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import PageTabs from './PageTabs.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const pageTabsStore = usePageTabsStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

type TitleRoute = Pick<RouteLocationNormalizedLoaded | RouteLocationResolved, 'name' | 'params' | 'meta' | 'path'>

function translateTitleKey(titleKey: unknown, fallback = ''): string {
  if (typeof titleKey !== 'string' || !titleKey.trim()) return fallback
  const translated = i18n.global.t(titleKey)
  return translated && translated !== titleKey ? translated : fallback
}

function resolveRouteTitle(targetRoute: TitleRoute = route): string {
  if (targetRoute.name === 'CustomPage') {
    const id = typeof targetRoute.params.id === 'string' ? targetRoute.params.id : ''
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }

  const translated = translateTitleKey(targetRoute.meta.titleKey)
  if (translated) return translated
  return typeof targetRoute.meta.title === 'string' ? targetRoute.meta.title : targetRoute.path
}

function registerCurrentRouteTab(): void {
  if (route.meta.requiresAuth === false) return

  const resolvedHomeRoute = router.resolve(homePath.value)
  pageTabsStore.ensurePinnedTab(homePath.value, resolveRouteTitle(resolvedHomeRoute))
  pageTabsStore.addTabFromRoute(route, resolveRouteTitle(route))
}

watch(
  [
    () => route.fullPath,
    () => route.meta.title,
    () => route.meta.titleKey,
    () => route.params.id,
    () => appStore.cachedPublicSettings?.custom_menu_items,
    () => authStore.isAdmin,
    () => adminSettingsStore.customMenuItems,
  ],
  registerCurrentRouteTab,
  { deep: true, immediate: true }
)

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
