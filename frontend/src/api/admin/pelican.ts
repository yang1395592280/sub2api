import { apiClient } from '../client'
import type { GenerationTestKind } from './accounts'

export interface GroupPelicanConfig {
  question_ids: number[]
  group_id: number
  enabled: boolean
  model_id: string
  prompt: string
  interval_minutes: number
  last_run_at: string | null
  next_run_at: string | null
}

export const DEFAULT_PELICAN_PROMPT = '创建一个HTML，内容是绘制一个鹈鹕骑自行车的2D动画，你不需要任何测试'

export async function getGroupPelicanConfig(groupId: number, kind: GenerationTestKind = 'pelican'): Promise<GroupPelicanConfig> {
  return (await apiClient.get<GroupPelicanConfig>(`/admin/groups/${groupId}/${kind}-test`)).data
}

export async function saveGroupPelicanConfig(groupId: number, config: Pick<GroupPelicanConfig, 'enabled' | 'model_id' | 'prompt' | 'interval_minutes' | 'question_ids'>, kind: GenerationTestKind = 'pelican'): Promise<GroupPelicanConfig> {
  return (await apiClient.put<GroupPelicanConfig>(`/admin/groups/${groupId}/${kind}-test`, config)).data
}

export interface GenerationTestQuestion {
  id: number
  test_kind: GenerationTestKind
  name: string
  prompt: string
  expected_answer: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export type GenerationTestQuestionInput = Pick<GenerationTestQuestion, 'test_kind' | 'name' | 'prompt' | 'expected_answer' | 'enabled'>

export async function listTestQuestions(kind: GenerationTestKind): Promise<GenerationTestQuestion[]> {
  return (await apiClient.get<GenerationTestQuestion[]>('/admin/test-questions', { params: { test_kind: kind } })).data
}

export async function saveTestQuestion(id: number | null, input: GenerationTestQuestionInput): Promise<GenerationTestQuestion> {
  return (id == null
    ? await apiClient.post<GenerationTestQuestion>('/admin/test-questions', input)
    : await apiClient.put<GenerationTestQuestion>(`/admin/test-questions/${id}`, input)).data
}

export async function getTestPreference(kind: GenerationTestKind): Promise<{ question_ids: number[] }> {
  return (await apiClient.get<{ question_ids: number[] }>('/admin/test-questions/preferences', { params: { test_kind: kind } })).data
}

export const DEFAULT_INTELLIGENCE_PROMPT = `在一个黑色的袋子里放有三种口味的糖果，每种糖果有两种不同的形状（圆形和五角星形，不同的形状靠手感可以分辨）。已知不同口味的糖果和不同形状的数量统计如下表。最少取出多少个糖果才能保证手中同时拥有不同形状的苹果味和桃子味的糖？（手中有圆形苹果味配五角星桃子味，或有圆形桃子味配五角星苹果味都满足）

| | 苹果味 | 桃子味 | 西瓜味 |
|---|---|---|---|
| 圆形 | 7 | 9 | 8 |
| 五角星形 | 6 | 6 | 4 |

请给出最终答案数字。
不要联网。`
