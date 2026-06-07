<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card overflow-hidden">
        <div class="bg-gradient-to-r from-amber-50 via-orange-50 to-white px-6 py-6 dark:from-amber-950/30 dark:via-orange-950/20 dark:to-dark-800">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <div class="inline-flex items-center rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
                {{ t('admin.dashboard.spendingRankingUsage') }}
              </div>
              <h1 class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
                {{ t('admin.dashboard.spendingRankingTitle') }}
              </h1>
              <p class="mt-2 max-w-2xl text-sm text-gray-600 dark:text-gray-300">
                {{ t('admin.dashboard.spendingRankingDescription') }}
              </p>
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.spendingRankingSpend') }}</div>
                <div class="mt-1 text-xl font-semibold text-amber-600 dark:text-amber-300">${{ formatCost(summary.total_actual_cost) }}</div>
              </div>
              <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.spendingRankingRequests') }}</div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatNumber(summary.total_requests) }}</div>
              </div>
              <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.rankUsers') }}</div>
                <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatNumber(summary.total) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-4">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.timeRange') }}:</span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="handleDateRangeChange"
            />
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.dashboard.sortBy') }}:</span>
            <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-600 dark:bg-dark-800">
              <button
                v-for="option in sortOptions"
                :key="option.value"
                type="button"
                class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                :class="sortBy === option.value
                  ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                  : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
                :data-test="`sort-${option.value}`"
                @click="handleSortByChange(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              data-test="sort-order"
              @click="toggleSortOrder"
            >
              {{ sortOrderLabel }}
            </button>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadRanking">
            {{ t('common.refresh') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            data-test="export"
            :disabled="exporting"
            @click="exportAllRanking"
          >
            {{ exporting ? t('usage.exporting') : t('admin.dashboard.exportRanking') }}
          </button>
        </div>
      </div>

      <div v-if="loading && !rows.length" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <TopUsersLeaderboard
        v-else
        :users="rows"
        :summary="summary"
        :subtitle="subtitle"
        :empty-text="t('admin.dashboard.noDataAvailable')"
        :clickable="true"
        @select="handleSelectUser"
      />

      <div class="card p-0">
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { saveAs } from 'file-saver'
import { adminAPI } from '@/api/admin'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import type { UserSpendingRankingItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TopUsersLeaderboard from '@/components/admin/payment/TopUsersLeaderboard.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

const defaultRange = getLast24HoursRangeDates()
const parseSortBy = (value: unknown): 'actual_cost' | 'requests' | 'tokens' => {
  if (value === 'requests' || value === 'tokens') return value
  return 'actual_cost'
}

const parseSortOrder = (value: unknown): 'asc' | 'desc' => {
  if (value === 'asc') return 'asc'
  return 'desc'
}

const parsePositiveInt = (value: unknown, fallback: number): number => {
  if (typeof value !== 'string') return fallback
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

const startDate = ref(typeof route.query.start_date === 'string' ? route.query.start_date : defaultRange.start)
const endDate = ref(typeof route.query.end_date === 'string' ? route.query.end_date : defaultRange.end)
const loading = ref(false)
const exporting = ref(false)
const rows = ref<UserSpendingRankingItem[]>([])
const sortBy = ref<'actual_cost' | 'requests' | 'tokens'>(parseSortBy(route.query.sort_by))
const sortOrder = ref<'asc' | 'desc'>(parseSortOrder(route.query.sort_order))
const summary = ref({
  total_actual_cost: 0,
  total_requests: 0,
  total: 0
})
const pagination = ref({
  page: parsePositiveInt(route.query.page, 1),
  page_size: parsePositiveInt(route.query.page_size, getPersistedPageSize()),
  total: 0,
  pages: 0
})

const subtitle = computed(() =>
  t('admin.dashboard.spendingRankingWindow', {
    start: startDate.value,
    end: endDate.value
  })
)

const sortOptions = computed(() => [
  { value: 'actual_cost' as const, label: t('admin.dashboard.spendingRankingSpend') },
  { value: 'requests' as const, label: t('admin.dashboard.spendingRankingRequests') },
  { value: 'tokens' as const, label: t('admin.dashboard.spendingRankingTokens') }
])

const sortOrderLabel = computed(() =>
  sortOrder.value === 'desc' ? t('admin.dashboard.sortDesc') : t('admin.dashboard.sortAsc')
)

function formatNumber(value: number): string {
  return value.toLocaleString()
}

function formatCost(value: number): string {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}K`
  if (value >= 1) return value.toFixed(2)
  if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}

function syncRouteQuery(): void {
  void router.replace({
    query: {
      ...route.query,
      start_date: startDate.value,
      end_date: endDate.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      page: String(pagination.value.page),
      page_size: String(pagination.value.page_size)
    }
  })
}

async function loadRanking(): Promise<void> {
  loading.value = true
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      page: pagination.value.page,
      page_size: pagination.value.page_size,
      sort_by: sortBy.value,
      sort_order: sortOrder.value
    })
    rows.value = response.ranking || []
    summary.value = {
      total_actual_cost: response.total_actual_cost || 0,
      total_requests: response.total_requests || 0,
      total: response.total || 0
    }
    pagination.value = {
      page: response.page || pagination.value.page,
      page_size: response.page_size || pagination.value.page_size,
      total: response.total || 0,
      pages: response.pages || 0
    }
  } catch (error) {
    console.error('Failed to load user spending ranking:', error)
    appStore.showError(t('admin.dashboard.failedToLoad'))
    rows.value = []
    summary.value = { total_actual_cost: 0, total_requests: 0, total: 0 }
    pagination.value.total = 0
    pagination.value.pages = 0
  } finally {
    loading.value = false
  }
}

function handleDateRangeChange(): void {
  pagination.value.page = 1
  syncRouteQuery()
  void loadRanking()
}

function handleSortByChange(nextSortBy: 'actual_cost' | 'requests' | 'tokens'): void {
  if (sortBy.value === nextSortBy) return
  sortBy.value = nextSortBy
  pagination.value.page = 1
  syncRouteQuery()
  void loadRanking()
}

function toggleSortOrder(): void {
  sortOrder.value = sortOrder.value === 'desc' ? 'asc' : 'desc'
  pagination.value.page = 1
  syncRouteQuery()
  void loadRanking()
}

function handlePageChange(page: number): void {
  pagination.value.page = page
  syncRouteQuery()
  void loadRanking()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  syncRouteQuery()
  void loadRanking()
}

function handleSelectUser(user: { user_id: number }): void {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(user.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

async function exportAllRanking(): Promise<void> {
  if (exporting.value) return
  if (summary.value.total <= 0) {
    appStore.showError(t('usage.noDataToExport'))
    return
  }

  exporting.value = true
  try {
    const XLSX = await import('xlsx')
    const headers = [
      t('admin.dashboard.spendingRankingUser'),
      t('admin.dashboard.spendingRankingUserId'),
      t('admin.dashboard.spendingRankingSpend'),
      t('admin.dashboard.spendingRankingRequests'),
      t('admin.dashboard.spendingRankingTokens')
    ]
    const ws = XLSX.utils.aoa_to_sheet([headers])
    let page = 1
    const pageSize = 100

    while (true) {
      const response = await adminAPI.dashboard.getUserSpendingRanking({
        start_date: startDate.value,
        end_date: endDate.value,
        page,
        page_size: pageSize,
        sort_by: sortBy.value,
        sort_order: sortOrder.value
      })
      const exportRows = (response.ranking || []).map((item) => [
        item.email || t('admin.redeem.userPrefix', { id: item.user_id }),
        item.user_id,
        item.actual_cost.toFixed(6),
        item.requests,
        item.tokens
      ])
      if (exportRows.length > 0) {
        XLSX.utils.sheet_add_aoa(ws, exportRows, { origin: -1 })
      }
      if (page >= (response.pages || 0) || exportRows.length < pageSize) {
        break
      }
      page += 1
    }

    const wb = XLSX.utils.book_new()
    XLSX.utils.book_append_sheet(wb, ws, 'UserSpendingRanking')
    saveAs(
      new Blob([XLSX.write(wb, { bookType: 'xlsx', type: 'array' })], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
      }),
      `user_spending_ranking_${startDate.value}_to_${endDate.value}_${sortBy.value}_${sortOrder.value}.xlsx`
    )
    appStore.showSuccess(t('usage.exportExcelSuccess'))
  } catch (error) {
    console.error('Failed to export user spending ranking:', error)
    appStore.showError(t('usage.exportExcelFailed'))
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  syncRouteQuery()
  void loadRanking()
})
</script>
