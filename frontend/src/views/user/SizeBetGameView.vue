<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-amber-200/70 bg-gradient-to-br from-amber-50 via-white to-rose-50 p-6 shadow-sm shadow-amber-100/60 dark:border-amber-500/20 dark:from-slate-900 dark:via-slate-900 dark:to-amber-950/30 dark:shadow-none">
        <div class="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
          <div class="space-y-5">
            <div class="inline-flex w-fit items-center rounded-full bg-white/80 px-3 py-1 text-xs font-medium uppercase tracking-[0.28em] text-amber-600 ring-1 ring-amber-200/70 dark:bg-white/10 dark:text-amber-200 dark:ring-white/10">
              {{ phaseLabel }}
            </div>
            <div>
              <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white">{{ t('sizeBet.title') }}</h1>
              <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.heroSubtitle') }}</p>
            </div>
            <div class="flex flex-wrap items-end gap-4">
              <div>
                <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.countdownLabel') }}</p>
                <p class="text-5xl font-semibold tabular-nums text-slate-950 dark:text-white">{{ currentRound?.countdown_seconds ?? '--' }}</p>
              </div>
              <div class="rounded-2xl bg-white/80 px-4 py-3 ring-1 ring-slate-200/70 backdrop-blur dark:bg-white/10 dark:ring-white/10">
                <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.betClosesIn') }}</p>
                <p class="mt-1 text-2xl font-semibold tabular-nums text-slate-900 dark:text-white">{{ closeCountdownDisplay }}</p>
              </div>
            </div>
            <div class="space-y-2">
              <div class="flex items-center justify-between gap-4 text-xs font-medium text-slate-500 dark:text-slate-400">
                <span>{{ t('sizeBet.player.currentSelection') }}</span>
                <span class="text-right">{{ selectionSummary }}</span>
              </div>
              <div class="h-3 overflow-hidden rounded-full bg-white/70 ring-1 ring-slate-200/70 dark:bg-white/10 dark:ring-white/10">
                <div class="h-full rounded-full bg-gradient-to-r from-amber-400 via-orange-400 to-rose-400 transition-all duration-1000" :style="{ width: `${progressWidth}%` }"></div>
              </div>
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            <div class="rounded-2xl bg-white/85 p-4 ring-1 ring-slate-200/70 backdrop-blur dark:bg-white/10 dark:ring-white/10">
              <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.dealer.title') }}</p>
              <p class="mt-2 text-xl font-semibold text-slate-900 dark:text-white">{{ currentRound?.round_no ?? '--' }}</p>
            </div>
            <div class="rounded-2xl bg-white/85 p-4 ring-1 ring-slate-200/70 backdrop-blur dark:bg-white/10 dark:ring-white/10">
              <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.previousRound.title') }}</p>
              <p class="mt-2 text-sm font-medium text-slate-900 dark:text-white">{{ previousRoundSummary }}</p>
            </div>
            <div class="rounded-2xl bg-slate-950/90 p-4 text-white shadow-sm dark:bg-slate-900">
              <p class="text-xs uppercase tracking-[0.22em] text-amber-200/80">{{ t('sizeBet.seedTitle') }}</p>
              <p class="mt-2 break-all text-sm font-medium text-amber-50">{{ currentRound?.server_seed_hash ?? previousRound?.server_seed_hash ?? '--' }}</p>
            </div>
          </div>
        </div>
      </section>
      <div v-if="loading" class="flex justify-center py-16"><LoadingSpinner /></div>
      <template v-else>
        <section v-if="!currentView?.enabled" class="card px-6 py-12">
          <EmptyState :title="t('sizeBet.maintenance.title')" :description="t('sizeBet.maintenance.description')" />
        </section>
        <div v-else class="grid gap-6 xl:grid-cols-[1.08fr_0.92fr]">
          <section class="card overflow-hidden">
            <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
              <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.dealer.title') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.dealer.duration', { seconds: rules?.round_duration_seconds ?? '--', close: rules?.bet_close_offset_seconds ?? '--' }) }}</p>
            </div>
            <div class="grid gap-6 px-6 py-6 lg:grid-cols-[0.94fr_1.06fr]">
              <div class="space-y-4">
                <div class="rounded-2xl bg-amber-50/80 p-4 ring-1 ring-amber-100 dark:bg-amber-500/10 dark:ring-amber-500/20">
                  <p class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('sizeBet.dealer.roundLabel', { round: currentRound?.round_no ?? '--' }) }}</p>
                  <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.dealer.probability', { small: currentRound?.prob_small ?? 0, mid: currentRound?.prob_mid ?? 0, big: currentRound?.prob_big ?? 0 }) }}</p>
                  <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.dealer.odds', { small: currentRound?.odds_small ?? 0, mid: currentRound?.odds_mid ?? 0, big: currentRound?.odds_big ?? 0 }) }}</p>
                </div>
                <div class="rounded-2xl bg-slate-50 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
                  <h3 class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.previousRound.title') }}</h3>
                  <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ previousRoundSummary }}</p>
                  <p v-if="previousRound?.server_seed" class="mt-2 text-xs break-all text-slate-500 dark:text-slate-400">{{ t('sizeBet.previousRound.reveal', { seed: previousRound.server_seed }) }}</p>
                </div>
              </div>
              <div class="rounded-2xl bg-white p-5 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
                <h3 class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.rules.title') }}</h3>
                <div class="markdown-body prose prose-sm mt-4 max-w-none dark:prose-invert" v-html="rulesHtml"></div>
              </div>
            </div>
          </section>
          <section class="card overflow-hidden">
            <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.player.title') }}</h2>
                  <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ statusHint }}</p>
                </div>
                <div class="rounded-full px-3 py-1 text-xs font-medium" :class="isBettingOpen ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300' : 'bg-slate-100 text-slate-600 dark:bg-white/10 dark:text-slate-300'">
                  {{ phaseLabel }}
                </div>
              </div>
            </div>
            <div class="space-y-6 px-6 py-6">
              <div class="rounded-2xl bg-slate-50 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
                <p class="text-xs uppercase tracking-[0.22em] text-slate-500 dark:text-slate-400">{{ t('sizeBet.player.currentSelection') }}</p>
                <p class="mt-2 text-lg font-semibold text-slate-900 dark:text-white">{{ selectionSummary }}</p>
              </div>
              <div class="space-y-3">
                <p class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('sizeBet.player.chooseDirection') }}</p>
                <div class="grid grid-cols-3 gap-3">
                  <button v-for="option in directionOptions" :key="option.value" type="button" class="rounded-2xl border px-4 py-4 text-left transition" :class="selectedDirection === option.value ? 'border-amber-400 bg-amber-50 text-amber-900 shadow-sm dark:border-amber-400 dark:bg-amber-500/10 dark:text-amber-100' : 'border-slate-200 bg-white text-slate-700 hover:border-amber-200 hover:bg-amber-50/60 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:border-amber-500/30 dark:hover:bg-amber-500/10'" :data-test="`direction-${option.value}`" @click="selectedDirection = option.value">
                    <p class="text-lg font-semibold">{{ option.label }}</p>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ option.odd }}x</p>
                  </button>
                </div>
              </div>
              <div class="space-y-3">
                <p class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('sizeBet.player.chooseStake') }}</p>
                <div class="grid grid-cols-4 gap-3">
                  <button v-for="stake in allowedStakes" :key="stake" type="button" class="rounded-2xl border px-3 py-3 text-sm font-medium transition" :class="selectedStake === stake ? 'border-slate-900 bg-slate-900 text-white dark:border-amber-300 dark:bg-amber-300 dark:text-slate-950' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 dark:border-white/10 dark:bg-white/5 dark:text-slate-200'" :data-test="`stake-${stake}`" @click="selectedStake = stake">
                    {{ stake }}
                  </button>
                </div>
              </div>
              <button type="button" class="btn btn-primary h-12 w-full justify-center text-base" data-test="bet-submit" :disabled="submitDisabled" @click="submitBet">
                {{ submitting ? t('sizeBet.player.submitting') : t('sizeBet.player.submit') }}
              </button>
            </div>
          </section>
        </div>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { sizeBetAPI } from '@/api'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import type { SizeBetCurrentRoundView, SizeBetDirection, SizeBetRulesView } from '@/types/sizeBet'
