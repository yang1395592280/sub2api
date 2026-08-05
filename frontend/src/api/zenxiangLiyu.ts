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
  ticket_usage_threshold: number
  daily_ticket_limit: number
  ticket_capacity: number
  ticket_retention_days: number
  tickets_available: number
  legacy_tickets_available?: number
  today_tickets_earned: number
  today_tickets_from_usage: number
  today_tickets_granted: number
  today_tickets_redeemed: number
  today_tickets_used: number
  today_tickets_available: number
  today_tickets_from_affiliate?: number
  affiliate_tickets_available?: number
  next_ticket_usage_target: number
  next_ticket_usage_missing: number
  lucky_coin_enabled: boolean
  lucky_coin_double_probability: number
  guess_size_enabled: boolean
  prizes: ZenxiangLiyuPrize[]
}

export interface ZenxiangLiyuPlayResult {
  id: number
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
  lucky_coin_available: boolean
  lucky_coin_played: boolean
}

export interface ZenxiangLiyuLuckyCoinResult {
  record_id: number
  outcome: 'double' | 'zero'
  original_reward: number
  adjustment_amount: number
  balance_after: number
  double_probability: number
  played_at: string
  lucky_coin_available: boolean
  guess_size_available: boolean
}

export type ZenxiangLiyuGuessSizeChoice = 'big' | 'small' | 'skip'

export interface ZenxiangLiyuGuessSizeResult {
  record_id: number
  choice: ZenxiangLiyuGuessSizeChoice
  outcome: 'big' | 'small' | 'skipped'
  won: boolean
  adjustment_amount: number
  balance_after: number
  big_probability: number
  small_probability: number
  played_at: string
  skipped: boolean
}

export interface ZenxiangLiyuRecord {
  id: number
  request_id: string
  ticket_amount: number
  reward_amount: number
  user_net_amount: number
  lucky_coin_played: boolean
  lucky_coin_outcome?: string
  lucky_coin_adjustment: number
  balance_after_lucky?: number
  guess_size_played: boolean
  guess_size_choice?: string
  guess_size_outcome?: string
  guess_size_won: boolean
  guess_size_adjustment: number
  balance_after_guess_size?: number
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

export async function playZenxiangLiyuLuckyCoin(recordId: number): Promise<ZenxiangLiyuLuckyCoinResult> {
  const { data } = await apiClient.post<ZenxiangLiyuLuckyCoinResult>(`/zenxiang-liyu/records/${recordId}/lucky-coin`, {})
  return data
}

export async function playZenxiangLiyuGuessSize(
  recordId: number,
  choice: ZenxiangLiyuGuessSizeChoice
): Promise<ZenxiangLiyuGuessSizeResult> {
  const { data } = await apiClient.post<ZenxiangLiyuGuessSizeResult>(`/zenxiang-liyu/records/${recordId}/guess-size`, { choice })
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
