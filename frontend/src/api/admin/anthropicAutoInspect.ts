import { apiClient } from '../client'

export interface AnthropicAutoInspectSettings {
  enabled: boolean
  interval_minutes: number
  error_cooldown_minutes: number
}

export interface AnthropicAutoInspectLog {
  id: number
  batch_id: number
  account_id: number
  account_name_snapshot: string
  platform: string
  account_type: string
  result: 'success' | 'rate_limited' | 'error' | 'skipped'
  skip_reason: string
  response_text: string
  error_message: string
  rate_limit_reset_at: string | null
  temp_unschedulable_until: string | null
  schedulable_changed: boolean
  started_at: string
  finished_at: string
  latency_ms: number
  created_at: string
}

export interface AnthropicAutoInspectBatch {
  id: number
  trigger_source: string
  status: string
  skip_reason: string
  total_accounts: number
  processed_accounts: number
  success_count: number
  rate_limited_count: number
  error_count: number
  skipped_count: number
  started_at: string
  finished_at: string | null
  created_at: string
}

export interface AnthropicAutoInspectPagination {
  total: number
  page: number
  page_size: number
  pages: number
}

export interface AnthropicAutoInspectListResponse<T> {
  items: T[]
  pagination: AnthropicAutoInspectPagination
}

export async function getSettings(): Promise<AnthropicAutoInspectSettings> {
  const { data } = await apiClient.get<AnthropicAutoInspectSettings>('/admin/anthropic-auto-inspect/settings')
  return data
}

export async function updateSettings(payload: AnthropicAutoInspectSettings): Promise<void> {
  await apiClient.put('/admin/anthropic-auto-inspect/settings', payload)
}

export async function runNow(): Promise<void> {
  await apiClient.post('/admin/anthropic-auto-inspect/run')
}

export async function listLogs(params: {
  page?: number
  page_size?: number
  search?: string
  result?: string
  started_from?: string
  started_to?: string
} = {}): Promise<AnthropicAutoInspectListResponse<AnthropicAutoInspectLog>> {
  const { data } = await apiClient.get<AnthropicAutoInspectListResponse<AnthropicAutoInspectLog>>(
    '/admin/anthropic-auto-inspect/logs',
    { params }
  )
  return data
}

export async function listBatches(params: {
  page?: number
  page_size?: number
} = {}): Promise<AnthropicAutoInspectListResponse<AnthropicAutoInspectBatch>> {
  const { data } = await apiClient.get<AnthropicAutoInspectListResponse<AnthropicAutoInspectBatch>>(
    '/admin/anthropic-auto-inspect/batches',
    { params }
  )
  return data
}

export const anthropicAutoInspectAPI = {
  getSettings,
  updateSettings,
  runNow,
  listLogs,
  listBatches
}

export default anthropicAutoInspectAPI
