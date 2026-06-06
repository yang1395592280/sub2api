import type { BasePaginationResponse } from './index'

export type SizeBetDirection = 'small' | 'mid' | 'big'
export type SizeBetRoundStatus = 'open' | 'settled'
export type SizeBetStatus = 'placed' | 'won' | 'lost' | 'refunded'
export type SizeBetPhase = 'betting' | 'closed' | 'preparing' | 'maintenance'

export interface SizeBet {
  id: number
  round_id: number
  direction: SizeBetDirection
  stake_amount: number
  payout_amount: number
  net_result_amount: number
  status: SizeBetStatus
  placed_at?: string
  settled_at?: string
}

export interface SizeBetCurrentRound {
  id: number
  round_no: number
  status: SizeBetRoundStatus | string
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
  server_seed_hash: string
  countdown_seconds: number
  bet_countdown_seconds: number
}

export interface SizeBetRoundSummary {
  id: number
  round_no: number
  status: SizeBetRoundStatus | string
  starts_at: string
  settles_at: string
  result_number?: number
  result_direction?: SizeBetDirection | string
  server_seed_hash?: string
  server_seed?: string
}

export interface SizeBetCurrentRoundView {
  enabled: boolean
  phase: SizeBetPhase | string
  server_time: string
  round?: SizeBetCurrentRound
  my_bet?: SizeBet
  previous_round?: SizeBetRoundSummary
}

export interface SizeBetPlaceBetRequest {
  round_id: number
  direction: SizeBetDirection
  stake_amount: number
  idempotency_key?: string
}

export interface SizeBetUserHistoryItem {
  bet_id: number
  round_no: number
  direction: SizeBetDirection | string
  selection: SizeBetDirection | string
  result_number?: number
  result_direction?: SizeBetDirection | string
  stake_amount: number
  payout_amount: number
  net_result_amount: number
  points_after?: number
  status: SizeBetStatus | string
  placed_at: string
  settled_at?: string
}

export interface SizeBetLeaderboardEntry {
  rank: number
  user_id: number
  email: string
  username: string
  points: number
  net_profit: number
  win_count: number
  bet_count: number
  hit_rate: number
}

export interface SizeBetLeaderboardView {
  scope: string
  scope_key: string
  refreshed_at?: string
  items: SizeBetLeaderboardEntry[]
}

export interface SizeBetProbabilityConfig {
  small: number
  mid: number
  big: number
}

export interface SizeBetOddsConfig {
  small: number
  mid: number
  big: number
}

export interface SizeBetRulesView {
  enabled: boolean
  round_duration_seconds: number
  bet_close_offset_seconds: number
  allowed_stakes: number[]
  custom_stake_min: number
  custom_stake_max: number
  probabilities: SizeBetProbabilityConfig
  odds: SizeBetOddsConfig
  rules_markdown: string
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
  username: string
  total_stake: number
  won_count: number
  lost_count: number
  refunded_count: number
  net_result: number
}

export type SizeBetHistoryResponse = BasePaginationResponse<SizeBetUserHistoryItem>
export type SizeBetRoundsResponse = BasePaginationResponse<SizeBetRoundSummary>
export type SizeBetStatsUsersResponse = BasePaginationResponse<SizeBetStatsUserItem>
