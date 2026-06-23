<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  OpenAIRoutingSummary,
  OpenAIRoutingSummaryReason,
  OpenAIRoutingStatusLabel,
} from '@/api/admin/openaiScheduler'

const props = defineProps<{
  summary?: OpenAIRoutingSummary | null
}>()

defineEmits<{
  open: []
}>()

const { t, te } = useI18n()

const translateStatus = (status: OpenAIRoutingStatusLabel) => {
  const key = `admin.accounts.routingPriority.status.${status}`
  return te(key) ? t(key) : status
}

const translateReason = (reason: OpenAIRoutingSummaryReason) => {
  const summaryKey = `admin.accounts.routingPriority.summary.${reason}`
  if (te(summaryKey)) return t(summaryKey)

  const reasonKey = `admin.accounts.routingPriority.reasons.${reason}`
  if (te(reasonKey)) return t(reasonKey)

  return reason
}

const badgeClass = computed(() => {
  const summary = props.summary
  if (!summary) return 'border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-400'
  if (!summary.is_schedulable_now) {
    return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300'
  }
  if (summary.tier === 'primary') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300'
  }
  if (summary.tier === 'observe') {
    return 'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300'
  }
  if (summary.tier === 'standby') {
    return 'border-indigo-200 bg-indigo-50 text-indigo-700 dark:border-indigo-500/30 dark:bg-indigo-500/10 dark:text-indigo-300'
  }
  return 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200'
})

const statusText = computed(() => {
  if (!props.summary) return ''
  return translateStatus(props.summary.status_label)
})

const primaryText = computed(() => {
  const summary = props.summary
  if (!summary) return ''
  const primaryReason = summary.block_reasons?.[0] ?? summary.summary_reason
  return translateReason(primaryReason)
})
</script>

<template>
  <button
    type="button"
    class="inline-flex min-w-[132px] items-center gap-1.5 rounded-md border px-2 py-1 text-left text-xs transition-colors hover:bg-white dark:hover:bg-dark-700"
    :class="badgeClass"
    @click="$emit('open')"
  >
    <span class="font-semibold leading-none">
      {{ summary?.is_schedulable_now && summary.rank ? `#${summary.rank}` : statusText }}
    </span>
    <span
      v-if="summary?.is_schedulable_now"
      class="rounded bg-white/70 px-1 py-0.5 text-[10px] font-medium leading-none text-current dark:bg-dark-900/40"
    >
      {{ statusText }}
    </span>
    <span class="truncate leading-none">
      {{ primaryText }}
    </span>
  </button>
</template>
