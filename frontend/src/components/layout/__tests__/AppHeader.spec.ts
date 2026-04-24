import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader QQ group entry', () => {
  it('contains the QQ group quick-join link and group number', () => {
    expect(componentSource).toContain("const qqGroupLink = 'https://qm.qq.com/q/73qQYmuU0g'")
    expect(componentSource).toContain('1006817250')
    expect(componentSource).toContain('点击链接加入群聊【llmx公益站】')
    expect(componentSource).toContain('加QQ群')
  })
})
