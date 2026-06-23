import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('account routing status locale keys', () => {
  it('uses routing status wording in zh account list', () => {
    expect(zh.admin.accounts.columns.stability).toBe('调度状态')
    expect(zh.admin.accounts.stability.noData).toBe('暂无调度状态数据')
    expect(zh.admin.accounts.stabilityHint).toContain('主力、备用、观察、隔离')
  })

  it('uses routing status wording in en account list', () => {
    expect(en.admin.accounts.columns.stability).toBe('Routing Status')
    expect(en.admin.accounts.stability.noData).toBe('No routing status data')
    expect(en.admin.accounts.stabilityHint).toContain('primary, standby, observe, isolated')
  })
})
