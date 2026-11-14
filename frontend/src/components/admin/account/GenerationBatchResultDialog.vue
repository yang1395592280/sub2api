<template>
  <BaseDialog :show="show && tests.length > 1" :title="`${accountName} · ${t('pelican.library.batch')}`" width="extra-wide" @close="emit('close')">
    <div class="mb-4 flex flex-wrap gap-4 text-sm">
      <span class="font-semibold">{{ t(intelligence ? 'pelican.library.batchSummary' : 'pelican.library.artworkSummary', summary) }}</span>
      <span class="text-gray-500">{{ t('pelican.library.pendingSummary', summary) }}</span>
    </div>
    <GenerationTestTotals :tests="tests" class="mb-4" />
    <div class="grid gap-3" :class="intelligence ? '' : 'sm:grid-cols-2'">
      <button v-for="test in tests" :key="test.id" type="button" class="overflow-hidden rounded-xl border border-gray-200 text-left hover:border-primary-400 dark:border-dark-600" @click="selectedId = test.id">
        <PelicanInlinePreview v-if="!intelligence && test.status === 'previewable'" :test-id="test.id" responsive class="!rounded-none !border-0" />
        <div class="space-y-2 p-4">
          <div class="flex justify-between gap-3"><strong class="break-words">{{ test.question_name || `#${test.id}` }}</strong><span class="shrink-0 text-xs" :class="['correct','previewable'].includes(test.status) ? 'text-emerald-600' : 'text-amber-600'">{{ t(`${intelligence ? 'pelican.intelligence.status' : 'pelican.cardStatus'}.${test.status}`) }}</span></div>
          <p v-if="intelligence" class="text-sm"><b class="font-mono">{{ test.answer || '—' }}</b><span class="ml-4 text-gray-500">{{ t('pelican.library.expected', { answer: test.expected_answer || '20' }) }}</span></p>
          <p class="text-xs text-gray-500">{{ test.model_id }} · {{ test.total_tokens?.toLocaleString() ?? '—' }} Tokens</p>
          <TestEstimatedCost :test="test" />
        </div>
      </button>
    </div>
  </BaseDialog>
  <PelicanResultDialog :show="show && detailId !== null" :test-id="detailId" :account-name="accountName" :intelligence="intelligence" @close="closeDetail" />
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PelicanTest } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PelicanResultDialog from './PelicanResultDialog.vue'
import PelicanInlinePreview from './PelicanInlinePreview.vue'
import TestEstimatedCost from './TestEstimatedCost.vue'
import GenerationTestTotals from './GenerationTestTotals.vue'
import { summarizeGenerationTests } from '@/utils/generationTests'
const props = defineProps<{ show: boolean; tests: PelicanTest[]; accountName: string; intelligence?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const selectedId = ref<number | null>(null)
const detailId = computed(() => props.tests.length === 1 ? props.tests[0].id : selectedId.value)
const summary = computed(() => summarizeGenerationTests(props.tests, props.intelligence ? 'intelligence' : 'pelican'))
watch(() => props.show, () => { selectedId.value = null })
function closeDetail() { if (props.tests.length === 1) emit('close'); else selectedId.value = null }
</script>
