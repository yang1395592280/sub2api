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

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    search?: string
    date?: string
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

const checkinsAPI = {
  list
}

export default checkinsAPI
