<template>
  <div class="space-y-5">
    <section v-if="detail.candy" class="rounded-2xl border border-gray-200 p-5 dark:border-dark-700">
      <h4 class="text-sm font-semibold">{{ t('openAIQuality.candyQuestion') }}</h4>
      <p class="mt-3 whitespace-pre-line text-sm leading-7 text-gray-600 dark:text-dark-200">{{ detail.candy.question }}</p>
      <details class="mt-4 border-t border-gray-100 pt-3 dark:border-dark-700"><summary class="cursor-pointer text-xs text-gray-500 dark:text-dark-300">{{ t('openAIQuality.fullPrompt') }} · v{{ detail.run.input_snapshot?.condition.benchmark_version }}</summary><pre class="mt-3 whitespace-pre-wrap break-words text-xs leading-relaxed text-gray-500 dark:text-dark-300">{{ detail.candy.prompt }}</pre></details>
    </section>
    <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-gray-50 px-5 py-4 dark:bg-dark-900">
      <div><p class="text-xs text-gray-500 dark:text-dark-300">{{ t('openAIQuality.modelAnswer') }}</p><p class="mt-1 font-mono text-4xl font-semibold">{{ answer ?? '—' }}<span class="ml-2 font-sans text-sm font-normal text-gray-400">{{ t('openAIQuality.candiesUnit') }}</span></p></div>
      <div class="text-right"><p class="text-xs text-gray-500 dark:text-dark-300">{{ t('openAIQuality.expectedAnswer') }}</p><p class="mt-1 font-mono text-2xl text-gray-400">21</p></div>
    </div>
    <section>
      <h4 class="text-sm font-semibold">{{ t('openAIQuality.modelExplanation') }}</h4>
      <p class="mt-1 text-xs leading-relaxed text-gray-400">{{ t('openAIQuality.explanationHelp') }}</p>
      <div v-if="html" class="quality-explanation mt-4 text-sm leading-7 text-gray-700 dark:text-dark-200" v-html="html" />
      <p v-else class="mt-3 rounded-xl bg-gray-50 p-4 text-sm leading-relaxed text-gray-500 dark:text-dark-300 dark:bg-dark-900">{{ emptyReason }}</p>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import type { QualityRunDetail } from '@/api/admin/openaiAccountQuality'
const props = defineProps<{ detail: QualityRunDetail; answer?: number | null }>()
const { t } = useI18n()
// Model-authored Markdown is display-only. No links, images, styles or embedded resources.
const html = computed(() => {
  if (!props.detail.response_available || props.detail.response_expired || !props.detail.candy?.explanation) return ''
  return DOMPurify.sanitize(marked.parse(props.detail.candy.explanation, { async: false }), {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'del', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'hr'],
    ALLOWED_ATTR: [],
  })
})
const emptyReason = computed(() => t(props.detail.response_expired ? 'openAIQuality.responseExpired' : !props.detail.response_available ? 'openAIQuality.responseUnavailable' : !props.detail.candy?.explanation_requested ? 'openAIQuality.legacyExplanationMissing' : 'openAIQuality.explanationMissing'))
</script>
<style scoped>
.quality-explanation { overflow-wrap: anywhere; overflow-x: auto; }
.quality-explanation :deep(h1), .quality-explanation :deep(h2), .quality-explanation :deep(h3), .quality-explanation :deep(h4) { font-size: 1rem; font-weight: 600; margin: 1.25rem 0 .5rem; }
.quality-explanation :deep(p), .quality-explanation :deep(ul), .quality-explanation :deep(ol) { margin: .65rem 0; }
.quality-explanation :deep(ul) { list-style: disc; padding-left: 1.5rem; }
.quality-explanation :deep(ol) { list-style: decimal; padding-left: 1.5rem; }
.quality-explanation :deep(table) { width: 100%; border-collapse: collapse; margin: 1rem 0; }
.quality-explanation :deep(th), .quality-explanation :deep(td) { border: 1px solid rgb(156 163 175 / .3); padding: .5rem .75rem; text-align: left; }
.quality-explanation :deep(th) { background: rgb(156 163 175 / .1); font-weight: 600; }
.quality-explanation :deep(pre) { overflow-x: auto; padding: 1rem; border-radius: .75rem; background: rgb(156 163 175 / .1); }
.quality-explanation :deep(blockquote) { border-left: 3px solid rgb(156 163 175 / .4); padding-left: 1rem; }
</style>
