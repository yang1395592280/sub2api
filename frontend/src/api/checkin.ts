import { apiClient } from './client'

export interface CheckinRecord {
  id?: number
  user_id?: number
  checkin_date: string
  reward_amount: number
  base_reward_amount: number
  bonus_status: 'none' | 'win' | 'lose'
  bonus_delta_amount: number
}

export interface CheckinRisk {
  id: number
  user_id: number
  status: 'observing' | 'cleared' | 'revoked' | 'restored'
  reason: string
  evidence: string
  observe_until: string
  cleared_amount: number
  created_at: string
  decided_at: string | null
}

export interface CheckinSettings {
  enabled: boolean
  min_reward: number
  max_reward: number
  bonus_enabled: boolean
  bonus_success_rate: number
  distribution: Array<{ start_percent: number; end_percent: number; weight: number }> | null
}

export interface CheckinStatus extends Omit<CheckinSettings, 'distribution'> {
  bonus_available: boolean
  timezone: string
  today_record: CheckinRecord | null
  risk?: { case_id: number; reason: string; cleared_amount: number; decided_at: string }
  stats: { total_reward: number; total_checkins: number; checked_in_today: boolean; records: CheckinRecord[] }
}

export interface CheckinProof {
  website?: string
  turnstile_token?: string
  tencent_ticket?: string
  tencent_randstr?: string
}

export const checkinAPI = {
  async status(month?: string) {
    return (await apiClient.get<CheckinStatus>('/user/checkin', { params: { month } })).data
  },
  async play(bonus: boolean, proof: CheckinProof) {
    const { data: challenge } = await apiClient.get<{ token: string }>('/user/checkin/challenge')
    return (await apiClient.post<CheckinRecord>(bonus ? '/user/checkin/lucky-bonus' : '/user/checkin', {
      ...proof, page_token: challenge.token
    })).data
  },
  async settings() { return (await apiClient.get<CheckinSettings>('/admin/checkins/settings')).data },
  async saveSettings(settings: CheckinSettings) { return (await apiClient.put<CheckinSettings>('/admin/checkins/settings', settings)).data },
  async records(page = 1) {
    return (await apiClient.get<{ items: Array<CheckinRecord & { user_email: string; created_at: string }>; total: number }>('/admin/checkins', { params: { page, page_size: 20 } })).data
  },
  async risks(page = 1) {
    return (await apiClient.get<{ items: CheckinRisk[]; total: number }>('/admin/checkins/risks', { params: { page, page_size: 20 } })).data
  },
  async confirmRisk(userID: number, reason: string, evidence: string) {
    return (await apiClient.post<CheckinRisk>('/admin/checkins/risks', { user_id: userID, reason, evidence })).data
  },
  async resolveRisk(id: number, action: 'revoke' | 'restore') {
    await apiClient.post(`/admin/checkins/risks/${id}/resolve`, { action })
  }
}
