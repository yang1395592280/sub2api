import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { WorkbenchMessage, WorkbenchMode, WorkbenchConversation } from '@/api/workbench'

export interface AdminWorkbenchConversation extends WorkbenchConversation {
  user_email: string
  username: string
  image_count: number
  image_bytes: number
}

export interface AdminWorkbenchStats {
  total_conversations: number
  total_messages: number
  image_messages: number
  expired_conversations: number
  image_bytes: number
  retention_days: number
}

export interface AdminWorkbenchConversationFilters {
  page?: number
  page_size?: number
  mode?: WorkbenchMode | ''
  status?: 'pending' | 'success' | 'error' | ''
  search?: string
  user_id?: number
  has_images?: boolean
  older_than_days?: number
}

export interface AdminWorkbenchConversationDetail {
  conversation: AdminWorkbenchConversation
  messages: WorkbenchMessage[]
}

async function getStats(retentionDays = 7): Promise<AdminWorkbenchStats> {
  const { data } = await apiClient.get<AdminWorkbenchStats>('/admin/workbench/stats', {
    params: { retention_days: retentionDays },
  })
  return data
}

async function listConversations(params: AdminWorkbenchConversationFilters): Promise<PaginatedResponse<AdminWorkbenchConversation>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminWorkbenchConversation>>('/admin/workbench/conversations', { params })
  return data
}

async function getConversation(conversationId: number): Promise<AdminWorkbenchConversationDetail> {
  const { data } = await apiClient.get<AdminWorkbenchConversationDetail>(`/admin/workbench/conversations/${conversationId}`)
  return data
}

async function batchDeleteConversations(conversationIds: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/workbench/conversations/batch-delete', {
    conversation_ids: conversationIds,
  })
  return data
}

async function cleanupExpiredConversations(retentionDays = 7): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/workbench/conversations/cleanup-expired', {
    retention_days: retentionDays,
  })
  return data
}

export const adminWorkbenchAPI = {
  getStats,
  listConversations,
  getConversation,
  batchDeleteConversations,
  cleanupExpiredConversations,
}

export default adminWorkbenchAPI
