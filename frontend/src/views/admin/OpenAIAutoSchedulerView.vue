<template>
  <AppLayout>
    <div data-testid="scheduler-page" class="min-h-[calc(100vh-8rem)] bg-gray-50/60 dark:bg-dark-950">
      <header class="border-b border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="mx-auto flex max-w-[1800px] flex-col gap-4 px-4 py-4 sm:px-6 xl:flex-row xl:items-center xl:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-lg font-semibold text-gray-950 dark:text-white">{{ t('admin.openaiAutoScheduler.title') }}</h1>
              <span :class="modeBadgeClass">{{ settings?.mode === 'legacy' ? t('admin.openaiAutoScheduler.modes.legacy') : t('admin.openaiAutoScheduler.modes.balanced') }}</span>
              <span v-if="settings?.shadow_mode" class="rounded bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">{{ t('admin.openaiAutoScheduler.modes.shadow') }}</span>
              <span v-else-if="settings?.mode === 'balanced'" class="rounded bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">{{ t('admin.openaiAutoScheduler.modes.live') }}</span>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ selectedGroup?.name || t('admin.openaiAutoScheduler.allGroups') }}</p>
          </div>

          <div class="flex flex-wrap items-center gap-3">
            <div v-if="activeTab === 'overview'" class="inline-flex rounded-md border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-700 dark:bg-dark-800">
              <button
                v-for="item in windows"
                :key="item.value"
                type="button"
                class="min-w-12 rounded px-2.5 py-1.5 text-xs font-medium"
                :class="window === item.value ? 'bg-white text-gray-950 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-dark-400'"
                @click="selectWindow(item.value)"
              >{{ item.label }}</button>
            </div>
            <div class="flex items-center gap-2">
              <span class="text-xs font-medium text-gray-600 dark:text-dark-300">{{ t('admin.openaiAutoScheduler.global') }}</span>
              <Toggle v-if="settings" :modelValue="settings.enabled" @update:modelValue="handleGlobalToggle" />
            </div>
            <button type="button" class="rounded-md border border-gray-200 p-2 text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:border-dark-700 dark:hover:bg-dark-800 dark:hover:text-white" :title="t('admin.openaiAutoScheduler.refresh')" @click="refreshActiveTab">
              <Icon name="refresh" size="sm" /><span class="sr-only">{{ t('admin.openaiAutoScheduler.refresh') }}</span>
            </button>
          </div>
        </div>

        <div class="mx-auto max-w-[1800px] overflow-x-auto px-4 sm:px-6">
          <nav class="flex min-w-max gap-1" :aria-label="t('admin.openaiAutoScheduler.title')">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              :data-testid="`scheduler-tab-${tab.value}`"
              class="border-b-2 px-3 py-3 text-sm font-medium"
              :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-dark-100'"
              @click="selectTab(tab.value)"
            >{{ tab.label }}</button>
          </nav>
        </div>
      </header>

      <main class="mx-auto grid max-w-[1800px] gap-5 px-4 py-5 sm:px-6 lg:grid-cols-[240px_minmax(0,1fr)]">
        <SchedulerGroupList
          :groups="groups"
          :modelValue="selectedGroupId"
          @update:modelValue="selectGroup"
          @toggle="handleGroupToggle"
        />

        <div class="min-w-0">
          <div v-if="errors.initialize" class="border-l-4 border-red-500 bg-red-50 px-4 py-3 text-sm text-red-800 dark:bg-red-500/10 dark:text-red-300">
            {{ errors.initialize }}
            <button type="button" class="ml-2 font-medium" @click="initialize">{{ t('admin.openaiAutoScheduler.actions.retry') }}</button>
          </div>

          <SchedulerOverview
            v-if="activeTab === 'overview'"
            :overview="overview"
            :loading="loading.overview || loading.initialize"
            :selectedGroupId="selectedGroupId"
            @show-health-filter="showGroupHealth"
          />

          <SchedulerHealthTable
            v-else-if="activeTab === 'health'"
            :rows="healthPage.items"
            :loading="loading.health"
            :total="healthPage.total"
            :page="healthPage.page || healthFilters.page || 1"
            :pageSize="healthPage.page_size || healthFilters.page_size || 20"
            :filters="healthFilters"
            @filter="applyHealthFilters"
            @page="setHealthPage"
            @select="openAccountDrawer"
            @probe="handleProbe"
            @reset="requestReset"
          />

          <SchedulerEventsPanel
            v-else-if="activeTab === 'events'"
            :events="eventsPage.items"
            :loading="loading.events"
            :total="eventsPage.total"
            :page="eventsPage.page || eventsPagination.page"
            :pageSize="eventsPage.page_size || eventsPagination.page_size"
            @page="setEventsPage"
          />

          <SchedulerSettingsPanel
            v-else-if="activeTab === 'settings' && settings"
            :modelValue="settings"
            :saving="loading.settings"
            @save="handleSaveSettings"
          />
        </div>
      </main>

      <SchedulerAccountDrawer
        :open="Boolean(selectedAccount)"
        :account="selectedAccount"
        :events="drawerEvents"
        @close="selectedAccount = null"
        @probe="handleProbe"
        @reset="requestReset"
      />

      <ConfirmDialog
        :show="Boolean(pendingReset)"
        :title="t('admin.openaiAutoScheduler.reset.title')"
        :message="pendingReset ? t('admin.openaiAutoScheduler.reset.message', { name: pendingReset.account_name, id: pendingReset.account_id }) : ''"
        :confirmText="t('admin.openaiAutoScheduler.actions.confirmReset')"
        danger
        @confirm="confirmReset"
        @cancel="pendingReset = null"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import SchedulerOverview from '@/components/admin/openai-scheduler/SchedulerOverview.vue'
