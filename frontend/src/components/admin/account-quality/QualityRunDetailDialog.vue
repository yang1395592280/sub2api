<template>
  <BaseDialog :show="!!detail" :title="t('openAIQuality.runDetailsTitle', { id: detail?.run.id ?? '' })" width="extra-wide" :z-index="60" @close="$emit('close')">
    <template v-if="detail">
      <div class="flex flex-wrap items-center gap-2 text-sm">
        <span class="rounded-lg px-2.5 py-1 font-semibold" :class="qualityOutcomeClass(qualityRunOutcome(detail.run))">{{ label(qualityRunOutcome(detail.run)) }}</span>
        <span class="font-medium">{{ detail.run.input_snapshot?.account_name || `#${detail.run.account_id}` }}</span>
        <span class="text-gray-400">·</span><span class="text-gray-500">{{ detail.run.input_snapshot?.condition.model_id }}</span>
        <time class="ml-auto text-xs text-gray-400">{{ formatDate(detail.run.started_at || detail.run.queued_at) }}</time>
      </div>
      <p v-if="dual" class="mt-3 text-xs text-gray-500">{{ t('openAIQuality.dualVerdictHelp') }}</p>
      <nav v-if="dual" class="mt-4 flex gap-1 border-b border-gray-100 dark:border-dark-700" :aria-label="t('openAIQuality.checkStages')">
        <button v-for="key in stages" :key="key" class="border-b-2 px-4 py-3 text-sm" :class="stage === key ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" :aria-pressed="stage === key" @click="stage = key">{{ t(`openAIQuality.stageNames.${key}`) }}<span class="ml-2 inline-block h-1.5 w-1.5 rounded-full" :class="stageColor(stageResult(key)?.verdict)" /></button>
      </nav>
      <div class="mt-5 grid gap-6 lg:grid-cols-[minmax(0,1fr)_280px]">
        <section class="min-w-0">
          <div class="mb-3 flex items-center justify-between gap-2"><h3 class="text-sm font-semibold">{{ t(`openAIQuality.stageNames.${stage}`) }}</h3><span class="rounded-md px-2 py-1 text-xs font-medium" :class="qualityOutcomeClass(current?.verdict || 'unknown')">{{ label(current?.verdict || 'unknown') }}</span></div>
          <template v-if="stage === 'pelican'">
            <div v-if="detail.run.evidence?.svg" class="aspect-[3/2] overflow-hidden rounded-xl bg-gray-50 dark:bg-dark-900"><QualitySVGPreview :svg="detail.run.evidence.svg" :alt="t('openAIQuality.pelicanArtwork')" /></div>
            <p v-else class="rounded-xl bg-gray-50 p-10 text-center text-sm text-gray-500 dark:bg-dark-900">{{ detail.response_expired ? t('openAIQuality.artworkExpired') : t('openAIQuality.artworkUnavailable') }}</p>
          </template>
          <div v-if="stage === 'review'" class="grid gap-3 sm:grid-cols-2">
            <div v-for="key in reviewKeys" :key="key" class="flex items-center justify-between gap-3 rounded-xl border border-gray-100 p-3 dark:border-dark-700"><span class="text-sm">{{ t(`openAIQuality.reviewChecks.${key}`) }}</span><span class="rounded-md px-2 py-1 text-xs" :class="qualityOutcomeClass(checkVerdict(key))">{{ label(checkVerdict(key)) }}</span></div>
          </div>
          <div v-if="stage === 'candy'" class="mb-4 flex items-center gap-6 rounded-xl bg-gray-50 p-5 dark:bg-dark-900"><div><p class="text-xs text-gray-500">{{ t('openAIQuality.answer') }}</p><p class="mt-2 font-mono text-3xl font-semibold">{{ current?.final_answer ?? '—' }}</p></div><span class="text-2xl text-gray-300">/</span><div><p class="text-xs text-gray-500">{{ t('openAIQuality.expectedAnswer') }}</p><p class="mt-2 font-mono text-3xl text-gray-400">21</p></div></div>
          <p class="my-3 text-xs text-gray-500">{{ label(current?.reason_code || detail.run.reason_code) }}</p>
          <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="mb-3 flex items-center justify-between"><h4 class="text-xs font-medium text-gray-500">{{ t('openAIQuality.finalResponse') }}</h4><button v-if="stage === 'pelican' && detail.run.evidence?.svg" class="text-xs text-primary-600" @click="downloadSVG">{{ t('openAIQuality.downloadSVG') }}</button></div>
            <p v-if="detail.response_expired" class="text-sm text-gray-500">{{ t('openAIQuality.responseExpired') }}</p>
            <p v-else-if="!detail.response_available" class="text-sm text-gray-500">{{ t('openAIQuality.responseUnavailable') }}</p>
            <pre v-else class="max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-4 text-xs leading-relaxed dark:bg-dark-900">{{ rawText || t('openAIQuality.responseUnavailable') }}</pre>
            <p v-if="current?.response_truncated" class="mt-2 text-xs text-amber-700">{{ t('openAIQuality.responseTruncated') }}</p>
          </div>
        </section>
        <aside class="space-y-5 rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <dl class="grid grid-cols-2 gap-4 text-sm">
            <div v-for="metric in metrics" :key="metric.label"><dt class="text-[11px] text-gray-500">{{ metric.label }}</dt><dd class="mt-1 font-mono font-medium">{{ metric.value }}</dd></div>
          </dl>
          <dl class="space-y-4 border-t border-gray-200 pt-4 text-xs dark:border-dark-700">
            <div><dt class="text-gray-500">{{ t('openAIQuality.sourceGroup') }}</dt><dd class="mt-1">{{ detail.run.input_snapshot?.source_group_name || groupName || t('openAIQuality.unavailableGroup') }}</dd></div>
            <div><dt class="text-gray-500">{{ t('openAIQuality.model') }}</dt><dd class="mt-1 break-all">{{ detail.run.input_snapshot?.condition.model_id }}<span v-if="detail.run.input_snapshot?.effective_upstream_model !== detail.run.input_snapshot?.condition.model_id"> → {{ detail.run.input_snapshot?.effective_upstream_model }}</span></dd></div>
            <div><dt class="text-gray-500">{{ t('openAIQuality.requestParameters') }}</dt><dd class="mt-1 leading-relaxed">{{ detail.run.input_snapshot?.condition.protocol }} · {{ detail.run.input_snapshot?.condition.transport }}<br />{{ label(detail.run.input_snapshot?.effective_reasoning_effort || '') }} · {{ label(detail.run.input_snapshot?.effective_service_tier || '') }}</dd></div>
            <div v-if="detail.run.input_snapshot?.reviewer"><dt class="text-gray-500">{{ t('openAIQuality.reviewerModel') }}</dt><dd class="mt-1 break-all">{{ detail.run.input_snapshot.reviewer.condition.model_id }}</dd></div>
            <div><dt class="text-gray-500">{{ t('openAIQuality.trigger') }}</dt><dd class="mt-1">{{ label(detail.run.trigger_source) }}</dd></div>
            <div><dt class="text-gray-500">{{ t('openAIQuality.stateParticipation') }}</dt><dd class="mt-1 leading-relaxed">{{ detail.run.state_applied ? t('openAIQuality.appliedToState') : label(detail.run.state_skip_reason || 'not_applied') }}</dd></div>
            <div><dt class="text-gray-500">{{ t('openAIQuality.totalSuiteLatency') }}</dt><dd class="mt-1 font-mono">{{ (detail.run.result.latency_ms / 1000).toFixed(2) }} s · {{ detail.run.result.dispatched_stages?.length ?? (detail.run.upstream_dispatched_at ? 1 : '—') }} {{ t('openAIQuality.modelCalls') }}</dd></div>
          </dl>
        </aside>
      </div>
    </template>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { QualityCheck, QualityRunDetail } from '@/api/admin/openaiAccountQuality'
