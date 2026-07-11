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
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.ticketAmount') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatAmount(status.ticket_amount) }}</p>
          </div>
          <div class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.remainingPlays') }}</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ status.remaining_plays }} / {{ status.daily_play_limit }}</p>
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
              <p v-else class="mt-5 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.playHint', { amount: formatNumber(status.ticket_amount) }) }}</p>

              <button
                data-testid="zenxiang-play"
                type="button"
                class="btn btn-primary mt-4 min-w-36"
                :disabled="!canPlay"
                @click="play"
              >
                {{ isPlaying ? t('zenxiangLiyu.opening') : t('zenxiangLiyu.open') }}
              </button>

              <p v-if="playError" class="mt-3 text-center text-sm text-red-600 dark:text-red-400">{{ playError }}</p>
            </div>
          </div>

          <aside class="card overflow-hidden">
            <div class="border-b border-gray-200 px-5 py-4 dark:border-gray-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('zenxiangLiyu.configuredRewards') }}</h2>
            </div>
            <ul v-if="status.prizes.length" class="divide-y divide-gray-100 dark:divide-gray-700">
              <li v-for="prize in status.prizes" :key="prize.id" class="flex items-center justify-between gap-3 px-5 py-3">
                <span class="min-w-0 truncate text-sm text-gray-700 dark:text-gray-200" :title="prize.name">{{ prize.name }}</span>
                <span class="shrink-0 text-xs font-medium text-gray-700 dark:text-gray-200">{{ formatAmount(prize.reward_amount) }}</span>
              </li>
            </ul>
            <p v-else class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.noRewards') }}</p>
          </aside>
        </section>

        <section v-if="result" class="rounded-lg border border-emerald-200 bg-emerald-50 p-5 dark:border-emerald-900/70 dark:bg-emerald-950/30" aria-live="polite">
          <div class="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <p class="text-sm font-medium text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.rewardResult', { amount: formatNumber(result.reward_amount) }) }}</p>
              <h2 class="mt-1 text-xl font-semibold text-emerald-950 dark:text-white">{{ result.prize_name }}</h2>
            </div>
            <p class="text-sm text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.latestBalance', { amount: formatNumber(result.balance_after_reward) }) }}</p>
          </div>
          <div class="mt-4 grid grid-cols-1 gap-3 border-t border-emerald-200 pt-4 text-sm sm:grid-cols-2 dark:border-emerald-900/70">
            <p class="text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.rewardAmount', { amount: formatNumber(result.reward_amount) }) }}</p>
            <p class="text-emerald-800 dark:text-emerald-200">{{ t('zenxiangLiyu.netAmount', { amount: formatNumber(result.user_net_amount) }) }}</p>
          </div>
        </section>

        <section class="card overflow-hidden">
          <div class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-gray-700">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('zenxiangLiyu.todayRecords') }}</h2>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ todayRecords.length }}</span>
          </div>
          <div v-if="todayRecords.length" class="divide-y divide-gray-100 dark:divide-gray-700">
            <div v-for="record in todayRecords" :key="record.id" class="grid gap-2 px-5 py-3 text-sm sm:grid-cols-[minmax(0,1fr)_auto_auto_auto] sm:items-center">
              <div class="min-w-0">
                <p class="truncate font-medium text-gray-900 dark:text-white">{{ record.prize_name }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ formatTime(record.played_at) }}</p>
              </div>
              <span class="text-gray-600 dark:text-gray-300">{{ t('zenxiangLiyu.rewardAmount', { amount: formatNumber(record.reward_amount) }) }}</span>
              <span class="text-gray-600 dark:text-gray-300">{{ t('zenxiangLiyu.ticketCost', { amount: formatNumber(record.ticket_amount) }) }}</span>
              <span :class="record.user_net_amount >= 0 ? 'text-emerald-600 dark:text-emerald-300' : 'text-red-600 dark:text-red-300'">{{ t('zenxiangLiyu.netAmount', { amount: formatNumber(record.user_net_amount) }) }}</span>
            </div>
          </div>
          <p v-else class="px-5 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.noTodayRecords') }}</p>
        </section>

        <div v-if="showResultDialog && result" class="fixed inset-0 z-50 flex items-center justify-center bg-gray-950/45 px-4 py-6" role="dialog" aria-modal="true">
          <div class="w-full max-w-sm rounded-xl border border-emerald-200 bg-white p-6 text-center shadow-2xl dark:border-emerald-900 dark:bg-gray-900">
            <p class="text-sm font-medium text-emerald-600 dark:text-emerald-300">{{ t('zenxiangLiyu.resultDialogEyebrow') }}</p>
            <h2 class="mt-2 text-2xl font-semibold text-gray-950 dark:text-white">{{ result.prize_name }}</h2>
            <p class="mt-3 text-4xl font-bold text-emerald-600 dark:text-emerald-300">+{{ formatAmount(result.reward_amount) }}</p>
            <div class="mt-5 grid grid-cols-2 gap-2 rounded-lg bg-gray-50 p-3 text-sm dark:bg-gray-800">
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.ticketAmount') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ formatAmount(result.ticket_amount) }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.netLabel') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ formatAmount(result.user_net_amount) }}</span>
              <span class="text-gray-500 dark:text-gray-400">{{ t('zenxiangLiyu.latestPoints') }}</span>
              <span class="text-right text-gray-900 dark:text-white">{{ formatAmount(result.balance_after_reward) }}</span>
            </div>
            <button type="button" class="btn btn-primary mt-6 w-full" @click="showResultDialog = false">{{ t('common.confirm') }}</button>
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
import { listZenxiangLiyuRecords, playZenxiangLiyu, type ZenxiangLiyuPlayResult, type ZenxiangLiyuRecord } from '@/api/zenxiangLiyu'
import { useAuthStore, useZenxiangLiyuStore } from '@/stores'

