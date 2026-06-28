import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('openai auto scheduler locale copy', () => {
  it('exposes admin page title and description at the route meta keys', () => {
    expect(zh.admin.openaiAutoScheduler.title).toBe('OpenAI 自动调度')
    expect(zh.admin.openaiAutoScheduler.description).toContain('OpenAI 账号调度分数')
    expect(en.admin.openaiAutoScheduler.title).toBe('OpenAI Auto Scheduler')
    expect(en.admin.openaiAutoScheduler.description).toContain('OpenAI account scheduling scores')
  })
})
