<template>
  <BaseDialog :show="!!detail" :title="t('openAIQuality.runDetailsTitle', { id: detail?.run.id ?? '' })" width="wide" :z-index="60" @close="$emit('close')">
    <template v-if="detail">
      <div class="mb-5 flex flex-wrap items-center gap-2 text-sm">
        <span class="rounded-lg px-2.5 py-1 font-medium" :class="qualityOutcomeClass(qualityRunOutcome(detail.run))">{{ label(qualityRunOutcome(detail.run)) }}</span>
        <span class="text-gray-500">{{ formatDate(detail.run.started_at || detail.run.queued_at) }}</span>
      </div>
      <dl class="grid gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.account') }}</dt><dd class="mt-1 break-all font-medium">{{ detail.run.input_snapshot?.account_name || `#${detail.run.account_id}` }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.sourceGroup') }}</dt><dd class="mt-1">{{ detail.run.input_snapshot?.source_group_name || groupName || t('openAIQuality.unavailableGroup') }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.model') }}</dt><dd class="mt-1">{{ detail.run.input_snapshot?.condition.model_id || '—' }}<span v-if="detail.run.input_snapshot?.effective_upstream_model !== detail.run.input_snapshot?.condition.model_id" class="text-gray-500"> → {{ detail.run.input_snapshot?.effective_upstream_model }}</span></dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.trigger') }}</dt><dd class="mt-1">{{ label(detail.run.trigger_source) }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.answer') }}</dt><dd class="mt-1 font-mono">{{ detail.run.result.final_answer ?? '—' }} / {{ detail.run.result.expected_answer }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.latency') }}</dt><dd class="mt-1">{{ detail.run.result.latency_ms }} ms</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.reason') }}</dt><dd class="mt-1">{{ label(detail.run.result.reason_code || detail.run.reason_code) }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.stateParticipation') }}</dt><dd class="mt-1">{{ detail.run.state_applied ? t('openAIQuality.appliedToState') : label(detail.run.state_skip_reason || 'not_applied') }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.tokenUsage') }}</dt><dd class="mt-1">{{ detail.run.result.input_tokens ?? '—' }} / {{ detail.run.result.output_tokens ?? '—' }} / {{ detail.run.result.total_tokens ?? '—' }}</dd></div>
        <div><dt class="text-xs text-gray-500">{{ t('openAIQuality.estimatedCost') }}</dt><dd class="mt-1">{{ detail.run.result.estimated_cost_usd ?? '—' }}</dd></div>
      </dl>
      <section class="mt-6 border-t border-gray-100 pt-4 dark:border-dark-700">
        <h4 class="mb-3 text-sm font-semibold">{{ t('openAIQuality.finalResponse') }}</h4>
        <p v-if="detail.response_expired" class="text-sm text-gray-500">{{ t('openAIQuality.responseExpired') }}</p>
        <p v-else-if="!detail.response_available" class="text-sm text-gray-500">{{ t('openAIQuality.responseUnavailable') }}</p>
        <pre v-else class="max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-4 text-sm leading-relaxed dark:bg-dark-900">{{ detail.run.response_text }}</pre>
        <p v-if="detail.run.result.response_truncated" class="mt-2 text-xs text-amber-700">{{ t('openAIQuality.responseTruncated') }}</p>
      </section>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { QualityRunDetail } from '@/api/admin/openaiAccountQuality'
import { qualityOutcomeClass, qualityRunOutcome } from './qualityPresentation'

defineProps<{ detail: QualityRunDetail | null; groupName?: string }>()
defineEmits<{ (event: 'close'): void }>()
const { t, te } = useI18n()
function label(value: string) { return te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
function formatDate(value: string) { return new Date(value).toLocaleString() }
</script>
