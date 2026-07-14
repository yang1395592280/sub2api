<template>
  <AppLayout>
    <div class="mx-auto max-w-5xl space-y-5 px-4 py-6 sm:px-6 lg:px-8">
      <header class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('zenxiangLiyu.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.description') }}</p>
        </div>
        <button
          type="button"
          class="self-start text-sm font-medium text-primary-600 hover:text-primary-700 disabled:cursor-not-allowed disabled:text-gray-400 dark:text-primary-400 dark:hover:text-primary-300 dark:disabled:text-gray-600"
          :disabled="statusLoading || isPlaying"
          @click="refreshStatus"
        >
          {{ t('zenxiangLiyu.refresh') }}
        </button>
      </header>

      <p v-if="statusRefreshError" class="text-sm text-amber-700 dark:text-amber-300" role="status">
        {{ statusRefreshError }}
      </p>

      <div v-if="statusLoading && !status" class="card p-8 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('zenxiangLiyu.loading') }}
      </div>

      <div v-else-if="loadError" class="card flex flex-col gap-3 p-6 sm:flex-row sm:items-center sm:justify-between">
        <p class="text-sm text-red-600 dark:text-red-400">{{ loadError }}</p>
        <button type="button" class="btn btn-primary shrink-0" @click="refreshStatus">{{ t('common.retry') }}</button>
      </div>

      <template v-else-if="status">
        <section class="grid grid-cols-1 gap-3 sm:grid-cols-3" aria-label="活动数据">
          <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.currentBalance') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatAmount(currentBalance) }}</p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.availableTickets') }}</p>
            <div class="mt-1 flex items-baseline gap-2">
              <p class="text-lg font-semibold text-gray-900 dark:text-white">{{ status.tickets_available }}</p>
              <span class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">{{ t('zenxiangLiyu.ticketUnit') }}</span>
            </div>
            <p class="mt-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">{{ t('zenxiangLiyu.ticketRetentionHint', { limit: status.ticket_capacity, days: status.ticket_retention_days }) }}</p>
            <p v-if="status.today_tickets_granted > 0" class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-300">
              {{ t('zenxiangLiyu.ticketGiftHint', { count: status.today_tickets_granted }) }}
            </p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.ticketProgress') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ status.today_tickets_used }} / {{ status.today_tickets_earned }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ nextTicketHint || t('zenxiangLiyu.ticketEarnHint', { threshold: formatNumber(status.ticket_usage_threshold), limit: status.daily_ticket_limit }) }}</p>
          </div>
        </section>

        <section class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <div class="card p-5 sm:p-7">
            <div class="mx-auto flex max-w-md flex-col items-center">
              <div class="relative aspect-square w-full max-w-[19rem]">
                <div class="absolute left-1/2 top-0 z-10 h-0 w-0 -translate-x-1/2 border-x-8 border-t-[14px] border-x-transparent border-t-gray-900 dark:border-t-white" />
                <div class="zenxiang-wheel" :class="{ 'zenxiang-wheel--spinning': isSpinning }" :style="{ background: wheelBackground, transform: `rotate(${wheelRotation}deg)` }">
                  <div class="zenxiang-wheel__center">
                    <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.title') }}</span>
                  </div>
                  <span
                    v-for="(prize, index) in status.prizes"
                    :key="prize.id"
                    class="zenxiang-wheel__label"
                    :style="wheelLabelStyle(index, status.prizes.length)"
                    :title="prize.name"
                    :aria-label="prize.name"
                  >
                    {{ wheelLabel(prize.name, index, status.prizes.length) }}
                  </span>
                </div>
              </div>

              <p v-if="unavailableReason" class="mt-5 text-center text-sm text-amber-700 dark:text-amber-300">{{ unavailableReason }}</p>
              <p v-else-if="status.tickets_available > 0" class="mt-5 text-center text-sm text-emerald-700 dark:text-emerald-300">
                {{ t('zenxiangLiyu.ticketPlayHint', { count: status.tickets_available }) }}
              </p>
              <p v-else class="mt-5 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.noTicketHint', { threshold: formatNumber(status.ticket_usage_threshold) }) }}</p>

              <button
                data-testid="zenxiang-play"
                type="button"
                class="btn btn-primary zenxiang-play-button mt-4 min-w-36"
                :disabled="!canPlay"
                @click="play"
              >
                <span v-if="isPlaying" class="zenxiang-play-button__spinner" aria-hidden="true"></span>
                {{ isPlaying ? t('zenxiangLiyu.drawing') : t('zenxiangLiyu.draw') }}
              </button>

              <p v-if="playError" class="mt-3 text-center text-sm text-red-600 dark:text-red-400">{{ playError }}</p>
            </div>
          </div>

          <aside class="card overflow-hidden">
            <div class="border-b border-gray-200 px-5 py-4 dark:border-gray-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('zenxiangLiyu.configuredRewards') }}</h2>
            </div>
            <ul v-if="status.prizes.length" class="divide-y divide-gray-100 dark:divide-gray-700">
              <li v-for="prize in status.prizes" :key="prize.id" class="px-5 py-3">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-800 dark:text-gray-100" :title="prize.name">{{ prize.name }}</p>
                    <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.winProbability', { value: formatProbability(prize.probability) }) }}</p>
                  </div>
                  <span class="shrink-0 rounded-full bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">{{ formatAmount(prize.reward_amount) }}</span>
                </div>
                <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700">
                  <div class="h-full rounded-full bg-emerald-500/80 transition-all" :style="{ width: probabilityBarWidth(prize.probability) }"></div>
                </div>
              </li>
            </ul>
            <p v-else class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.noRewards') }}</p>
          </aside>
        </section>

        <section v-if="result" class="rounded-lg border border-emerald-200 bg-emerald-50 p-5 dark:border-emerald-900/70 dark:bg-emerald-950/30" aria-live="polite">
          <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p class="text-sm font-medium text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.rewardResult', { amount: signedAmount(finalRewardAmount) }) }}</p>
              <h2 class="mt-1 text-xl font-semibold text-emerald-950 dark:text-white">{{ result.prize_name }}</h2>
            </div>
            <p class="text-sm text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.latestBalance', { amount: formatNumber(latestResultBalance) }) }}</p>
          </div>
          <div class="mt-4 grid grid-cols-1 gap-3 border-t border-emerald-200 pt-4 text-sm sm:grid-cols-3 dark:border-emerald-900/70">
            <p class="text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.rewardAmount', { amount: formatNumber(result.reward_amount) }) }}</p>
            <p class="font-medium" :class="finalRewardClass">{{ t('zenxiangLiyu.finalRewardAmount', { amount: signedAmount(finalRewardAmount) }) }}</p>
            <p class="text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.netAmount', { amount: formatNumber(latestResultNetAmount) }) }}</p>
          </div>
          <p v-if="luckyCoinResult" class="mt-3 text-sm font-medium" :class="luckyCoinResult.outcome === 'double' ? 'text-emerald-700 dark:text-emerald-300' : 'text-rose-700 dark:text-rose-300'">
            {{ luckyCoinResultText }}
          </p>
        </section>

        <section class="card overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-200 bg-gray-50/80 px-5 py-4 dark:border-gray-700 dark:bg-gray-800/60">
            <div>
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('zenxiangLiyu.todayRecords') }}</h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.todayRecordHint') }}</p>
            </div>
            <span class="rounded-full bg-white px-2.5 py-1 text-xs font-semibold text-gray-600 shadow-sm ring-1 ring-gray-200 dark:bg-gray-900 dark:text-gray-300 dark:ring-gray-700">{{ todayRecords.length }}</span>
          </div>
          <div v-if="todayRecords.length" class="space-y-2 bg-gray-50/40 p-3 dark:bg-gray-900/20">
            <div v-for="record in todayRecords" :key="record.id" class="zenxiang-record-item rounded-lg border border-gray-100 bg-white px-4 py-3 shadow-sm dark:border-gray-700 dark:bg-gray-800">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ record.prize_name }}</p>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ formatTime(record.played_at) }}</span>
                    <span v-if="record.probability > 0" class="rounded-full bg-gray-100 px-2 py-0.5 text-gray-600 dark:bg-gray-700 dark:text-gray-300">{{ t('zenxiangLiyu.probabilityShort', { value: formatProbability(record.probability) }) }}</span>
                  </div>
                </div>
                <div class="flex flex-wrap gap-2 text-xs sm:justify-end">
                  <span class="rounded-full px-2.5 py-1 font-medium" :class="recordRewardClass(record)">{{ t('zenxiangLiyu.finalRewardShort', { amount: signedAmount(recordFinalReward(record)) }) }}</span>
                  <span v-if="record.lucky_coin_played" class="rounded-full px-2.5 py-1 font-medium" :class="recordLuckyCoinClass(record)">{{ recordLuckyCoinText(record) }}</span>
                  <span class="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-300">{{ t('zenxiangLiyu.ticketUsedShort') }}</span>
                  <span class="rounded-full px-2.5 py-1 font-semibold" :class="recordNetClass(record.user_net_amount)">{{ t('zenxiangLiyu.netShort', { amount: signedAmount(record.user_net_amount) }) }}</span>
                </div>
              </div>
            </div>
          </div>
          <p v-else class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.noTodayRecords') }}</p>
        </section>

        <div v-if="showResultDialog && result" class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/45 px-4 py-6" role="dialog" aria-modal="true">
          <div class="result-dialog w-full max-w-sm rounded-xl border border-emerald-200 bg-white p-6 text-center shadow-2xl dark:border-emerald-900 dark:bg-gray-900">
            <p class="text-sm font-medium text-emerald-600 dark:text-emerald-300">{{ t('zenxiangLiyu.resultDialogEyebrow') }}</p>
            <h2 class="mt-2 text-2xl font-semibold text-gray-950 dark:text-white">{{ result.prize_name }}</h2>
            <p class="reward-burst mt-3 text-4xl font-bold" :class="finalRewardClass">{{ signedAmountWithUnit(finalRewardAmount) }}</p>
            <div class="mt-5 grid grid-cols-3 gap-2 rounded-lg bg-gray-50 p-3 text-sm dark:bg-gray-800">
              <div class="rounded-lg bg-white/80 px-2 py-2 dark:bg-gray-900/50">
                <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.baseRewardLabel') }}</span>
                <span class="mt-1 block font-semibold text-gray-900 dark:text-white">{{ formatAmount(result.reward_amount) }}</span>
              </div>
              <div class="rounded-lg bg-white/80 px-2 py-2 dark:bg-gray-900/50">
                <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.luckyCoinDeltaLabel') }}</span>
                <span class="mt-1 block font-semibold" :class="amountToneClass(luckyCoinResult?.adjustment_amount ?? 0)">{{ signedAmount(luckyCoinResult?.adjustment_amount ?? 0) }}</span>
              </div>
              <div class="rounded-lg bg-white/80 px-2 py-2 dark:bg-gray-900/50">
                <span class="block text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.finalRewardLabel') }}</span>
                <span class="mt-1 block font-semibold" :class="finalRewardClass">{{ signedAmount(finalRewardAmount) }}</span>
              </div>
            </div>
            <div class="mt-3 grid grid-cols-2 gap-2 rounded-lg bg-gray-50 p-3 text-sm dark:bg-gray-800">
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.ticketUsedLabel') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ t('zenxiangLiyu.oneTicket') }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.netLabel') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ formatAmount(latestResultNetAmount) }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.latestPoints') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ formatAmount(latestResultBalance) }}</span>
            </div>
            <div
              v-if="luckyCoinResult"
              class="lucky-result-panel mt-4"
              :class="luckyCoinResult.outcome === 'double' ? 'lucky-result-panel--win' : 'lucky-result-panel--lose'"
            >
              <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.luckyCoinResultTitle') }}</div>
              <div class="mt-1 text-base font-semibold">{{ luckyCoinResultText }}</div>
            </div>
            <p v-if="luckyCoinError" class="mt-4 text-sm text-rose-600 dark:text-rose-300">{{ luckyCoinError }}</p>
            <div class="mt-6 grid gap-2" :class="showLuckyCoinAction ? 'grid-cols-2' : 'grid-cols-1'">
              <button type="button" class="btn btn-secondary min-h-14 w-full" @click="showResultDialog = false">{{ t('common.confirm') }}</button>
              <button
                v-if="showLuckyCoinAction"
                data-testid="zenxiang-lucky-coin"
                type="button"
                class="lucky-coin-card"
                :class="{ 'lucky-coin-card--flipping': luckyCoinFlipping, 'lucky-coin-card--win': luckyCoinResult?.outcome === 'double', 'lucky-coin-card--lose': luckyCoinResult?.outcome === 'zero' }"
                :disabled="!canUseLuckyCoin || luckyCoinFlipping"
                @click="playLuckyCoin"
              >
                <span class="lucky-coin-card__shine"></span>
                <span class="lucky-coin-card__face">
                  {{ luckyCoinFlipping ? t('zenxiangLiyu.luckyCoinFlipping') : luckyCoinButtonText }}
                </span>
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { listZenxiangLiyuRecords, playZenxiangLiyu, playZenxiangLiyuLuckyCoin, type ZenxiangLiyuLuckyCoinResult, type ZenxiangLiyuPlayResult, type ZenxiangLiyuRecord } from '@/api/zenxiangLiyu'
import { useAuthStore, useZenxiangLiyuStore } from '@/stores'

