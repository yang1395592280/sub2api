import { apiClient } from '../client'

const basePath = '/admin/openai-auto-scheduler'

export type OpenAIAutoSchedulerState = 'running' | 'observing' | 'open' | 'half_open'

export type OpenAIAutoSchedulerEventType =
  | 'success'
  | 'slow'
  | 'severe_slow'
  | 'error'
  | 'rate_limited'
  | 'probe_success'
  | 'probe_error'
  | 'manual_reset'

export interface OpenAIAutoSchedulerSettings {
  enabled: boolean
  mode?: 'legacy' | 'balanced'
  shadow_mode?: boolean
  top_k?: number
  exploration_rate?: number
  session_escape_min_gap_ms?: number
  session_escape_ratio?: number
  health_ttl_seconds?: number
  real_sample_fresh_seconds?: number
  probe_jitter_seconds?: number
  probe_model: string
  probe_interval_seconds: number
  slow_threshold_ms: number
  severe_slow_threshold_ms: number
  consecutive_slow_breaker_threshold: number
  consecutive_error_breaker_threshold: number
  cooldown_seconds: number
  half_open_success_threshold: number
  cost_weight: number
  recovery_step: number
}

export interface OpenAIAutoSchedulerGroup {
  id: number
  name: string
  status: string
  enabled: boolean
}

export interface OpenAIAutoSchedulerScore {
  account_id: number
  account_name?: string
  channel_price?: number | null
  group_id: number
  model: string
  base_score: number
  base_score_percent: number
  final_score: number
  final_score_percent: number
  latency_score: number
  latency_score_percent: number
  error_score: number
  error_score_percent: number
  recovery_score: number
  recovery_score_percent: number
  cost_score: number
  cost_score_percent: number
  state: OpenAIAutoSchedulerState
  consecutive_slow_count: number
  consecutive_error_count: number
  consecutive_success_count: number
  request_count: number
  ttfb_sample_count: number
  slow_rate: number
  error_rate: number
  stuck_rate: number
  cooldown_until: string | null
  last_latency_ms: number | null
  last_ttfb_ms: number | null
  last_status_code: number | null
  last_error: string | null
  reason: string
  last_checked_at: string | null
}

export interface OpenAIAutoSchedulerEvent {
  account_id: number
  group_id: number
  model: string
  event_type: OpenAIAutoSchedulerEventType
  score_before: number
  score_before_percent: number
  score_after: number
  score_after_percent: number
  latency_ms: number | null
  ttfb_ms: number | null
  status_code: number | null
  message: string
  created_at: string
}

export interface OpenAIAutoSchedulerListParams {
  group_id?: number
  model?: string
  state?: OpenAIAutoSchedulerState | ''
  search?: string
  page?: number
  page_size?: number
}

export interface OpenAIAutoSchedulerListResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface OpenAIAutoSchedulerScoreActionParams {
  group_id: number
  model: string
}

export interface OpenAIAutoSchedulerProbeResponse {
  event_type: OpenAIAutoSchedulerEventType
  success: boolean
  message: string
  latency_ms: number | null
  ttfb_ms: number | null
}

export async function getSettings(): Promise<OpenAIAutoSchedulerSettings> {
  const { data } = await apiClient.get<OpenAIAutoSchedulerSettings>(`${basePath}/settings`)
  return data
}

export async function updateSettings(
  settings: OpenAIAutoSchedulerSettings
): Promise<OpenAIAutoSchedulerSettings> {
  const { data } = await apiClient.put<OpenAIAutoSchedulerSettings>(`${basePath}/settings`, settings)
  return data
}

export async function listGroups(): Promise<OpenAIAutoSchedulerGroup[]> {
  const { data } = await apiClient.get<OpenAIAutoSchedulerGroup[]>(`${basePath}/groups`)
  return data
}

export async function updateGroup(
  id: number,
  params: { enabled: boolean }
): Promise<OpenAIAutoSchedulerGroup> {
  const { data } = await apiClient.put<OpenAIAutoSchedulerGroup>(`${basePath}/groups/${id}`, params)
  return data
}

export async function listScores(
  params: OpenAIAutoSchedulerListParams = {},
  options?: { signal?: AbortSignal }
): Promise<OpenAIAutoSchedulerListResponse<OpenAIAutoSchedulerScore>> {
  const config = options?.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAIAutoSchedulerListResponse<OpenAIAutoSchedulerScore>>(
    `${basePath}/scores`,
    config
  )
  return data
}

export async function listEvents(
  params: OpenAIAutoSchedulerListParams = {},
  options?: { signal?: AbortSignal }
): Promise<OpenAIAutoSchedulerListResponse<OpenAIAutoSchedulerEvent>> {
  const config = options?.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAIAutoSchedulerListResponse<OpenAIAutoSchedulerEvent>>(
    `${basePath}/events`,
    config
  )
  return data
}

export async function resetScore(
  accountId: number,
  params: OpenAIAutoSchedulerScoreActionParams
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    `${basePath}/scores/accounts/${accountId}/reset`,
    undefined,
    { params }
  )
  return data
}

export async function probeScore(
  accountId: number,
  params: OpenAIAutoSchedulerScoreActionParams
): Promise<OpenAIAutoSchedulerProbeResponse> {
  const { data } = await apiClient.post<OpenAIAutoSchedulerProbeResponse>(
    `${basePath}/scores/accounts/${accountId}/probe`,
    undefined,
    { params }
  )
  return data
}

export const openaiAutoSchedulerAPI = {
  getSettings,
  updateSettings,
  listGroups,
  updateGroup,
  listScores,
  listEvents,
  resetScore,
  probeScore,
}

export default openaiAutoSchedulerAPI
