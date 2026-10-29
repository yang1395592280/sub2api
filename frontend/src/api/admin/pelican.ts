import { apiClient } from '../client'

export interface GroupPelicanConfig {
  group_id: number
  enabled: boolean
  model_id: string
  prompt: string
  interval_minutes: number
  last_run_at: string | null
  next_run_at: string | null
}

export const DEFAULT_PELICAN_PROMPT = '创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试'

export async function getGroupPelicanConfig(groupId: number): Promise<GroupPelicanConfig> {
  return (await apiClient.get<GroupPelicanConfig>(`/admin/groups/${groupId}/pelican-test`)).data
}

export async function saveGroupPelicanConfig(groupId: number, config: Pick<GroupPelicanConfig, 'enabled' | 'model_id' | 'prompt'>): Promise<GroupPelicanConfig> {
  return (await apiClient.put<GroupPelicanConfig>(`/admin/groups/${groupId}/pelican-test`, config)).data
}
