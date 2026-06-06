<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-emerald-100 via-teal-50 to-cyan-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-emerald-500/10 dark:via-teal-500/5 dark:to-cyan-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">大小单双统计</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              按日期查看下注结果
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              这里展示的是 Size Bet 自己的下注统计模型，包括 total_stake、total_payout 和 total_user_net。
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <input v-model="selectedDate" type="date" class="input w-44" />
            <button type="button" class="btn btn-primary" :disabled="loading" @click="loadData">查询</button>
          </div>
        </div>
      </section>

      <div v-if="loading && !overview" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="overview">
        <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-5">
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">统计日期</p>
            <p class="mt-3 text-xl font-semibold text-gray-900 dark:text-white">{{ overview.date }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">参与人数</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.participant_count }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">Total Stake</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.total_stake }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">Total Payout</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ overview.total_payout }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">House Net</p>
            <p
              :class="overview.house_net >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'"
              class="mt-3 text-2xl font-semibold"
            >
              {{ overview.house_net >= 0 ? '+' : '' }}{{ overview.house_net }}
            </p>
          </article>
        </section>

        <section class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-5 flex items-center justify-between gap-3">
            <div>
              <p class="text-lg font-semibold text-gray-900 dark:text-white">用户统计明细</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按用户汇总当天的 stake、胜负场次和净输赢。</p>
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
                  <th class="pb-3 font-medium">用户名</th>
                  <th class="pb-3 font-medium">Total Stake</th>
                  <th class="pb-3 font-medium">Won</th>
                  <th class="pb-3 font-medium">Lost</th>
                  <th class="pb-3 font-medium">Refunded</th>
                  <th class="pb-3 font-medium">Net Result</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in items"
                  :key="item.username"
                  class="border-b border-gray-50 last:border-b-0 dark:border-dark-800"
                >
                  <td class="py-3 pr-4 text-gray-900 dark:text-white">{{ item.username }}</td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.total_stake }}</td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.won_count }}</td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.lost_count }}</td>
                  <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.refunded_count }}</td>
                  <td
                    :class="item.net_result >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'"
                    class="py-3 font-medium"
                  >
                    {{ item.net_result >= 0 ? '+' : '' }}{{ item.net_result }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <EmptyState
            v-else
            title="暂无统计数据"
            description="所选日期还没有产生可展示的大小单双用户统计。"
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
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { sizeBetAPI } from '@/api/gameCenter'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SizeBetStatsOverview, SizeBetStatsUserItem } from '@/types'

const appStore = useAppStore()

const overview = ref<SizeBetStatsOverview | null>(null)
const items = ref<SizeBetStatsUserItem[]>([])
const loading = ref(false)
const selectedDate = ref(formatDateInput(new Date()))
const pagination = ref({
  page: 1,
  pageSize: 20,
  total: 0,
})

async function loadData() {
  loading.value = true
  try {
    const [overviewData, usersData] = await Promise.all([
      sizeBetAPI.getStatsOverview({ date: selectedDate.value }),
      sizeBetAPI.getStatsUsers({
        date: selectedDate.value,
        page: pagination.value.page,
        page_size: pagination.value.pageSize,
      }),
    ])
    overview.value = overviewData
    items.value = usersData.items
    pagination.value.total = usersData.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '大小单双统计加载失败'))
  } finally {
    loading.value = false
  }
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

function formatDateInput(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

onMounted(loadData)
</script>
