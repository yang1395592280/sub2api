import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OpenAIHealthView from '../OpenAIHealthView.vue'

const appStoreMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      getAll: vi.fn().mockResolvedValue([
        { id: 33, name: 'GPT Plus', platform: 'openai', status: 'active' },
      ]),
    },
    openaiHealth: {
      getOverview: vi.fn().mockResolvedValue({
        time_window: '6h',
        window_start: '2026-06-16T03:50:00Z',
        window_end: '2026-06-16T09:50:00Z',
        total_monitors: 1,
        healthy_monitors: 1,
        degraded_monitors: 0,
        failed_monitors: 0,
        average_availability_pct: 93,
        average_first_token_ms: 1186,
        items: [
          {
            id: 7,
            name: 'Kedaya',
            endpoint: 'https://sub.kedaya.xyz',
            group_name: 'GPT Plus',
            primary_model: 'gpt-5.1',
            enabled: true,
            latest_status: 'operational',
            latest_first_token_ms: 1186,
            latest_ping_latency_ms: 3,
            last_checked_at: '2026-06-16T09:50:00Z',
            total_checks: 30,
            operational_checks: 28,
            failed_checks: 2,
            error_checks: 0,
            availability_pct: 93,
            avg_first_token_ms: 1186,
            p95_first_token_ms: 1600,
            avg_ping_latency_ms: 3,
            trend: [
              { status: 'operational', latency_ms: 1100, ping_latency_ms: 3, checked_at: '2026-06-16T09:49:00Z' },
              { status: 'failed', latency_ms: null, ping_latency_ms: 4, checked_at: '2026-06-16T09:50:00Z' },
            ],
          },
        ],
      }),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreMocks,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('OpenAIHealthView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads scheduler summary and OpenAI monitor rows', async () => {
    const { adminAPI } = await import('@/api/admin')

    const wrapper = mount(OpenAIHealthView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
          },
          DataTable: {
            props: ['data'],
            template: `
              <div>
                <div v-for="row in data" :key="row.id">
                  <slot name="cell-name" :row="row" />
                  <slot name="cell-group_name" :row="row" />
                  <slot name="cell-primary_status" :row="row" />
                  <slot name="cell-primary_latency_ms" :row="row" />
                  <slot name="cell-availability_7d" :row="row" />
                  <slot name="cell-trend" :row="row" />
                  <slot name="cell-last_checked_at" :row="row" />
                </div>
              </div>
            `,
          },
          Pagination: true,
          EmptyState: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>',
          },
        },
      },
    })

    await flushPromises()

    expect(adminAPI.groups.getAll).toHaveBeenCalledWith('openai')
    expect(adminAPI.openaiHealth.getOverview).toHaveBeenCalledWith(expect.objectContaining({ window: '6h' }), expect.any(Object))
    expect(wrapper.text()).toContain('Kedaya')
    expect(wrapper.text()).toContain('https://sub.kedaya.xyz')
    expect(wrapper.text()).toContain('93%')
    expect(wrapper.text()).toContain('1186 ms')
  })
})