const SPIN_DURATION_MS = 4200

const { t } = useI18n()
const authStore = useAuthStore()
const zenxiangLiyuStore = useZenxiangLiyuStore()
const isPlaying = ref(false)
const loadError = ref('')
const statusRefreshError = ref('')
const playError = ref('')
const result = ref<ZenxiangLiyuPlayResult | null>(null)
const luckyCoinResult = ref<ZenxiangLiyuLuckyCoinResult | null>(null)
const luckyCoinFlipping = ref(false)
const luckyCoinError = ref('')
const todayRecords = ref<ZenxiangLiyuRecord[]>([])
const wheelRotation = ref(0)
const isSpinning = ref(false)
const showResultDialog = ref(false)
const status = computed(() => zenxiangLiyuStore.status)
const statusLoading = computed(() => zenxiangLiyuStore.loading)
const currentBalance = computed(() => {
  if (luckyCoinResult.value) return luckyCoinResult.value.balance_after
  if (result.value) return result.value.balance_after_reward
  return status.value?.balance ?? authStore.user?.balance ?? 0
})

const unavailableReason = computed(() => {
  if (!status.value || (status.value.visible && status.value.can_play)) return ''

  switch (status.value.reason) {
    case 'insufficient_balance':
      return t('zenxiangLiyu.insufficientBalance', { amount: formatNumber(status.value.minimum_balance) })
    case 'daily_limit_reached':
    case 'daily_play_limit_reached':
    case 'zenxiang liyu daily limit reached':
      return t('zenxiangLiyu.dailyLimitReached')
    case 'zenxiang liyu no ticket':
      return t('zenxiangLiyu.noTicket')
    case 'maintenance':
    case 'not_visible':
    case 'disabled':
    case 'zenxiang liyu is disabled':
    case 'zenxiang liyu unauthorized':
      return t('zenxiangLiyu.maintenance')
    default:
      return status.value.visible ? t('zenxiangLiyu.unavailable') : t('zenxiangLiyu.maintenance')
  }
})