import SchedulerGroupList from '@/components/admin/openai-scheduler/SchedulerGroupList.vue'
import SchedulerHealthTable from '@/components/admin/openai-scheduler/SchedulerHealthTable.vue'
import SchedulerAccountDrawer from '@/components/admin/openai-scheduler/SchedulerAccountDrawer.vue'
import SchedulerEventsPanel from '@/components/admin/openai-scheduler/SchedulerEventsPanel.vue'
import SchedulerSettingsPanel from '@/components/admin/openai-scheduler/SchedulerSettingsPanel.vue'
import { useOpenAISchedulerDashboard, type OpenAISchedulerDashboardTab } from '@/composables/useOpenAISchedulerDashboard'
import { useAppStore } from '@/stores/app'
import type { OpenAIAutoSchedulerSettings, OpenAISchedulerHealthRow, OpenAISchedulerWindow } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { t } = useI18n()
const dashboard = useOpenAISchedulerDashboard()
const {
  activeTab,
  selectedGroupId,
  selectedGroup,
  window,
  settings,
  groups,
  overview,
  healthPage,
  eventsPage,
  drawerEvents,
  healthFilters,
  eventsPagination,
  loading,
  errors,
  initialize,
  selectGroup,
  selectTab,
  selectWindow,
  refreshActiveTab,
  applyHealthFilters,
  setHealthPage,
  setEventsPage,
  loadAccountEvents,
} = dashboard

const selectedAccount = ref<OpenAISchedulerHealthRow | null>(null)
const pendingReset = ref<OpenAISchedulerHealthRow | null>(null)

const tabs = computed<Array<{ value: OpenAISchedulerDashboardTab; label: string }>>(() => [
  { value: 'overview', label: t('admin.openaiAutoScheduler.tabs.overview') },
  { value: 'health', label: t('admin.openaiAutoScheduler.tabs.health') },
  { value: 'events', label: t('admin.openaiAutoScheduler.tabs.events') },
  { value: 'settings', label: t('admin.openaiAutoScheduler.tabs.settings') },
])

const windows: Array<{ value: OpenAISchedulerWindow; label: string }> = [
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
]

const modeBadgeClass = computed(() => {
  const live = settings.value?.mode === 'balanced' && !settings.value?.shadow_mode
  return live
    ? 'rounded bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
    : 'rounded bg-sky-50 px-2 py-1 text-xs font-medium text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'
})

async function handleGlobalToggle(enabled: boolean): Promise<void> {
  try {
    await dashboard.setGlobalEnabled(enabled)
    appStore.showSuccess(enabled ? t('admin.openaiAutoScheduler.messages.enabled') : t('admin.openaiAutoScheduler.messages.disabled'))
  } catch (error: unknown) {
    appStore.showError(messageOf(error, t('admin.openaiAutoScheduler.messages.globalFailed')))
  }
}

async function handleGroupToggle(groupId: number, enabled: boolean): Promise<void> {
  try {
    await dashboard.setGroupEnabled(groupId, enabled)
    appStore.showSuccess(enabled ? t('admin.openaiAutoScheduler.messages.groupEnabled') : t('admin.openaiAutoScheduler.messages.groupDisabled'))
  } catch (error: unknown) {
    appStore.showError(messageOf(error, t('admin.openaiAutoScheduler.messages.groupFailed')))
  }
}

async function handleSaveSettings(payload: OpenAIAutoSchedulerSettings): Promise<void> {
  try {
    await dashboard.saveSettings(payload)
    appStore.showSuccess(t('admin.openaiAutoScheduler.messages.settingsSaved'))
  } catch (error: unknown) {
    appStore.showError(messageOf(error, t('admin.openaiAutoScheduler.messages.settingsFailed')))
  }
}

async function openAccountDrawer(row: OpenAISchedulerHealthRow): Promise<void> {
  selectedAccount.value = row
  await loadAccountEvents(row)
}

async function handleProbe(row: OpenAISchedulerHealthRow): Promise<void> {
  try {
    await dashboard.probeAccount(row)
    appStore.showSuccess(t('admin.openaiAutoScheduler.messages.probeDone'))
  } catch (error: unknown) {
    appStore.showError(messageOf(error, t('admin.openaiAutoScheduler.messages.probeFailed')))
  }
}

function requestReset(row: OpenAISchedulerHealthRow): void {
  pendingReset.value = row
}

async function confirmReset(): Promise<void> {
  const row = pendingReset.value
  if (!row) return
  try {
    await dashboard.resetAccount(row)
    appStore.showSuccess(t('admin.openaiAutoScheduler.messages.resetDone'))
    if (selectedAccount.value?.account_id === row.account_id) selectedAccount.value = null
  } catch (error: unknown) {
    appStore.showError(messageOf(error, t('admin.openaiAutoScheduler.messages.resetFailed')))
  } finally {
    pendingReset.value = null
  }
}

async function showGroupHealth(groupId: number): Promise<void> {
  await dashboard.showGroupHealth(groupId)
}

function messageOf(error: unknown, fallback: string): string {
  const candidate = error as { message?: string }
  return candidate?.message || fallback
}

onMounted(initialize)
</script>
