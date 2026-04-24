import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type { SizeBetDirection, SizeBetRoundStatus, SizeBetStatus } from '@/types/sizeBet'

type SizeBetSettingsPayload = {
  enabled: boolean
  round_duration_seconds: number
  bet_close_offset_seconds: number
  allowed_stakes: number[]
  custom_stake_min: number
  custom_stake_max: number
  probabilities: {
    small: number
    mid: number
    big: number
  }
  odds: {
    small: number
    mid: number
    big: number
  }
  rules_markdown: string
}

type RawSizeBetSettingsResponse = {
  enabled: boolean
  round_duration_seconds: number
  bet_close_offset_seconds: number
  allowed_stakes: number[]
  custom_stake_min?: number
  custom_stake_max?: number
  prob_small?: number
  prob_mid?: number
  prob_big?: number
  odds_small?: number
  odds_mid?: number
  odds_big?: number
  probabilities?: SizeBetSettingsPayload['probabilities']
  odds?: SizeBetSettingsPayload['odds']
  rules_markdown: string
}

export interface SizeBetAdminRound {
  id: number
  round_no: number
  status: SizeBetRoundStatus
  starts_at: string
  bet_closes_at: string
  settles_at: string
  prob_small: number
  prob_mid: number
  prob_big: number
  odds_small: number
  odds_mid: number
  odds_big: number
  allowed_stakes: number[]
  result_number?: number | null
  result_direction?: SizeBetDirection | '' | null
  server_seed_hash?: string
  server_seed?: string | null
}

export interface SizeBetAdminBet {
  id: number
  round_id: number
  round_no: number
  user_id: number
  username: string
  direction: SizeBetDirection
  stake_amount: number
  payout_amount: number
  net_result_amount: number
  status: SizeBetStatus
  placed_at: string
  settled_at?: string | null
}

export interface SizeBetAdminLedgerEntry {
  id: number
  user_id: number
  round_id?: number | null
  bet_id?: number | null
  entry_type: string
  direction?: string
  stake_amount: number
  delta_amount: number
  balance_before: number
  balance_after: number
  reason?: string
  created_at: string
}

export interface SizeBetRefundResult {
  round_id: number
  refunded_count: number
  refunded_at: string
}

export interface SizeBetStatsOverview {
  date: string
  participant_count: number
  total_stake: number
  total_payout: number
  total_user_net: number
  house_net: number
}

export interface SizeBetStatsUserItem {
  user_id: number
  username: string
  total_stake: number
  won_count: number
  lost_count: number
  refunded_count: number
  net_result: number
}

export type SizeBetAdminSettings = SizeBetSettingsPayload
export type UpdateSizeBetSettingsRequest = SizeBetSettingsPayload

function normalizeSettings(data: RawSizeBetSettingsResponse): SizeBetAdminSettings {
  return {
    enabled: data.enabled,
    round_duration_seconds: data.round_duration_seconds,
    bet_close_offset_seconds: data.bet_close_offset_seconds,
    allowed_stakes: [...(data.allowed_stakes ?? [])],
    custom_stake_min: data.custom_stake_min ?? 1,
    custom_stake_max: data.custom_stake_max ?? 9999,
    probabilities: data.probabilities ?? {
      small: data.prob_small ?? 0,
      mid: data.prob_mid ?? 0,
      big: data.prob_big ?? 0
    },
    odds: data.odds ?? {
      small: data.odds_small ?? 0,
      mid: data.odds_mid ?? 0,
      big: data.odds_big ?? 0
    },
    rules_markdown: data.rules_markdown
  }
}

export async function getSettings(): Promise<SizeBetAdminSettings> {
  const { data } = await apiClient.get<RawSizeBetSettingsResponse>('/admin/games/size-bet/settings')
  return normalizeSettings(data)
}

export async function updateSettings(payload: UpdateSizeBetSettingsRequest): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/games/size-bet/settings', payload)
  return data
}

export async function listRounds(page = 1, pageSize = 20): Promise<BasePaginationResponse<SizeBetAdminRound>> {
  const { data } = await apiClient.get<BasePaginationResponse<SizeBetAdminRound>>('/admin/games/size-bet/rounds', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function listBets(
  page = 1,
  pageSize = 20,
  filters?: { round_id?: number; user_id?: number; status?: string }
): Promise<BasePaginationResponse<SizeBetAdminBet>> {
  const { data } = await apiClient.get<BasePaginationResponse<SizeBetAdminBet>>('/admin/games/size-bet/bets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function listLedger(
  page = 1,
  pageSize = 20,
  filters?: { round_id?: number; user_id?: number; entry_type?: string }
): Promise<BasePaginationResponse<SizeBetAdminLedgerEntry>> {
  const { data } = await apiClient.get<BasePaginationResponse<SizeBetAdminLedgerEntry>>('/admin/games/size-bet/ledger', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function refundRound(roundID: number): Promise<SizeBetRefundResult> {
  const { data } = await apiClient.post<SizeBetRefundResult>(`/admin/games/size-bet/rounds/${roundID}/refund`)
  return data
}

export async function getStatsOverview(date: string): Promise<SizeBetStatsOverview> {
  const { data } = await apiClient.get<SizeBetStatsOverview>('/admin/games/size-bet/stats/overview', {
    params: { date }
  })
  return data
}

export async function listStatsUsers(page = 1, pageSize = 20, date = ''): Promise<BasePaginationResponse<SizeBetStatsUserItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<SizeBetStatsUserItem>>('/admin/games/size-bet/stats/users', {
    params: { page, page_size: pageSize, date }
  })
  return data
}

const sizeBetAdminAPI = {
  getSettings,
  updateSettings,
  listRounds,
  listBets,
  listLedger,
  refundRound,
  getStatsOverview,
  listStatsUsers
}

export default sizeBetAdminAPI
