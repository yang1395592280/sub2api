<template>
  <section class="border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
    <div :class="statusClass" class="border-b px-4 py-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <span class="text-sm font-semibold">{{ effectiveStatus }}</span>
          <span class="ml-2 text-xs opacity-80">{{ groupName }}</span>
        </div>
        <span class="text-xs">{{ partitionSummary }}</span>
      </div>
      <div v-if="result" class="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs opacity-80">
        <span>{{ t('admin.openaiAutoScheduler.ranking.candidates') }} {{ result.summary.candidate_count }}</span>
        <span>{{ t('admin.openaiAutoScheduler.ranking.eligible') }} {{ result.summary.eligible_count }}</span>
        <span>{{ t('admin.openaiAutoScheduler.ranking.lowConfidence') }} {{ result.summary.low_confidence_count }}</span>
        <span>{{ t('admin.openaiAutoScheduler.ranking.rejected') }} {{ result.summary.rejected_count }}</span>
        <span>{{ t('admin.openaiAutoScheduler.ranking.requests') }} {{ result.summary.request_count }}</span>
        <span>{{ result.policy_context.policy_version }} · {{ formatCalculatedAt(result.policy_context.calculated_at) }}</span>
      </div>
    </div>

    <div class="flex flex-wrap items-center gap-2 border-b border-gray-100 px-3 py-3 dark:border-dark-800">
      <div class="inline-flex rounded-md border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-700 dark:bg-dark-800">
        <button
          v-for="item in windows"
          :key="item"
          type="button"
          class="min-w-11 rounded px-2 py-1.5 text-xs font-medium"
          :class="filters.window === item ? 'bg-white text-gray-950 shadow-sm dark:bg-dark-700 dark:text-white' : 'text-gray-500 dark:text-dark-400'"
          @click="emit('filter', { window: item })"
        >{{ item }}</button>
      </div>
      <input class="input w-36" :value="filters.model_family || ''" :placeholder="t('admin.openaiAutoScheduler.health.model')" @change="emitInput('model_family', $event)" />
      <select class="input w-48" :value="filters.endpoint || ''" @change="emitInput('endpoint', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.health.allEndpoints') }}</option>
        <option value="responses">Responses</option>
        <option value="chat_completions">Chat Completions</option>
        <option value="embeddings">Embeddings</option>
      </select>
      <select class="input w-44" :value="filters.transport || ''" @change="emitInput('transport', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.health.allTransports') }}</option>
        <option value="http_sse">HTTP / SSE</option>
        <option value="responses_websockets_v2">WebSocket V2</option>
      </select>
      <select class="input w-40" :value="filters.eligibility || ''" @change="emitInput('eligibility', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.ranking.allEligibility') }}</option>
        <option value="eligible">{{ eligibilityLabel('eligible') }}</option>
        <option value="low_confidence">{{ eligibilityLabel('low_confidence') }}</option>
        <option value="latency_tail">{{ eligibilityLabel('latency_tail') }}</option>
        <option value="hard_rejected">{{ eligibilityLabel('hard_rejected') }}</option>
      </select>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full min-w-[1420px] table-fixed text-left text-sm">
        <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
          <tr>
            <th class="w-16 px-3 py-3 text-center font-medium">{{ t('admin.openaiAutoScheduler.ranking.rank') }}</th>
            <th class="w-56 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.accountChannel') }}</th>
            <th class="w-32 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.qualification') }}</th>
            <th class="w-24 px-3 py-3 text-right font-medium">{{ t('admin.openaiAutoScheduler.ranking.utility') }}</th>
            <th class="w-32 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.share') }}</th>
            <th class="w-32 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.performance') }}</th>
            <th class="w-32 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.stability') }}</th>
            <th class="w-28 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.load') }}</th>
            <th class="w-28 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.cost') }}</th>
            <th class="w-40 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.ranking.decision') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-if="loading" v-for="index in 5" :key="`ranking-loading-${index}`">
            <td v-for="cell in 10" :key="cell" class="px-3 py-4"><span class="block h-4 animate-pulse rounded bg-gray-100 dark:bg-dark-700" /></td>
          </tr>
          <tr v-else-if="!result?.items.length">
            <td colspan="10" class="px-4 py-16 text-center text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.ranking.noData') }}</td>
          </tr>
          <tr v-for="row in result?.items || []" v-else :key="rankingKey(row)" class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-800/60" @click="emit('select', row)">
            <td class="px-3 py-3 text-center text-lg font-semibold text-gray-900 dark:text-white">{{ row.rank || '—' }}</td>
            <td class="px-3 py-3">
              <span class="block truncate font-medium text-gray-900 dark:text-white" :title="row.account_name">{{ row.account_name }}</span>
              <span class="block truncate text-xs text-gray-500 dark:text-dark-400">#{{ row.account_id }} · {{ accountScope(row) }}</span>
              <span class="block truncate text-xs text-gray-400">{{ accountDimension(row) }}</span>
            </td>
            <td class="px-3 py-3"><span :class="eligibilityClass(row.eligibility)">{{ eligibilityLabel(row.eligibility) }}</span><span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ trafficClassLabel(row.traffic_class) }}</span></td>
            <td class="px-3 py-3 text-right"><span class="text-base font-semibold text-gray-950 dark:text-white">{{ row.utility_score.toFixed(1) }}</span><span class="text-xs text-gray-400"> / 100</span></td>
            <td class="px-3 py-3">
              <span class="block font-semibold text-primary-600 dark:text-primary-400">{{ percent(row.target_share) }} {{ t('admin.openaiAutoScheduler.ranking.target') }}</span>
              <span class="block text-xs text-gray-500">{{ percent(row.actual_share) }} {{ t('admin.openaiAutoScheduler.ranking.actual') }} · {{ row.selected_requests }}</span>
            </td>
            <td class="px-3 py-3"><span class="block">{{ duration(row.predicted_ttft_ms) }}</span><span class="text-xs text-gray-500">P50 {{ duration(row.ttft_p50_ms) }} · P90 {{ duration(row.ttft_p90_ms) }}</span></td>
            <td class="px-3 py-3"><span class="block">{{ percent(1 - row.error_rate) }}</span><span class="text-xs text-gray-500">429 {{ percent(row.rate_limited_rate) }} · 5xx {{ percent(row.server_error_rate) }}</span></td>
            <td class="px-3 py-3"><span class="block">{{ row.load_inflight }} / {{ row.load_capacity }}</span><span class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.health.waiting') }} {{ row.waiting_count }}</span></td>
            <td class="px-3 py-3"><span class="block">{{ price(row.channel_price) }}</span><span class="text-xs text-gray-500">{{ row.estimated_cost.toFixed(4) }}</span></td>
            <td class="px-3 py-3"><span class="block truncate font-medium" :title="decisionLabel(row.decision_summary)">{{ decisionLabel(row.decision_summary) }}</span><span class="block truncate text-xs text-gray-500" :title="deviationLabel(row.deviation_reasons)">{{ deviationLabel(row.deviation_reasons) }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>

    <Pagination v-if="result && result.total > result.page_size" :page="result.page" :pageSize="result.page_size" :total="result.total" @update:page="emit('page', $event, result!.page_size)" @update:pageSize="emit('page', 1, $event)" />
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Pagination from '@/components/common/Pagination.vue'
import type { OpenAISchedulerRankingItem, OpenAISchedulerRankingParams, OpenAISchedulerRankingResult, OpenAISchedulerRankingWindow } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ result: OpenAISchedulerRankingResult | null; loading: boolean; groupName: string; filters: Omit<OpenAISchedulerRankingParams, 'group_id'> }>()
const emit = defineEmits<{
  (event: 'filter', value: Partial<Omit<OpenAISchedulerRankingParams, 'group_id'>>): void
  (event: 'page', page: number, pageSize: number): void
  (event: 'select', row: OpenAISchedulerRankingItem): void
}>()
const { t } = useI18n()
const windows: OpenAISchedulerRankingWindow[] = ['15m', '1h', '6h', '24h', '7d']

