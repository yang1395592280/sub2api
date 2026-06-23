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
  last_selected_at?: string | null
  last_error_at?: string | null
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

export type OpenAIRoutingReasonCode =
  | 'status_error'
  | 'status_inactive'
  | 'manual_unschedulable'
  | 'rate_limited'
  | 'overloaded'
  | 'temp_unschedulable'
  | 'runtime_blocked'
  | 'high_latency'
  | 'upstream_5xx'
  | 'timeout'
  | 'transport_error'
  | 'manual'
  | 'recovering'
  | 'health_degraded'
  | 'model_unsupported'
  | 'capability_unsupported'
  | 'transport_unsupported'
  | 'group_mismatch'
  | 'privacy_not_set'
  | 'quota_auto_paused'
  | 'concurrency_full'
  | 'channel_restricted'
  | 'compact_unsupported'

export type OpenAIRoutingQuotaWindow = '5h' | '7d'

export type OpenAIRoutingBlockSource =
  | 'persistent_account_state'
  | 'advanced_scheduler_health'
  | 'runtime_block'
  | 'ui_countdown_state'

export type OpenAIRoutingStatusLabel = 'candidate' | 'skipped' | 'degraded'

export type OpenAIRoutingSummaryCode =
  | 'cost_advantage'
  | 'low_load'
  | 'low_latency'
  | 'high_priority'
  | 'schedulable'

export type OpenAIRoutingSummaryReason = OpenAIRoutingReasonCode | OpenAIRoutingSummaryCode

export type OpenAIRoutingExplainNote =
  | 'sticky_may_override_ranking'
  | 'weighted_top_k_not_strict_best'

export type OpenAIRoutingRankingSource = 'empty' | 'scheduler_snapshot'

export interface OpenAIRoutingScoreBreakdown {
  total: number
  priority: number
  load: number
  queue: number
  error_rate: number
  ttft: number
  price: number
  health: number
}

export interface OpenAIRoutingQuotaDecision {
  window?: OpenAIRoutingQuotaWindow
  threshold?: number
  utilization?: number
  snapshot_at: string
}

export interface OpenAIRoutingBlockDetail {
  reason: OpenAIRoutingReasonCode
  source: OpenAIRoutingBlockSource
  until?: string | null
  quota_decision?: OpenAIRoutingQuotaDecision
  snapshot_at: string
}

export interface OpenAIRoutingSummary {
  account_id: number
  account_name: string
  rank?: number
  tier: OpenAISchedulerTier
  score: OpenAIRoutingScoreBreakdown
  status_label: OpenAIRoutingStatusLabel
  summary_reason: OpenAIRoutingSummaryReason
  summary_reasons: OpenAIRoutingSummaryReason[]
  is_schedulable_now: boolean
  block_reasons?: OpenAIRoutingReasonCode[]
  block_details?: OpenAIRoutingBlockDetail[]
  snapshot_at: string
}

export interface OpenAIRoutingRankingResponse {
  items: OpenAIRoutingSummary[]
  source: OpenAIRoutingRankingSource
  snapshot_at: string
}

export interface OpenAIRoutingAccountExplain {
  account: OpenAIRoutingSummary
  top: OpenAIRoutingSummary[]
  notes: OpenAIRoutingExplainNote[]
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

export async function getRoutingRanking(
  params: { group_id?: number; model?: string; platform?: string } = {},
  options?: { signal?: AbortSignal }
): Promise<OpenAIRoutingRankingResponse> {
  const { data } = await apiClient.get<OpenAIRoutingRankingResponse>('/admin/openai-scheduler/ranking', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function getRoutingExplain(
  id: number,
  params: { group_id?: number; model?: string; platform?: string } = {}
): Promise<OpenAIRoutingAccountExplain> {
  const { data } = await apiClient.get<OpenAIRoutingAccountExplain>(
    `/admin/openai-scheduler/accounts/${id}/routing-explain`,
    { params }
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
  getRoutingRanking,
  getRoutingExplain,
}

export default openaiSchedulerAPI
