<template>
  <div class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.checkinAnalytics.charts.trend') }}
      </h3>
    </div>

    <div v-if="loading" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="points.length === 0" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.checkinAnalytics.empty') }}
    </div>
    <div v-else class="space-y-4">
      <div class="h-72">
        <Line :data="chartData" :options="options" />
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="point in points"
          :key="point.date"
          type="button"
          class="rounded-full border border-gray-200 px-3 py-1 text-xs font-medium text-gray-600 transition hover:border-primary-300 hover:text-primary-600 dark:border-dark-700 dark:text-gray-300 dark:hover:border-primary-700 dark:hover:text-primary-400"
          @click="emit('select-date', point.date)"
        >
          {{ point.date }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, Filler, Legend, LineElement, LinearScale, PointElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { AdminCheckinTrendPoint } from '@/api/admin/checkins'

defineOptions({ name: 'CheckinTrendChart' })

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = withDefaults(defineProps<{
  points: AdminCheckinTrendPoint[]
  loading?: boolean
}>(), {
  loading: false
})

const emit = defineEmits<{
  'select-date': [date: string]
}>()

const { t } = useI18n()

const chartData = computed(() => ({
  labels: props.points.map((point) => point.date),
  datasets: [
    {
      label: t('admin.checkinAnalytics.charts.checkinCount'),
      data: props.points.map((point) => point.checkin_count),
      borderColor: '#3b82f6',
      backgroundColor: '#3b82f620',
      fill: true,
      tension: 0.35,
      pointRadius: 3,
      pointHoverRadius: 5,
      yAxisID: 'y'
    },
    {
      label: t('admin.checkinAnalytics.charts.rewardAmount'),
      data: props.points.map((point) => point.reward_amount),
      borderColor: '#10b981',
      backgroundColor: '#10b98120',
      fill: false,
      tension: 0.35,
      pointRadius: 3,
      pointHoverRadius: 5,
      yAxisID: 'y1'
    }
  ]
}))

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { position: 'top' as const }
  },
  scales: {
    y: {
      type: 'linear' as const,
      display: true,
      position: 'left' as const
    },
    y1: {
      type: 'linear' as const,
      display: true,
      position: 'right' as const,
      grid: { drawOnChartArea: false }
    }
  }
}))
</script>
