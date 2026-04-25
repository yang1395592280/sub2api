import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export interface GameCenterClaimScheduleItem {
  batch_key: string
  claim_time: string
  points_amount: number
  enabled: boolean
}

export interface GameCenterExchangeSettings {
  balance_to_points_enabled: boolean
  points_to_balance_enabled: boolean
  balance_to_points_rate: number
  points_to_balance_rate: number
  min_balance_amount: number
  min_points_amount: number
}

export interface GameCenterAdminSettings {
  game_center_enabled: boolean
  claim_enabled: boolean
  claim_schedule: GameCenterClaimScheduleItem[]
  exchange: GameCenterExchangeSettings
}

export type UpdateGameCenterSettingsRequest = GameCenterAdminSettings

export interface GameCenterCatalogItem {
  game_key: string
  name: string
  subtitle: string
  cover_image?: string
  description: string
  enabled: boolean
  sort_order: number
  default_open_mode: 'embed' | 'standalone' | 'dual'
  supports_embed: boolean
  supports_standalone: boolean
}

export interface UpdateGameCenterCatalogRequest {
  enabled: boolean
  sort_order: number
  default_open_mode: 'embed' | 'standalone' | 'dual'
  supports_embed: boolean
  supports_standalone: boolean
}

export interface GameCenterAdminLedgerItem {
  id: number
  user_id: number
  entry_type: string
  delta_points: number
  points_before: number
  points_after: number
  reason: string
  related_game_key: string
  created_at: string
}

export interface GameCenterClaimRecord {
  id: number
  user_id: number
  claim_date: string
  batch_key: string
  points_amount: number
  claimed_at: string
}

export interface GameCenterExchangeRecord {
  id: number
  user_id: number
  direction: 'balance_to_points' | 'points_to_balance'
  source_amount: number
  source_points: number
  target_amount: number
  target_points: number
  rate: number
  status: string
  reason: string
  created_at: string
}

export interface AdjustGameCenterPointsRequest {
  delta_points: number
  reason: string
}

export async function getSettings(): Promise<GameCenterAdminSettings> {
  const { data } = await apiClient.get<GameCenterAdminSettings>('/admin/game-center/settings')
  return data
}

export async function updateSettings(payload: UpdateGameCenterSettingsRequest): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/game-center/settings', payload)
  return data
}

export async function getCatalog(): Promise<GameCenterCatalogItem[]> {
  const { data } = await apiClient.get<GameCenterCatalogItem[]>('/admin/game-center/catalog')
  return data
}

export async function updateCatalog(gameKey: string, payload: UpdateGameCenterCatalogRequest): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>(`/admin/game-center/catalog/${gameKey}`, payload)
  return data
}

export async function listLedger(userID?: number): Promise<BasePaginationResponse<GameCenterAdminLedgerItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterAdminLedgerItem>>('/admin/game-center/ledger', {
    params: userID ? { user_id: userID } : undefined
  })
  return data
}

export async function listClaims(userID?: number): Promise<BasePaginationResponse<GameCenterClaimRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterClaimRecord>>('/admin/game-center/claims', {
    params: userID ? { user_id: userID } : undefined
  })
  return data
}

export async function listExchanges(userID?: number): Promise<BasePaginationResponse<GameCenterExchangeRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<GameCenterExchangeRecord>>('/admin/game-center/exchanges', {
    params: userID ? { user_id: userID } : undefined
  })
  return data
}

export async function adjustPoints(userID: number, payload: AdjustGameCenterPointsRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/game-center/users/${userID}/points/adjust`, payload)
  return data
}

const gameCenterAdminAPI = {
  getSettings,
  updateSettings,
  getCatalog,
  updateCatalog,
  listLedger,
  listClaims,
  listExchanges,
  adjustPoints
}

export default gameCenterAdminAPI
