<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-cyan-200/60 bg-[radial-gradient(circle_at_top_left,_#0f766e,_#0f172a)] p-6 text-white shadow-lg shadow-cyan-900/20">
        <p class="text-xs uppercase tracking-[0.3em] text-cyan-200">{{ t('gameCenter.hero.tag') }}</p>
        <div class="mt-3 flex flex-col gap-5 md:flex-row md:items-end md:justify-between">
          <div class="space-y-3">
            <h1 class="text-3xl font-semibold tracking-tight">{{ t('gameCenter.hero.title') }}</h1>
            <p class="text-sm text-cyan-100">{{ t('gameCenter.hero.subtitle') }}</p>
            <div class="flex flex-wrap items-center gap-4 pt-1">
              <div class="rounded-2xl bg-white/10 px-4 py-2 ring-1 ring-white/15 backdrop-blur-sm">
                <span class="text-xs text-cyan-200">当前游戏积分(筹码)</span>
                <p class="text-2xl font-bold text-amber-300">{{ formatPoints(overview?.points ?? authStore.user?.points ?? 0) }}</p>
              </div>
              <div class="rounded-2xl bg-white/10 px-4 py-2 ring-1 ring-white/15 backdrop-blur-sm">
                <span class="text-xs text-cyan-200">可用福利余额</span>
                <p class="text-2xl font-bold text-emerald-300">¥{{ totalWelfareBalance.toFixed(2) }}</p>
              </div>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary border-white/30 bg-white/10 text-white hover:bg-white/20"
              data-test="exchange-entry"
              @click="exchangeOpen = !exchangeOpen"
            >
              {{ exchangeOpen ? '收起福利兑换' : '福利余额 ⇆ 筹码兑换' }}
            </button>
            <RouterLink to="/game/size-bet" class="btn btn-primary bg-amber-500 hover:bg-amber-600 border-none text-slate-950 font-bold">
              进入猜大小
            </RouterLink>
          </div>
        </div>
      </section>

      <!-- 福利余额划转面板 -->
      <section v-if="exchangeOpen" class="card overflow-hidden border-2 border-emerald-500/20 shadow-md">
        <div class="border-b border-slate-200/80 bg-slate-50/50 px-6 py-5 dark:border-white/10 dark:bg-white/5">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="text-xl font-semibold text-slate-900 dark:text-white">福利余额 ⇆ 筹码互兑</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">
                本游戏纯属休闲娱乐，<strong>严禁付费充值购买积分</strong>。玩游戏启动资金仅来自签到、甄选礼遇抽奖及福利赠送。
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary btn-sm text-xs" @click="showCheckinDialog = true">
                去签到领福利
              </button>
              <RouterLink to="/zenxiang-liyu" class="btn btn-secondary btn-sm text-xs">
                甄选礼遇抽奖
              </RouterLink>
            </div>
          </div>
        </div>

        <div class="grid gap-6 px-6 py-6 md:grid-cols-2">
          <!-- 兑入筹码 (仅扣除福利余额) -->
          <article class="rounded-2xl border border-emerald-200/70 bg-gradient-to-br from-emerald-50/50 to-white p-5 dark:border-emerald-500/20 dark:from-slate-900 dark:to-emerald-950/20">
            <div class="flex items-center justify-between">
              <p class="text-base font-semibold text-slate-900 dark:text-white">兑入筹码 (福利余额 -> 积分)</p>
              <span class="rounded-full bg-emerald-100 px-2.5 py-0.5 text-xs font-medium text-emerald-800 dark:bg-emerald-500/20 dark:text-emerald-300">
                汇率 1 元 = {{ balanceToPointsRate }} 积分
              </span>
            </div>
            <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">
              当前可用福利总额: <strong class="text-emerald-600 dark:text-emerald-400">¥{{ totalWelfareBalance.toFixed(2) }}</strong>
              (优先扣除签到、礼遇赠送余额，绝不扣付费余额)
            </p>

            <div v-if="!overview?.exchange.balance_to_points_enabled" class="mt-3 rounded-xl bg-amber-50 p-2.5 text-xs text-amber-800 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/20">
              ⚠️ 管理员暂时关闭了福利余额兑入筹码功能
            </div>

            <div class="mt-4">
              <label class="block text-xs font-medium uppercase tracking-[0.16em] text-slate-500 dark:text-slate-300">划转福利金额 (元)</label>
              <div class="mt-2 flex items-center gap-2">
                <input
                  v-model.number="balanceAmount"
                  type="number"
                  :min="minBalanceAmount"
                  step="0.01"
                  :disabled="!overview?.exchange.balance_to_points_enabled"
                  class="input flex-1"
                  :placeholder="`最小 ${minBalanceAmount}`"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="!overview?.exchange.balance_to_points_enabled"
                  @click="balanceAmount = Math.floor(totalWelfareBalance * 100) / 100"
                >
                  全部
                </button>
              </div>
              <p class="mt-1 text-xs text-slate-400">预计获得积分: {{ Math.round((balanceAmount || 0) * balanceToPointsRate) }}</p>
            </div>

            <button
              type="button"
              class="btn btn-primary mt-5 w-full justify-center"
              :disabled="!canBalanceToPoints || exchangeLoadingDirection === 'balance_to_points' || totalWelfareBalance <= 0"
              @click="handleBalanceToPoints"
            >
              {{
                !overview?.exchange.balance_to_points_enabled
                  ? '兑入功能已暂停'
                  : exchangeLoadingDirection === 'balance_to_points'
                  ? '划转中...'
                  : totalWelfareBalance <= 0
                  ? '福利余额不足 (请先签到或抽奖)'
                  : '确认划转至游戏筹码'
              }}
            </button>
          </article>

          <!-- 兑出余额 (仅进入福利余额) -->
          <article class="rounded-2xl border border-amber-200/70 bg-gradient-to-br from-amber-50/50 to-white p-5 dark:border-amber-500/20 dark:from-slate-900 dark:to-amber-950/20">
            <div class="flex items-center justify-between">
              <p class="text-base font-semibold text-slate-900 dark:text-white">兑出福利 (积分 -> 福利余额)</p>
              <span class="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-500/20 dark:text-amber-300">
                汇率 {{ pointsPerYuanDisplay }} 积分 = 1 元
              </span>
            </div>
            <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">
              当前持有积分筹码: <strong class="text-amber-600 dark:text-amber-400">{{ formatPoints(overview?.points ?? authStore.user?.points ?? 0) }}</strong>
              (兑出后自动存入福利余额，可抵扣模型API消费)
            </p>

            <div v-if="!overview?.exchange.points_to_balance_enabled" class="mt-3 rounded-xl bg-amber-50 p-2.5 text-xs text-amber-800 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/20">
              ⚠️ 管理员暂时关闭了筹码兑出福利功能
            </div>

            <div class="mt-4">
              <label class="block text-xs font-medium uppercase tracking-[0.16em] text-slate-500 dark:text-slate-300">兑出积分数量</label>
              <div class="mt-2 flex items-center gap-2">
                <input
                  v-model.number="pointsAmount"
                  type="number"
                  :min="minPointsAmount"
                  step="1"
                  :disabled="!overview?.exchange.points_to_balance_enabled"
                  class="input flex-1"
                  :placeholder="`最小 ${minPointsAmount}`"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="!overview?.exchange.points_to_balance_enabled"
                  @click="pointsAmount = Math.trunc(overview?.points ?? authStore.user?.points ?? 0)"
                >
                  全部
                </button>
              </div>
              <p class="mt-1 text-xs text-slate-400">预计获得福利余额: ¥{{ estimatedWelfareFromPoints.toFixed(2) }}</p>
            </div>

            <button
              type="button"
              class="btn btn-primary mt-5 w-full justify-center bg-amber-600 hover:bg-amber-700 border-none"
              :disabled="!canPointsToBalance || exchangeLoadingDirection === 'points_to_balance'"
              @click="handlePointsToBalance"
            >
              {{
                !overview?.exchange.points_to_balance_enabled
                  ? '兑出功能已暂停'
                  : exchangeLoadingDirection === 'points_to_balance'
                  ? '兑出中...'
                  : '确认变现为福利余额'
              }}
            </button>
          </article>
        </div>
      </section>

      <div v-if="loadState === 'loading'" class="flex justify-center py-16">
        <LoadingSpinner />
      </div>

      <section v-else-if="loadState === 'error'" class="card px-6 py-12">
        <EmptyState
          :title="t('gameCenter.loadError.title')"
          :description="t('gameCenter.loadError.description')"
          :action-text="t('common.retry')"
          @action="loadOverview"
        />
      </section>

      <section v-else-if="!gameCenterEnabled" class="card px-6 py-12">
        <EmptyState
          :title="t('gameCenter.disabled.title')"
          :description="t('gameCenter.disabled.description')"
        />
      </section>

      <template v-else>
        <!-- 每日定时福利领取 (如果配置了 Claim Batches) -->
        <section v-if="claimBatches.length > 0" class="card overflow-hidden">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.claim.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.claim.subtitle') }}</p>
          </div>
          <div class="grid gap-3 px-4 py-4 sm:grid-cols-2 xl:grid-cols-3">
            <article
              v-for="batch in claimBatches"
              :key="batch.batch_key"
              class="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4 dark:border-white/10 dark:bg-white/5"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <p class="text-sm font-semibold text-slate-900 dark:text-white">{{ batch.batch_key }}</p>
                  <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ batch.claim_time || '--:--' }}</p>
                </div>
                <span class="rounded-full px-2 py-1 text-xs font-medium" :class="claimStatusClass(batch.status)">
                  {{ claimStatusLabel(batch.status) }}
                </span>
              </div>
              <p class="mt-3 text-sm text-slate-600 dark:text-slate-300">
                {{ t('gameCenter.claim.reward', { points: formatPoints(batch.points_amount) }) }}
              </p>
              <button
                type="button"
                class="btn btn-primary mt-4 w-full justify-center"
                :disabled="batch.status !== 'claimable' || claimLoadingBatchKey === batch.batch_key"
                @click="handleClaim(batch.batch_key)"
              >
                {{
                  claimLoadingBatchKey === batch.batch_key
                    ? t('gameCenter.claim.claiming')
                    : t('gameCenter.claim.action')
                }}
              </button>
            </article>
          </div>
        </section>

        <!-- 游戏入口大厅 -->
        <section class="card overflow-hidden">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.catalog.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.catalog.subtitle') }}</p>
          </div>
          <div class="grid gap-6 px-6 py-6 md:grid-cols-2">
            <!-- 猜大小专属卡片 -->
            <article class="relative overflow-hidden rounded-3xl border border-amber-200/80 bg-gradient-to-br from-amber-500/10 via-slate-900/5 to-rose-500/10 p-6 shadow-sm transition hover:shadow-md dark:border-amber-500/20 dark:from-amber-950/30 dark:via-slate-900 dark:to-rose-950/30">
              <div class="flex items-start justify-between gap-4">
                <div>
                  <span class="inline-block rounded-full bg-amber-500/20 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-amber-700 dark:text-amber-300">
                    🎲 极速竞猜 · 60秒一轮
                  </span>
                  <h3 class="mt-3 text-2xl font-bold text-slate-900 dark:text-white">大小中竞猜 (Size Bet)</h3>
                  <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">1~11 随机摇号 · 可验证公平性 (SHA256)</p>
                </div>
                <div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-500 text-2xl text-slate-950 shadow-md">
                  🎲
                </div>
              </div>

              <p class="mt-4 text-sm leading-6 text-slate-600 dark:text-slate-300">
                简单刺激的猜大小游戏，猜【小】(1-5)、【中】(6)、【大】(7-11)。全自动定时轮调度，开奖实时到账福利筹码。
              </p>

              <div class="mt-6 flex flex-wrap gap-3">
                <RouterLink to="/game/size-bet" class="btn btn-primary flex-1 justify-center bg-amber-500 hover:bg-amber-600 text-slate-950 font-bold border-none">
                  立即开始游戏
                </RouterLink>
                <RouterLink to="/game/size-bet/stats" class="btn btn-secondary flex-1 justify-center">
                  查看历史走势
                </RouterLink>
              </div>
            </article>

            <!-- 福利获取直通卡片 -->
            <article class="flex flex-col justify-between rounded-3xl border border-sky-200/80 bg-gradient-to-br from-sky-500/10 via-white to-indigo-500/10 p-6 shadow-sm dark:border-sky-500/20 dark:from-sky-950/30 dark:via-slate-900 dark:to-indigo-950/30">
              <div>
                <span class="inline-block rounded-full bg-sky-500/20 px-3 py-1 text-xs font-semibold uppercase tracking-wider text-sky-700 dark:text-sky-300">
                  🎁 福利补给站
                </span>
                <h3 class="mt-3 text-2xl font-bold text-slate-900 dark:text-white">获取启动资金</h3>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">本站游戏不允许充值，筹码全部来源于福利活动！</p>
                
                <div class="mt-4 space-y-3">
                  <div class="flex items-center justify-between rounded-2xl bg-white/70 p-3 ring-1 ring-slate-200/70 dark:bg-white/5 dark:ring-white/10">
                    <div>
                      <p class="text-sm font-semibold text-slate-900 dark:text-white">每日签到</p>
                      <p class="text-xs text-slate-500 dark:text-slate-400">每天点击签到免费赠送福利余额</p>
                    </div>
                    <button type="button" class="btn btn-secondary btn-sm" @click="showCheckinDialog = true">去签到</button>
                  </div>
                  <div class="flex items-center justify-between rounded-2xl bg-white/70 p-3 ring-1 ring-slate-200/70 dark:bg-white/5 dark:ring-white/10">
                    <div>
                      <p class="text-sm font-semibold text-slate-900 dark:text-white">甄选礼遇</p>
                      <p class="text-xs text-slate-500 dark:text-slate-400">参与活动抽奖，赢取丰厚福利大奖</p>
                    </div>
                    <RouterLink to="/zenxiang-liyu" class="btn btn-secondary btn-sm">去抽奖</RouterLink>
                  </div>
                </div>
              </div>

              <div class="mt-6 rounded-xl bg-sky-50 p-3 text-xs text-sky-700 dark:bg-sky-500/10 dark:text-sky-300">
                📌 提示：获得福利余额后，在本页点击“福利余额 ⇆ 筹码兑换”即可 1:{{ balanceToPointsRate }} 划转为游戏筹码！
              </div>
            </article>
          </div>
        </section>

        <!-- 我的积分明细 -->
        <section class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-slate-200/80 px-6 py-5 dark:border-white/10 md:flex-row md:items-end md:justify-between">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.ledger.title') }}</h2>
            <div class="flex flex-wrap items-end gap-3">
              <div>
                <label class="block text-xs font-medium text-slate-500 dark:text-slate-400">{{ t('gameCenter.ledger.startDate') }}</label>
                <input v-model="ledgerStartDate" type="date" class="input mt-1 w-40" />
              </div>
              <div>
                <label class="block text-xs font-medium text-slate-500 dark:text-slate-400">{{ t('gameCenter.ledger.endDate') }}</label>
                <input v-model="ledgerEndDate" type="date" class="input mt-1 w-40" />
              </div>
              <button type="button" class="btn btn-secondary" :disabled="ledgerLoading" @click="refreshLedger">{{ t('common.refresh') }}</button>
            </div>
          </div>
          <div class="space-y-2 px-4 py-4">
            <div
              v-for="ledger in ledgerView.items"
              :key="ledger.id"
              class="flex items-center justify-between rounded-xl bg-slate-50 px-3 py-3 text-sm dark:bg-white/5"
            >
              <div>
                <p class="font-medium text-slate-900 dark:text-white">{{ ledgerTypeLabel(ledger.entry_type) }}</p>
                <p class="text-xs text-slate-500 dark:text-slate-400">{{ formatDateTime(ledger.created_at) }} · {{ ledger.reason || '--' }}</p>
              </div>
              <p class="font-semibold" :class="ledger.delta_points >= 0 ? 'text-emerald-600' : 'text-rose-600'">
                {{ formatSignedPoints(ledger.delta_points) }}
              </p>
            </div>
            <p v-if="!ledgerView.items.length" class="text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.ledger.empty') }}</p>
          </div>
          <Pagination :total="ledgerView.total" :page="ledgerView.page" :page-size="ledgerView.page_size" @update:page="changeLedgerPage" @update:pageSize="changeLedgerPageSize" />
        </section>
      </template>
    </div>

    <!-- 签到弹窗交互 -->
    <BaseDialog :show="showCheckinDialog" :title="t('wallet.checkinTitle')" width="wide" @close="handleCheckinClose">
      <ProfileCheckinCard v-if="showCheckinDialog" />
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { gameCenterAPI } from '@/api/gameCenter'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { BasePaginationResponse } from '@/types'
import type { GameCenterClaimStatus, GameCenterLedgerItem, GameCenterOverview } from '@/types/gameCenter'

