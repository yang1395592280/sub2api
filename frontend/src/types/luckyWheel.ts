import type { BasePaginationResponse } from '@/types'

export type LuckyWheelPrizeType = 'reward' | 'penalty' | 'thanks'

export interface LuckyWheelPrizeConfig {
  key: string
  label: string
  type: LuckyWheelPrizeType
  delta_points: number
  probability: number
}

export interface LuckyWheelSpinRecord {
  id: number
  user_id: number
  email?: string
  username?: string
  spin_date: string
  spin_index: number
  prize_key: string
  prize_label: string
  prize_type: LuckyWheelPrizeType
  delta_points: number
  points_before: number
  points_after: number
  probability: number
  created_at: string
}

export interface LuckyWheelLeaderboardItem {
  rank: number
  user_id: number
  email: string
  username: string
  points: number
  net_points: number
  spin_count: number
  best_delta: number
  best_prize_label: string
}

export interface LuckyWheelOverview {
  enabled: boolean
  server_time: string
  points: number
  daily_spin_limit: number
  spins_used_today: number
  spins_remaining_today: number
  min_points_required: number
  prizes: LuckyWheelPrizeConfig[]
  rules_markdown: string
  leaderboard: LuckyWheelLeaderboardItem[]
  recent_history: LuckyWheelSpinRecord[]
}

export interface LuckyWheelSpinResult {
  record: LuckyWheelSpinRecord
  spins_used_today: number
  spins_remaining_today: number
}

export interface LuckyWheelLeaderboardView {
  date: string
  items: LuckyWheelLeaderboardItem[]
}

export interface LuckyWheelAdminSettings {
  enabled: boolean
  daily_spin_limit: number
  prizes: LuckyWheelPrizeConfig[]
  rules_markdown: string
}

export interface LuckyWheelAdminSpinQuery {
  user_id?: number
  start_date?: string
  end_date?: string
  page?: number
  page_size?: number
}

export type LuckyWheelHistoryView = BasePaginationResponse<LuckyWheelSpinRecord>
