import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export type WorkbenchMode = 'chat' | 'image'
export type WorkbenchEndpoint = 'chat_completions' | 'images_generations'
export type WorkbenchRole = 'user' | 'assistant' | 'system'
export type WorkbenchMessageStatus = 'pending' | 'success' | 'error'

export interface WorkbenchConversation {
  id: number
  user_id: number
  title: string
  mode: WorkbenchMode
  api_key_id?: number | null
  endpoint: WorkbenchEndpoint
  model: string
  last_message_preview: string
  last_error?: string | null
  message_count: number
  created_at: string
  updated_at: string
}

export interface WorkbenchImageOutput {
  url?: string
  b64_json?: string
  mime_type?: string
}

export interface WorkbenchMessage {
  id: number
  conversation_id: number
  user_id: number
  mode: WorkbenchMode
  role: WorkbenchRole
  content: string
  api_key_id?: number | null
  endpoint: WorkbenchEndpoint
  model: string
  request_options: Record<string, unknown>
  response_metadata: Record<string, unknown>
  image_outputs: WorkbenchImageOutput[]
  status: WorkbenchMessageStatus
  error_message?: string | null
  created_at: string
  updated_at: string
}

export interface CreateWorkbenchConversationRequest {
  mode?: WorkbenchMode
  title?: string
  api_key_id?: number | null
  endpoint?: WorkbenchEndpoint
  model?: string
}

export interface WorkbenchSendRequest {
  mode: WorkbenchMode
  api_key_id: number
  endpoint: WorkbenchEndpoint
  model: string
  input: string
  options?: Record<string, unknown>
}

export interface WorkbenchSendResult {
  user_message: WorkbenchMessage
  assistant_message: WorkbenchMessage
  conversation: WorkbenchConversation
}

async function listConversations(params?: { mode?: WorkbenchMode; page?: number; page_size?: number }): Promise<PaginatedResponse<WorkbenchConversation>> {
  const { data } = await apiClient.get<PaginatedResponse<WorkbenchConversation>>('/workbench/conversations', { params })
  return data
}

async function createConversation(payload: CreateWorkbenchConversationRequest): Promise<WorkbenchConversation> {
  const { data } = await apiClient.post<WorkbenchConversation>('/workbench/conversations', payload)
  return data
}

async function listMessages(conversationId: number): Promise<WorkbenchMessage[]> {
  const { data } = await apiClient.get<WorkbenchMessage[]>(`/workbench/conversations/${conversationId}/messages`)
  return data
}

async function deleteConversation(conversationId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/workbench/conversations/${conversationId}`)
  return data
}

async function send(conversationId: number, payload: WorkbenchSendRequest): Promise<WorkbenchSendResult> {
  const { data } = await apiClient.post<WorkbenchSendResult>(`/workbench/conversations/${conversationId}/send`, payload)
  return data
}

export const workbenchAPI = { listConversations, createConversation, listMessages, deleteConversation, send }
export default workbenchAPI
