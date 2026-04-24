import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('common locale keys', () => {
  it('contains zh labels for used common action keys', () => {
    expect(zh.common.apply).toBe('应用')
    expect(zh.common.clear).toBe('清空')
    expect(zh.common.creating).toBe('创建中...')
    expect(zh.common.login).toBe('登录')
    expect(zh.common.required).toBe('不能为空')
    expect(zh.common.retry).toBe('重试')
    expect(zh.common.sending).toBe('发送中...')
    expect(zh.common.submitting).toBe('提交中...')
  })

  it('contains en labels for used common action keys', () => {
    expect(en.common.apply).toBe('Apply')
    expect(en.common.clear).toBe('Clear')
    expect(en.common.creating).toBe('Creating...')
    expect(en.common.login).toBe('Login')
    expect(en.common.required).toBe('is required')
    expect(en.common.retry).toBe('Retry')
    expect(en.common.sending).toBe('Sending...')
    expect(en.common.submitting).toBe('Submitting...')
  })
})
