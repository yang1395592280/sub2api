import type { GenerationTestKind, PelicanTest } from '@/api/admin/accounts'

export const isGenerationActive = (test: PelicanTest) => ['queued', 'running'].includes(test.status)
export function summarizeGenerationTests(rows: PelicanTest[], kind: GenerationTestKind) {
  const done = rows.filter(row => !isGenerationActive(row)).length
  const success = rows.filter(row => row.status === (kind === 'intelligence' ? 'correct' : 'previewable')).length
  return { total: rows.length, done, success, failed: done - success }
}
export function groupGenerationBatches(rows: PelicanTest[]) {
  const groups = new Map<string, PelicanTest[]>()
  for (const row of rows) {
    const key = row.batch_id || String(row.id)
    const group = groups.get(key) ?? []
    group.push(row)
    groups.set(key, group)
  }
  return Array.from(groups, ([id, items]) => ({ id, items }))
}
