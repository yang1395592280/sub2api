<template>
  <div class="card overflow-hidden">
    <div class="border-b border-gray-100 bg-gradient-to-r from-sky-50 via-cyan-50 to-teal-50 px-6 py-5 dark:border-dark-700 dark:from-sky-500/10 dark:via-cyan-500/10 dark:to-teal-500/10">
      <div class="flex items-start justify-between gap-4">
        <div>
          <p class="text-sm font-medium text-sky-700 dark:text-sky-300">全站用量排行榜</p>
          <h2 class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ displayMetricLabel }} 榜单预览
          </h2>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
            统计日期 {{ overview.date }}，支持按 Requests 或 Tokens 查看当日排名。
          </p>
        </div>
        <div class="rounded-2xl bg-white/80 p-3 text-sky-600 shadow-sm ring-1 ring-sky-100 dark:bg-dark-900/70 dark:text-sky-300 dark:ring-sky-500/20">
          <Icon name="chart" size="lg" />
        </div>
      </div>
    </div>

    <div class="space-y-5 p-6">
      <div class="grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-sky-50 p-4 dark:bg-sky-500/10">
          <p class="text-xs font-medium uppercase tracking-wide text-sky-700 dark:text-sky-300">参与人数</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ formatCompactNumber(overview.participant_count, { allowBillions: false }) }}
          </p>
        </div>
        <div class="rounded-2xl bg-teal-50 p-4 dark:bg-teal-500/10">
          <p class="text-xs font-medium uppercase tracking-wide text-teal-700 dark:text-teal-300">我的排名</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ overview.current_user?.rank ?? '-' }}
          </p>
        </div>
      </div>

      <div class="space-y-2">
        <div
          v-for="item in topItems"
          :key="item.user_id"
          class="flex items-center justify-between rounded-2xl border border-gray-100 px-4 py-3 dark:border-dark-700"
        >
          <div class="flex min-w-0 items-center gap-3">
            <span
              :class="rankClass(item.rank)"
              class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-xs font-semibold"
            >
              {{ item.rank }}
            </span>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.username || item.email }}</p>
              <p class="truncate text-xs text-gray-500 dark:text-dark-400">{{ item.email }}</p>
            </div>
          </div>
          <div class="text-right">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ formatMetricValue(item.value) }}
            </p>
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ displayMetricLabel }}</p>
          </div>
        </div>
      </div>

      <RouterLink
        to="/usage-leaderboard"
        class="inline-flex items-center gap-2 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200"
      >
        打开完整榜单
        <Icon name="arrowRight" size="sm" />
      </RouterLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCompactNumber } from '@/utils/format'
import type { UsageLeaderboardOverview } from '@/types'

interface Props {
  overview: UsageLeaderboardOverview
}

const props = defineProps<Props>()

const topItems = computed(() => props.overview.top_items.slice(0, 3))
const displayMetricLabel = computed(() => (
  props.overview.metric === 'requests' ? 'Requests' : 'Tokens'
))

function formatMetricValue(value: number): string {
  return formatCompactNumber(value)
}

function rankClass(rank: number): string {
  if (rank === 1) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/15 dark:text-yellow-300'
  if (rank === 2) return 'bg-slate-100 text-slate-700 dark:bg-slate-500/15 dark:text-slate-300'
  if (rank === 3) return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-300'
}
</script>