const { t } = useI18n()
const authStore = useAuthStore()
const zenxiangLiyuStore = useZenxiangLiyuStore()
const status = computed(() => zenxiangLiyuStore.status)
const statusLoading = computed(() => zenxiangLiyuStore.loading)
const currentBalance = computed(() => status.value?.balance ?? authStore.user?.balance ?? 0)
const isPlaying = ref(false)
const loadError = ref('')
const statusRefreshError = ref('')
const playError = ref('')
const result = ref<ZenxiangLiyuPlayResult | null>(null)
const todayRecords = ref<ZenxiangLiyuRecord[]>([])
const wheelRotation = ref(0)
const isSpinning = ref(false)
const showResultDialog = ref(false)

const unavailableReason = computed(() => {
  if (!status.value || (status.value.visible && status.value.can_play)) return ''

  switch (status.value.reason) {
    case 'insufficient_balance':
      return t('zenxiangLiyu.insufficientBalance', { amount: formatNumber(status.value.minimum_balance) })
    case 'daily_limit_reached':
    case 'daily_play_limit_reached':
    case 'zenxiang liyu daily limit reached':
      return t('zenxiangLiyu.dailyLimitReached')
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

function errorMessage(error: unknown): string {
  const responseMessage = (error as { response?: { data?: { message?: unknown } } })?.response?.data?.message
  return typeof responseMessage === 'string' && responseMessage.trim() ? responseMessage : t('zenxiangLiyu.playFailed')
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
    todayRecords.value = response.items
  } catch {
    todayRecords.value = []
  }
}

async function refreshStatus(): Promise<void> {
  await loadStatus(false)
  if (status.value?.visible) {
    await loadTodayRecords()
  }
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

async function spinToPrize(prizeId: number): Promise<void> {
  const prizes = status.value?.prizes ?? []
  const index = Math.max(0, prizes.findIndex((prize) => prize.id === prizeId))
  const count = Math.max(prizes.length, 1)
  const segment = 360 / count
  const targetCenter = index * segment + segment / 2
  const currentBase = Math.ceil(wheelRotation.value / 360) * 360
  isSpinning.value = true
  wheelRotation.value = currentBase + 360 * 5 - targetCenter
  await wait(3200)
  isSpinning.value = false
}

async function play(): Promise<void> {
  if (!canPlay.value) return

  isPlaying.value = true
  playError.value = ''
  showResultDialog.value = false
  try {
    wheelRotation.value += 360
    const playResult = await playZenxiangLiyu(newRequestId())
    await spinToPrize(playResult.prize_id)
    result.value = playResult
    showResultDialog.value = true
    await loadStatus(true)
    await loadTodayRecords()
  } catch (error) {
    playError.value = errorMessage(error)
    isSpinning.value = false
  } finally {
    isPlaying.value = false
  }
}

onMounted(() => {
  void refreshStatus()
})
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
  transition: transform 3.2s cubic-bezier(0.12, 0.72, 0.12, 1);
  will-change: transform;
}

.zenxiang-wheel--spinning {
  filter: saturate(1.08);
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
</style>
