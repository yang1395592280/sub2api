import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { ZenxiangLiyuPrize } from '../zenxiangLiyu'

const basePath = '/admin/zenxiang-liyu'

export interface ZenxiangLiyuSettings {
  global_enabled: boolean
  ticket_amount: number
  minimum_balance: number
  daily_play_limit: number
  ticket_usage_threshold: number
  daily_ticket_limit: number
  unit_sale_price: number
  unit_cost_price: number
  lucky_coin_enabled: boolean
  lucky_coin_double_probability: number
}

export interface ZenxiangLiyuPrizeInput {
  id?: number
  name: string
  reward_amount: number
  probability: number
  enabled: boolean
  sort_order: number
}

export interface ZenxiangLiyuGrant {
  user_id: number
  user_email: string
  enabled: boolean
  granted_by?: number
  notes: string
  created_at: string
  updated_at: string
}

export interface ZenxiangLiyuGrantInput {
  user_id: number
  enabled?: boolean
  notes?: string
}

export interface ZenxiangLiyuTicketGift {
  id: number
  request_id: string
  user_id: number
  user_email?: string
  play_date: string
  ticket_count: number
  granted_by?: number
  notes: string
  created_at: string
  updated_at: string
}

export interface ZenxiangLiyuTicketGiftInput {
  request_id: string
  user_id: number
  ticket_count: number
  notes?: string
}

export interface ZenxiangLiyuOverviewStats {
  total_plays: number
  total_revenue: number
  total_expense: number
  net_profit: number
  participating_users: number
}

export interface ZenxiangLiyuUserStats {
  user_id: number
  user_email: string
  balance: number
  usage_amount: number
  play_count: number
  ticket_amount: number
  reward_amount: number
  user_net_amount: number
}

export interface ZenxiangLiyuPrizeStats {
  prize_id?: number
  prize_name: string
  hit_count: number
  reward_amount: number
  probability: number
}

export interface ZenxiangLiyuPeriodStats {
  period_start: string
  period_label: string
  play_count: number
  participant_count: number
  usage_amount: number
  tickets_used: number
  ticket_amount: number
  reward_amount: number
  average_reward: number
  user_net_amount: number
  system_revenue: number
  system_expense: number
  system_profit: number
  most_hit_prize_name?: string
  most_hit_prize_count: number
}

export interface ZenxiangLiyuResetDailyResult {
  user_id: number
  play_date: string
  previous_play_count: number
  effective_play_count: number
  remaining_plays: number
}

export interface ZenxiangLiyuPaginationParams {
  page?: number
  page_size?: number
  date?: string
}

export interface ZenxiangLiyuSimulationRequest {
  user_count: number
  plays_per_user: number
  initial_balance: number
  ticket_amount: number
  minimum_balance: number
  daily_play_limit: number
  prizes: ZenxiangLiyuPrizeInput[]
}

export interface ZenxiangLiyuSimulationPrizeHit {
  prize_id: number
  prize_name: string
  hit_count: number
  actual_rate: number
}

export interface ZenxiangLiyuSimulationResult {
  total_plays: number
  total_revenue: number
  total_expense: number
  net_profit: number
  profit_rate: number
  profitable_users: number
  losing_users: number
  break_even_users: number
  prize_hits: ZenxiangLiyuSimulationPrizeHit[]
}

export interface ZenxiangLiyuRecommendationRequest {
  target_profit_rate: number
  ticket_amount: number
  prizes: ZenxiangLiyuPrizeInput[]
}

export interface ZenxiangLiyuRecommendationPlan {
  prizes: ZenxiangLiyuPrize[]
  probability_total: number
  theory_expense: number
  theory_profit: number
  theory_profit_rate: number
}

export interface ZenxiangLiyuRecommendationResult {
  target_expense: number
  plans: ZenxiangLiyuRecommendationPlan[]
}

export interface ZenxiangLiyuProfitPreviewRequest {
  consumption_amount: number
  ticket_usage_threshold: number
  daily_ticket_limit: number
  unit_sale_price: number
  unit_cost_price: number
  prizes: ZenxiangLiyuPrizeInput[]
}

export interface ZenxiangLiyuProfitPreviewResult {
  expected_reward_per_ticket: number
  expected_tickets: number
  expected_reward_total: number
  gross_profit_before_reward: number
  gross_profit_after_reward: number
  gross_profit_rate_before: number
  gross_profit_rate_after: number
  reward_rate: number
}

async function getSettings(): Promise<ZenxiangLiyuSettings> {
  const { data } = await apiClient.get<ZenxiangLiyuSettings>(`${basePath}/settings`)
  return data
}

async function updateSettings(settings: ZenxiangLiyuSettings): Promise<ZenxiangLiyuSettings> {
  const { data } = await apiClient.put<ZenxiangLiyuSettings>(`${basePath}/settings`, settings)
  return data
}

