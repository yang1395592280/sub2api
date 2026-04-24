<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-amber-200/70 bg-gradient-to-br from-amber-50 via-white to-orange-50 p-6 shadow-sm shadow-amber-100/60 dark:border-amber-500/20 dark:from-slate-900 dark:via-slate-900 dark:to-amber-950/30 dark:shadow-none">
        <div class="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-2xl space-y-3">
            <div class="inline-flex w-fit items-center rounded-full bg-white/80 px-3 py-1 text-xs font-medium uppercase tracking-[0.28em] text-amber-600 ring-1 ring-amber-200/70 dark:bg-white/10 dark:text-amber-200 dark:ring-white/10">
              {{ t('sizeBet.statsPage.title') }}
            </div>
            <div>
              <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white">{{ t('sizeBet.statsPage.title') }}</h1>
              <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.statsPage.description') }}</p>
            </div>
            <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.statsPage.subtitle') }}</p>
          </div>
          <div class="flex flex-wrap items-end gap-3">
            <div>
              <label class="input-label">{{ t('sizeBet.statsPage.date') }}</label>
              <input v-model="statsDate" type="date" class="input w-44" />
            </div>
            <button type="button" class="btn btn-primary" :disabled="loading" @click="applyDateFilter">{{ t('common.apply') }}</button>
          </div>
        </div>
      </section>

      <div v-if="loading && !overview" class="flex justify-center py-16"><LoadingSpinner /></div>
      <section v-else-if="loadState === 'error'" class="card px-6 py-12">
        <EmptyState :title="t('sizeBet.loadError.title')" :description="errorMessage || t('sizeBet.loadError.description')" :action-text="t('common.retry')" @action="loadStats(true)" />
      </section>
      <template v-else>
        <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <div class="rounded-2xl bg-white p-5 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
            <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('admin.sizeBet.stats.participantCount') }}</p>
            <p class="mt-3 text-3xl font-semibold text-slate-900 dark:text-white">{{ overview?.participant_count ?? 0 }}</p>
          </div>
          <div class="rounded-2xl bg-white p-5 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
            <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('admin.sizeBet.stats.totalStake') }}</p>
            <p class="mt-3 text-3xl font-semibold text-slate-900 dark:text-white">{{ formatAmount(overview?.total_stake ?? 0) }}</p>
          </div>
          <div class="rounded-2xl bg-white p-5 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
            <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('admin.sizeBet.stats.totalUserNet') }}</p>
            <p class="mt-3 text-3xl font-semibold" :class="netAmountClass(overview?.total_user_net ?? 0)">{{ formatSignedAmount(overview?.total_user_net ?? 0) }}</p>
          </div>
          <div class="rounded-2xl border border-sky-200/70 bg-gradient-to-br from-sky-50 via-white to-indigo-50 p-5 text-slate-900 shadow-sm shadow-sky-100/70 dark:border-sky-400/20 dark:bg-gradient-to-br dark:from-slate-900 dark:via-slate-900 dark:to-sky-950/40 dark:text-white dark:shadow-none">
            <p class="text-xs uppercase tracking-[0.22em] text-sky-700/75 dark:text-sky-200/80">{{ t('admin.sizeBet.stats.houseNet') }}</p>
            <p class="mt-3 text-3xl font-semibold">{{ formatSignedAmount(overview?.house_net ?? 0) }}</p>
          </div>
        </section>

        <section class="card overflow-hidden">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.statsPage.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.statsPage.description') }}</p>
          </div>
          <div class="px-6 py-6">
            <DataTable :columns="columns" :data="rankedItems" :loading="loading">
              <template #empty>
                <EmptyState :title="t('sizeBet.statsPage.emptyTitle')" :description="t('sizeBet.statsPage.emptyDescription')" />
              </template>
            </DataTable>
          </div>
          <div v-if="pagination.total > 0" class="border-t border-slate-200/80 px-6 py-5 dark:border-white/10">
            <Pagination :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { sizeBetAPI } from '@/api'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores/app'
import type { SizeBetStatsOverview, SizeBetStatsUserItem } from '@/types/sizeBet'

type LoadState = 'loading' | 'ready' | 'error'
type RankedStatsUserItem = SizeBetStatsUserItem & { rank: number }

const { t } = useI18n()
const appStore = useAppStore()
const pageSize = getPersistedPageSize()
const loadState = ref<LoadState>('loading')
const loading = ref(false)
const errorMessage = ref('')
const statsDate = ref(new Date().toISOString().slice(0, 10))
const overview = ref<SizeBetStatsOverview | null>(null)
const items = ref<SizeBetStatsUserItem[]>([])
const pagination = reactive({ page: 1, page_size: pageSize, total: 0, pages: 1 })

const rankedItems = computed<RankedStatsUserItem[]>(() => items.value.map((item, index) => ({
  ...item,
  rank: (pagination.page - 1) * pagination.page_size + index + 1,
})))

const columns = computed<Column[]>(() => [
  { key: 'rank', label: t('sizeBet.statsPage.rank') },
  { key: 'username', label: t('admin.sizeBet.columns.user') },
  { key: 'total_stake', label: t('admin.sizeBet.stats.totalStake'), formatter: (value) => formatAmount(Number(value)) },
  { key: 'won_count', label: t('admin.sizeBet.stats.wonCount') },
  { key: 'lost_count', label: t('admin.sizeBet.stats.lostCount') },
  { key: 'refunded_count', label: t('admin.sizeBet.stats.refundedCount') },
  { key: 'net_result', label: t('admin.sizeBet.stats.totalUserNet'), formatter: (value) => formatSignedAmount(Number(value)) },
])

onMounted(() => {
  void loadStats()
})

function formatAmount(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2)
}

function formatSignedAmount(value: number) {
  const amount = formatAmount(Math.abs(value))
  if (value > 0) return `+${amount}`
  if (value < 0) return `-${amount}`
  return amount
}

function netAmountClass(value: number) {
  if (value > 0) return 'text-emerald-600 dark:text-emerald-300'
  if (value < 0) return 'text-rose-600 dark:text-rose-300'
  return 'text-slate-900 dark:text-white'
}

async function loadStats(force = false) {
  if (loading.value && !force) return
  loading.value = true
  loadState.value = overview.value ? 'ready' : 'loading'
  errorMessage.value = ''
  try {
    const [overviewResp, usersResp] = await Promise.all([
      sizeBetAPI.getStatsOverview(statsDate.value),
      sizeBetAPI.listStatsUsers(pagination.page, pagination.page_size, statsDate.value),
    ])
    overview.value = overviewResp
    items.value = usersResp.items
    Object.assign(pagination, {
      total: usersResp.total,
      pages: usersResp.pages,
      page: usersResp.page,
      page_size: usersResp.page_size,
    })
    loadState.value = 'ready'
  } catch (error: any) {
    loadState.value = 'error'
    errorMessage.value = error?.message || t('sizeBet.loadError.description')
    appStore.showError(errorMessage.value)
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadStats(true)
}

function handlePageSizeChange(nextPageSize: number) {
  Object.assign(pagination, { page: 1, page_size: nextPageSize })
  void loadStats(true)
}

function applyDateFilter() {
  pagination.page = 1
  void loadStats(true)
}
</script>