const ProfileCheckinCard = defineAsyncComponent(() => import('@/components/user/profile/ProfileCheckinCard.vue'))

type LoadState = 'loading' | 'ready' | 'error'
type ExchangeDirection = 'balance_to_points' | 'points_to_balance'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loadState = ref<LoadState>('loading')
const overview = ref<GameCenterOverview | null>(null)
const claimLoadingBatchKey = ref('')
const exchangeLoadingDirection = ref<ExchangeDirection | ''>('')
const exchangeOpen = ref(false)
const showCheckinDialog = ref(false)
const balanceAmount = ref(1)
const pointsAmount = ref(10)
const pointsAmountInitialized = ref(false)
const today = toDateInput(new Date())
const ledgerStartDate = ref(today)
const ledgerEndDate = ref(today)
const ledgerLoading = ref(false)
const ledgerView = ref<BasePaginationResponse<GameCenterLedgerItem>>({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })

const gameCenterEnabled = computed(() => {
  return (appStore.cachedPublicSettings as any)?.game_center_enabled !== false
})
const claimBatches = computed(() => overview.value?.claim_batches ?? [])

const balanceToPointsRate = computed(() => {
  const rate = Number(overview.value?.exchange.balance_to_points_rate)
  return Number.isFinite(rate) && rate > 0 ? rate : 0
})

