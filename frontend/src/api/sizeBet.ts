import { apiClient } from './client'
import type {
  PlaceSizeBetRequest,
  SizeBetBet,
  SizeBetCurrentRoundView,
  SizeBetHistoryView,
  SizeBetLeaderboardView,
  SizeBetRoundsView,
  SizeBetRulesView,
  SizeBetStatsOverview,
  SizeBetStatsUsersView,
} from '@/types/sizeBet'

export async function getCurrent(): Promise<SizeBetCurrentRoundView> {
  const { data } = await apiClient.get<SizeBetCurrentRoundView>('/game/size-bet/current')
  return data
}

export async function getRules(): Promise<SizeBetRulesView> {
  const { data } = await apiClient.get<SizeBetRulesView>('/game/size-bet/rules')
  return data
}

export async function getHistory(page = 1, pageSize = 10): Promise<SizeBetHistoryView> {
  const { data } = await apiClient.get<SizeBetHistoryView>('/game/size-bet/history', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function getRounds(page = 1, pageSize = 10): Promise<SizeBetRoundsView> {
  const { data } = await apiClient.get<SizeBetRoundsView>('/game/size-bet/rounds', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function getStatsOverview(date = ''): Promise<SizeBetStatsOverview> {
  const { data } = await apiClient.get<SizeBetStatsOverview>('/game/size-bet/stats/overview', {
    params: { date },
  })
  return data
}

export async function listStatsUsers(page = 1, pageSize = 20, date = ''): Promise<SizeBetStatsUsersView> {
  const { data } = await apiClient.get<SizeBetStatsUsersView>('/game/size-bet/stats/users', {
    params: { page, page_size: pageSize, date },
  })
  return data
}

export async function getLeaderboard(scope = 'all'): Promise<SizeBetLeaderboardView> {
  const { data } = await apiClient.get<SizeBetLeaderboardView>('/game/size-bet/leaderboard', {
    params: { scope },
  })
  return data
}

export async function placeBet(payload: PlaceSizeBetRequest): Promise<SizeBetBet> {
  const { data } = await apiClient.post<SizeBetBet>('/game/size-bet/bet', payload)
  return data
}

export const sizeBetAPI = {
  getCurrent,
  getRules,
  getHistory,
  getRounds,
  getStatsOverview,
  listStatsUsers,
  getLeaderboard,
  placeBet,
}

export default sizeBetAPI
