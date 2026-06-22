import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('image API docs route', () => {
  it('registers a public static image API docs page', () => {
    expect(routerSource).toContain("path: '/image-api-docs'")
    expect(routerSource).toContain("name: 'ImageAPIDocs'")
    expect(routerSource).toContain("import('@/views/public/ImageAPIDocsView.vue')")
    expect(routerSource).toContain("requiresAuth: false")
    expect(routerSource).toContain("titleKey: 'imageApiDocs.title'")
    expect(routerSource).toContain("'/image-api-docs'")
  })
})
