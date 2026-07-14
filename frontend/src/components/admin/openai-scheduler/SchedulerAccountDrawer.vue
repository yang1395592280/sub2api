<template>
  <Teleport to="body">
    <div v-if="open && account" class="fixed inset-0 z-50 flex justify-end bg-black/35" @click.self="emit('close')">
      <aside
        data-testid="scheduler-account-drawer"
        class="flex h-full w-full flex-col bg-white shadow-xl dark:bg-dark-900 sm:max-w-xl"
        :aria-label="t('admin.openaiAutoScheduler.health.drawerLabel')"
      >
        <header class="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div class="min-w-0">
            <h2 class="truncate text-base font-semibold text-gray-950 dark:text-white">{{ account.account_name }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">#{{ account.account_id }} · {{ account.model_family }} · {{ account.endpoint }}</p>
          </div>
          <button data-testid="drawer-close" type="button" class="rounded p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-700" :title="t('admin.openaiAutoScheduler.actions.close')" @click="emit('close')">
            <Icon name="x" size="sm" /><span class="sr-only">{{ t('admin.openaiAutoScheduler.actions.close') }}</span>
          </button>
        </header>

        <div class="flex-1 overflow-y-auto px-5 py-5">
          <div class="flex flex-wrap items-center gap-2">
            <span class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-dark-200">{{ stateLabel(account.state) }}</span>
            <span class="rounded bg-sky-50 px-2 py-1 text-xs font-medium text-sky-700 dark:bg-sky-500/15 dark:text-sky-300">{{ account.scheduler_mode === 'balanced' ? t('admin.openaiAutoScheduler.modes.balanced') : t('admin.openaiAutoScheduler.modes.legacy') }}</span>
            <span v-if="account.shadow_mode" class="rounded bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-500/15 dark:text-amber-300">{{ t('admin.openaiAutoScheduler.modes.shadow') }}</span>
          </div>

          <dl class="mt-5 grid grid-cols-2 gap-x-5 gap-y-4 border-y border-gray-200 py-5 dark:border-dark-700">
            <div><dt>{{ t('admin.openaiAutoScheduler.health.predictedTTFT') }}</dt><dd>{{ formatDuration(account.predicted_ttft_ms) }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.snapshotAge') }}</dt><dd>{{ formatDuration(account.snapshot_age_ms) }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.realSamples') }}</dt><dd>{{ account.real_sample_count }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.probeSamples') }}</dt><dd>{{ account.probe_sample_count }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.totalErrorRate') }}</dt><dd>{{ formatPercent(account.error_rate) }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.rateLimitedRate') }}</dt><dd>{{ formatPercent(account.rate_limited_rate) }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.serverErrorRate') }}</dt><dd>{{ formatPercent(account.server_error_rate) }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.loadCapacity') }}</dt><dd>{{ account.load_inflight }} / {{ account.load_capacity }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.waitingRequests') }}</dt><dd>{{ account.waiting_count }}</dd></div>
            <div><dt>{{ t('admin.openaiAutoScheduler.health.channelPrice') }}</dt><dd>{{ account.channel_price == null ? '—' : `${account.channel_price}x` }}</dd></div>
            <div class="col-span-2"><dt>{{ t('admin.openaiAutoScheduler.health.decisionTitle') }}</dt><dd>{{ decisionLabel(account.decision) }}</dd></div>
            <div v-if="account.cooldown_until" class="col-span-2"><dt>{{ t('admin.openaiAutoScheduler.health.cooldownUntil') }}</dt><dd>{{ formatDate(account.cooldown_until) }}</dd></div>
          </dl>

          <section class="mt-5">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.health.recentEvents') }}</h3>
            <div v-if="events.length" class="mt-3 divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-800 dark:border-dark-800">
              <div v-for="event in events" :key="`${event.created_at}:${event.event_type}`" class="py-3 text-sm">
                <div class="flex items-center justify-between gap-3">
                  <span class="font-medium text-gray-800 dark:text-dark-100">{{ eventLabel(event.event_type) }}</span>
                  <time class="text-xs text-gray-500 dark:text-dark-400">{{ formatDate(event.created_at) }}</time>
                </div>
                <p v-if="event.message" class="mt-1 break-words text-xs text-gray-500 dark:text-dark-400">{{ event.message }}</p>
              </div>
            </div>
            <p v-else class="mt-5 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.health.noEvents') }}</p>
          </section>
        </div>

        <footer class="flex justify-end gap-2 border-t border-gray-200 px-5 py-4 dark:border-dark-700">
          <button data-testid="drawer-probe" type="button" class="btn btn-secondary" @click="emit('probe', account)"><Icon name="beaker" size="sm" />{{ t('admin.openaiAutoScheduler.actions.manualProbe') }}</button>
          <button data-testid="drawer-reset" type="button" class="btn btn-secondary text-red-600" @click="emit('reset', account)"><Icon name="refresh" size="sm" />{{ t('admin.openaiAutoScheduler.actions.resetHealth') }}</button>
        </footer>
      </aside>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { OpenAIAutoSchedulerEvent, OpenAISchedulerHealthRow } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  open: boolean
  account: OpenAISchedulerHealthRow | null
  events: OpenAIAutoSchedulerEvent[]
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'probe', row: OpenAISchedulerHealthRow): void
  (event: 'reset', row: OpenAISchedulerHealthRow): void
}>()

function stateLabel(state: string): string {
  return { running: t('admin.openaiAutoScheduler.health.states.running'), observing: t('admin.openaiAutoScheduler.health.states.observing'), open: t('admin.openaiAutoScheduler.health.states.open'), half_open: t('admin.openaiAutoScheduler.health.states.halfOpen') }[state] || t('admin.openaiAutoScheduler.health.states.unknown')
}

function decisionLabel(decision: string): string {
  return { context_required: t('admin.openaiAutoScheduler.health.detailDecision.contextRequired'), circuit_rejected: t('admin.openaiAutoScheduler.health.detailDecision.circuitRejected'), stale: t('admin.openaiAutoScheduler.health.detailDecision.stale'), health_unavailable: t('admin.openaiAutoScheduler.health.detailDecision.unavailable'), hard_filtered: t('admin.openaiAutoScheduler.health.detailDecision.hardFiltered') }[decision] || t('admin.openaiAutoScheduler.health.decision.pending')
}

function eventLabel(event: string): string {
  return { success: t('admin.openaiAutoScheduler.events.types.success'), slow: t('admin.openaiAutoScheduler.events.types.slow'), severe_slow: t('admin.openaiAutoScheduler.events.types.severeSlow'), error: t('admin.openaiAutoScheduler.events.types.error'), rate_limited: t('admin.openaiAutoScheduler.events.types.rateLimited'), probe_success: t('admin.openaiAutoScheduler.events.types.probeSuccess'), probe_error: t('admin.openaiAutoScheduler.events.types.probeError'), manual_reset: t('admin.openaiAutoScheduler.events.types.manualReset') }[event] || event
}

function formatDuration(value?: number | null): string {
  if (value == null) return '—'
  return value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms`
}

function formatPercent(value: number): string {
  return `${(value * 100).toFixed(value > 0 && value < 0.1 ? 1 : 0)}%`
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}
</script>

<style scoped>
dt {
  font-size: 0.75rem;
  color: rgb(107 114 128);
}

dd {
  margin-top: 0.25rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: rgb(17 24 39);
}

:global(.dark) dt {
  color: rgb(156 163 175);
}

:global(.dark) dd {
  color: white;
}
</style>
