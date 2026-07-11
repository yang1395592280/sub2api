import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export interface ZenxiangLiyuPrize {
  id: number
  name: string
  reward_amount: number
  probability: number
  enabled: boolean
  sort_order: number
}

export interface ZenxiangLiyuStatus {
  visible: boolean
  can_play: boolean
  reason?: string
  balance?: number
  ticket_amount: number
  effective_ticket_amount: number
  minimum_balance: number
  daily_play_limit: number
  today_play_count: number
  remaining_plays: number
  today_usage_amount: number
  free_play_usage_threshold: number
  free_play_available: boolean
  free_play_used: boolean
  prizes: ZenxiangLiyuPrize[]
}

export interface ZenxiangLiyuPlayResult {
  applied: boolean
  request_id: string
  prize_id: number
  prize_name: string
  reward_amount: number
  ticket_amount: number
  free_play: boolean
  user_net_amount: number
  balance_before: number
  balance_after_ticket: number
  balance_after_reward: number
  played_at: string
}

export interface ZenxiangLiyuRecord {
  id: number
  request_id: string
  ticket_amount: number
  reward_amount: number
  user_net_amount: number
  prize_id?: number
  prize_name: string
  probability: number
  played_at: string
}

export interface ZenxiangLiyuDailySummary {
  play_date: string
  play_count: number
  ticket_amount: number
  reward_amount: number
  user_net_amount: number
}

export interface ZenxiangLiyuPaginationParams {
  page?: number
  page_size?: number
}

export async function getZenxiangLiyuStatus(): Promise<ZenxiangLiyuStatus> {
  const { data } = await apiClient.get<ZenxiangLiyuStatus>('/zenxiang-liyu/status')
  return data
}

export async function playZenxiangLiyu(requestId: string): Promise<ZenxiangLiyuPlayResult> {
  const { data } = await apiClient.post<ZenxiangLiyuPlayResult>('/zenxiang-liyu/play', { request_id: requestId })
  return data
}

export async function listZenxiangLiyuRecords(
  params: ZenxiangLiyuPaginationParams = {}
): Promise<PaginatedResponse<ZenxiangLiyuRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<ZenxiangLiyuRecord>>('/zenxiang-liyu/records', { params })
  return data
}

export async function getZenxiangLiyuDailySummary(): Promise<ZenxiangLiyuDailySummary> {
  const { data } = await apiClient.get<ZenxiangLiyuDailySummary>('/zenxiang-liyu/daily-summary')
  return data
}