const canPlay = computed(() => Boolean(status.value?.visible && status.value.can_play && !isPlaying.value))
const canUseLuckyCoin = computed(() => Boolean(
  result.value?.id &&
  result.value.lucky_coin_available &&
  !result.value.lucky_coin_played &&
  !luckyCoinResult.value &&
  !luckyCoinFlipping.value
))
const showLuckyCoinAction = computed(() => Boolean(canUseLuckyCoin.value || luckyCoinResult.value || luckyCoinFlipping.value))
const latestResultBalance = computed(() => luckyCoinResult.value?.balance_after ?? result.value?.balance_after_reward ?? 0)
const latestResultNetAmount = computed(() => Number(result.value?.user_net_amount ?? 0) + Number(luckyCoinResult.value?.adjustment_amount ?? 0))
const finalRewardAmount = computed(() => Number(result.value?.reward_amount ?? 0) + Number(luckyCoinResult.value?.adjustment_amount ?? 0))
const finalRewardClass = computed(() => amountToneClass(finalRewardAmount.value))
const luckyCoinButtonText = computed(() => {
  if (!luckyCoinResult.value) return t('zenxiangLiyu.luckyCoinDouble')
  return luckyCoinResult.value.outcome === 'double' ? t('zenxiangLiyu.luckyCoinWin') : t('zenxiangLiyu.luckyCoinLose')
})
const luckyCoinResultText = computed(() => {
  const current = luckyCoinResult.value
  if (!current) return ''
  if (current.outcome === 'double') {
    return t('zenxiangLiyu.luckyCoinWinDetail', { amount: formatNumber(current.adjustment_amount) })
  }
  return t('zenxiangLiyu.luckyCoinLoseDetail', { amount: formatNumber(Math.abs(current.adjustment_amount)) })
})
const nextTicketHint = computed(() => {
  const current = status.value
  if (!current) return ''
  if (current.daily_ticket_limit > 0 && current.today_tickets_earned >= current.daily_ticket_limit) {
    return t('zenxiangLiyu.dailyTicketLimitReached')
  }
  const missing = Number(current.next_ticket_usage_missing || 0)
  if (missing > 0) {
    return t('zenxiangLiyu.nextTicketMissing', { amount: formatNumber(missing) })
  }
  return ''
})
const wheelBackground = computed(() => {
  const prizes = status.value?.prizes ?? []
  if (!prizes.length) return 'conic-gradient(#e5e7eb 0deg 360deg)'

  const colors = ['#0f766e', '#0369a1', '#7c3aed', '#b45309', '#be123c', '#4d7c0f']
  const step = 360 / prizes.length
  return `conic-gradient(${prizes.map((_, index) => `${colors[index % colors.length]} ${index * step}deg ${(index + 1) * step}deg`).join(', ')})`
})

