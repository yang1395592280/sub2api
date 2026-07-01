import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
  },
}))

import businessAnalyticsAPI, {
  type BusinessAnalyticsFilter,
  type BusinessRecordsParams,
} from '@/api/admin/businessAnalytics'

describe('business analytics admin api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    put.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('loads overview through the business analytics overview endpoint', async () => {
    const overview = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
      revenue: 120,
      channel_cost: 80,
      gross_profit: 40,
      profit_margin: 0.3333,
      active_users: 12,
      requests: 300,
      revenue_per_active_user: 10,
      profit_per_active_user: 3.3333,
      trend: [],
    }
    const params: BusinessAnalyticsFilter = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
      granularity: 'day',
    }
    get.mockResolvedValueOnce({ data: overview })

    await expect(businessAnalyticsAPI.getOverview(params)).resolves.toEqual(overview)

    expect(get).toHaveBeenCalledWith('/admin/business-analytics/overview', { params })
  })

  it('loads groups with analytics filters', async () => {
    const groups = [{ group_id: 10, group_name: 'paid', revenue: 100, avg_rate_multiplier: 1.125 }]
    const params: BusinessAnalyticsFilter = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
      group_id: 10,
      account_id: 20,
      platform: 'openai',
    }
    get.mockResolvedValueOnce({ data: groups })

    await expect(businessAnalyticsAPI.getGroups(params)).resolves.toEqual(groups)

    expect(get).toHaveBeenCalledWith('/admin/business-analytics/groups', { params })
  })

  it('preserves aggregate average price fields and row snapshot flags', async () => {
    const groups = [{ group_id: 10, group_name: 'paid', avg_rate_multiplier: 1.125 }]
    const channels = [{ account_id: 20, account_name: 'main', avg_channel_price: 0.875 }]
    const records = {
      items: [
        {
          id: 1,
          rate_multiplier: 1.125,
          channel_price_snapshot: 0.875,
          channel_price_snapshot_missing: false,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    }
    const params: BusinessAnalyticsFilter = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
      granularity: 'week',
    }
    get.mockResolvedValueOnce({ data: groups })
    get.mockResolvedValueOnce({ data: channels })
    get.mockResolvedValueOnce({ data: records })

    await expect(businessAnalyticsAPI.getGroups(params)).resolves.toEqual(groups)
    await expect(businessAnalyticsAPI.getChannels(params)).resolves.toEqual(channels)
    await expect(businessAnalyticsAPI.getRecords(params)).resolves.toEqual(records)
  })

  it('preserves expanded price impact metrics', async () => {
    const impact = {
      group_id: 10,
      change_date: '2026-06-05',
      before_requests: 10,
      after_requests: 12,
      before_active_users: 3,
      after_active_users: 4,
      before_revenue: 30,
      after_revenue: 42,
      revenue_delta: 12,
      before_channel_cost: 18,
      after_channel_cost: 21,
      before_gross_profit: 12,
      after_gross_profit: 21,
      gross_profit_delta: 9,
      before_profit_margin: 0.4,
      after_profit_margin: 0.5,
      before_avg_rate_multiplier: 1.25,
      after_avg_rate_multiplier: 1.5,
      new_users: 2,
      lost_users: 1,
    }
    get.mockResolvedValueOnce({ data: impact })

    await expect(
      businessAnalyticsAPI.getPriceChangeImpact({
        group_id: 10,
        change_date: '2026-06-05',
      })
    ).resolves.toEqual(impact)
  })

  it('loads records with pagination params', async () => {
    const records = {
      items: [],
      total: 0,
      page: 2,
      page_size: 50,
    }
    const params: BusinessRecordsParams = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
      page: 2,
      page_size: 50,
      account_id: 88,
    }
    get.mockResolvedValueOnce({ data: records })

    await expect(businessAnalyticsAPI.getRecords(params)).resolves.toEqual(records)

    expect(get).toHaveBeenCalledWith('/admin/business-analytics/records', { params })
  })

  it('uses nested group and channel routes with filters', async () => {
    const channels = [{ account_id: 20, account_name: 'openai-main' }]
    const groups = [{ group_id: 10, group_name: 'paid' }]
    const params: BusinessAnalyticsFilter = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
    }
    get.mockResolvedValueOnce({ data: channels })
    get.mockResolvedValueOnce({ data: groups })

    await expect(businessAnalyticsAPI.getGroupChannels(10, params)).resolves.toEqual(channels)
    await expect(businessAnalyticsAPI.getChannelGroups(20, params)).resolves.toEqual(groups)

    expect(get).toHaveBeenNthCalledWith(1, '/admin/business-analytics/groups/10/channels', {
      params,
    })
    expect(get).toHaveBeenNthCalledWith(2, '/admin/business-analytics/channels/20/groups', {
      params,
    })
  })

  it('exports csv through the export endpoint with blob response type', async () => {
    const csv = new Blob(['created_at,revenue'])
    const params: BusinessAnalyticsFilter = {
      start_date: '2026-06-01',
      end_date: '2026-06-30',
    }
    get.mockResolvedValueOnce({ data: csv })

    await expect(businessAnalyticsAPI.exportCsv(params)).resolves.toEqual(csv)

    expect(get).toHaveBeenCalledWith('/admin/business-analytics/export', {
      params,
      responseType: 'blob',
    })
  })

  it('manages channel price refresh runtime settings', async () => {
    const settings = {
      enabled: true,
      interval_seconds: 600,
      concurrency: 3,
      timeout_seconds: 30,
      last_result: { attempted: 2, success: 2, failed: 0 },
    }
    get.mockResolvedValueOnce({ data: settings })
    put.mockResolvedValueOnce({ data: { ...settings, interval_seconds: 900 } })
    post.mockResolvedValueOnce({ data: { attempted: 4, success: 3, failed: 1 } })

    await expect(businessAnalyticsAPI.getChannelPriceRefreshSettings()).resolves.toEqual(settings)
    await expect(
      businessAnalyticsAPI.updateChannelPriceRefreshSettings({
        enabled: true,
        interval_seconds: 900,
        concurrency: 3,
        timeout_seconds: 30,
      })
    ).resolves.toEqual({ ...settings, interval_seconds: 900 })
    await expect(businessAnalyticsAPI.runChannelPriceRefresh()).resolves.toEqual({
      attempted: 4,
      success: 3,
      failed: 1,
    })

    expect(get).toHaveBeenCalledWith('/admin/business-analytics/channel-price-refresh')
    expect(put).toHaveBeenCalledWith('/admin/business-analytics/channel-price-refresh', {
      enabled: true,
      interval_seconds: 900,
      concurrency: 3,
      timeout_seconds: 30,
    })
    expect(post).toHaveBeenCalledWith('/admin/business-analytics/channel-price-refresh/run')
  })
})