const pointsToBalanceRate = computed(() => {
  const rate = Number(overview.value?.exchange.points_to_balance_rate)
  return Number.isFinite(rate) && rate > 0 ? rate : 0
})

const pointsPerYuanDisplay = computed(() => {
  const rate = pointsToBalanceRate.value
  if (rate <= 0) return 0
  return Number((1 / rate).toFixed(4))
})

const minBalanceAmount = computed(() => {
  const min = Number(overview.value?.exchange.min_balance_amount)
  return min > 0 ? min : 0.01
})

const minPointsAmount = computed(() => {
  const min = Number(overview.value?.exchange.min_points_amount)
  return min > 0 ? min : 1
})

const estimatedWelfareFromPoints = computed(() => {
  return (Number(pointsAmount.value) || 0) * pointsToBalanceRate.value
})

const totalWelfareBalance = computed(() => {
  if (overview.value?.welfare_balance != null) {
    return Math.max(0, Number(overview.value.welfare_balance))
  }
  const u = authStore.user
  if (!u) return 0
  const positiveSum =
    Math.max(0, Number(u.checkin_balance ?? 0)) +
    Math.max(0, Number(u.liyu_balance ?? 0)) +
    Math.max(0, Number(u.gift_balance ?? 0))
  if (positiveSum > 0) {
    return Number(positiveSum.toFixed(4))
  }
  return Math.max(0, Number((u.welfare_balance ?? 0).toFixed(4)))
})

