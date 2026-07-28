import { apiClient } from '../client'

const basePath = '/admin/openai-auto-scheduler'

export type OpenAIAutoSchedulerState = 'running' | 'observing' | 'open' | 'half_open'

export type OpenAIAutoSchedulerEventType =
  | 'success'
  | 'slow'
  | 'severe_slow'
  | 'error'
  | 'request_error'
  | 'rate_limited'
  | 'probe_success'
  | 'probe_error'
  | 'manual_reset'

export interface OpenAIAutoSchedulerSettings {
  enabled: boolean
  mode?: 'legacy' | 'balanced' | 'performance_first' | 'cost_first' | 'efficiency'
  shadow_mode?: boolean
  top_k?: number
  adaptive_top_k_enabled?: boolean
  exploration_rate?: number
  exploration_budget?: number
  exploration_min_interval_seconds?: number
  exploration_max_real_samples_per_hour?: number
  stale_open_requires_probe?: boolean
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
  temperature?: number
  max_account_share?: number
  low_confidence_max_share?: number
  latency_budget_ms?: number
  early_sse_preamble_flush_enabled?: boolean
  first_output_timeout_seconds?: number
  high_effort_first_output_timeout_seconds?: number
  weights?: OpenAISchedulerPolicyWeights
}

export interface OpenAISchedulerPolicyWeights {
  latency: number
  reliability: number
  cost: number
  capacity: number
  quota: number
  priority: number
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
  account_id?: number
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

export type OpenAISchedulerWindow = '1h' | '6h' | '24h' | '7d'
export type OpenAISchedulerRankingWindow = '15m' | '1h' | '6h' | '24h' | '7d'

export interface OpenAISchedulerRequestOptions {
  signal?: AbortSignal
}

export interface OpenAISchedulerOverviewParams {
  group_id?: number
  window: OpenAISchedulerWindow
}

export interface OpenAISchedulerGroupSummary {
  id: number
  name: string
  enabled: boolean
  account_count: number
  e2e_ttft_p90_ms: number | null
  alert_level: 'ok' | 'warning' | 'critical' | 'disabled'
}

export interface OpenAISchedulerTrendPoint {
  bucket: string
  e2e_ttft_p50_ms: number | null
  e2e_ttft_p90_ms: number | null
}

export interface OpenAISchedulerSlowCause {
  reason: 'upstream_ttft' | 'queue' | 'retry'
  count: number
  ratio: number
}

export interface OpenAISchedulerOverview {
  e2e_ttft_p50_ms: number | null
  e2e_ttft_p90_ms: number | null
  selection_p95_ms: number | null
  probe_ratio: number
  groups: OpenAISchedulerGroupSummary[]
  trend: OpenAISchedulerTrendPoint[]
  slow_causes: OpenAISchedulerSlowCause[]
  runtime: OpenAISchedulerRuntimeMetrics
}

export interface OpenAISchedulerRuntimeMetrics {
  exploration_allowed_total: number
  exploration_rejected_total: number
  exploration_interval_total: number
  exploration_hourly_total: number
  exploration_error_total: number
  low_confidence_fallback_total: number
  unified_health_reads_total: number
  unified_health_dimensions_total: number
  unified_health_fallbacks_total: number
}

export type OpenAISchedulerHealthSort =
  | 'account_id'
  | 'predicted_ttft_ms'
  | 'error_rate'
  | 'real_sample_count'
  | 'probe_sample_count'
  | 'snapshot_age_ms'
  | 'channel_price'

export interface OpenAISchedulerHealthParams {
  group_id?: number
  state?: string
  model_family?: string
  endpoint?: string
  transport?: string
  sort?: OpenAISchedulerHealthSort
  order?: 'asc' | 'desc'
  page?: number
  page_size?: number
}

export interface OpenAISchedulerHealthRow {
  account_id: number
  account_name: string
  group_id: number
  model_family: string
  endpoint: string
  transport: string
  state: string
  predicted_ttft_ms: number | null
  real_sample_count: number
  probe_sample_count: number
  error_rate: number
  rate_limited_rate: number
  server_error_rate: number
  load_inflight: number
  load_capacity: number
  waiting_count: number
  channel_price: number | null
  decision: string
  decision_reason: string
  scheduler_mode: string
  shadow_mode: boolean
  sticky_escape_reason: string | null
  snapshot_age_ms: number | null
  cooldown_until: string | null
}

export type OpenAISchedulerEligibility = 'eligible' | 'low_confidence' | 'latency_tail' | 'hard_rejected'

export interface OpenAISchedulerRankingParams {
  group_id: number
  window: OpenAISchedulerRankingWindow
  model_family?: string
  endpoint?: string
  transport?: string
  eligibility?: OpenAISchedulerEligibility | ''
  page?: number
  page_size?: number
}

export interface OpenAISchedulerRankingPartition {
  group_id: number
  model_family: string
  endpoint: string
  transport: string
}

export interface OpenAISchedulerPolicyContext {
  engine_enabled: boolean
  global_enabled: boolean
  group_enabled: boolean
  configured_mode: string
  effective_mode: string
  shadow_mode: boolean
  fallback_reason?: string
  policy_version: string
  calculated_at: string
}

export interface OpenAISchedulerRankingSummary {
  candidate_count: number
  eligible_count: number
  rejected_count: number
  low_confidence_count: number
  request_count: number
}

export interface OpenAISchedulerRankingItem {
  partition: OpenAISchedulerRankingPartition
  partition_count: number
  rank: number
  account_id: number
  account_name: string
  eligibility: OpenAISchedulerEligibility
  eligibility_reason: string
  traffic_class?: 'normal' | 'exploration' | 'fallback' | 'mixed'
  utility_score: number
  target_share: number
  actual_share: number
  selected_requests: number
  predicted_ttft_ms: number
  ttft_p50_ms: number
  ttft_p90_ms: number
  error_rate: number
  rate_limited_rate: number
  server_error_rate: number
  load_inflight: number
  load_capacity: number
  waiting_count: number
  channel_price: number | null
  estimated_cost: number
  confidence: string
  real_sample_count: number
  probe_sample_count: number
  snapshot_age_ms: number
  latency_score: number
  reliability_score: number
  cost_score: number
  capacity_score: number
  quota_score: number
  priority_score: number
  deviation_reasons: string[]
  decision_summary: string
}

export interface OpenAISchedulerRankingResult {
  policy_context: OpenAISchedulerPolicyContext
  summary: OpenAISchedulerRankingSummary
  items: OpenAISchedulerRankingItem[]
  total: number
  page: number
  page_size: number
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

export async function getOverview(
  params: OpenAISchedulerOverviewParams,
  options: OpenAISchedulerRequestOptions = {}
): Promise<OpenAISchedulerOverview> {
  const config = options.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAISchedulerOverview>(`${basePath}/overview`, config)
  return data
}

export async function listHealth(
  params: OpenAISchedulerHealthParams = {},
  options: OpenAISchedulerRequestOptions = {}
): Promise<OpenAIAutoSchedulerListResponse<OpenAISchedulerHealthRow>> {
  const config = options.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAIAutoSchedulerListResponse<OpenAISchedulerHealthRow>>(
    `${basePath}/health`,
    config
  )
  return data
}

export async function listRankings(
  params: OpenAISchedulerRankingParams,
  options: OpenAISchedulerRequestOptions = {}
): Promise<OpenAISchedulerRankingResult> {
  const config = options.signal ? { params, signal: options.signal } : { params }
  const { data } = await apiClient.get<OpenAISchedulerRankingResult>(`${basePath}/rankings`, config)
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
  getOverview,
  listHealth,
  listRankings,
  resetScore,
  probeScore,
}

export default openaiAutoSchedulerAPI
