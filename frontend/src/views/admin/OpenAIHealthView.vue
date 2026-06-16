<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.openaiHealth.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.openaiHealth.description') }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <RouterLink class="btn btn-secondary" to="/admin/openai-scheduler">
            {{ t('admin.openaiHealth.schedulerTab') }}
          </RouterLink>
          <RouterLink class="btn btn-secondary" to="/admin/channels/monitor">
            {{ t('admin.openaiHealth.monitorTab') }}
          </RouterLink>
          <button class="btn btn-primary" :disabled="loading" @click="reload">
            {{ t('admin.openaiHealth.refresh') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 xl:grid-cols-6">
        <div
          v-for="metric in overviewMetrics"
          :key="metric.key"
          class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ metric.value }}</div>
        </div>
      </div>

      <TablePageLayout>
        <template #filters>
          <div class="flex flex-col gap-3">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.openaiHealth.timeWindow') }}
              </span>
              <button
                v-for="option in windowOptions"
                :key="option.value"
                class="btn btn-sm"
                :class="timeWindow === option.value ? 'btn-primary' : 'btn-secondary'"
                @click="timeWindow = option.value"
              >
                {{ option.label }}
              </button>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <select v-model="selectedGroupName" class="input max-w-xs" @change="reload">
                <option value="">
                  {{ openaiGroups.length > 0 ? t('admin.openaiHealth.allGroups') : t('admin.openaiHealth.noGroups') }}
                </option>
                <option v-for="group in openaiGroups" :key="group.id" :value="group.name">
                  {{ group.name }}
                </option>
              </select>
              <input
                v-model="search"
                class="input max-w-sm"
                :placeholder="t('admin.openaiHealth.searchPlaceholder')"
                @input="handleSearch"
              >
            </div>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="overview?.items || []" :loading="loading">
            <template #cell-name="{ row }">
              <div>
                <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
                <div class="text-xs text-gray-500">{{ row.endpoint }}</div>
              </div>
            </template>
            <template #cell-group_name="{ row }">
              <span class="text-sm text-gray-900 dark:text-gray-100">{{ row.group_name || '-' }}</span>
            </template>
            <template #cell-tier>
              <span class="text-sm text-gray-500">-</span>
            </template>
            <template #cell-channel_price="{ row }">
              {{ row.total_checks }}
            </template>
            <template #cell-primary_status="{ row }">
              <span
                class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium"
                :class="statusClass(row.latest_status)"
              >
                {{ formatStatus(row.latest_status) }}
              </span>
            </template>
            <template #cell-primary_latency_ms="{ row }">
              <div class="font-semibold text-gray-900 dark:text-white">{{ formatLatency(row.avg_first_token_ms, true) }}</div>
              <div class="text-xs text-gray-500">
                P95: {{ formatLatency(row.p95_first_token_ms, false) }} · TCP: {{ formatLatency(row.avg_ping_latency_ms, false) }}
              </div>
            </template>
            <template #cell-availability_7d="{ row }">
              {{ formatPercent(row.availability_pct) }}
            </template>
            <template #cell-trend="{ row }">
              <div class="flex min-w-32 items-end gap-0.5">
                <span
                  v-for="(point, index) in row.trend"
                  :key="`${row.id}-${index}`"
                  class="inline-block w-1 rounded-sm"
                  :class="point.status === 'operational' ? 'bg-cyan-500' : 'bg-red-500'"
                  :style="{ height: pointHeight(point.latency_ms) }"
                  :title="`${formatStatus(point.status)} · ${formatLatency(point.latency_ms, true)} · ${formatDateTime(point.checked_at)}`"
                />
              </div>
            </template>
            <template #cell-last_checked_at="{ row }">
              {{ formatDateTime(row.last_checked_at) }}
            </template>
            <template #empty>
              <EmptyState
                :title="t('admin.openaiHealth.emptyTitle')"
                :description="t('admin.openaiHealth.emptyDescription')"
              />
            </template>
          </DataTable>
        </template>
      </TablePageLayout>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { MonitorStatus } from '@/api/admin/channelMonitor'
import type { OpenAIHealthOverview, OpenAIHealthWindow } from '@/api/admin/openaiHealth'
import type { Column } from '@/components/common/types'
import type { AdminGroup } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const openaiGroups = ref<AdminGroup[]>([])
const selectedGroupName = ref('')
const search = ref('')
const timeWindow = ref<OpenAIHealthWindow>('6h')
const overview = ref<OpenAIHealthOverview | null>(null)

let abortController: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const windowOptions: { value: OpenAIHealthWindow; label: string }[] = [
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
]

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.openaiHealth.columns.name'), sortable: false },
  { key: 'group_name', label: t('admin.openaiHealth.columns.group'), sortable: false },
  { key: 'tier', label: t('admin.openaiHealth.columns.tier'), sortable: false },
  { key: 'channel_price', label: t('admin.openaiHealth.columns.multiplier'), sortable: false },
  { key: 'primary_status', label: t('admin.openaiHealth.columns.latestStatus'), sortable: false },
  { key: 'primary_latency_ms', label: t('admin.openaiHealth.columns.firstToken'), sortable: false },
  { key: 'availability_7d', label: t('admin.openaiHealth.columns.availability'), sortable: false },
  { key: 'trend', label: t('admin.openaiHealth.columns.trend'), sortable: false },
  { key: 'last_checked_at', label: t('admin.openaiHealth.columns.lastChecked'), sortable: false },
])

const overviewMetrics = computed(() => {
  const data = overview.value
  return [
    { key: 'total', label: t('admin.openaiHealth.overview.primaryAccounts'), value: data?.total_monitors ?? 0 },
    { key: 'healthy', label: t('admin.openaiHealth.overview.healthyMonitors'), value: data?.healthy_monitors ?? 0 },
    { key: 'degraded', label: t('admin.openaiHealth.overview.degradedAccounts'), value: data?.degraded_monitors ?? 0 },
    { key: 'failed', label: t('admin.openaiHealth.overview.failedMonitors'), value: data?.failed_monitors ?? 0 },
    { key: 'availability', label: t('admin.openaiHealth.overview.avgAvailability'), value: formatPercent(data?.average_availability_pct) },
    { key: 'avgTtft', label: t('admin.openaiHealth.overview.avgTtft'), value: formatLatency(data?.average_first_token_ms, true) },
  ]
})

async function loadGroups() {
  openaiGroups.value = await adminAPI.groups.getAll('openai')
}

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    if (openaiGroups.value.length === 0) {
      await loadGroups()
    }
    const overviewRes = await adminAPI.openaiHealth.getOverview({
      group_name: selectedGroupName.value,
      search: search.value.trim(),
      window: timeWindow.value,
    }, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    overview.value = overviewRes
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.openaiHealth.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function handleSearch() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    reload()
  }, 300)
}

function pointHeight(value?: number | null): string {
  if (!value || value <= 0) return '8px'
  const h = Math.max(8, Math.min(34, Math.round(value / 80)))
  return `${h}px`
}

function formatPercent(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `${Math.round(value)}%`
}

function formatLatency(value?: number | null, spaced = false): string {
  if (!value) return '-'
  const unit = spaced ? ' ms' : 'ms'
  return `${Math.round(value)}${unit}`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

function formatStatus(status?: MonitorStatus | ''): string {
  if (!status) return '-'
  return status
}

function statusClass(status?: MonitorStatus | ''): string {
  if (status === 'operational') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (status === 'degraded') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'failed' || status === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

onMounted(reload)
</script>
