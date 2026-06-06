<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-fuchsia-100 via-rose-50 to-amber-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-fuchsia-500/10 dark:via-rose-500/5 dark:to-amber-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-fuchsia-700 dark:text-fuchsia-300">Lucky Wheel</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              每日积分转盘
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              每次抽奖都会直接影响积分余额，奖项可能是奖励、惩罚或谢谢参与，不存在余额与积分互换逻辑。
            </p>
          </div>
          <div class="flex flex-wrap gap-3">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadData">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              <span class="ml-2">刷新</span>
            </button>
            <button
              type="button"
              class="btn btn-primary"
              :disabled="!canSpin || spinning"
              @click="handleSpin"
            >
              {{ spinning ? '抽奖中...' : canSpin ? '立即抽奖' : '今日次数已用完' }}
            </button>
          </div>
        </div>
      </section>

      <div v-if="loading && !overview" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="overview">
        <section v-if="lastSpinResult" class="rounded-3xl border border-fuchsia-200 bg-fuchsia-50 p-5 dark:border-fuchsia-500/30 dark:bg-fuchsia-500/10">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p class="text-sm font-medium text-fuchsia-700 dark:text-fuchsia-300">最近一次结果</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                {{ lastSpinResult.record.prize_label }}
              </p>
            </div>
            <p
              :class="lastSpinResult.record.delta_points >= 0
                ? 'text-emerald-600 dark:text-emerald-300'
                : 'text-rose-600 dark:text-rose-300'"
              class="text-2xl font-semibold"
            >
              {{ lastSpinResult.record.delta_points >= 0 ? '+' : '' }}{{ lastSpinResult.record.delta_points }} 积分
            </p>
          </div>
        </section>

        <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">当前积分</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ formatCompactNumber(overview.points) }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">今日剩余次数</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ overview.spins_remaining_today }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">最低参与门槛</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ overview.min_points_required }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">今日已抽次数</p>
            <p class="mt-3 text-3xl font-semibold text-gray-900 dark:text-white">{{ overview.spins_used_today }}</p>
          </article>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">奖池配置</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">每个奖项都直接以积分变化表示。</p>
            </div>

            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <article
                v-for="prize in overview.prizes"
                :key="prize.key"
                class="rounded-3xl border border-gray-100 p-5 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-base font-semibold text-gray-900 dark:text-white">{{ prize.label }}</p>
                    <p class="mt-1 text-xs uppercase tracking-wide text-gray-500 dark:text-dark-400">{{ prize.type }}</p>
                  </div>
                  <span
                    :class="prize.delta_points > 0
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
                      : prize.delta_points < 0
                        ? 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'
                        : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-300'"
                    class="rounded-full px-3 py-1 text-xs font-medium"
                  >
                    {{ prize.delta_points > 0 ? '+' : '' }}{{ prize.delta_points }} 积分
                  </span>
                </div>
                <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">
                  概率 {{ formatProbability(prize.probability) }}
                </p>
              </article>
            </div>
          </div>

          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5 flex items-center justify-between gap-3">
              <div>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">今日排行榜</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ leaderboardDate }}</p>
              </div>
            </div>

            <div v-if="leaderboardItems.length" class="space-y-3">
              <div
                v-for="item in leaderboardItems.slice(0, 5)"
                :key="item.user_id"
                class="rounded-2xl border border-gray-100 px-4 py-3 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                      #{{ item.rank }} {{ item.username || item.email }}
                    </p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                      最佳奖项：{{ item.best_prize_label || '-' }}
                    </p>
                  </div>
                  <div class="text-right">
                    <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ item.spin_count }} 次</p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">
                      净值 {{ item.net_points >= 0 ? '+' : '' }}{{ item.net_points }}
                    </p>
                  </div>
                </div>
              </div>
            </div>
            <EmptyState
              v-else
              title="暂无榜单数据"
              description="今天还没有产生 Lucky Wheel 排名。"
            />
          </div>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">最近开奖记录</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">展示最近的积分增减情况和落点结果。</p>
            </div>
            <div v-if="historyItems.length" class="overflow-x-auto">
              <table class="min-w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-100 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="pb-3 font-medium">时间</th>
                    <th class="pb-3 font-medium">奖项</th>
                    <th class="pb-3 font-medium">变化</th>
                    <th class="pb-3 font-medium">抽奖后积分</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="record in historyItems"
                    :key="record.id"
                    class="border-b border-gray-50 last:border-b-0 dark:border-dark-800"
                  >
                    <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ formatDateTime(record.created_at) }}</td>
                    <td class="py-3 pr-4 text-gray-900 dark:text-white">{{ record.prize_label }}</td>
                    <td
                      :class="record.delta_points >= 0
                        ? 'text-emerald-600 dark:text-emerald-300'
                        : 'text-rose-600 dark:text-rose-300'"
                      class="py-3 pr-4 font-medium"
                    >
                      {{ record.delta_points >= 0 ? '+' : '' }}{{ record.delta_points }}
                    </td>
                    <td class="py-3 text-gray-600 dark:text-dark-300">{{ record.points_after }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <EmptyState
              v-else
              title="暂无开奖记录"
              description="你或其他用户还没有产生可展示的 Lucky Wheel 历史。"
            />
          </div>

          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">玩法规则</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">内容由后端规则配置直接渲染。</p>
            </div>
            <div
              v-if="rulesHtml"
              class="prose prose-sm max-w-none dark:prose-invert"
              v-html="rulesHtml"
            ></div>
            <EmptyState
              v-else
              title="暂无规则说明"
              description="当前没有可展示的 Lucky Wheel 规则文案。"
            />
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { luckyWheelAPI } from '@/api/gameCenter'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import type {
  LuckyWheelHistoryResponse,
  LuckyWheelLeaderboardView,
  LuckyWheelOverview,
  LuckyWheelSpinResult,
} from '@/types'

marked.setOptions({
  breaks: true,
  gfm: true,
})

const appStore = useAppStore()

const overview = ref<LuckyWheelOverview | null>(null)
const history = ref<LuckyWheelHistoryResponse | null>(null)
const leaderboard = ref<LuckyWheelLeaderboardView | null>(null)
const loading = ref(false)
const spinning = ref(false)
const lastSpinResult = ref<LuckyWheelSpinResult | null>(null)

const canSpin = computed(() => Boolean(
  overview.value?.enabled &&
  overview.value.points >= overview.value.min_points_required &&
  overview.value.spins_remaining_today > 0,
))
const historyItems = computed(() => history.value?.items ?? overview.value?.recent_history ?? [])
const leaderboardItems = computed(() => leaderboard.value?.items ?? overview.value?.leaderboard ?? [])
const leaderboardDate = computed(() => leaderboard.value?.date ?? overview.value?.server_time?.slice(0, 10) ?? '-')
const rulesHtml = computed(() => {
  const markdown = overview.value?.rules_markdown?.trim() || ''
  if (!markdown) return ''
  return DOMPurify.sanitize(marked.parse(markdown) as string)
})

async function loadData() {
  loading.value = true
  try {
    const [overviewData, historyData, leaderboardData] = await Promise.all([
      luckyWheelAPI.getOverview(),
      luckyWheelAPI.getHistory({ page: 1, page_size: 10 }),
      luckyWheelAPI.getLeaderboard(),
    ])
    overview.value = overviewData
    history.value = historyData
    leaderboard.value = leaderboardData
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, 'Lucky Wheel 页面加载失败'))
  } finally {
    loading.value = false
  }
}

async function handleSpin() {
  spinning.value = true
  try {
    lastSpinResult.value = await luckyWheelAPI.spin()
    appStore.showSuccess(`抽奖完成：${lastSpinResult.value.record.prize_label}`)
    await loadData()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, 'Lucky Wheel 抽奖失败'))
  } finally {
    spinning.value = false
  }
}

function formatProbability(value: number): string {
  return `${(value * 100).toFixed(2)}%`
}

onMounted(loadData)
</script>
