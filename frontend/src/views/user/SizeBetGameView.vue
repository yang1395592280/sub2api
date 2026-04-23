<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-amber-200/70 bg-gradient-to-br from-amber-50 via-white to-rose-50 p-6 shadow-sm shadow-amber-100/60 dark:border-amber-500/20 dark:from-slate-900 dark:via-slate-900 dark:to-amber-950/30 dark:shadow-none">
        <div class="grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
          <div class="space-y-5">
            <div class="inline-flex w-fit items-center rounded-full bg-white/80 px-3 py-1 text-xs font-medium uppercase tracking-[0.28em] text-amber-600 ring-1 ring-amber-200/70 dark:bg-white/10 dark:text-amber-200 dark:ring-white/10">{{ statusBadgeLabel }}</div>
            <div>
              <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white">{{ t('sizeBet.title') }}</h1>
              <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.heroSubtitle') }}</p>
            </div>
            <div class="flex flex-wrap items-end gap-4">
              <div>
                <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.countdownLabel') }}</p>
                <p class="text-5xl font-semibold tabular-nums text-slate-950 dark:text-white">{{ roundCountdownDisplay }}</p>
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

      <div v-if="loadState === 'loading'" class="flex justify-center py-16"><LoadingSpinner /></div>
      <section v-else-if="loadState === 'error'" class="card px-6 py-12">
        <EmptyState :title="t('sizeBet.loadError.title')" :description="loadErrorMessage || t('sizeBet.loadError.description')" :action-text="t('common.retry')" @action="loadPage" />
      </section>
      <section v-else-if="!currentView?.enabled" class="card px-6 py-12">
        <EmptyState :title="t('sizeBet.maintenance.title')" :description="t('sizeBet.maintenance.description')" />
      </section>
      <div v-else class="space-y-6">
        <div class="grid gap-6 xl:grid-cols-[1.08fr_0.92fr]">
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
                  <p v-if="previousRound?.server_seed" class="mt-2 break-all text-xs text-slate-500 dark:text-slate-400">{{ t('sizeBet.previousRound.reveal', { seed: previousRound.server_seed }) }}</p>
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
                <div class="rounded-full px-3 py-1 text-xs font-medium" :class="isBettingOpen ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300' : 'bg-slate-100 text-slate-600 dark:bg-white/10 dark:text-slate-300'">{{ phaseLabel }}</div>
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
                  <button v-for="option in directionOptions" :key="option.value" type="button" class="rounded-2xl border px-4 py-4 text-left transition disabled:cursor-not-allowed disabled:opacity-60" :class="selectedDirection === option.value ? 'border-amber-400 bg-amber-50 text-amber-900 shadow-sm dark:border-amber-400 dark:bg-amber-500/10 dark:text-amber-100' : 'border-slate-200 bg-white text-slate-700 hover:border-amber-200 hover:bg-amber-50/60 dark:border-white/10 dark:bg-white/5 dark:text-slate-200 dark:hover:border-amber-500/30 dark:hover:bg-amber-500/10'" :data-test="`direction-${option.value}`" :disabled="controlsLocked" @click="selectedDirection = option.value">
                    <p class="text-lg font-semibold">{{ option.label }}</p>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ option.odd }}x</p>
                  </button>
                </div>
              </div>
              <div class="space-y-3">
                <p class="text-sm font-medium text-slate-700 dark:text-slate-200">{{ t('sizeBet.player.chooseStake') }}</p>
                <div class="grid grid-cols-4 gap-3">
                  <button v-for="stake in allowedStakes" :key="stake" type="button" class="rounded-2xl border px-3 py-3 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60" :class="selectedStake === stake ? 'border-slate-900 bg-slate-900 text-white dark:border-amber-300 dark:bg-amber-300 dark:text-slate-950' : 'border-slate-200 bg-white text-slate-700 hover:border-slate-300 dark:border-white/10 dark:bg-white/5 dark:text-slate-200'" :data-test="`stake-${stake}`" :disabled="controlsLocked" @click="selectedStake = stake">
                    {{ stake }}
                  </button>
                </div>
              </div>
              <button type="button" class="btn btn-primary h-12 w-full justify-center text-base" data-test="bet-submit" :disabled="submitDisabled" @click="submitBet">{{ submitting ? t('sizeBet.player.submitting') : t('sizeBet.player.submit') }}</button>
            </div>
          </section>
        </div>

        <section class="card overflow-hidden">
          <div class="flex items-center justify-between gap-4 border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <div>
              <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.history.title') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.history.subtitle') }}</p>
            </div>
            <button v-if="recentHistory.length > visibleHistory.length" type="button" class="btn btn-secondary btn-sm" @click="showAllHistory = !showAllHistory">{{ t(showAllHistory ? 'sizeBet.history.toggleLess' : 'sizeBet.history.toggleMore') }}</button>
          </div>
          <div class="space-y-3 px-6 py-6">
            <div v-if="historyRefreshError" class="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-100">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <p>{{ historyRefreshError }}</p>
                <button type="button" class="btn btn-secondary btn-sm" data-test="history-retry" @click="retryHistory">{{ t('sizeBet.history.refreshRetry') }}</button>
              </div>
            </div>
            <p v-if="!recentHistory.length" class="text-sm text-slate-500 dark:text-slate-400">{{ t('sizeBet.history.empty') }}</p>
            <div v-for="item in visibleHistory" :key="item.bet_id" class="rounded-2xl bg-slate-50/90 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('sizeBet.history.roundLabel', { round: item.round_no }) }}</p>
                  <p class="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">{{ historyStatusLabel(item.status) }}</p>
                </div>
                <p class="text-sm font-semibold" :class="historyAmountClass(item)">{{ historyAmountLabel(item) }}</p>
              </div>
              <div class="mt-3 grid gap-2 text-sm text-slate-600 dark:text-slate-300 md:grid-cols-2">
                <p>{{ t('sizeBet.history.selection', { direction: directionLabel(item.direction) }) }}</p>
                <p>{{ historyResultLabel(item) }}</p>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
    <BaseDialog :show="!!resultModal" :title="t('sizeBet.resultModal.title')" width="narrow" @close="resultModal = null">
      <div v-if="resultModal" class="space-y-4">
        <div class="rounded-2xl p-4" :class="resultBannerClass(resultModal)">
          <p class="text-sm font-medium">{{ t('sizeBet.resultModal.roundLabel', { round: resultModal.round_no }) }}</p>
          <p class="mt-2 text-2xl font-semibold" :class="resultAmountClass(resultModal)">{{ resultSummary(resultModal) }}</p>
          <p class="mt-2 text-sm leading-6 text-slate-700 dark:text-slate-200">{{ resultMessage(resultModal) }}</p>
        </div>
        <div class="rounded-2xl bg-slate-50 p-4 ring-1 ring-slate-200/80 dark:bg-white/5 dark:ring-white/10">
          <p class="text-sm text-slate-600 dark:text-slate-300">{{ t('sizeBet.resultModal.selection', { direction: directionLabel(resultModal.direction), stake: formatAmount(resultModal.stake_amount) }) }}</p>
          <p class="mt-2 text-sm text-slate-600 dark:text-slate-300">{{ resultDetailLabel(resultModal) }}</p>
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-primary" @click="resultModal = null">{{ t('sizeBet.resultModal.close') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>
<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { sizeBetAPI } from '@/api'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { SizeBetCurrentRoundView, SizeBetDirection, SizeBetHistoryItem, SizeBetPhase, SizeBetRulesView, SizeBetStatus } from '@/types/sizeBet'
type LoadState = 'loading' | 'ready' | 'error'
type HistoryRefreshMode = 'background' | 'manual'
const TICK_MS = 1000
const HISTORY_PAGE_SIZE = 10
const VISIBLE_HISTORY_COUNT = 5
const MAINTENANCE_POLL_MS = 15000
const ROUND_SYNC_MS = 3000
const RESUME_SYNC_MS = 1000
const LAST_SEEN_SETTLED_BET_KEY = 'size-bet:last-seen-settled-bet'
marked.setOptions({ breaks: true, gfm: true })
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const loadState = ref<LoadState>('loading')
const loadErrorMessage = ref('')
const submitting = ref(false)
const currentView = ref<SizeBetCurrentRoundView | null>(null)
const rules = ref<SizeBetRulesView | null>(null)
const selectedDirection = ref<SizeBetDirection | null>(null)
const selectedStake = ref<number | null>(null)
const showAllHistory = ref(false)
const syncedServerMs = ref<number | null>(null)
const syncedClientMs = ref<number | null>(null)
const clientNowMs = ref(Date.now())
const lastRoundId = ref<number | null>(null)
const lastAutoSyncAt = ref(0)
const lastSeenSettledBetId = ref<number | null>(null)
const historyRefreshError = ref('')
const recentHistory = ref<SizeBetHistoryItem[]>([])
const resultModal = ref<SizeBetHistoryItem | null>(null)
let tickTimer: number | null = null
let syncInFlight = false
let historySyncInFlight = false
const currentRound = computed(() => currentView.value?.round ?? null)
const currentBet = computed(() => currentView.value?.my_bet ?? null)
const previousRound = computed(() => currentView.value?.previous_round ?? null)
const allowedStakes = computed(() => currentRound.value?.allowed_stakes ?? rules.value?.allowed_stakes ?? [])
const visibleHistory = computed(() => showAllHistory.value ? recentHistory.value : recentHistory.value.slice(0, VISIBLE_HISTORY_COUNT))
const estimatedServerNowMs = computed(() => syncedServerMs.value == null || syncedClientMs.value == null ? clientNowMs.value : syncedServerMs.value + (clientNowMs.value - syncedClientMs.value))
const closeCountdownSeconds = computed(() => secondsUntil(currentRound.value?.bet_closes_at, currentRound.value?.bet_countdown_seconds ?? 0))
const roundCountdownSeconds = computed(() => secondsUntil(currentRound.value?.settles_at, currentRound.value?.countdown_seconds ?? 0))
const phase = computed<SizeBetPhase>(() => !currentView.value?.enabled ? 'maintenance' : !currentRound.value ? currentView.value?.phase ?? 'maintenance' : closeCountdownSeconds.value > 0 ? 'betting' : 'closed')
const statusBadgeLabel = computed(() => loadState.value === 'loading' ? t('common.loading') : loadState.value === 'error' ? t('sizeBet.loadError.badge') : t(`sizeBet.phase.${phase.value}`))
const phaseLabel = computed(() => t(`sizeBet.phase.${phase.value}`))
const roundCountdownDisplay = computed(() => currentRound.value ? roundCountdownSeconds.value : '--')
const closeCountdownDisplay = computed(() => currentRound.value ? closeCountdownSeconds.value : '--')
const isBettingOpen = computed(() => loadState.value === 'ready' && currentView.value?.enabled === true && !!currentRound.value && closeCountdownSeconds.value > 0)
const controlsLocked = computed(() => submitting.value || !isBettingOpen.value || !!currentBet.value)
const submitDisabled = computed(() => controlsLocked.value || !selectedDirection.value || selectedStake.value == null)
const rulesHtml = computed(() => DOMPurify.sanitize((marked.parse(rules.value?.rules_markdown ?? '') as string) || ''))
const progressWidth = computed(() => {
  const closeOffset = rules.value?.bet_close_offset_seconds ?? 0
  if (!currentRound.value || closeOffset <= 0) return 0
  return Math.min(100, Math.max(0, ((closeOffset - closeCountdownSeconds.value) / closeOffset) * 100))
})
const directionOptions = computed(() => [
  { value: 'small' as const, label: t('sizeBet.directions.small'), odd: currentRound.value?.odds_small ?? rules.value?.odds.small ?? 0 },
  { value: 'mid' as const, label: t('sizeBet.directions.mid'), odd: currentRound.value?.odds_mid ?? rules.value?.odds.mid ?? 0 },
  { value: 'big' as const, label: t('sizeBet.directions.big'), odd: currentRound.value?.odds_big ?? rules.value?.odds.big ?? 0 },
])
const selectionSummary = computed(() => currentBet.value ? t('sizeBet.player.myBet', { direction: directionLabel(currentBet.value.direction), stake: currentBet.value.stake_amount }) : !selectedDirection.value || selectedStake.value == null ? t('sizeBet.player.pending') : `${directionLabel(selectedDirection.value)} / ${selectedStake.value}`)
const statusHint = computed(() => currentBet.value ? t('sizeBet.player.placedHint') : isBettingOpen.value ? t('sizeBet.player.openHint') : t('sizeBet.player.closedHint'))
const previousRoundSummary = computed(() => !previousRound.value?.result_number || !previousRound.value.result_direction ? t('sizeBet.previousRound.empty') : t('sizeBet.previousRound.result', { round: previousRound.value.round_no, number: previousRound.value.result_number, direction: directionLabel(previousRound.value.result_direction) }))

