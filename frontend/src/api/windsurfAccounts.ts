import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface WindsurfAccountItem {
  id: number
  account: string
  password_masked: string
  enabled: boolean
  maintained_by_id: number
  maintained_by_name: string
  maintained_by_email: string
  maintained_at: string
  status_updated_by?: number
  status_updated_at?: string
  created_at: string
  updated_at: string
}

export interface WindsurfAccountListParams {
  page?: number
  page_size?: number
  search?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export async function listWindsurfAccounts(params: WindsurfAccountListParams = {}): Promise<BasePaginationResponse<WindsurfAccountItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<WindsurfAccountItem>>('/windsurf-accounts', {
    params,
  })
  return data
}

export async function createWindsurfAccount(payload: {
  account: string
  password: string
}): Promise<WindsurfAccountItem> {
  const { data } = await apiClient.post<WindsurfAccountItem>('/windsurf-accounts', payload)
  return data
}

export async function updateWindsurfAccount(id: number, payload: {
  account: string
  password?: string
}): Promise<WindsurfAccountItem> {
  const { data } = await apiClient.put<WindsurfAccountItem>(`/windsurf-accounts/${id}`, payload)
  return data
}

export async function updateWindsurfAccountStatus(id: number, enabled: boolean): Promise<WindsurfAccountItem> {
  const { data } = await apiClient.put<WindsurfAccountItem>(`/windsurf-accounts/${id}/status`, { enabled })
  return data
}

export async function revealWindsurfAccountPassword(id: number): Promise<string> {
  const { data } = await apiClient.get<{ password: string }>(`/windsurf-accounts/${id}/password`)
  return data.password
}

export async function deleteWindsurfAccount(id: number): Promise<void> {
  await apiClient.delete(`/windsurf-accounts/${id}`)
}

export const windsurfAccountsAPI = {
  list: listWindsurfAccounts,
  create: createWindsurfAccount,
  update: updateWindsurfAccount,
  updateStatus: updateWindsurfAccountStatus,
  revealPassword: revealWindsurfAccountPassword,
  delete: deleteWindsurfAccount,
}

export default windsurfAccountsAPI
