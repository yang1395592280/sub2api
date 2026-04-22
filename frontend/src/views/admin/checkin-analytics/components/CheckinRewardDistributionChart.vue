<template>
  <div class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.checkinAnalytics.charts.rewardDistribution') }}
      </h3>
    </div>

    <div v-if="loading" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="buckets.length === 0" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.checkinAnalytics.empty') }}
    </div>
    <div v-else class="h-72">
      <Bar :data="chartData" :options="options" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import type { AdminCheckinRewardBucket } from '@/api/admin/checkins'

defineOptions({ name: 'CheckinRewardDistributionChart' })

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend)

const props = withDefaults(defineProps<{
  buckets: AdminCheckinRewardBucket[]
  loading?: boolean
}>(), {
  loading: false
})

const { t } = useI18n()

const chartData = computed(() => ({
  labels: props.buckets.map((bucket) => bucket.label),
  datasets: [
    {
      label: t('admin.checkinAnalytics.charts.checkinCount'),
      data: props.buckets.map((bucket) => bucket.count),
      backgroundColor: '#f59e0b'
    }
  ]
}))

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false }
  }
}))
</script>
