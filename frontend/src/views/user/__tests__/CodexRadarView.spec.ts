import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('CodexRadarView', () => {
  it('embeds the same-origin proxy in a restricted sandbox', () => {
    const source = readFileSync(resolve(process.cwd(), 'src/views/user/CodexRadarView.vue'), 'utf8')

    expect(source).toContain('src="/api/v1/codex-radar/embed"')
    expect(source).toContain('sandbox="allow-forms allow-scripts"')
    expect(source).not.toContain('allow-same-origin')
    expect(source).not.toContain('target="_blank"')
  })
})
