<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="overflow-hidden rounded-3xl bg-gradient-to-br from-cyan-100 via-sky-50 to-indigo-100 p-6 shadow-sm ring-1 ring-black/5 dark:from-cyan-500/10 dark:via-sky-500/5 dark:to-indigo-500/10 dark:ring-white/10">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-3xl">
            <p class="text-sm font-medium text-cyan-700 dark:text-cyan-300">大小单双</p>
            <h1 class="mt-2 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">
              当前局下注面板
            </h1>
            <p class="mt-3 text-sm leading-6 text-gray-700 dark:text-dark-200">
              页面保留 `stake_amount / payout_amount / net_result_amount` 语义，展示下注、派彩和净输赢，不套用积分流水模型。
            </p>
          </div>
          <div class="flex flex-wrap gap-3">
            <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadData">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              <span class="ml-2">刷新</span>
            </button>
            <RouterLink to="/game/size-bet/stats" class="btn btn-secondary">打开统计页</RouterLink>
          </div>
        </div>
      </section>

      <div v-if="loading && !currentView" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <template v-else-if="currentView">
        <section class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">当前阶段</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">{{ currentView.phase }}</p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">当前局号</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ currentView.round?.round_no ?? '-' }}
            </p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">投注倒计时</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatSeconds(currentView.round?.bet_countdown_seconds ?? 0) }}
            </p>
          </article>
          <article class="rounded-3xl border border-gray-100 bg-white p-5 dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500 dark:text-dark-400">总倒计时</p>
            <p class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatSeconds(currentView.round?.countdown_seconds ?? 0) }}
            </p>
          </article>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5 flex items-center justify-between gap-3">
              <div>
                <p class="text-lg font-semibold text-gray-900 dark:text-white">当前下注</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">基于本局允许的 stake_amount 直接下注。</p>
              </div>
              <span
                :class="bettingAvailable ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300' : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-300'"
                class="rounded-full px-3 py-1 text-xs font-medium"
              >
                {{ bettingAvailable ? '可下注' : '不可下注' }}
              </span>
            </div>

            <div v-if="currentView.round" class="space-y-6">
              <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                <button
                  v-for="option in directionCards"
                  :key="option.key"
                  type="button"
                  :class="selectedDirection === option.key
                    ? 'border-primary-400 bg-primary-50 dark:border-primary-500/40 dark:bg-primary-500/10'
                    : 'border-gray-100 hover:border-primary-200 dark:border-dark-700 dark:hover:border-primary-500/30'"
                  class="rounded-3xl border p-5 text-left transition"
                  @click="selectedDirection = option.key"
                >
                  <div class="flex items-center justify-between gap-3">
                    <p class="text-base font-semibold text-gray-900 dark:text-white">{{ option.label }}</p>
                    <span class="rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-300">
                      {{ option.probability }}
                    </span>
                  </div>
                  <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">赔率 {{ option.odds }}</p>
                </button>
              </div>

              <div>
                <p class="mb-3 text-sm font-medium text-gray-900 dark:text-white">选择 Stake Amount</p>
                <div class="flex flex-wrap gap-3">
                  <button
                    v-for="stake in allowedStakes"
                    :key="stake"
                    type="button"
                    :class="selectedStake === stake
                      ? 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-500/40 dark:bg-primary-500/10 dark:text-primary-300'
                      : 'border-gray-200 text-gray-700 hover:border-primary-200 dark:border-dark-600 dark:text-dark-200 dark:hover:border-primary-500/30'"
                    class="rounded-2xl border px-4 py-2 text-sm font-medium transition"
                    @click="selectedStake = stake"
                  >
                    {{ stake }}
                  </button>
                </div>
              </div>

              <div class="rounded-3xl bg-gray-50 p-5 dark:bg-dark-800/70">
                <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">已选方向</p>
                    <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ directionLabel(selectedDirection) }}</p>
                  </div>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">Stake Amount</p>
                    <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ selectedStake }}</p>
                  </div>
                  <div>
                    <p class="text-xs text-gray-500 dark:text-dark-400">预估 Payout</p>
                    <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ estimatedPayout.toFixed(2) }}</p>
                  </div>
                </div>

                <div class="mt-5 flex flex-wrap gap-3">
                  <button
                    type="button"
                    class="btn btn-primary"
                    :disabled="!canPlaceBet || placingBet"
                    @click="handlePlaceBet"
                  >
                    {{ placingBet ? '提交中...' : currentView.my_bet ? '本局已下注' : '提交下注' }}
                  </button>
                  <span class="inline-flex items-center rounded-full bg-white px-3 py-2 text-xs text-gray-500 ring-1 ring-gray-200 dark:bg-dark-900 dark:text-dark-300 dark:ring-dark-600">
                    Server seed hash: {{ currentView.round.server_seed_hash }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-6">
            <section class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-4">
                <p class="text-lg font-semibold text-gray-900 dark:text-white">我的本局记录</p>
                <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">如果已经下注，这里直接展示 stake/payout/net 语义。</p>
              </div>
              <div v-if="currentView.my_bet" class="space-y-3">
                <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-dark-400">Direction</p>
                  <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ directionLabel(currentView.my_bet.direction) }}</p>
                </div>
                <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-dark-400">Stake Amount</p>
                  <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ currentView.my_bet.stake_amount }}</p>
                </div>
                <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-dark-400">Payout Amount</p>
                  <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ currentView.my_bet.payout_amount }}</p>
                </div>
                <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                  <p class="text-xs text-gray-500 dark:text-dark-400">Net Result Amount</p>
                  <p
                    :class="currentView.my_bet.net_result_amount >= 0
                      ? 'text-emerald-600 dark:text-emerald-300'
                      : 'text-rose-600 dark:text-rose-300'"
                    class="mt-1 text-sm font-semibold"
                  >
                    {{ currentView.my_bet.net_result_amount >= 0 ? '+' : '' }}{{ currentView.my_bet.net_result_amount }}
                  </p>
                </div>
              </div>
              <EmptyState
                v-else
                title="本局尚未下注"
                description="选择方向和 stake amount 后即可提交。"
              />
            </section>

            <section class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
              <div class="mb-4 flex items-center justify-between gap-3">
                <div>
                  <p class="text-lg font-semibold text-gray-900 dark:text-white">排行榜</p>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">可切换查看日榜或总榜。</p>
                </div>
                <div class="flex gap-2">
                  <button
                    v-for="scopeOption in scopeOptions"
                    :key="scopeOption"
                    type="button"
                    :class="leaderboardScope === scopeOption
                      ? 'bg-primary-600 text-white'
                      : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-dark-200'"
                    class="rounded-full px-3 py-1.5 text-xs font-medium"
                    @click="changeScope(scopeOption)"
                  >
                    {{ scopeOption }}
                  </button>
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
                        胜率 {{ (item.hit_rate * 100).toFixed(1) }}% / {{ item.win_count }} 胜 {{ item.bet_count }} 局
                      </p>
                    </div>
                    <p
                      :class="item.net_profit >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'"
                      class="text-sm font-semibold"
                    >
                      {{ item.net_profit >= 0 ? '+' : '' }}{{ item.net_profit }}
                    </p>
                  </div>
                </div>
              </div>
              <EmptyState
                v-else
                title="暂无排行数据"
                description="当前榜单还没有可展示的下注结果。"
              />
            </section>
          </div>
        </section>

        <section class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">最近下注记录</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">最近开奖记录保留原始 stake/payout/net 语义。</p>
            </div>
            <div v-if="historyItems.length" class="overflow-x-auto">
              <table class="min-w-full text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-100 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="pb-3 font-medium">局号</th>
                    <th class="pb-3 font-medium">方向</th>
                    <th class="pb-3 font-medium">Stake</th>
                    <th class="pb-3 font-medium">Payout</th>
                    <th class="pb-3 font-medium">Net</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in historyItems"
                    :key="item.bet_id"
                    class="border-b border-gray-50 last:border-b-0 dark:border-dark-800"
                  >
                    <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">#{{ item.round_no }}</td>
                    <td class="py-3 pr-4 text-gray-900 dark:text-white">{{ directionLabel(item.selection) }}</td>
                    <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.stake_amount }}</td>
                    <td class="py-3 pr-4 text-gray-600 dark:text-dark-300">{{ item.payout_amount }}</td>
                    <td
                      :class="item.net_result_amount >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-rose-600 dark:text-rose-300'"
                      class="py-3 font-medium"
                    >
                      {{ item.net_result_amount >= 0 ? '+' : '' }}{{ item.net_result_amount }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <EmptyState
              v-else
              title="暂无下注记录"
              description="当前用户还没有产生任何大小单双下注历史。"
            />
          </div>

          <div class="rounded-3xl border border-gray-100 bg-white p-6 dark:border-dark-700 dark:bg-dark-900">
            <div class="mb-5">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">玩法规则</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">规则和赔率来自 `/game/size-bet/rules`。</p>
            </div>
            <div class="mb-5 grid grid-cols-2 gap-3">
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">Round Duration</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ rules?.round_duration_seconds ?? 0 }}s</p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">Close Offset</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ rules?.bet_close_offset_seconds ?? 0 }}s</p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">Custom Min</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ rules?.custom_stake_min ?? 0 }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 px-4 py-3 dark:bg-dark-800/70">
                <p class="text-xs text-gray-500 dark:text-dark-400">Custom Max</p>
                <p class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{{ rules?.custom_stake_max ?? 0 }}</p>
              </div>
            </div>
            <div
              v-if="rulesHtml"
              class="prose prose-sm max-w-none dark:prose-invert"
              v-html="rulesHtml"
            ></div>
            <EmptyState
              v-else
              title="暂无规则内容"
              description="当前没有可展示的 Size Bet 规则文案。"
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
import { sizeBetAPI } from '@/api/gameCenter'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  SizeBetCurrentRoundView,
  SizeBetDirection,
  SizeBetHistoryResponse,
  SizeBetLeaderboardView,
  SizeBetRulesView,
} from '@/types'

