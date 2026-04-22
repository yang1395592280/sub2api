<template>
  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
    <div
      v-for="item in items"
      :key="item.label"
      class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
    >
      <div class="text-xs uppercase tracking-[0.16em] text-gray-400">
        {{ item.label }}
      </div>
      <div class="mt-3 text-2xl font-semibold text-gray-900 dark:text-white">
        {{ loading ? '--' : item.value }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatCurrency, formatNumber } from '@/utils/format'
import type { AdminCheckinOverview } from '@/api/admin/checkins'

defineOptions({ name: 'CheckinAnalyticsOverviewCards' })

const props = withDefaults(defineProps<{
  overview?: AdminCheckinOverview | null
  loading?: boolean
}>(), {
  overview: null,
  loading: false
})

const { t } = useI18n()

const items = computed(() => [
  {
    label: t('admin.checkinAnalytics.cards.totalCheckins'),
    value: formatNumber(props.overview?.total_checkins ?? 0)
  },
  {
    label: t('admin.checkinAnalytics.cards.totalRewardAmount'),
    value: formatCurrency(props.overview?.total_reward_amount ?? 0)
  },
  {
    label: t('admin.checkinAnalytics.cards.todayCheckins'),
    value: formatNumber(props.overview?.today_checkins ?? 0)
  },
  {
    label: t('admin.checkinAnalytics.cards.avgRewardAmount'),
    value: formatCurrency(props.overview?.avg_reward_amount ?? 0)
  }
])
</script>
