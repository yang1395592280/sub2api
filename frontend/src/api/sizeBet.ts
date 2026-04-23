import { apiClient } from './client'
import type {
  PlaceSizeBetRequest,
  SizeBetBet,
  SizeBetCurrentRoundView,
  SizeBetHistoryView,
  SizeBetRulesView,
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

export async function placeBet(payload: PlaceSizeBetRequest): Promise<SizeBetBet> {
  const { data } = await apiClient.post<SizeBetBet>('/game/size-bet/bet', payload)
  return data
}

export const sizeBetAPI = {
  getCurrent,
  getRules,
  getHistory,
  placeBet,
}

export default sizeBetAPI
