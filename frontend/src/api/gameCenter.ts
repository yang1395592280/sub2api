import { apiClient } from './client'
import type {
  ExchangeBalanceToPointsRequest,
  ExchangePointsToBalanceRequest,
  GameCenterLedgerItem,
  GameCenterOverview,
  GameCenterPointsLeaderboardItem,
} from '@/types/gameCenter'
import type { BasePaginationResponse } from '@/types'

export interface GameCenterLedgerParams {
  page?: number
  page_size?: number
  start_date?: string
  end_date?: string
}

export async function getOverview(): Promise<GameCenterOverview> {
  const { data } = await apiClient.get<GameCenterOverview>('/game-center/overview')
  return data
}

export async function claimPoints(batchKey: string): Promise<void> {
  await apiClient.post(`/game-center/claims/${batchKey}`)
}

export async function exchangeBalanceToPoints(payload: ExchangeBalanceToPointsRequest): Promise<void> {
  await apiClient.post('/game-center/exchange/balance-to-points', payload)
}

export async function exchangePointsToBalance(payload: ExchangePointsToBalanceRequest): Promise<void> {
  await apiClient.post('/game-center/exchange/points-to-balance', payload)
}

export async function getLedger(params: GameCenterLedgerParams = {}): Promise<BasePaginationResponse<GameCenterLedgerItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterLedgerItem>>('/game-center/ledger', { params })
  return data
}

export async function getPointsLeaderboard(page = 1, pageSize = 10): Promise<BasePaginationResponse<GameCenterPointsLeaderboardItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterPointsLeaderboardItem>>('/game-center/leaderboard', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function getUserLedger(userID: number, params: GameCenterLedgerParams = {}): Promise<BasePaginationResponse<GameCenterLedgerItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterLedgerItem>>(`/game-center/users/${userID}/ledger`, { params })
  return data
}

export const gameCenterAPI = {
  getOverview,
  claimPoints,
  exchangeBalanceToPoints,
  exchangePointsToBalance,
  getLedger,
  getPointsLeaderboard,
  getUserLedger,
}

export default gameCenterAPI
