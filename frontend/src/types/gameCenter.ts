import type { BasePaginationResponse } from './index'

export interface CheckinRecordSummary {
  checkin_date: string
  reward_points: number
  base_reward_points: number
  bonus_status: string
  bonus_delta_points: number
}

export interface CheckinTodayRecord {
  checkin_date: string
  reward_points: number
  base_reward_points: number
  bonus_status: string
  bonus_delta_points: number
}

export interface CheckinStats {
  total_reward_points: number
  total_checkins: number
  checkin_count: number
  checked_in_today: boolean
  records: CheckinRecordSummary[]
}

export interface CheckinStatus {
  enabled: boolean
  min_reward_points: number
  max_reward_points: number
  bonus_enabled: boolean
  bonus_available: boolean
  bonus_success_rate: number
  today_record?: CheckinTodayRecord
  stats: CheckinStats
}

export interface CheckinActionResponse {
  checkin_date: string
  reward_points: number
  base_reward_points?: number
  bonus_status?: string
  bonus_delta_points?: number
}

export interface GameCatalog {
  game_key: string
  name: string
  subtitle: string
  cover_image: string
  description: string
  enabled: boolean
  sort_order: number
  default_open_mode: string
  supports_embed: boolean
  supports_standalone: boolean
}

export interface GameCenterLedgerItem {
  id: number
  user_id?: number
  email?: string
  username?: string
  entry_type: string
  delta_points: number
  points_after: number
  reason: string
  created_at: string
}

export interface GameCenterOverview {
  enabled: boolean
  points: number
  checkin?: CheckinStatus
  catalogs: GameCatalog[]
  recent_ledger: GameCenterLedgerItem[]
}

export interface GameCenterLedgerQuery {
  page?: number
  page_size?: number
  start_time?: string
  end_time?: string
  start_date?: string
  end_date?: string
  timezone?: string
}

export type GameCenterLedgerResponse = BasePaginationResponse<GameCenterLedgerItem>
