import { apiClient } from '../client'

export interface DailyUsage {
  total_cost: number
  paid_cost: number
  welfare_cost: number
  checkin_cost: number
  liyu_cost: number
  gift_cost: number
  subscription_cost: number
  unclassified_cost: number
  requests: number
  tokens: number
}

export interface DailyActivityStats {
  event_count: number
  checkin_events: number
  checkin_count: number
  checkin_base: number
  checkin_bonus: number
  checkin_net: number
  liyu_events: number
  liyu_draws: number
  liyu_reward: number
  liyu_lucky: number
  liyu_guess: number
  liyu_net: number
  liyu_ticket_face_value: number
  gift_credit: number
  welfare_adjustment: number
}

export interface UserDailyStatsRow {
  user_id: number
  email: string
  username: string
  status: string
  deleted: boolean
  balance: number
  welfare_balance: number
  checkin_balance: number
  liyu_balance: number
  gift_balance: number
  frozen_balance: number
  total_recharged: number
  spending_rank: number | null
  today_usage: DailyUsage
  previous_usage: DailyUsage
  activity: DailyActivityStats
}

export interface UserDailyStatsSummary {
  users: number
  active_users: number
  checkin_users: number
  liyu_users: number
  spending_users: number
  balance: number
  welfare_balance: number
  today_usage: DailyUsage
  previous_usage: DailyUsage
  activity: DailyActivityStats
}

export interface DailyPage<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface UserDailyStatsResponse extends DailyPage<UserDailyStatsRow> {
  date: string
  previous_date: string
  timezone: string
  generated_at: string
  summary: UserDailyStatsSummary
}

export interface UserDailyActivity {
  kind: string
  record_id: number
  amount: number
  ticket_face_value: number
  outcome: string
  description: string
  created_at: string
}

export type DailySort = 'today_cost' | 'previous_cost' | 'checkin_net' | 'liyu_net' | 'balance' | 'welfare_balance' | 'requests'
export type DailyActivityFilter = 'all' | 'active' | 'checkin' | 'liyu' | 'usage'

export interface UserDailyStatsParams {
  date: string
  search?: string
  activity?: DailyActivityFilter
  sort_by?: DailySort
  sort_order?: 'asc' | 'desc'
  page?: number
  page_size?: number
}

export const userDailyStatsAPI = {
  async list(params: UserDailyStatsParams, signal?: AbortSignal) {
    return (await apiClient.get<UserDailyStatsResponse>('/admin/user-daily-stats', { params, signal })).data
  },
  async activities(userID: number, params: Pick<UserDailyStatsParams, 'date' | 'page' | 'page_size'>, signal?: AbortSignal) {
    return (await apiClient.get<DailyPage<UserDailyActivity>>(`/admin/user-daily-stats/${userID}/activities`, { params, signal })).data
  }
}
