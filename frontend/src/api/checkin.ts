import { apiClient } from './client'
import type { CheckinActionResponse, CheckinStatus } from '@/types'

export interface CheckinRequest {
  turnstile_token?: string
  timezone?: string
}

export interface CheckinStatusQuery {
  month?: string
  timezone?: string
}

export async function getStatus(params?: CheckinStatusQuery): Promise<CheckinStatus> {
  const { data } = await apiClient.get<CheckinStatus>('/user/checkin', { params })
  return data
}

export async function checkin(payload?: CheckinRequest): Promise<CheckinActionResponse> {
  const { data } = await apiClient.post<CheckinActionResponse>('/user/checkin', payload)
  return data
}

export async function playLuckyBonus(payload?: CheckinRequest): Promise<CheckinActionResponse> {
  const { data } = await apiClient.post<CheckinActionResponse>(
    '/user/checkin/lucky-bonus',
    payload
  )
  return data
}

export const checkinAPI = {
  getStatus,
  checkin,
  playLuckyBonus,
}

export default checkinAPI
