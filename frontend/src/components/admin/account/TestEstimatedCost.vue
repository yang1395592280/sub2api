<template>
  <span class="block text-xs text-gray-500 dark:text-gray-400" :title="hint">
    {{ test.estimated_cost_usd != null ? t('pelican.estimatedCost', { amount: formatCost(test.estimated_cost_usd) }) : t('pelican.costUnavailable') }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PelicanTest } from '@/api/admin/accounts'
const props = defineProps<{ test: PelicanTest }>()
const { t } = useI18n()
const formatCost = (amount: number) => amount === 0 ? '$0' : `$${amount < 0.000001 ? amount.toPrecision(3) : amount.toFixed(6)}`
const hint = computed(() => props.test.estimated_cost_usd != null
  ? t('pelican.costBasis', { model: props.test.cost_model_id, rate: props.test.upstream_rate_multiplier })
  : t('pelican.costUnavailableHint'))
</script>