function formatAmount(amount?: number): string {
  return `${formatNumber(amount)} ${t('zenxiangLiyu.balanceUnit')}`
}

function formatNumber(amount?: number): string {
  return Number(amount ?? 0).toLocaleString()
}

function formatProbability(probability?: number): string {
  const value = Number(probability ?? 0)
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })}%`
}

function probabilityBarWidth(probability?: number): string {
  const value = Math.max(0, Math.min(100, Number(probability ?? 0)))
  return `${value}%`
}

function signedAmount(amount?: number): string {
  const value = Number(amount ?? 0)
  if (value > 0) return `+${formatNumber(value)}`
  return formatNumber(value)
}

function signedAmountWithUnit(amount?: number): string {
  return `${signedAmount(amount)} ${t('zenxiangLiyu.balanceUnit')}`
}

function amountToneClass(amount?: number): string {
  const value = Number(amount ?? 0)
  if (value > 0) return 'text-emerald-700 dark:text-emerald-300'
  if (value < 0) return 'text-rose-700 dark:text-rose-300'
  return 'text-gray-700 dark:text-gray-300'
}

function recordNetClass(amount?: number): string {
  const value = Number(amount ?? 0)
  if (value > 0) return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
  if (value < 0) return 'bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300'
  return 'bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-300'
}

function recordFinalReward(record: ZenxiangLiyuRecord): number {
  return Number(record.reward_amount ?? 0) + Number(record.lucky_coin_adjustment ?? 0)
}

function recordRewardClass(record: ZenxiangLiyuRecord): string {
  return recordNetClass(recordFinalReward(record))
}

function recordLuckyCoinClass(record: ZenxiangLiyuRecord): string {
  if (record.lucky_coin_outcome === 'double') return 'bg-amber-50 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300'
  if (record.lucky_coin_outcome === 'zero') return 'bg-rose-50 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300'
  return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
}

function recordLuckyCoinText(record: ZenxiangLiyuRecord): string {
  if (!record.lucky_coin_played) return ''
  const amount = signedAmount(record.lucky_coin_adjustment)
  return record.lucky_coin_outcome === 'double'
    ? t('zenxiangLiyu.recordLuckyCoinWin', { amount })
    : t('zenxiangLiyu.recordLuckyCoinLose', { amount })
}

function wheelLabelStyle(index: number, count: number): Record<string, string> {
  const angle = (360 / Math.max(count, 1)) * (index + 0.5)
  return { transform: `translate(-50%, -50%) rotate(${angle}deg) translateY(-650%) rotate(${-angle}deg)` }
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function wheelLabel(name: string, index: number, count: number): string {
  return count > 8 ? String(index + 1) : name
}

function newRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `zenxiang-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function errorMessage(error: unknown, fallback = t('zenxiangLiyu.playFailed')): string {
  const apiError = error as {
    message?: unknown
    error?: unknown
    detail?: unknown
    response?: { data?: { message?: unknown, error?: unknown, detail?: unknown } }
  }
  const candidates = [
    apiError.response?.data?.message,
    apiError.response?.data?.error,
    apiError.response?.data?.detail,
    apiError.message,
    apiError.error,
    apiError.detail,
  ]
  const message = candidates.find((value): value is string => typeof value === 'string' && value.trim().length > 0)
  return message ? message.trim() : fallback
}

