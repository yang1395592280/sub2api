<template>
  <component :is="props.embedded ? 'div' : AppLayout">
    <div class="space-y-6">
      <div v-if="showGameCenterBack" class="flex flex-wrap items-center justify-between gap-3">
        <RouterLink to="/game-center" class="btn btn-secondary">{{ t('luckyWheel.backToGameCenter') }}</RouterLink>
        <div class="rounded-full bg-emerald-50 px-4 py-2 text-sm font-semibold text-emerald-700 ring-1 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-200 dark:ring-emerald-500/20">
          {{ t('luckyWheel.pointsBalance', { points: formatPoints(overview?.points ?? 0) }) }}
        </div>
      </div>

      <section class="overflow-hidden rounded-[28px] border border-emerald-200/70 bg-[radial-gradient(circle_at_top_left,_#fef3c7,_#dcfce7_35%,_#ecfccb_75%,_#f8fafc)] p-6 shadow-sm shadow-emerald-100/60 dark:border-emerald-500/20 dark:bg-[radial-gradient(circle_at_top_left,_#1f2937,_#064e3b_35%,_#0f172a_75%,_#020617)] dark:shadow-none">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="space-y-2">
            <div class="inline-flex items-center rounded-full bg-white/80 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-emerald-700 ring-1 ring-emerald-100 dark:bg-white/10 dark:text-emerald-200 dark:ring-white/10">{{ t('luckyWheel.badge') }}</div>
            <h1 class="text-3xl font-semibold tracking-tight text-slate-900 dark:text-white">{{ t('luckyWheel.title') }}</h1>
            <p class="text-sm text-slate-600 dark:text-slate-300">{{ t('luckyWheel.heroSubtitle') }}</p>
          </div>
          <div class="grid gap-3 sm:grid-cols-3">
            <article class="group rounded-2xl bg-white/60 p-4 ring-1 ring-white shadow-sm backdrop-blur-md transition-all hover:bg-white/80 dark:bg-white/5 dark:ring-white/10 dark:hover:bg-white/10">
              <p class="text-[10px] font-bold uppercase tracking-widest text-slate-400 group-hover:text-slate-500 dark:text-slate-500 dark:group-hover:text-slate-400">{{ t('luckyWheel.dailyLimit') }}</p>
              <p class="mt-2 text-2xl font-black tracking-tight text-slate-900 dark:text-white">{{ overview?.daily_spin_limit ?? '--' }}</p>
            </article>
            <article class="group rounded-2xl bg-white/60 p-4 ring-1 ring-white shadow-sm backdrop-blur-md transition-all hover:bg-white/80 dark:bg-white/5 dark:ring-white/10 dark:hover:bg-white/10">
              <p class="text-[10px] font-bold uppercase tracking-widest text-slate-400 group-hover:text-slate-500 dark:text-slate-500 dark:group-hover:text-slate-400">{{ t('luckyWheel.usedToday') }}</p>
              <p class="mt-2 text-2xl font-black tracking-tight text-slate-900 dark:text-white">{{ overview?.spins_used_today ?? '--' }}</p>
            </article>
            <article class="group relative overflow-hidden rounded-2xl bg-emerald-500 p-4 shadow-lg shadow-emerald-500/20 transition-all hover:scale-[1.02] hover:shadow-emerald-500/30">
              <p class="relative z-10 text-[10px] font-bold uppercase tracking-widest text-emerald-100/80">{{ t('luckyWheel.remainingToday') }}</p>
              <p class="relative z-10 mt-2 text-2xl font-black tracking-tight text-white">{{ overview?.spins_remaining_today ?? '--' }}</p>
              <div class="absolute -right-4 -top-4 h-20 w-20 rounded-full bg-white/10 blur-2xl"></div>
            </article>
          </div>
        </div>
      </section>

      <div v-if="loadState === 'loading'" class="flex justify-center py-16"><LoadingSpinner /></div>
      <section v-else-if="loadState === 'error'" class="card px-6 py-12">
        <EmptyState :title="t('luckyWheel.loadError.title')" :description="loadErrorMessage || t('luckyWheel.loadError.description')" :action-text="t('common.retry')" @action="() => loadOverview()" />
      </section>
      <section v-else-if="!overview?.enabled" class="card px-6 py-12">
        <EmptyState :title="t('luckyWheel.maintenance.title')" :description="t('luckyWheel.maintenance.description')" />
      </section>

      <div v-else class="space-y-6">
        <div class="grid gap-8 lg:grid-cols-[1fr_1.2fr_1fr]">
          <!-- Left Column: Leaderboard -->
          <section class="card flex flex-col overflow-hidden border-none bg-white/50 backdrop-blur-sm dark:bg-slate-900/50">
            <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
              <h2 class="text-xl font-bold tracking-tight text-slate-900 dark:text-white">{{ t('luckyWheel.leaderboard.title') }}</h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('luckyWheel.leaderboard.subtitle') }}</p>
            </div>
            <div class="flex-1 space-y-3 overflow-y-auto px-4 py-4" style="max-height: 500px;">
              <div v-for="row in overview.leaderboard" :key="row.user_id" class="group relative overflow-hidden rounded-2xl bg-white p-4 shadow-sm transition-all hover:shadow-md dark:bg-white/5">
                <div class="relative z-10 flex items-center justify-between gap-3">
                  <div class="flex items-center gap-3">
                    <span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-bold text-slate-600 dark:bg-white/10 dark:text-slate-300" :class="{ '!bg-amber-100 !text-amber-700 dark:!bg-amber-500/20 dark:!text-amber-300': row.rank === 1 }">
                      {{ row.rank }}
                    </span>
                    <span class="truncate text-sm font-semibold text-slate-900 dark:text-white">{{ row.username || row.email }}</span>
                  </div>
                  <span class="shrink-0 text-sm font-bold text-emerald-600">{{ formatSigned(row.net_points) }}</span>
                </div>
                <p class="mt-2 text-xs text-slate-500 dark:text-slate-400">{{ t('luckyWheel.leaderboard.meta', { count: row.spin_count, prize: row.best_prize_label || '--' }) }}</p>
              </div>
              <p v-if="!overview.leaderboard.length" class="py-12 text-center text-sm text-slate-500 dark:text-slate-400">{{ t('luckyWheel.leaderboard.empty') }}</p>
            </div>
          </section>

          <!-- Middle Column: The Wheel -->
          <section class="relative flex flex-col items-center justify-center space-y-10 py-4">
            <div class="relative flex items-center justify-center">
              <!-- Simple Outer Frame -->
              <div class="absolute -inset-4 rounded-full border-[8px] border-amber-400 bg-white shadow-xl"></div>

              <!-- Pointer (Classic Top Style) -->
              <div class="absolute -top-6 z-40 h-10 w-8 bg-rose-600 shadow-md [clip-path:polygon(50%_100%,_0_0,_100%_0)]"></div>

              <!-- Main Wheel Container -->
              <div class="relative h-[26rem] w-[26rem] overflow-hidden rounded-full border-4 border-white shadow-inner">
                <div class="relative h-full w-full transition-transform duration-[4200ms] [transition-timing-function:cubic-bezier(0.14,0.82,0.2,1)]" :style="{ background: wheelGradient, transform: `rotate(${rotation}deg)` }">
                  <!-- Segment Borders -->
                  <div v-for="segment in segments" :key="'border-' + segment.key" class="absolute left-1/2 top-1/2 h-[50%] w-[1px] origin-bottom bg-white/30" :style="{ transform: `translate(-50%, -100%) rotate(${segment.center_deg - segment.span_deg/2}deg)` }"></div>
                  
                  <!-- Labels -->
                  <div
                    v-for="segment in segments"
                    :key="segment.key"
                    class="absolute left-1/2 top-0 flex h-1/2 w-24 origin-bottom -translate-x-1/2 items-center justify-center pt-10 text-center"
                    :style="{ transform: `translateX(-50%) rotate(${segment.center_deg}deg)` }"
                  >
                    <div class="flex flex-col items-center justify-center">
                      <span class="text-lg font-black leading-tight text-white [text-shadow:0_2px_4px_rgba(0,0,0,0.3)]">
                        {{ wheelLabelText(segment) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Center Spin Button -->
              <button 
                type="button" 
                class="absolute z-30 flex h-28 w-28 items-center justify-center rounded-full border-4 border-white bg-amber-500 text-center text-xl font-black text-white shadow-lg transition-transform hover:scale-105 active:scale-95 disabled:cursor-not-allowed disabled:opacity-80" 
                :disabled="!canSpin" 
                @click="handleSpin"
              >
                {{ spinning ? '...' : '抽奖' }}
              </button>
            </div>

            <div class="w-full max-w-sm rounded-3xl bg-white p-6 text-center shadow-md ring-1 ring-slate-200">
              <p class="text-sm font-bold text-slate-800">{{ t('luckyWheel.currentStatus') }}</p>
              <p class="mt-2 text-sm text-slate-500">{{ spinHint }}</p>
            </div>
          </section>

          <!-- Right Column: Prize Pool & Rules -->
          <div class="flex flex-col gap-6">
            <section class="card flex-1 overflow-hidden border-none bg-white/50 backdrop-blur-sm dark:bg-slate-900/50">
              <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
                <h2 class="text-xl font-bold tracking-tight text-slate-900 dark:text-white">{{ t('luckyWheel.prizePool.title') }}</h2>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('luckyWheel.prizePool.subtitle') }}</p>
              </div>
              <div class="space-y-2.5 px-4 py-4 overflow-y-auto" style="max-height: 350px;">
                <div v-for="prize in overview.prizes" :key="prize.key" class="flex items-center justify-between gap-3 rounded-2xl bg-white px-4 py-3.5 shadow-sm ring-1 ring-slate-100 transition-all hover:ring-emerald-200 dark:bg-white/5 dark:ring-transparent dark:hover:bg-white/10">
                  <span class="text-sm font-semibold text-slate-900 dark:text-white">{{ prize.label }}</span>
                  <span class="rounded-lg bg-slate-50 px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:bg-white/10 dark:text-slate-400">
                    {{ prize.probability }}%
                  </span>
                </div>
              </div>
            </section>

            <section class="card overflow-hidden border-none bg-white/50 backdrop-blur-sm dark:bg-slate-900/50">
              <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
                <h2 class="text-xl font-bold tracking-tight text-slate-900 dark:text-white">{{ t('luckyWheel.rulesTitle') }}</h2>
              </div>
              <div class="markdown-body prose prose-sm max-w-none px-6 py-5 dark:prose-invert" v-html="rulesHtml"></div>
            </section>
          </div>
        </div>

        <section class="card overflow-hidden border-none bg-white/50 backdrop-blur-sm dark:bg-slate-900/50">
          <div class="border-b border-slate-200/80 px-6 py-5 dark:border-white/10">
            <h2 class="text-xl font-bold tracking-tight text-slate-900 dark:text-white">{{ t('luckyWheel.history.title') }}</h2>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ t('luckyWheel.history.subtitle') }}</p>
          </div>
          <div class="grid gap-4 px-6 py-6 md:grid-cols-2 lg:grid-cols-3">
            <div v-for="item in overview.recent_history" :key="item.id" class="relative overflow-hidden rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-100 dark:bg-white/5 dark:ring-transparent">
              <div class="flex items-center justify-between gap-3">
                <p class="text-sm font-bold text-slate-900 dark:text-white">{{ item.prize_label }}</p>
                <p class="text-sm font-black" :class="item.delta_points >= 0 ? 'text-emerald-600' : 'text-rose-600'">{{ formatSigned(item.delta_points) }}</p>
              </div>
              <div class="mt-3 flex items-center justify-between">
                <p class="text-[10px] font-medium uppercase tracking-wider text-slate-400">{{ item.created_at ? formatDateTime(item.created_at) : '--' }}</p>
                <p class="text-[10px] font-bold text-slate-500">{{ formatPoints(item.points_after) }} PTS</p>
              </div>
            </div>
            <p v-if="!overview.recent_history.length" class="col-span-full py-8 text-center text-sm text-slate-500 dark:text-slate-400">{{ t('luckyWheel.history.empty') }}</p>
          </div>
        </section>
      </div>

      <!-- Celebration Modal -->
      <Transition name="fade">
        <div v-if="resultOpen && latestResult" class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4 backdrop-blur-sm">
          <div class="relative w-full max-w-lg overflow-hidden rounded-[40px] bg-white p-10 text-center shadow-2xl dark:bg-slate-900">
            <!-- Decorative background for modal -->
            <div class="absolute -right-20 -top-20 h-64 w-64 rounded-full bg-emerald-500/10 blur-3xl"></div>
            <div class="absolute -bottom-20 -left-20 h-64 w-64 rounded-full bg-amber-500/10 blur-3xl"></div>
            
            <div class="relative z-10">
              <div class="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-3xl bg-emerald-100 text-4xl dark:bg-emerald-500/20">
                {{ latestResult.delta_points >= 0 ? '🎉' : '😅' }}
              </div>
              <p class="text-xs font-black uppercase tracking-[0.3em] text-emerald-600">{{ t('luckyWheel.result.title') }}</p>
              <h3 class="mt-4 text-4xl font-black tracking-tight text-slate-900 dark:text-white">{{ latestResult.prize_label }}</h3>
              
              <div class="mt-8 rounded-3xl bg-slate-50 py-8 dark:bg-white/5">
                <p class="text-5xl font-black tracking-tighter" :class="latestResult.delta_points >= 0 ? 'text-emerald-600' : 'text-rose-600'">
                  {{ formatSigned(latestResult.delta_points) }}
                </p>
                <p class="mt-4 text-xs font-bold uppercase tracking-widest text-slate-400">
                  {{ t('luckyWheel.result.balance', { points: formatPoints(latestResult.points_after) }) }}
                </p>
              </div>

              <button type="button" class="btn btn-primary mt-10 h-14 w-full rounded-2xl text-lg font-bold shadow-lg shadow-emerald-500/20" @click="resultOpen = false">
                {{ t('common.confirm') }}
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </div>
  </component>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@keyframes pulse-fast {
  0%, 100% { opacity: 1; transform: translateX(-50%) scale(1.2); filter: brightness(1.2); }
  50% { opacity: 0.4; transform: translateX(-50%) scale(0.8); filter: brightness(0.8); }
}

