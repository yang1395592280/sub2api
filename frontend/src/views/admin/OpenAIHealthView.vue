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
          <RouterLink class="btn btn-secondary" to="/admin/accounts?platform=openai">
            {{ t('admin.openaiHealth.accountTab') }}
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
              <select v-model.number="selectedGroupId" class="input max-w-xs" @change="handleGroupChange">
                <option :value="0">
                  {{ openaiGroups.length > 0 ? t('admin.openaiHealth.allGroups') : t('admin.openaiHealth.noGroups') }}
                </option>
                <option v-for="group in openaiGroups" :key="group.id" :value="group.id">
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
          <DataTable :columns="columns" :data="accounts" :loading="loading">
            <template #cell-account_name="{ row }">
              <div>
                <div class="font-medium text-gray-900 dark:text-white">{{ row.account_name }}</div>
                <div class="text-xs text-gray-500">
                  #{{ row.account_id }} · {{ row.type }} · {{ row.status }}
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
            <template #cell-health_score="{ row }">
              <div class="font-semibold text-gray-900 dark:text-white">{{ row.health.health_score.toFixed(1) }}</div>
              <div v-if="row.health.degrade_reason" class="text-xs text-red-500">{{ row.health.degrade_reason }}</div>
            </template>
            <template #cell-success_rate="{ row }">
              {{ formatRatio(row.health.success_rate_ewma) }}
            </template>
            <template #cell-ttft="{ row }">
              {{ formatLatency(row.health.ttft_ewma_ms) }}
            </template>
            <template #cell-channel_price="{ row }">
              {{ formatChannelPrice(row.channel_price) }}
            </template>
            <template #cell-select_count="{ row }">
              {{ accountStat(row.account_id)?.select_count ?? 0 }}
            </template>
            <template #cell-last_selected_at="{ row }">
              {{ formatDateTime(accountStat(row.account_id)?.last_selected_at || row.health.last_selected_at) }}
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
import type {
  OpenAISchedulerAccount,
  OpenAISchedulerAccountDailyStat,
  OpenAISchedulerDailyStats,
  OpenAISchedulerOverview,
  OpenAISchedulerTier,
} from '@/api/admin/openaiScheduler'
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
const selectedGroupId = ref(0)
const search = ref('')
const overview = ref<OpenAISchedulerOverview | null>(null)
const accounts = ref<OpenAISchedulerAccount[]>([])
const dailyStats = ref<OpenAISchedulerDailyStats | null>(null)

let abortController: AbortController | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'account_name', label: t('admin.openaiHealth.columns.account'), sortable: false },
  { key: 'tier', label: t('admin.openaiHealth.columns.tier'), sortable: false },
  { key: 'health_score', label: t('admin.openaiHealth.columns.healthScore'), sortable: false },
  { key: 'success_rate', label: t('admin.openaiHealth.columns.successRate'), sortable: false },
  { key: 'ttft', label: t('admin.openaiHealth.columns.ttft'), sortable: false },
  { key: 'channel_price', label: t('admin.openaiHealth.columns.channelPrice'), sortable: false },
  { key: 'select_count', label: t('admin.openaiHealth.columns.selectCount'), sortable: false },
  { key: 'last_selected_at', label: t('admin.openaiHealth.columns.lastSelected'), sortable: false },
])

const overviewMetrics = computed(() => {
  const tierCounts = overview.value?.tier_counts || {}
  const total = accounts.value.length
  const active = accounts.value.filter((item) => item.status === 'active').length
  const avgHealth = average(accounts.value.map((item) => item.health.health_score))
  const avgTtft = average(accounts.value.map((item) => item.health.ttft_ewma_ms).filter((value) => value > 0))
  return [
    { key: 'total', label: t('admin.openaiHealth.overview.totalAccounts'), value: total },
    { key: 'active', label: t('admin.openaiHealth.overview.activeAccounts'), value: active },
    { key: 'primary', label: t('admin.openaiScheduler.tier.primary'), value: tierCounts.primary ?? 0 },
    { key: 'degraded', label: t('admin.openaiScheduler.tier.degraded'), value: tierCounts.degraded ?? 0 },
    { key: 'avgHealth', label: t('admin.openaiHealth.overview.avgHealth'), value: formatScore(avgHealth) },
    { key: 'avgTtft', label: t('admin.openaiHealth.overview.avgTtft'), value: formatLatency(avgTtft) },
  ]
})

const statsByAccount = computed(() => {
  const result = new Map<number, OpenAISchedulerAccountDailyStat>()
  for (const item of dailyStats.value?.accounts || []) {
    result.set(item.account_id, item)
  }
  return result
})

async function loadGroups() {
  openaiGroups.value = await adminAPI.groups.getAll('openai')
  if (!selectedGroupId.value && openaiGroups.value.length > 0) {
    selectedGroupId.value = openaiGroups.value[0].id
  }
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
    const groupId = selectedGroupId.value || undefined
    const [overviewRes, accountsRes, statsRes] = await Promise.all([
      adminAPI.openaiScheduler.getOverview({ group_id: groupId }),
      adminAPI.openaiScheduler.listAccounts({
        group_id: groupId,
        search: search.value.trim(),
        page: 1,
        page_size: 200,
      }, { signal: ctrl.signal }),
      groupId
        ? adminAPI.openaiScheduler.getDailyStats({ group_id: groupId, date: todayDateString() })
        : Promise.resolve(null),
    ])
    if (ctrl.signal.aborted || abortController !== ctrl) return
    overview.value = overviewRes
    accounts.value = accountsRes.items || []
    dailyStats.value = statsRes
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

function handleGroupChange() {
  reload()
}

function average(values: number[]): number | null {
  if (values.length === 0) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

function formatRatio(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return '-'
  return `${Math.round(value * 100)}%`
}

function formatScore(value?: number | null): string {
  if (value == null || Number.isNaN(value)) return '-'
  return value.toFixed(1)
}

function formatLatency(value?: number | null): string {
  if (!value) return '-'
  return `${Math.round(value)}ms`
}

function formatChannelPrice(value?: number | null): string {
  return value != null ? value.toFixed(3) : '-'
}

function accountStat(accountId: number): OpenAISchedulerAccountDailyStat | undefined {
  return statsByAccount.value.get(accountId)
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

onMounted(reload)
</script>