async function loadStatus(afterPlay: boolean): Promise<void> {
  loadError.value = ''
  statusRefreshError.value = ''
  try {
    await zenxiangLiyuStore.loadStatus()
  } catch {
    if (afterPlay) {
      statusRefreshError.value = t('zenxiangLiyu.statusRefreshFailed')
      return
    }
    loadError.value = t('zenxiangLiyu.loadFailed')
  }
}

async function loadTodayRecords(): Promise<void> {
  try {
    const response = await listZenxiangLiyuRecords({ page: 1, page_size: 20 })
    todayRecords.value = mergeCurrentResultIntoRecords(response.items)
  } catch {
    todayRecords.value = mergeCurrentResultIntoRecords([])
  }
}

async function refreshStatus(): Promise<void> {
  await Promise.all([loadStatus(false), loadTodayRecords()])
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function nextAnimationFrame(): Promise<void> {
  return new Promise((resolve) => window.requestAnimationFrame(() => resolve()))
}

async function spinToPrize(prizeId: number): Promise<void> {
  const prizes = status.value?.prizes ?? []
  const index = Math.max(0, prizes.findIndex((prize) => prize.id === prizeId))
  const count = Math.max(prizes.length, 1)
  const segment = 360 / count
  const targetCenter = index * segment + segment / 2
  const normalizedRotation = ((wheelRotation.value % 360) + 360) % 360
  const targetRotation = (360 - targetCenter + 360) % 360
  const deltaToTarget = (targetRotation - normalizedRotation + 360) % 360
  isSpinning.value = true
  await nextAnimationFrame()
  wheelRotation.value += 360 * 6 + deltaToTarget
  await wait(SPIN_DURATION_MS)
  isSpinning.value = false
}

async function play(): Promise<void> {
  if (!canPlay.value) return

  isPlaying.value = true
  playError.value = ''
  result.value = null
  luckyCoinResult.value = null
  luckyCoinError.value = ''
  luckyCoinFlipping.value = false
  showResultDialog.value = false
  try {
    const playResult = await playZenxiangLiyu(newRequestId())
    await spinToPrize(playResult.prize_id)
    result.value = playResult
    todayRecords.value = mergeCurrentResultIntoRecords(todayRecords.value)
    showResultDialog.value = true
    await Promise.allSettled([
      loadStatus(true),
      loadTodayRecords(),
      authStore.refreshUser(),
    ])
  } catch (error) {
    playError.value = errorMessage(error)
    isSpinning.value = false
  } finally {
    isPlaying.value = false
  }
}

async function playLuckyCoin(): Promise<void> {
  if (!result.value?.id || !canUseLuckyCoin.value) return

  luckyCoinError.value = ''
  luckyCoinFlipping.value = true
  try {
    await wait(900)
    const coinResult = await playZenxiangLiyuLuckyCoin(result.value.id)
    luckyCoinResult.value = coinResult
    result.value = { ...result.value, lucky_coin_available: false, lucky_coin_played: true }
    todayRecords.value = mergeCurrentResultIntoRecords(todayRecords.value)
    await Promise.allSettled([
      loadStatus(true),
      loadTodayRecords(),
      authStore.refreshUser(),
    ])
  } catch (error) {
    luckyCoinError.value = errorMessage(error, t('zenxiangLiyu.luckyCoinFailed'))
  } finally {
    luckyCoinFlipping.value = false
  }
}

onMounted(() => {
  void refreshStatus()
})

function mergeCurrentResultIntoRecords(records: ZenxiangLiyuRecord[]): ZenxiangLiyuRecord[] {
  if (!result.value) return records

  const next = [...records]
  const index = next.findIndex((record) => record.id === result.value?.id)
  const merged = applyLuckyCoinToRecord(index >= 0 ? next[index] : recordFromPlayResult(result.value))
  if (index >= 0) {
    next.splice(index, 1, merged)
  } else {
    next.unshift(merged)
  }
  return next
}

function recordFromPlayResult(playResult: ZenxiangLiyuPlayResult): ZenxiangLiyuRecord {
  const prize = status.value?.prizes.find((item) => item.id === playResult.prize_id)
  return {
    id: playResult.id,
    request_id: playResult.request_id,
    ticket_amount: playResult.ticket_amount,
    reward_amount: playResult.reward_amount,
    user_net_amount: playResult.user_net_amount,
    lucky_coin_played: playResult.lucky_coin_played,
    lucky_coin_outcome: '',
    lucky_coin_adjustment: 0,
    prize_id: playResult.prize_id,
    prize_name: playResult.prize_name,
    probability: prize?.probability ?? 0,
    played_at: playResult.played_at,
  }
}

function applyLuckyCoinToRecord(record: ZenxiangLiyuRecord): ZenxiangLiyuRecord {
  const coin = luckyCoinResult.value
  if (!coin || coin.record_id !== record.id) return record

  return {
    ...record,
    user_net_amount: Number(record.user_net_amount ?? 0) + Number(coin.adjustment_amount ?? 0),
    lucky_coin_played: true,
    lucky_coin_outcome: coin.outcome,
    lucky_coin_adjustment: coin.adjustment_amount,
    balance_after_lucky: coin.balance_after,
  }
}
</script>

<style scoped>
.zenxiang-wheel {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border: 8px solid rgb(229 231 235);
  border-radius: 9999px;
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.3);
  transform: translateZ(0);
  transition: transform 4.2s cubic-bezier(0.08, 0.82, 0.16, 1);
  will-change: transform;
}