marked.setOptions({ breaks: true, gfm: true })
const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const submitting = ref(false)
const currentView = ref<SizeBetCurrentRoundView | null>(null)
const rules = ref<SizeBetRulesView | null>(null)
const selectedDirection = ref<SizeBetDirection | null>(null)
const selectedStake = ref<number | null>(null)
let ticker: number | null = null
let refreshInFlight = false
const currentRound = computed(() => currentView.value?.round ?? null)
const currentBet = computed(() => currentView.value?.my_bet ?? null)
const previousRound = computed(() => currentView.value?.previous_round ?? null)
const allowedStakes = computed(() => currentRound.value?.allowed_stakes ?? rules.value?.allowed_stakes ?? [])
const isBettingOpen = computed(() => currentView.value?.phase === 'betting' && (currentRound.value?.bet_countdown_seconds ?? 0) > 0)
const closeCountdownDisplay = computed(() => currentRound.value?.bet_countdown_seconds ?? '--')
const phaseLabel = computed(() => t(`sizeBet.phase.${currentView.value?.phase ?? 'maintenance'}`))
const rulesHtml = computed(() => DOMPurify.sanitize((marked.parse(rules.value?.rules_markdown ?? '') as string) || ''))
const progressWidth = computed(() => {
  const closeOffset = rules.value?.bet_close_offset_seconds ?? 0
  if (!currentRound.value || closeOffset <= 0) return 0
  const elapsed = closeOffset - Math.max(currentRound.value.bet_countdown_seconds, 0)
  return Math.min(100, Math.max(0, (elapsed / closeOffset) * 100))
})
const directionOptions = computed(() => [
  { value: 'small' as const, label: t('sizeBet.directions.small'), odd: currentRound.value?.odds_small ?? rules.value?.odds.small ?? 0 },
  { value: 'mid' as const, label: t('sizeBet.directions.mid'), odd: currentRound.value?.odds_mid ?? rules.value?.odds.mid ?? 0 },
  { value: 'big' as const, label: t('sizeBet.directions.big'), odd: currentRound.value?.odds_big ?? rules.value?.odds.big ?? 0 },
])
const submitDisabled = computed(() => {
  return submitting.value || !isBettingOpen.value || !!currentBet.value || !selectedDirection.value || selectedStake.value == null
})
const selectionSummary = computed(() => {
  if (currentBet.value) {
    return t('sizeBet.player.myBet', {
      direction: directionLabel(currentBet.value.direction),
      stake: currentBet.value.stake_amount,
    })
  }
  if (!selectedDirection.value || selectedStake.value == null) {
    return t('sizeBet.player.pending')
  }
  return `${directionLabel(selectedDirection.value)} / ${selectedStake.value}`
})
const statusHint = computed(() => {
  if (currentBet.value) return t('sizeBet.player.placedHint')
  return isBettingOpen.value ? t('sizeBet.player.openHint') : t('sizeBet.player.closedHint')
})
const previousRoundSummary = computed(() => {
  if (!previousRound.value?.result_number || !previousRound.value.result_direction) {
    return t('sizeBet.previousRound.empty')
  }
  return t('sizeBet.previousRound.result', {
    round: previousRound.value.round_no,
    number: previousRound.value.result_number,
    direction: directionLabel(previousRound.value.result_direction),
  })
})
function directionLabel(direction: SizeBetDirection) {
  return t(`sizeBet.directions.${direction}`)
}
function syncSelection() {
  if (currentBet.value) {
    selectedDirection.value = currentBet.value.direction
    selectedStake.value = currentBet.value.stake_amount
    return
  }
  if (!allowedStakes.value.includes(selectedStake.value ?? -1)) {
    selectedStake.value = allowedStakes.value[0] ?? null
  }
}
function buildIdempotencyKey() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `size-bet-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
async function refreshCurrent(silent = false) {
  try {
    currentView.value = await sizeBetAPI.getCurrent()
    syncSelection()
  } catch (error: any) {
    if (!silent) {
      appStore.showError(error?.message || t('common.error'))
    }
    throw error
  }
}
async function loadPage() {
  loading.value = true
  try {
    const [view, rulesView] = await Promise.all([sizeBetAPI.getCurrent(), sizeBetAPI.getRules()])
    currentView.value = view
    rules.value = rulesView
    syncSelection()
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    loading.value = false
  }
}
function tickCountdown() {
  const view = currentView.value
  const round = view?.round
  if (!view || !round) return
  if (round.countdown_seconds > 0) round.countdown_seconds -= 1
  if (round.bet_countdown_seconds > 0) round.bet_countdown_seconds -= 1
  if (view.phase === 'betting' && round.bet_countdown_seconds <= 0) {
    view.phase = 'closed'
  }
  if (round.countdown_seconds <= 0 && !refreshInFlight) {
    refreshInFlight = true
    void refreshCurrent(true).finally(() => {
      refreshInFlight = false
    })
  }
}
async function submitBet() {
  if (!selectedDirection.value) {
    appStore.showError(t('sizeBet.player.selectDirection'))
    return
  }
  if (selectedStake.value == null || !currentRound.value) {
    appStore.showError(t('sizeBet.player.selectStake'))
    return
  }
  submitting.value = true
  try {
    const bet = await sizeBetAPI.placeBet({
      round_id: currentRound.value.id,
      direction: selectedDirection.value,
      stake_amount: selectedStake.value,
      idempotency_key: buildIdempotencyKey(),
    })
    if (currentView.value) {
      currentView.value.my_bet = bet
    }
    syncSelection()
    appStore.showSuccess(t('sizeBet.player.placedSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    submitting.value = false
  }
}
onMounted(() => {
  void loadPage()
  ticker = window.setInterval(tickCountdown, 1000)
})
onBeforeUnmount(() => {
  if (ticker !== null) window.clearInterval(ticker)
})
</script>
