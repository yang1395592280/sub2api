import { apiClient } from './client'
import type {
  PlaceSizeBetRequest,
  SizeBetBet,
  SizeBetCurrentRoundView,
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

export async function placeBet(payload: PlaceSizeBetRequest): Promise<SizeBetBet> {
  const { data } = await apiClient.post<SizeBetBet>('/game/size-bet/bet', payload)
  return data
}

export const sizeBetAPI = {
  getCurrent,
  getRules,
  placeBet,
}

export default sizeBetAPI