const canBalanceToPoints = computed(() => {
  if (!overview.value?.exchange.balance_to_points_enabled || balanceToPointsRate.value <= 0) return false
  const amount = Number(balanceAmount.value)
  return amount > 0 && amount >= minBalanceAmount.value && amount <= totalWelfareBalance.value && Math.round(amount * balanceToPointsRate.value) > 0
})

const canPointsToBalance = computed(() => {
  if (!overview.value?.exchange.points_to_balance_enabled || pointsToBalanceRate.value <= 0) return false
  const currentPts = overview.value?.points ?? authStore.user?.points ?? 0
  return Number.isInteger(pointsAmount.value) && Number(pointsAmount.value) >= minPointsAmount.value && Number(pointsAmount.value) <= currentPts
})

function toDateInput(value: Date): string {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatPoints(value: number): string {
  return new Intl.NumberFormat().format(Math.trunc(value))
}

function formatSignedPoints(value: number): string {
  if (value >= 0) {
    return `+${formatPoints(value)}`
  }
  return `-${formatPoints(Math.abs(value))}`
}

function formatDateTime(value?: string): string {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

function ledgerTypeLabel(type: string): string {
  return t(`gameCenter.ledger.types.${type}`)
}

function claimStatusLabel(status: GameCenterClaimStatus): string {
  return t(`gameCenter.claim.status.${status}`)
}

function claimStatusClass(status: GameCenterClaimStatus): string {
  if (status === 'claimable') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (status === 'claimed') return 'bg-slate-100 text-slate-600 dark:bg-white/10 dark:text-slate-200'
  return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
}

function normalizeError(error: unknown, fallbackKey: string): string {
  const message = (error as { response?: { data?: { message?: string } }, message?: string })?.response?.data?.message
    || (error as { message?: string })?.message
  if (message) return message
  return t(fallbackKey)
}

async function handleCheckinClose() {
  showCheckinDialog.value = false
  await authStore.refreshUser?.()
  await loadOverview()
}

async function loadOverview(): Promise<void> {
  loadState.value = 'loading'
  try {
    overview.value = await gameCenterAPI.getOverview()
    if (authStore.user && overview.value?.points != null) {
      authStore.user.points = overview.value.points
    }
    if (!pointsAmountInitialized.value && overview.value?.exchange) {
      pointsAmount.value = Math.max(minPointsAmount.value, Math.round(pointsPerYuanDisplay.value))
      pointsAmountInitialized.value = true
    }
    loadState.value = 'ready'
    await loadLedger()
  } catch (error) {
    loadState.value = 'error'
    appStore.showError(normalizeError(error, 'gameCenter.loadError.description'))
  }
}

async function loadLedger(page = ledgerView.value.page, pageSize = ledgerView.value.page_size): Promise<void> {
  ledgerLoading.value = true
  try {
    ledgerView.value = await gameCenterAPI.getLedger({
      page,
      page_size: pageSize,
      start_date: ledgerStartDate.value || undefined,
      end_date: ledgerEndDate.value || undefined,
    })
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.loadError.description'))
  } finally {
    ledgerLoading.value = false
  }
}

function refreshLedger(): void {
  void loadLedger(1, ledgerView.value.page_size)
}

function changeLedgerPage(page: number): void {
  void loadLedger(page, ledgerView.value.page_size)
}

function changeLedgerPageSize(pageSize: number): void {
  void loadLedger(1, pageSize)
}

async function handleClaim(batchKey: string): Promise<void> {
  if (!batchKey) return
  claimLoadingBatchKey.value = batchKey
  try {
    await gameCenterAPI.claimPoints(batchKey)
    appStore.showSuccess(t('gameCenter.claim.success'))
    await loadOverview()
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.claim.failed'))
  } finally {
    claimLoadingBatchKey.value = ''
  }
}

async function handleBalanceToPoints(): Promise<void> {
  if (!canBalanceToPoints.value) return
  exchangeLoadingDirection.value = 'balance_to_points'
  try {
    await gameCenterAPI.exchangeBalanceToPoints({ amount: Number(balanceAmount.value) })
    appStore.showSuccess(t('gameCenter.exchange.success'))
    await authStore.refreshUser?.()
    await loadOverview()
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.exchange.failed'))
  } finally {
    exchangeLoadingDirection.value = ''
  }
}

async function handlePointsToBalance(): Promise<void> {
  if (!canPointsToBalance.value) return
  exchangeLoadingDirection.value = 'points_to_balance'
  try {
    await gameCenterAPI.exchangePointsToBalance({ points: Math.trunc(Number(pointsAmount.value)) })
    appStore.showSuccess(t('gameCenter.exchange.success'))
    await authStore.refreshUser?.()
    await loadOverview()
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.exchange.failed'))
  } finally {
    exchangeLoadingDirection.value = ''
  }
}

onMounted(() => {
  loadOverview()
})
</script>
