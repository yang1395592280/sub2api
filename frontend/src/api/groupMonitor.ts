import { apiClient } from './client'

export const CUSTOM_FEATURE_KEYS = [
  'zenxiang_liyu_enabled', 'redeem_code_purchase_enabled', 'join_group_enabled', 'compact_home_enabled',
  'affiliate_enabled', 'affiliate_ticket_campaign_enabled', 'affiliate_admin_recharge_enabled',
  'available_channels_enabled', 'model_plaza_enabled', 'channel_monitor_enabled', 'channel_monitor_user_visible',
  'channel_monitor_show_quota', 'channel_monitor_pelican_enabled', 'channel_monitor_pelican_user_visible',
  'hide_ccs_import_button', 'openai_advanced_scheduler_enabled', 'openai_low_upstream_rate_priority_enabled', 'openai_advanced_scheduler_sticky_weighted_enabled',
  'openai_advanced_scheduler_subscription_priority_enabled', 'openai_codex_version_auto_sync_enabled', 'openai_codex_ticket_enabled'
] as const
export interface CustomFeatures {
  flags: Record<string, boolean>
  menus: { id: string; label: string; enabled: boolean }[]
}
export interface GroupMonitorConfig {
  models: string[]
  enabled: boolean
  probe_interval_seconds: number
  pelican_enabled: boolean
  pelican_interval_seconds: number
  pelican_prompt: string
  daily_request_limit: number
  keep_history: number
}
export interface MonitorRunDetail {
  account_id?: number
  account_name?: string
  mapped_model?: string
  trace_id?: string
  upstream_message_id?: string
  upstream_request_id?: string
  error?: string
  latency_ms: number
  ttft_ms?: number
  input_tokens?: number
  output_tokens?: number
  total_tokens?: number
  html?: string
  review: string
}
export interface MonitorRun {
  id: number
  monitor_id?: number
  kind: 'probe' | 'pelican'
  model: string
  status: string
  created_at: string
  finished_at?: string
  detail?: MonitorRunDetail
  latency_ms?: number
  ttft_ms?: number
  html?: string
  review?: string
}
export interface GroupMonitor {
  id: number
  group_id: number
  group_name: string
  platform: string
  config?: GroupMonitorConfig
  models?: string[]
  pelican_visible?: boolean
  next_probe_at?: string
  next_pelican_at?: string
  lease_until?: string
  requests_used?: number
  latest_runs: MonitorRun[]
}
export const groupMonitorPath = (admin: boolean) => `${admin ? '/admin' : ''}/channel-group-monitors`
export const groupMonitorAPI = {
  async list(admin: boolean): Promise<GroupMonitor[]> { return (await apiClient.get<GroupMonitor[]>(groupMonitorPath(admin))).data },
  async save(group_ids: number[], config: GroupMonitorConfig) { await apiClient.put(groupMonitorPath(true), { group_ids, config }) },
  async remove(id: number) { await apiClient.delete(`${groupMonitorPath(true)}/${id}`) },
  async run(id: number, kind: string) { await apiClient.post(`${groupMonitorPath(true)}/${id}/run`, { kind }) },
  async history(admin: boolean, id: number, kind: string, model = ''): Promise<MonitorRun[]> {
    return (await apiClient.get<MonitorRun[]>(`${groupMonitorPath(admin)}/${id}/history`, { params: { kind, model } })).data
  },
  async getRun(admin: boolean, id: number, runId: number): Promise<MonitorRun> { return (await apiClient.get<MonitorRun>(`${groupMonitorPath(admin)}/${id}/runs/${runId}`)).data },
  async review(id: number, runId: number, review: string) { await apiClient.patch(`${groupMonitorPath(true)}/${id}/runs/${runId}`, { review }) }
}
