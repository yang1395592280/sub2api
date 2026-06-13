import { apiClient } from '../client'

export type OpenAISchedulerTier = 'primary' | 'standby' | 'observe' | 'degraded'

export interface OpenAISchedulerSettings {
  health_ranking_enabled: boolean
  primary_ratio: number
  primary_min_count: number
  ttft_degrade_ms: number
  error_rate_degrade_threshold: number
  consecutive_failure_threshold: number
  recover_success_threshold: number
  cooldown_seconds: number
  observe_probe_ratio: number
}

export interface OpenAIAccountHealth {
  account_id: number
  health_score: number
  tier: OpenAISchedulerTier
  degrade_reason: string
  cooldown_until?: string | null
  success_rate_ewma: number
  error_rate_ewma: number
  ttft_ewma_ms: number
  consecutive_errors: number
  consecutive_ok: number
  decision_reason: string
}

export interface OpenAISchedulerAccount {
  account_id: number
  account_name: string
  platform: string
  type: string
  status: string
  manual_priority: number
  channel_price?: number | null
  groups: number[]
  health: OpenAIAccountHealth
}

export interface OpenAISchedulerAccountDailyStat {
  account_id: number
  select_count: number
  select_ratio: number
  last_selected_at?: string | null
}

export interface OpenAISchedulerDailyStats {
  date: string
  group_id: number
  total_selects: number
  accounts: OpenAISchedulerAccountDailyStat[]
}

export interface OpenAISchedulerOverview {
  settings: OpenAISchedulerSettings
  metrics?: Record<string, unknown>
  tier_counts?: Partial<Record<OpenAISchedulerTier, number>>
  primary_count?: number
  standby_count?: number
  observe_count?: number
  degraded_count?: number
  average_health_score?: number
  average_ttft_ms?: number
}

export interface ListAccountsParams {
  page?: number
  page_size?: number
  group_id?: number
  tier?: OpenAISchedulerTier | ''
  search?: string
}

export interface ListAccountsResponse {
  items: OpenAISchedulerAccount[]
  total: number
  page: number
  page_size: number
}

export interface SchedulerActionRequest {
  action: 'run_probe' | 'promote_observe' | 'cooldown' | 'clear_cooldown'
  reason?: string
  duration_seconds?: number
}

export async function getOverview(params: { group_id?: number } = {}): Promise<OpenAISchedulerOverview> {
  const { data } = await apiClient.get<OpenAISchedulerOverview>('/admin/openai-scheduler/overview', {
    params,
  })
  return data
}

export async function getDailyStats(params: { group_id: number; date?: string }): Promise<OpenAISchedulerDailyStats> {
  const { data } = await apiClient.get<OpenAISchedulerDailyStats>('/admin/openai-scheduler/stats', {
    params,
  })
  return data
}

export async function recomputeDailyStats(params: { group_id: number; date?: string }): Promise<OpenAISchedulerDailyStats> {
  const { data } = await apiClient.post<OpenAISchedulerDailyStats>('/admin/openai-scheduler/stats/recompute', null, {
    params,
  })
  return data
}

export async function listAccounts(
  params: ListAccountsParams = {},
  options?: { signal?: AbortSignal }
): Promise<ListAccountsResponse> {
  const { data } = await apiClient.get<ListAccountsResponse>('/admin/openai-scheduler/accounts', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function getAccount(id: number): Promise<OpenAISchedulerAccount> {
  const { data } = await apiClient.get<OpenAISchedulerAccount>(
    `/admin/openai-scheduler/accounts/${id}`
  )
  return data
}

export async function applyAction(
  id: number,
  payload: SchedulerActionRequest
): Promise<{ success: boolean }> {
  const { data } = await apiClient.post<{ success: boolean }>(
    `/admin/openai-scheduler/accounts/${id}/actions`,
    payload
  )
  return data
}

export async function getSettings(): Promise<OpenAISchedulerSettings> {
  const { data } = await apiClient.get<OpenAISchedulerSettings>('/admin/openai-scheduler/settings')
  return data
}

export async function updateSettings(
  payload: OpenAISchedulerSettings
): Promise<OpenAISchedulerSettings> {
  const { data } = await apiClient.put<OpenAISchedulerSettings>(
    '/admin/openai-scheduler/settings',
    payload
  )
  return data
}

export const openaiSchedulerAPI = {
  getOverview,
  getDailyStats,
  recomputeDailyStats,
  listAccounts,
  getAccount,
  applyAction,
  getSettings,
  updateSettings,
}

export default openaiSchedulerAPI
