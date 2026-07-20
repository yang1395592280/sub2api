<template>
  <section :data-group-id="selectedGroupId || undefined" class="min-w-0 space-y-5">
    <div class="grid grid-cols-2 gap-3 xl:grid-cols-4">
      <div
        v-for="metric in metrics"
        :key="metric.key"
        :data-testid="`scheduler-metric-${metric.key}`"
        class="min-h-[92px] rounded-md border border-gray-200 bg-white p-3.5 dark:border-dark-700 dark:bg-dark-900"
      >
        <span class="text-xs text-gray-500 dark:text-dark-400">{{ metric.label }}</span>
        <span class="mt-2 block text-xl font-semibold text-gray-950 dark:text-white">
          <span v-if="loading" class="inline-block h-6 w-20 animate-pulse rounded bg-gray-100 dark:bg-dark-700" />
          <template v-else>{{ metric.value }}</template>
        </span>
        <small class="mt-1 block text-xs text-gray-400 dark:text-dark-500">{{ metric.note }}</small>
      </div>
    </div>

    <section class="border-y border-gray-200 py-3 dark:border-dark-700">
      <div class="mb-2 flex items-center justify-between gap-3">
        <h3 class="text-xs font-semibold text-gray-700 dark:text-dark-200">{{ t('admin.openaiAutoScheduler.overview.runtimeCounters') }}</h3>
        <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.openaiAutoScheduler.overview.currentInstance') }}</span>
      </div>
      <dl class="grid grid-cols-2 gap-x-4 gap-y-2 text-xs md:grid-cols-3 xl:grid-cols-6">
        <div v-for="counter in runtimeCounters" :key="counter.key" class="min-w-0">
          <dt class="truncate text-gray-500 dark:text-dark-400" :title="counter.label">{{ counter.label }}</dt>
          <dd class="mt-0.5 font-semibold tabular-nums text-gray-900 dark:text-white">{{ counter.value }}</dd>
        </div>
      </dl>
    </section>

    <div
      v-if="highestAlert"
      class="flex items-center justify-between gap-4 border-l-4 border-amber-500 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:bg-amber-500/10 dark:text-amber-200"
    >
      <span class="min-w-0 truncate">{{ highestAlert.name }} P90 {{ formatDuration(highestAlert.e2e_ttft_p90_ms) }}</span>
      <button type="button" class="shrink-0 font-medium" @click="emit('show-health-filter', highestAlert.id)">
        {{ t('admin.openaiAutoScheduler.overview.viewAccounts') }}
      </button>
    </div>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(260px,1fr)]">
      <div class="min-w-0 border-t border-gray-200 pt-4 dark:border-dark-700">
        <div class="mb-2 flex items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.overview.trend') }}</h3>
          <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.overview.realRequests') }}</span>
        </div>
        <SchedulerTTFTChart :points="overview?.trend || []" />
      </div>

      <div class="border-t border-gray-200 pt-4 dark:border-dark-700">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.overview.slowCauses') }}</h3>
        <div v-if="overview?.slow_causes.length" class="mt-3 space-y-3">
          <div v-for="cause in overview.slow_causes" :key="cause.reason">
            <div class="flex items-center justify-between gap-3 text-sm">
              <span class="text-gray-700 dark:text-dark-200">{{ slowCauseLabel(cause.reason) }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ formatPercent(cause.ratio) }}</span>
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded bg-gray-100 dark:bg-dark-700">
              <div class="h-full bg-amber-500" :style="{ width: `${Math.min(100, cause.ratio * 100)}%` }" />
            </div>
          </div>
        </div>
        <p v-else class="mt-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.overview.noSlowRequests') }}</p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SchedulerTTFTChart from './SchedulerTTFTChart.vue'
import type { OpenAISchedulerOverview } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  overview: OpenAISchedulerOverview | null
  loading: boolean
  selectedGroupId: number | null
}>()

const emit = defineEmits<{
  (event: 'show-health-filter', groupId: number): void
}>()

const metrics = computed(() => [
  { key: 'p50', label: t('admin.openaiAutoScheduler.overview.e2eP50'), value: formatDuration(props.overview?.e2e_ttft_p50_ms), note: t('admin.openaiAutoScheduler.overview.typicalExperience') },
  { key: 'p90', label: t('admin.openaiAutoScheduler.overview.e2eP90'), value: formatDuration(props.overview?.e2e_ttft_p90_ms), note: t('admin.openaiAutoScheduler.overview.tailExperience') },
  { key: 'selection', label: t('admin.openaiAutoScheduler.overview.selectionP95'), value: formatDuration(props.overview?.selection_p95_ms), note: t('admin.openaiAutoScheduler.overview.routingProxy') },
  { key: 'probe', label: t('admin.openaiAutoScheduler.overview.probeRatio'), value: formatPercent(props.overview?.probe_ratio), note: t('admin.openaiAutoScheduler.overview.probeRatioNote') },
])

const runtimeCounters = computed(() => {
  const runtime = props.overview?.runtime
  return [
    { key: 'exploration-allowed', label: t('admin.openaiAutoScheduler.overview.explorationAllowed'), value: formatCount(runtime?.exploration_allowed_total) },
    { key: 'exploration-interval', label: t('admin.openaiAutoScheduler.overview.explorationInterval'), value: formatCount(runtime?.exploration_interval_total) },
    { key: 'exploration-hourly', label: t('admin.openaiAutoScheduler.overview.explorationHourly'), value: formatCount(runtime?.exploration_hourly_total) },
    { key: 'exploration-errors', label: t('admin.openaiAutoScheduler.overview.explorationErrors'), value: formatCount(runtime?.exploration_error_total) },
    { key: 'low-confidence-fallback', label: t('admin.openaiAutoScheduler.overview.lowConfidenceFallback'), value: formatCount(runtime?.low_confidence_fallback_total) },
    { key: 'health-fallback', label: t('admin.openaiAutoScheduler.overview.healthFallback'), value: formatCount(runtime?.unified_health_fallbacks_total) }
  ]
})

const highestAlert = computed(() => {
  const priority = { critical: 3, warning: 2, ok: 1, disabled: 0 }
  return [...(props.overview?.groups || [])]
    .filter((group) => group.alert_level === 'critical' || group.alert_level === 'warning')
    .sort((a, b) => priority[b.alert_level] - priority[a.alert_level])[0]
})

function formatDuration(value?: number | null): string {
  if (value == null) return '—'
  if (value >= 1000) return `${(value / 1000).toFixed(value >= 10_000 ? 1 : 2)}s`
  return `${Math.round(value)}ms`
}

function formatPercent(value?: number | null): string {
  if (value == null) return '—'
  return `${Math.round(value * 100)}%`
}

function formatCount(value?: number | null): string {
  return value == null ? '—' : new Intl.NumberFormat().format(value)
}

function slowCauseLabel(reason: string): string {
  return { upstream_ttft: t('admin.openaiAutoScheduler.overview.causeUpstream'), queue: t('admin.openaiAutoScheduler.overview.causeQueue'), retry: t('admin.openaiAutoScheduler.overview.causeRetry') }[reason] || reason
}
</script>
