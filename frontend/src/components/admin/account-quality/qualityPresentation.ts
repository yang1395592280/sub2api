import type { QualityCheck, QualityPolicy, QualityRun } from '@/api/admin/openaiAccountQuality'

export function qualityVisualOutcome(check?: QualityCheck, runStatus = 'completed'): string {
  if (['queued', 'running'].includes(runStatus)) return runStatus
  if (!check) return 'not_reviewed'
  if (['passed', 'incorrect'].includes(check.verdict)) return check.verdict
  if (['review_not_configured', 'renderer_not_configured', 'review_pending', 'generation_completed'].includes(check.reason_code)) return 'not_reviewed'
  if (check.reason_code === 'review_uncertain') return 'visual_uncertain'
  return 'visual_check_error'
}

export interface QualityAccountRow {
  key: string
  accountId: number
  accountName: string
  sourceGroupId: number
  policies: QualityPolicy[]
}

export function groupQualityPolicies(policies: QualityPolicy[]): QualityAccountRow[] {
  const rows = new Map<string, QualityAccountRow>()
  for (const policy of policies) {
    const key = `${policy.account_id}:${policy.source_group_id}`
    let row = rows.get(key)
    if (!row) {
      row = { key, accountId: policy.account_id, accountName: policy.account_name, sourceGroupId: policy.source_group_id, policies: [] }
      rows.set(key, row)
    }
    row.policies.push(policy)
  }
  return [...rows.values()]
}

export function qualityRunOutcome(run: QualityRun): string {
  return run.verdict || run.status
}

export function qualityOutcomeClass(value: string): string {
  if (['passed', 'normal', 'fresh', 'healthy'].includes(value)) return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300'
  if (['incorrect', 'quarantined', 'recovering', 'deny'].includes(value)) return 'bg-red-50 text-red-700 dark:bg-red-900/25 dark:text-red-300'
  if (['unknown', 'visual_uncertain', 'visual_check_error', 'observing', 'stale', 'fallback'].includes(value)) return 'bg-amber-50 text-amber-700 dark:bg-amber-900/25 dark:text-amber-300'
  if (['running', 'queued'].includes(value)) return 'bg-primary-50 text-primary-700 dark:bg-primary-900/25 dark:text-primary-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
}
