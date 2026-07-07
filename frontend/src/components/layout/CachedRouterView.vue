<template>
  <RouterView v-slot="{ Component, route }">
    <KeepAlive>
      <component
        :is="Component"
        v-if="shouldCacheRoute(route)"
        :key="pageTabsStore.routeCacheKey(route)"
      />
    </KeepAlive>
    <component
      :is="Component"
      v-if="!shouldCacheRoute(route)"
      :key="route.fullPath"
    />
  </RouterView>
</template>

<script setup lang="ts">
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { usePageTabsStore } from '@/stores/pageTabs'

const pageTabsStore = usePageTabsStore()

function shouldCacheRoute(route: RouteLocationNormalizedLoaded): boolean {
  return route.meta.requiresAuth !== false
}
</script>