.zenxiang-wheel--spinning {
  filter: saturate(1.08);
  box-shadow:
    inset 0 0 0 1px rgb(255 255 255 / 0.35),
    0 18px 35px rgb(15 118 110 / 0.14);
}

.dark .zenxiang-wheel {
  border-color: rgb(55 65 81);
}

.zenxiang-wheel__center {
  position: absolute;
  top: 50%;
  left: 50%;
  display: flex;
  width: 30%;
  aspect-ratio: 1;
  align-items: center;
  justify-content: center;
  border: 4px solid rgb(229 231 235);
  border-radius: 9999px;
  background: rgb(255 255 255);
  text-align: center;
  transform: translate(-50%, -50%);
}

.dark .zenxiang-wheel__center {
  border-color: rgb(55 65 81);
  background: rgb(31 41 55);
}

.zenxiang-wheel__label {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 30%;
  display: -webkit-box;
  overflow: hidden;
  color: white;
  font-size: 0.75rem;
  line-height: 1rem;
  text-align: center;
  text-overflow: ellipsis;
  word-break: break-word;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.zenxiang-play-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
}

.zenxiang-play-button__spinner {
  width: 1rem;
  height: 1rem;
  border: 2px solid rgb(255 255 255 / 0.45);
  border-top-color: rgb(255 255 255);
  border-radius: 9999px;
  animation: zenxiang-spin 0.8s linear infinite;
}

