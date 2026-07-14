import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const lineProps = vi.hoisted(() => vi.fn())
vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    setup(props: unknown) {
      lineProps(props)
      return () => null
    },
  },
}))
vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  CategoryScale: {},
  LinearScale: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Legend: {},
  Filler: {},
}))

import SchedulerTTFTChart from '../SchedulerTTFTChart.vue'
import { createSchedulerTestI18n } from './testI18n'

describe('SchedulerTTFTChart', () => {
  it('renders separate P50 and P90 datasets in a stable plot', () => {
    mount(SchedulerTTFTChart, {
      props: {
        points: [
          { bucket: '2026-07-14T10:00:00Z', e2e_ttft_p50_ms: 1200, e2e_ttft_p90_ms: 2800 },
          { bucket: '2026-07-14T11:00:00Z', e2e_ttft_p50_ms: null, e2e_ttft_p90_ms: 3100 },
        ],
      }, global: { plugins: [createSchedulerTestI18n()] },
    })

    const props = lineProps.mock.calls[0][0] as { data: { datasets: Array<{ label: string; data: Array<number | null> }> } }
    expect(props.data.datasets.map((dataset) => dataset.label)).toEqual(['P50', 'P90'])
    expect(props.data.datasets[0].data).toEqual([1200, null])
  })
})