async function listPrizes(): Promise<ZenxiangLiyuPrize[]> {
  const { data } = await apiClient.get<ZenxiangLiyuPrize[]>(`${basePath}/prizes`)
  return data
}

async function createPrize(prize: ZenxiangLiyuPrizeInput): Promise<ZenxiangLiyuPrize> {
  const { data } = await apiClient.post<ZenxiangLiyuPrize>(`${basePath}/prizes`, prize)
  return data
}

async function replacePrizes(prizes: ZenxiangLiyuPrizeInput[]): Promise<ZenxiangLiyuPrize[]> {
  const { data } = await apiClient.put<ZenxiangLiyuPrize[]>(`${basePath}/prizes`, { prizes })
  return data
}

async function updatePrize(id: number, prize: ZenxiangLiyuPrizeInput): Promise<ZenxiangLiyuPrize> {
  const { data } = await apiClient.put<ZenxiangLiyuPrize>(`${basePath}/prizes/${id}`, prize)
  return data
}

async function deletePrize(id: number): Promise<{ id: number }> {
  const { data } = await apiClient.delete<{ id: number }>(`${basePath}/prizes/${id}`)
  return data
}

async function listGrants(params: ZenxiangLiyuPaginationParams = {}): Promise<PaginatedResponse<ZenxiangLiyuGrant>> {
  const { data } = await apiClient.get<PaginatedResponse<ZenxiangLiyuGrant>>(`${basePath}/grants`, { params })
  return data
}

async function createGrant(grant: ZenxiangLiyuGrantInput): Promise<ZenxiangLiyuGrant> {
  const { data } = await apiClient.post<ZenxiangLiyuGrant>(`${basePath}/grants`, grant)
  return data
}

async function deleteGrant(userId: number): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(`${basePath}/grants/${userId}`)
  return data
}

async function resetGrantDailyPlays(userId: number): Promise<ZenxiangLiyuResetDailyResult> {
  const { data } = await apiClient.post<ZenxiangLiyuResetDailyResult>(`${basePath}/grants/${userId}/reset-daily`)
  return data
}

async function giftTickets(gift: ZenxiangLiyuTicketGiftInput): Promise<ZenxiangLiyuTicketGift> {
  const { data } = await apiClient.post<ZenxiangLiyuTicketGift>(`${basePath}/tickets/gift`, gift)
  return data
}

async function getOverviewStats(): Promise<ZenxiangLiyuOverviewStats> {
  const { data } = await apiClient.get<ZenxiangLiyuOverviewStats>(`${basePath}/stats/overview`)
  return data
}

async function listPeriodStats(period: 'day' | 'week' | 'month' = 'day'): Promise<ZenxiangLiyuPeriodStats[]> {
  const { data } = await apiClient.get<ZenxiangLiyuPeriodStats[]>(`${basePath}/stats/periods`, { params: { period } })
  return data
}

async function listUserStats(params: ZenxiangLiyuPaginationParams = {}): Promise<PaginatedResponse<ZenxiangLiyuUserStats>> {
  const { data } = await apiClient.get<PaginatedResponse<ZenxiangLiyuUserStats>>(`${basePath}/stats/users`, { params })
  return data
}

async function listPrizeStats(): Promise<ZenxiangLiyuPrizeStats[]> {
  const { data } = await apiClient.get<ZenxiangLiyuPrizeStats[]>(`${basePath}/stats/prizes`)
  return data
}

async function simulate(request: ZenxiangLiyuSimulationRequest): Promise<ZenxiangLiyuSimulationResult> {
  const { data } = await apiClient.post<ZenxiangLiyuSimulationResult>(`${basePath}/simulate`, request)
  return data
}

async function recommend(request: ZenxiangLiyuRecommendationRequest): Promise<ZenxiangLiyuRecommendationResult> {
  const { data } = await apiClient.post<ZenxiangLiyuRecommendationResult>(`${basePath}/simulate/recommend`, request)
  return data
}

async function previewProfit(request: ZenxiangLiyuProfitPreviewRequest): Promise<ZenxiangLiyuProfitPreviewResult> {
  const { data } = await apiClient.post<ZenxiangLiyuProfitPreviewResult>(`${basePath}/simulate/profit-preview`, request)
  return data
}

async function applySimulation(prizes: ZenxiangLiyuPrizeInput[]): Promise<ZenxiangLiyuPrize[]> {
  const { data } = await apiClient.post<ZenxiangLiyuPrize[]>(`${basePath}/simulate/apply`, { prizes })
  return data
}

export const adminZenxiangLiyuAPI = {
  getSettings,
  updateSettings,
  listPrizes,
  createPrize,
  replacePrizes,
  updatePrize,
  deletePrize,
  listGrants,
  createGrant,
  deleteGrant,
  resetGrantDailyPlays,
  giftTickets,
  getOverviewStats,
  listPeriodStats,
  listUserStats,
  listPrizeStats,
  simulate,
  recommend,
  previewProfit,
  applySimulation,
}

export default adminZenxiangLiyuAPI
