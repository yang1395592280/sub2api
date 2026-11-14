<template>
  <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
    <span>{{ tokens === null ? t('admin.accounts.pelican.tokensUnavailable') : `${tokens.toLocaleString()} Tokens` }}</span>
    <span :title="cost === null ? t('pelican.costUnavailableHint') : undefined">{{ cost === null ? t('pelican.costUnavailable') : t('pelican.library.totalCost', { amount: `$${cost.toFixed(6)}` }) }}</span>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PelicanTest } from '@/api/admin/accounts'
const props = defineProps<{ tests: PelicanTest[] }>()
const { t } = useI18n()
const tokens = computed(() => props.tests.length && props.tests.every(test => test.total_tokens != null) ? props.tests.reduce((sum, test) => sum + test.total_tokens!, 0) : null)
const cost = computed(() => props.tests.length && props.tests.every(test => test.estimated_cost_usd != null) ? props.tests.reduce((sum, test) => sum + test.estimated_cost_usd!, 0) : null)
</script>
