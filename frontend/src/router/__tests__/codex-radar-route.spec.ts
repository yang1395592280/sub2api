import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('Codex Radar route', () => {
  it('registers the authenticated in-app page', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')

    expect(source).toContain("path: '/codex-radar'")
    expect(source).toContain("name: 'CodexRadar'")
    expect(source).toContain("import('@/views/user/CodexRadarView.vue')")
    expect(source).toContain("titleKey: 'codexRadar.title'")
  })
})
