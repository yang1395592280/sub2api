<template>
  <Teleport to="body">
    <div v-if="open && item" class="fixed inset-0 z-50 flex justify-end bg-black/35" @click.self="emit('close')">
      <aside class="h-full w-full max-w-lg overflow-y-auto bg-white shadow-xl dark:bg-dark-900" :aria-label="t('admin.openaiAutoScheduler.ranking.drawerLabel')">
        <header class="sticky top-0 z-10 flex items-start justify-between border-b border-gray-200 bg-white px-5 py-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="min-w-0"><h2 class="truncate text-base font-semibold text-gray-950 dark:text-white">{{ item.account_name }}</h2><p class="text-xs text-gray-500">#{{ item.account_id }} · {{ item.partition.model_family }} · {{ item.partition.endpoint }}</p></div>
          <button type="button" class="rounded p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-800" :title="t('admin.openaiAutoScheduler.actions.close')" @click="emit('close')"><Icon name="x" size="sm" /></button>
        </header>
        <div class="space-y-6 px-5 py-5">
          <section class="grid grid-cols-3 gap-3 border-b border-gray-100 pb-5 text-center dark:border-dark-800">
            <div><span class="block text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.ranking.utility') }}</span><strong class="text-xl">{{ item.utility_score.toFixed(1) }}</strong></div>
            <div><span class="block text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.ranking.target') }}</span><strong class="text-xl text-primary-600">{{ percent(item.target_share) }}</strong></div>
            <div><span class="block text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.ranking.actual') }}</span><strong class="text-xl">{{ percent(item.actual_share) }}</strong></div>
          </section>
          <section>
            <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.ranking.scoreBreakdown') }}</h3>
            <div class="space-y-3">
              <div v-for="score in scores" :key="score.key" class="grid grid-cols-[92px_1fr_44px] items-center gap-3 text-xs">
                <span class="text-gray-600 dark:text-dark-300">{{ score.label }}</span>
                <div class="h-2 overflow-hidden bg-gray-100 dark:bg-dark-700"><div class="h-full bg-primary-500" :style="{ width: `${Math.max(0, Math.min(100, score.value * 100))}%` }" /></div>
                <span class="text-right font-medium">{{ (score.value * 100).toFixed(0) }}</span>
              </div>
            </div>
          </section>
          <section class="border-t border-gray-100 pt-5 dark:border-dark-800">
            <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.openaiAutoScheduler.ranking.runtimeFacts') }}</h3>
            <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
              <div><dt class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.health.predictedTTFT') }}</dt><dd>{{ duration(item.predicted_ttft_ms) }}</dd></div>
              <div><dt class="text-xs text-gray-500">P90</dt><dd>{{ duration(item.ttft_p90_ms) }}</dd></div>
              <div><dt class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.health.totalErrorRate') }}</dt><dd>{{ percent(item.error_rate) }}</dd></div>
              <div><dt class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.health.loadCapacity') }}</dt><dd>{{ item.load_inflight }} / {{ item.load_capacity }}</dd></div>
              <div><dt class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.ranking.confidence') }}</dt><dd>{{ item.confidence }}</dd></div>
              <div><dt class="text-xs text-gray-500">{{ t('admin.openaiAutoScheduler.health.samples') }}</dt><dd>{{ item.real_sample_count }} / {{ item.probe_sample_count }}</dd></div>
            </dl>
          </section>
        </div>
      </aside>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import type { OpenAISchedulerRankingItem } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ open: boolean; item: OpenAISchedulerRankingItem | null }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const scores = computed(() => props.item ? [
  { key: 'latency', label: t('admin.openaiAutoScheduler.ranking.scores.latency'), value: props.item.latency_score },
  { key: 'reliability', label: t('admin.openaiAutoScheduler.ranking.scores.reliability'), value: props.item.reliability_score },
  { key: 'cost', label: t('admin.openaiAutoScheduler.ranking.scores.cost'), value: props.item.cost_score },
  { key: 'capacity', label: t('admin.openaiAutoScheduler.ranking.scores.capacity'), value: props.item.capacity_score },
  { key: 'quota', label: t('admin.openaiAutoScheduler.ranking.scores.quota'), value: props.item.quota_score },
  { key: 'priority', label: t('admin.openaiAutoScheduler.ranking.scores.priority'), value: props.item.priority_score },
] : [])
function percent(value: number): string { return `${(Math.max(0, value || 0) * 100).toFixed(1)}%` }
function duration(value?: number | null): string { return !value ? '—' : value >= 1000 ? `${(value / 1000).toFixed(2)}s` : `${Math.round(value)}ms` }
</script>