.animate-pulse-fast {
  animation: pulse-fast 0.6s infinite;
}
</style>


<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { luckyWheelAPI } from '@/api'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores/app'
import type { LuckyWheelOverview, LuckyWheelSpinRecord } from '@/types/luckyWheel'
import type { LuckyWheelSegment } from '@/utils/luckyWheel'
import { buildLuckyWheelGradient, buildLuckyWheelSegments, computeLuckyWheelRotation } from '@/utils/luckyWheel'

const props = withDefaults(defineProps<{ embedded?: boolean }>(), { embedded: false })
const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const loadState = ref<'loading' | 'ready' | 'error'>('loading')
const loadErrorMessage = ref('')
const overview = ref<LuckyWheelOverview | null>(null)
const spinning = ref(false)
const rotation = ref(0)
const latestResult = ref<LuckyWheelSpinRecord | null>(null)
const resultOpen = ref(false)
const showGameCenterBack = computed(() => !props.embedded && route.query.from === 'game-center')
const segments = computed(() => buildLuckyWheelSegments(overview.value?.prizes ?? []))
const wheelGradient = computed(() => buildLuckyWheelGradient(overview.value?.prizes ?? []))
const needsMorePoints = computed(() => (overview.value?.points ?? 0) < (overview.value?.min_points_required ?? 0))
const canSpin = computed(() => Boolean(overview.value?.enabled) && !spinning.value && (overview.value?.spins_remaining_today ?? 0) > 0 && !needsMorePoints.value)
const spinHint = computed(() => {
  if (!overview.value?.enabled) return t('luckyWheel.maintenance.description')
  if ((overview.value.spins_remaining_today ?? 0) <= 0) return t('luckyWheel.limitReached')
  if (needsMorePoints.value) return t('luckyWheel.minPointsHint', { points: formatPoints(overview.value.min_points_required) })
  return t('luckyWheel.readyHint', { remaining: overview.value.spins_remaining_today })
})
const rulesHtml = computed(() => DOMPurify.sanitize(marked.parse(overview.value?.rules_markdown || '') as string))