.zenxiang-record-item {
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.zenxiang-record-item:hover {
  border-color: rgb(16 185 129 / 0.35);
  box-shadow: 0 10px 24px rgb(15 23 42 / 0.08);
  transform: translateY(-1px);
}

.result-dialog {
  position: relative;
  overflow: hidden;
}

.result-dialog::before {
  position: absolute;
  inset: 0;
  pointer-events: none;
  content: '';
  background:
    radial-gradient(circle at 50% 0%, rgb(16 185 129 / 0.18), transparent 34%),
    linear-gradient(135deg, rgb(255 255 255 / 0.58), transparent 44%);
}

.result-dialog > * {
  position: relative;
  z-index: 1;
}

.reward-burst {
  animation: reward-burst 0.42s cubic-bezier(0.2, 0.85, 0.25, 1.2);
}

.lucky-result-panel {
  border-radius: 1rem;
  padding: 0.85rem 1rem;
  text-align: left;
}

.lucky-result-panel--win {
  border: 1px solid rgb(16 185 129 / 0.28);
  background: linear-gradient(135deg, rgb(236 253 245), rgb(209 250 229));
  color: rgb(6 95 70);
}

.lucky-result-panel--lose {
  border: 1px solid rgb(244 63 94 / 0.24);
  background: linear-gradient(135deg, rgb(255 241 242), rgb(255 228 230));
  color: rgb(159 18 57);
}

.dark .lucky-result-panel--win {
  background: linear-gradient(135deg, rgb(6 78 59 / 0.55), rgb(20 83 45 / 0.35));
  color: rgb(167 243 208);
}

.dark .lucky-result-panel--lose {
  background: linear-gradient(135deg, rgb(136 19 55 / 0.5), rgb(127 29 29 / 0.32));
  color: rgb(254 205 211);
}

.lucky-coin-card {
  position: relative;
  display: inline-flex;
  width: 100%;
  min-height: 3.5rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid rgb(202 138 4 / 0.5);
  border-radius: 0.5rem;
  background:
    radial-gradient(circle at 22% 18%, rgb(254 243 199 / 0.85), transparent 24%),
    linear-gradient(135deg, rgb(146 64 14), rgb(217 119 6) 45%, rgb(161 98 7));
  color: white;
  font-size: 0.875rem;
  font-weight: 700;
  letter-spacing: 0;
  text-shadow: 0 1px 1px rgb(0 0 0 / 0.18);
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.24),
    0 8px 18px rgb(180 83 9 / 0.18);
  transform-style: preserve-3d;
  transition: transform 0.2s ease, filter 0.2s ease, box-shadow 0.2s ease;
}

