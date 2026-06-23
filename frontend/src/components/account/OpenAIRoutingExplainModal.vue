<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type {
  OpenAIRoutingAccountExplain,
  OpenAIRoutingBlockSource,
  OpenAIRoutingExplainNote,
  OpenAIRoutingStatusLabel,
  OpenAIRoutingSummaryReason,
} from '@/api/admin/openaiScheduler'

const props = defineProps<{
  show: boolean
  accountId?: number | null
  loading?: boolean
  explain: OpenAIRoutingAccountExplain | null
}>()

defineEmits<{
  close: []
}>()

const { t, te } = useI18n()

const scoreOrder = ['total', 'priority', 'load', 'queue', 'error_rate', 'ttft', 'price', 'health'] as const

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

const translateBlockSource = (source: OpenAIRoutingBlockSource) => {
  const key = `admin.accounts.routingPriority.blockSources.${source}`
  return te(key) ? t(key) : source
}

const translateNote = (note: OpenAIRoutingExplainNote) => {
  const key = `admin.accounts.routingPriority.notes.${note}`
  return te(key) ? t(key) : note
}

const formatNumber = (value: number) => value.toFixed(3)

const formatDateTime = (value?: string | null) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

const scoreItems = computed(() => {
  const score = props.explain?.account.score
  if (!score) return []

  return scoreOrder.map((key) => ({
    key,
    label: t(`admin.accounts.routingPriority.score.${key}`),
    value: score[key],
  }))
})

const noteItems = computed(() => props.explain?.notes.map(translateNote) ?? [])
</script>

<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.routingPriority.modal.title')"
    width="wide"
    @close="$emit('close')"
  >
    <div v-if="loading" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="!explain" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.accounts.routingPriority.modal.empty') }}
    </div>

    <div v-else class="space-y-4">
      <section class="rounded-lg border border-gray-200 px-4 py-3 dark:border-dark-600">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.routingPriority.sections.summary') }}
            </div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ explain.account.account_name }}
            </div>
            <div class="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ props.accountId ?? explain.account.account_id }}</span>
              <span>{{ translateStatus(explain.account.status_label) }}</span>
              <span v-if="explain.account.rank">#{{ explain.account.rank }}</span>
              <span>{{ translateReason(explain.account.summary_reason) }}</span>
            </div>
          </div>
          <div class="rounded-md bg-gray-50 px-3 py-2 text-right dark:bg-dark-700">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.routingPriority.score.total') }}
            </div>
            <div class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ formatNumber(explain.account.score.total) }}
            </div>
          </div>
        </div>
      </section>

      <section>
        <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.accounts.routingPriority.sections.score') }}
        </div>
        <div class="grid grid-cols-2 gap-2 md:grid-cols-4">
          <div
            v-for="item in scoreItems"
            :key="item.key"
            class="rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600"
          >
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ item.label }}</div>
            <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
              {{ formatNumber(item.value) }}
            </div>
          </div>
        </div>
      </section>

      <section v-if="explain.account.block_details?.length || explain.account.block_reasons?.length">
        <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.accounts.routingPriority.sections.blockReasons') }}
        </div>
        <div class="space-y-2">
          <div
            v-for="detail in explain.account.block_details ?? []"
            :key="`${detail.reason}-${detail.source}-${detail.until ?? ''}`"
            class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200"
          >
            <div class="font-medium">{{ translateReason(detail.reason) }}</div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 opacity-90">
              <span>{{ translateBlockSource(detail.source) }}</span>
              <span v-if="detail.until">{{ formatDateTime(detail.until) }}</span>
            </div>
          </div>
          <div
            v-for="reason in explain.account.block_reasons ?? []"
            :key="reason"
            class="rounded-md border border-amber-200 px-3 py-2 text-xs text-amber-800 dark:border-amber-500/30 dark:text-amber-200"
          >
            {{ translateReason(reason) }}
          </div>
        </div>
      </section>

      <section v-if="explain.top.length">
        <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.accounts.routingPriority.sections.topCandidates') }}
        </div>
        <div class="space-y-2">
          <div
            v-for="row in explain.top"
            :key="row.account_id"
            class="flex items-center justify-between gap-3 rounded-md border border-gray-200 px-3 py-2 text-xs dark:border-dark-600"
          >
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">
                {{ row.rank ? `#${row.rank}` : '-' }} {{ row.account_name }}
              </div>
              <div class="mt-1 truncate text-gray-500 dark:text-gray-400">
                {{ translateReason(row.summary_reason) }}
              </div>
            </div>
            <div class="shrink-0 font-mono text-gray-700 dark:text-gray-200">
              {{ formatNumber(row.score.total) }}
            </div>
          </div>
        </div>
      </section>

      <section v-if="noteItems.length">
        <div class="mb-2 text-xs font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.accounts.routingPriority.sections.notes') }}
        </div>
        <div class="space-y-1 text-xs text-gray-500 dark:text-gray-400">
          <p v-for="note in noteItems" :key="note">{{ note }}</p>
        </div>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>
