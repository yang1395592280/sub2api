import { apiClient } from './client'
import type { LuckyWheelHistoryView, LuckyWheelLeaderboardView, LuckyWheelOverview, LuckyWheelSpinResult } from '@/types/luckyWheel'

export async function getOverview(): Promise<LuckyWheelOverview> {
  const { data } = await apiClient.get<LuckyWheelOverview>('/game/lucky-wheel/overview')
  return data
}

export async function spin(): Promise<LuckyWheelSpinResult> {
  const { data } = await apiClient.post<LuckyWheelSpinResult>('/game/lucky-wheel/spin')
  return data
}

export async function getHistory(page = 1, pageSize = 20): Promise<LuckyWheelHistoryView> {
  const { data } = await apiClient.get<LuckyWheelHistoryView>('/game/lucky-wheel/history', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function getLeaderboard(): Promise<LuckyWheelLeaderboardView> {
  const { data } = await apiClient.get<LuckyWheelLeaderboardView>('/game/lucky-wheel/leaderboard')
  return data
}

export const luckyWheelAPI = { getOverview, spin, getHistory, getLeaderboard }

export default luckyWheelAPI
