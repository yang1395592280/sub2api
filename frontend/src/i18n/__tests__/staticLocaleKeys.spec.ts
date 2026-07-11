import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

function flattenMessages(value: unknown, prefix = '', out = new Set<string>()): Set<string> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      flattenMessages(child, prefix ? `${prefix}.${key}` : key, out)
    }
    return out
  }

  if (prefix) out.add(prefix)
  return out
}

function collectSourceFiles(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      collectSourceFiles(file, out)
      continue
    }
    if (/\.(vue|ts|js)$/.test(entry.name) && !file.includes(`${path.sep}i18n${path.sep}locales${path.sep}`)) {
      out.push(file)
    }
  }
  return out
}

function collectStaticLocaleKeys(): string[] {
  const testDir = path.dirname(fileURLToPath(import.meta.url))
  const srcDir = path.resolve(testDir, '../..')
  const keys = new Set<string>()
  const pattern = /\b(?:t|te)\(\s*['"`]([^'"`]+)['"`]/g

  for (const file of collectSourceFiles(srcDir)) {
    const content = fs.readFileSync(file, 'utf8')
    let match: RegExpExecArray | null
    while ((match = pattern.exec(content))) {
      const key = match[1]
      if (!key.includes('+') && !key.includes('${') && !key.endsWith('.')) {
        keys.add(key)
      }
    }
  }

  return [...keys].sort()
}

describe('static locale keys', () => {
  it('has zh and en messages for every static t()/te() key', () => {
    const zhKeys = flattenMessages(zh)
    const enKeys = flattenMessages(en)
    const usedKeys = collectStaticLocaleKeys()

    const missingZh = usedKeys.filter((key) => !zhKeys.has(key))
    const missingEn = usedKeys.filter((key) => !enKeys.has(key))

    expect(missingZh).toEqual([])
    expect(missingEn).toEqual([])
  })
})
