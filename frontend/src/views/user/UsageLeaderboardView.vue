<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-sky-100 via-cyan-50 to-teal-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-sky-500/10 dark:via-cyan-500/5 dark:to-teal-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-sky-700 dark:text-sky-300">Usage Leaderboard</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              全站用量排行榜
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              支持选择统计日期，并在 Requests 与 Tokens 两种维度之间切换。排行榜契约只使用这两个 metric。
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model="date" type="date" class="input w-44" />
            <div class="flex rounded-full bg-white/80 p-1 ring-1 ring-sky-100 dark:bg-dark-900/70 dark:ring-sky-500/20">
              <button
                v-for="option in metricOptions"
                :key="option"
                type="button"
                :class="metric === option
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 dark:text-dark-200'"
                class="rounded-full px-4 py-2 text-sm font-medium transition"
                @click="changeMetric(option)"
              >
                {{ option }}
              </button>
            </div>
            <button type="button" class="btn btn-primary" :disabled="loading" @click="loadData">刷新榜单</button>
          </div>
        </div>
      </section>

      <div v-if="loading && !overview" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="overview">
        <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">统计日期</p>
            <p class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">{{ overview.date }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">统计维度</p>
            <p class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">{{ displayMetric }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">参与人数</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.participant_count }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">我的排名</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.current_user?.rank ?? '-' }}</p>
          </article>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">Top 3 预览</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">优先展示本日当前维度下的前三名。</p>
            </div>
            <div v-if="overview.top_items.length" class="space-y-3">
              <div
                v-for="item in overview.top_items.slice(0, 3)"
                :key="item.user_id"
                class="rounded-2xl border border-gray-100 px-4 py-3 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                      #{{ item.rank }} {{ item.username || item.email }}
                    </p>
                    <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ item.email }}</p>
                  </div>
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ formatMetricValue(item.value) }}</p>
                </div>
              </div>
            </div>
            <EmptyState
              v-else
              title="暂无排行榜预览"
              description="当前日期和维度下还没有产生榜单数据。"
            />
          </div>

          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">当前用户位置</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">若当前用户未上榜，则显示为空。</p>
            </div>
            <div v-if="overview.current_user" class="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">排名</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ overview.current_user.rank }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">用户名</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ overview.current_user.username || overview.current_user.email }}
                </p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ displayMetric }}</p>
                <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ formatMetricValue(overview.current_user.value) }}
                </p>
              </div>
            </div>
            <EmptyState
              v-else
              title="当前用户未上榜"
              description="说明所选日期下当前用户在此维度没有可统计数据。"
            />
          </div>
        </section>

        <section class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-5 flex items-center justify-between gap-3">
            <div>
              <p class="text-lg font-semibold text-gray-900 dark:text-white">完整榜单</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">分页展示当日所有可统计用户。</p>
            </div>
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadData">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              <span class="ml-2">刷新</span>
            </button>
          </div>

          <div v-if="items.length" class="overflow-x-auto">
            <table class="min-w-full text-left text-sm">
              <thead>
                <tr class="border-b border-gray-100 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="pb-3 font-medium">Rank</th>
                  <th class="pb-3 font-medium">User</th>
                  <th class="pb-3 font-medium">Requests</th>
                  <th class="pb-3 font-medium">Tokens</th>
                  <th class="pb-3 font-medium">{{ displayMetric }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in items"
                  :key="`${item.user_id}-${item.rank}`"
                  :class="item.is_current_user ? 'bg-primary-50/80 dark:bg-primary-500/10' : ''"
                  class="border-b border-gray-50 last:border-b-0 dark:border-dark-800"
                >
                  <td class="py-3 pr-4 text-gray-900 dark:text-white">#{{ item.rank }}</td>
                  <td class="py-3 pr-4">
                    <p class="font-medium text-gray-900 dark:text-white">{{ item.username || item.email }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ item.email }}</p>
                  </td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.requests }}</td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ formatMetricValue(item.tokens) }}</td>
                  <td class="py-3 font-semibold text-gray-900 dark:text-white">{{ formatMetricValue(item.value) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <EmptyState
            v-else
            title="暂无榜单数据"
            description="所选日期和维度下还没有用量排行。"
          />

          <Pagination
            v-if="pagination.total > pagination.pageSize"
            :total="pagination.total"
            :page="pagination.page"
            :page-size="pagination.pageSize"
            @update:page="handlePageChange"
            @update:page-size="handlePageSizeChange"
          />
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { usageLeaderboardAPI } from '@/api/usageLeaderboard'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber } from '@/utils/format'
import type { UsageLeaderboardItem, UsageLeaderboardMetric, UsageLeaderboardOverview } from '@/types'

const appStore = useAppStore()

const metricOptions: UsageLeaderboardMetric[] = ['requests', 'tokens']
const date = ref(formatDateInput(new Date()))
const metric = ref<UsageLeaderboardMetric>('requests')
const overview = ref<UsageLeaderboardOverview | null>(null)
const items = ref<UsageLeaderboardItem[]>([])
const loading = ref(false)
const pagination = ref({
  page: 1,
  pageSize: 20,
  total: 0,
})

const displayMetric = computed(() => (metric.value === 'requests' ? 'Requests' : 'Tokens'))

async function loadData() {
  loading.value = true
  try {
    const params = {
      date: date.value,
      metric: metric.value,
    }
    const [overviewData, itemsData] = await Promise.all([
      usageLeaderboardAPI.getOverview(params),
      usageLeaderboardAPI.getItems({
        ...params,
        page: pagination.value.page,
        page_size: pagination.value.pageSize,
      }),
    ])
    overview.value = overviewData
    items.value = itemsData.items
    pagination.value.total = itemsData.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '用量排行榜加载失败'))
  } finally {
    loading.value = false
  }
}

async function changeMetric(nextMetric: UsageLeaderboardMetric) {
  if (metric.value === nextMetric) return
  metric.value = nextMetric
  pagination.value.page = 1
  await loadData()
}

async function handlePageChange(page: number) {
  pagination.value.page = page
  await loadData()
}

async function handlePageSizeChange(pageSize: number) {
  pagination.value.pageSize = pageSize
  pagination.value.page = 1
  await loadData()
}

function formatMetricValue(value: number): string {
  if (metric.value === 'requests') return String(value)
  return formatCompactNumber(value)
}

function formatDateInput(dateValue: Date): string {
  const year = dateValue.getFullYear()
  const month = String(dateValue.getMonth() + 1).padStart(2, '0')
  const day = String(dateValue.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

onMounted(loadData)
</script>