const effectiveStatus = computed(() => {
  const context = props.result?.policy_context
  if (!context) return t('admin.openaiAutoScheduler.ranking.pending')
  if (!context.engine_enabled) return t('admin.openaiAutoScheduler.ranking.engineDisabled')
  if (!context.global_enabled) return t('admin.openaiAutoScheduler.ranking.globalDisabled')
  if (!context.group_enabled) return t('admin.openaiAutoScheduler.ranking.groupDisabled')
  if (context.shadow_mode) return t('admin.openaiAutoScheduler.ranking.shadowEffective', { mode: modeLabel(context.configured_mode) })
  return t('admin.openaiAutoScheduler.ranking.liveEffective', { mode: modeLabel(context.effective_mode) })
})
const statusClass = computed(() => {
  const context = props.result?.policy_context
  if (context?.engine_enabled && context.global_enabled && context.group_enabled && !context.shadow_mode) return 'border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200'
  if (context?.shadow_mode) return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200'
  return 'border-gray-200 bg-gray-50 text-gray-800 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200'
})
const partitionSummary = computed(() => {
  if (!props.result) return '—'
  return t('admin.openaiAutoScheduler.ranking.comprehensiveScope')
})

function emitInput(key: keyof Omit<OpenAISchedulerRankingParams, 'group_id'>, event: Event): void { emit('filter', { [key]: (event.target as HTMLInputElement).value }) }
function rankingKey(row: OpenAISchedulerRankingItem): string { return String(row.account_id) }
function accountScope(row: OpenAISchedulerRankingItem): string {
  if (row.partition_count > 1) return t('admin.openaiAutoScheduler.ranking.aggregatedPartitions', { count: row.partition_count })
  return row.partition.model_family || t('admin.openaiAutoScheduler.ranking.comprehensiveScope')
}
function accountDimension(row: OpenAISchedulerRankingItem): string {
  if (row.partition_count > 1) return t('admin.openaiAutoScheduler.ranking.comprehensiveAccount')
  return `${endpointLabel(row.partition.endpoint)} · ${transportLabel(row.partition.transport)}`
}
function percent(value: number): string { return `${(Math.max(0, value || 0) * 100).toFixed(1)}%` }
function duration(value?: number | null): string { return !value ? '—' : value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms` }
function price(value?: number | null): string { return value == null ? '—' : `${Number(value.toFixed(4))}x` }
function endpointLabel(value: string): string { return value?.replace(/_/g, ' ') || '—' }
function transportLabel(value: string): string { return { http_sse: 'HTTP / SSE', responses_websockets_v2: 'WebSocket V2' }[value] || value || '—' }
function modeLabel(value: string): string { return t(`admin.openaiAutoScheduler.modes.${({ performance_first: 'performance', cost_first: 'cost', efficiency: 'efficiency' } as Record<string, string>)[value] || value}`) }
function eligibilityLabel(value: string): string { return t(`admin.openaiAutoScheduler.ranking.eligibility.${({ low_confidence: 'lowConfidence', latency_tail: 'latencyTail', hard_rejected: 'hardRejected' } as Record<string, string>)[value] || value}`) }
function eligibilityClass(value: string): string { const colors: Record<string, string> = { eligible: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300', low_confidence: 'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300', latency_tail: 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300', hard_rejected: 'bg-red-50 text-red-700 dark:bg-red-500/15 dark:text-red-300' }; return `inline-flex rounded px-2 py-1 text-xs font-medium ${colors[value] || colors.hard_rejected}` }
function trafficClassLabel(value?: string): string { return t(`admin.openaiAutoScheduler.ranking.trafficClass.${value || 'fallback'}`) }
function decisionLabel(value: string): string { return t(`admin.openaiAutoScheduler.ranking.decisions.${({ highest_utility: 'highestUtility', weighted_allocation: 'weightedAllocation', fallback_only: 'fallbackOnly', health_unavailable: 'healthUnavailable', latency_budget_exceeded: 'latencyBudgetExceeded' } as Record<string, string>)[value] || 'fallbackOnly'}`) }
function deviationLabel(values: string[]): string { if (!values?.length) return t('admin.openaiAutoScheduler.ranking.noDeviation'); return values.map((value) => t(`admin.openaiAutoScheduler.ranking.deviations.${({ health_low_confidence: 'healthLowConfidence', insufficient_window_samples: 'insufficientSamples', shadow_mode: 'shadowMode', legacy_fallback: 'legacyFallback' } as Record<string, string>)[value] || 'other'}`)).join(' · ') }
function formatCalculatedAt(value?: string): string { return value ? new Date(value).toLocaleTimeString() : '—' }
</script>
