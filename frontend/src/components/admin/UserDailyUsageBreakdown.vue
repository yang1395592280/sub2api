<template>
  <div class="space-y-1 tabular-nums">
    <div class="font-semibold text-gray-900 dark:text-gray-100">${{ money(usage.total_cost) }}</div>
    <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.userDailyStats.paidCost') }} <span class="text-gray-800 dark:text-gray-200">${{ money(usage.paid_cost) }}</span></div>
    <div class="text-xs text-emerald-700 dark:text-emerald-400">{{ t('admin.userDailyStats.welfareCost') }} ${{ money(usage.welfare_cost) }}</div>
    <details class="text-xs text-gray-500 dark:text-gray-400">
      <summary class="cursor-pointer py-1">{{ t('admin.userDailyStats.breakdown') }}</summary>
      <div class="space-y-1 py-1">
        <div>{{ t('admin.userDailyStats.checkin') }} ${{ money(usage.checkin_cost) }}</div>
        <div>{{ t('admin.userDailyStats.liyu') }} ${{ money(usage.liyu_cost) }}</div>
        <div>{{ t('admin.userDailyStats.gift') }} ${{ money(usage.gift_cost) }}</div>
        <div>{{ t('admin.userDailyStats.subscription') }} ${{ money(usage.subscription_cost) }}</div>
        <div :class="usage.unclassified_cost !== 0 ? 'text-amber-700 dark:text-amber-400' : ''">{{ t('admin.userDailyStats.unclassified') }} ${{ money(usage.unclassified_cost) }}</div>
      </div>
    </details>
    <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.userDailyStats.traffic', { requests: usage.requests.toLocaleString(), tokens: usage.tokens.toLocaleString() }) }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { DailyUsage } from '@/api/admin/userDailyStats'
import { formatCostFixed } from '@/utils/format'

defineProps<{ usage: DailyUsage }>()
const { t } = useI18n()
const money = (value: number) => formatCostFixed(value, 8)
</script>