marked.setOptions({
  breaks: true,
  gfm: true,
})

const appStore = useAppStore()

const currentView = ref<SizeBetCurrentRoundView | null>(null)
const rules = ref<SizeBetRulesView | null>(null)
const history = ref<SizeBetHistoryResponse | null>(null)
const leaderboard = ref<SizeBetLeaderboardView | null>(null)
const loading = ref(false)
const placingBet = ref(false)
const leaderboardScope = ref('daily')
const selectedDirection = ref<SizeBetDirection>('small')
const selectedStake = ref(0)

const scopeOptions = ['daily', 'all']

const allowedStakes = computed(() => currentView.value?.round?.allowed_stakes ?? rules.value?.allowed_stakes ?? [])
const historyItems = computed(() => history.value?.items ?? [])
const leaderboardItems = computed(() => leaderboard.value?.items ?? [])
const bettingAvailable = computed(() => Boolean(
  currentView.value?.enabled &&
  currentView.value.phase === 'betting' &&
  currentView.value.round,
))
const canPlaceBet = computed(() => Boolean(
  bettingAvailable.value &&
  selectedStake.value > 0 &&
  !currentView.value?.my_bet,
))
const estimatedPayout = computed(() => {
  const round = currentView.value?.round
  if (!round) return 0
  const odds = selectedDirection.value === 'small'
    ? round.odds_small
    : selectedDirection.value === 'mid'
      ? round.odds_mid
      : round.odds_big
  return selectedStake.value * odds
})
const directionCards = computed(() => {
  const round = currentView.value?.round
  if (!round) return []
  return [
    { key: 'small' as const, label: '小', probability: `${(round.prob_small * 100).toFixed(1)}%`, odds: round.odds_small.toFixed(2) },
    { key: 'mid' as const, label: '中', probability: `${(round.prob_mid * 100).toFixed(1)}%`, odds: round.odds_mid.toFixed(2) },
    { key: 'big' as const, label: '大', probability: `${(round.prob_big * 100).toFixed(1)}%`, odds: round.odds_big.toFixed(2) },
  ]
})
const rulesHtml = computed(() => {
  const markdown = rules.value?.rules_markdown?.trim() || ''
  if (!markdown) return ''
  return DOMPurify.sanitize(marked.parse(markdown) as string)
})

