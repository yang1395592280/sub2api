<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.openaiScheduler.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.openaiScheduler.description') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <input
            v-model="statsDate"
            class="input w-36"
            type="date"
            :aria-label="t('admin.openaiScheduler.stats.date')"
            @change="reload"
          >
          <button class="btn btn-secondary" :disabled="!selectedGroupId || loading" @click="recomputeStats">
            {{ t('admin.openaiScheduler.stats.recompute') }}
          </button>
          <button class="btn btn-secondary" @click="reload">
            {{ t('admin.openaiScheduler.refresh') }}
          </button>
          <button class="btn btn-primary" @click="saveSettings">
            {{ t('admin.openaiScheduler.saveSettings') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div
          v-for="metric in metrics"
          :key="metric.key"
          class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ metric.value }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div
          v-for="metric in statsMetrics"
          :key="metric.key"
          class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ metric.value }}
          </div>
        </div>
      </div>

      <div class="grid gap-4 xl:grid-cols-[280px_minmax(0,1fr)]">
        <section class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-4 flex items-center justify-between">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.openaiScheduler.saveSettings') }}
            </h2>
            <Toggle
              :modelValue="settings.health_ranking_enabled"
              @update:modelValue="settings.health_ranking_enabled = $event"
            />
          </div>
          <div class="space-y-3">
            <label class="block">
              <span class="input-label">{{ t('admin.openaiScheduler.settings.primaryRatio') }}</span>
              <input
                v-model.number="settings.primary_ratio"
                class="input"
                type="number"
                min="0"
                max="1"
                step="0.05"
              >
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.openaiScheduler.settings.ttftDegradeMs') }}</span>
              <input
                v-model.number="settings.ttft_degrade_ms"
                class="input"
                type="number"
                min="1"
              >
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.openaiScheduler.settings.errorRateThreshold') }}</span>
              <input
                v-model.number="settings.error_rate_degrade_threshold"
                class="input"
                type="number"
                min="0"
                max="1"
                step="0.05"
              >
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.openaiScheduler.settings.cooldownSeconds') }}</span>
              <input
                v-model.number="settings.cooldown_seconds"
                class="input"
                type="number"
                min="1"
              >
            </label>
          </div>
        </section>

        <TablePageLayout>
          <template #filters>
            <div class="flex flex-wrap items-center gap-2">
              <select
                v-model.number="selectedGroupId"
                class="input max-w-xs"
                :disabled="groupsLoading || openaiGroups.length === 0"
                @change="handleGroupChange"
              >
                <option
                  v-if="openaiGroups.length === 0"
                  :value="0"
                >
                  {{ groupsLoading ? t('common.loading') : t('admin.openaiScheduler.noGroups') }}
                </option>
                <option
                  v-for="group in openaiGroups"
                  :key="group.id"
                  :value="group.id"
                >
                  {{ group.name }}
                </option>
              </select>
              <input
                v-model="search"
                class="input max-w-xs"
                :placeholder="t('admin.openaiScheduler.searchPlaceholder')"
                @input="handleSearch"
              >
              <button
                v-for="tier in tierFilters"
                :key="tier.value"
                class="btn btn-sm"
                :class="tierFilter === tier.value ? 'btn-primary' : 'btn-secondary'"
                @click="setTier(tier.value)"
              >
                {{ tier.label }}
              </button>
            </div>
          </template>

          <template #table>
            <DataTable :columns="columns" :data="accounts" :loading="loading">
              <template #cell-account_name="{ row }">
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">{{ row.account_name }}</div>
                  <div class="text-xs text-gray-500">
                    #{{ row.account_id }} · {{ row.type }} · P{{ row.manual_priority }}
                  </div>
                </div>
              </template>
              <template #cell-tier="{ row }">
                <span
                  class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium"
                  :class="tierClass(row.health.tier)"
                >
                  {{ t(`admin.openaiScheduler.tier.${row.health.tier}`) }}
                </span>
              </template>
              <template #cell-health="{ row }">
                {{ row.health.health_score.toFixed(1) }}
              </template>
              <template #cell-success="{ row }">
                {{ formatPercent(row.health.success_rate_ewma) }}
              </template>
              <template #cell-ttft="{ row }">
                {{ formatLatency(row.health.ttft_ewma_ms) }}
              </template>
              <template #cell-select_count="{ row }">
                {{ accountStat(row.account_id)?.select_count ?? 0 }}
              </template>
              <template #cell-select_ratio="{ row }">
                {{ formatPercent(accountStat(row.account_id)?.select_ratio ?? 0) }}
              </template>
              <template #cell-last_selected_at="{ row }">
                {{ formatDateTime(accountStat(row.account_id)?.last_selected_at) }}
              </template>
              <template #cell-actions="{ row }">
                <div class="flex justify-end gap-1">
                  <button class="btn btn-xs btn-secondary" @click="apply(row.account_id, 'promote_observe')">
                    {{ t('admin.openaiScheduler.actions.promoteObserve') }}
                  </button>
                  <button class="btn btn-xs btn-secondary" @click="apply(row.account_id, 'clear_cooldown')">
                    {{ t('admin.openaiScheduler.actions.clearCooldown') }}
                  </button>
                </div>
              </template>
            </DataTable>
          </template>

          <template #pagination>
            <Pagination
              v-if="pagination.total > 0"
              :page="pagination.page"
              :total="pagination.total"
              :page-size="pagination.page_size"
              @update:page="onPageChange"
              @update:pageSize="onPageSizeChange"
            />
          </template>
        </TablePageLayout>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  OpenAISchedulerAccount,
  OpenAISchedulerAccountDailyStat,
  OpenAISchedulerDailyStats,
  OpenAISchedulerOverview,
  OpenAISchedulerSettings,
  OpenAISchedulerTier,
} from '@/api/admin/openaiScheduler'
import type { Column } from '@/components/common/types'
import type { AdminGroup } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const groupsLoading = ref(false)
const openaiGroups = ref<AdminGroup[]>([])
const selectedGroupId = ref(0)
const accounts = ref<OpenAISchedulerAccount[]>([])
const overview = ref<OpenAISchedulerOverview | null>(null)
const dailyStats = ref<OpenAISchedulerDailyStats | null>(null)
const statsDate = ref(todayDateString())
const search = ref('')
const tierFilter = ref<OpenAISchedulerTier | ''>('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let searchTimer: ReturnType<typeof setTimeout> | null = null

const settings = reactive<OpenAISchedulerSettings>({
  health_ranking_enabled: false,
  primary_ratio: 0.3,
  primary_min_count: 1,
  ttft_degrade_ms: 2500,
  error_rate_degrade_threshold: 0.35,
  consecutive_failure_threshold: 3,
  recover_success_threshold: 5,
  cooldown_seconds: 600,
  observe_probe_ratio: 0,
})

const columns = computed<Column[]>(() => [
  { key: 'account_name', label: t('admin.openaiScheduler.columns.account'), sortable: false },
  { key: 'tier', label: t('admin.openaiScheduler.columns.tier'), sortable: false },
  { key: 'health', label: t('admin.openaiScheduler.columns.health'), sortable: false },
  { key: 'success', label: t('admin.openaiScheduler.columns.successRate'), sortable: false },
  { key: 'ttft', label: t('admin.openaiScheduler.columns.ttft'), sortable: false },
  { key: 'select_count', label: t('admin.openaiScheduler.columns.selectCount'), sortable: false },
  { key: 'select_ratio', label: t('admin.openaiScheduler.columns.selectRatio'), sortable: false },
  { key: 'last_selected_at', label: t('admin.openaiScheduler.columns.lastSelectedAt'), sortable: false },
  { key: 'actions', label: t('admin.openaiScheduler.columns.actions'), sortable: false },
])

const tierFilters = computed(() => [
  { value: '' as const, label: 'All' },
  { value: 'primary' as const, label: t('admin.openaiScheduler.tier.primary') },
  { value: 'standby' as const, label: t('admin.openaiScheduler.tier.standby') },
  { value: 'observe' as const, label: t('admin.openaiScheduler.tier.observe') },
  { value: 'degraded' as const, label: t('admin.openaiScheduler.tier.degraded') },
])

const metrics = computed(() => {
  const tierCounts = overview.value?.tier_counts || {}
  return [
    {
      key: 'primary',
      label: t('admin.openaiScheduler.tier.primary'),
      value: tierCounts.primary ?? overview.value?.primary_count ?? 0,
    },
    {
      key: 'standby',
      label: t('admin.openaiScheduler.tier.standby'),
      value: tierCounts.standby ?? overview.value?.standby_count ?? 0,
    },
    {
      key: 'observe',
      label: t('admin.openaiScheduler.tier.observe'),
      value: tierCounts.observe ?? overview.value?.observe_count ?? 0,
    },
    {
      key: 'degraded',
      label: t('admin.openaiScheduler.tier.degraded'),
      value: tierCounts.degraded ?? overview.value?.degraded_count ?? 0,
    },
  ]
})

const statsByAccount = computed(() => {
  const result = new Map<number, OpenAISchedulerAccountDailyStat>()
  for (const item of dailyStats.value?.accounts || []) {
    result.set(item.account_id, item)
  }
  return result
})

const statsMetrics = computed(() => {
  const total = dailyStats.value?.total_selects ?? 0
  const activeAccounts = (dailyStats.value?.accounts || []).filter((item) => item.select_count > 0).length
  const top = dailyStats.value?.accounts?.[0]
  const topName = top ? accountName(top.account_id) : '-'
  return [
    {
      key: 'total',
      label: t('admin.openaiScheduler.stats.totalSelects'),
      value: total,
    },
    {
      key: 'activeAccounts',
      label: t('admin.openaiScheduler.stats.activeAccounts'),
      value: activeAccounts,
    },
    {
      key: 'topAccount',
      label: t('admin.openaiScheduler.stats.topAccount'),
      value: top ? `${topName} · ${formatPercent(top.select_ratio)}` : '-',
    },
  ]
})

function assignSettings(next: OpenAISchedulerSettings) {
  Object.assign(settings, next)
}

async function reload() {
  if (!selectedGroupId.value) {
    overview.value = null
    accounts.value = []
    dailyStats.value = null
    pagination.total = 0
    return
  }
  loading.value = true
  try {
    const groupID = selectedGroupId.value
    const [overviewRes, accountsRes, statsRes] = await Promise.all([
      adminAPI.openaiScheduler.getOverview({ group_id: groupID }),
      adminAPI.openaiScheduler.listAccounts({
        page: pagination.page,
        page_size: pagination.page_size,
        group_id: groupID,
        tier: tierFilter.value,
        search: search.value.trim(),
      }),
      adminAPI.openaiScheduler.getDailyStats({
        group_id: groupID,
        date: statsDate.value,
      }),
    ])
    overview.value = overviewRes
    dailyStats.value = statsRes
    if (overviewRes.settings) assignSettings(overviewRes.settings)
    accounts.value = accountsRes.items || []
    pagination.total = accountsRes.total
    pagination.page = accountsRes.page
    pagination.page_size = accountsRes.page_size
  } catch {
    appStore.showError(t('admin.openaiScheduler.loadError'))
  } finally {
    loading.value = false
  }
}

async function recomputeStats() {
  if (!selectedGroupId.value) return
  loading.value = true
  try {
    dailyStats.value = await adminAPI.openaiScheduler.recomputeDailyStats({
      group_id: selectedGroupId.value,
      date: statsDate.value,
    })
    appStore.showSuccess(t('admin.openaiScheduler.stats.recomputeSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.openaiScheduler.stats.recomputeFailed')))
  } finally {
    loading.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    openaiGroups.value = await adminAPI.groups.getAll('openai')
    if (!selectedGroupId.value && openaiGroups.value.length > 0) {
      selectedGroupId.value = openaiGroups.value[0].id
    }
  } catch {
    appStore.showError(t('admin.openaiScheduler.groupsLoadError'))
  } finally {
    groupsLoading.value = false
  }
}

