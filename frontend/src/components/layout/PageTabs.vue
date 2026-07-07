<template>
  <div
    v-if="tabs.length"
    class="sticky top-16 z-20 border-b border-gray-200/70 bg-white/85 px-3 backdrop-blur dark:border-dark-700/70 dark:bg-dark-950/85 md:px-5"
  >
    <div class="scrollbar-hide flex h-11 items-end gap-1 overflow-x-auto">
      <button
        v-for="tab in tabs"
        :key="tab.path"
        type="button"
        data-testid="page-tab"
        :data-tab-path="tab.path"
        :data-active="isActive(tab.path)"
        class="group relative flex h-10 min-w-0 max-w-[220px] items-center gap-2 border-b-2 px-3 text-sm transition-colors"
        :class="isActive(tab.path)
          ? 'border-primary-500 bg-primary-50/70 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
          : 'border-transparent text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white'"
        @click="activateTab(tab)"
        @contextmenu.prevent="openContextMenu($event, tab.path)"
      >
        <span class="truncate" :data-testid="`page-tab-${tab.path}`">{{ tab.title }}</span>
        <span
          v-if="tab.pinned"
          class="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-primary-400"
          aria-hidden="true"
        ></span>
        <button
          v-else
          type="button"
          :data-testid="`page-tab-close-${tab.path}`"
          class="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-md text-gray-400 opacity-80 transition hover:bg-gray-200 hover:text-gray-700 group-hover:opacity-100 dark:hover:bg-dark-700 dark:hover:text-white"
          :title="`关闭 ${tab.title}`"
          @click.stop="closeTab(tab.path)"
        >
          <Icon name="x" size="xs" />
        </button>
      </button>
    </div>

    <teleport to="body">
      <div
        v-if="contextMenu"
        class="fixed z-50 min-w-36 rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-900"
        :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
        @click.stop
      >
        <button
          type="button"
          data-testid="page-tab-refresh"
          class="context-item"
          @click="refreshContextTab"
        >
          <Icon name="refresh" size="sm" />
          <span>刷新当前页</span>
        </button>
        <button
          type="button"
          class="context-item"
          :disabled="contextTab?.pinned"
          @click="closeContextTab"
        >
          <Icon name="x" size="sm" />
          <span>关闭当前页</span>
        </button>
        <button type="button" class="context-item" @click="closeOtherTabs">
          <Icon name="swap" size="sm" />
          <span>关闭其它页</span>
        </button>
        <button type="button" class="context-item" @click="closeRightTabs">
          <Icon name="arrowRight" size="sm" />
          <span>关闭右侧页</span>
        </button>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { usePageTabsStore, type PageTab } from '@/stores/pageTabs'

const router = useRouter()
const route = useRoute()
const pageTabsStore = usePageTabsStore()

const tabs = computed(() => pageTabsStore.tabs)
const contextMenu = ref<{ path: string; x: number; y: number } | null>(null)
const contextTab = computed(() => tabs.value.find((tab) => tab.path === contextMenu.value?.path))

function isActive(path: string): boolean {
  return route.path === path
}

async function activateTab(tab: PageTab): Promise<void> {
  if (route.path === tab.path && route.fullPath === tab.fullPath) return
  await router.push(tab.fullPath)
}

async function closeTab(path: string): Promise<void> {
  const closingActiveTab = route.path === path
  const fallback = pageTabsStore.closeTab(path)
  contextMenu.value = null

  if (closingActiveTab && fallback) {
    await router.push(fallback.fullPath)
  }
}

function openContextMenu(event: MouseEvent, path: string): void {
  contextMenu.value = {
    path,
    x: event.clientX,
    y: event.clientY
  }
}

function refreshContextTab(): void {
  if (!contextMenu.value) return
  pageTabsStore.refreshTab(contextMenu.value.path)
  contextMenu.value = null
}

async function closeContextTab(): Promise<void> {
  if (!contextMenu.value || contextTab.value?.pinned) return
  await closeTab(contextMenu.value.path)
}

function closeOtherTabs(): void {
  if (!contextMenu.value) return
  pageTabsStore.closeOtherTabs(contextMenu.value.path)
  contextMenu.value = null
}

function closeRightTabs(): void {
  if (!contextMenu.value) return
  pageTabsStore.closeRightTabs(contextMenu.value.path)
  contextMenu.value = null
}

function closeContextMenu(): void {
  contextMenu.value = null
}

onMounted(() => {
  document.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeContextMenu)
})
</script>

<style scoped>
.context-item {
  @apply flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:text-gray-400 disabled:hover:bg-transparent dark:text-dark-200 dark:hover:bg-dark-800 dark:disabled:text-dark-500;
}
</style>
