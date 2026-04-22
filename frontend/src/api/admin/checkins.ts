import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export interface AdminCheckinRecord {
  id: number
  user_id: number
  user_email: string
  user_name: string
  checkin_date: string
  reward_amount: number
  user_timezone: string
  created_at: string
}

export interface AdminCheckinOverview {
  total_checkins: number
  total_reward_amount: number
  today_checkins: number
  avg_reward_amount: number
}

export interface AdminCheckinTrendPoint {
  date: string
  checkin_count: number
  reward_amount: number
}

export interface AdminCheckinRewardBucket {
  label: string
  count: number
  reward_amount: number
}

export interface AdminCheckinTopUser {
  user_id: number
  user_email: string
  user_name: string
  checkin_count: number
  reward_amount: number
}

export interface AdminCheckinAnalyticsResponse {
  overview: AdminCheckinOverview
  trend: AdminCheckinTrendPoint[]
  reward_distribution: AdminCheckinRewardBucket[]
  top_users: AdminCheckinTopUser[]
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    search?: string
    date?: string
    timezone?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<BasePaginationResponse<AdminCheckinRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<AdminCheckinRecord>>('/admin/checkins', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export async function getAnalytics(
  filters: {
    start_date: string
    end_date: string
    search?: string
    timezone?: string
    top_limit?: number
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<AdminCheckinAnalyticsResponse> {
  const { data } = await apiClient.get<AdminCheckinAnalyticsResponse>('/admin/checkins/analytics', {
    params: filters,
    signal: options?.signal
  })
  return data
}

const checkinsAPI = {
  list,
  getAnalytics
}

export default checkinsAPI
