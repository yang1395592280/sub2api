import { apiClient } from './client'

export interface CheckinRecordSummary {
  checkin_date: string
  reward_amount: number
}

export interface CheckinStats {
  total_reward: number
  total_checkins: number
  checkin_count: number
  checked_in_today: boolean
  records: CheckinRecordSummary[]
}

export interface CheckinStatus {
  enabled: boolean
  min_reward: number
  max_reward: number
  stats: CheckinStats
}

export interface CheckinResponse {
  checkin_date: string
  reward_amount: number
}

const getBrowserTimezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

export async function getStatus(month: string): Promise<CheckinStatus> {
  const { data } = await apiClient.get<CheckinStatus>('/user/checkin', {
    params: { month }
  })
  return data
}

export async function doCheckin(turnstileToken?: string): Promise<CheckinResponse> {
  const { data } = await apiClient.post<CheckinResponse>('/user/checkin', {
    turnstile_token: turnstileToken,
    timezone: getBrowserTimezone()
  })
  return data
}

export const checkinAPI = {
  getStatus,
  doCheckin
}

export default checkinAPI