function directionLabel(direction: SizeBetDirection) { return t(`sizeBet.directions.${direction}`) }
function historyStatusLabel(status: SizeBetStatus) { return t(`sizeBet.history.status.${status}`) }
function formatAmount(value: number) { return Number.isInteger(value) ? `${value}` : value.toFixed(2).replace(/\.?0+$/, '') }
function formatSigned(value: number) { return `${value >= 0 ? '+' : '-'}${formatAmount(Math.abs(value))}` }
function historyAmountLabel(item: SizeBetHistoryItem) {
  if (item.status === 'placed') return t('sizeBet.history.pendingAmount')
  if (item.status === 'refunded') return t('sizeBet.history.refundedAmount', { amount: formatAmount(item.stake_amount) })
  return formatSigned(item.net_result_amount)
}
function historyAmountClass(item: SizeBetHistoryItem) { return item.status === 'won' ? 'text-emerald-600 dark:text-emerald-300' : item.status === 'lost' ? 'text-rose-600 dark:text-rose-300' : 'text-slate-600 dark:text-slate-300' }
function historyResultLabel(item: SizeBetHistoryItem) { return item.result_number == null || !item.result_direction ? t('sizeBet.history.pendingResult') : t('sizeBet.history.result', { number: item.result_number, direction: directionLabel(item.result_direction) }) }
function resultDetailLabel(item: SizeBetHistoryItem) { return item.result_number == null || !item.result_direction ? t('sizeBet.history.pendingResult') : t('sizeBet.resultModal.result', { number: item.result_number, direction: directionLabel(item.result_direction) }) }
function resultAmountClass(item: SizeBetHistoryItem) { return item.status === 'won' ? 'text-emerald-600 dark:text-emerald-300' : item.status === 'lost' ? 'text-rose-600 dark:text-rose-300' : 'text-slate-700 dark:text-slate-100' }
function resultBannerClass(item: SizeBetHistoryItem) { return item.status === 'won' ? 'bg-emerald-50 ring-1 ring-emerald-200 dark:bg-emerald-500/10 dark:ring-emerald-500/20' : item.status === 'lost' ? 'bg-rose-50 ring-1 ring-rose-200 dark:bg-rose-500/10 dark:ring-rose-500/20' : 'bg-amber-50 ring-1 ring-amber-200 dark:bg-amber-500/10 dark:ring-amber-500/20' }
function resultSummary(item: SizeBetHistoryItem) { return t(`sizeBet.resultModal.summary.${item.status}`, { amount: item.status === 'refunded' ? formatAmount(item.stake_amount) : formatSigned(item.net_result_amount) }) }
function resultMessage(item: SizeBetHistoryItem) { return t(`sizeBet.resultModal.message.${item.status}`) }
function parseMs(value?: string | null) { const parsed = value ? Date.parse(value) : Number.NaN; return Number.isNaN(parsed) ? null : parsed }
function secondsUntil(target?: string | null, fallback = 0) { const targetMs = parseMs(target); return targetMs == null ? Math.max(0, fallback) : Math.max(0, Math.ceil((targetMs - estimatedServerNowMs.value) / 1000)) }
function syncClock(serverTime: string) { const now = Date.now(); syncedServerMs.value = parseMs(serverTime) ?? now; syncedClientMs.value = now; clientNowMs.value = now }
function lastSeenSettledBetStorageKey() { return `${LAST_SEEN_SETTLED_BET_KEY}:${authStore.user?.id ?? 'guest'}` }
function restoreLastSeenSettledBetId() {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.sessionStorage.getItem(lastSeenSettledBetStorageKey())
    const parsed = raw == null ? Number.NaN : Number.parseInt(raw, 10)
    return Number.isFinite(parsed) ? parsed : null
  } catch {
    return null
  }
}
function persistLastSeenSettledBetId(betId: number | null) {
  if (typeof window === 'undefined') return
  try {
    const key = lastSeenSettledBetStorageKey()
    if (betId == null) window.sessionStorage.removeItem(key)
    else window.sessionStorage.setItem(key, `${betId}`)
  } catch {
    // 忽略存储不可用场景，避免影响主流程
  }
}
function syncSelection(view: SizeBetCurrentRoundView | null) {
  const nextRoundId = view?.round?.id ?? null
  const stakes = view?.round?.allowed_stakes ?? rules.value?.allowed_stakes ?? []
  if (view?.my_bet) {
    selectedDirection.value = view.my_bet.direction
    selectedStake.value = view.my_bet.stake_amount
  } else if (nextRoundId !== lastRoundId.value) {
    selectedDirection.value = null
    selectedStake.value = stakes[0] ?? null
  } else if (!stakes.includes(selectedStake.value ?? Number.NaN)) {
    selectedStake.value = stakes[0] ?? null
  }
  lastRoundId.value = nextRoundId
}
function maybeAutoSync(intervalMs: number) {
  if (syncInFlight || Date.now() - lastAutoSyncAt.value < intervalMs) return
  void syncCurrent(true).catch(() => undefined)
}
function maybeOpenResultModal(items: SizeBetHistoryItem[]) {
  const latest = items[0]
  if (!latest || latest.status === 'placed' || latest.bet_id === lastSeenSettledBetId.value) return
  lastSeenSettledBetId.value = latest.bet_id
  persistLastSeenSettledBetId(latest.bet_id)
  resultModal.value = latest
}
async function refreshRules(silent = true) {
  try {
    rules.value = await sizeBetAPI.getRules()
    syncSelection(currentView.value)
  } catch (error: any) {
    if (!silent) throw error
  }
}
async function loadHistory(mode: HistoryRefreshMode = 'background') {
  if (historySyncInFlight) return
  historySyncInFlight = true
  try {
    const response = await sizeBetAPI.getHistory(1, HISTORY_PAGE_SIZE)
    recentHistory.value = response.items
    historyRefreshError.value = ''
    maybeOpenResultModal(response.items)
  } catch (error: any) {
    const hadHistoryRefreshError = !!historyRefreshError.value
    const fallbackMessage = t('sizeBet.history.refreshFailed')
    historyRefreshError.value = fallbackMessage
    if (mode === 'manual') {
      appStore.showError(error?.message || fallbackMessage)
    } else if (!hadHistoryRefreshError) {
      appStore.showWarning(fallbackMessage)
    }
  } finally {
    historySyncInFlight = false
  }
}
async function syncCurrent(silent = true) {
  if (syncInFlight) return
  syncInFlight = true
  lastAutoSyncAt.value = Date.now()
  try {
    const nextView = await sizeBetAPI.getCurrent()
    const recovered = currentView.value?.enabled === false && nextView.enabled
    currentView.value = nextView
    syncClock(nextView.server_time)
    syncSelection(nextView)
    loadState.value = 'ready'
    loadErrorMessage.value = ''
    await loadHistory()
    if (recovered) void refreshRules(true)
  } catch (error: any) {
    if (!silent) appStore.showError(error?.message || t('common.error'))
    throw error
  } finally {
    syncInFlight = false
  }
}
async function loadPage() {
  loadState.value = 'loading'
  loadErrorMessage.value = ''
  try {
    const [view, rulesView] = await Promise.all([sizeBetAPI.getCurrent(), sizeBetAPI.getRules()])
    rules.value = rulesView
    currentView.value = view
    syncClock(view.server_time)
    syncSelection(view)
    lastAutoSyncAt.value = Date.now()
    loadState.value = 'ready'
    await loadHistory()
  } catch (error: any) {
    currentView.value = null
    lastRoundId.value = null
    loadState.value = 'error'
    loadErrorMessage.value = error?.message || t('sizeBet.loadError.description')
    appStore.showError(loadErrorMessage.value)
  }
}
function handleResumeSync() {
  clientNowMs.value = Date.now()
  if (loadState.value !== 'ready' || syncInFlight || Date.now() - lastAutoSyncAt.value < RESUME_SYNC_MS) return
  void syncCurrent(true).catch(() => undefined)
}
function handleVisibilityChange() {
  if (document.visibilityState === 'visible') handleResumeSync()
}
function tick() {
  clientNowMs.value = Date.now()
  if (loadState.value !== 'ready') return
  if (currentView.value?.enabled === false) return maybeAutoSync(MAINTENANCE_POLL_MS)
  if (!currentRound.value || roundCountdownSeconds.value <= 0) maybeAutoSync(ROUND_SYNC_MS)
}
function buildIdempotencyKey() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `size-bet-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
function retryHistory() {
  void loadHistory('manual')
}
async function submitBet() {
  if (!selectedDirection.value) return appStore.showError(t('sizeBet.player.selectDirection'))
  if (selectedStake.value == null || !currentRound.value) return appStore.showError(t('sizeBet.player.selectStake'))
  submitting.value = true
  try {
    const bet = await sizeBetAPI.placeBet({ round_id: currentRound.value.id, direction: selectedDirection.value, stake_amount: selectedStake.value, idempotency_key: buildIdempotencyKey() })
    if (currentView.value) currentView.value.my_bet = bet
    syncSelection(currentView.value)
    await loadHistory()
    appStore.showSuccess(t('sizeBet.player.placedSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('common.error'))
  } finally {
    submitting.value = false
  }
}
onMounted(() => {
  lastSeenSettledBetId.value = restoreLastSeenSettledBetId()
  void loadPage()
  tickTimer = window.setInterval(tick, TICK_MS)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  window.addEventListener('focus', handleResumeSync)
})
onBeforeUnmount(() => {
  if (tickTimer !== null) window.clearInterval(tickTimer)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  window.removeEventListener('focus', handleResumeSync)
})
</script>
