import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import BusinessAnalyticsView from '../BusinessAnalyticsView.vue'

const {
  getOverview,
  getGroups,
  getChannels,
  getPriceChangeImpact,
  getRecords,
} = vi.hoisted(() => ({
  getOverview: vi.fn(),
  getGroups: vi.fn(),
  getChannels: vi.fn(),
  getPriceChangeImpact: vi.fn(),
  getRecords: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    businessAnalytics: {
      getOverview,
      getGroups,
      getChannels,
      getPriceChangeImpact,
      getRecords,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
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

const overview = {
  start_date: '2026-06-01',
  end_date: '2026-06-07',
  revenue: 120,
  channel_cost: 72,
  gross_profit: 48,
  profit_margin: 0.4,
  active_users: 9,
  requests: 300,
  revenue_per_active_user: 13.33,
  profit_per_active_user: 5.33,
  active_api_keys: 7,
  total_tokens: 120000,
  missing_channel_price_records: 2,
  trend: [
    {
      date: '2026-06-01',
      requests: 100,
      active_users: 3,
      revenue: 40,
      channel_cost: 20,
      gross_profit: 20,
      profit_margin: 0.5,
    },
  ],
}

function mountView() {
  return mount(BusinessAnalyticsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DateRangePicker: {
          props: ['startDate', 'endDate'],
          emits: ['update:startDate', 'update:endDate', 'change'],
          template: `
            <div data-test="date-range-picker">
              <button
                data-test="change-range"
                @click="$emit('update:startDate', '2026-06-10'); $emit('update:endDate', '2026-06-12'); $emit('change')"
              >
                change range
              </button>
            </div>
          `,
        },
        Pagination: true,
        Icon: true,
      },
    },
  })
}

describe('BusinessAnalyticsView', () => {
  beforeEach(() => {
    getOverview.mockReset()
    getGroups.mockReset()
    getChannels.mockReset()
    getPriceChangeImpact.mockReset()
    getRecords.mockReset()

    getOverview.mockResolvedValue(overview)
    getGroups.mockResolvedValue([
      {
        group_id: 10,
        group_name: 'Team A',
        platform: 'openai',
        current_rate_multiplier: 1.2,
        avg_rate_multiplier: 1.125,
        revenue: 80,
        channel_cost: 50,
        gross_profit: 30,
        profit_margin: 0.375,
        active_users: 5,
        requests: 180,
        revenue_per_active_user: 16,
        profit_per_active_user: 6,
        active_api_keys: 4,
        total_tokens: 76000,
        previous_revenue: 70,
        previous_gross_profit: 20,
        revenue_change_rate: 0.1429,
        gross_profit_change_rate: 0.5,
      },
    ])
    getChannels.mockResolvedValue([
      {
        account_id: 20,
        account_name: 'Channel A',
        channel_id: 200,
        platform: 'openai',
        status: 'active',
        current_channel_price: 0.7,
        avg_channel_price: 0.875,
        balance_status: 'ok',
        revenue: 90,
        channel_cost: 45,
        gross_profit: 45,
        profit_margin: 0.5,
        active_users: 6,
        requests: 210,
        revenue_per_active_user: 15,
        profit_per_active_user: 7.5,
        active_api_keys: 5,
        total_tokens: 88000,
        missing_channel_price_records: 1,
      },
    ])
    getPriceChangeImpact.mockResolvedValue({
      group_id: 10,
      change_date: '2026-06-05',
      before_revenue: 30,
      after_revenue: 42,
      revenue_delta: 12,
      before_gross_profit: 10,
      after_gross_profit: 18,
      gross_profit_delta: 8,
      change_at: '2026-06-05T00:00:00Z',
    })
    getRecords.mockResolvedValue({
      items: [
        {
          id: 1,
          created_at: '2026-06-06T08:00:00Z',
          user_id: 3,
          user_email: 'u@example.com',
          api_key_id: 4,
          api_key_name: 'prod-key',
          group_id: 10,
          group_name: 'Team A',
          account_id: 20,
          account_name: 'Channel A',
          model: 'gpt-5-mini',
          requests: 2,
          total_tokens: 900,
          revenue: 1.2,
          channel_cost: 0.7,
          gross_profit: 0.5,
          channel_price_snapshot: 0.875,
          channel_price_snapshot_missing: false,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
  })

  it('loads overview by default', async () => {
    const wrapper = mountView()

    await flushPromises()

    expect(getOverview).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="business-analytics-page"]').exists()).toBe(true)
    expect(wrapper.get('[data-test="metric-revenue"]').text()).toContain('$120.00')
    expect(wrapper.text()).toContain('admin.businessAnalytics.tabs.overview')
  })

  it('reloads the active tab when filters change', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="change-range"]').trigger('click')
    await flushPromises()

    expect(getOverview).toHaveBeenCalledTimes(2)
    expect(getOverview).toHaveBeenLastCalledWith(
      expect.objectContaining({
        start_date: '2026-06-10',
        end_date: '2026-06-12',
      })
    )
  })

  it('loads groups, channels, records, and price impact APIs when switching tabs', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-groups"]').trigger('click')
    await flushPromises()
    expect(getGroups).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Team A')
    expect(wrapper.text()).toContain('admin.businessAnalytics.columns.averageRate')
    expect(wrapper.text()).toContain('1.1250')

    await wrapper.get('[data-test="tab-channels"]').trigger('click')
    await flushPromises()
    expect(getChannels).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Channel A')
    expect(wrapper.text()).toContain('admin.businessAnalytics.columns.averagePrice')
    expect(wrapper.text()).toContain('0.8750')

    await wrapper.get('[data-test="tab-records"]').trigger('click')
    await flushPromises()
    expect(getRecords).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('u@example.com')

    await wrapper.get('[data-test="tab-priceImpact"]').trigger('click')
    await flushPromises()
    expect(getPriceChangeImpact).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="price-impact-delta"]').text()).toContain('$12.00')
  })

  it('selects a loaded group before querying price impact', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-priceImpact"]').trigger('click')
    await flushPromises()

    const groupSelect = wrapper.get('[data-test="price-impact-group-select"]')
    expect(groupSelect.text()).toContain('Team A')

    await groupSelect.setValue('10')
    await flushPromises()

    expect(getPriceChangeImpact).toHaveBeenLastCalledWith(
      expect.objectContaining({
        group_id: 10,
      })
    )
  })

  it('renders records empty and approximate snapshot states', async () => {
    getOverview.mockResolvedValue({ ...overview, missing_channel_price_records: 3 })
    getRecords.mockResolvedValueOnce({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="tab-records"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="records-empty"]').text()).toContain(
      'admin.businessAnalytics.empty.records'
    )
    expect(wrapper.text()).toContain('admin.businessAnalytics.historicalApproximation')

    getRecords.mockResolvedValueOnce({
      items: [
        {
          id: 2,
          created_at: '2026-06-06T09:00:00Z',
          user_id: 5,
          user_email: 'missing@example.com',
          api_key_id: 8,
          api_key_name: 'missing-price',
          group_id: 10,
          group_name: 'Team A',
          account_id: 20,
          account_name: 'Channel A',
          model: 'gpt-5-mini',
          requests: 1,
          total_tokens: 300,
          revenue: 0.8,
          channel_cost: 0,
          gross_profit: 0.8,
          channel_price_snapshot: null,
          channel_price_snapshot_missing: true,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })

    await wrapper.get('[data-test="reload-records"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="records-empty"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="records-approx-summary"]').text()).toContain(
      'admin.businessAnalytics.historicalApproximation'
    )
    expect(wrapper.find('[data-test="record-approximate-2"]').exists()).toBe(true)
  })

  it('renders record channel price snapshot when present', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="tab-records"]').trigger('click')
    await flushPromises()

    const snapshot = wrapper.get('[data-test="record-channel-price-snapshot-1"]')
    expect(snapshot.text()).toContain('0.8750')
    expect(snapshot.text()).not.toContain('admin.businessAnalytics.snapshotUnavailable')
    expect(wrapper.find('[data-test="record-approximate-1"]').exists()).toBe(false)
  })

  it('hides records approximation hint when no channel price records are missing', async () => {
    getOverview.mockResolvedValue({ ...overview, missing_channel_price_records: 0 })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="tab-records"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="records-approx-summary"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.businessAnalytics.recordsApproxHint')
  })
})