import QualitySVGPreview from './QualitySVGPreview.vue'
import { qualityOutcomeClass, qualityRunOutcome } from './qualityPresentation'
const props = defineProps<{ detail: QualityRunDetail | null; groupName?: string }>()
defineEmits<{ (event: 'close'): void }>()
const { t, te } = useI18n()
const stages = ['candy', 'pelican', 'review'] as const
const reviewKeys = ['pelican', 'bicycle', 'riding', 'motion'] as const
const stage = ref<typeof stages[number]>('candy')
const dual = computed(() => props.detail?.run.input_snapshot?.condition.benchmark_id === 'dual')
watch(() => props.detail?.run.id, () => { stage.value = props.detail?.run.evidence?.svg ? 'pelican' : 'candy' }, { immediate: true })
function stageResult(key: typeof stages[number]): QualityCheck | undefined { const result = props.detail?.run.result; return result?.[key] || (key === 'candy' && !dual.value ? result : undefined) }
const current = computed(() => stageResult(stage.value))
const rawText = computed(() => props.detail?.run.evidence?.[stage.value] || (stage.value === 'candy' ? props.detail?.run.response_text : ''))
function label(value: string) { return value === 'incorrect' ? t('openAIQuality.failedChecks') : te(`openAIQuality.labels.${value}`) ? t(`openAIQuality.labels.${value}`) : value || '—' }
function formatDate(value: string) { return new Date(value).toLocaleString() }
function stageColor(value?: string) { return value === 'passed' ? 'bg-emerald-500' : value === 'incorrect' ? 'bg-red-500' : 'bg-amber-400' }
function checkVerdict(key: string) { const value = props.detail?.run.result.review?.checks?.[key]; return value === true ? 'passed' : value === false ? 'incorrect' : 'unknown' }
const metrics = computed(() => {
  const r = current.value
  const number = (value?: number | null) => value == null ? '—' : value.toLocaleString()
  return [
    { label: t('openAIQuality.latency'), value: r ? `${(r.latency_ms / 1000).toFixed(2)} s` : '—' },
    { label: 'TPS', value: r?.tokens_per_second?.toFixed(1) ?? '—' },
    { label: t('openAIQuality.inputTokens'), value: number(r?.input_tokens) },
    { label: t('openAIQuality.outputTokens'), value: number(r?.output_tokens) },
    { label: t('openAIQuality.reasoningTokens'), value: number(r?.reasoning_tokens) },
    { label: t('openAIQuality.totalTokens'), value: number(r?.total_tokens) },
    { label: t('openAIQuality.estimatedCost'), value: r?.estimated_cost_usd == null ? '—' : `$${r.estimated_cost_usd.toFixed(5)}` },
  ]
})
function downloadSVG() {
  const svg = props.detail?.run.evidence?.svg
  if (!svg) return
  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }))
  const link = document.createElement('a'); link.href = url; link.download = `pelican-${props.detail?.run.id}.svg`; link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}
</script>
