import { apiClient } from '../client'
import type { MonitorStatus } from './channelMonitor'

export type OpenAIHealthWindow = '6h' | '24h' | '7d' | '30d'

export interface OpenAIHealthTrendPoint {
  status: MonitorStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  checked_at: string
}

export interface OpenAIHealthItem {
  id: number
  name: string
  endpoint: string
  group_name: string
  primary_model: string
  enabled: boolean
  latest_status: MonitorStatus | ''
  latest_first_token_ms: number | null
  latest_ping_latency_ms: number | null
  last_checked_at: string | null
  total_checks: number
  operational_checks: number
  failed_checks: number
  error_checks: number
  availability_pct: number
  avg_first_token_ms: number | null
  p95_first_token_ms: number | null
  avg_ping_latency_ms: number | null
  trend: OpenAIHealthTrendPoint[]
}

export interface OpenAIHealthOverview {
  time_window: OpenAIHealthWindow
  window_start: string
  window_end: string
  total_monitors: number
  healthy_monitors: number
  degraded_monitors: number
  failed_monitors: number
  average_availability_pct: number
  average_first_token_ms: number | null
  items: OpenAIHealthItem[]
}

export interface OpenAIHealthOverviewParams {
  group_name?: string
  search?: string
  window?: OpenAIHealthWindow
}

export async function getOverview(
  params: OpenAIHealthOverviewParams = {},
  options?: { signal?: AbortSignal }
): Promise<OpenAIHealthOverview> {
  const { data } = await apiClient.get<OpenAIHealthOverview>('/admin/openai-health/overview', {
    params,
    signal: options?.signal,
  })
  return data
}

export const openaiHealthAPI = {
  getOverview,
}

export default openaiHealthAPI