async function initialize() {
  await loadGroups()
  await reload()
}

async function saveSettings() {
  try {
    const updated = await adminAPI.openaiScheduler.updateSettings({ ...settings })
    assignSettings(updated)
    appStore.showSuccess(t('admin.openaiScheduler.settingsSaved'))
  } catch {
    appStore.showError(t('admin.openaiScheduler.settingsLoadError'))
  }
}

function handleGroupChange() {
  pagination.page = 1
  reload()
}

async function apply(accountId: number, action: 'promote_observe' | 'clear_cooldown') {
  try {
    await adminAPI.openaiScheduler.applyAction(accountId, { action })
    appStore.showSuccess(t('admin.openaiScheduler.actionSuccess'))
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.openaiScheduler.actionFailed')))
  }
}

function handleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 300)
}

function setTier(tier: OpenAISchedulerTier | '') {
  tierFilter.value = tier
  pagination.page = 1
  reload()
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  reload()
}

function formatPercent(value: number): string {
  return `${Math.round((value || 0) * 100)}%`
}

function formatLatency(value: number): string {
  if (!value) return '-'
  return `${Math.round(value)}ms`
}

function accountStat(accountId: number): OpenAISchedulerAccountDailyStat | undefined {
  return statsByAccount.value.get(accountId)
}

function accountName(accountId: number): string {
  return accounts.value.find((item) => item.account_id === accountId)?.account_name || `#${accountId}`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

function todayDateString(): string {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function tierClass(tier: OpenAISchedulerTier): string {
  if (tier === 'primary') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (tier === 'standby') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (tier === 'observe') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
}

onMounted(initialize)
</script>