onMounted(() => { void loadOverview() })

async function loadOverview(silent = false): Promise<void> {
  if (!silent) loadState.value = 'loading'
  try {
    overview.value = await luckyWheelAPI.getOverview()
    loadState.value = 'ready'
  } catch (error: any) {
    loadState.value = 'error'
    loadErrorMessage.value = normalizeError(error, 'luckyWheel.loadError.description')
    appStore.showError(loadErrorMessage.value)
  }
}

async function handleSpin(): Promise<void> {
  if (!canSpin.value || !overview.value) return
  spinning.value = true
  try {
    const result = await luckyWheelAPI.spin()
    latestResult.value = result.record
    rotation.value += computeLuckyWheelRotation(overview.value.prizes, result.record.prize_key, 7)
    overview.value = {
      ...overview.value,
      points: result.record.points_after,
      spins_used_today: result.spins_used_today,
      spins_remaining_today: result.spins_remaining_today,
      recent_history: [result.record, ...overview.value.recent_history].slice(0, 10),
    }
    window.setTimeout(() => {
      spinning.value = false
      resultOpen.value = true
      void loadOverview(true)
    }, 4200)
  } catch (error: any) {
    spinning.value = false
    appStore.showError(normalizeError(error, 'luckyWheel.spinFailed'))
  }
}

function formatPoints(value: number): string {
  return new Intl.NumberFormat().format(Math.trunc(value))
}

function formatSigned(value: number): string {
  return value >= 0 ? `+${formatPoints(value)}` : `-${formatPoints(Math.abs(value))}`
}

function wheelLabelText(segment: LuckyWheelSegment): string {
  return segment.value_label
}

function formatDateTime(value?: string): string {
  if (!value) return '--'
  return new Date(value).toLocaleString()
}

function normalizeError(error: unknown, fallbackKey: string): string {
  const message = (error as { response?: { data?: { message?: string } }, message?: string })?.response?.data?.message || (error as { message?: string })?.message
  return message || t(fallbackKey)
}
</script>
