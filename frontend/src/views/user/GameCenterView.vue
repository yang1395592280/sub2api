<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="overflow-hidden rounded-[28px] border border-cyan-200/60 bg-[radial-gradient(circle_at_top_left,_#0f766e,_#0f172a)] p-6 text-white shadow-lg shadow-cyan-900/20">
        <p class="text-xs uppercase tracking-[0.3em] text-cyan-100">{{ t('gameCenter.hero.tag') }}</p>
        <div class="mt-3 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div class="space-y-2">
            <h1 class="text-3xl font-semibold tracking-tight">{{ t('gameCenter.hero.title') }}</h1>
            <p class="text-sm text-cyan-100">
              {{ t('gameCenter.hero.subtitle') }}
              <span class="ml-2 text-2xl font-semibold text-white">{{ formatPoints(overview?.points ?? 0) }}</span>
            </p>
            <p class="text-sm text-cyan-100">{{ authStore.user?.username }}</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary border-white/30 bg-white/10 text-white hover:bg-white/20"
            data-test="exchange-entry"
            @click="exchangeOpen = !exchangeOpen"
          >
            {{ t('gameCenter.exchange.entry') }}
          </button>
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
        <section class="card overflow-hidden">
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
            <article
              v-if="!claimBatches.length"
              class="rounded-2xl border border-dashed border-slate-300 p-4 text-sm text-slate-500 dark:border-white/20 dark:text-slate-300"
            >
              {{ t('gameCenter.claim.empty') }}
            </article>
          </div>
        </section>

        <section v-if="exchangeOpen" class="card overflow-hidden">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.exchange.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.exchange.subtitle') }}</p>
          </div>
          <div class="grid gap-4 px-4 py-4 md:grid-cols-2">
            <article class="rounded-2xl border border-slate-200/80 p-4 dark:border-white/10">
              <p class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.exchange.balanceToPoints') }}</p>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ t('gameCenter.exchange.rate', { rate: overview?.exchange.balance_to_points_rate ?? 0 }) }}</p>
              <label class="mt-3 block text-xs font-medium uppercase tracking-[0.16em] text-slate-500 dark:text-slate-300">{{ t('gameCenter.exchange.amount') }}</label>
              <input v-model.number="balanceAmount" type="number" min="0" step="0.01" class="input mt-2 w-full" />
              <button
                type="button"
                class="btn btn-primary mt-4 w-full justify-center"
                :disabled="!canBalanceToPoints || exchangeLoadingDirection === 'balance_to_points'"
                @click="handleBalanceToPoints"
              >
                {{
                  exchangeLoadingDirection === 'balance_to_points'
                    ? t('gameCenter.exchange.submitting')
                    : t('gameCenter.exchange.submit')
                }}
              </button>
            </article>

            <article class="rounded-2xl border border-slate-200/80 p-4 dark:border-white/10">
              <p class="text-sm font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.exchange.pointsToBalance') }}</p>
              <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ t('gameCenter.exchange.rate', { rate: overview?.exchange.points_to_balance_rate ?? 0 }) }}</p>
              <label class="mt-3 block text-xs font-medium uppercase tracking-[0.16em] text-slate-500 dark:text-slate-300">{{ t('gameCenter.exchange.points') }}</label>
              <input v-model.number="pointsAmount" type="number" min="0" step="1" class="input mt-2 w-full" />
              <button
                type="button"
                class="btn btn-primary mt-4 w-full justify-center"
                :disabled="!canPointsToBalance || exchangeLoadingDirection === 'points_to_balance'"
                @click="handlePointsToBalance"
              >
                {{
                  exchangeLoadingDirection === 'points_to_balance'
                    ? t('gameCenter.exchange.submitting')
                    : t('gameCenter.exchange.submit')
                }}
              </button>
            </article>
          </div>
        </section>

        <section class="card overflow-hidden">
          <div class="flex flex-col gap-3 border-b border-slate-200/80 px-6 py-5 dark:border-white/10 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.leaderboard.title') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.leaderboard.subtitle') }}</p>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="leaderboardLoading" @click="() => loadPointsLeaderboard()">
              {{ t('common.refresh') }}
            </button>
          </div>
          <div class="grid gap-4 px-4 py-4 xl:grid-cols-[1.1fr_0.9fr]">
            <div class="overflow-hidden rounded-2xl border border-slate-200/80 dark:border-white/10">
              <div class="max-h-96 overflow-auto">
                <button
                  v-for="row in pointsLeaderboard.items"
                  :key="row.user_id"
                  type="button"
                  class="grid w-full grid-cols-[3rem_1fr_auto] items-center gap-3 border-b border-slate-100 px-4 py-3 text-left text-sm last:border-b-0 hover:bg-slate-50 dark:border-white/10 dark:hover:bg-white/5"
                  :disabled="!canOpenLeaderboardDetail(row.user_id)"
                  @click="openLeaderboardDetail(row)"
                >
                  <span class="text-xs font-semibold text-slate-500">#{{ row.rank }}</span>
                  <span>
                    <span class="block font-medium text-slate-900 dark:text-white">{{ row.username || row.email }}</span>
                    <span class="block text-xs text-slate-500 dark:text-slate-400">{{ row.email }}</span>
                  </span>
                  <span class="font-semibold text-cyan-700 dark:text-cyan-300">{{ formatPoints(row.points) }}</span>
                </button>
                <p v-if="!pointsLeaderboard.items.length" class="px-4 py-6 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.leaderboard.empty') }}</p>
              </div>
              <Pagination :total="pointsLeaderboard.total" :page="pointsLeaderboard.page" :page-size="pointsLeaderboard.page_size" :show-page-size-selector="false" @update:page="changeLeaderboardPage" @update:pageSize="noopPageSize" />
            </div>
            <div class="rounded-2xl border border-slate-200/80 p-4 dark:border-white/10">
              <div class="flex items-center justify-between gap-3">
                <h3 class="text-sm font-semibold text-slate-900 dark:text-white">{{ leaderboardDetailTitle }}</h3>
                <button v-if="selectedLeaderboardUserID" type="button" class="btn btn-secondary btn-sm" @click="closeLeaderboardDetail">{{ t('common.close') }}</button>
              </div>
              <div class="mt-3 max-h-80 space-y-2 overflow-auto">
                <div v-for="item in leaderboardLedger.items" :key="item.id" class="rounded-xl bg-slate-50 px-3 py-3 text-sm dark:bg-white/5">
                  <div class="flex items-center justify-between gap-3">
                    <span class="font-medium text-slate-900 dark:text-white">{{ ledgerTypeLabel(item.entry_type) }}</span>
                    <span class="font-semibold" :class="item.delta_points >= 0 ? 'text-emerald-600' : 'text-rose-600'">{{ formatSignedPoints(item.delta_points) }}</span>
                  </div>
                  <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ formatDateTime(item.created_at) }} · {{ item.reason || '--' }}</p>
                </div>
                <p v-if="!leaderboardLedger.items.length" class="text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.leaderboard.detailHint') }}</p>
              </div>
              <Pagination
                v-if="selectedLeaderboardUserID"
                :total="leaderboardLedger.total"
                :page="leaderboardLedger.page"
                :page-size="leaderboardLedger.page_size"
                :show-page-size-selector="false"
                @update:page="changeLeaderboardLedgerPage"
                @update:pageSize="noopPageSize"
              />
            </div>
          </div>
        </section>

        <section class="card overflow-hidden">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{{ t('gameCenter.catalog.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.catalog.subtitle') }}</p>
          </div>
          <div class="grid gap-4 px-4 py-4 md:grid-cols-2">
            <article
              v-for="game in catalogs"
              :key="game.game_key"
              class="grid gap-4 rounded-2xl border border-slate-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/5 lg:grid-cols-[1fr_0.9fr]"
            >
              <div class="rounded-xl bg-slate-50 p-3 dark:bg-white/5">
                <p class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">{{ t('gameCenter.catalog.gameTop10') }}</p>
                <div class="mt-3 max-h-64 space-y-2 overflow-auto">
                  <div v-for="row in gameLeaderboard(game.game_key)" :key="row.user_id" class="flex items-center justify-between gap-3 rounded-lg bg-white px-3 py-2 text-sm dark:bg-white/5">
                    <span class="truncate text-slate-700 dark:text-slate-200">#{{ row.rank }} {{ row.email || row.username }}</span>
                    <span class="shrink-0 font-medium text-cyan-700 dark:text-cyan-300">{{ t('gameCenter.catalog.totalRemainingPoints', { points: formatPoints(row.points) }) }}</span>
                  </div>
                  <p v-if="!gameLeaderboard(game.game_key).length" class="text-sm text-slate-500 dark:text-slate-400">{{ t('gameCenter.leaderboard.empty') }}</p>
                </div>
              </div>
              <div class="flex flex-col justify-between gap-4">
                <div class="space-y-2">
                  <h3 class="text-lg font-semibold text-slate-900 dark:text-white">{{ game.name }}</h3>
                  <p class="text-sm text-slate-500 dark:text-slate-300">{{ game.subtitle || t('gameCenter.catalog.noSubtitle') }}</p>
                  <p class="text-sm leading-6 text-slate-500 dark:text-slate-400">{{ game.description }}</p>
                </div>
                <div class="flex flex-col gap-2 sm:flex-row lg:flex-col">
                  <button
                    type="button"
                    class="btn btn-primary flex-1 justify-center"
                    :data-test="`quick-start-${game.game_key}`"
                    @click="openQuickStart(game.game_key)"
                  >
                    {{ t('gameCenter.launch.quick') }}
                  </button>
                  <RouterLink class="btn btn-secondary flex-1 justify-center" :to="`/game-center/${game.game_key}`">
                    {{ t('gameCenter.launch.fullscreen') }}
                  </RouterLink>
                </div>
              </div>
            </article>
            <article
              v-if="!catalogs.length"
              class="rounded-2xl border border-dashed border-slate-300 p-4 text-sm text-slate-500 dark:border-white/20 dark:text-slate-300"
            >
              {{ t('gameCenter.catalog.empty') }}
            </article>
          </div>
        </section>

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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { gameCenterAPI } from '@/api/gameCenter'
import { sizeBetAPI } from '@/api/sizeBet'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { BasePaginationResponse } from '@/types'
import type { GameCenterClaimStatus, GameCenterLedgerItem, GameCenterOverview, GameCenterPointsLeaderboardItem } from '@/types/gameCenter'
import type { SizeBetLeaderboardItem } from '@/types/sizeBet'