async function loadData() {
  loading.value = true
  try {
    const [currentData, rulesData, historyData, leaderboardData] = await Promise.all([
      sizeBetAPI.getCurrent(),
      sizeBetAPI.getRules(),
      sizeBetAPI.getHistory({ page: 1, page_size: 10 }),
      sizeBetAPI.getLeaderboard({ scope: leaderboardScope.value }),
    ])
    currentView.value = currentData
    rules.value = rulesData
    history.value = historyData
    leaderboard.value = leaderboardData

    if (!selectedStake.value && allowedStakes.value.length) {
      selectedStake.value = allowedStakes.value[0]
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '大小单双页面加载失败'))
  } finally {
    loading.value = false
  }
}

async function handlePlaceBet() {
  if (!currentView.value?.round) return
  placingBet.value = true
  try {
    await sizeBetAPI.placeBet({
      round_id: currentView.value.round.id,
      direction: selectedDirection.value,
      stake_amount: selectedStake.value,
      idempotency_key: typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : `size-bet-${Date.now()}`,
    })
    appStore.showSuccess('下注已提交')
    await loadData()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '下注失败'))
  } finally {
    placingBet.value = false
  }
}

async function changeScope(scope: string) {
  leaderboardScope.value = scope
  await loadData()
}

function directionLabel(direction: string): string {
  if (direction === 'small') return '小'
  if (direction === 'mid') return '中'
  if (direction === 'big') return '大'
  return direction
}

function formatSeconds(value: number): string {
  const safe = Math.max(0, Math.floor(value))
  const minutes = Math.floor(safe / 60)
  const seconds = safe % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

onMounted(loadData)
</script>
