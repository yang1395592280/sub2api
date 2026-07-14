<template>
  <div data-testid="scheduler-ttft-chart" class="h-64 min-h-64 w-full">
    <Line v-if="points.length" :data="chartData" :options="chartOptions" />
    <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-dark-400">
      {{ t('admin.openaiAutoScheduler.overview.noTrend') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  Filler,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  Tooltip,
} from 'chart.js'
import { Line } from 'vue-chartjs'
import type { OpenAISchedulerTrendPoint } from '@/api/admin/openaiAutoScheduler'
import { useI18n } from 'vue-i18n'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()

const props = defineProps<{ points: OpenAISchedulerTrendPoint[] }>()

const chartData = computed(() => ({
  labels: props.points.map((point) =>
    new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(
      new Date(point.bucket)
    )
  ),
  datasets: [
    {
      label: 'P50',
      data: props.points.map((point) => point.e2e_ttft_p50_ms),
      borderColor: '#059669',
      backgroundColor: 'rgba(5, 150, 105, 0.08)',
      fill: true,
      tension: 0.28,
      pointRadius: 0,
      pointHitRadius: 10,
      spanGaps: true,
    },
    {
      label: 'P90',
      data: props.points.map((point) => point.e2e_ttft_p90_ms),
      borderColor: '#d97706',
      backgroundColor: 'transparent',
      fill: false,
      tension: 0.28,
      pointRadius: 0,
      pointHitRadius: 10,
      spanGaps: true,
    },
  ],
}))

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  animation: false as const,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'end' as const,
      labels: { usePointStyle: true, boxWidth: 8, boxHeight: 8 },
    },
    tooltip: {
      callbacks: {
        label: (context: { dataset: { label?: string }; parsed: { y: number | null } }) =>
          `${context.dataset.label || ''}: ${context.parsed.y == null ? '—' : formatDuration(context.parsed.y)}`,
      },
    },
  },
  scales: {
    x: { grid: { display: false }, ticks: { maxTicksLimit: 8 } },
    y: {
      beginAtZero: true,
      ticks: { callback: (value: string | number) => formatDuration(Number(value)) },
    },
  },
}

function formatDuration(value: number): string {
  if (value >= 1000) return `${(value / 1000).toFixed(value >= 10_000 ? 1 : 2)}s`
  return `${Math.round(value)}ms`
}
</script>
