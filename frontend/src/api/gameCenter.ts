import { apiClient } from './client'
import type {
  ExchangeBalanceToPointsRequest,
  ExchangePointsToBalanceRequest,
  GameCenterOverview,
} from '@/types/gameCenter'

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

export const gameCenterAPI = {
  getOverview,
  claimPoints,
  exchangeBalanceToPoints,
  exchangePointsToBalance,
}

export default gameCenterAPI
