import { apiClient } from '../client'

export interface QualitySettings {
  revision: number
  control_epoch: number
  automation_enabled: boolean
  control_enabled: boolean
  global_concurrency: number
  queue_limit: number
  queue_timeout_seconds: number
  global_daily_call_limit: number
  default_policy_daily_call_limit: number
  raw_retention_days: number
  metadata_retention_days: number
  store_failure_policy: 'fail_closed'
}
export interface QualityRuntimeHealth { queue_depth: number; running_count: number; store_status: string; last_sync_at: string }
export interface QualityBudget { budget_date: string; timezone: string; limit: number; consumed: number; reserved: number; remaining: number }
export interface QualityCondition {
  model_id: string
  protocol: 'responses' | 'chat_completions'
  transport: 'http' | 'websocket_v2'
  reasoning_effort: string
  service_tier: string
  benchmark_id: 'candy'
  benchmark_version: '1'
  scorer_version: 'strict-json-v1'
}
export interface QualityRules {
  auto_check_enabled: boolean
  control_enabled: boolean
  interval_seconds: number
  jitter_seconds: number
  failure_threshold: number
  recovery_threshold: number
  cooldown_seconds: number
  recovery_interval_seconds: number
  evidence_window_seconds: number
  timeout_seconds: number
  max_output_tokens: number
  daily_call_limit: number
}
export interface QualityTarget {
  account_id: number
  account_name: string
  source_group_id: number
  source_group_name: string
  compatible_source_group_ids: number[]
  eligibility: string
  can_probe: boolean
  can_enforce: boolean
  reasons: string[]
}
export interface QualityPolicy {
  id: number
  account_id: number
  account_name: string
  source_group_id: number
  source_group_name?: string
  control_supported?: boolean
  compatible_source_group_ids: number[]
  condition: QualityCondition
  effective_upstream_model: string
  effective_reasoning_effort: string
  effective_service_tier: string
  scope_hash: string
  revision: number
  control_epoch: number
  eligibility: string
  rules: QualityRules
  state: { state: string; freshness: string; failure_streak: number; recovery_streak: number; cooldown_until: string | null; last_checked_at: string | null; last_verdict: string }
  control: { effective: boolean; reason: string; action: string }
  next_run_at?: string | null
}
export interface QualityRun {
  id: number
  policy_id: number
  account_id: number
  trigger_source: string
  status: string
  verdict: string | null
  reason_code: string
  queued_at: string
  started_at: string | null
  finished_at: string | null
  state_applied: boolean
  state_skip_reason: string
  response_text?: string | null
  response_expires_at?: string | null
  input_snapshot?: QualityPolicy
  result: { verdict: string; final_answer: number | null; expected_answer: number; reason_code: string; latency_ms: number; input_tokens: number | null; output_tokens: number | null; total_tokens: number | null; estimated_cost_usd: number | null; response_truncated: boolean }
}
export interface QualityEvent {
  id: number
  policy_id: number | null
  account_id: number | null
  run_id: number | null
  event_type: string
  old_state: string
  new_state: string
  reason_code: string
  actor_user_id: number
  revision: number
  global_control_epoch: number
  policy_control_epoch: number
  created_at: string
}
export interface QualityRunDetail { run: QualityRun; response_available: boolean; response_expired: boolean }
export interface QualityBucket { index: number; start_at: string; end_at: string; status: string; total: number; status_counts: Record<string, number>; sample_run_ids: number[]; has_more: boolean }
export interface QualityMonitor { policy_id: number; scope_hash: string; as_of: string; bucket_seconds: number; complete: boolean; summary: Record<string, number | null>; buckets: QualityBucket[]; evidence_view: string }
export interface QualityPage<T> { items: T[]; total: number; page: number; page_size: number; pages: number }
const root = '/admin/account-quality'
const idem = () => crypto.randomUUID()
export const qualityIdempotencyKey = idem
export async function getQualitySettings() { return (await apiClient.get<{ settings: QualitySettings; health: QualityRuntimeHealth; budget: QualityBudget }>(`${root}/settings`)).data }
export async function saveQualitySettings(settings: Omit<QualitySettings, 'revision' | 'control_epoch'> & { expected_revision: number }) { return (await apiClient.put(`${root}/settings`, settings, { headers: { 'Idempotency-Key': idem() } })).data }
export async function getQualityTargets(params: Record<string, unknown> = {}) { return (await apiClient.get<QualityPage<QualityTarget>>(`${root}/targets`, { params })).data }
export async function getQualityOverview(params: Record<string, unknown> = {}) { return (await apiClient.get(`${root}/overview`, { params })).data as any }
export async function getQualityPolicies(params: Record<string, unknown> = {}) { return (await apiClient.get<QualityPage<QualityPolicy>>(`${root}/policies`, { params })).data }
async function getAllPages<T>(fetchPage: (page: number) => Promise<QualityPage<T>>) {
  const first = await fetchPage(1)
  const items = [...first.items]
  for (let start = 2; start <= first.pages; start += 5) {
    const pageNumbers = Array.from({ length: Math.min(5, first.pages - start + 1) }, (_, index) => start + index)
    const pages = await Promise.all(pageNumbers.map(fetchPage))
    for (const page of pages) items.push(...page.items)
  }
  return { ...first, items }
}
export async function getAllQualityTargets() { return getAllPages(page => getQualityTargets({ page, page_size: 100 })) }
export async function getAllQualityPolicies(params: Record<string, unknown> = {}) { return getAllPages(page => getQualityPolicies({ ...params, page, page_size: 100 })) }
export interface NewQualityPolicy { account_id: number; source_group_id: number; condition: QualityCondition; rules: QualityRules }
export interface QualityPolicyUpdate { policy_id: number; expected_revision: number; rules: QualityRules }
export async function createQualityPolicies(items: NewQualityPolicy[]) { return (await apiClient.post<{ items: QualityPolicy[] }>(`${root}/policies`, { items }, { headers: { 'Idempotency-Key': idem() } })).data }
export async function createQualityPolicy(item: NewQualityPolicy) { return createQualityPolicies([item]) }
export async function previewQuality(items: unknown[], mode: 'create' | 'update' = 'create') { return (await apiClient.post(`${root}/preview`, { mode, items })).data as any }
export async function previewQualitySettings(settings: QualitySettings) {
  const { revision, control_epoch: _controlEpoch, ...payload } = settings
  return (await apiClient.post(`${root}/preview`, { mode: 'settings', settings: { ...payload, expected_revision: revision } })).data as any
}
export async function emergencyDisableQuality(reason: string) { return (await apiClient.post(`${root}/settings/emergency-disable`, { reason }, { headers: { 'Idempotency-Key': idem() } })).data }
export async function submitQualityRuns(policy_ids: number[]) { return (await apiClient.post(`${root}/runs`, { policy_ids }, { headers: { 'Idempotency-Key': idem() } })).data }
export async function getQualityMonitor(id: number, bucket_seconds?: number) { return (await apiClient.get<QualityMonitor>(`${root}/policies/${id}/monitor`, { params: { bucket_seconds } })).data }
export async function getQualityRuns(params: Record<string, unknown> = {}) { return (await apiClient.get<QualityPage<QualityRun>>(`${root}/runs`, { params })).data }
export async function getQualityRun(id: number, includeResponse = true) { return (await apiClient.get<QualityRunDetail>(`${root}/runs/${id}`, { params: { include_response: includeResponse } })).data }
export async function cancelQualityRun(id: number, reason: string) { return (await apiClient.post(`${root}/runs/${id}/cancel`, { reason }, { headers: { 'Idempotency-Key': idem() } })).data }
export async function getQualityEvents(params: Record<string, unknown> = {}) { return (await apiClient.get<QualityPage<QualityEvent>>(`${root}/events`, { params })).data }
export async function getQualityBucketRuns(policy: QualityPolicy, monitor: QualityMonitor, index: number, page = 1, page_size = 100) { const params = { policy_id: policy.id, scope_hash: policy.scope_hash, monitor_as_of: monitor.as_of, bucket_seconds: monitor.bucket_seconds, bucket_index: index, page, page_size }; return getQualityRuns(params) }
export async function updateQualityPolicies(items: QualityPolicyUpdate[]) { return (await apiClient.post(`${root}/policies/batch-update`, { items }, { headers: { 'Idempotency-Key': idem() } })).data }
export async function updateQualityPolicy(policy: QualityPolicy, rules: QualityRules) { return updateQualityPolicies([{ policy_id: policy.id, expected_revision: policy.revision, rules }]) }
export async function qualityPolicyAction(id: number, action: 'release' | 'archive', expected_revision: number, reason: string) { return (await apiClient.post(`${root}/policies/${id}/${action}`, { expected_revision, reason }, { headers: { 'Idempotency-Key': idem() } })).data }
export interface QualityAccountSummary {
  account_id: number
  configured: boolean
  active_policy_count: number
  worst_state: string
  fresh: number
  stale: number
  enforced_count: number
}
export async function getQualityAccountSummaries(account_ids: number[]) { return (await apiClient.post<{ items: QualityAccountSummary[] }>(`${root}/summaries`, { account_ids })).data.items }
