<template>
  <section class="min-w-0">
    <div class="overflow-x-auto">
      <table class="w-full min-w-[880px] text-left text-sm">
        <thead class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
          <tr>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.time') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.accountModel') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.result') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.latency') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.scoreChange') }}</th>
            <th class="px-3 py-3 font-medium">{{ t('admin.openaiAutoScheduler.events.detail') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-if="loading" v-for="index in 6" :key="index">
            <td v-for="cell in 6" :key="cell" class="px-3 py-4"><span class="block h-4 animate-pulse rounded bg-gray-100 dark:bg-dark-700" /></td>
          </tr>
          <tr v-else-if="!events.length">
            <td colspan="6" class="px-4 py-16 text-center text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.events.noData') }}</td>
          </tr>
          <tr v-for="event in events" v-else :key="eventKey(event)">
            <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-500 dark:text-dark-400">{{ formatDate(event.created_at) }}</td>
            <td class="px-3 py-3">
              <span class="block font-medium text-gray-900 dark:text-white">#{{ event.account_id }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ event.model }} · {{ t('admin.openaiAutoScheduler.health.group') }} {{ event.group_id }}</span>
            </td>
            <td class="px-3 py-3"><span :class="eventClass(event.event_type)">{{ eventLabel(event.event_type) }}</span></td>
            <td class="px-3 py-3 text-gray-700 dark:text-dark-200">
              <span v-if="event.ttfb_ms != null" class="block">{{ t('admin.openaiAutoScheduler.events.ttfb') }} {{ formatDuration(event.ttfb_ms) }}</span>
              <span v-if="event.latency_ms != null" class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.openaiAutoScheduler.events.totalDuration') }} {{ formatDuration(event.latency_ms) }}</span>
              <span v-if="event.ttfb_ms == null && event.latency_ms == null">—</span>
            </td>
            <td class="whitespace-nowrap px-3 py-3 font-medium text-gray-800 dark:text-dark-100">{{ formatScore(event.score_before) }} → {{ formatScore(event.score_after) }}</td>
            <td class="max-w-sm px-3 py-3">
              <details v-if="event.message" class="text-xs text-gray-600 dark:text-dark-300">
                <summary class="cursor-pointer truncate">{{ event.message }}</summary>
                <p class="mt-2 break-words whitespace-pre-wrap">{{ event.message }}</p>
              </details>
              <span v-else class="text-gray-400">—</span>
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
import Pagination from '@/components/common/Pagination.vue'
import type { OpenAIAutoSchedulerEvent } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  events: OpenAIAutoSchedulerEvent[]
  loading: boolean
  total: number
  page: number
  pageSize: number
}>()

const emit = defineEmits<{ (event: 'page', page: number, pageSize: number): void }>()

function eventKey(event: OpenAIAutoSchedulerEvent): string {
  return `${event.created_at}:${event.account_id}:${event.group_id}:${event.model}:${event.event_type}`
}

function eventLabel(event: string): string {
  return { success: t('admin.openaiAutoScheduler.events.types.success'), slow: t('admin.openaiAutoScheduler.events.types.slow'), severe_slow: t('admin.openaiAutoScheduler.events.types.severeSlow'), error: t('admin.openaiAutoScheduler.events.types.error'), rate_limited: t('admin.openaiAutoScheduler.events.types.rateLimited'), probe_success: t('admin.openaiAutoScheduler.events.types.probeSuccess'), probe_error: t('admin.openaiAutoScheduler.events.types.probeError'), manual_reset: t('admin.openaiAutoScheduler.events.types.manualReset') }[event] || event
}

function eventClass(event: string): string {
  const danger = ['error', 'probe_error', 'severe_slow', 'rate_limited'].includes(event)
  const warning = event === 'slow'
  const color = danger ? 'bg-red-50 text-red-700 dark:bg-red-500/15 dark:text-red-300' : warning ? 'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300' : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  return `inline-flex rounded px-2 py-1 text-xs font-medium ${color}`
}

function formatDuration(value: number): string {
  return value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms`
}

function formatScore(value: number): string {
  return (Math.max(0, Math.min(10000, value)) / 10000).toFixed(4)
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}
</script>
