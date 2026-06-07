<template>
  <div class="card overflow-hidden">
    <div
      class="border-b border-amber-100 bg-gradient-to-r from-amber-50 via-orange-50 to-white px-5 py-4 dark:border-amber-900/40 dark:from-amber-950/30 dark:via-orange-950/20 dark:to-dark-800"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="inline-flex items-center rounded-full bg-amber-100 px-2.5 py-1 text-[11px] font-semibold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">
            {{ t('admin.dashboard.spendingRankingUsage') }}
          </div>
          <h3 class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
            {{ title || t('admin.dashboard.spendingRankingTitle') }}
          </h3>
          <p v-if="subtitle" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ subtitle }}
          </p>
        </div>
        <button
          v-if="showViewAll"
          type="button"
          class="btn btn-secondary btn-sm"
          @click="$emit('view-all')"
        >
          {{ t('admin.dashboard.viewAllRanking') }}
        </button>
      </div>
      <div v-if="summary" class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.spendingRankingSpend') }}</div>
          <div class="mt-1 text-xl font-semibold text-amber-600 dark:text-amber-300">${{ formatCost(summary.total_actual_cost) }}</div>
        </div>
        <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.spendingRankingRequests') }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatNumber(summary.total_requests) }}</div>
        </div>
        <div class="rounded-2xl bg-white/80 px-4 py-3 shadow-sm ring-1 ring-amber-100 dark:bg-dark-700/70 dark:ring-amber-900/30">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.rankUsers') }}</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ formatNumber(summary.total) }}</div>
        </div>
      </div>
    </div>
    <div
      v-if="!users?.length"
      class="flex h-48 items-center justify-center px-4 text-sm text-gray-500 dark:text-gray-400"
    >
      {{ emptyText || t('admin.dashboard.noDataAvailable') }}
    </div>
    <div v-else class="space-y-2 px-4 py-4">
      <div class="hidden grid-cols-[72px_minmax(0,1fr)_120px_120px_120px] gap-3 px-2 text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-gray-500 lg:grid">
        <span>{{ t('admin.dashboard.spendingRankingTitle') }}</span>
        <span>{{ t('admin.dashboard.spendingRankingUser') }}</span>
        <span class="text-right">{{ t('admin.dashboard.spendingRankingRequests') }}</span>
        <span class="text-right">{{ t('admin.dashboard.spendingRankingTokens') }}</span>
        <span class="text-right">{{ t('admin.dashboard.spendingRankingSpend') }}</span>
      </div>
      <div
        v-for="(user, idx) in users"
        :key="user.user_id"
        class="grid gap-3 rounded-2xl border border-gray-100 px-4 py-4 transition-colors hover:border-amber-200 hover:bg-amber-50/60 dark:border-dark-700 dark:hover:border-amber-900/40 dark:hover:bg-dark-700/70 lg:grid-cols-[72px_minmax(0,1fr)_120px_120px_120px]"
        :class="clickable ? 'cursor-pointer' : ''"
        @click="handleRowClick(user)"
      >
        <div class="flex items-center gap-3">
          <span
            :class="[
              'flex h-10 w-10 items-center justify-center rounded-2xl text-sm font-bold shadow-sm',
              rankClass(idx),
            ]"
          >
            {{ idx + 1 }}
          </span>
        </div>
        <div class="min-w-0">
          <div class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ user.email || t('admin.redeem.userPrefix', { id: user.user_id }) }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">ID: {{ user.user_id }}</div>
        </div>
        <div class="text-left lg:text-right">
          <div class="text-[11px] text-gray-400 dark:text-gray-500 lg:hidden">{{ t('admin.dashboard.spendingRankingRequests') }}</div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">{{ formatNumber(user.requests ?? 0) }}</div>
        </div>
        <div class="text-left lg:text-right">
          <div class="text-[11px] text-gray-400 dark:text-gray-500 lg:hidden">{{ t('admin.dashboard.spendingRankingTokens') }}</div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">{{ formatTokens(user.tokens ?? 0) }}</div>
        </div>
        <div class="text-left lg:text-right">
          <div class="text-[11px] text-gray-400 dark:text-gray-500 lg:hidden">{{ t('admin.dashboard.spendingRankingSpend') }}</div>
          <div class="text-base font-semibold text-amber-600 dark:text-amber-300">${{ formatCost(user.actual_cost ?? user.amount ?? 0) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface RankingUser {
  user_id: number
  email: string
  amount?: number
  actual_cost?: number
  requests?: number
  tokens?: number
}

const props = withDefaults(defineProps<{
  users: RankingUser[]
  title?: string
  subtitle?: string
  emptyText?: string
  clickable?: boolean
  showViewAll?: boolean
  summary?: {
    total_actual_cost: number
    total_requests: number
    total: number
  } | null
}>(), {
  title: '',
  subtitle: '',
  emptyText: '',
  clickable: false,
  showViewAll: false,
  summary: null
})

const emit = defineEmits<{
  (e: 'select', user: RankingUser): void
  (e: 'view-all'): void
}>()

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (idx === 1) return 'bg-slate-200 text-slate-700 dark:bg-slate-700 dark:text-slate-200'
  if (idx === 2) return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

function formatNumber(value: number): string {
  return value.toLocaleString()
}

function formatTokens(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

function formatCost(value: number): string {
  if (value >= 1000) return `${(value / 1000).toFixed(2)}K`
  if (value >= 1) return value.toFixed(2)
  if (value >= 0.01) return value.toFixed(3)
  return value.toFixed(4)
}

function handleRowClick(user: RankingUser): void {
  if (!props.clickable) return
  emit('select', user)
}
</script>
