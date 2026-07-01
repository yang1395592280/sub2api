import { apiClient } from '../client'

const basePath = '/admin/business-analytics'

export interface BusinessAnalyticsFilter {
  start_date: string
  end_date: string
  granularity?: 'day' | 'week'
  group_id?: number
  account_id?: number
  platform?: string
  timezone?: string
}

export interface BusinessMetricSummary {
  revenue: number
  channel_cost: number
  gross_profit: number
  profit_margin: number | null
  active_users: number
  requests: number
  revenue_per_active_user: number | null
  profit_per_active_user: number | null
}

export interface BusinessTrendPoint {
  date: string
  requests: number
  active_users: number
  revenue: number
  channel_cost: number
  gross_profit: number
  profit_margin?: number | null
}

export interface BusinessOverview extends BusinessMetricSummary {
  start_date: string
  end_date: string
  active_api_keys: number
  total_tokens: number
  missing_channel_price_records: number
  trend: BusinessTrendPoint[]
}

export interface BusinessGroupRow extends BusinessMetricSummary {
  group_id: number
  group_name: string
  platform: string
  current_rate_multiplier?: number | null
  avg_rate_multiplier?: number | null
  active_api_keys: number
  total_tokens: number
  previous_revenue: number
  previous_gross_profit: number
  revenue_change_rate?: number | null
  gross_profit_change_rate?: number | null
}

export interface BusinessChannelRow extends BusinessMetricSummary {
  account_id: number
  account_name: string
  channel_id: number
  platform: string
  status: string
  current_channel_price?: number | null
  avg_channel_price?: number | null
  balance_status?: string
  active_api_keys: number
  total_tokens: number
  missing_channel_price_records: number
}

export interface BusinessPriceChangeImpactParams {
  group_id: number
  change_date: string
  days?: number
  timezone?: string
}

export interface BusinessPriceChangeImpact {
  group_id: number
  change_date: string
  before_revenue: number
  after_revenue: number
  revenue_delta: number
  before_gross_profit: number
  after_gross_profit: number
  gross_profit_delta: number
  change_at?: string
}

export interface BusinessRecordsParams extends BusinessAnalyticsFilter {
  page?: number
  page_size?: number
}

export interface BusinessRecordRow {
  id: number
  created_at: string
  user_id: number
  user_email: string
  api_key_id: number
  api_key_name: string
  group_id: number
  group_name: string
  account_id: number
  account_name: string
  model: string
  requests: number
  total_tokens: number
  revenue: number
  channel_cost: number
  gross_profit: number
  channel_price_snapshot?: number | null
  channel_price_snapshot_missing: boolean
}

export interface BusinessRecordsResponse {
  items: BusinessRecordRow[]
  total: number
  page: number
  page_size: number
}

export async function getOverview(params: BusinessAnalyticsFilter): Promise<BusinessOverview> {
  const { data } = await apiClient.get<BusinessOverview>(`${basePath}/overview`, { params })
  return data
}

export async function getGroups(params: BusinessAnalyticsFilter): Promise<BusinessGroupRow[]> {
  const { data } = await apiClient.get<BusinessGroupRow[]>(`${basePath}/groups`, { params })
  return data
}

export async function getGroupChannels(
  groupId: number,
  params: BusinessAnalyticsFilter
): Promise<BusinessChannelRow[]> {
  const { data } = await apiClient.get<BusinessChannelRow[]>(
    `${basePath}/groups/${groupId}/channels`,
    { params }
  )
  return data
}

export async function getChannels(params: BusinessAnalyticsFilter): Promise<BusinessChannelRow[]> {
  const { data } = await apiClient.get<BusinessChannelRow[]>(`${basePath}/channels`, { params })
  return data
}

export async function getChannelGroups(
  channelId: number,
  params: BusinessAnalyticsFilter
): Promise<BusinessGroupRow[]> {
  const { data } = await apiClient.get<BusinessGroupRow[]>(
    `${basePath}/channels/${channelId}/groups`,
    { params }
  )
  return data
}

export async function getPriceChangeImpact(
  params: BusinessPriceChangeImpactParams
): Promise<BusinessPriceChangeImpact> {
  const { data } = await apiClient.get<BusinessPriceChangeImpact>(
    `${basePath}/price-change-impact`,
    { params }
  )
  return data
}

export async function getRecords(
  params: BusinessRecordsParams
): Promise<BusinessRecordsResponse> {
  const { data } = await apiClient.get<BusinessRecordsResponse>(`${basePath}/records`, { params })
  return data
}

export async function exportCsv(params: BusinessAnalyticsFilter): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`${basePath}/export`, {
    params,
    responseType: 'blob',
  })
  return data
}

export const businessAnalyticsAPI = {
  getOverview,
  getGroups,
  getGroupChannels,
  getChannels,
  getChannelGroups,
  getPriceChangeImpact,
  getRecords,
  exportCsv,
}

export default businessAnalyticsAPI
