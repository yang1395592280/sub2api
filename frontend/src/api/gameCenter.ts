import { apiClient } from './client'
import type {
  GameCenterOverview,
  GameCenterLedgerQuery,
  GameCenterLedgerResponse,
  LuckyWheelOverview,
  LuckyWheelSpinResult,
  LuckyWheelLeaderboardView,
  LuckyWheelHistoryResponse,
  SizeBetCurrentRoundView,
  SizeBetPlaceBetRequest,
  SizeBet,
  SizeBetHistoryResponse,
  SizeBetRoundsResponse,
  SizeBetStatsOverview,
  SizeBetStatsUsersResponse,
  SizeBetLeaderboardView,
  SizeBetRulesView,
} from '@/types'

export async function getOverview(params?: { page?: number; page_size?: number; timezone?: string }): Promise<GameCenterOverview> {
  const { data } = await apiClient.get<GameCenterOverview>('/game-center/overview', { params })
  return data
}

export async function getLedger(params?: GameCenterLedgerQuery): Promise<GameCenterLedgerResponse> {
  const { data } = await apiClient.get<GameCenterLedgerResponse>('/game-center/ledger', { params })
  return data
}

export const luckyWheelAPI = {
  async getOverview(): Promise<LuckyWheelOverview> {
    const { data } = await apiClient.get<LuckyWheelOverview>('/game/lucky-wheel/overview')
    return data
  },
  async spin(): Promise<LuckyWheelSpinResult> {
    const { data } = await apiClient.post<LuckyWheelSpinResult>('/game/lucky-wheel/spin')
    return data
  },
  async getHistory(params?: { page?: number; page_size?: number }): Promise<LuckyWheelHistoryResponse> {
    const { data } = await apiClient.get<LuckyWheelHistoryResponse>('/game/lucky-wheel/history', {
      params
    })
    return data
  },
  async getLeaderboard(): Promise<LuckyWheelLeaderboardView> {
    const { data } = await apiClient.get<LuckyWheelLeaderboardView>('/game/lucky-wheel/leaderboard')
    return data
  },
}

export const sizeBetAPI = {
  async getCurrent(): Promise<SizeBetCurrentRoundView> {
    const { data } = await apiClient.get<SizeBetCurrentRoundView>('/game/size-bet/current')
    return data
  },
  async placeBet(payload: SizeBetPlaceBetRequest): Promise<SizeBet> {
    const { data } = await apiClient.post<SizeBet>('/game/size-bet/bet', payload)
    return data
  },
  async getHistory(params?: { page?: number; page_size?: number }): Promise<SizeBetHistoryResponse> {
    const { data } = await apiClient.get<SizeBetHistoryResponse>('/game/size-bet/history', {
      params
    })
    return data
  },
  async getRounds(params?: { page?: number; page_size?: number }): Promise<SizeBetRoundsResponse> {
    const { data } = await apiClient.get<SizeBetRoundsResponse>('/game/size-bet/rounds', { params })
    return data
  },
  async getStatsOverview(params?: { date?: string }): Promise<SizeBetStatsOverview> {
    const { data } = await apiClient.get<SizeBetStatsOverview>('/game/size-bet/stats/overview', {
      params
    })
    return data
  },
  async getStatsUsers(params?: {
    date?: string
    page?: number
    page_size?: number
  }): Promise<SizeBetStatsUsersResponse> {
    const { data } = await apiClient.get<SizeBetStatsUsersResponse>('/game/size-bet/stats/users', {
      params
    })
    return data
  },
  async getLeaderboard(params?: { scope?: string }): Promise<SizeBetLeaderboardView> {
    const { data } = await apiClient.get<SizeBetLeaderboardView>('/game/size-bet/leaderboard', {
      params
    })
    return data
  },
  async getRules(): Promise<SizeBetRulesView> {
    const { data } = await apiClient.get<SizeBetRulesView>('/game/size-bet/rules')
    return data
  },
}

export const gameCenterAPI = {
  getOverview,
  getLedger,
}

export default gameCenterAPI
