import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('OpenAI health route', () => {
  it('registers the admin OpenAI health dashboard route', () => {
    expect(routerSource).toContain("path: '/admin/openai-health'")
    expect(routerSource).toContain("name: 'AdminOpenAIHealth'")
    expect(routerSource).toContain("import('@/views/admin/OpenAIHealthView.vue')")
    expect(routerSource).toContain("titleKey: 'admin.openaiHealth.title'")
  })
})