type LoadState = 'loading' | 'ready' | 'error'
type ExchangeDirection = 'balance_to_points' | 'points_to_balance'

const GAME_ROUTE_MAP: Record<string, string> = {
  size_bet: '/game/size-bet',
}

const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loadState = ref<LoadState>('loading')
const overview = ref<GameCenterOverview | null>(null)
const claimLoadingBatchKey = ref('')
const exchangeLoadingDirection = ref<ExchangeDirection | ''>('')
const exchangeOpen = ref(false)
const balanceAmount = ref(1)
const pointsAmount = ref(100)
const today = toDateInput(new Date())
const ledgerStartDate = ref(today)
const ledgerEndDate = ref(today)
const ledgerLoading = ref(false)
const leaderboardLoading = ref(false)
const selectedLeaderboardUserID = ref<number | null>(null)
const selectedLeaderboardUserName = ref('')
const ledgerView = ref<BasePaginationResponse<GameCenterLedgerItem>>({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
const pointsLeaderboard = ref<BasePaginationResponse<GameCenterPointsLeaderboardItem>>({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
const leaderboardLedger = ref<BasePaginationResponse<GameCenterLedgerItem>>({ items: [], total: 0, page: 1, page_size: 10, pages: 1 })
const gameLeaderboards = ref<Record<string, SizeBetLeaderboardItem[]>>({})

const gameCenterEnabled = computed(() => appStore.gameCenterEnabled)
const claimBatches = computed(() => overview.value?.claim_batches ?? [])
const catalogs = computed(() => overview.value?.catalogs ?? [])
const leaderboardDetailTitle = computed(() => selectedLeaderboardUserID.value ? t('gameCenter.leaderboard.detailTitle', { user: selectedLeaderboardUserName.value }) : t('gameCenter.leaderboard.detailPlaceholder'))

const canBalanceToPoints = computed(() => {
  if (!overview.value?.exchange.balance_to_points_enabled) return false
  return Number(balanceAmount.value) > 0
})

const canPointsToBalance = computed(() => {
  if (!overview.value?.exchange.points_to_balance_enabled) return false
  return Number(pointsAmount.value) > 0
})

function resolveGamePath(gameKey: string): string | null {
  return GAME_ROUTE_MAP[gameKey] ?? null
}

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

function normalizeGameLeaderboard(items: SizeBetLeaderboardItem[]): SizeBetLeaderboardItem[] {
  return [...items]
    .sort((left, right) =>
      right.points - left.points
      || right.net_profit - left.net_profit
      || right.win_count - left.win_count
      || left.user_id - right.user_id)
    .map((item, index) => ({
      ...item,
      rank: index + 1,
    }))
}

function ledgerTypeLabel(type: string): string {
  return t(`gameCenter.ledger.types.${type}`)
}

function gameLeaderboard(gameKey: string): SizeBetLeaderboardItem[] {
  return (gameLeaderboards.value[gameKey] ?? []).slice(0, 10)
}

function canOpenLeaderboardDetail(userID: number): boolean {
  return authStore.user?.role === 'admin' || authStore.user?.id === userID
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

async function loadOverview(): Promise<void> {
  loadState.value = 'loading'
  try {
    overview.value = await gameCenterAPI.getOverview()
    loadState.value = 'ready'
    await Promise.all([loadLedger(), loadPointsLeaderboard(), loadGameLeaderboards()])
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

async function loadPointsLeaderboard(page = pointsLeaderboard.value.page): Promise<void> {
  leaderboardLoading.value = true
  try {
    pointsLeaderboard.value = await gameCenterAPI.getPointsLeaderboard(page, pointsLeaderboard.value.page_size)
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.loadError.description'))
  } finally {
    leaderboardLoading.value = false
  }
}

async function loadGameLeaderboards(): Promise<void> {
  if (!catalogs.value.some((item) => item.game_key === 'size_bet')) return
  try {
    const view = await sizeBetAPI.getLeaderboard('all')
    gameLeaderboards.value = {
      ...gameLeaderboards.value,
      size_bet: normalizeGameLeaderboard(view.items ?? []),
    }
  } catch {
    gameLeaderboards.value = { ...gameLeaderboards.value, size_bet: [] }
  }
}

async function loadLeaderboardLedger(userID: number, page = leaderboardLedger.value.page): Promise<void> {
  try {
    leaderboardLedger.value = await gameCenterAPI.getUserLedger(userID, { page, page_size: leaderboardLedger.value.page_size })
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.loadError.description'))
  }
}

async function openLeaderboardDetail(row: GameCenterPointsLeaderboardItem): Promise<void> {
  if (!canOpenLeaderboardDetail(row.user_id)) {
    appStore.showWarning(t('gameCenter.leaderboard.ownOnly'))
    return
  }
  selectedLeaderboardUserID.value = row.user_id
  selectedLeaderboardUserName.value = row.username || row.email
  await loadLeaderboardLedger(row.user_id, 1)
}

function closeLeaderboardDetail(): void {
  selectedLeaderboardUserID.value = null
  selectedLeaderboardUserName.value = ''
  leaderboardLedger.value = { items: [], total: 0, page: 1, page_size: 10, pages: 1 }
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

function changeLeaderboardPage(page: number): void {
  void loadPointsLeaderboard(page)
}

function changeLeaderboardLedgerPage(page: number): void {
  if (!selectedLeaderboardUserID.value) return
  void loadLeaderboardLedger(selectedLeaderboardUserID.value, page)
}

function noopPageSize(): void {}

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
    await loadOverview()
  } catch (error) {
    appStore.showError(normalizeError(error, 'gameCenter.exchange.failed'))
  } finally {
    exchangeLoadingDirection.value = ''
  }
}

async function openQuickStart(gameKey: string): Promise<void> {
  const gamePath = resolveGamePath(gameKey)
  if (!gamePath) {
    appStore.showError(t('gameCenter.catalog.unsupportedGame'))
    return
  }
  await router.push({ path: gamePath, query: { from: 'game-center' } })
}

onMounted(() => {
  loadOverview()
})
</script>
