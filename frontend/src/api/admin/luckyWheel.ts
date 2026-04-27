import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type { LuckyWheelAdminSettings, LuckyWheelAdminSpinQuery, LuckyWheelSpinRecord } from '@/types/luckyWheel'

export async function getSettings(): Promise<LuckyWheelAdminSettings> {
  const { data } = await apiClient.get<LuckyWheelAdminSettings>('/admin/games/lucky-wheel/settings')
  return data
}

export async function updateSettings(payload: LuckyWheelAdminSettings): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/games/lucky-wheel/settings', payload)
  return data
}

export async function listSpins(params?: LuckyWheelAdminSpinQuery): Promise<BasePaginationResponse<LuckyWheelSpinRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<LuckyWheelSpinRecord>>('/admin/games/lucky-wheel/spins', {
    params,
  })
  return data
}

const luckyWheelAdminAPI = { getSettings, updateSettings, listSpins }

export default luckyWheelAdminAPI
