<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <template v-else-if="stats">
        <UserDashboardStats
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
        />
        <UserDashboardCharts
          v-model:startDate="startDate"
          v-model:endDate="endDate"
          v-model:granularity="granularity"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @dateRangeChange="loadCharts"
          @granularityChange="loadCharts"
          @refresh="refreshAll"
        />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2">
            <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
          </div>
          <div class="lg:col-span-1">
            <UserDashboardQuickActions />
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-2">
          <div v-if="gameCenterOverview">
            <UserGameCenterPreviewCard :overview="gameCenterOverview" />
          </div>
          <div
            v-else
            class="card flex min-h-[220px] items-center justify-center p-6 text-sm text-gray-500 dark:text-dark-400"
          >
            游戏中心预览暂不可用
          </div>

          <div v-if="usageLeaderboardOverview">
            <UserUsageLeaderboardPreviewCard :overview="usageLeaderboardOverview" />
          </div>
          <div
            v-else
            class="card flex min-h-[220px] items-center justify-center p-6 text-sm text-gray-500 dark:text-dark-400"
          >
            用量排行榜预览暂不可用
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import { gameCenterAPI } from '@/api/gameCenter'
import { usageLeaderboardAPI } from '@/api/usageLeaderboard'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserGameCenterPreviewCard from '@/components/user/dashboard/UserGameCenterPreviewCard.vue'
import UserUsageLeaderboardPreviewCard from '@/components/user/dashboard/UserUsageLeaderboardPreviewCard.vue'
import type {
  GameCenterOverview,
  ModelStat,
  PlatformQuotaItem,
  TrendDataPoint,
  UsageLeaderboardOverview,
  UsageLog,
} from '@/types'

const authStore = useAuthStore()
const user = computed(() => authStore.user)

const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const gameCenterOverview = ref<GameCenterOverview | null>(null)
const usageLeaderboardOverview = ref<UsageLeaderboardOverview | null>(null)

const startDate = ref(formatDateInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateInput(new Date()))
const granularity = ref('day')

async function loadStats() {
  loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

async function loadCharts() {
  loadingCharts.value = true
  try {
    const [trendResponse, modelsResponse] = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value as any,
      }),
      usageAPI.getDashboardModels({
        start_date: startDate.value,
        end_date: endDate.value,
      }),
    ])
    trendData.value = trendResponse.trend || []
    modelStats.value = modelsResponse.models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}

async function loadRecent() {
  loadingUsage.value = true
  try {
    const response = await usageAPI.getByDateRange(startDate.value, endDate.value)
    recentUsage.value = response.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

async function loadPlatformQuotas() {
  try {
    const data = await getMyPlatformQuotas()
    platformQuotas.value = data.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
    platformQuotas.value = []
  }
}

async function loadGameCenterPreview() {
  try {
    gameCenterOverview.value = await gameCenterAPI.getOverview({
      page: 1,
      page_size: 5,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
  } catch (error) {
    console.warn('Failed to load game center preview:', error)
    gameCenterOverview.value = null
  }
}

async function loadUsageLeaderboardPreview() {
  try {
    usageLeaderboardOverview.value = await usageLeaderboardAPI.getOverview({
      date: endDate.value,
      metric: 'tokens',
    })
  } catch (error) {
    console.warn('Failed to load usage leaderboard preview:', error)
    usageLeaderboardOverview.value = null
  }
}

function refreshAll() {
  void loadStats()
  void loadCharts()
  void loadRecent()
  void loadPlatformQuotas()
  void loadGameCenterPreview()
  void loadUsageLeaderboardPreview()
}

function formatDateInput(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

onMounted(() => {
  refreshAll()
})
</script>
