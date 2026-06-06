import { apiClient } from './client'
import type {
  BasePaginationResponse,
  UsageLeaderboardQuery,
  UsageLeaderboardItem,
  UsageLeaderboardOverview,
} from '@/types'

export async function getOverview(
  params?: UsageLeaderboardQuery
): Promise<UsageLeaderboardOverview> {
  const { data } = await apiClient.get<UsageLeaderboardOverview>('/usage-leaderboard/overview', {
    params
  })
  return data
}

export async function getItems(
  params?: UsageLeaderboardQuery & { page?: number; page_size?: number }
): Promise<BasePaginationResponse<UsageLeaderboardItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<UsageLeaderboardItem>>(
    '/usage-leaderboard/items',
    { params }
  )
  return data
}

export const usageLeaderboardAPI = {
  getOverview,
  getItems,
}

export default usageLeaderboardAPI