.lucky-coin-card:disabled {
  cursor: not-allowed;
  opacity: 0.9;
}

.lucky-coin-card:not(:disabled):hover {
  transform: translateY(-1px);
  filter: saturate(1.1);
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.28),
    0 10px 22px rgb(180 83 9 / 0.22);
}

.lucky-coin-card--flipping {
  animation: lucky-card-flip 0.9s cubic-bezier(0.2, 0.8, 0.2, 1) infinite;
}

.lucky-coin-card--win {
  border-color: rgb(52 211 153 / 0.85);
  background:
    radial-gradient(circle at 22% 18%, rgb(167 243 208 / 0.85), transparent 24%),
    linear-gradient(135deg, rgb(6 95 70), rgb(5 150 105), rgb(13 148 136));
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.22),
    0 8px 20px rgb(16 185 129 / 0.22);
}

.lucky-coin-card--lose {
  border-color: rgb(251 113 133 / 0.85);
  background:
    radial-gradient(circle at 22% 18%, rgb(254 205 211 / 0.85), transparent 24%),
    linear-gradient(135deg, rgb(136 19 55), rgb(190 18 60), rgb(127 29 29));
  box-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.2),
    0 8px 20px rgb(225 29 72 / 0.2);
}

.lucky-coin-card__shine {
  position: absolute;
  inset: -65% -30%;
  background: linear-gradient(115deg, transparent 38%, rgb(255 255 255 / 0.28), transparent 62%);
  transform: translateX(-55%) rotate(10deg);
  animation: lucky-shine 2.8s linear infinite;
}

.lucky-coin-card__face {
  position: relative;
  z-index: 1;
  padding: 0 0.875rem;
  text-align: center;
}

@keyframes lucky-card-flip {
  0% {
    transform: rotateY(0deg) scale(1);
  }
  50% {
    transform: rotateY(180deg) scale(1.04);
  }
  100% {
    transform: rotateY(360deg) scale(1);
  }
}

@keyframes lucky-shine {
  to {
    transform: translateX(45%) rotate(10deg);
  }
}

@keyframes reward-burst {
  0% {
    opacity: 0;
    transform: translateY(8px) scale(0.92);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes zenxiang-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
