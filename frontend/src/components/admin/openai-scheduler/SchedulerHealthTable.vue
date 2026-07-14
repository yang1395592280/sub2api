<template>
  <section class="min-w-0">
    <div class="grid gap-3 border-b border-gray-200 pb-4 dark:border-dark-700 sm:grid-cols-2 xl:grid-cols-5">
      <select class="input" :value="filters?.state || ''" :aria-label="t('admin.openaiAutoScheduler.health.state')" @change="emitFilter('state', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.health.allStates') }}</option>
        <option value="running">{{ t('admin.openaiAutoScheduler.health.states.running') }}</option>
        <option value="observing">{{ t('admin.openaiAutoScheduler.health.states.observing') }}</option>
        <option value="open">{{ t('admin.openaiAutoScheduler.health.states.open') }}</option>
        <option value="half_open">{{ t('admin.openaiAutoScheduler.health.states.halfOpen') }}</option>
      </select>
      <input class="input" :value="filters?.model_family || ''" :aria-label="t('admin.openaiAutoScheduler.health.model')" :placeholder="t('admin.openaiAutoScheduler.health.model')" @change="emitFilter('model_family', $event)" />
      <select class="input" :value="filters?.endpoint || ''" aria-label="Endpoint" @change="emitFilter('endpoint', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.health.allEndpoints') }}</option>
        <option value="responses">Responses</option>
        <option value="chat_completions">Chat Completions</option>
        <option value="embeddings">Embeddings</option>
        <option value="images_generations">Images Generations</option>
        <option value="images_edits">Images Edits</option>
      </select>
      <select class="input" :value="filters?.transport || ''" aria-label="Transport" @change="emitFilter('transport', $event)">
        <option value="">{{ t('admin.openaiAutoScheduler.health.allTransports') }}</option>
        <option value="http_sse">HTTP / SSE</option>
        <option value="websocket">WebSocket</option>
      </select>
      <select class="input" :value="filters?.sort || 'predicted_ttft_ms'" :aria-label="t('admin.openaiAutoScheduler.health.sort')" @change="emitFilter('sort', $event)">
        <option value="predicted_ttft_ms">{{ t('admin.openaiAutoScheduler.health.predictedTTFT') }}</option>
        <option value="error_rate">{{ t('admin.openaiAutoScheduler.health.totalErrorRate') }}</option>
        <option value="real_sample_count">{{ t('admin.openaiAutoScheduler.health.realSamples') }}</option>
        <option value="snapshot_age_ms">{{ t('admin.openaiAutoScheduler.health.snapshotAge') }}</option>
        <option value="channel_price">{{ t('admin.openaiAutoScheduler.health.channelPrice') }}</option>
      </select>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full min-w-[1180px] table-fixed text-left text-sm">
        <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
          <tr>
            <th class="w-52 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.accountChannel') }}</th>
            <th class="w-28 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.state') }}</th>
            <th class="w-52 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.modelPath') }}</th>
            <th class="w-28 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.predictedTTFT') }}</th>
            <th class="w-32 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.samples') }}</th>
            <th class="w-28 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.load') }}</th>
            <th class="w-24 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.price') }}</th>
            <th class="w-40 px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.health.decisionTitle') }}</th>
            <th class="w-24 px-3 py-3 text-right font-medium">{{ t('admin.openaiAutoScheduler.health.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-if="loading" v-for="index in 5" :key="`loading-${index}`">
            <td v-for="cell in 9" :key="cell" class="px-3 py-4"><span class="block h-4 animate-pulse rounded bg-gray-100 dark:bg-dark-700" /></td>
          </tr>
          <tr v-else-if="!rows.length">
            <td colspan="9" class="px-4 py-16 text-center text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.health.noData') }}</td>
          </tr>
          <tr
            v-for="row in rows"
            v-else
            :key="healthKey(row)"
            :data-testid="`health-row-${row.account_id}`"
            class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-800/60"
            @click="emit('select', row)"
          >
            <td class="px-3 py-3">
              <span class="block truncate font-medium text-gray-900 dark:text-white" :title="row.account_name">{{ row.account_name }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">#{{ row.account_id }} · {{ t('admin.openaiAutoScheduler.health.group') }} {{ row.group_id }}</span>
            </td>
            <td class="px-3 py-3"><span :class="stateClass(row.state)">{{ stateLabel(row.state) }}</span></td>
            <td class="px-3 py-3 text-gray-700 dark:text-dark-200">
              <span class="block truncate" :title="row.model_family">{{ row.model_family || '—' }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ endpointLabel(row.endpoint) }} · {{ transportLabel(row.transport) }}</span>
            </td>
            <td class="px-3 py-3 font-medium text-gray-900 dark:text-white">{{ formatDuration(row.predicted_ttft_ms) }}</td>
            <td class="px-3 py-3 text-gray-700 dark:text-dark-200">
              <span class="block">{{ t('admin.openaiAutoScheduler.health.realSamples') }} {{ row.real_sample_count }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.health.probeSamples') }} {{ row.probe_sample_count }}</span>
            </td>
            <td class="px-3 py-3 text-gray-700 dark:text-dark-200">
              <span class="block">{{ row.load_inflight }} / {{ row.load_capacity }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.health.waiting') }} {{ row.waiting_count }}</span>
            </td>
            <td class="px-3 py-3 text-gray-700 dark:text-dark-200">{{ formatPrice(row.channel_price) }}</td>
            <td class="px-3 py-3">
              <span class="block font-medium text-gray-900 dark:text-white">{{ decisionLabel(row.decision) }}</span>
              <span class="block truncate text-xs text-gray-500 dark:text-dark-400" :title="decisionReasonLabel(row.decision_reason)">{{ decisionReasonLabel(row.decision_reason) }}</span>
            </td>
            <td class="px-3 py-3">
              <div class="flex justify-end gap-1">
                <button
                  type="button"
                  :data-testid="`health-probe-${row.account_id}`"
                  class="rounded p-2 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700"
                  :title="t('admin.openaiAutoScheduler.actions.manualProbe')"
                  @click.stop="emit('probe', row)"
                ><Icon name="beaker" size="sm" /><span class="sr-only">{{ t('admin.openaiAutoScheduler.actions.manualProbe') }}</span></button>
                <button
                  type="button"
                  :data-testid="`health-reset-${row.account_id}`"
                  class="rounded p-2 text-gray-500 hover:bg-gray-100 hover:text-red-600 dark:hover:bg-dark-700"
                  :title="t('admin.openaiAutoScheduler.actions.resetHealth')"
                  @click.stop="emit('reset', row)"
                ><Icon name="refresh" size="sm" /><span class="sr-only">{{ t('admin.openaiAutoScheduler.actions.resetHealth') }}</span></button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <Pagination
      v-if="total > pageSize"
      :page="page"
      :pageSize="pageSize"
      :total="total"
      @update:page="emit('page', $event, pageSize)"
      @update:pageSize="emit('page', 1, $event)"
    />
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { OpenAISchedulerHealthParams, OpenAISchedulerHealthRow } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  rows: OpenAISchedulerHealthRow[]
  loading: boolean
  total: number
  page: number
  pageSize: number
  filters?: OpenAISchedulerHealthParams
}>()

const emit = defineEmits<{
  (event: 'filter', value: Partial<OpenAISchedulerHealthParams>): void
  (event: 'page', page: number, pageSize: number): void
  (event: 'select', row: OpenAISchedulerHealthRow): void
  (event: 'probe', row: OpenAISchedulerHealthRow): void
  (event: 'reset', row: OpenAISchedulerHealthRow): void
}>()

function emitFilter(key: keyof OpenAISchedulerHealthParams, event: Event): void {
  emit('filter', { [key]: (event.target as HTMLInputElement).value })
}

function healthKey(row: OpenAISchedulerHealthRow): string {
  return `${row.account_id}:${row.group_id}:${row.model_family}:${row.endpoint}:${row.transport}`
}

function stateLabel(state: string): string {
  return { running: t('admin.openaiAutoScheduler.health.states.running'), observing: t('admin.openaiAutoScheduler.health.states.observing'), open: t('admin.openaiAutoScheduler.health.states.open'), half_open: t('admin.openaiAutoScheduler.health.states.halfOpen') }[state] || t('admin.openaiAutoScheduler.health.states.unknown')
}

function stateClass(state: string): string {
  const color = { running: 'emerald', observing: 'amber', open: 'red', half_open: 'sky' }[state] || 'gray'
  const classes: Record<string, string> = {
    emerald: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
    amber: 'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
    red: 'bg-red-50 text-red-700 dark:bg-red-500/15 dark:text-red-300',
    sky: 'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300',
    gray: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300',
  }
  return `inline-flex rounded px-2 py-1 text-xs font-medium ${classes[color]}`
}

function decisionLabel(decision: string): string {
  return {
    context_required: t('admin.openaiAutoScheduler.health.decision.contextRequired'),
    circuit_rejected: t('admin.openaiAutoScheduler.health.decision.circuitRejected'),
    stale: t('admin.openaiAutoScheduler.health.decision.stale'),
    health_unavailable: t('admin.openaiAutoScheduler.health.decision.unavailable'),
    hard_filtered: t('admin.openaiAutoScheduler.health.decision.hardFiltered'),
  }[decision] || t('admin.openaiAutoScheduler.health.decision.pending')
}

function decisionReasonLabel(reason: string): string {
  const labels: Record<string, string> = {
    request_context_required: t('admin.openaiAutoScheduler.health.reasons.requestContext'),
    snapshot_expired: t('admin.openaiAutoScheduler.health.reasons.snapshotExpired'),
    snapshot_missing: t('admin.openaiAutoScheduler.health.reasons.snapshotMissing'),
    group_inactive: t('admin.openaiAutoScheduler.health.reasons.groupInactive'),
    group_scheduler_disabled: t('admin.openaiAutoScheduler.health.reasons.groupDisabled'),
    account_inactive: t('admin.openaiAutoScheduler.health.reasons.accountInactive'),
    account_unschedulable: t('admin.openaiAutoScheduler.health.reasons.accountUnschedulable'),
    account_expired: t('admin.openaiAutoScheduler.health.reasons.accountExpired'),
    account_overloaded: t('admin.openaiAutoScheduler.health.reasons.accountOverloaded'),
    account_rate_limited: t('admin.openaiAutoScheduler.health.reasons.accountRateLimited'),
    running: t('admin.openaiAutoScheduler.health.states.running'),
    observing: t('admin.openaiAutoScheduler.health.states.observing'),
    open: t('admin.openaiAutoScheduler.health.states.open'),
    half_open: t('admin.openaiAutoScheduler.health.states.halfOpen'),
  }
  return labels[reason] || (reason.startsWith('temporarily_blocked') ? t('admin.openaiAutoScheduler.health.reasons.temporarilyBlocked') : reason || '—')
}

function endpointLabel(endpoint: string): string {
  return endpoint.replace(/_/g, ' ') || '—'
}

function transportLabel(transport: string): string {
  return { http_sse: 'HTTP / SSE', websocket: 'WebSocket' }[transport] || transport || '—'
}

function formatDuration(value?: number | null): string {
  if (value == null) return '—'
  return value >= 1000 ? `${(value / 1000).toFixed(value >= 10_000 ? 2 : 2)}s` : `${Math.round(value)}ms`
}

function formatPrice(value?: number | null): string {
  return value == null ? '—' : `${Number(value.toFixed(4))}x`
}
</script>
